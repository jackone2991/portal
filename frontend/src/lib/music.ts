// Data layer for the music vertical. TanStack owns server state (D-32); types
// mirror the wired handler JSON (snake_case) in
// `backend/internal/modules/music/handler.go` (`trackJSON` / `writeTrackList`).
//
// Note what the wire does NOT carry: a track has no duration. Duration lives on
// the linked *audio asset* (`media_assets.duration_ms`), and for playback we get
// it for free from the <audio> element's `loadedmetadata` — so nothing here
// fetches an asset just to render a running time.

import { api, baseURL } from "./api-client";

export type TrackStatus = "draft" | "published";

export interface Track {
  id: string;
  owner_id: string;
  title: string;
  artist: string | null;
  album: string | null;
  description: string | null;
  audio_asset_id: string | null;
  cover_asset_id: string | null;
  status: TrackStatus;
  created_at: string;
  updated_at: string;

  /* Catalogue lookup (0039) — what the audio file cannot contain. */
  release_year: number | null;
  genre: string | null;
  mb_recording_id: string | null;
  mb_release_id: string | null;
  /**
   * `no_match` is an ORDINARY outcome — most libraries contain something
   * MusicBrainz has never heard of — so it must not be shown as a failure.
   * `failed` is the one that means something went wrong.
   */
  lookup_status: "none" | "pending" | "matched" | "no_match" | "failed";
  lookup_note: string | null;
  lookup_at: string | null;
}

export interface TracksPage {
  tracks: Track[];
  next_cursor?: string | null;
}

/**
 * Direct stream URL for a track's audio.
 *
 * Points at `GET /assets/{id}/original` — the same owner-authenticated
 * attachment stream the media library downloads from. The browser attaches
 * cookies to an `<audio src>` on a same-site request exactly as it does for
 * `<img src>`, so no bearer plumbing is needed. Audio assets are not
 * transcoded to HLS (the pipeline only does that for video), so the original
 * IS the playback source.
 */
export function trackAudioURL(audioAssetId: string): string {
  return `${baseURL}/api/v1/assets/${audioAssetId}/original`;
}

/** Cover art variant URL — same asset-variant route the comic/media views use. */
export function trackCoverURL(
  coverAssetId: string,
  variant: "thumb" | "medium" | "poster" = "thumb",
): string {
  return `${baseURL}/api/v1/assets/${coverAssetId}/variants/${variant}`;
}

/** A track can only be played once it has an audio asset attached. */
export function isPlayable(t: Track): boolean {
  return Boolean(t.audio_asset_id);
}

/** Display label with a sensible fallback — artist is nullable on the wire. */
export function trackArtist(t: Track): string {
  return t.artist?.trim() || "Unknown artist";
}

/* ── reads ────────────────────────────────────────────────────────── */

/**
 * The server's ceiling on `?limit` (`maxLimit` in `music/types.go`). Anything
 * above it is silently ignored and you get the 30-row default back instead — so
 * the "fetch the whole library" walk below asks for exactly this and no more.
 */
export const TRACKS_MAX_PAGE = 50;

function tracksQuery(cursor?: string, limit?: number): string {
  const p = new URLSearchParams();
  if (cursor) p.set("cursor", cursor);
  if (limit) p.set("limit", String(limit));
  const q = p.toString();
  return q ? `?${q}` : "";
}

export async function listTracks(cursor?: string, limit?: number): Promise<TracksPage> {
  const r = await api<TracksPage>(`/api/v1/tracks${tracksQuery(cursor, limit)}`);
  return { tracks: r.tracks ?? [], next_cursor: r.next_cursor };
}

export async function listMyTracks(cursor?: string, limit?: number): Promise<TracksPage> {
  const r = await api<TracksPage>(`/api/v1/tracks/mine${tracksQuery(cursor, limit)}`);
  return { tracks: r.tracks ?? [], next_cursor: r.next_cursor };
}

/**
 * Every track in a scope, by walking the cursor to exhaustion.
 *
 * This exists for "Phát tất cả", which has to mean *all* — queueing only the
 * rows that happen to be on screen turns the button into a lie the moment the
 * list is paginated, and the user has no way to tell that it stopped at 30.
 *
 * Bounded by `maxTracks` because "play all" on a 50k-row library is not a
 * request anyone means literally, and an unbounded walk would hammer the API
 * while the page sits frozen. The caller is told when the cap was hit
 * (`truncated`) so it can say so rather than silently dropping the tail.
 */
