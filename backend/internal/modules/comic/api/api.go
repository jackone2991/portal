// Package api is the public surface of the comic module (SPEC-02).
//
// comic owns comics/chapters/pages + reading progress, built over media's image
// assets. Its cross-module couplings: it CONSUMES media:asset_deleted to reap
// dangling page/cover references (P0.6, soft cascade — never a foreign key), and
// (P1.9, deferred) emits comic:chapter_{published,deleted} to the life stream.
//
// Only this package may be imported by other modules.
package api

const (
	// TaskOnAssetDeleted is comic's consumer task for the media:asset_deleted
	// event (P0.6). cmd/worker subscribes media:asset_deleted → this task.
	TaskOnAssetDeleted = "comic:on_asset_deleted"

	// P1.9 life-stream events (emit deferred).
	EventChapterPublished = "comic:chapter_published"
	EventChapterDeleted   = "comic:chapter_deleted"
)

// AssetDeletedPayload mirrors the media:asset_deleted event body consumed at P0.6.
type AssetDeletedPayload struct {
	AssetID     string `json:"asset_id"`
	OwnerUserID string `json:"owner_user_id"`
}
