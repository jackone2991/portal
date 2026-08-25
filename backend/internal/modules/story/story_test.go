package story

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	storyapi "github.com/portal/backend/internal/modules/story/api"
)

// ══ fakes ═══════════════════════════════════════════════════════════════════

type fakeRepo struct {
	stories     map[uuid.UUID]Story
	chapters    map[uuid.UUID][]Chapter // storyID → chapters, in order
	nulledCover []uuid.UUID
	reordered   []uuid.UUID
}

func newFake() *fakeRepo {
	return &fakeRepo{stories: map[uuid.UUID]Story{}, chapters: map[uuid.UUID][]Chapter{}}
}

func (r *fakeRepo) put(st Story) Story {
	if st.ID == uuid.Nil {
		st.ID = uuid.New()
	}
	if st.Status == "" {
		st.Status = StatusDraft
	}
	st.CreatedAt, st.UpdatedAt = time.Now(), time.Now()
	r.stories[st.ID] = st
	return st
}

// addChapter appends a chapter; an empty body is what blocks publish.
func (r *fakeRepo) addChapter(storyID uuid.UUID, title, body string) Chapter {
	c := Chapter{ID: uuid.New(), StoryID: storyID, Title: title, BodyMd: body,
		SortOrder: (len(r.chapters[storyID]) + 1) * 10, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.chapters[storyID] = append(r.chapters[storyID], c)
	return c
}

func (r *fakeRepo) CreateStory(_ context.Context, in CreateStoryInput) (Story, error) {
	return r.put(Story{OwnerID: in.OwnerID, Title: in.Title, Description: in.Description, CoverAssetID: in.CoverAssetID}), nil
}

func (r *fakeRepo) GetStory(_ context.Context, id uuid.UUID) (Story, error) {
	st, ok := r.stories[id]
	if !ok {
		return Story{}, ErrNotFound
	}
	return st, nil
}

func (r *fakeRepo) ListPublished(_ context.Context, _ ListInput) ([]Story, error) {
	out := []Story{}
	for _, st := range r.stories {
		if st.Status == StatusPublished {
			out = append(out, st)
		}
	}
	return out, nil
}

func (r *fakeRepo) ListOwn(_ context.Context, in ListInput) ([]Story, error) {
	out := []Story{}
	for _, st := range r.stories {
		if st.OwnerID == in.OwnerID {
			out = append(out, st)
		}
	}
	return out, nil
}

func (r *fakeRepo) UpdateStory(_ context.Context, in UpdateStoryInput) (Story, error) {
	st, ok := r.stories[in.ID]
	if !ok {
		return Story{}, ErrNotFound
	}
	if in.Title != nil {
		st.Title = *in.Title
	}
	if in.SetCover {
		st.CoverAssetID = in.CoverAssetID
	}
	r.stories[in.ID] = st
	return st, nil
}

func (r *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, status string) (Story, error) {
	st, ok := r.stories[id]
	if !ok {
		return Story{}, ErrNotFound
	}
	st.Status = status
	r.stories[id] = st
	return st, nil
}

func (r *fakeRepo) DeleteStory(_ context.Context, id uuid.UUID) error {
	delete(r.stories, id)
	return nil
}

func (r *fakeRepo) OwnerByStory(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	st, ok := r.stories[id]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return st.OwnerID, nil
}

func (r *fakeRepo) CountChapters(_ context.Context, storyID uuid.UUID) (int, error) {
	return len(r.chapters[storyID]), nil
}

func (r *fakeRepo) EmptyChapters(_ context.Context, storyID uuid.UUID) ([]ChapterRef, error) {
	out := []ChapterRef{}
	for _, c := range r.chapters[storyID] {
		if c.BodyMd == "" {
			out = append(out, ChapterRef{ID: c.ID, Title: c.Title})
		}
	}
	return out, nil
}

func (r *fakeRepo) NullCoverByAsset(_ context.Context, assetID uuid.UUID) error {
	r.nulledCover = append(r.nulledCover, assetID)
	return nil
}

func (r *fakeRepo) CreateChapter(_ context.Context, storyID uuid.UUID, title, bodyMd string, _ int) (Chapter, error) {
	return r.addChapter(storyID, title, bodyMd), nil
}

func (r *fakeRepo) GetChapter(_ context.Context, id uuid.UUID) (Chapter, error) {
	for _, cs := range r.chapters {
		for _, c := range cs {
			if c.ID == id {
				return c, nil
			}
		}
	}
	return Chapter{}, ErrNotFound
}

func (r *fakeRepo) ListChapters(_ context.Context, storyID uuid.UUID) ([]Chapter, error) {
	return r.chapters[storyID], nil
}

func (r *fakeRepo) UpdateChapter(_ context.Context, id uuid.UUID, title, bodyMd *string) (Chapter, error) {
	for sid, cs := range r.chapters {
		for i, c := range cs {
			if c.ID != id {
				continue
			}
			if title != nil {
				c.Title = *title
			}
			if bodyMd != nil {
				c.BodyMd = *bodyMd
			}
			r.chapters[sid][i] = c
			return c, nil
		}
	}
	return Chapter{}, ErrNotFound
}

func (r *fakeRepo) DeleteChapter(_ context.Context, id uuid.UUID) error {
	for sid, cs := range r.chapters {
		for i, c := range cs {
			if c.ID == id {
				r.chapters[sid] = append(cs[:i], cs[i+1:]...)
				return nil
			}
		}
	}
	return ErrNotFound
}

func (r *fakeRepo) ReorderChapters(_ context.Context, _ uuid.UUID, orderedIDs []uuid.UUID) error {
	r.reordered = orderedIDs
	return nil
}

func (r *fakeRepo) OwnerAndStoryByChapter(_ context.Context, chapterID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	for sid, cs := range r.chapters {
		for _, c := range cs {
			if c.ID == chapterID {
				return r.stories[sid].OwnerID, sid, nil
			}
		}
	}
	return uuid.Nil, uuid.Nil, ErrNotFound
}

type fakeMedia struct{ assets map[uuid.UUID]*mediaapi.Asset }

func (m *fakeMedia) GetAsset(_ context.Context, id uuid.UUID) (*mediaapi.Asset, error) {
	return m.assets[id], nil
}

func (m *fakeMedia) asset(ownerID uuid.UUID, kind mediaapi.AssetKind, status mediaapi.AssetStatus) uuid.UUID {
	id := uuid.New()
	m.assets[id] = &mediaapi.Asset{ID: id, OwnerID: ownerID, Kind: kind, Status: status}
	return id
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

// ══ visibility ══════════════════════════════════════════════════════════════

func TestDraftIsInvisibleToOthers(t *testing.T) {
	svc, repo, _, _ := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "Draft"})

	if _, err := svc.GetStory(context.Background(), owner, st.ID); err != nil {
		t.Fatalf("owner cannot read own draft: %v", err)
	}
	if _, err := svc.GetStory(context.Background(), stranger, st.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger got %v, want ErrNotFound — a draft must not be visible", err)
	}
}

