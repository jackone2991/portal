package comic

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// ── fake media ───────────────────────────────────────────────────────

type fakeMedia struct{ assets map[uuid.UUID]*mediaapi.Asset }

func (m *fakeMedia) GetAsset(_ context.Context, id uuid.UUID) (*mediaapi.Asset, error) {
	return m.assets[id], nil
}
func (m *fakeMedia) AssetStatuses(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]mediaapi.AssetStatus, error) {
	out := make(map[uuid.UUID]mediaapi.AssetStatus, len(ids))
	for _, id := range ids {
		if a := m.assets[id]; a != nil {
			out[id] = a.Status
		}
	}
	return out, nil
}
func (m *fakeMedia) IngestImage(_ context.Context, owner uuid.UUID, _, _ string, _ []byte) (uuid.UUID, error) {
	return m.put(owner, mediaapi.KindImage, mediaapi.StatusReady), nil
}
func (m *fakeMedia) put(owner uuid.UUID, kind mediaapi.AssetKind, status mediaapi.AssetStatus) uuid.UUID {
	id := uuid.New()
	m.assets[id] = &mediaapi.Asset{ID: id, OwnerID: owner, Kind: kind, Status: status}
	return id
}

// ── fake repo ────────────────────────────────────────────────────────

type fakeRepo struct {
	comics   map[uuid.UUID]*Comic
	chapters map[uuid.UUID]*Chapter
	pages    map[uuid.UUID]*Page
	progress map[string]*Progress
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{comics: map[uuid.UUID]*Comic{}, chapters: map[uuid.UUID]*Chapter{}, pages: map[uuid.UUID]*Page{}, progress: map[string]*Progress{}}
}
func pkey(u, c uuid.UUID) string { return u.String() + "|" + c.String() }

