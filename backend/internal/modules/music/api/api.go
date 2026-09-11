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

	// TaskEnrichTrack fills in what lives inside the audio file — embedded cover
	// art and any missing tags — after the track already exists (0038). Split
	// from the import because the cover half is slow: an ffmpeg extraction, a
	// second asset ingest, and a wait for the image pipeline.
	TaskEnrichTrack = "music:enrich_track"

	// TaskLookupTrack asks MusicBrainz for what the file cannot say — release
	// year, genre, and a Cover Art Archive cover (0039). The only outbound
	// third-party call in the codebase, throttled to 1 req/s and off by default.
	TaskLookupTrack = "music:lookup_track"

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

// EnrichTrackPayload is the music:enrich_track task body. OwnerID rides along
// for the same reason ImportZipPayload carries it: the worker has no request
// tenant and cannot read an RLS-fenced table to find out.
type EnrichTrackPayload struct {
	TrackID string `json:"track_id"`
	OwnerID string `json:"owner_id"`
}

// LookupTrackPayload is the music:lookup_track task body. OwnerID rides along
// for the same reason the other music payloads carry it: the worker has no
// request tenant and cannot read an RLS-fenced table to find out.
type LookupTrackPayload struct {
	TrackID string `json:"track_id"`
	OwnerID string `json:"owner_id"`
}