export async function fetchAllTracks(
  scope: "published" | "mine",
  maxTracks = 1000,
): Promise<{ tracks: Track[]; truncated: boolean }> {
  const page = scope === "mine" ? listMyTracks : listTracks;
  const tracks: Track[] = [];
  let cursor: string | undefined;

  for (;;) {
    const r = await page(cursor, TRACKS_MAX_PAGE);
    tracks.push(...r.tracks);
    cursor = r.next_cursor ?? undefined;
    if (!cursor) return { tracks, truncated: false };
    if (tracks.length >= maxTracks) return { tracks: tracks.slice(0, maxTracks), truncated: true };
  }
}

export async function getTrack(id: string): Promise<Track> {
  return api<Track>(`/api/v1/tracks/${id}`);
}

/* ── writes ───────────────────────────────────────────────────────── */

export interface CreateTrackInput {
  title: string;
  artist?: string | null;
  album?: string | null;
  description?: string | null;
  audio_asset_id?: string | null;
  cover_asset_id?: string | null;
}

export async function createTrack(body: CreateTrackInput): Promise<Track> {
  return api<Track>("/api/v1/tracks", { method: "POST", body: JSON.stringify(body) });
}

/**
 * Patch track metadata (owner, or `music:write:any`). Only send changed fields —
 * an absent key means "unchanged", an explicit `null` clears the field.
 */
export type TrackPatch = Partial<{
  title: string;
  artist: string | null;
  album: string | null;
  description: string | null;
  audio_asset_id: string | null;
  cover_asset_id: string | null;
}>;

export async function updateTrack(id: string, patch: TrackPatch): Promise<Track> {
  return api<Track>(`/api/v1/tracks/${id}`, { method: "PATCH", body: JSON.stringify(patch) });
}

export async function deleteTrack(id: string): Promise<void> {
  await api<void>(`/api/v1/tracks/${id}`, { method: "DELETE" });
}

export async function publishTrack(id: string): Promise<Track> {
  return api<Track>(`/api/v1/tracks/${id}/publish`, { method: "POST" });
}

export async function unpublishTrack(id: string): Promise<Track> {
  return api<Track>(`/api/v1/tracks/${id}/unpublish`, { method: "POST" });
}

/* ── audio upload ─────────────────────────────────────────────────── */

/**
 * Upload an audio file and return the ready asset id, for attaching to a track.
 *
 * Same three-step asset flow the upload studio uses (create session → PUT the
 * original → confirm), but with no polling step: the API marks an `audio` asset
 * ready inside `/complete` because there is no transcode to wait for. That is
 * also why this lives here rather than in the video-shaped upload studio, whose
 * whole UI is "upload, wait for HLS, play it back in a video element".
 *
 * `onProgress` reports 0-100 for the byte-transfer step only.
 */
export async function uploadAudioAsset(
  file: File,
  onProgress?: (pct: number) => void,
): Promise<string> {
  if (!file.type.startsWith("audio/")) {
    throw new Error("Chỉ chấp nhận tệp âm thanh.");
  }

  const created = await api<{ asset: { id: string } }>("/api/v1/assets", {
    method: "POST",
    body: JSON.stringify({
      filename: file.name,
      content_type: file.type,
      size_bytes: file.size,
    }),
  });
  const assetId = created.asset.id;

  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", `${baseURL}/api/v1/assets/${assetId}/source`);
    xhr.withCredentials = true; // session cookie, cross-subdomain
    xhr.setRequestHeader("Content-Type", file.type);
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300
        ? resolve()
        : reject(new Error(`Tải lên thất bại (${xhr.status}).`));
    xhr.onerror = () => reject(new Error("Lỗi mạng khi tải lên."));
    xhr.send(file);
  });

  await api<unknown>(`/api/v1/assets/${assetId}/complete`, { method: "POST" });
  return assetId;
}

/* ── bulk import ──────────────────────────────────────────────────── */

export type MusicImportStatus = "pending" | "uploaded" | "processing" | "done" | "failed";

export interface MusicImportReportEntry {
  name: string;
  ok: boolean;
  track_id?: string;
  title?: string;
  error?: string;
}

export interface MusicImport {
  id: string;
  status: MusicImportStatus;
  total: number;
  succeeded: number;
  /** Entries that failed. Does not stop the job — the report says which. */
  failed: number;
  report: MusicImportReportEntry[];
  /** Job-level failure. Null unless `status` is "failed". */
  error: string | null;
  created_at: string;
  updated_at: string;
}