func (r *fakeRepo) CreateComic(_ context.Context, in CreateComicInput) (Comic, error) {
	id := uuid.New()
	c := Comic{ID: id, OwnerID: in.OwnerID, Title: in.Title, Description: in.Description, CoverAssetID: in.CoverAssetID, Status: StatusDraft, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	r.comics[id] = &c
	return c, nil
}
func (r *fakeRepo) GetComic(_ context.Context, id uuid.UUID) (Comic, error) {
	c, ok := r.comics[id]
	if !ok {
		return Comic{}, ErrNotFound
	}
	return *c, nil
}
func (r *fakeRepo) ListPublished(_ context.Context, _ ListInput) ([]Comic, error) {
	var out []Comic
	for _, c := range r.comics {
		if c.Status == StatusPublished {
			out = append(out, *c)
		}
	}
	return out, nil
}
func (r *fakeRepo) ListOwn(_ context.Context, in ListInput) ([]Comic, error) {
	var out []Comic
	for _, c := range r.comics {
		if c.OwnerID == in.OwnerID {
			out = append(out, *c)
		}
	}
	return out, nil
}
func (r *fakeRepo) UpdateComic(_ context.Context, in UpdateComicInput) (Comic, error) {
	c, ok := r.comics[in.ID]
	if !ok {
		return Comic{}, ErrNotFound
	}
	if in.Title != nil {
		c.Title = *in.Title
	}
	if in.Description != nil {
		c.Description = in.Description
	}
	if in.SetCover {
		c.CoverAssetID = in.CoverAssetID
	}
	return *c, nil
}
func (r *fakeRepo) SetStatus(_ context.Context, id uuid.UUID, status string) (Comic, error) {
	c, ok := r.comics[id]
	if !ok {
		return Comic{}, ErrNotFound
	}
	c.Status = status
	return *c, nil
}
func (r *fakeRepo) DeleteComic(_ context.Context, id uuid.UUID) error {
	delete(r.comics, id)
	return nil
}
func (r *fakeRepo) OwnerByComic(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	c, ok := r.comics[id]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return c.OwnerID, nil
}
func (r *fakeRepo) ChaptersWithoutPages(_ context.Context, comicID uuid.UUID) ([]ChapterRef, error) {
	var out []ChapterRef
	for _, ch := range r.chapters {
		if ch.ComicID != comicID {
			continue
		}
		has := false
		for _, p := range r.pages {
			if p.ChapterID == ch.ID {
				has = true
				break
			}
		}
		if !has {
			out = append(out, ChapterRef{ID: ch.ID, Title: ch.Title})
		}
	}
	return out, nil
}
func (r *fakeRepo) CountChapters(_ context.Context, comicID uuid.UUID) (int, error) {
	n := 0
	for _, ch := range r.chapters {
		if ch.ComicID == comicID {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) CreateChapter(_ context.Context, comicID uuid.UUID, title string, sortOrder int) (Chapter, error) {
	id := uuid.New()
	c := Chapter{ID: id, ComicID: comicID, Title: title, SortOrder: sortOrder, CreatedAt: time.Now()}
	r.chapters[id] = &c
	return c, nil
}
func (r *fakeRepo) GetChapter(_ context.Context, id uuid.UUID) (Chapter, error) {
	c, ok := r.chapters[id]
	if !ok {
		return Chapter{}, ErrNotFound
	}
	return *c, nil
}
func (r *fakeRepo) ListChapters(_ context.Context, comicID uuid.UUID) ([]Chapter, error) {
	var out []Chapter
	for _, ch := range r.chapters {
		if ch.ComicID == comicID {
			out = append(out, *ch)
		}
	}
	return out, nil
}
func (r *fakeRepo) UpdateChapter(_ context.Context, id uuid.UUID, title *string) (Chapter, error) {
	c, ok := r.chapters[id]
	if !ok {
		return Chapter{}, ErrNotFound
	}
	if title != nil {
		c.Title = *title
	}
	return *c, nil
}
func (r *fakeRepo) DeleteChapter(_ context.Context, id uuid.UUID) error {
	delete(r.chapters, id)
	return nil
}
func (r *fakeRepo) ReorderChapters(_ context.Context, _ uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		if c, ok := r.chapters[id]; ok {
			c.SortOrder = (i + 1) * 10
		}
	}
	return nil
}
func (r *fakeRepo) OwnerAndComicByChapter(_ context.Context, chapterID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	ch, ok := r.chapters[chapterID]
	if !ok {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	c := r.comics[ch.ComicID]
	return c.OwnerID, ch.ComicID, nil
}
func (r *fakeRepo) CreatePage(_ context.Context, chapterID, assetID uuid.UUID, sortOrder int) (Page, error) {
	id := uuid.New()
	p := Page{ID: id, ChapterID: chapterID, AssetID: assetID, SortOrder: sortOrder}
	r.pages[id] = &p
	return p, nil
}
func (r *fakeRepo) GetPage(_ context.Context, id uuid.UUID) (Page, error) {
	p, ok := r.pages[id]
	if !ok {
		return Page{}, ErrNotFound
	}
	return *p, nil
}
func (r *fakeRepo) ListPages(_ context.Context, chapterID uuid.UUID) ([]Page, error) {
	var out []Page
	for _, p := range r.pages {
		if p.ChapterID == chapterID {
			out = append(out, *p)
		}
	}
	return out, nil
}
func (r *fakeRepo) DeletePage(_ context.Context, id uuid.UUID) error { delete(r.pages, id); return nil }
func (r *fakeRepo) ReorderPages(_ context.Context, _ uuid.UUID, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		if p, ok := r.pages[id]; ok {
			p.SortOrder = (i + 1) * 10
		}
	}
	return nil
}
func (r *fakeRepo) OwnerByPage(_ context.Context, pageID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	p, ok := r.pages[pageID]
	if !ok {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	ch := r.chapters[p.ChapterID]
	c := r.comics[ch.ComicID]
	return c.OwnerID, ch.ComicID, nil
}
func (r *fakeRepo) UpsertProgress(_ context.Context, userID, comicID, chapterID uuid.UUID, pageID *uuid.UUID) error {
	r.progress[pkey(userID, comicID)] = &Progress{ChapterID: chapterID, PageID: pageID, UpdatedAt: time.Now()}
	return nil
}
func (r *fakeRepo) GetProgress(_ context.Context, userID, comicID uuid.UUID) (Progress, error) {
	p, ok := r.progress[pkey(userID, comicID)]
	if !ok {
		return Progress{}, ErrNotFound
	}
	return *p, nil
}
func (r *fakeRepo) PageMembership(_ context.Context, pageID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	p, ok := r.pages[pageID]
	if !ok {
		return uuid.Nil, uuid.Nil, ErrNotFound
	}
	ch := r.chapters[p.ChapterID]
	return p.ChapterID, ch.ComicID, nil
}
func (r *fakeRepo) ChapterComic(_ context.Context, chapterID uuid.UUID) (uuid.UUID, error) {
	ch, ok := r.chapters[chapterID]
	if !ok {
		return uuid.Nil, ErrNotFound
	}
	return ch.ComicID, nil
}
func (r *fakeRepo) DeletePagesByAsset(_ context.Context, assetID uuid.UUID) error {
	for id, p := range r.pages {
		if p.AssetID == assetID {
			delete(r.pages, id)
		}
	}
	return nil
}
func (r *fakeRepo) NullCoverByAsset(_ context.Context, assetID uuid.UUID) error {
	for _, c := range r.comics {
		if c.CoverAssetID != nil && *c.CoverAssetID == assetID {
			c.CoverAssetID = nil
		}
	}
	return nil
}
func (r *fakeRepo) CreateImport(_ context.Context, comicID, chapterID, ownerID uuid.UUID) (ImportJob, error) {
	return ImportJob{ID: uuid.New(), ComicID: comicID, ChapterID: &chapterID, OwnerUserID: ownerID, Status: ImportPending}, nil
}
func (r *fakeRepo) CreateComicImport(_ context.Context, comicID, ownerID uuid.UUID) (ImportJob, error) {
	return ImportJob{ID: uuid.New(), ComicID: comicID, OwnerUserID: ownerID, Status: ImportPending}, nil
}
func (r *fakeRepo) GetImport(_ context.Context, id uuid.UUID) (ImportJob, error) {
	return ImportJob{ID: id}, nil
}
func (r *fakeRepo) SetImportUpload(_ context.Context, id uuid.UUID, ref string) (ImportJob, error) {
	return ImportJob{ID: id, UploadRef: &ref, Status: ImportUploaded}, nil
}
func (r *fakeRepo) StartImport(_ context.Context, _ uuid.UUID, _ int) error { return nil }
func (r *fakeRepo) UpdateImportProgress(_ context.Context, _ uuid.UUID, _, _ int, _ []ImportFileResult) error {
	return nil
}
func (r *fakeRepo) FinishImport(_ context.Context, _ uuid.UUID, _ string, _, _ int, _ []ImportFileResult, _ *string) error {
	return nil
}
func (r *fakeRepo) CreateSyncSource(_ context.Context, comicID, ownerID uuid.UUID, sourceURL, site, hint string) (SyncSource, error) {
	return SyncSource{ID: uuid.New(), ComicID: comicID, OwnerUserID: ownerID, SourceURL: sourceURL, SourceSite: site, ChaptersHint: hint, LastStatus: "idle"}, nil
}
func (r *fakeRepo) ListSyncSources(_ context.Context, _ uuid.UUID) ([]SyncSource, error) {
	return nil, nil
}
func (r *fakeRepo) GetSyncSource(_ context.Context, id uuid.UUID) (SyncSource, error) {
	return SyncSource{ID: id}, nil
}
func (r *fakeRepo) DeleteSyncSource(_ context.Context, _ uuid.UUID) error { return nil }
func (r *fakeRepo) UpdateSyncStatus(_ context.Context, _ uuid.UUID, _ string, _ *uuid.UUID, _ *string, _ bool) error {
	return nil
}
func (r *fakeRepo) UpdateSyncProgress(_ context.Context, _ uuid.UUID, _, _ int) error { return nil }
func (r *fakeRepo) SetSyncLastImport(_ context.Context, _, _ uuid.UUID) error         { return nil }

// ── fixtures ─────────────────────────────────────────────────────────

func newSvc() (*Service, *fakeRepo, *fakeMedia) {
	repo := newFakeRepo()
	media := &fakeMedia{assets: map[uuid.UUID]*mediaapi.Asset{}}
	return &Service{repo: repo, media: media}, repo, media
}

// ── tests ────────────────────────────────────────────────────────────

func TestPublishValidation(t *testing.T) {
	svc, repo, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	c, _ := svc.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "Test"})

	// no chapters → not publishable
	if _, err := svc.Publish(ctx, c.ID); err == nil {
		t.Fatal("empty comic should not publish")
	}
	// chapter with no pages → not publishable, names the chapter
	ch, _ := svc.CreateChapter(ctx, c.ID, "Ch1", 10)
	_, err := svc.Publish(ctx, c.ID)
	var np *NotPublishableError
	if !errors.As(err, &np) || len(np.Chapters) != 1 || np.Chapters[0].ID != ch.ID {
		t.Fatalf("expected NotPublishableError naming ch1, got %v", err)
	}
	// add a page → publishable
	repo.pages[uuid.New()] = &Page{ID: uuid.New(), ChapterID: ch.ID, AssetID: uuid.New(), SortOrder: 10}
	pub, err := svc.Publish(ctx, c.ID)
	if err != nil || pub.Status != StatusPublished {
		t.Fatalf("publish = %+v, %v", pub, err)
	}
}

func TestDraftVisibility(t *testing.T) {
	svc, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	reader := uuid.New()
	c, _ := svc.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "Draft"})

	// owner sees the draft
	if _, err := svc.GetComic(ctx, owner, c.ID); err != nil {
		t.Fatalf("owner draft: %v", err)
	}
	// reader gets 404 (existence never leaks)
	if _, err := svc.GetComic(ctx, reader, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("reader draft = %v, want ErrNotFound", err)
	}
	// once published, reader sees it
	_, _ = svc.repo.SetStatus(ctx, c.ID, StatusPublished)
	if _, err := svc.GetComic(ctx, reader, c.ID); err != nil {
		t.Fatalf("reader published: %v", err)
	}
}

