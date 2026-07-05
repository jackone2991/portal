package media

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	"github.com/portal/backend/internal/modules/media/worker"
)

// Errors surfaced to the handler.
var (
	ErrNotFound  = errors.New("media: asset not found")
	ErrForbidden = errors.New("media: not the asset owner")
	ErrNotReady  = errors.New("media: asset not ready")
)

type Status string

const (
	StatusUploading  Status = "uploading"
	StatusProcessing Status = "processing"
	StatusReady      Status = "ready"
	StatusFailed     Status = "failed"
)

// Asset is the media module's internal record.
type Asset struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	Kind         string
	Status       Status
	SourceKey    string
	OutputPrefix string
	MimeType     string
	SizeBytes    int64
	DurationMs   *int
	Width        *int
	Height       *int
	ErrorMessage string
	CreatedAt    time.Time
}

type CreateAssetInput struct {
	ID        uuid.UUID
	OwnerID   uuid.UUID
	Kind      string
	SourceKey string
	MimeType  string
	SizeBytes int64
}

// Repository is the persistence surface. It embeds worker.Repo (MarkReady /
// MarkFailed) so a single adapter satisfies both the HTTP and worker sides.
type Repository interface {
	CreateAsset(ctx context.Context, in CreateAssetInput) (Asset, error)
	GetAsset(ctx context.Context, id uuid.UUID) (Asset, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]Asset, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	worker.Repo
}

// Enqueuer schedules background jobs (satisfied by *asynq.Client).
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}