export function getImport(id: string): Promise<MusicImport> {
  return api<MusicImport>(`/api/v1/tracks/imports/${id}`);
}

/**
 * Upload a zip of audio files and start the import.
 *
 * Two requests on purpose: the job id has to exist before a multi-gigabyte body
 * starts moving, so a dropped connection leaves a job to retry against rather
 * than an orphaned upload. Returns the job; poll `getImport` for progress.
 *
 * `onProgress` reports 0-100 for the byte transfer only — unpacking happens on
 * the worker afterwards and is reported through the job's counters.
 */
export async function importZip(
  file: File,
  onProgress?: (pct: number) => void,
): Promise<MusicImport> {
  const job = await api<MusicImport>("/api/v1/tracks/imports", { method: "POST" });

  await new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", `${baseURL}/api/v1/tracks/imports/${job.id}/upload`);
    xhr.withCredentials = true; // session cookie, cross-subdomain
    xhr.setRequestHeader("Content-Type", "application/zip");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress?.(Math.round((e.loaded / e.total) * 100));
    };
    xhr.onload = () =>
      xhr.status >= 200 && xhr.status < 300
        ? resolve()
        : reject(new Error(uploadErrorMessage(xhr)));
    xhr.onerror = () => reject(new Error("Lỗi mạng khi tải lên."));
    xhr.send(file);
  });

  return getImport(job.id);
}

/**
 * Pull the RFC 7807 `detail` out of an XHR failure.
 *
 * The import limits (4 GiB, 2000 entries) are enforced server-side, so the
 * message that matters — "tệp zip 5.2 GB vượt giới hạn 4.0 GB" — arrives in the
 * body. A bare status code would send the user guessing.
 */
function uploadErrorMessage(xhr: XMLHttpRequest): string {
  try {
    const body = JSON.parse(xhr.responseText) as { detail?: string };
    if (body.detail) return body.detail;
  } catch {
    // non-JSON body; fall through
  }
  return `Tải lên thất bại (${xhr.status}).`;
}

/**
 * Ask the server to fill in cover art and any missing tags for every track an
 * import created — a second pass, because the cover half is slow (an ffmpeg
 * extraction plus a wait for the image pipeline) and the import is meant to be
 * fast. Resolves with how many tracks were queued.
 *
 * Enrichment only fills gaps: it never overwrites a title, artist, album or
 * cover that already has a value.
 */
export async function enrichImport(importId: string): Promise<number> {
  const r = await api<{ queued: number }>(`/api/v1/tracks/imports/${importId}/enrich`, {
    method: "POST",
  });
  return r.queued;
}

/** The same, for one track. Works for any track with an audio file. */
export async function enrichTrack(trackId: string): Promise<void> {
  await api<{ queued: number }>(`/api/v1/tracks/${trackId}/enrich`, { method: "POST" });
}

/** True while the job still has work left, i.e. the client should keep polling. */
export function importInFlight(job: MusicImport): boolean {
  return job.status === "pending" || job.status === "uploaded" || job.status === "processing";
}

/* ── filename metadata ────────────────────────────────────────────── */

export interface FilenameMeta {
  title: string;
  artist?: string;
}

/**
 * Guess title and artist from a filename.
 *
 * A deliberate port of `titleFromFilename` in
 * `backend/internal/modules/music/import.go`, kept in step with it. The zip
 * importer reads embedded tags with ffprobe and only falls back to this; the
 * browser has no ffprobe, so for a multi-file upload this IS the metadata. Two
 * paths producing different names for the same file is the kind of difference
 * nobody can explain to a user, so at minimum they agree on the filename rules.
 *
 * The remaining, honest gap: a multi-file upload cannot see embedded tags. A
 * file whose ID3 says "Bản Tình Ca" imports as "tagged-one" here and as
 * "Bản Tình Ca" through the zip.
 */
export function metaFromFilename(filename: string): FilenameMeta {
  const stem = stripTrackNumber(filename.replace(/\.[^.]+$/, "").trim());
  const parts = stem.split(" - ").map((p) => p.trim());

  if (parts.length <= 1) return { title: parts[0] || stem };
  // Only the FIRST separator splits artist from title — a dash inside the title
  // is common ("Paranoid - Android") and splitting on all of them truncates it.
  return { artist: parts[0], title: parts.slice(1).join(" - ") };
}