func TestPublishedIsVisibleToEveryone(t *testing.T) {
	svc, repo, _, _ := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "Public", Status: StatusPublished})

	if _, err := svc.GetStory(context.Background(), stranger, st.ID); err != nil {
		t.Fatalf("stranger cannot read a published story: %v", err)
	}
}

// ══ publish gate ════════════════════════════════════════════════════════════

// A story with no chapters is not a story yet.
func TestPublishRejectsAStoryWithNoChapters(t *testing.T) {
	svc, repo, _, events := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "Empty"})

	_, err := svc.Publish(context.Background(), st.ID)
	var np *NotPublishableError
	if !errors.As(err, &np) {
		t.Fatalf("got %v, want *NotPublishableError", err)
	}
	if len(events.published) != 0 {
		t.Errorf("a failed publish emitted %v", events.published)
	}
	if repo.stories[st.ID].Status != StatusDraft {
		t.Error("a failed publish still flipped the status")
	}
}

// An empty-bodied chapter blocks publish, and the error must NAME it — the
// client shows which chapter to fix rather than restating the rule.
func TestPublishNamesTheEmptyChapters(t *testing.T) {
	svc, repo, _, _ := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "Half written"})
	repo.addChapter(st.ID, "Chương 1", "Nội dung đầy đủ.")
	empty := repo.addChapter(st.ID, "Chương 2", "")

	_, err := svc.Publish(context.Background(), st.ID)
	var np *NotPublishableError
	if !errors.As(err, &np) {
		t.Fatalf("got %v, want *NotPublishableError", err)
	}
	if len(np.Chapters) != 1 || np.Chapters[0].ID != empty.ID {
		t.Fatalf("Chapters = %+v, want exactly the empty chapter %v", np.Chapters, empty.ID)
	}
	if np.Chapters[0].Title != "Chương 2" {
		t.Errorf("title = %q — the client needs the title to point the user at it", np.Chapters[0].Title)
	}
}

func TestPublishSucceedsAndEmits(t *testing.T) {
	svc, repo, _, events := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "Đắc Nhân Tâm"})
	repo.addChapter(st.ID, "Chương 1", "Nội dung.")

	pub, err := svc.Publish(context.Background(), st.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if pub.Status != StatusPublished {
		t.Errorf("status = %q, want published", pub.Status)
	}
	if len(events.published) != 1 || events.published[0] != storyapi.EventStoryPublished {
		t.Errorf("emitted %v, want exactly [%s]", events.published, storyapi.EventStoryPublished)
	}
}

