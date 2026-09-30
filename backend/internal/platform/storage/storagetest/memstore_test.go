package storagetest

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/portal/backend/internal/platform/storage"
)

// The fake's semantics are what module tests lean on; pin them.

// read drains a (reader, error) pair the way every caller wants to.
func read(t *testing.T, open func() (io.ReadCloser, error)) string {
	t.Helper()
	rc, err := open()
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMissingKeyIsErrNotFound(t *testing.T) {
	m := New()
	ctx := context.Background()
	if _, err := m.Get(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Get missing = %v, want storage.ErrNotFound", err)
	}
	if _, err := m.GetRange(ctx, "nope", 4); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetRange missing = %v, want storage.ErrNotFound", err)
	}
	if _, err := m.Size(ctx, "nope"); !errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("Size missing = %v, want storage.ErrNotFound", err)
	}
	if ok, _ := m.Exists(ctx, "nope"); ok {
		t.Fatal("Exists reported a missing key")
	}
}

func TestPutGetRangesAndSize(t *testing.T) {
	m := New()
	ctx := context.Background()
	if err := m.Put(ctx, "a/b", strings.NewReader("0123456789"), "text/plain"); err != nil {
		t.Fatal(err)
	}
	if got := read(t, func() (io.ReadCloser, error) { return m.Get(ctx, "a/b") }); got != "0123456789" {
		t.Fatalf("Get = %q", got)
	}
	if got := read(t, func() (io.ReadCloser, error) { return m.GetRange(ctx, "a/b", 4) }); got != "0123" {
		t.Fatalf("GetRange(4) = %q, want the first four bytes", got)
	}
	if got := read(t, func() (io.ReadCloser, error) { return m.GetRange(ctx, "a/b", 100) }); got != "0123456789" {
		t.Fatalf("GetRange past the end = %q, want everything", got)
	}
	for _, c := range []struct {
		start, end int64
		want       string
	}{
		{2, 4, "234"},    // inclusive bounds
		{5, -1, "56789"}, // negative end: to the end
		{8, 100, "89"},   // end past the object
		{-3, 1, "01"},    // start clamped at 0
	} {
		if got := read(t, func() (io.ReadCloser, error) { return m.GetByteRange(ctx, "a/b", c.start, c.end) }); got != c.want {
			t.Fatalf("GetByteRange(%d, %d) = %q, want %q", c.start, c.end, got, c.want)
		}
	}
	// A start past the object is refused, as S3 refuses it (416) — not an
	// empty body a caller might mistake for a short object.
	if _, err := m.GetByteRange(ctx, "a/b", 100, 200); err == nil || errors.Is(err, storage.ErrNotFound) {
		t.Fatalf("GetByteRange past the end = %v, want a range error", err)
	}
	if n, _ := m.Size(ctx, "a/b"); n != 10 {
		t.Fatalf("Size = %d", n)
	}
	m.SetSize("a/b", 1<<30)
	if n, _ := m.Size(ctx, "a/b"); n != 1<<30 {
		t.Fatalf("Size with override = %d, want the override", n)
	}
	if ct := m.ContentType("a/b"); ct != "text/plain" {
		t.Fatalf("ContentType = %q", ct)
	}
}

func TestSeedDeleteAndPrefix(t *testing.T) {
	m := New()
	ctx := context.Background()
	m.Seed("v/1/thumb", []byte("t"), "image/webp")
	m.Seed("v/1/medium", []byte("m"), "image/webp")
	m.Seed("v/2/thumb", []byte("t"), "image/webp")
	if got := strings.Join(m.Keys(), ","); got != "v/1/medium,v/1/thumb,v/2/thumb" {
		t.Fatalf("Keys = %s", got)
	}
	if err := m.DeletePrefix(ctx, "v/1/"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.Keys(), ","); got != "v/2/thumb" {
		t.Fatalf("after DeletePrefix Keys = %s", got)
	}
	if err := m.Delete(ctx, "v/2/thumb"); err != nil {
		t.Fatal(err)
	}
	if len(m.Keys()) != 0 || m.ContentType("v/2/thumb") != "" {
		t.Fatal("Delete left something behind")
	}
	// Seed copies in and Object copies out: mutating either side's slice
	// does not change the store.
	in := []byte("abc")
	m.Seed("x", in, "")
	in[0] = 'q'
	b, _ := m.Object("x")
	b[0] = 'z'
	if got, _ := m.Object("x"); string(got) != "abc" {
		t.Fatalf("the store shares a slice with a caller: %q", got)
	}
}

func TestCallsRecordWhatTheCodeUnderTestTouched(t *testing.T) {
	m := New()
	ctx := context.Background()
	m.Seed("k", []byte("x"), "") // not a call
	if _, err := m.Get(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if err := m.Put(ctx, "k2", strings.NewReader("y"), ""); err != nil {
		t.Fatal(err)
	}
	if err := m.DeletePrefix(ctx, "k"); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(m.Calls(), ","); got != "Get,Put,DeletePrefix" {
		t.Fatalf("Calls = %s", got)
	}
}

func TestPresignedURLsAreWellFormed(t *testing.T) {
	m := New()
	put, err := m.PresignPut(context.Background(), "k", "image/png", 0)
	if err != nil || put.Method != "PUT" || !strings.HasSuffix(put.URL, "/k") {
		t.Fatalf("PresignPut = %+v, %v", put, err)
	}
	// The real store signs Content-Type in; the client has to replay it.
	if put.Headers["Content-Type"] != "image/png" {
		t.Fatalf("PresignPut headers = %v, want Content-Type signed in", put.Headers)
	}
	untyped, _ := m.PresignPut(context.Background(), "k", "", 0)
	if len(untyped.Headers) != 0 {
		t.Fatalf("PresignPut without a type signed %v", untyped.Headers)
	}
	get, err := m.PresignGet(context.Background(), "k", 0)
	if err != nil || get.Method != "GET" || !strings.HasSuffix(get.URL, "/k") {
		t.Fatalf("PresignGet = %+v, %v", get, err)
	}
}
