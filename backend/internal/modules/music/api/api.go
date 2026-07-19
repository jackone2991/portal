// Package api is the public surface of the music module — tracks over media's
// audio assets (mirroring the movie vertical). Cross-module couplings: it
// CONSUMES media:asset_deleted to reap dangling audio/cover references and emits
// music:track_published on publish (emit-only). Only this package may be imported
// by other modules.
package api

const (
	// TaskOnAssetDeleted is music's consumer task for media:asset_deleted.
	TaskOnAssetDeleted = "music:on_asset_deleted"
	// EventTrackPublished is emitted on a track publish (emit-only).
	EventTrackPublished = "music:track_published"
)

// AssetDeletedPayload mirrors the media:asset_deleted event body.
type AssetDeletedPayload struct {
	AssetID     string `json:"asset_id"`
	OwnerUserID string `json:"owner_user_id"`
}

// TrackPublishedEvent is the music:track_published body.
type TrackPublishedEvent struct {
	TrackID     string `json:"track_id"`
	OwnerUserID string `json:"owner_user_id"`
	Title       string `json:"title"`
}
