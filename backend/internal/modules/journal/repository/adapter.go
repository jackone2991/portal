package journalrepo

// Adapter bridges the sqlc-generated Queries to the journal.Repository interface
// the module declares. cmd/api and cmd/worker construct it with NewAdapter; all
// pgtype ↔ domain juggling lives here so the module code stays sqlc-agnostic.

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/journal"
)

// journal projection identifiers (SPEC-06 P0.1a) — journal entry rows carry
// these uniform source/event names even without bus delivery.
const (
	streamSourceJournal = "journal"
	streamEventEntry    = "journal:entry_created"
)

// Adapter wraps *Queries plus an injected RunInTx for the transactional journal
// projection (entry + stream row in one tx). Construct with NewAdapter.
type Adapter struct {
	q       *Queries
	runInTx func(context.Context, func(pgx.Tx) error) error
}

// NewAdapter builds the adapter over a DBTX (the context-aware platform/db.Conn)
// plus a RunInTx that opens/reuses the request transaction.
func NewAdapter(db DBTX, runInTx func(context.Context, func(pgx.Tx) error) error) *Adapter {
	return &Adapter{q: New(db), runInTx: runInTx}
}

// CreateEntry inserts the entry AND its stream projection row in one transaction
// (SPEC-06 P0.1a — kills the post-create refetch race; no bus, no redelivery).
func (a *Adapter) CreateEntry(ctx context.Context, in journal.CreateEntryInput) (journal.Entry, error) {
	var entry journal.Entry
	err := a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		row, err := q.CreateEntry(ctx, CreateEntryParams{
			UserID:     pgUUID(in.UserID),
			BodyMd:     in.BodyMd,
			OccurredAt: pgTime(in.OccurredAt),
			Mood:       in.Mood,
		})
		if err != nil {
			return err
		}
		if err := q.InsertStreamItem(ctx, InsertStreamItemParams{
			UserID: row.UserID, SourceModule: streamSourceJournal, EventType: streamEventEntry,
			RefID: row.ID, OccurredAt: row.OccurredAt,
		}); err != nil {
			return err
		}
		entry = toEntry(row)
		return nil
	})
	if err != nil {
		return journal.Entry{}, err
	}
	return entry, nil
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

// PatchEntry updates the entry and moves its stream row to the edited
// occurred_at (P0.1a — else a backdated edit leaves the item at a stale position).
func (a *Adapter) PatchEntry(ctx context.Context, in journal.PatchEntryInput) (journal.Entry, error) {
	var entry journal.Entry
	err := a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		p := PatchEntryParams{
			BodyMd: in.BodyMd,
			Mood:   in.Mood,
			ID:     pgUUID(in.ID),
			UserID: pgUUID(in.UserID),
		}
		if in.OccurredAt != nil {
			p.OccurredAt = pgTime(*in.OccurredAt)
		}
		row, err := q.PatchEntry(ctx, p)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return journal.ErrEntryNotFound
			}
			return err
		}
		if err := q.UpdateStreamOccurredAt(ctx, UpdateStreamOccurredAtParams{
			SourceModule: streamSourceJournal, EventType: streamEventEntry, RefID: row.ID, OccurredAt: row.OccurredAt,
		}); err != nil {
			return err
		}
		entry = toEntry(row)
		return nil
	})
	if err != nil {
		return journal.Entry{}, err
	}
	return entry, nil
}

// DeleteEntry removes the entry and its stream row in one tx (P0.1a).
func (a *Adapter) DeleteEntry(ctx context.Context, userID, id uuid.UUID) error {
	return a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		if _, err := q.DeleteEntry(ctx, DeleteEntryParams{ID: pgUUID(id), UserID: pgUUID(userID)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return journal.ErrEntryNotFound // idempotent: nothing matched → 404
			}
			return err
		}
		if err := q.DeleteStreamItem(ctx, DeleteStreamItemParams{
			SourceModule: streamSourceJournal, EventType: streamEventEntry, RefID: pgUUID(id),
		}); err != nil {
			return err
		}
		return nil
	})
}

// ── stream projection (SPEC-06 P0.1b + P0.2) ─────────────────────────

func (a *Adapter) InsertStreamItem(ctx context.Context, userID uuid.UUID, sourceModule, eventType string, refID uuid.UUID, payload json.RawMessage, occurredAt time.Time) error {
	return a.q.InsertStreamItem(ctx, InsertStreamItemParams{
		UserID: pgUUID(userID), SourceModule: sourceModule, EventType: eventType,
		RefID: pgUUID(refID), OccurredAt: pgTime(occurredAt), Payload: payloadOr(payload),
	})
}

func (a *Adapter) UpsertStreamItem(ctx context.Context, userID uuid.UUID, sourceModule, eventType string, refID uuid.UUID, payload json.RawMessage, occurredAt time.Time) error {
	return a.q.UpsertStreamItem(ctx, UpsertStreamItemParams{
		UserID: pgUUID(userID), SourceModule: sourceModule, EventType: eventType,
		RefID: pgUUID(refID), OccurredAt: pgTime(occurredAt), Payload: payloadOr(payload),
	})
}

func (a *Adapter) DeleteStreamItem(ctx context.Context, sourceModule, eventType string, refID uuid.UUID) error {
	return a.q.DeleteStreamItem(ctx, DeleteStreamItemParams{SourceModule: sourceModule, EventType: eventType, RefID: pgUUID(refID)})
}

func (a *Adapter) DeleteStreamByRef(ctx context.Context, sourceModule string, refID uuid.UUID) error {
	return a.q.DeleteStreamByRef(ctx, DeleteStreamByRefParams{SourceModule: sourceModule, RefID: pgUUID(refID)})
}

func (a *Adapter) ListStream(ctx context.Context, in journal.StreamListInput) ([]journal.StreamItem, error) {
	p := ListStreamCursorParams{UserID: pgUUID(in.UserID), Lim: int32(in.Limit)}
	if !in.CursorAt.IsZero() {
		p.CursorOccurredAt = pgTime(in.CursorAt)
		p.CursorID = pgUUID(in.CursorID)
	}
	rows, err := a.q.ListStreamCursor(ctx, p)
	if err != nil {
		return nil, err
	}
	out := make([]journal.StreamItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, journal.StreamItem{
			ID: uuidFrom(r.ID), SourceModule: r.SourceModule, EventType: r.EventType,
			RefID: uuidFrom(r.RefID), Payload: json.RawMessage(r.Payload), OccurredAt: r.OccurredAt.Time,
			BodyMd: r.BodyMd, Mood: r.Mood,
		})
	}
	return out, nil
}

// payloadOr returns nil for an empty payload so pgx sends SQL NULL — the query's
// COALESCE(...::jsonb, '{}'::jsonb) then substitutes an empty object. sqlc types
// the param as []byte now that the query casts it explicitly.
func payloadOr(p json.RawMessage) []byte {
	if len(p) == 0 {
		return nil
	}
	return []byte(p)
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
