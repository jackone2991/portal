package worker

// The poster end to end (SPEC-01 P0.2; backlog #9): Thumbnailer.Handle over an
// in-memory store and a recording repo, with videos synthesised by ffmpeg's
// lavfi sources — no binary fixtures. Pins the three things the row promises:
// a poster is cut and stored at the right size, an audio-only container is
// skipped, and the skip is non-fatal (Handle returns nil, nothing is written).

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/portal/backend/internal/platform/storage"
)

// memStore is the smallest storage.Storage the worker path touches: Get and
// Put. Everything else is unreachable from Handle and says so.
type memStore struct {
	mu      sync.Mutex
	objects map[string][]byte
	types   map[string]string
}

func newMemStore() *memStore {
	return &memStore{objects: map[string][]byte{}, types: map[string]string{}}
}

func (m *memStore) put(key string, b []byte) { m.mu.Lock(); defer m.mu.Unlock(); m.objects[key] = b }

func (m *memStore) Bucket() string { return "test" }
func (m *memStore) Put(_ context.Context, key string, body io.Reader, contentType string) error {
	b, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key], m.types[key] = b, contentType
	return nil
}
func (m *memStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, fmt.Errorf("memStore: no object %q", key)
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (m *memStore) Exists(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.objects[key]
	return ok, nil
}
func (m *memStore) Size(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return int64(len(m.objects[key])), nil
}
func (m *memStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}
func (m *memStore) DeletePrefix(context.Context, string) error {
	return fmt.Errorf("memStore: DeletePrefix not used by the worker")
}
func (m *memStore) GetRange(context.Context, string, int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("memStore: GetRange not used by the worker")
}
func (m *memStore) GetByteRange(context.Context, string, int64, int64) (io.ReadCloser, error) {
	return nil, fmt.Errorf("memStore: GetByteRange not used by the worker")
}
func (m *memStore) PresignPut(context.Context, string, string, time.Duration) (*storage.PresignedRequest, error) {
	return nil, fmt.Errorf("memStore: PresignPut not used by the worker")
}
func (m *memStore) PresignGet(context.Context, string, time.Duration) (*storage.PresignedRequest, error) {
	return nil, fmt.Errorf("memStore: PresignGet not used by the worker")
}

var _ storage.Storage = (*memStore)(nil)

// recordingRepo keeps every variant row the worker inserts — and every
// MarkFailed, which the poster path must never call: a poster is cosmetics,
// the asset stays ready.
type recordingRepo struct {
	variants []variantCall
	failed   []string
}

type variantCall struct {
	asset   uuid.UUID
	variant string
	key     string
	w, h    int
	size    int64
}

func (r *recordingRepo) MarkReady(context.Context, uuid.UUID, string, *int, *int, *int) error {
	return nil
}
func (r *recordingRepo) MarkFailed(_ context.Context, _ uuid.UUID, msg string) error {
	r.failed = append(r.failed, msg)
	return nil
}
func (r *recordingRepo) MarkImageReady(context.Context, uuid.UUID, int, int) error { return nil }
func (r *recordingRepo) InsertVariant(_ context.Context, assetID uuid.UUID, variant, key string, w, h int, size int64) error {
	r.variants = append(r.variants, variantCall{assetID, variant, key, w, h, size})
	return nil
}
func (r *recordingRepo) LoadAssetMeta(context.Context, uuid.UUID) (AssetMeta, error) {
	return AssetMeta{}, nil
}

var _ Repo = (*recordingRepo)(nil)

// synthesise runs ffmpeg with lavfi sources into a container under dir and
// returns its bytes. The native mpeg4/aac encoders are in every ffmpeg build.
func synthesise(t *testing.T, dir, name string, args ...string) []byte {
	t.Helper()
	out := filepath.Join(dir, name)
	full := append([]string{"-y", "-v", "error"}, args...)
	full = append(full, out)
	if b, err := exec.CommandContext(context.Background(), "ffmpeg", full...).CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg %v: %v: %s", args, err, b)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func posterTask(t *testing.T, id uuid.UUID, key string) *asynq.Task {
	t.Helper()
	task, err := NewThumbnailTask(ThumbnailPayload{AssetID: id.String(), SourceKey: key, OwnerUserID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestPosterFromASynthesisedVideo(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	store, repo := newMemStore(), &recordingRepo{}
	th := NewThumbnailer(store, repo, nil)
	id := uuid.New()

	// Two seconds of the SMPTE-ish test pattern at 1280×720: wider than the
	// poster's 640 cap, so the scale rule has something to do.
	store.put("src/video", synthesise(t, dir, "video.mp4",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=1280x720:rate=10",
		"-c:v", "mpeg4", "-q:v", "5", "-pix_fmt", "yuv420p"))

	if err := th.Handle(context.Background(), posterTask(t, id, "src/video")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(repo.variants) != 1 {
		t.Fatalf("variant rows = %d, want exactly one poster", len(repo.variants))
	}
	v := repo.variants[0]
	key := fmt.Sprintf("variants/%s/poster.webp", id)
	if v.asset != id || v.variant != "poster" || v.key != key {
		t.Fatalf("variant row = %+v, want poster at %s", v, key)
	}
	if v.w != posterMaxWidth || v.h != 360 || v.size <= 0 {
		t.Fatalf("poster = %d×%d, %d bytes; want %d×360 and some bytes", v.w, v.h, v.size, posterMaxWidth)
	}
	if ok, _ := store.Exists(context.Background(), key); !ok {
		t.Fatalf("poster object %s was not uploaded", key)
	}
	if ct := store.types[key]; ct != "image/webp" {
		t.Fatalf("poster uploaded as %q, want image/webp", ct)
	}
	if len(repo.failed) != 0 {
		t.Fatalf("MarkFailed called on the happy path: %v", repo.failed)
	}

	// A clip narrower than the cap is never upscaled.
	small := uuid.New()
	store.put("src/small", synthesise(t, dir, "small.mp4",
		"-f", "lavfi", "-i", "testsrc=duration=1:size=320x240:rate=10", "-c:v", "mpeg4", "-q:v", "5"))
	if err := th.Handle(context.Background(), posterTask(t, small, "src/small")); err != nil {
		t.Fatalf("Handle (small): %v", err)
	}
	if last := repo.variants[len(repo.variants)-1]; last.asset != small || last.w != 320 || last.h != 240 {
		t.Fatalf("small poster = %+v, want 320×240 (never upscaled)", last)
	}
}

// An audio-only container (an .mp3 renamed .mp4) has no frame to cut: the
// poster is skipped, and the skip is non-fatal — Handle returns nil so Asynq
// does not retry a job that can never succeed, and nothing is written.
func TestAudioOnlyContainerSkipsThePosterWithoutFailing(t *testing.T) {
	requireFFmpeg(t)
	dir := t.TempDir()
	store, repo := newMemStore(), &recordingRepo{}
	th := NewThumbnailer(store, repo, nil)
	id := uuid.New()

	store.put("src/audio", synthesise(t, dir, "audio.mp4",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-c:a", "aac"))

	if err := th.Handle(context.Background(), posterTask(t, id, "src/audio")); err != nil {
		t.Fatalf("Handle must swallow the skip, got %v", err)
	}
	if len(repo.variants) != 0 {
		t.Fatalf("a poster row was written for an audio-only container: %+v", repo.variants)
	}
	if len(repo.failed) != 0 {
		t.Fatalf("the skip marked the asset failed: %v — a poster is cosmetics, the asset stays ready", repo.failed)
	}
	if ok, _ := store.Exists(context.Background(), fmt.Sprintf("variants/%s/poster.webp", id)); ok {
		t.Fatal("a poster object was uploaded for an audio-only container")
	}
}

// A missing source is the same story: warn, return nil, write nothing.
func TestMissingSourceIsNonFatal(t *testing.T) {
	store, repo := newMemStore(), &recordingRepo{}
	th := NewThumbnailer(store, repo, nil)
	if err := th.Handle(context.Background(), posterTask(t, uuid.New(), "src/nope")); err != nil {
		t.Fatalf("Handle must swallow a download failure, got %v", err)
	}
	if len(repo.variants) != 0 || len(repo.failed) != 0 {
		t.Fatalf("a download failure wrote something: variants %+v, failed %v", repo.variants, repo.failed)
	}
}
