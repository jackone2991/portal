package movierepo

// Adapter bridges the sqlc-generated Queries to movie.Repository. cmd/api and
// cmd/worker construct it with NewAdapter(conn); all pgtype ↔ domain juggling
// lives here. tenant_id is populated by the movies.tenant_id column DEFAULT
// (RequireTenant sets app.current_tenant) — never written from Go.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/movie"
)

type Adapter struct{ q *Queries }

// NewAdapter builds the adapter over any pgx-compatible handle (pool or the
// context-aware platform/db.Conn).
func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

var _ movie.Repository = (*Adapter)(nil)

func (a *Adapter) CreateMovie(ctx context.Context, in movie.CreateMovieInput) (movie.Movie, error) {
	row, err := a.q.CreateMovie(ctx, CreateMovieParams{
		OwnerUserID:   pgUUID(in.OwnerID),
		Title:         in.Title,
		Description:   in.Description,
		VideoAssetID:  optUUID(in.VideoAssetID),
		PosterAssetID: optUUID(in.PosterAssetID),
		ReleaseYear:   optI32(in.ReleaseYear),
	})
	if err != nil {
		return movie.Movie{}, err
	}
	return toMovie(row), nil
}

func (a *Adapter) GetMovie(ctx context.Context, id uuid.UUID) (movie.Movie, error) {
	row, err := a.q.GetMovie(ctx, pgUUID(id))
	if err != nil {
		return movie.Movie{}, mapNotFound(err)
	}
	return toMovie(row), nil
}

func (a *Adapter) ListPublished(ctx context.Context, in movie.ListInput) ([]movie.Movie, error) {
	rows, err := a.q.ListPublishedMovies(ctx, ListPublishedMoviesParams{
		CursorUpdatedAt: optTS(in.CursorAt), CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	return toMovies(rows), nil
}

func (a *Adapter) ListOwn(ctx context.Context, in movie.ListInput) ([]movie.Movie, error) {
	rows, err := a.q.ListOwnMovies(ctx, ListOwnMoviesParams{
		OwnerUserID: pgUUID(in.OwnerID), CursorUpdatedAt: optTS(in.CursorAt), CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	return toMovies(rows), nil
}

func (a *Adapter) UpdateMovie(ctx context.Context, in movie.UpdateMovieInput) (movie.Movie, error) {
	row, err := a.q.UpdateMovie(ctx, UpdateMovieParams{
		Title:         in.Title,
		Description:   in.Description,
		SetVideo:      in.SetVideo,
		VideoAssetID:  optUUID(in.VideoAssetID),
		SetPoster:     in.SetPoster,
		PosterAssetID: optUUID(in.PosterAssetID),
		ReleaseYear:   optI32(in.ReleaseYear),
		ID:            pgUUID(in.ID),
	})
	if err != nil {
		return movie.Movie{}, mapNotFound(err)
	}
	return toMovie(row), nil
}

func (a *Adapter) SetStatus(ctx context.Context, id uuid.UUID, status string) (movie.Movie, error) {
	row, err := a.q.UpdateMovieStatus(ctx, UpdateMovieStatusParams{ID: pgUUID(id), Status: status})
	if err != nil {
		return movie.Movie{}, mapNotFound(err)
	}
	return toMovie(row), nil
}

func (a *Adapter) DeleteMovie(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteMovie(ctx, pgUUID(id))
}

func (a *Adapter) OwnerByMovie(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row, err := a.q.GetMovieOwner(ctx, pgUUID(id))
	if err != nil {
		return uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row), nil
}

func (a *Adapter) NullVideoByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.NullVideoByAsset(ctx, pgUUID(assetID))
}

func (a *Adapter) NullPosterByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.NullPosterByAsset(ctx, pgUUID(assetID))
}

// ── mapping helpers ────────────────────────────────────────────────────

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return movie.ErrNotFound
	}
	return err
}

func toMovies(rows []Movie) []movie.Movie {
	out := make([]movie.Movie, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMovie(r))
	}
	return out
}

func toMovie(r Movie) movie.Movie {
	return movie.Movie{
		ID:            uuidFrom(r.ID),
		OwnerID:       uuidFrom(r.OwnerUserID),
		Title:         r.Title,
		Description:   r.Description,
		VideoAssetID:  uuidPtr(r.VideoAssetID),
		PosterAssetID: uuidPtr(r.PosterAssetID),
		ReleaseYear:   i32ToIntP(r.ReleaseYear),
		Status:        r.Status,
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

func optUUID(p *uuid.UUID) pgtype.UUID {
	if p == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *p, Valid: true}
}

func optTS(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func i32ToIntP(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}

func optI32(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}
