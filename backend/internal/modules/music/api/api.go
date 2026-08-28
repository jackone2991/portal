// Package api is the public surface of the music module — tracks over media's
// audio assets (mirroring the movie vertical). Cross-module couplings: it
// CONSUMES media:asset_deleted to reap dangling audio/cover references and emits
// music:track_published on publish (emit-only). Only this package may be imported
// by other modules.
package api

const (
	// TaskOnAssetDeleted is music's consumer task for media:asset_deleted.
	TaskOnAssetDeleted = "music:on_asset_deleted"
	// TaskImportZip unpacks an uploaded zip of audio files into tracks (0038).
	// Owned by music; the worker registers it, the API only enqueues.
	TaskImportZip = "music:import_zip"

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

// ImportZipPayload is the music:import_zip task body.
//
// OwnerID rides along because `music_imports` is RLS-fenced and the worker has
// no request tenant: reading the job to learn its owner would need a tenant
// scope that can only be opened once the owner is known. It is a routing hint,
// not an authorisation claim — every read and write still happens inside that
// owner's tenant scope, so a forged payload reaches that tenant's rows and no
// others, where the job id simply will not be found.
type ImportZipPayload struct {
	ImportID string `json:"import_id"`
	OwnerID  string `json:"owner_id"`
}
