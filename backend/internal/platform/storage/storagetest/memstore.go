// Package storagetest is an in-memory storage.Storage for tests: the object
// store a service or worker reads from and writes to, without MinIO or R2. It
// keeps the real store's observable semantics where a test could depend on
// them — a missing key is storage.ErrNotFound, GetRange is the first n bytes,
// GetByteRange follows the HTTP Range convention (inclusive bounds, a negative
// end meaning "to the end", a start past the object an error, as S3 answers
// 416), DeletePrefix is by string prefix, a presigned PUT carries the
// Content-Type header the client must replay, presigned URLs are fake but
// well-formed.
//
// Two module test files carried their own copy until 2026-09-19 (backlog #9);
// this is the one owner, and the one a third module (comic's import) can
// import — platform never imports a module, modules may import platform.
package storagetest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/portal/backend/internal/platform/storage"
)

// MemStore is safe for concurrent use; a worker test may Put from several
// goroutines while the test reads.
type MemStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
	sizes   map[string]int64
	calls   []string
}

var _ storage.Storage = (*MemStore)(nil)

// New returns an empty store.
func New() *MemStore {
	return &MemStore{objects: map[string][]byte{}, types: map[string]string{}, sizes: map[string]int64{}}
}

// Seed puts a copy of b in place without going through Put — what "the client
// uploaded this earlier" looks like to the code under test. Not a call.
func (m *MemStore) Seed(key string, b []byte, contentType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = append([]byte(nil), b...)
	m.types[key] = contentType
}

// SetSize overrides what Size reports for key, so a test can pretend an
// object is larger than the bytes it holds (a >50 MB upload, say) without
// allocating it.
func (m *MemStore) SetSize(key string, n int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sizes[key] = n
}

// Object returns a copy of what is stored under key, and whether anything is.
func (m *MemStore) Object(key string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), b...), true
}

// ContentType returns what the last Put or Seed declared for key ("" if none).
func (m *MemStore) ContentType(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.types[key]
}

// Keys lists every stored key, sorted.
func (m *MemStore) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.objects))
	for k := range m.objects {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Calls lists the storage.Storage methods invoked so far, in order, by name —
// so a test can assert what the code under test did NOT touch (a poster job
// deletes nothing), which the private fakes used to prove by panicking.
func (m *MemStore) Calls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.calls...)
}

// record appends a call under the lock the caller already holds.
func (m *MemStore) record(name string) { m.calls = append(m.calls, name) }

func (m *MemStore) Bucket() string { return "test" }

func (m *MemStore) PresignPut(_ context.Context, key, contentType string, ttl time.Duration) (*storage.PresignedRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("PresignPut")
	// The real store signs Content-Type into the request when one is given,
	// and the client must replay it verbatim.
	headers := map[string]string{}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	return &storage.PresignedRequest{URL: "http://store/" + key, Method: "PUT", Headers: headers, Expires: time.Now().Add(ttl)}, nil
}

func (m *MemStore) PresignGet(_ context.Context, key string, ttl time.Duration) (*storage.PresignedRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("PresignGet")
	return &storage.PresignedRequest{URL: "http://store/" + key, Method: "GET", Headers: map[string]string{}, Expires: time.Now().Add(ttl)}, nil
}

func (m *MemStore) Put(_ context.Context, key string, body io.Reader, contentType string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Put")
	m.objects[key] = b
	m.types[key] = contentType
	return nil
}

func (m *MemStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Get")
	b, ok := m.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// GetRange is the first n bytes — what a sniff reads.
func (m *MemStore) GetRange(_ context.Context, key string, n int64) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("GetRange")
	b, ok := m.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	if n < 0 {
		n = 0
	}
	if int64(len(b)) > n {
		b = b[:n]
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// GetByteRange mirrors the HTTP Range convention the real store implements:
// inclusive bounds, a negative end meaning "to the end of the object", and a
// start past the object refused — S3 answers 416 there, not an empty body.
func (m *MemStore) GetByteRange(_ context.Context, key string, start, end int64) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("GetByteRange")
	b, ok := m.objects[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	if start < 0 {
		start = 0
	}
	if start >= int64(len(b)) {
		return nil, fmt.Errorf("storagetest: range %d-%d starts past the end of %q (%d bytes)", start, end, key, len(b))
	}
	stop := int64(len(b))
	if end >= start && end+1 < stop {
		stop = end + 1
	}
	return io.NopCloser(bytes.NewReader(b[start:stop])), nil
}

func (m *MemStore) Size(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Size")
	if sz, ok := m.sizes[key]; ok {
		return sz, nil
	}
	b, ok := m.objects[key]
	if !ok {
		return 0, storage.ErrNotFound
	}
	return int64(len(b)), nil
}

func (m *MemStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Delete")
	delete(m.objects, key)
	delete(m.types, key)
	return nil
}

func (m *MemStore) DeletePrefix(_ context.Context, prefix string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("DeletePrefix")
	for k := range m.objects {
		if strings.HasPrefix(k, prefix) {
			delete(m.objects, k)
			delete(m.types, k)
		}
	}
	return nil
}

func (m *MemStore) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Exists")
	_, ok := m.objects[key]
	return ok, nil
}
