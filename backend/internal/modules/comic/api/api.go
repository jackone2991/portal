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

	// TaskImportZip unpacks an uploaded chapter zip → media assets → pages
	// (SPEC-02 P1.7). Enqueued on the "default" queue (NOT heavy — it polls asset
	// status and must not occupy a heavy slot its own process_image tasks need).
	TaskImportZip = "comic:import_zip"

	// P1.9 events. chapter_deleted is emitted per chapter on a chapter/comic
	// delete, for consumers that track chapters individually.
	//
	// comic:published is the ONE event a publish emits for the reader-facing
	// story: this comic was published, with N chapters. The former per-chapter
	// chapter_published fan-out is gone — its only consumer was the life-stream
	// projection, and one card per chapter turned a 500-chapter title into 500
	// feed entries. A person wants to hear about the comic, once.
	EventComicPublished = "comic:published"
	EventChapterDeleted = "comic:chapter_deleted"
)

// AssetDeletedPayload mirrors the media:asset_deleted event body consumed at P0.6.
type AssetDeletedPayload struct {
	AssetID     string `json:"asset_id"`
	OwnerUserID string `json:"owner_user_id"`
}

// ImportZipPayload is the comic:import_zip task body (P1.7): which import job to run.
type ImportZipPayload struct {
	ImportID string `json:"import_id"`
}

// ComicPublishedEvent is the comic:published body. ChapterCount is what makes a
// re-publish worth telling anyone about — it is part of the notification's dedup
// key, so publishing the same comic twice is silent unless chapters were added.
type ComicPublishedEvent struct {
	ComicID      string `json:"comic_id"`
	OwnerUserID  string `json:"owner_user_id"`
	Title        string `json:"title"`
	ChapterCount int    `json:"chapter_count"`
}

// ChapterDeletedEvent is the comic:chapter_deleted body (P1.9), emitted per
// chapter on chapter/comic delete so the stream drops the published card
// (consumer keys on chapter_id).
type ChapterDeletedEvent struct {
	ComicID     string `json:"comic_id"`
	ChapterID   string `json:"chapter_id"`
	OwnerUserID string `json:"owner_user_id"`
}
