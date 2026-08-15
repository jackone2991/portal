package media

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	"github.com/portal/backend/internal/modules/media/worker"
	"github.com/portal/backend/internal/platform/storage"
)

// ── fakes ───────────────────────────────────────────────────────────

type progressRow struct {
	positionMs  int64
	completedAt *time.Time
	updatedAt   time.Time
}

type fakeRepo struct {
	m        map[uuid.UUID]Asset
	variants map[uuid.UUID]map[string]Variant
	progress map[string]progressRow // key: "<user>|<asset>"
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		m:        map[uuid.UUID]Asset{},
		variants: map[uuid.UUID]map[string]Variant{},
		progress: map[string]progressRow{},
	}
}

func progressKey(user, asset uuid.UUID) string { return user.String() + "|" + asset.String() }

func (r *fakeRepo) CreateAsset(_ context.Context, in CreateAssetInput) (Asset, error) {
	a := Asset{
		ID: in.ID, OwnerID: in.OwnerID, Kind: in.Kind, Status: StatusUploading,
		SourceKey: in.SourceKey, MimeType: in.MimeType, SizeBytes: in.SizeBytes,
		Title: in.Title, OriginalFilename: in.OriginalFilename, Origin: in.Origin,
		CreatedAt: time.Now(),
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
func (r *fakeRepo) GetAssetStatuses(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		if a, ok := r.m[id]; ok {
			out[id] = string(a.Status)
		}
	}
	return out, nil
}
func (r *fakeRepo) GetAssetOwner(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	a, ok := r.m[id]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return a.OwnerID, nil
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
func (r *fakeRepo) ListByOwnerCursor(_ context.Context, in ListCursorInput) ([]Asset, error) {
	var out []Asset
	for _, a := range r.m {
		if a.OwnerID != in.OwnerID || a.Status == StatusDeleting {
			continue
		}
		if in.Kind != "" && a.Kind != in.Kind {
			continue
		}
		if len(in.Statuses) > 0 && !containsStr(in.Statuses, string(a.Status)) {
			continue
		}
		out = append(out, a)
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	if !in.CursorAt.IsZero() {
		filtered := out[:0]
		for _, a := range out {
			if a.CreatedAt.Before(in.CursorAt) ||
				(a.CreatedAt.Equal(in.CursorAt) && a.ID.String() < in.CursorID.String()) {
				filtered = append(filtered, a)
			}
		}
		out = filtered
	}
	if in.Limit > 0 && len(out) > in.Limit {
		out = out[:in.Limit]
	}
	return out, nil
}
func (r *fakeRepo) ListForPurge(_ context.Context) ([]Asset, error) {
	var out []Asset
	for _, a := range r.m {
		if a.Status == StatusDeleting {
			out = append(out, a)
		}
	}
	return out, nil
}
func (r *fakeRepo) ListAbandonedUploads(_ context.Context) ([]Asset, error) {
	var out []Asset
	for _, a := range r.m {
		if a.Status == StatusUploading {
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
func (r *fakeRepo) SetStatusDeleting(_ context.Context, id uuid.UUID) error {
	a := r.m[id]
	a.Status = StatusDeleting
	r.m[id] = a
	return nil
}
func (r *fakeRepo) DeleteAsset(_ context.Context, id uuid.UUID) error {
	delete(r.m, id)
	return nil
}
func (r *fakeRepo) MarkReady(_ context.Context, id uuid.UUID, prefix string, dur, w, h *int) error {
	a := r.m[id]
	a.Status, a.OutputPrefix, a.DurationMs, a.Width, a.Height = StatusReady, prefix, dur, w, h
	r.m[id] = a
	return nil
}
func (r *fakeRepo) MarkImageReady(_ context.Context, id uuid.UUID, w, h int) error {
	a := r.m[id]
	a.Status, a.Width, a.Height = StatusReady, &w, &h
	r.m[id] = a
	return nil
}
func (r *fakeRepo) MarkFailed(_ context.Context, id uuid.UUID, msg string) error {
	a := r.m[id]
	a.Status, a.ErrorMessage = StatusFailed, msg
	r.m[id] = a
	return nil
}
func (r *fakeRepo) LoadAssetMeta(_ context.Context, id uuid.UUID) (worker.AssetMeta, error) {
	a, ok := r.m[id]
	if !ok {
		return worker.AssetMeta{}, ErrNotFound
	}
	return worker.AssetMeta{OwnerID: a.OwnerID, Kind: a.Kind, Title: a.Title, Origin: a.Origin}, nil
}
func (r *fakeRepo) InsertVariant(_ context.Context, assetID uuid.UUID, variant, key string, w, h int, size int64) error {
	if r.variants[assetID] == nil {
		r.variants[assetID] = map[string]Variant{}
	}
	r.variants[assetID][variant] = Variant{AssetID: assetID, Variant: variant, StorageKey: key, Width: w, Height: h, SizeBytes: size}
	return nil
}
func (r *fakeRepo) ListVariants(_ context.Context, assetID uuid.UUID) ([]Variant, error) {
	var out []Variant
	for _, v := range r.variants[assetID] {
		out = append(out, v)
	}
	return out, nil
}
func (r *fakeRepo) GetVariant(_ context.Context, assetID uuid.UUID, variant string) (Variant, error) {
	v, ok := r.variants[assetID][variant]
	if !ok {
		return Variant{}, ErrNotFound
	}
	return v, nil
}
func (r *fakeRepo) DeleteVariants(_ context.Context, assetID uuid.UUID) error {
	delete(r.variants, assetID)
	return nil
}

func (r *fakeRepo) UpsertPlaybackProgress(_ context.Context, ownerID, assetID uuid.UUID, positionMs int64, completedAt *time.Time) error {
	key := progressKey(ownerID, assetID)
	// completed_at latch: mirror the SQL COALESCE(existing, new) — once set, never cleared.
	ca := r.progress[key].completedAt
	if ca == nil {
		ca = completedAt
	}
	r.progress[key] = progressRow{positionMs: positionMs, completedAt: ca, updatedAt: time.Now()}
	return nil
}

func (r *fakeRepo) GetPlaybackProgress(_ context.Context, ownerID, assetID uuid.UUID) (int64, *time.Time, time.Time, error) {
	p, ok := r.progress[progressKey(ownerID, assetID)]
	if !ok {
		return 0, nil, time.Time{}, ErrNotFound
	}
	return p.positionMs, p.completedAt, p.updatedAt, nil
}

// GetContinueItems mirrors query/media_progress.sql's inclusion predicate
// (≥30s, <95%, non-NULL duration) so service-level tests can assert it.
func (r *fakeRepo) GetContinueItems(_ context.Context, userID uuid.UUID, limit int) ([]mediaapi.ContinueItem, error) {
	var out []mediaapi.ContinueItem
	for key, p := range r.progress {
		parts := strings.SplitN(key, "|", 2)
		if parts[0] != userID.String() {
			continue
		}
		assetID, err := uuid.Parse(parts[1])
		if err != nil {
			continue
		}
		a, ok := r.m[assetID]
		if !ok || a.DurationMs == nil || *a.DurationMs == 0 {
			continue
		}
		pct := int(float64(p.positionMs) / float64(*a.DurationMs) * 100.0)
		if p.positionMs < 30000 || pct >= 95 {
			continue
		}
		out = append(out, mediaapi.ContinueItem{
			Module:      "media",
			RefID:       assetID,
			Title:       a.Title,
			ProgressPct: pct,
			Href:        "/library/media/" + assetID.String(),
			UpdatedAt:   p.updatedAt,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

type fakeStore struct {
	obj   map[string][]byte
	sizes map[string]int64 // optional Size override (e.g. simulate a >50 MB object)
}

func newFakeStore() *fakeStore {
	return &fakeStore{obj: map[string][]byte{}, sizes: map[string]int64{}}
}

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
func (s *fakeStore) GetRange(_ context.Context, key string, n int64) (io.ReadCloser, error) {
	b, ok := s.obj[key]
	if !ok {
		return nil, storage.ErrNotFound
	}
	if int64(len(b)) > n {
		b = b[:n]
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}
func (s *fakeStore) Size(_ context.Context, key string) (int64, error) {
	if sz, ok := s.sizes[key]; ok {
		return sz, nil
	}
	b, ok := s.obj[key]
	if !ok {
		return 0, storage.ErrNotFound
	}
	return int64(len(b)), nil
}
func (s *fakeStore) Delete(_ context.Context, key string) error { delete(s.obj, key); return nil }
func (s *fakeStore) DeletePrefix(_ context.Context, prefix string) error {
	for k := range s.obj {
		if strings.HasPrefix(k, prefix) {
			delete(s.obj, k)
		}
	}
	return nil
}
func (s *fakeStore) Exists(_ context.Context, key string) (bool, error) {
	_, ok := s.obj[key]
	return ok, nil
}

type fakeEnqueuer struct{ tasks []*asynq.Task }

func (e *fakeEnqueuer) Enqueue(t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	e.tasks = append(e.tasks, t)
	return &asynq.TaskInfo{}, nil
}

type fakePublisher struct{ events []string }

func (p *fakePublisher) Publish(_ context.Context, name string, _ any) error {
	p.events = append(p.events, name)
	return nil
}

func newSvc() (*Service, *fakeRepo, *fakeStore, *fakeEnqueuer, *fakePublisher) {
	repo := newFakeRepo()
	store := newFakeStore()
	enq := &fakeEnqueuer{}
	pub := &fakePublisher{}
	svc := &Service{store: store, repo: repo, enqueue: enq, events: pub, baseURL: "https://api.test", uploadTTL: time.Minute}
	return svc, repo, store, enq, pub
}

// magic-byte fixtures.
var (
	jpegHeader = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	pngHeader  = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0, 0, 0, 0}
	webpHeader = append([]byte("RIFF\x00\x00\x00\x00"), []byte("WEBPVP8 ")...)
	// ISO-BMFF ftyp box, major brand "heic".
	heicHeader = []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'h', 'e', 'i', 'c', 0, 0, 0, 0, 'm', 'i', 'f', '1'}
	// major brand "mif1", HEIC only via compatible brand.
	heifCompatHeader = []byte{0, 0, 0, 0x18, 'f', 't', 'y', 'p', 'm', 'i', 'f', '1', 0, 0, 0, 0, 'h', 'e', 'i', 'c'}
)

// ── tests ───────────────────────────────────────────────────────────

func TestCreateUploadSession(t *testing.T) {
	svc, repo, _, _, _ := newSvc()
	owner := uuid.New()

	sess, err := svc.CreateUploadSession(context.Background(), owner, "clip.MP4", "video/mp4", 123)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Asset.Status != StatusUploading || sess.Asset.OwnerID != owner {
		t.Fatalf("asset = %+v", sess.Asset)
	}
	if sess.Asset.Kind != "video" {
		t.Fatalf("kind = %q, want video", sess.Asset.Kind)
	}
	if sess.Asset.Origin != "upload" || sess.Asset.OriginalFilename != "clip.MP4" {
		t.Fatalf("origin/filename = %+v", sess.Asset)
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

func TestCreateUploadSessionImageKind(t *testing.T) {
	svc, _, _, _, _ := newSvc()
	sess, err := svc.CreateUploadSession(context.Background(), uuid.New(), "pic.png", "image/png", 10)
	if err != nil {
		t.Fatal(err)
	}
	if sess.Asset.Kind != "image" {
		t.Fatalf("kind = %q, want image", sess.Asset.Kind)
	}
}

func TestCompleteUpload(t *testing.T) {
	svc, repo, store, enq, _ := newSvc()
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

func TestCompleteUploadImageAccepted(t *testing.T) {
	svc, repo, store, enq, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	sess, _ := svc.CreateUploadSession(ctx, owner, "p.jpg", "image/jpeg", 1)
	id := sess.Asset.ID
	store.obj[sess.Asset.SourceKey] = jpegHeader

	if err := svc.CompleteUpload(ctx, owner, id); err != nil {
		t.Fatalf("complete image: %v", err)
	}
	if repo.m[id].Status != StatusProcessing {
		t.Fatalf("status = %v, want processing", repo.m[id].Status)
	}
	if len(enq.tasks) != 1 || enq.tasks[0].Type() != worker.TaskTypeProcessImage {
		t.Fatalf("enqueued = %v, want media:process_image", enq.tasks)
	}
}

func TestCompleteUploadHEICRejected(t *testing.T) {
	svc, repo, store, enq, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	sess, _ := svc.CreateUploadSession(ctx, owner, "IMG.heic", "image/heic", 1)
	id := sess.Asset.ID
	store.obj[sess.Asset.SourceKey] = heicHeader

	err := svc.CompleteUpload(ctx, owner, id)
	var ufe *UnsupportedFormatError
	if !errors.As(err, &ufe) || !ufe.HEIC {
		t.Fatalf("err = %v, want HEIC UnsupportedFormatError", err)
	}
	if repo.m[id].Status != StatusFailed {
		t.Fatalf("status = %v, want failed", repo.m[id].Status)
	}
	if _, ok := store.obj[sess.Asset.SourceKey]; ok {
		t.Fatal("uploaded object should be deleted on rejection")
	}
	if len(enq.tasks) != 0 {
		t.Fatalf("no task should be enqueued, got %v", enq.tasks)
	}
}

func TestCompleteUploadUnknownFormatRejected(t *testing.T) {
	svc, repo, store, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	sess, _ := svc.CreateUploadSession(ctx, owner, "x.png", "image/png", 1)
	store.obj[sess.Asset.SourceKey] = []byte("not an image at all")

	var ufe *UnsupportedFormatError
	if err := svc.CompleteUpload(ctx, owner, sess.Asset.ID); !errors.As(err, &ufe) || ufe.HEIC {
		t.Fatalf("err = %v, want non-HEIC UnsupportedFormatError", err)
	}
	if repo.m[sess.Asset.ID].Status != StatusFailed {
		t.Fatal("asset should be failed")
	}
}

func TestCompleteUploadTooLarge(t *testing.T) {
	svc, repo, store, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	sess, _ := svc.CreateUploadSession(ctx, owner, "big.jpg", "image/jpeg", 1)
	store.obj[sess.Asset.SourceKey] = jpegHeader
	store.sizes[sess.Asset.SourceKey] = maxUploadBytes + 1 // simulate a >50 MB object

	if err := svc.CompleteUpload(ctx, owner, sess.Asset.ID); !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v, want ErrFileTooLarge", err)
	}
	if repo.m[sess.Asset.ID].Status != StatusFailed {
		t.Fatal("asset should be failed")
	}
	if _, ok := store.obj[sess.Asset.SourceKey]; ok {
		t.Fatal("oversized object should be deleted")
	}
}

func TestSniffImageType(t *testing.T) {
	cases := []struct {
		name string
		b    []byte
		want string
		ok   bool
	}{
		{"jpeg", jpegHeader, "image/jpeg", true},
		{"png", pngHeader, "image/png", true},
		{"webp", webpHeader, "image/webp", true},
		{"heic", heicHeader, "", false},
		{"garbage", []byte("hello world padding padding"), "", false},
		{"empty", nil, "", false},
	}
	for _, c := range cases {
		got, ok := sniffImageType(c.b)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: sniffImageType = (%q,%v), want (%q,%v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestIsHEIC(t *testing.T) {
	if !isHEIC(heicHeader) {
		t.Error("major-brand heic not detected")
	}
	if !isHEIC(heifCompatHeader) {
		t.Error("compatible-brand heic not detected")
	}
	if isHEIC(jpegHeader) || isHEIC(pngHeader) || isHEIC(webpHeader) {
		t.Error("false positive on a non-HEIC header")
	}
}

func TestCursorRoundTrip(t *testing.T) {
	a := Asset{ID: uuid.New(), CreatedAt: time.Now().UTC().Truncate(time.Nanosecond)}
	at, id, err := decodeCursor(encodeCursor(a))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !at.Equal(a.CreatedAt) || id != a.ID {
		t.Fatalf("round-trip = (%v,%v), want (%v,%v)", at, id, a.CreatedAt, a.ID)
	}
	if _, _, err := decodeCursor("!!! not base64 !!!"); err == nil {
		t.Fatal("want error for malformed cursor")
	}
}

func TestExpandStatuses(t *testing.T) {
	if got := expandStatuses(""); got != nil {
		t.Errorf(`"" → %v, want nil`, got)
	}
	if got := expandStatuses("all"); got != nil {
		t.Errorf(`"all" → %v, want nil`, got)
	}
	got := expandStatuses("processing")
	if len(got) != 2 || !containsStr(got, "processing") || !containsStr(got, "uploading") {
		t.Errorf(`"processing" → %v, want {processing, uploading}`, got)
	}
	if got := expandStatuses("ready"); len(got) != 1 || got[0] != "ready" {
		t.Errorf(`"ready" → %v`, got)
	}
}

func TestListPaginates(t *testing.T) {
	svc, repo, _, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	base := time.Now().UTC()
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.m[id] = Asset{ID: id, OwnerID: owner, Kind: "image", Status: StatusReady, CreatedAt: base.Add(time.Duration(i) * time.Second)}
	}

	page1, err := svc.List(ctx, owner, ListOptions{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1.Assets) != 2 || page1.NextCursor == "" {
		t.Fatalf("page1 = %d assets, cursor %q", len(page1.Assets), page1.NextCursor)
	}
	page2, err := svc.List(ctx, owner, ListOptions{Limit: 2, Cursor: page1.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2.Assets) != 1 || page2.NextCursor != "" {
		t.Fatalf("page2 = %d assets, cursor %q", len(page2.Assets), page2.NextCursor)
	}
}

func TestDeleteAsset(t *testing.T) {
	svc, repo, store, _, pub := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	id := uuid.New()
	repo.m[id] = Asset{ID: id, OwnerID: owner, Kind: "image", Status: StatusReady, SourceKey: "uploads/" + id.String() + "/original.jpg"}
	_ = repo.InsertVariant(ctx, id, "thumb", "variants/"+id.String()+"/thumb.webp", 320, 200, 10)
	store.obj["uploads/"+id.String()+"/original.jpg"] = []byte("orig")
	store.obj["variants/"+id.String()+"/thumb.webp"] = []byte("thumb")

	if err := svc.DeleteAsset(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := repo.m[id]; ok {
		t.Fatal("asset row should be gone")
	}
	if len(store.obj) != 0 {
		t.Fatalf("storage objects should be gone, have %v", store.obj)
	}
	if !containsStr(pub.events, EventAssetDeleted) {
		t.Fatalf("expected %s event, got %v", EventAssetDeleted, pub.events)
	}

	// idempotent: deleting again is ErrNotFound, never a 500
	if err := svc.DeleteAsset(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete = %v, want ErrNotFound", err)
	}
}

func TestPurgeOrphans(t *testing.T) {
	svc, repo, store, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	// a tombstoned asset → purged
	del := uuid.New()
	repo.m[del] = Asset{ID: del, OwnerID: owner, Status: StatusDeleting, SourceKey: "uploads/" + del.String() + "/original.jpg"}
	store.obj["uploads/"+del.String()+"/original.jpg"] = []byte("x")

	// an abandoned upload → marked failed
	ab := uuid.New()
	repo.m[ab] = Asset{ID: ab, OwnerID: owner, Status: StatusUploading}

	if err := svc.PurgeOrphans(ctx); err != nil {
		t.Fatalf("purge: %v", err)
	}
	if _, ok := repo.m[del]; ok {
		t.Fatal("tombstoned asset should be purged")
	}
	if len(store.obj) != 0 {
		t.Fatalf("objects should be purged, have %v", store.obj)
	}
	if repo.m[ab].Status != StatusFailed || repo.m[ab].ErrorMessage != "upload abandoned" {
		t.Fatalf("abandoned upload = %+v, want failed/upload abandoned", repo.m[ab])
	}
}

func TestServeVariant(t *testing.T) {
	svc, repo, store, _, _ := newSvc()
	ctx := context.Background()
	id := uuid.New()
	repo.m[id] = Asset{ID: id, Kind: "image", Status: StatusReady}
	_ = repo.InsertVariant(ctx, id, "thumb", "variants/"+id.String()+"/thumb.webp", 320, 200, 10)
	store.obj["variants/"+id.String()+"/thumb.webp"] = []byte("webpbytes")

	rc, ct, err := svc.ServeVariant(ctx, id, "thumb")
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if ct != "image/webp" {
		t.Fatalf("content-type = %q", ct)
	}
	// unknown variant name → not found
	if _, _, err := svc.ServeVariant(ctx, id, "bogus"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("bogus variant = %v, want ErrNotFound", err)
	}
	// missing variant → not found
	if _, _, err := svc.ServeVariant(ctx, id, "medium"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing variant = %v, want ErrNotFound", err)
	}
}

func TestDownloadOriginal(t *testing.T) {
	svc, repo, store, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	id := uuid.New()
	key := "uploads/" + id.String() + "/original.jpg"
	repo.m[id] = Asset{ID: id, OwnerID: owner, Kind: "image", Status: StatusReady, SourceKey: key, MimeType: "image/jpeg", OriginalFilename: "vacation.jpg"}
	store.obj[key] = []byte("JPEGDATA")

	rc, ct, filename, err := svc.DownloadOriginal(ctx, owner, id)
	if err != nil {
		t.Fatal(err)
	}
	rc.Close()
	if ct != "image/jpeg" || filename != "vacation.jpg" {
		t.Fatalf("ct=%q filename=%q", ct, filename)
	}
	// non-owner → forbidden
	if _, _, _, err := svc.DownloadOriginal(ctx, uuid.New(), id); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-owner = %v, want ErrForbidden", err)
	}
	// still uploading → not ready
	up := uuid.New()
	repo.m[up] = Asset{ID: up, OwnerID: owner, Status: StatusUploading, SourceKey: "k"}
	if _, _, _, err := svc.DownloadOriginal(ctx, owner, up); !errors.Is(err, ErrNotReady) {
		t.Fatalf("uploading = %v, want ErrNotReady", err)
	}
}

func TestHLSObjectSafety(t *testing.T) {
	svc, repo, store, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	ready := Asset{ID: uuid.New(), OwnerID: owner, Kind: "video", Status: StatusReady, OutputPrefix: "hls/x"}
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
	svc, repo, _, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	a := Asset{ID: uuid.New(), OwnerID: owner, Kind: "video", Status: StatusReady, OutputPrefix: "hls/x"}
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

func countStr(ss []string, want string) int {
	n := 0
	for _, s := range ss {
		if s == want {
			n++
		}
	}
	return n
}

// TestPutProgressGuards covers SPEC-07 P0.2: owner/kind/deleting 404s and the
// clamp-to-duration behaviour (a position past the end is clamped, never a 500).
func TestPutProgressGuards(t *testing.T) {
	svc, repo, _, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()

	dur := 60000
	vid := uuid.New()
	repo.m[vid] = Asset{ID: vid, OwnerID: owner, Kind: "video", Status: StatusReady, DurationMs: &dur}

	// non-owner → forbidden (no permission-based bypass — P0.2 AC)
	if err := svc.PutProgress(ctx, uuid.New(), vid, 1000); !errors.Is(err, ErrForbidden) {
		t.Fatalf("non-owner = %v, want ErrForbidden", err)
	}
	// non-video → not playable
	img := uuid.New()
	repo.m[img] = Asset{ID: img, OwnerID: owner, Kind: "image", Status: StatusReady}
	if err := svc.PutProgress(ctx, owner, img, 1000); !errors.Is(err, ErrNotPlayable) {
		t.Fatalf("image = %v, want ErrNotPlayable", err)
	}
	// deleting → not found
	del := uuid.New()
	repo.m[del] = Asset{ID: del, OwnerID: owner, Kind: "video", Status: StatusDeleting, DurationMs: &dur}
	if err := svc.PutProgress(ctx, owner, del, 1000); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting = %v, want ErrNotFound", err)
	}
	// position beyond duration → clamped to duration, no error
	if err := svc.PutProgress(ctx, owner, vid, 999999); err != nil {
		t.Fatalf("clamp put: %v", err)
	}
	pos, pct, _, _, err := svc.GetProgress(ctx, owner, vid)
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if pos != int64(dur) {
		t.Fatalf("position = %d, want clamped to %d", pos, dur)
	}
	if pct == nil || *pct != 100 {
		t.Fatalf("pct = %v, want 100", pct)
	}
}

// TestPutProgressCompletionLatch covers P1.5: the completion event fires exactly
// once, on the first NULL→set crossing of ≥95%, never again.
func TestPutProgressCompletionLatch(t *testing.T) {
	svc, repo, _, _, pub := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	dur := 100000
	vid := uuid.New()
	repo.m[vid] = Asset{ID: vid, OwnerID: owner, Kind: "video", Status: StatusReady, DurationMs: &dur, Title: "Clip"}

	// below 95% → no completion event
	if err := svc.PutProgress(ctx, owner, vid, 50000); err != nil {
		t.Fatal(err)
	}
	if n := countStr(pub.events, "media:playback_completed"); n != 0 {
		t.Fatalf("premature completion events = %d, want 0", n)
	}
	// cross 95% → exactly one event, completed_at latched
	if err := svc.PutProgress(ctx, owner, vid, 96000); err != nil {
		t.Fatal(err)
	}
	if n := countStr(pub.events, "media:playback_completed"); n != 1 {
		t.Fatalf("completion events = %d, want 1", n)
	}
	if _, completedAt, _, err := repo.GetPlaybackProgress(ctx, owner, vid); err != nil || completedAt == nil {
		t.Fatalf("completed_at not latched: %v (err %v)", completedAt, err)
	}
	// re-cross 95% → no second emit
	if err := svc.PutProgress(ctx, owner, vid, 99000); err != nil {
		t.Fatal(err)
	}
	if n := countStr(pub.events, "media:playback_completed"); n != 1 {
		t.Fatalf("re-crossing emitted again: events = %d, want 1", n)
	}
}

// TestGetProgressNoRow: GetProgress on a video with no saved row is ErrNotFound
// (the read path the player treats as "start at 0").
func TestGetProgressNoRow(t *testing.T) {
	svc, repo, _, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	dur := 100000
	vid := uuid.New()
	repo.m[vid] = Asset{ID: vid, OwnerID: owner, Kind: "video", Status: StatusReady, DurationMs: &dur}

	if _, _, _, _, err := svc.GetProgress(ctx, owner, vid); !errors.Is(err, ErrNotFound) {
		t.Fatalf("no row = %v, want ErrNotFound", err)
	}
}

// TestContinueItemsPredicate covers P0.3: only items with ≥30s watched and <95%
// complete appear; too-early, near-complete and NULL-duration items are excluded.
func TestContinueItemsPredicate(t *testing.T) {
	svc, repo, _, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	dur := 100000

	mk := func(nullDur bool) uuid.UUID {
		id := uuid.New()
		a := Asset{ID: id, OwnerID: owner, Kind: "video", Status: StatusReady, Title: "T"}
		if !nullDur {
			a.DurationMs = &dur
		}
		repo.m[id] = a
		return id
	}

	inProgress := mk(false) // 50% → included
	tooEarly := mk(false)   // 20s (<30s) → excluded
	almostDone := mk(false) // 97% → excluded
	nullDur := mk(true)     // no duration → excluded

	_ = svc.PutProgress(ctx, owner, inProgress, 50000)
	_ = svc.PutProgress(ctx, owner, tooEarly, 20000)
	_ = svc.PutProgress(ctx, owner, almostDone, 97000)
	_ = svc.PutProgress(ctx, owner, nullDur, 60000)

	items, err := svc.ContinueItems(ctx, owner, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].RefID != inProgress {
		t.Fatalf("continue items = %+v, want only the in-progress one", items)
	}
	if items[0].ProgressPct != 50 || items[0].Href != "/library/media/"+inProgress.String() {
		t.Fatalf("item shape = %+v", items[0])
	}
	// limit ≤ 0 defaults rather than returning an empty page
	if got, _ := svc.ContinueItems(ctx, owner, 0); len(got) != 1 {
		t.Fatalf("limit=0 should default, got %d items", len(got))
	}
}
