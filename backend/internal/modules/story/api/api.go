// Package api is the public surface of the story module — the long-form-text
// vertical (stories → text chapters), copying the comic reference pattern but
// slimmed: chapters hold inline markdown bodies, no pages, no reading progress.
//
// Cross-module couplings: it CONSUMES media:asset_deleted to reap dangling cover
// references (soft cascade, never a foreign key) and emits story:published to the
// life stream on publish (emit-only; a consumer can be wired later without
// touching the producer). Only this package may be imported by other modules.
package api

const (
	// TaskOnAssetDeleted is story's consumer task for the media:asset_deleted
	// event. cmd/worker subscribes media:asset_deleted → this task.
	TaskOnAssetDeleted = "story:on_asset_deleted"

	// EventStoryPublished is emitted on a story publish (a journal stream consumer
	// can render "<title> published" — wired like comic:chapter_published).
	EventStoryPublished = "story:published"
)

// AssetDeletedPayload mirrors the media:asset_deleted event body.
type AssetDeletedPayload struct {
	AssetID     string `json:"asset_id"`
	OwnerUserID string `json:"owner_user_id"`
}

// StoryPublishedEvent is the story:published body.
type StoryPublishedEvent struct {
	StoryID     string `json:"story_id"`
	OwnerUserID string `json:"owner_user_id"`
	Title       string `json:"title"`
}
