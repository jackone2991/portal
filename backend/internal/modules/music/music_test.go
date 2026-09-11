package music

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	musicapi "github.com/portal/backend/internal/modules/music/api"
)

// ══ fakes ═══════════════════════════════════════════════════════════════════

type fakeRepo struct {
	tracks      map[uuid.UUID]Track
	nulledAudio []uuid.UUID
	nulledCover []uuid.UUID
	getErr      error
}

func newFake() *fakeRepo { return &fakeRepo{tracks: map[uuid.UUID]Track{}} }

func (r *fakeRepo) put(m Track) Track {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Status == "" {
		m.Status = StatusDraft
	}
	m.CreatedAt, m.UpdatedAt = time.Now(), time.Now()
	r.tracks[m.ID] = m
	return m
}

func (r *fakeRepo) CreateTrack(_ context.Context, in CreateTrackInput) (Track, error) {
	return r.put(Track{
		OwnerID: in.OwnerID, Title: in.Title, Description: in.Description,
		AudioAssetID: in.AudioAssetID, CoverAssetID: in.CoverAssetID,
	}), nil
}

func (r *fakeRepo) GetTrack(_ context.Context, id uuid.UUID) (Track, error) {
	if r.getErr != nil {
		return Track{}, r.getErr
	}
	m, ok := r.tracks[id]
	if !ok {
		return Track{}, ErrNotFound
	}
	return m, nil
}

