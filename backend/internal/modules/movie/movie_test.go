package movie

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	movieapi "github.com/portal/backend/internal/modules/movie/api"
)

// ══ fakes ═══════════════════════════════════════════════════════════════════

type fakeRepo struct {
	movies       map[uuid.UUID]Movie
	nulledVideo  []uuid.UUID
	nulledPoster []uuid.UUID
	getErr       error
}

func newFake() *fakeRepo { return &fakeRepo{movies: map[uuid.UUID]Movie{}} }

func (r *fakeRepo) put(m Movie) Movie {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Status == "" {
		m.Status = StatusDraft
	}
	m.CreatedAt, m.UpdatedAt = time.Now(), time.Now()
	r.movies[m.ID] = m
	return m
}

func (r *fakeRepo) CreateMovie(_ context.Context, in CreateMovieInput) (Movie, error) {
	return r.put(Movie{
		OwnerID: in.OwnerID, Title: in.Title, Description: in.Description,
		VideoAssetID: in.VideoAssetID, PosterAssetID: in.PosterAssetID, ReleaseYear: in.ReleaseYear,
	}), nil
}

func (r *fakeRepo) GetMovie(_ context.Context, id uuid.UUID) (Movie, error) {
	if r.getErr != nil {
		return Movie{}, r.getErr
	}
	m, ok := r.movies[id]
	if !ok {
		return Movie{}, ErrNotFound
	}
	return m, nil
}

