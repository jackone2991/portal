package music

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

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

	// Bulk zip import (0038).
	CreateImport(ctx context.Context, ownerID uuid.UUID) (ImportJob, error)
	GetImport(ctx context.Context, id uuid.UUID) (ImportJob, error)
	ListImports(ctx context.Context, ownerID uuid.UUID, limit int) ([]ImportJob, error)
	SetImportUpload(ctx context.Context, id uuid.UUID, key string) (ImportJob, error)
	StartImport(ctx context.Context, id uuid.UUID, total int) error
	FinishImport(ctx context.Context, id uuid.UUID, status string, succeeded, failed int, report, errMsg string) error
}

// ImportJob is one bulk-import run. `Report` is the raw JSON array the client
// renders per file; it stays opaque here because the shape belongs to the
// importer, not to storage.
type ImportJob struct {
	ID          uuid.UUID
	OwnerUserID uuid.UUID
	Status      string // pending | uploaded | processing | done | failed
	UploadRef   string // storage key of the zip; "" until it is uploaded
	Total       int
	Succeeded   int
	Failed      int
	Report      []byte
	Error       string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// MediaAPI is the slice of media/api music needs: validating asset references,
// and — for the zip import — creating an audio asset from bytes.
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
	// Ingest runs the same three-step pipeline a browser upload does. Only the
	// worker side wires it; the API server never imports.
	Ingest(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error)
}

// Storage is the object store the zip lands in. API side puts, worker side gets
// and deletes.
type Storage interface {
	Put(ctx context.Context, key string, body io.Reader, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// Enqueuer schedules the music:import_zip task. *asynq.Client satisfies it.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
