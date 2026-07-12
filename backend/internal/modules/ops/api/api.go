// Package api is the public surface of the ops module (SPEC-09).
//
// ops owns platform operations: nightly Postgres backups, the restore drill, and
// the freshness sentinel. Its only cross-module contracts are the Asynq task type
// and the two emit-only events below (docs/reference/events.md). Producers/
// consumers never import ops internals — only this package.
package api

import "github.com/google/uuid"

// TaskBackupDatabase is the nightly pg_dump → storage task (P0.2). Registered on
// the worker's light mux and scheduled on the shared periodic scheduler; empty
// payload (a system-scoped sweep, no arguments).
const TaskBackupDatabase = "ops:backup_database"

// Emit-only backup lifecycle events (P0.2). No consumers today — audit + the
// /ops/status sentinel are the surface; a notify consumer lands later.
const (
	EventBackupCompleted = "ops:backup_completed"
	EventBackupFailed    = "ops:backup_failed"
)

// BackupCompletedEvent is the ops:backup_completed payload.
type BackupCompletedEvent struct {
	RunID     uuid.UUID `json:"run_id"`
	SizeBytes int64     `json:"size_bytes"`
}

// BackupFailedEvent is the ops:backup_failed payload.
type BackupFailedEvent struct {
	RunID uuid.UUID `json:"run_id"`
	Error string    `json:"error"`
}
