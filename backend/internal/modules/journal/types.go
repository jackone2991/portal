package journal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// Errors surfaced to the handler.
var (
	ErrEntryNotFound = errors.New("journal: entry not found")
	ErrInvalidBody   = errors.New("journal: invalid body")
	ErrInvalidMood   = errors.New("journal: invalid mood")
	// ErrInvalidAsset means the Attachment list as a whole is invalid (SPEC-12).
	// The service wraps it in an AssetError naming the offending id and reason.
	ErrInvalidAsset = errors.New("journal: invalid asset")
	// ErrInvalidLocation is a Location that breaks the 0045 CHECK rules: a
	// name without a point or a point without a name, a blank name, or a point
	// off the Earth (SPEC-12 T3).
	ErrInvalidLocation = errors.New("journal: invalid location")
	ErrBadCursor       = errors.New("journal: invalid cursor")
)

// AssetError is ErrInvalidAsset with the id and the reason a client needs to
// fix the right thing (SPEC-12 story 28). errors.Is(err, ErrInvalidAsset) holds.
type AssetError struct {
	ID     string // as the client sent it — it may not even be a uuid
	Reason string
}

func (e *AssetError) Error() string        { return fmt.Sprintf("journal: asset %s %s", e.ID, e.Reason) }
func (e *AssetError) Is(target error) bool { return target == ErrInvalidAsset }

// Length bounds — mirror the migration CHECKs (0011, relaxed by 0044), measured
// in characters (runes) to match Postgres char_length semantics. The body has no
// minimum since 0044: an Entry is text, or at least one Attachment (SPEC-12) —
// that rule is the service's, not the database's.
const (
	maxBodyLen  = 20000
	minMoodLen  = 1
	maxMoodLen  = 80
	maxAssetIDs = 10
)

// Location is where an Entry happened (SPEC-12 T3) — a property of the Entry,
// not an Attachment. Stored as three all-or-nothing columns (0045); this is
// the "all" half, nil is the "nothing" half. Coordinates are decimal degrees,
// kept to four places.
type Location struct {
	Name string
	Lat  float64
	Lon  float64
}

// Entry is the journal module's internal record of one human-authored entry.
type Entry struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	BodyMd     string
	Mood       *string // nil = no mood set
	AssetIDs   []uuid.UUID
	Location   *Location // nil = no Location
	OccurredAt time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateEntryInput is the persistence write side of a create. AssetIDs is the
// validated Attachment list in display order (empty = text-only); OccurredAt is
// already resolved (the service defaults it to now()).
type CreateEntryInput struct {
	UserID     uuid.UUID
	BodyMd     string
	Mood       *string
	AssetIDs   []uuid.UUID
	Location   *Location // validated; nil = none
	OccurredAt time.Time
}

// PatchEntryInput is the persistence side of a partial update. A nil pointer
// leaves the column unchanged (COALESCE in PatchEntry). AssetIDs is a pointer
// to a slice on purpose: nil = keep, a pointer to an empty slice = clear — the
// wire distinction between an absent `asset_ids` and `"asset_ids": []`.
// The Location has three wire states — absent (keep), null (clear), object
// (set) — so it travels as a flag plus a value: SetLocation false = keep;
// true with a nil Location = clear; true with one = set (SPEC-12 T3).
type PatchEntryInput struct {
	UserID      uuid.UUID
	ID          uuid.UUID
	BodyMd      *string
	Mood        *string
	AssetIDs    *[]uuid.UUID
	SetLocation bool
	Location    *Location
	OccurredAt  *time.Time
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

// StreamItem is one row of the merged life-stream (SPEC-06). BodyMd/Mood/
// AssetIDs/Location are set only for journal items (joined from
// journal_entries); nil for system items.
type StreamItem struct {
	ID           uuid.UUID
	SourceModule string
	EventType    string
	RefID        uuid.UUID
	Payload      json.RawMessage
	OccurredAt   time.Time
	BodyMd       *string
	Mood         *string
	AssetIDs     []uuid.UUID
	Location     *Location
}

// MediaAPI is the slice of media/api journal needs: the Attachment lookup that
// validates `asset_ids` (SPEC-12). Declared here, satisfied by media.Module.API()
// in cmd/api — journal never imports media's internals. Every call runs inside
// the request's tenant transaction, so an Asset in another tenant answers nil.
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
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
