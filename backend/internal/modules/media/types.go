package media

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	"github.com/portal/backend/internal/modules/media/worker"
)

// Errors surfaced to the handler.
var (
	ErrNotFound     = errors.New("media: asset not found")
	ErrForbidden    = errors.New("media: not the asset owner")
	ErrNotReady     = errors.New("media: asset not ready")
	ErrNotPlayable  = errors.New("media: asset not playable")
	ErrFileTooLarge = errors.New("media: file too large")
	ErrBadCursor    = errors.New("media: invalid cursor")
)

// UnsupportedFormatError is returned by CompleteUpload when the uploaded object's
// magic bytes match no accepted image type (or are HEIC/HEIF). The handler maps
// it to a 422 media/unsupported-format Problem; HEIC gets a convert-to-JPEG hint.
type UnsupportedFormatError struct {
	Detail string
	HEIC   bool
}

func (e *UnsupportedFormatError) Error() string { return e.Detail }

type Status string

const (
	StatusUploading  Status = "uploading"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
	StatusDeleting   Status = "deleting"
)

// Asset is the media module's internal record.
type Asset struct {
	ID               uuid.UUID
	OwnerID          uuid.UUID
	Kind             string
	Status           Status
	SourceKey        string
	OutputPrefix     string
	MimeType         string
	SizeBytes        int64
	DurationMs       *int
	Width            *int
	Height           *int
	ErrorMessage     string
	Title            string
	OriginalFilename string
	Origin           string
	CreatedAt        time.Time
}

// Variant is a derived, metadata-stripped artifact (thumb/medium/poster).
type Variant struct {
	ID         uuid.UUID
	AssetID    uuid.UUID
	Variant    string
	StorageKey string
	Width      int
	Height     int
	SizeBytes  int64
	CreatedAt  time.Time
}

type CreateAssetInput struct {
	ID               uuid.UUID
	OwnerID          uuid.UUID
	Kind             string
	SourceKey        string
	MimeType         string
	SizeBytes        int64
	Title            string
	OriginalFilename string
	Origin           string
}

// ListCursorInput is the keyset library query (P0.4). Kind "" = any; Statuses
// empty = any (the service expands "processing" → {processing, uploading}); a
// zero CursorAt means "first page".
type ListCursorInput struct {
	OwnerID  uuid.UUID
	Kind     string
	Statuses []string
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// Repository is the persistence surface. It embeds worker.Repo (MarkReady /
// MarkFailed / MarkImageReady / InsertVariant / LoadAssetMeta) so a single
// adapter satisfies both the HTTP and worker sides.
type Repository interface {
	CreateAsset(ctx context.Context, in CreateAssetInput) (Asset, error)
	GetAsset(ctx context.Context, id uuid.UUID) (Asset, error)
	GetAssetOwner(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]Asset, error)
	ListByOwnerCursor(ctx context.Context, in ListCursorInput) ([]Asset, error)
	ListForPurge(ctx context.Context) ([]Asset, error)
	ListAbandonedUploads(ctx context.Context) ([]Asset, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	SetStatusDeleting(ctx context.Context, id uuid.UUID) error
	DeleteAsset(ctx context.Context, id uuid.UUID) error
	ListVariants(ctx context.Context, assetID uuid.UUID) ([]Variant, error)
	GetVariant(ctx context.Context, assetID uuid.UUID, variant string) (Variant, error)
	DeleteVariants(ctx context.Context, assetID uuid.UUID) error
	UpsertPlaybackProgress(ctx context.Context, ownerID, assetID uuid.UUID, positionMs int64, completedAt *time.Time) error
	GetPlaybackProgress(ctx context.Context, ownerID, assetID uuid.UUID) (positionMs int64, completedAt *time.Time, updatedAt time.Time, err error)
	GetContinueItems(ctx context.Context, userID uuid.UUID, limit int) ([]mediaapi.ContinueItem, error)
	worker.Repo
}

// EventPublisher fans a domain event out to its subscribers (platform/events).
// Optional on the module — a nil publisher just skips emission.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}

// Enqueuer schedules background jobs (satisfied by *asynq.Client).
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
