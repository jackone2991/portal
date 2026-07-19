package movie

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// Errors surfaced to the handler. A hidden (draft, non-owner) row resolves to
// NotFound → 404, never 403 (don't leak existence).
var (
	ErrNotFound           = errors.New("movie: not found")
	ErrInvalidVideoAsset  = errors.New("movie: invalid video asset")
	ErrInvalidPosterAsset = errors.New("movie: invalid poster asset")
	ErrNotPublishable     = errors.New("movie: not publishable")
	ErrValidation         = errors.New("movie: validation error")
	ErrBadCursor          = errors.New("movie: invalid cursor")
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	maxTitleLen     = 200
	defaultLimit    = 30
	maxLimit        = 50
)

// Movie is the module's internal movie record.
type Movie struct {
	ID            uuid.UUID
	OwnerID       uuid.UUID
	Title         string
	Description   *string
	VideoAssetID  *uuid.UUID
	PosterAssetID *uuid.UUID
	ReleaseYear   *int
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ── inputs ────────────────────────────────────────────────────────────

type CreateMovieInput struct {
	OwnerID       uuid.UUID
	Title         string
	Description   *string
	VideoAssetID  *uuid.UUID
	PosterAssetID *uuid.UUID
	ReleaseYear   *int
}

type UpdateMovieInput struct {
	ID            uuid.UUID
	Title         *string
	Description   *string
	SetVideo      bool
	VideoAssetID  *uuid.UUID
	SetPoster     bool
	PosterAssetID *uuid.UUID
	ReleaseYear   *int
}

type ListInput struct {
	OwnerID  uuid.UUID // for the "mine" listing
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// Repository is the persistence surface. The sqlc-backed adapter implements it.
type Repository interface {
	CreateMovie(ctx context.Context, in CreateMovieInput) (Movie, error)
	GetMovie(ctx context.Context, id uuid.UUID) (Movie, error)
	ListPublished(ctx context.Context, in ListInput) ([]Movie, error)
	ListOwn(ctx context.Context, in ListInput) ([]Movie, error)
	UpdateMovie(ctx context.Context, in UpdateMovieInput) (Movie, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) (Movie, error)
	DeleteMovie(ctx context.Context, id uuid.UUID) error
	OwnerByMovie(ctx context.Context, id uuid.UUID) (uuid.UUID, error)

	// media:asset_deleted consumer
	NullVideoByAsset(ctx context.Context, assetID uuid.UUID) error
	NullPosterByAsset(ctx context.Context, assetID uuid.UUID) error
}

// MediaAPI is the slice of media/api movie needs to validate asset references.
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
}

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
