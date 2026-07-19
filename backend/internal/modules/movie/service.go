package movie

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	movieapi "github.com/portal/backend/internal/modules/movie/api"
)

// Service holds the movie business logic. Construct via the module.
type Service struct {
	repo   Repository
	media  MediaAPI
	events EventPublisher // optional: movie:published on publish
}

type ListResult struct {
	Items      []Movie
	NextCursor string
}

func (s *Service) CreateMovie(ctx context.Context, in CreateMovieInput) (Movie, error) {
	in.Title = strings.TrimSpace(in.Title)
	if !validTitle(in.Title) {
		return Movie{}, ErrValidation
	}
	if in.VideoAssetID != nil {
		if err := s.validateVideoAsset(ctx, *in.VideoAssetID, in.OwnerID); err != nil {
			return Movie{}, ErrInvalidVideoAsset
		}
	}
	if in.PosterAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.PosterAssetID, in.OwnerID); err != nil {
			return Movie{}, ErrInvalidPosterAsset
		}
	}
	return s.repo.CreateMovie(ctx, in)
}

// GetMovie enforces published-or-owner visibility (a draft is 404 to others).
func (s *Service) GetMovie(ctx context.Context, userID, id uuid.UUID) (Movie, error) {
	m, err := s.repo.GetMovie(ctx, id)
	if err != nil {
		return Movie{}, err
	}
	if m.Status != StatusPublished && m.OwnerID != userID {
		return Movie{}, ErrNotFound
	}
	return m, nil
}

func (s *Service) ListPublished(ctx context.Context, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{}, cursor, limit, s.repo.ListPublished)
}

func (s *Service) ListOwn(ctx context.Context, ownerID uuid.UUID, cursor string, limit int) (ListResult, error) {
	return s.list(ctx, ListInput{OwnerID: ownerID}, cursor, limit, s.repo.ListOwn)
}

func (s *Service) list(ctx context.Context, in ListInput, cursor string, limit int, fn func(context.Context, ListInput) ([]Movie, error)) (ListResult, error) {
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

func (s *Service) UpdateMovie(ctx context.Context, in UpdateMovieInput) (Movie, error) {
	m, err := s.repo.GetMovie(ctx, in.ID)
	if err != nil {
		return Movie{}, err
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if !validTitle(t) {
			return Movie{}, ErrValidation
		}
		in.Title = &t
	}
	if in.SetVideo && in.VideoAssetID != nil {
		if err := s.validateVideoAsset(ctx, *in.VideoAssetID, m.OwnerID); err != nil {
			return Movie{}, ErrInvalidVideoAsset
		}
	}
	if in.SetPoster && in.PosterAssetID != nil {
		if err := s.validateImageAsset(ctx, *in.PosterAssetID, m.OwnerID); err != nil {
			return Movie{}, ErrInvalidPosterAsset
		}
	}
	return s.repo.UpdateMovie(ctx, in)
}

// Publish requires a ready video asset owned by the creator, else ErrNotPublishable.
func (s *Service) Publish(ctx context.Context, id uuid.UUID) (Movie, error) {
	m, err := s.repo.GetMovie(ctx, id)
	if err != nil {
		return Movie{}, err
	}
	if m.VideoAssetID == nil {
		return Movie{}, ErrNotPublishable
	}
	if err := s.validateVideoAsset(ctx, *m.VideoAssetID, m.OwnerID); err != nil {
		return Movie{}, ErrNotPublishable
	}
	pub, err := s.repo.SetStatus(ctx, id, StatusPublished)
	if err != nil {
		return Movie{}, err
	}
	s.emitPublished(ctx, pub)
	return pub, nil
}

// emitPublished fires movie:published (emit-only, best-effort — a nil publisher
// or a publish error never fails the already-committed publish).
func (s *Service) emitPublished(ctx context.Context, m Movie) {
	if s.events == nil {
		return
	}
	ev := movieapi.MoviePublishedEvent{MovieID: m.ID.String(), OwnerUserID: m.OwnerID.String(), Title: m.Title}
	if err := s.events.Publish(ctx, movieapi.EventMoviePublished, ev); err != nil {
		log.Warn().Err(err).Str("movie", m.ID.String()).Msg("movie: published event failed")
	}
}

func (s *Service) Unpublish(ctx context.Context, id uuid.UUID) (Movie, error) {
	return s.repo.SetStatus(ctx, id, StatusDraft)
}

func (s *Service) DeleteMovie(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteMovie(ctx, id)
}

func (s *Service) OwnerByMovie(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.repo.OwnerByMovie(ctx, id)
}

// HandleAssetDeleted reaps dangling references idempotently (best-effort). A
// deleted video also unpublishes the movie (query-side).
func (s *Service) HandleAssetDeleted(ctx context.Context, assetID uuid.UUID) error {
	if err := s.repo.NullVideoByAsset(ctx, assetID); err != nil {
		return err
	}
	return s.repo.NullPosterByAsset(ctx, assetID)
}

// ── helpers ───────────────────────────────────────────────────────────

// validateVideoAsset: exists, kind video, status ready, owned by ownerID.
func (s *Service) validateVideoAsset(ctx context.Context, assetID, ownerID uuid.UUID) error {
	a, err := s.media.GetAsset(ctx, assetID)
	if err != nil {
		return err
	}
	if a == nil || a.Kind != mediaapi.KindVideo || a.Status != mediaapi.StatusReady || a.OwnerID != ownerID {
		return ErrValidation
	}
	return nil
}

// validateImageAsset: exists, kind image, status ready, owned (for the poster).
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

func encodeCursor(m Movie) string {
	raw := m.UpdatedAt.UTC().Format(time.RFC3339Nano) + "|" + m.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("movie: malformed cursor")
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
