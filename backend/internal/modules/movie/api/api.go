// Package api is the public surface of the movie module — the first domain
// vertical over media's video assets (copying the comic reference pattern).
//
// Cross-module couplings: it CONSUMES media:asset_deleted to reap dangling
// video/poster references (soft cascade, never a foreign key) and emits
// movie:published to the life stream on publish (emit-only; a consumer can be
// wired later without touching the producer). Only this package may be imported
// by other modules.
package api

const (
	// TaskOnAssetDeleted is movie's consumer task for the media:asset_deleted
	// event. cmd/worker subscribes media:asset_deleted → this task.
	TaskOnAssetDeleted = "movie:on_asset_deleted"

	// EventMoviePublished is emitted on a movie publish (a journal stream consumer
	// can render "<title> published" — wired the same way as comic:chapter_published).
	EventMoviePublished = "movie:published"
)

// AssetDeletedPayload mirrors the media:asset_deleted event body.
type AssetDeletedPayload struct {
	AssetID     string `json:"asset_id"`
	OwnerUserID string `json:"owner_user_id"`
}

// MoviePublishedEvent is the movie:published body.
type MoviePublishedEvent struct {
	MovieID     string `json:"movie_id"`
	OwnerUserID string `json:"owner_user_id"`
	Title       string `json:"title"`
}