func (r *fakeRepo) ListPublished(_ context.Context, _ ListInput) ([]Movie, error) {
	out := []Movie{}
	for _, m := range r.movies {
		if m.Status == StatusPublished {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListOwn(_ context.Context, in ListInput) ([]Movie, error) {
	out := []Movie{}
	for _, m := range r.movies {
		if m.OwnerID == in.OwnerID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeRepo) UpdateMovie(_ context.Context, in UpdateMovieInput) (Movie, error) {
	m, ok := r.movies[in.ID]
	if !ok {
		return Movie{}, ErrNotFound
	}
	if in.Title != nil {
		m.Title = *in.Title
	}
	if in.SetVideo {
		m.VideoAssetID = in.VideoAssetID
	}
	if in.SetPoster {
		m.PosterAssetID = in.PosterAssetID
	}
	r.movies[in.ID] = m
	return m, nil
}

func (r *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, status string) (Movie, error) {
	m, ok := r.movies[id]
	if !ok {
		return Movie{}, ErrNotFound
	}
	m.Status = status
	r.movies[id] = m
	return m, nil
}

func (r *fakeRepo) DeleteMovie(_ context.Context, id uuid.UUID) error {
	delete(r.movies, id)
	return nil
}

func (r *fakeRepo) OwnerByMovie(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	m, ok := r.movies[id]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return m.OwnerID, nil
}

func (r *fakeRepo) NullVideoByAsset(_ context.Context, assetID uuid.UUID) error {
	r.nulledVideo = append(r.nulledVideo, assetID)
	return nil
}

func (r *fakeRepo) NullPosterByAsset(_ context.Context, assetID uuid.UUID) error {
	r.nulledPoster = append(r.nulledPoster, assetID)
	return nil
}

type fakeMedia struct{ assets map[uuid.UUID]*mediaapi.Asset }

func (m *fakeMedia) GetAsset(_ context.Context, id uuid.UUID) (*mediaapi.Asset, error) {
	return m.assets[id], nil // (nil, nil) for "missing or another tenant's"
}

type fakeEvents struct{ published []string }

func (e *fakeEvents) Publish(_ context.Context, name string, _ any) error {
	e.published = append(e.published, name)
	return nil
}

// ══ harness ═════════════════════════════════════════════════════════════════

var (
	owner    = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	stranger = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

func newSvc() (*Service, *fakeRepo, *fakeMedia, *fakeEvents) {
	repo, media, events := newFake(), &fakeMedia{assets: map[uuid.UUID]*mediaapi.Asset{}}, &fakeEvents{}
	return &Service{repo: repo, media: media, events: events}, repo, media, events
}

// asset registers an asset with the fake media module and returns its id.
func (m *fakeMedia) asset(ownerID uuid.UUID, kind mediaapi.AssetKind, status mediaapi.AssetStatus) uuid.UUID {
	id := uuid.New()
	m.assets[id] = &mediaapi.Asset{ID: id, OwnerID: ownerID, Kind: kind, Status: status}
	return id
}

// ══ visibility ══════════════════════════════════════════════════════════════

// A draft is 404 to everyone but its owner. Not 403 — a 403 would confirm the
// movie exists, which is the leak this rule prevents.
func TestDraftIsInvisibleToOthers(t *testing.T) {
	svc, repo, _, _ := newSvc()
	m := repo.put(Movie{OwnerID: owner, Title: "Draft"})

	if _, err := svc.GetMovie(context.Background(), owner, m.ID); err != nil {
		t.Fatalf("owner cannot read own draft: %v", err)
	}
	_, err := svc.GetMovie(context.Background(), stranger, m.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger got %v, want ErrNotFound — a draft must not be visible", err)
	}
}

func TestPublishedIsVisibleToEveryone(t *testing.T) {
	svc, repo, _, _ := newSvc()
	m := repo.put(Movie{OwnerID: owner, Title: "Public", Status: StatusPublished})

	if _, err := svc.GetMovie(context.Background(), stranger, m.ID); err != nil {
		t.Fatalf("stranger cannot read a published movie: %v", err)
	}
}

// ══ publish gate ════════════════════════════════════════════════════════════

func TestPublishRequiresAVideoAsset(t *testing.T) {
	svc, repo, _, events := newSvc()
	m := repo.put(Movie{OwnerID: owner, Title: "No video"})

	_, err := svc.Publish(context.Background(), m.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("got %v, want ErrNotPublishable", err)
	}
	if len(events.published) != 0 {
		t.Errorf("a failed publish emitted %v", events.published)
	}
	if repo.movies[m.ID].Status != StatusDraft {
		t.Error("a failed publish still flipped the status")
	}
}

// The asset must be ready. Publishing against one still transcoding would give
// a published movie that cannot play.
func TestPublishRejectsAnUnreadyOrForeignAsset(t *testing.T) {
	for name, mk := range map[string]func(*fakeMedia) uuid.UUID{
		"still processing": func(fm *fakeMedia) uuid.UUID {
			return fm.asset(owner, mediaapi.KindVideo, mediaapi.AssetStatus("processing"))
		},
		"wrong kind":     func(fm *fakeMedia) uuid.UUID { return fm.asset(owner, mediaapi.KindImage, mediaapi.StatusReady) },
		"someone else's": func(fm *fakeMedia) uuid.UUID { return fm.asset(stranger, mediaapi.KindVideo, mediaapi.StatusReady) },
		"missing":        func(fm *fakeMedia) uuid.UUID { return uuid.New() },
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo, media, _ := newSvc()
			aid := mk(media)
			m := repo.put(Movie{OwnerID: owner, Title: "T", VideoAssetID: &aid})

			if _, err := svc.Publish(context.Background(), m.ID); !errors.Is(err, ErrNotPublishable) {
				t.Fatalf("got %v, want ErrNotPublishable", err)
			}
		})
	}
}

func TestPublishSucceedsAndEmits(t *testing.T) {
	svc, repo, media, events := newSvc()
	aid := media.asset(owner, mediaapi.KindVideo, mediaapi.StatusReady)
	m := repo.put(Movie{OwnerID: owner, Title: "Chuyến đi Đà Lạt", VideoAssetID: &aid})

	pub, err := svc.Publish(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pub.Status != StatusPublished {
		t.Errorf("status = %q, want published", pub.Status)
	}
	if len(events.published) != 1 || events.published[0] != movieapi.EventMoviePublished {
		t.Errorf("emitted %v, want exactly [%s]", events.published, movieapi.EventMoviePublished)
	}
}

// The event is best-effort: a publisher failure must not undo a committed
// publish, and a nil publisher must not panic.
func TestPublishSurvivesAFailingOrAbsentPublisher(t *testing.T) {
	svc, repo, media, _ := newSvc()
	svc.events = nil
	aid := media.asset(owner, mediaapi.KindVideo, mediaapi.StatusReady)
	m := repo.put(Movie{OwnerID: owner, Title: "T", VideoAssetID: &aid})

	if _, err := svc.Publish(context.Background(), m.ID); err != nil {
		t.Fatalf("publish with no publisher failed: %v", err)
	}
	if repo.movies[m.ID].Status != StatusPublished {
		t.Error("status did not flip")
	}
}

func TestUnpublishReturnsToDraft(t *testing.T) {
	svc, repo, _, _ := newSvc()
	m := repo.put(Movie{OwnerID: owner, Title: "T", Status: StatusPublished})

	got, err := svc.Unpublish(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if got.Status != StatusDraft {
		t.Errorf("status = %q, want draft", got.Status)
	}
}

// ══ asset validation on write ═══════════════════════════════════════════════

func TestCreateRejectsAForeignVideoAsset(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(stranger, mediaapi.KindVideo, mediaapi.StatusReady)

	_, err := svc.CreateMovie(context.Background(), CreateMovieInput{
		OwnerID: owner, Title: "Borrowed", VideoAssetID: &aid,
	})
	if !errors.Is(err, ErrInvalidVideoAsset) {
		t.Fatalf("got %v, want ErrInvalidVideoAsset — an asset you do not own must not attach", err)
	}
}

func TestCreateRejectsAnImageAsTheVideo(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(owner, mediaapi.KindImage, mediaapi.StatusReady)

	if _, err := svc.CreateMovie(context.Background(), CreateMovieInput{
		OwnerID: owner, Title: "Wrong kind", VideoAssetID: &aid,
	}); !errors.Is(err, ErrInvalidVideoAsset) {
		t.Fatalf("got %v, want ErrInvalidVideoAsset", err)
	}
}

func TestCreateRejectsAVideoAsThePoster(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(owner, mediaapi.KindVideo, mediaapi.StatusReady)

	if _, err := svc.CreateMovie(context.Background(), CreateMovieInput{
		OwnerID: owner, Title: "Wrong poster", PosterAssetID: &aid,
	}); !errors.Is(err, ErrInvalidPosterAsset) {
		t.Fatalf("got %v, want ErrInvalidPosterAsset", err)
	}
}

func TestCreateRejectsABlankTitle(t *testing.T) {
	svc, _, _, _ := newSvc()
	for _, title := range []string{"", "   ", "\t\n"} {
		if _, err := svc.CreateMovie(context.Background(), CreateMovieInput{OwnerID: owner, Title: title}); !errors.Is(err, ErrValidation) {
			t.Errorf("title %q: got %v, want ErrValidation", title, err)
		}
	}
}

func TestCreateTrimsTheTitle(t *testing.T) {
	svc, _, _, _ := newSvc()
	m, err := svc.CreateMovie(context.Background(), CreateMovieInput{OwnerID: owner, Title: "  Người Nhện  "})
	if err != nil {
		t.Fatalf("CreateMovie: %v", err)
	}
	if m.Title != "Người Nhện" {
		t.Errorf("title = %q, want it trimmed", m.Title)
	}
}

// ══ the media:asset_deleted consumer ════════════════════════════════════════

// Deleting an asset must clear BOTH reference columns. Clearing only the video
// would leave a poster pointing at storage that no longer exists.
func TestAssetDeletedClearsBothReferences(t *testing.T) {
	svc, repo, _, _ := newSvc()
	aid := uuid.New()

	if err := svc.HandleAssetDeleted(context.Background(), aid); err != nil {
		t.Fatalf("HandleAssetDeleted: %v", err)
	}
	if len(repo.nulledVideo) != 1 || repo.nulledVideo[0] != aid {
		t.Errorf("video refs nulled = %v, want [%v]", repo.nulledVideo, aid)
	}
	if len(repo.nulledPoster) != 1 || repo.nulledPoster[0] != aid {
		t.Errorf("poster refs nulled = %v, want [%v]", repo.nulledPoster, aid)
	}
}

// The consumer is idempotent: Asynq redelivers, so a second run must be a no-op
// rather than an error.
func TestAssetDeletedIsIdempotent(t *testing.T) {
	svc, _, _, _ := newSvc()
	aid := uuid.New()
	for i := 0; i < 3; i++ {
		if err := svc.HandleAssetDeleted(context.Background(), aid); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
}
