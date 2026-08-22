package comic

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	comicapi "github.com/portal/backend/internal/modules/comic/api"
	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// Service holds the comic business logic. Construct via the module.
type Service struct {
	repo   Repository
	media  MediaAPI
	events EventPublisher // optional: comic:chapter_published on publish (P1.9)

	// zip-import (P1.7) — nil on the API side except store/enqueue; the worker
	// side fills store + media.IngestImage + runInTenant.
	store       ObjectStore
	enqueue     Enqueuer
	runInTenant RunInTenant

	// external-source sync (P1.8) — API side only; nil disables the feature.
	scraper ScraperClient
}

// ReaderPage is one page in the reader payload (P0.3): id + asset + dims (from
// the media asset, for the CLS-safe <img>). Width/Height are nil if the asset is
// missing (the reader shows a per-page error tile).
type ReaderPage struct {
	PageID  uuid.UUID
	AssetID uuid.UUID
	Width   *int
	Height  *int
}

type ListResult struct {
	Items      []Comic
	NextCursor string
}

// PageInput is one entry of a batch page create.
type PageInput struct {
	AssetID   uuid.UUID
	SortOrder int
}

// ══ Comics ══════════════════════════════════════════════════════════════

func (s *Service) CreateComic(ctx context.Context, in CreateComicInput) (Comic, error) {
	in.Title = strings.TrimSpace(in.Title)
	if !validTitle(in.Title) {
		return Comic{}, ErrValidation
	}
	if in.CoverAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.CoverAssetID, in.OwnerID); err != nil {
			return Comic{}, ErrInvalidCoverAsset
		}
	}
	return s.repo.CreateComic(ctx, in)
}

// GetComic enforces published-or-owner visibility (a draft is 404 to others).
func (s *Service) GetComic(ctx context.Context, userID, id uuid.UUID) (Comic, error) {
	c, err := s.repo.GetComic(ctx, id)
	if err != nil {
		return Comic{}, err
	}
	if c.Status != StatusPublished && c.OwnerID != userID {
		return Comic{}, ErrNotFound
	}
	return c, nil
}

func (s *Service) ListPublished(ctx context.Context, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{}, cursor, limit, s.repo.ListPublished)
}

func (s *Service) ListOwn(ctx context.Context, ownerID uuid.UUID, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{OwnerID: ownerID}, cursor, limit, s.repo.ListOwn)
}

func (s *Service) list(ctx context.Context, in ListInput, cursor string, limit int, fn func(context.Context, ListInput) ([]Comic, error)) (ListResult, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return ListResult{}, ErrBadCursor
		}
		in.CursorAt, in.CursorID = at, id
	}
	in.Limit = limit + 1
	rows, err := fn(ctx, in)
	if err != nil {
		return ListResult{}, err
	}
	var res ListResult
	if len(rows) > limit {
		res.NextCursor = encodeCursor(rows[limit-1])
		rows = rows[:limit]
	}
	res.Items = rows
	return res, nil
}

func (s *Service) UpdateComic(ctx context.Context, in UpdateComicInput) (Comic, error) {
	c, err := s.repo.GetComic(ctx, in.ID)
	if err != nil {
		return Comic{}, err
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if !validTitle(t) {
			return Comic{}, ErrValidation
		}
		in.Title = &t
	}
	if in.SetCover && in.CoverAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.CoverAssetID, c.OwnerID); err != nil {
			return Comic{}, ErrInvalidCoverAsset
		}
	}
	return s.repo.UpdateComic(ctx, in)
}

// Publish validates ≥1 chapter and every chapter ≥1 page, else NotPublishableError.
func (s *Service) Publish(ctx context.Context, id uuid.UUID) (Comic, error) {
	n, err := s.repo.CountChapters(ctx, id)
	if err != nil {
		return Comic{}, err
	}
	empties, err := s.repo.ChaptersWithoutPages(ctx, id)
	if err != nil {
		return Comic{}, err
	}
	if n == 0 || len(empties) > 0 {
		return Comic{}, &NotPublishableError{Chapters: empties}
	}
	c, err := s.repo.SetStatus(ctx, id, StatusPublished)
	if err != nil {
		return Comic{}, err
	}
	s.emitChaptersPublished(ctx, c)
	return c, nil
}

