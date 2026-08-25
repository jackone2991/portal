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

// Life-stream consumer task types (SPEC-06 P0.1b). journal (owner of stream_items)
// subscribes one task per source event via the platform/events fan-out — a
// distinct task per event since the payload carries no event discriminator.
const (
	TaskStreamAssetReady        = "journal:stream_asset_ready"
	TaskStreamPlaybackCompleted = "journal:stream_playback_completed"
	TaskStreamAssetDeleted      = "journal:stream_asset_deleted"
	TaskStreamBankCreated       = "journal:stream_bank_created"
	TaskStreamBankUpdated       = "journal:stream_bank_updated"
	TaskStreamBankDeleted       = "journal:stream_bank_deleted"
	TaskStreamBirthday          = "journal:stream_birthday"
	TaskStreamComicPublished    = "journal:stream_comic_published"
	TaskStreamComicDeleted      = "journal:stream_comic_deleted"
	TaskStreamMoviePublished    = "journal:stream_movie_published"
	TaskStreamTrackPublished    = "journal:stream_track_published"
	TaskStreamStoryPublished    = "journal:stream_story_published"
)

// EntryCreatedEvent is the journal:entry_created payload (events.md): ids +
// occurred_at only. A consumer fetches any further detail through journal's api/
// once it grows a synchronous surface — payloads are not documents.
type EntryCreatedEvent struct {
	EntryID    uuid.UUID `json:"entry_id"`
	UserID     uuid.UUID `json:"user_id"`
	OccurredAt time.Time `json:"occurred_at"`
}
