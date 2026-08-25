package story

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	storyapi "github.com/portal/backend/internal/modules/story/api"
	"github.com/portal/backend/internal/platform/server"
)

// Service holds the story business logic. Construct via the module.
type Service struct {
	repo   Repository
	media  MediaAPI
	events EventPublisher // optional: story:published on publish
}

type ListResult struct {
	Items      []Story
	NextCursor string
}

// ══ Stories ══════════════════════════════════════════════════════════════

func (s *Service) CreateStory(ctx context.Context, in CreateStoryInput) (Story, error) {
	in.Title = strings.TrimSpace(in.Title)
	if !validTitle(in.Title) {
		return Story{}, ErrValidation
	}
	if in.CoverAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.CoverAssetID, in.OwnerID); err != nil {
			return Story{}, ErrInvalidCoverAsset
		}
	}
	return s.repo.CreateStory(ctx, in)
}

func (s *Service) GetStory(ctx context.Context, userID, id uuid.UUID) (Story, error) {
	st, err := s.repo.GetStory(ctx, id)
	if err != nil {
		return Story{}, err
	}
	if st.Status != StatusPublished && st.OwnerID != userID {
		return Story{}, ErrNotFound
	}
	return st, nil
}

func (s *Service) ListPublished(ctx context.Context, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{}, cursor, limit, s.repo.ListPublished)
}

func (s *Service) ListOwn(ctx context.Context, ownerID uuid.UUID, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{OwnerID: ownerID}, cursor, limit, s.repo.ListOwn)
}

func (s *Service) list(ctx context.Context, in ListInput, cursor string, limit int, fn func(context.Context, ListInput) ([]Story, error)) (ListResult, error) {
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

func (s *Service) UpdateStory(ctx context.Context, in UpdateStoryInput) (Story, error) {
	st, err := s.repo.GetStory(ctx, in.ID)
	if err != nil {
		return Story{}, err
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if !validTitle(t) {
			return Story{}, ErrValidation
		}
		in.Title = &t
	}
	if in.SetCover && in.CoverAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.CoverAssetID, st.OwnerID); err != nil {
			return Story{}, ErrInvalidCoverAsset
		}
	}
	return s.repo.UpdateStory(ctx, in)
}

// Publish validates ≥1 chapter and every chapter has a non-empty body, else
// NotPublishableError listing the offenders.
func (s *Service) Publish(ctx context.Context, id uuid.UUID) (Story, error) {
	n, err := s.repo.CountChapters(ctx, id)
	if err != nil {
		return Story{}, err
	}
	empties, err := s.repo.EmptyChapters(ctx, id)
	if err != nil {
		return Story{}, err
	}
	if n == 0 || len(empties) > 0 {
		return Story{}, &NotPublishableError{Chapters: empties}
	}
	pub, err := s.repo.SetStatus(ctx, id, StatusPublished)
	if err != nil {
		return Story{}, err
	}
	s.emitPublished(ctx, pub)
	return pub, nil
}

func (s *Service) emitPublished(ctx context.Context, st Story) {
	if s.events == nil {
		return
	}
	ev := storyapi.StoryPublishedEvent{StoryID: st.ID.String(), OwnerUserID: st.OwnerID.String(), Title: st.Title}
	if err := s.events.Publish(ctx, storyapi.EventStoryPublished, ev); err != nil {
		log.Warn().Err(err).Str("story", st.ID.String()).Msg("story: published event failed")
	}
}

func (s *Service) Unpublish(ctx context.Context, id uuid.UUID) (Story, error) {
	return s.repo.SetStatus(ctx, id, StatusDraft)
}

func (s *Service) DeleteStory(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteStory(ctx, id)
}

func (s *Service) OwnerByStory(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.repo.OwnerByStory(ctx, id)
}

func (s *Service) HandleAssetDeleted(ctx context.Context, assetID uuid.UUID) error {
	return s.repo.NullCoverByAsset(ctx, assetID)
}

// ══ Chapters ═════════════════════════════════════════════════════════════

func (s *Service) CreateChapter(ctx context.Context, storyID uuid.UUID, title, bodyMd string, sortOrder int) (Chapter, error) {
	title = strings.TrimSpace(title)
	if title == "" || utf8.RuneCountInString(title) > maxTitleLen {
		return Chapter{}, ErrValidation
	}
	if utf8.RuneCountInString(bodyMd) > maxBodyLen {
		return Chapter{}, ErrValidation
	}
	return s.repo.CreateChapter(ctx, storyID, title, bodyMd, sortOrder)
}

func (s *Service) ListChapters(ctx context.Context, storyID uuid.UUID) ([]Chapter, error) {
	return s.repo.ListChapters(ctx, storyID)
}

// ChaptersVisible enforces published-or-owner on the story before returning the
// reader payload (draft invisibility → 404).
func (s *Service) ChaptersVisible(ctx context.Context, userID, storyID uuid.UUID) ([]Chapter, error) {
	if _, err := s.GetStory(ctx, userID, storyID); err != nil {
		return nil, err
	}
	return s.repo.ListChapters(ctx, storyID)
}

func (s *Service) UpdateChapter(ctx context.Context, id uuid.UUID, title, bodyMd *string) (Chapter, error) {
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" || utf8.RuneCountInString(t) > maxTitleLen {
			return Chapter{}, ErrValidation
		}
		title = &t
	}
	if bodyMd != nil && utf8.RuneCountInString(*bodyMd) > maxBodyLen {
		return Chapter{}, ErrValidation
	}
	return s.repo.UpdateChapter(ctx, id, title, bodyMd)
}

func (s *Service) DeleteChapter(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteChapter(ctx, id)
}

func (s *Service) ReorderChapters(ctx context.Context, storyID uuid.UUID, orderedIDs []uuid.UUID) error {
	return s.repo.ReorderChapters(ctx, storyID, orderedIDs)
}

// ── owner resolution (for RequireOwnerOrPermission extractors) ──────────

func (s *Service) OwnerByChapter(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	owner, _, err := s.repo.OwnerAndStoryByChapter(ctx, id)
	return owner, err
}

// ── helpers ───────────────────────────────────────────────────────────

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

func encodeCursor(st Story) string {
	return server.EncodeCursor(st.UpdatedAt.UTC().Format(time.RFC3339Nano), st.ID)
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	key, id, err := server.DecodeCursor(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, key)
	if err != nil {
		return time.Time{}, uuid.Nil, server.ErrBadCursor
	}
	return at, id, nil
}