func TestPageAssetValidation(t *testing.T) {
	svc, _, media := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	c, _ := svc.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "T"})
	ch, _ := svc.CreateChapter(ctx, c.ID, "Ch", 10)

	readyImg := media.put(owner, mediaapi.KindImage, mediaapi.StatusReady)
	video := media.put(owner, mediaapi.KindVideo, mediaapi.StatusReady)
	processing := media.put(owner, mediaapi.KindImage, mediaapi.StatusProcessing)
	othersImg := media.put(uuid.New(), mediaapi.KindImage, mediaapi.StatusReady)

	for _, bad := range []uuid.UUID{video, processing, othersImg, uuid.New()} {
		if _, err := svc.CreatePages(ctx, ch.ID, []PageInput{{AssetID: bad, SortOrder: 10}}); !errors.Is(err, ErrInvalidPageAsset) {
			t.Fatalf("bad asset %v = %v, want ErrInvalidPageAsset", bad, err)
		}
	}
	if _, err := svc.CreatePages(ctx, ch.ID, []PageInput{{AssetID: readyImg, SortOrder: 10}}); err != nil {
		t.Fatalf("ready image page: %v", err)
	}
}

func TestCoverAssetValidation(t *testing.T) {
	svc, _, media := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	video := media.put(owner, mediaapi.KindVideo, mediaapi.StatusReady)
	if _, err := svc.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "T", CoverAssetID: &video}); !errors.Is(err, ErrInvalidCoverAsset) {
		t.Fatalf("video cover = %v, want ErrInvalidCoverAsset", err)
	}
}