func (r *fakeRepo) ListPublished(_ context.Context, _ ListInput) ([]Track, error) {
	out := []Track{}
	for _, m := range r.tracks {
		if m.Status == StatusPublished {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListOwn(_ context.Context, in ListInput) ([]Track, error) {
	out := []Track{}
	for _, m := range r.tracks {
		if m.OwnerID == in.OwnerID {
			out = append(out, m)
		}
	}
	return out, nil
}

func (r *fakeRepo) UpdateTrack(_ context.Context, in UpdateTrackInput) (Track, error) {
	m, ok := r.tracks[in.ID]
	if !ok {
		return Track{}, ErrNotFound
	}
	if in.Title != nil {
		m.Title = *in.Title
	}
	if in.SetAudio {
		m.AudioAssetID = in.AudioAssetID
	}
	if in.SetCover {
		m.CoverAssetID = in.CoverAssetID
	}
	r.tracks[in.ID] = m
	return m, nil
}

func (r *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, status string) (Track, error) {
	m, ok := r.tracks[id]
	if !ok {
		return Track{}, ErrNotFound
	}
	m.Status = status
	r.tracks[id] = m
	return m, nil
}

func (r *fakeRepo) DeleteTrack(_ context.Context, id uuid.UUID) error {
	delete(r.tracks, id)
	return nil
}

func (r *fakeRepo) OwnerByTrack(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	m, ok := r.tracks[id]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return m.OwnerID, nil
}

func (r *fakeRepo) NullAudioByAsset(_ context.Context, assetID uuid.UUID) error {
	r.nulledAudio = append(r.nulledAudio, assetID)
	return nil
}

func (r *fakeRepo) NullCoverByAsset(_ context.Context, assetID uuid.UUID) error {
	r.nulledCover = append(r.nulledCover, assetID)
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
// track exists, which is the leak this rule prevents.
func TestDraftIsInvisibleToOthers(t *testing.T) {
	svc, repo, _, _ := newSvc()
	m := repo.put(Track{OwnerID: owner, Title: "Draft"})

	if _, err := svc.GetTrack(context.Background(), owner, m.ID); err != nil {
		t.Fatalf("owner cannot read own draft: %v", err)
	}
	_, err := svc.GetTrack(context.Background(), stranger, m.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger got %v, want ErrNotFound — a draft must not be visible", err)
	}
}

func TestPublishedIsVisibleToEveryone(t *testing.T) {
	svc, repo, _, _ := newSvc()
	m := repo.put(Track{OwnerID: owner, Title: "Public", Status: StatusPublished})

	if _, err := svc.GetTrack(context.Background(), stranger, m.ID); err != nil {
		t.Fatalf("stranger cannot read a published movie: %v", err)
	}
}

// ══ publish gate ════════════════════════════════════════════════════════════

func TestPublishRequiresAVideoAsset(t *testing.T) {
	svc, repo, _, events := newSvc()
	m := repo.put(Track{OwnerID: owner, Title: "No video"})

	_, err := svc.Publish(context.Background(), m.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("got %v, want ErrNotPublishable", err)
	}
	if len(events.published) != 0 {
		t.Errorf("a failed publish emitted %v", events.published)
	}
	if repo.tracks[m.ID].Status != StatusDraft {
		t.Error("a failed publish still flipped the status")
	}
}

// The asset must be ready. Publishing against one still transcoding would give
// a published track that cannot play.
func TestPublishRejectsAnUnreadyOrForeignAsset(t *testing.T) {
	for name, mk := range map[string]func(*fakeMedia) uuid.UUID{
		"still processing": func(fm *fakeMedia) uuid.UUID {
			return fm.asset(owner, mediaapi.KindAudio, mediaapi.AssetStatus("processing"))
		},
		"wrong kind":     func(fm *fakeMedia) uuid.UUID { return fm.asset(owner, mediaapi.KindImage, mediaapi.StatusReady) },
		"someone else's": func(fm *fakeMedia) uuid.UUID { return fm.asset(stranger, mediaapi.KindAudio, mediaapi.StatusReady) },
		"missing":        func(fm *fakeMedia) uuid.UUID { return uuid.New() },
	} {
		t.Run(name, func(t *testing.T) {
			svc, repo, media, _ := newSvc()
			aid := mk(media)
			m := repo.put(Track{OwnerID: owner, Title: "T", AudioAssetID: &aid})

			if _, err := svc.Publish(context.Background(), m.ID); !errors.Is(err, ErrNotPublishable) {
				t.Fatalf("got %v, want ErrNotPublishable", err)
			}
		})
	}
}

func TestPublishSucceedsAndEmits(t *testing.T) {
	svc, repo, media, events := newSvc()
	aid := media.asset(owner, mediaapi.KindAudio, mediaapi.StatusReady)
	m := repo.put(Track{OwnerID: owner, Title: "Chuyến đi Đà Lạt", AudioAssetID: &aid})

	pub, err := svc.Publish(context.Background(), m.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pub.Status != StatusPublished {
		t.Errorf("status = %q, want published", pub.Status)
	}
	if len(events.published) != 1 || events.published[0] != musicapi.EventTrackPublished {
		t.Errorf("emitted %v, want exactly [%s]", events.published, musicapi.EventTrackPublished)
	}
}

// The event is best-effort: a publisher failure must not undo a committed
// publish, and a nil publisher must not panic.
func TestPublishSurvivesAFailingOrAbsentPublisher(t *testing.T) {
	svc, repo, media, _ := newSvc()
	svc.events = nil
	aid := media.asset(owner, mediaapi.KindAudio, mediaapi.StatusReady)
	m := repo.put(Track{OwnerID: owner, Title: "T", AudioAssetID: &aid})

	if _, err := svc.Publish(context.Background(), m.ID); err != nil {
		t.Fatalf("publish with no publisher failed: %v", err)
	}
	if repo.tracks[m.ID].Status != StatusPublished {
		t.Error("status did not flip")
	}
}

func TestUnpublishReturnsToDraft(t *testing.T) {
	svc, repo, _, _ := newSvc()
	m := repo.put(Track{OwnerID: owner, Title: "T", Status: StatusPublished})

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
	aid := media.asset(stranger, mediaapi.KindAudio, mediaapi.StatusReady)

	_, err := svc.CreateTrack(context.Background(), CreateTrackInput{
		OwnerID: owner, Title: "Borrowed", AudioAssetID: &aid,
	})
	if !errors.Is(err, ErrInvalidAudioAsset) {
		t.Fatalf("got %v, want ErrInvalidAudioAsset — an asset you do not own must not attach", err)
	}
}

func TestCreateRejectsAnImageAsTheVideo(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(owner, mediaapi.KindImage, mediaapi.StatusReady)

	if _, err := svc.CreateTrack(context.Background(), CreateTrackInput{
		OwnerID: owner, Title: "Wrong kind", AudioAssetID: &aid,
	}); !errors.Is(err, ErrInvalidAudioAsset) {
		t.Fatalf("got %v, want ErrInvalidAudioAsset", err)
	}
}

func TestCreateRejectsAVideoAsThePoster(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(owner, mediaapi.KindAudio, mediaapi.StatusReady)

	if _, err := svc.CreateTrack(context.Background(), CreateTrackInput{
		OwnerID: owner, Title: "Wrong cover", CoverAssetID: &aid,
	}); !errors.Is(err, ErrInvalidCoverAsset) {
		t.Fatalf("got %v, want ErrInvalidCoverAsset", err)
	}
}

func TestCreateRejectsABlankTitle(t *testing.T) {
	svc, _, _, _ := newSvc()
	for _, title := range []string{"", "   ", "\t\n"} {
		if _, err := svc.CreateTrack(context.Background(), CreateTrackInput{OwnerID: owner, Title: title}); !errors.Is(err, ErrValidation) {
			t.Errorf("title %q: got %v, want ErrValidation", title, err)
		}
	}
}

func TestCreateTrimsTheTitle(t *testing.T) {
	svc, _, _, _ := newSvc()
	m, err := svc.CreateTrack(context.Background(), CreateTrackInput{OwnerID: owner, Title: "  Người Nhện  "})
	if err != nil {
		t.Fatalf("CreateTrack: %v", err)
	}
	if m.Title != "Người Nhện" {
		t.Errorf("title = %q, want it trimmed", m.Title)
	}
}

// ══ the media:asset_deleted consumer ════════════════════════════════════════

// Deleting an asset must clear BOTH reference columns. Clearing only the audio
// would leave a cover pointing at storage that no longer exists.
func TestAssetDeletedClearsBothReferences(t *testing.T) {
	svc, repo, _, _ := newSvc()
	aid := uuid.New()

	if err := svc.HandleAssetDeleted(context.Background(), aid); err != nil {
		t.Fatalf("HandleAssetDeleted: %v", err)
	}
	if len(repo.nulledAudio) != 1 || repo.nulledAudio[0] != aid {
		t.Errorf("video refs nulled = %v, want [%v]", repo.nulledAudio, aid)
	}
	if len(repo.nulledCover) != 1 || repo.nulledCover[0] != aid {
		t.Errorf("cover refs nulled = %v, want [%v]", repo.nulledCover, aid)
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
