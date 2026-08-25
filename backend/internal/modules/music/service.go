package music

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	musicapi "github.com/portal/backend/internal/modules/music/api"
	"github.com/portal/backend/internal/platform/server"
)

// Service holds the music business logic. Construct via the module.
type Service struct {
	repo   Repository
	media  MediaAPI
	events EventPublisher // optional: music:track_published on publish
}

type ListResult struct {
	Items      []Track
	NextCursor string
}

func (s *Service) CreateTrack(ctx context.Context, in CreateTrackInput) (Track, error) {
	in.Title = strings.TrimSpace(in.Title)
	if !validTitle(in.Title) {
		return Track{}, ErrValidation
	}
	if in.AudioAssetID != nil {
		if err := s.validateAudioAsset(ctx, *in.AudioAssetID, in.OwnerID); err != nil {
			return Track{}, ErrInvalidAudioAsset
		}
	}
	if in.CoverAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.CoverAssetID, in.OwnerID); err != nil {
			return Track{}, ErrInvalidCoverAsset
		}
	}
	return s.repo.CreateTrack(ctx, in)
}

func (s *Service) GetTrack(ctx context.Context, userID, id uuid.UUID) (Track, error) {
	t, err := s.repo.GetTrack(ctx, id)
	if err != nil {
		return Track{}, err
	}
	if t.Status != StatusPublished && t.OwnerID != userID {
		return Track{}, ErrNotFound
	}
	return t, nil
}

func (s *Service) ListPublished(ctx context.Context, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{}, cursor, limit, s.repo.ListPublished)
}

func (s *Service) ListOwn(ctx context.Context, ownerID uuid.UUID, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{OwnerID: ownerID}, cursor, limit, s.repo.ListOwn)
}

func (s *Service) list(ctx context.Context, in ListInput, cursor string, limit int, fn func(context.Context, ListInput) ([]Track, error)) (ListResult, error) {
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

func (s *Service) UpdateTrack(ctx context.Context, in UpdateTrackInput) (Track, error) {
	t, err := s.repo.GetTrack(ctx, in.ID)
	if err != nil {
		return Track{}, err
	}
	if in.Title != nil {
		title := strings.TrimSpace(*in.Title)
		if !validTitle(title) {
			return Track{}, ErrValidation
		}
		in.Title = &title
	}
	if in.SetAudio && in.AudioAssetID != nil {
		if err := s.validateAudioAsset(ctx, *in.AudioAssetID, t.OwnerID); err != nil {
			return Track{}, ErrInvalidAudioAsset
		}
	}
	if in.SetCover && in.CoverAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.CoverAssetID, t.OwnerID); err != nil {
			return Track{}, ErrInvalidCoverAsset
		}
	}
	return s.repo.UpdateTrack(ctx, in)
}

// Publish requires a ready audio asset owned by the creator, else ErrNotPublishable.
func (s *Service) Publish(ctx context.Context, id uuid.UUID) (Track, error) {
	t, err := s.repo.GetTrack(ctx, id)
	if err != nil {
		return Track{}, err
	}
	if t.AudioAssetID == nil {
		return Track{}, ErrNotPublishable
	}
	if err := s.validateAudioAsset(ctx, *t.AudioAssetID, t.OwnerID); err != nil {
		return Track{}, ErrNotPublishable
	}
	pub, err := s.repo.SetStatus(ctx, id, StatusPublished)
	if err != nil {
		return Track{}, err
	}
	s.emitPublished(ctx, pub)
	return pub, nil
}

func (s *Service) emitPublished(ctx context.Context, t Track) {
	if s.events == nil {
		return
	}
	ev := musicapi.TrackPublishedEvent{TrackID: t.ID.String(), OwnerUserID: t.OwnerID.String(), Title: t.Title}
	if err := s.events.Publish(ctx, musicapi.EventTrackPublished, ev); err != nil {
		log.Warn().Err(err).Str("track", t.ID.String()).Msg("music: published event failed")
	}
}

func (s *Service) Unpublish(ctx context.Context, id uuid.UUID) (Track, error) {
	return s.repo.SetStatus(ctx, id, StatusDraft)
}

func (s *Service) DeleteTrack(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteTrack(ctx, id)
}

func (s *Service) OwnerByTrack(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.repo.OwnerByTrack(ctx, id)
}

// HandleAssetDeleted reaps dangling references idempotently (best-effort).
func (s *Service) HandleAssetDeleted(ctx context.Context, assetID uuid.UUID) error {
	if err := s.repo.NullAudioByAsset(ctx, assetID); err != nil {
		return err
	}
	return s.repo.NullCoverByAsset(ctx, assetID)
}

// ── helpers ───────────────────────────────────────────────────────────

func (s *Service) validateAudioAsset(ctx context.Context, assetID, ownerID uuid.UUID) error {
	a, err := s.media.GetAsset(ctx, assetID)
	if err != nil {
		return err
	}
	if a == nil || a.Kind != mediaapi.KindAudio || a.Status != mediaapi.StatusReady || a.OwnerID != ownerID {
		return ErrValidation
	}
	return nil
}

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

func encodeCursor(t Track) string {
	return server.EncodeCursor(t.UpdatedAt.UTC().Format(time.RFC3339Nano), t.ID)
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