func TestProgressMembership(t *testing.T) {
	svc, _, _ := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	c, _ := svc.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "T"})
	ch, _ := svc.CreateChapter(ctx, c.ID, "Ch", 10)
	page, _ := svc.repo.CreatePage(ctx, ch.ID, uuid.New(), 10)

	// valid membership
	if err := svc.SaveProgress(ctx, owner, c.ID, ch.ID, &page.ID); err != nil {
		t.Fatalf("valid progress: %v", err)
	}
	// chapter not in this comic
	otherComic := uuid.New()
	if err := svc.SaveProgress(ctx, owner, otherComic, ch.ID, nil); !errors.Is(err, ErrInvalidProgressTarget) {
		t.Fatalf("foreign chapter = %v, want ErrInvalidProgressTarget", err)
	}
	// page not in this chapter
	otherCh, _ := svc.CreateChapter(ctx, c.ID, "Ch2", 20)
	if err := svc.SaveProgress(ctx, owner, c.ID, otherCh.ID, &page.ID); !errors.Is(err, ErrInvalidProgressTarget) {
		t.Fatalf("page in wrong chapter = %v, want ErrInvalidProgressTarget", err)
	}
}

func TestAssetDeletedConsumer(t *testing.T) {
	svc, repo, media := newSvc()
	ctx := context.Background()
	owner := uuid.New()
	asset := media.put(owner, mediaapi.KindImage, mediaapi.StatusReady)
	c, _ := svc.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "T", CoverAssetID: &asset})
	ch, _ := svc.CreateChapter(ctx, c.ID, "Ch", 10)
	page, _ := repo.CreatePage(ctx, ch.ID, asset, 10)

	if err := svc.HandleAssetDeleted(ctx, asset); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.pages[page.ID]; ok {
		t.Fatal("page referencing the deleted asset should be reaped")
	}
	if repo.comics[c.ID].CoverAssetID != nil {
		t.Fatal("cover referencing the deleted asset should be NULLed")
	}
	// idempotent + no-op on an unknown asset
	if err := svc.HandleAssetDeleted(ctx, uuid.New()); err != nil {
		t.Fatalf("idempotent handle: %v", err)
	}
}

