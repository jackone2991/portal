package music

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

var (
	ErrNotFound          = errors.New("music: not found")
	ErrInvalidAudioAsset = errors.New("music: invalid audio asset")
	ErrInvalidCoverAsset = errors.New("music: invalid cover asset")
	ErrNotPublishable    = errors.New("music: not publishable")
	ErrValidation        = errors.New("music: validation error")
	ErrBadCursor         = errors.New("music: invalid cursor")
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	maxTitleLen     = 200
	defaultLimit    = 30
	maxLimit        = 50
)

// Track is the module's internal record of one audio track.
type Track struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	Title        string
	Artist       *string
	Album        *string
	Description  *string
	AudioAssetID *uuid.UUID
	CoverAssetID *uuid.UUID
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateTrackInput struct {
	OwnerID      uuid.UUID
	Title        string
	Artist       *string
	Album        *string
	Description  *string
	AudioAssetID *uuid.UUID
	CoverAssetID *uuid.UUID
}

type UpdateTrackInput struct {
	ID           uuid.UUID
	Title        *string
	SetArtist    bool
	Artist       *string
	SetAlbum     bool
	Album        *string
	Description  *string
	SetAudio     bool
	AudioAssetID *uuid.UUID
	SetCover     bool
	CoverAssetID *uuid.UUID
}

type ListInput struct {
	OwnerID  uuid.UUID
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// Repository is the persistence surface. The sqlc-backed adapter implements it.
type Repository interface {
	CreateTrack(ctx context.Context, in CreateTrackInput) (Track, error)
	GetTrack(ctx context.Context, id uuid.UUID) (Track, error)
	ListPublished(ctx context.Context, in ListInput) ([]Track, error)
	ListOwn(ctx context.Context, in ListInput) ([]Track, error)
	UpdateTrack(ctx context.Context, in UpdateTrackInput) (Track, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) (Track, error)
	DeleteTrack(ctx context.Context, id uuid.UUID) error
	OwnerByTrack(ctx context.Context, id uuid.UUID) (uuid.UUID, error)

	// media:asset_deleted consumer
	NullAudioByAsset(ctx context.Context, assetID uuid.UUID) error
	NullCoverByAsset(ctx context.Context, assetID uuid.UUID) error
}

// MediaAPI is the slice of media/api music needs to validate asset references.
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
}

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
