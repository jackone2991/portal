package opsrepo

// Adapter bridges the sqlc-generated Queries to the ops.Repository interface the
// module declares. cmd/api and cmd/worker construct it with NewAdapter; all
// pgtype ↔ domain juggling lives here so the module code stays sqlc-agnostic.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/ops"
)

// Adapter wraps *Queries. Construct with NewAdapter.
type Adapter struct {
	q *Queries
}

// NewAdapter builds the adapter over any pgx-compatible handle (pool or tx).
func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

// Compile-time proof the adapter satisfies the ops persistence surface.
var _ ops.Repository = (*Adapter)(nil)

func (a *Adapter) InsertBackupRun(ctx context.Context) (uuid.UUID, error) {
	id, err := a.q.InsertBackupRun(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return uuidFrom(id), nil
}

func (a *Adapter) FinishBackupRunOK(ctx context.Context, id uuid.UUID, sizeBytes int64, storageKey string) error {
	sb, sk := sizeBytes, storageKey
	return a.q.FinishBackupRunOK(ctx, FinishBackupRunOKParams{
		ID:         pgUUID(id),
		SizeBytes:  &sb,
		StorageKey: &sk,
	})
}

func (a *Adapter) FinishBackupRunFailed(ctx context.Context, id uuid.UUID, msg string) error {
	m := msg
	return a.q.FinishBackupRunFailed(ctx, FinishBackupRunFailedParams{
		ID:    pgUUID(id),
		Error: &m,
	})
}

func (a *Adapter) GetLastRun(ctx context.Context) (*ops.BackupRun, error) {
	return oneRun(a.q.GetLastRun(ctx))
}

func (a *Adapter) GetLastSuccessfulRun(ctx context.Context) (*ops.BackupRun, error) {
	return oneRun(a.q.GetLastSuccessfulRun(ctx))
}

func (a *Adapter) GetLastCompletedRun(ctx context.Context) (*ops.BackupRun, error) {
	return oneRun(a.q.GetLastCompletedRun(ctx))
}

func (a *Adapter) ListSuccessfulRuns(ctx context.Context) ([]ops.SuccessfulRunRef, error) {
	rows, err := a.q.ListSuccessfulRuns(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ops.SuccessfulRunRef, 0, len(rows))
	for _, r := range rows {
		ref := ops.SuccessfulRunRef{StorageKey: derefStr(r.StorageKey)}
		if r.FinishedAt.Valid {
			ref.FinishedAt = r.FinishedAt.Time
		}
		out = append(out, ref)
	}
	return out, nil
}

// ── mapping helpers ─────────────────────────────────────────────────

// oneRun maps a single-row query result: pgx.ErrNoRows → (nil, nil) so the
// sentinel can tell "never ran" apart from a real error.
func oneRun(row OpsBackupRun, err error) (*ops.BackupRun, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	r := toRun(row)
	return &r, nil
}

func toRun(r OpsBackupRun) ops.BackupRun {
	out := ops.BackupRun{
		ID:         uuidFrom(r.ID),
		Status:     r.Status,
		SizeBytes:  r.SizeBytes,
		StorageKey: r.StorageKey,
		Error:      r.Error,
	}
	if r.StartedAt.Valid {
		out.StartedAt = r.StartedAt.Time
	}
	if r.FinishedAt.Valid {
		t := r.FinishedAt.Time
		out.FinishedAt = &t
	}
	return out
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