func TestChapterSortOrder(t *testing.T) {
	cases := []struct {
		title string
		want  int
		ok    bool
	}{
		{"12", 120, true},      // scraper names its folders with the bare number
		{"12.5", 125, true},    // a .5 side-story sits between 12 and 13
		{"12,5", 125, true},    // comma decimal
		{"216-1", 2161, true},  // hyphen side-chapter must NOT collapse onto 216
		{"Chương 7", 70, true}, // hand-made zip
		{"Chapter 108", 1080, true},
		{"1", 10, true},
		{"", 0, false}, // no number → caller appends instead
		{"Ngoại truyện", 0, false},
		{"99999999999", 0, false}, // absurd: a digit run that isn't a chapter number
	}
	for _, c := range cases {
		got, ok := chapterSortOrder(c.title)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("chapterSortOrder(%q) = (%d, %v), want (%d, %v)", c.title, got, ok, c.want, c.ok)
		}
	}

	// The property the parallel scraper depends on: a chapter's slot is decided by
	// its title alone, so importing out of order still reads back in order.
	forward := []string{"1", "2", "3", "10", "10-5", "11"}
	reverse := []string{"11", "10-5", "10", "3", "2", "1"}
	slot := func(titles []string) []int {
		out := make([]int, len(titles))
		for i, ti := range titles {
			out[i], _ = chapterSortOrder(ti)
		}
		sort.Ints(out)
		return out
	}
	f, r := slot(forward), slot(reverse)
	if !slices.Equal(f, r) {
		t.Fatalf("arrival order changed the slots: %v vs %v", f, r)
	}
	if !sort.IntsAreSorted(f) || f[0] != 10 || f[len(f)-1] != 110 {
		t.Fatalf("unexpected slot sequence: %v", f)
	}
}
