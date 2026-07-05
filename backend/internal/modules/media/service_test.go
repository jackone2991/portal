package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/portal/backend/internal/modules/media/worker"
	"github.com/portal/backend/internal/platform/storage"
)

// ── fakes ───────────────────────────────────────────────────────────

type fakeRepo struct{ m map[uuid.UUID]Asset }

func (r *fakeRepo) CreateAsset(_ context.Context, in CreateAssetInput) (Asset, error) {
	a := Asset{
		ID: in.ID, OwnerID: in.OwnerID, Kind: in.Kind, Status: StatusUploading,
		SourceKey: in.SourceKey, MimeType: in.MimeType, SizeBytes: in.SizeBytes, CreatedAt: time.Now(),
	}
	r.m[in.ID] = a
	return a, nil
}
func (r *fakeRepo) GetAsset(_ context.Context, id uuid.UUID) (Asset, error) {
	a, ok := r.m[id]
	if !ok {
		return Asset{}, ErrNotFound
	}
	return a, nil
}
func (r *fakeRepo) ListByOwner(_ context.Context, owner uuid.UUID, _, _ int) ([]Asset, error) {
	var out []Asset
	for _, a := range r.m {
		if a.OwnerID == owner {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *fakeRepo) MarkProcessing(_ context.Context, id uuid.UUID) error {
	a := r.m[id]
	a.Status = StatusProcessing
	r.m[id] = a
	return nil
}
func (r *fakeRepo) MarkReady(_ context.Context, id uuid.UUID, prefix string, dur, w, h *int) error {
	a := r.m[id]
	a.Status, a.OutputPrefix, a.DurationMs, a.Width, a.Height = StatusReady, prefix, dur, w, h
	r.m[id] = a
	return nil
}
func (r *fakeRepo) MarkFailed(_ context.Context, id uuid.UUID, msg string) error {
	a := r.m[id]
	a.Status, a.ErrorMessage = StatusFailed, msg
	r.m[id] = a
	return nil
}

type fakeStore struct{ obj map[string][]byte }

func (s *fakeStore) Bucket() string { return "test" }
func (s *fakeStore) PresignPut(_ context.Context, key, _ string, ttl time.Duration) (*storage.PresignedRequest, error) {
	return &storage.PresignedRequest{URL: "http://store/" + key, Method: "PUT", Expires: time.Now().Add(ttl)}, nil
}
func (s *fakeStore) PresignGet(_ context.Context, key string, ttl time.Duration) (*storage.PresignedRequest, error) {
	return &storage.PresignedRequest{URL: "http://store/" + key, Method: "GET", Expires: time.Now().Add(ttl)}, nil
}
func (s *fakeStore) Put(_ context.Context, key string, body io.Reader, _ string) error {
	b, _ := io.ReadAll(body)
	s.obj[key] = b
	return nil
}
func (s *fakeStore) Get(_ context.Context, key string) (io.ReadCloser, error) {
	b, ok := s.obj[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *fakeStore) Delete(_ context.Context, key string) error { delete(s.obj, key); return nil }
func (s *fakeStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.obj[key]
	return ok, nil
}

type fakeEnqueuer struct{ tasks []*asynq.Task }

func (e *fakeEnqueuer) Enqueue(t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	e.tasks = append(e.tasks, t)
	return &asynq.TaskInfo{}, nil
}

func newSvc() (*Service, *fakeRepo, *fakeStore, *fakeEnqueuer) {
	repo := &fakeRepo{m: map[uuid.UUID]Asset{}}
	store := &fakeStore{obj: map[string][]byte{}}
	enq := &fakeEnqueuer{}
	return &Service{store: store, repo: repo, enqueue: enq, baseURL: "https://api.test", uploadTTL: time.Minute}, repo, store, enq
}

// ── tests ───────────────────────────────────────────────────────────

func TestCreateUploadSession(t *testing.T) {
	svc, repo, _, _ := newSvc()
	owner := uuid.New()

	sess, err := svc.CreateUploadSession(context.Background(), owner, "clip.MP4", "video/mp4", 123)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Asset.Status != StatusUploading || sess.Asset.OwnerID != owner {
		t.Fatalf("asset = %+v", sess.Asset)
	}
	if _, ok := repo.m[sess.Asset.ID]; !ok {
		t.Fatal("asset not persisted")
	}
	if !strings.HasSuffix(sess.Asset.SourceKey, ".mp4") { // ext normalised, lowercased
		t.Fatalf("source key = %q", sess.Asset.SourceKey)
	}
	if sess.URL == "" || sess.Method != "PUT" {
		t.Fatalf("presign = %+v", sess)
	}
}

func TestCompleteUpload(t *testing.T) {
	svc, repo, store, enq := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	sess, _ := svc.CreateUploadSession(ctx, owner, "c.mp4", "video/mp4", 1)
	id := sess.Asset.ID

	// wrong owner → forbidden
	if err := svc.CompleteUpload(ctx, uuid.New(), id); !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner mismatch = %v, want ErrForbidden", err)
	}
	// original not uploaded yet → not ready
	if err := svc.CompleteUpload(ctx, owner, id); !errors.Is(err, ErrNotReady) {
		t.Fatalf("missing source = %v, want ErrNotReady", err)
	}
	// upload landed → completes, marks processing, enqueues one transcode task
	store.obj[sess.Asset.SourceKey] = []byte("mp4")
	if err := svc.CompleteUpload(ctx, owner, id); err != nil {
		t.Fatal(err)
	}
	if repo.m[id].Status != StatusProcessing {
		t.Fatalf("status = %v, want processing", repo.m[id].Status)
	}
	if len(enq.tasks) != 1 || enq.tasks[0].Type() != worker.TaskTypeTranscode {
		t.Fatalf("enqueued = %v", enq.tasks)
	}
}

func TestHLSObjectSafety(t *testing.T) {
	svc, repo, store, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	ready := Asset{ID: uuid.New(), OwnerID: owner, Status: StatusReady, OutputPrefix: "hls/x"}
	repo.m[ready.ID] = ready
	store.obj["hls/x/index.m3u8"] = []byte("#EXTM3U")

	// valid file streams back with the right content-type
	rc, ct, err := svc.HLSObject(ctx, ready.ID, "index.m3u8")
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if ct != "application/vnd.apple.mpegurl" {
		t.Fatalf("content-type = %q", ct)
	}

	// path traversal cannot escape the asset's prefix (resolves to a miss, not /etc/passwd)
	if _, _, err := svc.HLSObject(ctx, ready.ID, "../../etc/passwd"); err == nil {
		t.Fatal("expected traversal to fail")
	}

	// not-ready asset is not served
	proc := Asset{ID: uuid.New(), OwnerID: owner, Status: StatusProcessing}
	repo.m[proc.ID] = proc
	if _, _, err := svc.HLSObject(ctx, proc.ID, "index.m3u8"); !errors.Is(err, ErrNotReady) {
		t.Fatalf("not-ready = %v, want ErrNotReady", err)
	}
}

func TestGetOwnerScoped(t *testing.T) {
	svc, repo, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	a := Asset{ID: uuid.New(), OwnerID: owner, Status: StatusReady, OutputPrefix: "hls/x"}
	repo.m[a.ID] = a

	got, hls, err := svc.Get(ctx, owner, a.ID)
	if err != nil || got.ID != a.ID {
		t.Fatalf("Get(owner) = %+v, %v", got, err)
	}
	if !strings.HasSuffix(hls, "/hls/index.m3u8") {
		t.Fatalf("hls url = %q", hls)
	}
	if _, _, err := svc.Get(ctx, uuid.New(), a.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Get(other) = %v, want ErrForbidden", err)
	}
}
