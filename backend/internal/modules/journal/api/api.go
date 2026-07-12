// Package api is the public surface of the journal module (SPEC-05).
//
// journal owns human-authored life-stream entries. At v1 the only cross-module
// contract is the emit-only journal:entry_created event, announced on the
// platform/events bus after a create commits (P0.3) — journal exposes no
// synchronous call surface yet. SPEC-06's stream projection is maintained
// transactionally in-module, so it does NOT consume this event; the event exists
// for future external consumers (docs/reference/events.md). Deliberately no
// entry_updated / entry_deleted events at v1.
//
// Only this package may be imported by other modules; journal's service/handler/
// repository internals stay private.
package api

import (
	"time"

	"github.com/google/uuid"
)

// EventEntryCreated is published (emit-only) once a journal entry's create
// transaction commits. Payload: EntryCreatedEvent.
const EventEntryCreated = "journal:entry_created"

// EntryCreatedEvent is the journal:entry_created payload (events.md): ids +
// occurred_at only. A consumer fetches any further detail through journal's api/
// once it grows a synchronous surface — payloads are not documents.
type EntryCreatedEvent struct {
	EntryID    uuid.UUID `json:"entry_id"`
	UserID     uuid.UUID `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
}