// emitChaptersPublished fires comic:chapter_published once per chapter after a
// publish (SPEC-02 P1.9 — life-stream producer #2; the journal stream projection
// keys on chapter_id). Emit-only and best-effort: a nil publisher or a publish
// error never fails the already-committed publish. Redelivery/re-publish is
// idempotent downstream via the stream's (source, event, ref_id) unique.
func (s *Service) emitChaptersPublished(ctx context.Context, c Comic) {
	if s.events == nil {
		return
	}
	chapters, err := s.repo.ListChapters(ctx, c.ID)
	if err != nil {
		log.Warn().Err(err).Str("comic", c.ID.String()).Msg("comic: list chapters for publish event failed")
		return
	}
	for _, ch := range chapters {
		ev := comicapi.ChapterPublishedEvent{
			ComicID:     c.ID.String(),
			ChapterID:   ch.ID.String(),
			OwnerUserID: c.OwnerID.String(),
			Title:       ch.Title,
		}
		if err := s.events.Publish(ctx, comicapi.EventChapterPublished, ev); err != nil {
			log.Warn().Err(err).Str("chapter", ch.ID.String()).Msg("comic: chapter_published publish failed")
		}
	}
}

func (s *Service) Unpublish(ctx context.Context, id uuid.UUID) (Comic, error) {
	return s.repo.SetStatus(ctx, id, StatusDraft)
}

func (s *Service) DeleteComic(ctx context.Context, id uuid.UUID) error {
	// Capture chapters + owner before the cascade delete so the stream-removal
	// events can be emitted (P1.9). Only published chapters have a stream card;
	// emitting for the rest is a harmless no-op downstream (idempotent).
	var chapters []Chapter
	var owner uuid.UUID
	if s.events != nil {
		chapters, _ = s.repo.ListChapters(ctx, id)
		owner, _ = s.repo.OwnerByComic(ctx, id)
	}
	if err := s.repo.DeleteComic(ctx, id); err != nil {
		return err
	}
	for _, ch := range chapters {
		s.emitChapterDeleted(ctx, id, ch.ID, owner)
	}
	return nil
}

// ══ Chapters ════════════════════════════════════════════════════════════

func (s *Service) CreateChapter(ctx context.Context, comicID uuid.UUID, title string, sortOrder int) (Chapter, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleLen {
		return Chapter{}, ErrValidation
	}
	return s.repo.CreateChapter(ctx, comicID, title, sortOrder)
}

func (s *Service) ListChapters(ctx context.Context, comicID uuid.UUID) ([]Chapter, error) {
	return s.repo.ListChapters(ctx, comicID)
}

func (s *Service) UpdateChapter(ctx context.Context, id uuid.UUID, title *string) (Chapter, error) {
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" || utf8.RuneCountInString(t) > maxTitleLen {
			return Chapter{}, ErrValidation
		}
		title = &t
	}
	return s.repo.UpdateChapter(ctx, id, title)
}

func (s *Service) DeleteChapter(ctx context.Context, id uuid.UUID) error {
	var owner, comicID uuid.UUID
	if s.events != nil {
		owner, comicID, _ = s.repo.OwnerAndComicByChapter(ctx, id)
	}
	if err := s.repo.DeleteChapter(ctx, id); err != nil {
		return err
	}
	s.emitChapterDeleted(ctx, comicID, id, owner)
	return nil
}

// emitChapterDeleted fires comic:chapter_deleted so the stream drops the
// published-chapter card (P1.9). Emit-only, best-effort, idempotent downstream.
func (s *Service) emitChapterDeleted(ctx context.Context, comicID, chapterID, owner uuid.UUID) {
	if s.events == nil {
		return
	}
	ev := comicapi.ChapterDeletedEvent{
		ComicID:     comicID.String(),
		ChapterID:   chapterID.String(),
		OwnerUserID: owner.String(),
	}
	if err := s.events.Publish(ctx, comicapi.EventChapterDeleted, ev); err != nil {
		log.Warn().Err(err).Str("chapter", chapterID.String()).Msg("comic: chapter_deleted publish failed")
	}
}

func (s *Service) ReorderChapters(ctx context.Context, comicID uuid.UUID, orderedIDs []uuid.UUID) error {
	return s.repo.ReorderChapters(ctx, comicID, orderedIDs)
}

// ══ Pages ═══════════════════════════════════════════════════════════════

