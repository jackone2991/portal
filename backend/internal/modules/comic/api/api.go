// Package api is the public surface of the comic module (SPEC-02).
//
// comic owns comics/chapters/pages + reading progress, built over media's image
// assets. Its cross-module couplings: it CONSUMES media:asset_deleted to reap
// dangling page/cover references (P0.6, soft cascade — never a foreign key), and
// (P1.9) emits comic:chapter_published to the life stream on publish.
//
// Only this package may be imported by other modules.
package api

const (
	// TaskOnAssetDeleted is comic's consumer task for the media:asset_deleted
	// event (P0.6). cmd/worker subscribes media:asset_deleted → this task.
	TaskOnAssetDeleted = "comic:on_asset_deleted"

	// P1.9 life-stream events. chapter_published is emitted per chapter on a
	// comic publish (consumed by the journal stream projection, keyed on
	// chapter_id). chapter_deleted's stream-removal consumer is deferred; the
	// published card's href is generic (/library/comic) so a later delete leaves
	// no 404 — an accepted residual until the removal consumer lands.
	EventChapterPublished = "comic:chapter_published"
	EventChapterDeleted   = "comic:chapter_deleted"
)

// AssetDeletedPayload mirrors the media:asset_deleted event body consumed at P0.6.
type AssetDeletedPayload struct {
	AssetID     string `json:"asset_id"`
	OwnerUserID string `json:"owner_user_id"`
}

// ChapterPublishedEvent is the comic:chapter_published body (P1.9). The journal
// stream consumer reads chapter_id + owner_user_id and renders "<title> published".
type ChapterPublishedEvent struct {
	ComicID     string `json:"comic_id"`
	ChapterID   string `json:"chapter_id"`
	OwnerUserID string `json:"owner_user_id"`
	Title       string `json:"title"`
}
