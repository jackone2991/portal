package journalrepo

// Adapter bridges the sqlc-generated Queries to the journal.Repository interface
// the module declares. cmd/api and cmd/worker construct it with NewAdapter; all
// pgtype ↔ domain juggling lives here so the module code stays sqlc-agnostic.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/journal"
)

// Adapter wraps *Queries. Construct with NewAdapter.
type Adapter struct {
	q *Queries
}

// NewAdapter builds the adapter over any pgx-compatible handle (pool or tx).
func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

func (a *Adapter) CreateEntry(ctx context.Context, in journal.CreateEntryInput) (journal.Entry, error) {
	row, err := a.q.CreateEntry(ctx, CreateEntryParams{
		UserID:     pgUUID(in.UserID),
		BodyMd:     in.BodyMd,
		OccurredAt: pgTime(in.OccurredAt),
		Mood:       in.Mood,
	})
	if err != nil {
		return journal.Entry{}, err
	}
	return toEntry(row), nil
}

func (a *Adapter) GetEntry(ctx context.Context, userID, id uuid.UUID) (journal.Entry, error) {
	row, err := a.q.GetEntry(ctx, GetEntryParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return journal.Entry{}, journal.ErrEntryNotFound
		}
		return journal.Entry{}, err
	}
	return toEntry(row), nil
}

func (a *Adapter) ListByUserCursor(ctx context.Context, in journal.ListInput) ([]journal.Entry, error) {
	p := ListEntriesByUserCursorParams{
		UserID: pgUUID(in.UserID),
		Lim:    int32(in.Limit),
	}
	if !in.CursorAt.IsZero() { // zero = first page → NULL cursor, start at the top
		p.CursorOccurredAt = pgTime(in.CursorAt)
		p.CursorID = pgUUID(in.CursorID)
	}
	rows, err := a.q.ListEntriesByUserCursor(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]journal.Entry, 0, len(rows))
	for _, r := range rows {
		out = append(out, toEntry(r))
	}
	return out, nil
}

func (a *Adapter) PatchEntry(ctx context.Context, in journal.PatchEntryInput) (journal.Entry, error) {
	p := PatchEntryParams{
		BodyMd: in.BodyMd,
		Mood:   in.Mood,
		ID:     pgUUID(in.ID),
		UserID: pgUUID(in.UserID),
	}
	if in.OccurredAt != nil {
		p.OccurredAt = pgTime(*in.OccurredAt)
	}
	row, err := a.q.PatchEntry(ctx, p)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return journal.Entry{}, journal.ErrEntryNotFound
		}
		return journal.Entry{}, err
	}
	return toEntry(row), nil
}

func (a *Adapter) DeleteEntry(ctx context.Context, userID, id uuid.UUID) error {
	_, err := a.q.DeleteEntry(ctx, DeleteEntryParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return journal.ErrEntryNotFound // idempotent: nothing matched → 404
		}
		return err
	}
	return nil
}

// ── mapping helpers ─────────────────────────────────────────────────

func toEntry(r JournalEntry) journal.Entry {
	return journal.Entry{
		ID:         uuidFrom(r.ID),
		UserID:     uuidFrom(r.UserID),
		BodyMd:     r.BodyMd,
		Mood:       r.Mood,
		AssetIDs:   uuidsFrom(r.AssetIds),
		OccurredAt: r.OccurredAt.Time,
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidsFrom(ps []pgtype.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ps))
	for _, p := range ps {
		if p.Valid {
			out = append(out, uuid.UUID(p.Bytes))
		}
	}
	return out
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