// CreatePages validates every asset (ready image owned by the comic's creator)
// then inserts the pages (P0.1). Any invalid asset → ErrInvalidPageAsset, no rows.
func (s *Service) CreatePages(ctx context.Context, chapterID uuid.UUID, items []PageInput) ([]Page, error) {
	owner, _, err := s.repo.OwnerAndComicByChapter(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if err := s.validateImageAsset(ctx, it.AssetID, owner); err != nil {
			return nil, ErrInvalidPageAsset
		}
	}
	out := make([]Page, 0, len(items))
	for _, it := range items {
		p, err := s.repo.CreatePage(ctx, chapterID, it.AssetID, it.SortOrder)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Service) DeletePage(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeletePage(ctx, id)
}

func (s *Service) ReorderPages(ctx context.Context, chapterID uuid.UUID, orderedIDs []uuid.UUID) error {
	return s.repo.ReorderPages(ctx, chapterID, orderedIDs)
}

// ReaderPagesVisible enforces published-or-owner on the chapter's comic before
// returning the reader payload (draft invisibility → 404, P0.5).
func (s *Service) ReaderPagesVisible(ctx context.Context, userID, chapterID uuid.UUID) ([]ReaderPage, error) {
	comicID, err := s.repo.ChapterComic(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	if _, err := s.GetComic(ctx, userID, comicID); err != nil {
		return nil, err
	}
	return s.ReaderPages(ctx, chapterID)
}

// ReaderPages returns the chapter's pages with media dims for the reader (P0.3).
func (s *Service) ReaderPages(ctx context.Context, chapterID uuid.UUID) ([]ReaderPage, error) {
	pages, err := s.repo.ListPages(ctx, chapterID)
	if err != nil {
		return nil, err
	}
	out := make([]ReaderPage, 0, len(pages))
	for _, p := range pages {
		rp := ReaderPage{PageID: p.ID, AssetID: p.AssetID}
		if a, err := s.media.GetAsset(ctx, p.AssetID); err == nil && a != nil {
			rp.Width, rp.Height = a.Width, a.Height
		}
		out = append(out, rp)
	}
	return out, nil
}

// ══ Reading progress (P0.4) ═════════════════════════════════════════════

// SaveProgress validates chapter/page membership then upserts (keyed by page_id).
func (s *Service) SaveProgress(ctx context.Context, userID, comicID, chapterID uuid.UUID, pageID *uuid.UUID) error {
	cc, err := s.repo.ChapterComic(ctx, chapterID)
	if err != nil {
		return ErrInvalidProgressTarget
	}
	if cc != comicID {
		return ErrInvalidProgressTarget
	}
	if pageID != nil {
		pc, pcomic, err := s.repo.PageMembership(ctx, *pageID)
		if err != nil {
			return ErrInvalidProgressTarget
		}
		if pc != chapterID || pcomic != comicID {
			return ErrInvalidProgressTarget
		}
	}
	return s.repo.UpsertProgress(ctx, userID, comicID, chapterID, pageID)
}

func (s *Service) GetProgress(ctx context.Context, userID, comicID uuid.UUID) (Progress, error) {
	return s.repo.GetProgress(ctx, userID, comicID)
}

// ══ owner resolution (for RequireOwnerOrPermission extractors) ══════════

func (s *Service) OwnerByComic(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.repo.OwnerByComic(ctx, id)
}

func (s *Service) OwnerByChapter(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	owner, _, err := s.repo.OwnerAndComicByChapter(ctx, id)
	return owner, err
}

func (s *Service) OwnerByPage(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	owner, _, err := s.repo.OwnerByPage(ctx, id)
	return owner, err
}

// ══ media:asset_deleted consumer (P0.6) ═════════════════════════════════

// HandleAssetDeleted reaps dangling references idempotently (best-effort).
func (s *Service) HandleAssetDeleted(ctx context.Context, assetID uuid.UUID) error {
	if err := s.repo.DeletePagesByAsset(ctx, assetID); err != nil {
		return err
	}
	return s.repo.NullCoverByAsset(ctx, assetID)
}

// ── helpers ───────────────────────────────────────────────────────────

// validateImageAsset enforces P0.1: exists, kind image, status ready, owned by
// ownerID. Any failure returns a generic error the caller maps to the right
// Problem (invalid-cover-asset / invalid-page-asset).
func (s *Service) validateImageAsset(ctx context.Context, assetID, ownerID uuid.UUID) error {
	a, err := s.media.GetAsset(ctx, assetID)
	if err != nil {
		return err
	}
	if a == nil || a.Kind != mediaapi.KindImage || a.Status != mediaapi.StatusReady || a.OwnerID != ownerID {
		return ErrValidation
	}
	return nil
}

func validTitle(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= maxTitleLen
}

func encodeCursor(c Comic) string {
	raw := c.UpdatedAt.UTC().Format(time.RFC3339Nano) + "|" + c.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("comic: malformed cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return at, id, nil
}