/**
 * Drop a leading position marker ("01 - ", "1. ", "003_").
 *
 * The number must be followed by a real separator, never a bare space: "01 Creep"
 * and "99 Luftballons" are the same shape, and renaming "99 Luftballons" to
 * "Luftballons" is a worse failure than leaving a track number in a title.
 */
function stripTrackNumber(stem: string): string {
  const m = /^(\d{1,3})\s*([.\-_])\s*(.+)$/.exec(stem);
  return m && m[3] ? m[3].trim() : stem;
}

/**
 * Ask MusicBrainz for what the files cannot say — release year, genre, and a
 * Cover Art Archive cover — for every track an import created.
 *
 * The only outbound third-party call in the app, and off unless an operator
 * enabled it: a 503 here means exactly that, and `problemDisplayMessage` carries
 * the reason. Throttled server-side to one request per second, so a few hundred
 * tracks take minutes; it is a background sweep, not an interactive call.
 *
 * `skipped` is non-zero when the per-sweep cap truncated the batch — surfaced
 * rather than swallowed, because a sweep that quietly covered half a library
 * would read as having covered all of it.
 */
export function lookupImport(importId: string): Promise<{ queued: number; skipped: number }> {
  return api<{ queued: number; skipped: number }>(
    `/api/v1/tracks/imports/${importId}/lookup`,
    { method: "POST" },
  );
}

/** The same, for one track. */
export async function lookupTrack(trackId: string): Promise<void> {
  await api<{ queued: number }>(`/api/v1/tracks/${trackId}/lookup`, { method: "POST" });
}

/* ── Playlists + bulk actions (migration 0041) ───────────────────── */

/**
 * One playlist: an owner's ordered selection of their own tracks. `tracks` is
 * present only on the single-playlist read.
 */
export interface Playlist {
  id: string;
  name: string;
  description: string | null;
  track_count: number;
  created_at: string;
  updated_at: string;
  tracks?: Track[];
}

export async function listPlaylists(): Promise<Playlist[]> {
  const r = await api<{ playlists?: Playlist[] }>("/api/v1/playlists");
  return r.playlists ?? [];
}

export async function getPlaylist(id: string): Promise<Playlist> {
  return api<Playlist>(`/api/v1/playlists/${id}`);
}

export async function createPlaylist(name: string, description?: string): Promise<Playlist> {
  return api<Playlist>("/api/v1/playlists", {
    method: "POST",
    body: JSON.stringify({ name, description: description ?? null }),
  });
}

/**
 * Rename, or set/clear the description.
 *
 * The PATCH contract treats an absent key as "leave alone" and an explicit
 * `null` as "clear", so only send what actually changed — passing the whole
 * object back would clobber a description this form does not show.
 */
export async function renamePlaylist(
  id: string,
  patch: { name?: string; description?: string | null },
): Promise<Playlist> {
  return api<Playlist>(`/api/v1/playlists/${id}`, {
    method: "PATCH",
    body: JSON.stringify(patch),
  });
}

export async function deletePlaylist(id: string): Promise<void> {
  await api<void>(`/api/v1/playlists/${id}`, { method: "DELETE" });
}

/**
 * Add a whole selection at once. Returns how many actually landed: tracks
 * already in the playlist are skipped, so "added" can be lower than what you
 * sent, and the UI should say so rather than claim everything worked.
 */
export async function addTracksToPlaylist(
  playlistID: string,
  trackIDs: string[],
): Promise<{ added: number; requested: number }> {
  return api<{ added: number; requested: number }>(`/api/v1/playlists/${playlistID}/tracks`, {
    method: "POST",
    body: JSON.stringify({ track_ids: trackIDs }),
  });
}

export async function removeTrackFromPlaylist(playlistID: string, trackID: string): Promise<void> {
  await api<void>(`/api/v1/playlists/${playlistID}/tracks/${trackID}`, { method: "DELETE" });
}

/**
 * Publish or unpublish many tracks in one request. The per-track route would
 * mean one HTTP call per row, which for an imported library of hundreds is both
 * slow and a burst the API has no reason to absorb.
 */
export async function bulkSetTrackStatus(
  trackIDs: string[],
  status: "published" | "draft",
): Promise<{ changed: number; requested: number }> {
  return api<{ changed: number; requested: number }>("/api/v1/tracks/bulk-status", {
    method: "POST",
    body: JSON.stringify({ track_ids: trackIDs, status }),
  });
}
