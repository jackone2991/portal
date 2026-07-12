package ops

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Backup-run status values (ops_backup_runs.status CHECK — migration 0012).
const (
	StatusRunning = "running"
	StatusOK      = "ok"
	StatusFailed  = "failed"
)

// State is the freshness-sentinel verdict served by GET /ops/status (P0.5).
type State string

const (
	StateOK     State = "ok"
	StateStale  State = "stale"
	StateFailed State = "failed"
)

// Audit action codes (D-25 taxonomy: <module>.<resource>.<action>). Owned by ops;
// documented in the module README. Kept here (not platform/audit) so the ops
// namespace stays with its module.
const (
	ActionBackupCompleted = "ops.backup.completed"
	ActionBackupFailed    = "ops.backup.failed"
)

// BackupRun is the module's internal record of one row of ops_backup_runs.
// Nullable columns are pointers: a still-running row has no finished_at/size/key.
type BackupRun struct {
	ID         uuid.UUID
	StartedAt  time.Time
	FinishedAt *time.Time
	Status     string
	SizeBytes  *int64
	StorageKey *string
	Error      *string
}

// SuccessfulRunRef is the retention candidate projection: the dump's storage key
// plus when the run finished (fallback date when the key isn't a dated dump).
type SuccessfulRunRef struct {
	StorageKey string
	FinishedAt time.Time
}

// Repository is the persistence surface. The sqlc-backed adapter implements it;
// tests use an in-memory fake. "Get*" methods return (nil, nil) when no row
// matches (never a sentinel error) so the sentinel can distinguish never-ran.
type Repository interface {
	// InsertBackupRun opens a 'running' row and returns its id.
	InsertBackupRun(ctx context.Context) (uuid.UUID, error)
	// FinishBackupRunOK closes a run succeeded (size + storage key stamped).
	FinishBackupRunOK(ctx context.Context, id uuid.UUID, sizeBytes int64, storageKey string) error
	// FinishBackupRunFailed closes a run failed with a reason.
	FinishBackupRunFailed(ctx context.Context, id uuid.UUID, msg string) error
	// GetLastRun is the most recently started run of any status (nil when none).
	GetLastRun(ctx context.Context) (*BackupRun, error)
	// GetLastSuccessfulRun is the most recent status='ok' run (nil when none).
	GetLastSuccessfulRun(ctx context.Context) (*BackupRun, error)
	// GetLastCompletedRun is the most recent ok|failed run (nil when none).
	GetLastCompletedRun(ctx context.Context) (*BackupRun, error)
	// ListSuccessfulRuns is the retention candidate set, newest first.
	ListSuccessfulRuns(ctx context.Context) ([]SuccessfulRunRef, error)
}

// EventPublisher fans a domain event out to its subscribers (platform/events).
// Optional on the module — a nil publisher skips emission.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