func TestPublishSurvivesAnAbsentPublisher(t *testing.T) {
	svc, repo, _, _ := newSvc()
	svc.events = nil
	st := repo.put(Story{OwnerID: owner, Title: "T"})
	repo.addChapter(st.ID, "Chương 1", "Nội dung.")

	if _, err := svc.Publish(context.Background(), st.ID); err != nil {
		t.Fatalf("publish with no publisher failed: %v", err)
	}
	if repo.stories[st.ID].Status != StatusPublished {
		t.Error("status did not flip")
	}
}

func TestUnpublishReturnsToDraft(t *testing.T) {
	svc, repo, _, _ := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "T", Status: StatusPublished})

	got, err := svc.Unpublish(context.Background(), st.ID)
	if err != nil {
		t.Fatalf("Unpublish: %v", err)
	}
	if got.Status != StatusDraft {
		t.Errorf("status = %q, want draft", got.Status)
	}
}

// ══ cover validation ════════════════════════════════════════════════════════

func TestCreateRejectsAForeignCover(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(stranger, mediaapi.KindImage, mediaapi.StatusReady)

	if _, err := svc.CreateStory(context.Background(), CreateStoryInput{
		OwnerID: owner, Title: "Borrowed cover", CoverAssetID: &aid,
	}); !errors.Is(err, ErrInvalidCoverAsset) {
		t.Fatalf("got %v, want ErrInvalidCoverAsset", err)
	}
}

func TestCreateRejectsAVideoAsTheCover(t *testing.T) {
	svc, _, media, _ := newSvc()
	aid := media.asset(owner, mediaapi.KindVideo, mediaapi.StatusReady)

	if _, err := svc.CreateStory(context.Background(), CreateStoryInput{
		OwnerID: owner, Title: "Wrong kind", CoverAssetID: &aid,
	}); !errors.Is(err, ErrInvalidCoverAsset) {
		t.Fatalf("got %v, want ErrInvalidCoverAsset", err)
	}
}

func TestCreateRejectsABlankTitle(t *testing.T) {
	svc, _, _, _ := newSvc()
	for _, title := range []string{"", "   ", "\t\n"} {
		if _, err := svc.CreateStory(context.Background(), CreateStoryInput{OwnerID: owner, Title: title}); !errors.Is(err, ErrValidation) {
			t.Errorf("title %q: got %v, want ErrValidation", title, err)
		}
	}
}

// ══ chapters ════════════════════════════════════════════════════════════════

// A story's own chapter list is the reader payload; the visibility gate is on
// the story, not on each chapter.
func TestChaptersVisibleFollowsTheStoryGate(t *testing.T) {
	svc, repo, _, _ := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "Draft"})
	repo.addChapter(st.ID, "Chương 1", "Nội dung.")

	if _, err := svc.ChaptersVisible(context.Background(), owner, st.ID); err != nil {
		t.Fatalf("owner cannot read own draft chapters: %v", err)
	}
	if _, err := svc.ChaptersVisible(context.Background(), stranger, st.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stranger got %v, want ErrNotFound", err)
	}
}

// Reorder passes the caller's complete ordered list straight through — the
// server renumbers from array position, so the list IS the new order.
func TestReorderPassesTheOrderThrough(t *testing.T) {
	svc, repo, _, _ := newSvc()
	st := repo.put(Story{OwnerID: owner, Title: "T"})
	c1 := repo.addChapter(st.ID, "Chương 1", "a")
	c2 := repo.addChapter(st.ID, "Chương 2", "b")

	want := []uuid.UUID{c2.ID, c1.ID}
	if err := svc.ReorderChapters(context.Background(), st.ID, want); err != nil {
		t.Fatalf("ReorderChapters: %v", err)
	}
	if len(repo.reordered) != 2 || repo.reordered[0] != c2.ID || repo.reordered[1] != c1.ID {
		t.Errorf("reordered = %v, want %v", repo.reordered, want)
	}
}

// ══ the media:asset_deleted consumer ════════════════════════════════════════

func TestAssetDeletedClearsTheCover(t *testing.T) {
	svc, repo, _, _ := newSvc()
	aid := uuid.New()

	if err := svc.HandleAssetDeleted(context.Background(), aid); err != nil {
		t.Fatalf("HandleAssetDeleted: %v", err)
	}
	if len(repo.nulledCover) != 1 || repo.nulledCover[0] != aid {
		t.Errorf("cover refs nulled = %v, want [%v]", repo.nulledCover, aid)
	}
}

func TestAssetDeletedIsIdempotent(t *testing.T) {
	svc, _, _, _ := newSvc()
	aid := uuid.New()
	for i := 0; i < 3; i++ {
		if err := svc.HandleAssetDeleted(context.Background(), aid); err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
	}
}
