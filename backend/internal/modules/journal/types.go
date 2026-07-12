package journal

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors surfaced to the handler.
var (
	ErrEntryNotFound = errors.New("journal: entry not found")
	ErrInvalidBody   = errors.New("journal: invalid body")
	ErrInvalidMood   = errors.New("journal: invalid mood")
	ErrInvalidAsset  = errors.New("journal: invalid asset")
	ErrBadCursor     = errors.New("journal: invalid cursor")
)

// body_md / mood length bounds — mirror the 0011 migration CHECKs, measured in
// characters (runes) to match Postgres char_length semantics (P0.2 / §6).
const (
	minBodyLen = 1
	maxBodyLen = 20000
	minMoodLen = 1
	maxMoodLen = 80
)

// Entry is the journal module's internal record of one human-authored entry.
type Entry struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	BodyMd     string
	Mood       *string // nil = no mood set
	AssetIDs   []uuid.UUID
	OccurredAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateEntryInput is the persistence write side of a create. asset_ids is not
// included — it stays at the table default '{}' until P1.5 (the service rejects
// any asset_ids in the request before this runs). OccurredAt is already resolved
// (the service defaults it to now()).
type CreateEntryInput struct {
	UserID     uuid.UUID
	BodyMd     string
	Mood       *string
	OccurredAt time.Time
}

// PatchEntryInput is the persistence side of a partial update. A nil pointer
// leaves the column unchanged (COALESCE in PatchEntry).
type PatchEntryInput struct {
	UserID     uuid.UUID
	ID         uuid.UUID
	BodyMd     *string
	Mood       *string
	OccurredAt *time.Time
}

// ListInput is the keyset read query (P0.2). A zero CursorAt means "first page".
type ListInput struct {
	UserID   uuid.UUID
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// Repository is the persistence surface. The sqlc-backed adapter implements it;
// tests use an in-memory fake.
type Repository interface {
	CreateEntry(ctx context.Context, in CreateEntryInput) (Entry, error)
	// GetEntry is owner-scoped; ErrEntryNotFound for a missing or other-user id
	// (existence never leaks).
	GetEntry(ctx context.Context, userID, id uuid.UUID) (Entry, error)
	ListByUserCursor(ctx context.Context, in ListInput) ([]Entry, error)
	// PatchEntry is owner-scoped; ErrEntryNotFound for a missing or other-user id.
	PatchEntry(ctx context.Context, in PatchEntryInput) (Entry, error)
	// DeleteEntry is owner-scoped + idempotent; ErrEntryNotFound when nothing matched.
	DeleteEntry(ctx context.Context, userID, id uuid.UUID) error

	// ── life-stream projection (SPEC-06) ─────────────────────────────
	// InsertStreamItem is idempotent (ON CONFLICT DO NOTHING); UpsertStreamItem
	// refreshes payload+occurred_at (bank updated). DeleteStreamByRef removes all
	// event_types for a ref (media:asset_deleted).
	InsertStreamItem(ctx context.Context, userID uuid.UUID, sourceModule, eventType string, refID uuid.UUID, payload json.RawMessage, occurredAt time.Time) error
	UpsertStreamItem(ctx context.Context, userID uuid.UUID, sourceModule, eventType string, refID uuid.UUID, payload json.RawMessage, occurredAt time.Time) error
	DeleteStreamItem(ctx context.Context, sourceModule, eventType string, refID uuid.UUID) error
	DeleteStreamByRef(ctx context.Context, sourceModule string, refID uuid.UUID) error
	ListStream(ctx context.Context, in StreamListInput) ([]StreamItem, error)
}

// StreamItem is one row of the merged life-stream (SPEC-06). BodyMd/Mood are set
// only for journal items (joined from journal_entries); nil for system items.
type StreamItem struct {
	ID           uuid.UUID
	SourceModule string
	EventType    string
	RefID        uuid.UUID
	Payload      json.RawMessage
	OccurredAt   time.Time
	BodyMd       *string
	Mood         *string
}

// StreamListInput is the merged keyset read (P0.2). Zero CursorAt = first page.
type StreamListInput struct {
	UserID   uuid.UUID
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// EventPublisher fans a domain event out to its subscribers (platform/events).
// Optional on the module — a nil publisher just skips emission (and with zero
// subscribers the fan-out is already a no-op).
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
