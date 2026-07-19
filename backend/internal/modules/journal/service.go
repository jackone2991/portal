package journal

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	journalapi "github.com/portal/backend/internal/modules/journal/api"
)

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

// Service holds the journal business logic: owner-scoped entry CRUD plus the
// emit-only journal:entry_created event (P0.3). Construct via the module.
type Service struct {
	repo   Repository
	events EventPublisher // optional: journal:entry_created (emit-only)
	// runInUserTenant scopes a worker-side stream INSERT to the target user's
	// personal org (ADR-07 Increment 1b): resolve the org, open BeginTenantScope so
	// stream_items.tenant_id's DEFAULT current_setting is populated. nil on the API
	// side (already inside a request tenant tx) and in tests → the fn runs directly.
	runInUserTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error
}

// CreateParams is the service-level create request. HasAssetIDs records whether
// the caller supplied asset_ids at all — any presence is rejected until P1.5
// ships (fail closed rather than store unvalidated cross-module references, P0.2).
type CreateParams struct {
	UserID      uuid.UUID
	BodyMd      string
	Mood        *string    // nil = absent (no mood)
	OccurredAt  *time.Time // nil = default now
	HasAssetIDs bool
}

// PatchParams is the service-level partial update. A nil pointer means "leave
// unchanged"; HasAssetIDs rejects any asset_ids until P1.5, as create does.
type PatchParams struct {
	UserID      uuid.UUID
	ID          uuid.UUID
	BodyMd      *string
	Mood        *string
	OccurredAt  *time.Time
	HasAssetIDs bool
}

// ListResult is a keyset page plus the cursor for the next one ("" = last page).
type ListResult struct {
	Items      []Entry
	NextCursor string
}

// Create validates, inserts an entry, and only after the insert commits
// publishes journal:entry_created (emit-only, P0.3). A validation failure or an
// insert error publishes nothing.
func (s *Service) Create(ctx context.Context, p CreateParams) (Entry, error) {
	if p.HasAssetIDs {
		return Entry{}, ErrInvalidAsset // P1.5 not shipped — fail closed
	}
	if !validBody(p.BodyMd) {
		return Entry{}, ErrInvalidBody
	}
	mood, err := normalizeMood(p.Mood)
	if err != nil {
		return Entry{}, err
	}
	occurredAt := time.Now().UTC()
	if p.OccurredAt != nil {
		occurredAt = *p.OccurredAt // backdating / future-dating both unlimited
	}

	entry, err := s.repo.CreateEntry(ctx, CreateEntryInput{
		UserID:     p.UserID,
		BodyMd:     p.BodyMd,
		Mood:       mood,
		OccurredAt: occurredAt,
	})
	if err != nil {
		return Entry{}, err // rolled back → nothing published
	}
	s.emitCreated(ctx, entry) // strictly after commit
	return entry, nil
}

// Get returns one owner-scoped entry (ErrEntryNotFound for a missing or
// other-user id — the handler answers 404, never leaking existence).
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (Entry, error) {
	return s.repo.GetEntry(ctx, userID, id)
}

// List returns a keyset page of the caller's entries, newest first
// (occurred_at DESC, id DESC).
func (s *Service) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (ListResult, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}
	in := ListInput{UserID: userID, Limit: limit + 1} // fetch one extra to detect a next page
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return ListResult{}, ErrBadCursor
		}
		in.CursorAt, in.CursorID = at, id
	}

	rows, err := s.repo.ListByUserCursor(ctx, in)
	if err != nil {
		return ListResult{}, err
	}

	var res ListResult
	if len(rows) > limit {
		res.NextCursor = encodeCursor(rows[limit-1])
		rows = rows[:limit]
	}
	res.Items = rows
	return res, nil
}

// Patch applies a partial update (any subset of body_md/mood/occurred_at),
// owner-scoped. No entry_updated event is emitted (P0.3 — the only planned
// consumer maintains its projection transactionally in-module).
func (s *Service) Patch(ctx context.Context, p PatchParams) (Entry, error) {
	if p.HasAssetIDs {
		return Entry{}, ErrInvalidAsset
	}
	if p.BodyMd != nil && !validBody(*p.BodyMd) {
		return Entry{}, ErrInvalidBody
	}
	mood, err := normalizeMood(p.Mood)
	if err != nil {
		return Entry{}, err
	}
	return s.repo.PatchEntry(ctx, PatchEntryInput{
		UserID:     p.UserID,
		ID:         p.ID,
		BodyMd:     p.BodyMd,
		Mood:       mood,
		OccurredAt: p.OccurredAt,
	})
}

// Delete removes an owner-scoped entry. Idempotent: a missing/other-user id
// returns ErrEntryNotFound (the handler answers 404). No entry_deleted event
// (P0.3).
func (s *Service) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.DeleteEntry(ctx, userID, id)
}

// emitCreated publishes journal:entry_created best-effort — the entry is already
// committed, so a publish failure must not fail the request (it is logged).
func (s *Service) emitCreated(ctx context.Context, e Entry) {
	if s.events == nil {
		return
	}
	if err := s.events.Publish(ctx, journalapi.EventEntryCreated, journalapi.EntryCreatedEvent{
		EntryID:    e.ID,
		UserID:     e.UserID,
		OccurredAt: e.OccurredAt,
	}); err != nil {
		log.Warn().Err(err).Str("entry", e.ID.String()).Msg("journal: entry_created publish failed")
	}
}

// ── validation ──────────────────────────────────────────────────────

// validBody reports whether body_md is 1–20000 characters (runes), matching the
// §6 CHECK measured in char_length.
func validBody(body string) bool {
	n := utf8.RuneCountInString(body)
	return n >= minBodyLen && n <= maxBodyLen
}

// normalizeMood trims a present mood and enforces 1–80 chars (matching the §6
// CHECK). A nil pointer is "no mood"; a present but empty/whitespace-only mood is
// ErrInvalidMood — caught here so it can't surface as a 500 from the DB CHECK.
func normalizeMood(mood *string) (*string, error) {
	if mood == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*mood)
	n := utf8.RuneCountInString(trimmed)
	if n < minMoodLen || n > maxMoodLen {
		return nil, ErrInvalidMood
	}
	return &trimmed, nil
}

// ── cursor helpers (keyset "<occurred_at>|<id>", base64url) ─────────

func encodeCursor(e Entry) string {
	raw := e.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + e.ID.String()
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, errors.New("journal: malformed cursor")
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return at, id, nil
}
