# SPEC-15 — Music Vertical (tracks, bulk import, enrichment, catalogue lookup, playlists, player)

**Status:** current, rev 2 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `music` (`backend/internal/modules/music/`) · **Depends on:** SPEC-04 (asset ingest, `/assets/{id}/original`, image variants for covers, `media:asset_deleted`); `platform/events` fan-out (SPEC-04 P0.6); SPEC-05 for the one consumer of its event; the playback-resume question is SPEC-10's (Decision 2026-09-30, Audio)
**Upstream:** as-built spec, written retroactively from shipped code — migrations `0022_music_core` (`f11cf3f`), `0038_music_imports` (`6bf7c0b`), `0039_music_metadata_lookup` (`eaa0c36`), `0041_music_playlists` (`c3f3d00`); `git log --oneline -- backend/internal/modules/music` lists the rest. No brief or earlier spec existed; the root [`CLAUDE.md`](../../../CLAUDE.md) "Bulk music import", "Cover art is a SECOND pass", "Catalogue lookup" and "`/assets/{id}/original` must stay a `ServeContent` route" bullets were the only written description and are restated here as requirements. · **Refs:** [ADR-04](SPEC-04-media-image-pipeline.md#adr-04) (same S3 client, MinIO dev / R2 prod), [ADR-07](SPEC-01-account-identity-admin.md#adr-07) (tenancy), [ADR-08](README.md#adr-08) (entertainment facet), [ADR-10](README.md#adr-10) (spec-first), feature-inventory `D-7` (Problem types), `D-20` (per-domain progress), `D-22` (genres), `D-29` (envelopes), `D-32`/`D-33`/`D-34` (frontend), [backlog.md](../backlog.md) P2 lines 28–29
**Downstream consumers:** SPEC-05 (`notify:on_track_published`, the bell); the `layout` module (home-rail widget key `music`, menu item `/library/music`, `0036`); SPEC-10 `/continue` (not today — §11 (b)); SPEC-03 P1.7 takeout (§6)

---

## 1. Problem statement

The entertainment facet of the life OS ([ADR-08](README.md#adr-08))
needs the owner's own music library in the app: hundreds of files that already
exist on disk, mostly as `NN - Artist - Title.mp3` with tags of uneven quality.
Uploading one file at a time through the video-shaped upload studio does not
scale, a track that cannot seek is unusable, and filling metadata by hand for a
whole library is not going to happen. The module therefore has to (1) ingest a
library in one action, (2) recover metadata from the filename, the file, and —
only when the operator opts in — a public catalogue, without ever overwriting
what the owner typed, and (3) play it with a queue that survives navigation.

The module shipped across four migrations without a spec, so its contract lived
in code comments, `shared/openapi.yaml` (partially) and `CLAUDE.md`. This spec
is the contract; §12 lists where the code still diverges from it.

## 2. Goals

1. The owner turns a zip of a few hundred audio files into playable tracks in
   one upload, with a per-file report, and every imported track is playable the
   moment its row exists (audio has no transcode step).
2. Metadata is filled in three passes of increasing cost — **import**
   (filename + embedded tags), **enrich** (embedded cover art + missing tags,
   local), **lookup** (MusicBrainz + Cover Art Archive, network, opt-in) — and
   every pass **only fills gaps**.
3. The one outbound third-party call in the codebase is safe by construction:
   off by default, one request per second across the whole deployment, an
   identifying User-Agent, and no guessed matches.
4. Playback seeks reliably (Range requests), and a queue keeps playing across
   route changes.
5. The owner groups their own tracks into ordered playlists and acts on a
   selection (publish, unpublish, add to playlist) in one request.

## 3. Non-goals

- **Albums and artists as entities.** `artist` and `album` are free-text columns
  on the track. `backend/internal/modules/music/README.md` still lists `albums`,
  `artists` and `playlist_entries` tables; none exist and none are planned by
  this spec.
- **Audio transcoding.** Audio is served as uploaded (backlog P2 line 29 keeps
  the "audio transcode profile" item). There is no HLS and no duration probe for
  audio (SPEC-10 §11 row 4 owns `duration_ms`).
- **Public, shared or collaborative playlists.** A playlist is one owner's
  selection of their own tracks (`0041` header; feature-inventory §5's
  "public/private, collaborative" stays parked with the social layer).
- **Persisted queue / playback state.** The queue lives in memory and a reload
  stops the music (`MusicPlayerProvider.tsx`). Resume is an open question (§11).
- **Live catalogue sync, scrobbling, lyrics, streaming from third parties.**
- **A catalogue for other users.** "Published" tracks are visible to members of
  the same tenant only (RLS, §6); with personal organisations (ADR-07) that is
  the owner. Cross-user playback is §11 (e).

## 4. User stories

- As the owner, I drop a 2 GB zip of my library and come back to 300 tracks, a
  report naming the four files that failed and why. *(primary)*
- As the owner, I pick twelve files in the browser and get twelve tracks named
  the same way the zip importer would have named them.
- As the owner, after an import I press "fetch covers" and the covers embedded
  in my files appear, without my hand-typed album names being touched.
- As the owner of a deployment where I enabled MusicBrainz, I ask it for release
  years and covers; tracks it is not sure about say "no confident match" instead
  of getting a wrong album.
- As the owner, I drag the progress bar to 3:10 and the song plays from 3:10.
- As the owner, I start an album in the library, walk to the newsfeed, and it
  keeps playing in the now-playing bar.
- As the owner, I select thirty tracks and add them to "Chill"; the app says 28
  were added because two were already there.
- Edge: a file with no tags imports under its filename; a file with no embedded
  art keeps no cover, and that is not an error.

## 5. Requirements

Permission codes are the ones `0022_music_core` seeds (and `0003_account_rbac`
before it): `music:read` (granted to `guest` and `user`), `music:write:own` and
`music:publish:own` (`creator`; widened to `user` by P1.4), `music:write:any`
and `music:publish:any` (`editor`), `music:delete:any` (`admin`). "Owner-or-X" routes use
`RequireOwnerOrPermission(engine, X, byTrack)` built in `cmd/api/main.go`; a
missing track answers 404 there. All routes sit under `authTenant`
(RequireAuth + RequireTenant), so every request runs in one tenant-scoped
transaction that rolls back on a 5xx.

### P0.1 — Module scaffold, tenancy, wiring

`music.New(Deps)` in `module.go`; constructed and mounted in `cmd/api/main.go`
(`musicMod.MountHTTP(r)`) and constructed with `RegisterTasks(lightMux)` in
`cmd/worker/main.go`. Routes mount conditionally so a binary without a
dependency has no dead routes: `/tracks/imports/*` needs `Storage` and
`Enqueuer`; `/tracks/{id}/enrich|lookup` need `Enqueuer`; `/playlists/*` needs
`Playlists`. The depguard block `module-music-isolation` (`backend/.golangci.yml`)
and the `musicrepo` block in `backend/sqlc.yaml` exist. Other modules import only
`music/api` (task and event names, payload structs).

**Tenancy.** All four tables are tenant-scoped with FORCE RLS (§6). Worker tasks
(`import_zip`, `enrich_track`, `lookup_track`, `on_asset_deleted`) carry the
owner's user id in the payload as a **routing hint, not an authorisation claim**
(`api.ImportZipPayload` comment) and open that owner's tenant scope
(`Deps.RunInTenant` = `cmd/worker`'s `runInUserTenant`, the user's personal
organisation) **before any query** — `portal_app` cannot read an RLS table
unscoped; it errors. The scopes are many and short (one to read the job, one per
zip entry, one to finish), never one transaction around a whole import, and a
byte stream copied from storage is never read while a transaction is open.
Requests and tasks agree on the tenant only because both resolve the user's
personal organisation (`RequireTenant` → `GetOrCreatePersonalOrg`;
`runInUserTenant` → `PersonalOrg`); if tenant selection (`D-23`) ever lands, the
three task payloads must carry the row's `tenant_id` instead.

*Acceptance criteria.*
- Given the worker running as `portal_app`, when any music task runs, then its
  first query runs inside the payload owner's tenant scope (§12 row 1 for the
  consumer that does not).
- Given an import of N entries where entry k fails, then entries 1…k−1 and
  k+1…N stay committed.

### P0.2 — Track CRUD and visibility

| Operation | Gate | Behaviour |
|---|---|---|
| `POST /tracks` | `music:write:own` | Body `{title, artist?, album?, description?, audio_asset_id?, cover_asset_id?}`. `title` is trimmed and must be 1–200 runes, else 422 `music/validation`. An audio id must name an asset that exists, is `kind=audio`, `status=ready` and is owned by the caller, else 422 `music/invalid-audio-asset`; a cover id the same with `kind=image`, else 422 `music/invalid-cover-asset` (`service.go` `validateAudioAsset` / `validateImageAsset`, through `mediaapi.GetAsset`). A malformed asset id, or a body field of the wrong JSON type, is 422 `music/validation` (§12 row 14). `""` for an id means none. 201 with the Track; `status = draft`. |
| `GET /tracks/{id}` | `music:read` | A published track, or any track of the caller's; anything else — including a malformed id — is 404 `music/not-found` (never 403). |
| `PATCH /tracks/{id}` | owner or `music:write:any` | Absent key = unchanged. `title`, when present, is re-validated (1–200 runes; cannot be cleared). `null` clears `artist`, `album`, `description`, `audio_asset_id`, `cover_asset_id`. A non-string value for a string field is 422 `music/validation`, never a silent clear. New asset ids are validated against the **track owner**, not the caller. Clearing `audio_asset_id` on a published track returns it to `draft` in the same UPDATE — the rule `NullAudioByAsset` already applies. 200 with the Track. |
| `DELETE /tracks/{id}` | owner or `music:delete:any` | 204; a repeat is 404 (`DeleteTrack` is `:execrows`). The audio and cover **assets are not deleted** — they stay in the media library; playlist rows cascade. |

**Track response** (every track-returning operation, `handler.go` `trackJSON`):
`{id, owner_id, title, artist|null, album|null, description|null,
audio_asset_id|null, cover_asset_id|null, status: draft|published, created_at,
updated_at, release_year|null, genre|null, mb_recording_id|null,
mb_release_id|null, lookup_status: none|pending|matched|no_match|failed,
lookup_note|null, lookup_at|null}`. A track carries **no duration** — the player
reads it from the `<audio>` element's metadata. The catalogue fields are
system-written (P0.8) and never accepted on POST or PATCH.

*Acceptance criteria.*
- Given another user's draft, then `GET /tracks/{id}` is 404 `music/not-found`,
  byte-identical to a never-existing id.
- Given a ready audio asset owned by someone else, or an image as the audio,
  then POST is 422 `music/invalid-audio-asset`.
- Given `PATCH {description: null}`, then the description is NULL afterwards.
- Given `PATCH {artist: 123}`, then 422 `music/validation` and the artist is
  unchanged.
- Given a published track and `PATCH {audio_asset_id: null}`, then the response
  has `audio_asset_id: null` and `status: draft`.
- Given a track deleted twice, then 204 then 404.

### P0.3 — Publish, unpublish, bulk status, the event

- `POST /tracks/{id}/publish` (owner or `music:publish:any`): the track must
  have an audio asset that is `kind=audio`, `ready` and owned by the track
  owner, else 422 `music/not-publishable`. Sets `published`, then emits
  **`music:track_published`** `{track_id, owner_user_id, title}` **after the
  request transaction commits**. Publishing an already-published track succeeds
  and emits again (notify dedups on the track id). A publisher error is logged,
  never surfaced.
- `POST /tracks/{id}/unpublish` (owner or `music:publish:any`): sets `draft`;
  no event.
- `POST /tracks/bulk-status` (`music:write:own`) `{track_ids: uuid[1..500],
  status: published|draft}`. The id list is filtered through the caller's own
  tracks (`OwnedTrackIDs`) before anything is touched, because no ownership
  middleware can sit in front of ids that arrive in a body. Publishing applies
  the **same publishability rule** as the single route per track: a track that
  fails it is skipped, not published. Answers 200 `{changed, requested,
  status}`, where `changed` counts tracks that actually moved. An empty list,
  more than 500 ids, a malformed id or an unknown status is 422
  `music/validation`. One `music:track_published` per newly published track,
  after commit.

*Acceptance criteria.*
- Given a track whose audio asset is `processing`, owned by someone else, or an
  image, then publish is 422 `music/not-publishable` and the status stays draft.
- Given a successful publish, then exactly one `music:track_published` is
  published, and none if the transaction rolls back.
- Given bulk publish of 3 owned tracks (one without audio) and 2 foreign ids,
  then `{changed: 2, requested: 5}`, two events, and the audio-less track is
  still draft.

### P0.4 — Reaping on `media:asset_deleted`

The consumer task **`music:on_asset_deleted`** (subscribed to
`media:asset_deleted` in both binaries) clears dangling references: a deleted
audio asset sets `audio_asset_id = NULL` **and `status = 'draft'`** (a published
track with no audio is broken); a deleted cover sets `cover_asset_id = NULL`.
It is idempotent and runs inside the tenant scope of the payload's
`owner_user_id` (P0.1). A payload that cannot be parsed is dropped (`return nil`).

*Acceptance criteria.*
- Given a published track whose audio asset is deleted, then after the consumer
  runs the track is draft with no audio; a second delivery changes nothing.
- Given the worker connected as `portal_app`, then the consumer succeeds (RLS
  test; §12 row 1).

### P0.5 — Multi-file upload path and filename parity

For a handful of files the client does not use the zip path. The "files" mode
of `BulkImportModal.tsx` accepts files whose MIME type starts with `audio/` or
whose extension is one of `.mp3 .m4a .aac .flac .ogg .oga .opus .wav .wma`, then
for each file **sequentially** (per-file progress, failures attributable):
`uploadAudioAsset` (`POST /assets` → `PUT /assets/{id}/source` → `POST
/assets/{id}/complete`; audio is marked `ready` inside `/complete`, so there is
no polling) → `metaFromFilename(file.name)` → `POST /tracks {title, artist,
audio_asset_id}`. A failed file is reported and the loop continues. A file
accepted by extension whose browser MIME type is empty is uploaded with the
content type its extension maps to (§12 row 18).

**Filename rules — one rule set, two implementations.** `metaFromFilename` in
`frontend/src/lib/music.ts` is a deliberate port of `titleFromFilename` in
`music/import.go`; both must produce the same `{title, artist}` for every
filename:

1. Strip the extension (the last `.` segment) and trim.
2. Strip a leading position marker of **1–3 digits** followed (after optional
   spaces) by `.`, `-` or `_` — never by a bare space (`"99 Luftballons"` stays
   whole). If nothing remains after the marker, keep the original stem.
3. Split on `" - "`. One part → title only. Two or more → the **first** part is
   the artist and the rest, re-joined with `" - "`, is the title
   (`"Radiohead - Paranoid - Android"` → artist `Radiohead`, title
   `Paranoid - Android`). A blank artist is absent, not `""`.

The multi-file path cannot read embedded tags (the browser has no ffprobe), so a
tagged file can get a different title here than through the zip; that is the
one accepted difference.

*Acceptance criteria.*
- Given the shared fixture table (TC-MUS-050), then `metaFromFilename` and
  `titleFromFilename` agree on every row, including `"01 - Artist - Title.mp3"`,
  `"1. Title.flac"`, `"003_Title.ogg"`, `"99 Luftballons.mp3"`, `"07.mp3"`,
  `"Song.mp3"`, a tab before the separator and a trailing dot.
- Given one file of five failing to upload, then four tracks exist and the
  fifth row shows its error.

### P0.6 — Zip import (`music:import_zip`, `0038`)

Three requests, because a multi-gigabyte archive cannot ride with the JSON that
describes it. All gated by `music:write:own` and owner-scoped; another user's
job, or a malformed job id, is 404 `music/import-not-found`.

1. `POST /tracks/imports` → 201 MusicImport, `status = pending`.
2. `PUT /tracks/imports/{id}/upload`, raw body (`application/zip`, not
   multipart). Only a `pending` job accepts an upload; any other status is 409
   `music/import-already-uploaded` and nothing is enqueued (§12 row 2). The body
   is spooled to a temp file (the S3 SDK needs a seekable body of known length);
   more than **4 GiB** is 422 `music/validation` with a detail naming both sizes,
   and nothing reaches storage. The archive is stored at
   `import/music/{id}.zip`, the job moves to `uploaded`, and
   `music:import_zip {import_id, owner_id}` is enqueued with `MaxRetry(0)` (a
   retry would duplicate every track already created) and `Timeout(6h)` (the
   default lease would expire mid-import and re-queue it). → 202 MusicImport.
3. `GET /tracks/imports/{id}` polls; `GET /tracks/imports?limit=` lists the
   caller's jobs newest first so a reload can find a running import.

**Worker** (`import.go` `RunImport`): reads the job in the owner's scope, waiting
up to 5 s (25 × 200 ms) for `upload_ref` to become visible (the enqueue precedes
the commit); copies the archive to a temp file; selects entries with
`audioEntries` — not directories, not `__MACOSX/` or `._*` AppleDouble stubs
(after normalising `\` to `/`, since Windows tools write backslashes), extension
(case-insensitive) in the accepted list — **sorted by full path** so a re-run
reports the same order. Job-level failures — no upload, unreadable object, not a
zip, more than **2000** entries, zero audio entries — record `status = failed`
with a Vietnamese `error` sentence, delete the stored zip, and return nil (no
Asynq retry). Otherwise `status = processing, total = N`, then per entry, each
in its own committed scope:

- more than **512 MiB** uncompressed, or a compression ratio above **50×**
  (zip-bomb guard), fails the entry;
- tags via `ffprobe -show_format` on a spooled copy (seekable, so a trailing
  ID3v1 block is reachable; 20 s timeout): `title`; artist from `artist`, then
  `album_artist`, then `performer`; `album`; tag keys lower-cased. Missing tags
  fall back to P0.5's filename rules, applied to the entry's **base name after
  `\` → `/` normalisation** (§12 row 9);
- `mediaapi.Ingest` with `origin = import` (§12 row 17) → an audio asset, ready
  immediately;
- `CreateTrack` (draft; title > 200 runes fails that entry). A failed entry
  rolls its scope back so no half-made track commits.

The job finishes `done` with `succeeded`, `failed` and `report` even when
entries failed (`failed` is reserved for "could not run at all"), and the stored
zip is deleted. Report entries: `{name, ok, track_id?, title?, error?}`.
Imported tracks are drafts with no cover (P0.7 adds covers).

*Acceptance criteria.*
- Given a zip with `album/01 - A - First.mp3`, `album/cover.jpg`,
  `__MACOSX/album/._01 - A - First.mp3` and `album/notes.txt`, then `total = 1`
  and the non-audio entries are neither imported nor reported.
- Given a Windows-made zip with `Album\01 - Artist - Title.mp3` and no tags, then
  the track is titled `Title` by `Artist`.
- Given a second `PUT …/upload` on a `done` job, then 409 and no new tracks.
- Given a 5 GiB body, then 422 `music/validation` and no object in storage.
- Given 2001 audio entries, then the job is `failed` and no track exists.

### P0.7 — Enrichment pass (`music:enrich_track`)

`POST /tracks/{id}/enrich` (owner or `music:write:any`) → a track without an
audio asset is 422 `music/validation`; otherwise `music:enrich_track
{track_id, owner_id}` is enqueued (`default` queue, `Timeout(10m)`,
`MaxRetry(1)` — a storage blip deserves one retry, a file with no art never
grows one) → 202 `{queued: 1}`. `POST /tracks/imports/{id}/enrich` queues one
task per `ok` report entry → 202 `{queued: n}`. The task's `owner_id` is the
**track owner** (so an elevated caller enriches someone else's track rather than
getting a 404 — §12 row 12).

**Worker** (`enrich.go` `EnrichTrack`): reads the track in the owner's scope;
does nothing for a track without audio, and nothing when it already has a
cover, an artist and an album. It spools the audio — the asset lookup inside the
scope (an unscoped `assets` read answers "asset not found"), the byte copy
outside it, at most 512 MiB — then reads the tags as P0.6 does. If the track has
no cover: `ffprobe -select_streams v` picks the stream whose disposition is
`attached_pic` (so a music video muxed into an m4a is not mistaken for art),
codec `mjpeg|png|webp`; `ffmpeg -an -c:v copy -frames:v 1` extracts it (60 s
timeout, no re-encode); at most 16 MiB; ingested as an image asset (origin
`import`) and polled every 1 s for up to **90 s** until `ready` (the image
pipeline runs on the `image` server; `validateImageAsset` would refuse a cover
that is not ready). A cover that fails or times out is simply not attached.

**Fill gaps only, atomically.** `enrichPatch` decides what may be written:
artist when blank, album when blank, cover when NULL — never the title. The
write itself must not overwrite a value the user typed between the read and the
write: it goes through a query that COALESCEs against the stored row, the way
P0.8's does (§12 row 8).

*Acceptance criteria.*
- Given a track with album "Typed by me" and a file tagged album "From tags",
  then after enrichment the album is "Typed by me".
- Given a PATCH of `artist` committed while an enrichment is running, then the
  enrichment does not overwrite it.
- Given a file with no attached picture, then the task succeeds and the cover
  stays NULL.

### P0.8 — Catalogue lookup pass (`music:lookup_track`, `0039`)

**Off by default.** The feature is active only when `MUSICBRAINZ_ENABLED=true`
**and** `MUSICBRAINZ_CONTACT` is non-blank (`LookupConfig.active`); there is no
default contact. While inactive, `POST /tracks/{id}/lookup` and `POST
/tracks/imports/{id}/lookup` answer **503 `music/lookup-disabled`** with a
detail naming both variables — the routes stay mounted, because a 404 would read
as "never built". The disabled check runs before the track is read.

**Enqueue.** Per track (owner or `music:write:any`; owner taken from the track
as in P0.7): set `lookup_status = pending`, enqueue `music:lookup_track
{track_id, owner_id}` (`default` queue, `Timeout(5m)`, `MaxRetry(2)` — somebody
else's server having a bad minute is worth retrying) → 202 `{queued: 1}`. Per
import: one per `ok` report entry up to **500**; the rest are counted in
`skipped`, never silently dropped → 202 `{queued, skipped}`.

**Worker** (`lookup.go` `LookupTrack`, `musicbrainz.go`):

- **Search.** An empty title → `no_match`. Query `recording:"<title>"` plus
  ` AND artist:"<artist>"` only when the artist is known; both terms quoted with
  `"` and `\` escaped; `GET {MUSICBRAINZ_BASE_URL}/recording?query=…&fmt=json&limit=5`,
  15 s HTTP timeout, body capped at 4 MiB.
- **No guessing.** Results are score-ordered; the first with **score ≥ 88** wins
  and the scan stops at the first below it. Below that the outcome is
  `no_match`, with a note naming the query used.
- **Picking.** Artist = first artist credit; release = the **earliest dated**
  release (a release without a date only wins when none has one); year = the
  leading four digits when 1860–2200; genre = the most-voted non-blank community
  tag (ties keep the first).
- **Cover.** Only when the track has no cover and a release id is known:
  `GET {COVERART_BASE_URL}/release/{id}/front-500`; 404 = no artwork; a non-image
  content type or more than 8 MiB = none; otherwise ingested and waited for as in
  P0.7.
- **Throttle — one request per second, globally.** Every MusicBrainz and Cover
  Art Archive request first takes the Redis slot `music:mb:slot` with
  `SET NX PX 1100`; a waiter polls every 150 ms and gives up after 30 s (an
  error → `failed`, retried). Redis unreachable or absent → a local 1.1 s sleep,
  which is correct for one worker only. A process-local ticker is not
  acceptable: replicas would multiply the rate.
- **User-Agent.** `Portal/1.0 ( <MUSICBRAINZ_CONTACT> )` on every request.
- **Outcomes.** `matched` (note `MusicBrainz score N`), `no_match`, or `failed`
  (transport error, non-200, or 429/503 reported as "rate limited"; note ≤ 200
  characters; the error is returned so Asynq retries). The write
  (`SetTrackLookupResult`) **COALESCEs every value field** — `release_year`,
  `genre`, `cover_asset_id`, `mb_recording_id`, `mb_release_id`, and
  `artist`/`album` through `NULLIF(…, '')` — so a populated field is never
  replaced, even by a caller that passes a value; `lookup_status`, `lookup_note`
  and `lookup_at` are always overwritten. A worker that has lookups disabled
  while the API enqueued one records `failed` with a note saying so instead of
  leaving the track `pending` (§12 row 10).

*Acceptance criteria.*
- Given both variables unset, or `MUSICBRAINZ_ENABLED=true` with a blank
  contact, then both lookup routes answer 503 `music/lookup-disabled`.
- Given a best result scored 87, then `lookup_status = no_match` and no field
  changes.
- Given two worker replicas sweeping 10 tracks, then no two outbound requests
  start within 1.1 s of each other.
- Given a track whose album the user typed, and a match with another album, then
  the album is unchanged and `release_year`/`genre` are filled.
- Given an import of 600 tracks, then `{queued: 500, skipped: 100}`.

### P0.9 — Playlists (`0041`)

Every query is owner-scoped in its own predicate, on top of the tenant fence
(inside a shared org the fence alone would not say whose playlist it is).

| Operation | Gate | Behaviour |
|---|---|---|
| `GET /playlists` | `music:read` | The caller's playlists ordered `name, id`, each with `track_count`. Non-paginated. |
| `POST /playlists` | `music:write:own` | `{name, description?}`; name trimmed, 1–120 runes, else 422 `music/invalid-playlist`; unique per owner case- and edge-space-insensitively (`lower(btrim(name))`) → 409 `music/playlist-exists`. The insert is `ON CONFLICT DO NOTHING` (a raised unique would abort the tenant transaction and turn the 409 into a 500 at commit). 201 Playlist. |
| `GET /playlists/{id}` | `music:read` | The playlist plus `tracks` in playlist order (`position, id`) and `track_count`; another owner's or a malformed id → 404 `music/playlist-not-found`. |
| `PATCH /playlists/{id}` | `music:write:own` | `name?` (same rules; collision pre-checked → 409), `description?` — absent unchanged, `null` clears. |
| `DELETE /playlists/{id}` | `music:write:own` | 204; tracks untouched; repeat → 404. |
| `POST /playlists/{id}/tracks` | `music:write:own` | `{track_ids: uuid[1..500]}`. Ids the caller does not own are dropped before any write; a track already present keeps its position; new tracks append at `max(position)+1` **in the order the request lists them**. 200 `{added, requested}`. |
| `DELETE /playlists/{id}/tracks/{trackId}` | `music:write:own` | 204, also when the track was not in the playlist; 404 when the playlist is not the caller's. |

Deleting a track removes it from every playlist (FK cascade).

*Acceptance criteria.*
- Given a playlist "Chill", then creating " chill " is 409 and the database
  holds one row.
- Given `{track_ids: [B, A, C]}` on an empty playlist, then the detail lists B,
  A, C.
- Given a selection of 30 with 2 already present and 1 foreign, then
  `{added: 27, requested: 30}`.
- Given `PATCH {description: null}`, then the description is NULL.

### P0.10 — Playback

- **Source.** A track plays from `GET /api/v1/assets/{audio_asset_id}/original`
  (`lib/music.ts` `trackAudioURL`), owned by media. That route **must stay an
  `http.ServeContent` route** over the lazy `objectReader`
  (`media/objectreader.go`): it answers `Accept-Ranges`, `206` and
  `Content-Range`, and `Seek` costs nothing until the next `Read`. A copy loop
  makes the browser report `seekable = [0,0]` and silently refuse every seek.
  `Content-Disposition: inline`, `Cache-Control: private, no-store`. The route is
  owner-only (a non-owner gets 403 from media), so a published track plays for
  its owner only — §11 (e). Its state and size rules are SPEC-04's (§11 rows 4–6
  there).
- **Covers** use the public variant route `/assets/{cover_asset_id}/variants/{thumb|medium}`.
- **Player.** `MusicPlayerProvider` owns exactly one `<audio>` for the whole app,
  mounted in `MasterBase` above the router, so playback survives navigation.
  State: queue (in play order), index, playing, position, duration, shuffle
  (re-rolled with the current track first; turning it off restores the original
  order), repeat `off|one|all`, error (autoplay refusal, decode error, 404).
  Tracks without audio are filtered out of a queue. `NowPlayingBar` renders it;
  `jumpTo(index)` addresses the queue as displayed.
- **Play all** walks the list cursor to its end, at most 1000 tracks
  (`fetchAllTracks`), and says so when it truncated.
- **No progress is saved.** Music never calls media's progress API; resume and
  `/continue` membership are §11 (b).

*Acceptance criteria.*
- Given a ready audio asset, then `GET …/original` with `Range: bytes=1000-1999`
  answers 206 with `Content-Range: bytes 1000-1999/<size>` (media's
  `TestServeContentAnswersRangeRequests`).
- Given playback started on `/library/music`, when the user navigates to `/`,
  then audio continues and the bar stays.

### P0.11 — Frontend library

Routes `/library/music`, `/library/music/{id}`, `/library/music/playlists`,
`/library/music/playlists/{id}`: each `page.tsx` is an RSC shell exporting
`metadata` and rendering `activeTemplate().views.libraryMusic` /
`libraryMusicDetail` / `libraryMusicPlaylists` / `libraryMusicPlaylistDetail`
(declared in `templates/types.ts`); data is fetched in client islands with
TanStack (`["tracks", …]`, `["playlists"]`). The D-34 matcher covers them through
`'/library/:path*'`.

- Index: tabs **Library** (published) and **Mine** (`/tracks/mine`; a 403 —
  callers without `music:write:own` — hides the tab's actions); create-track
  modal with an in-dialog audio upload and a picker of ready audio assets; on
  **Mine**, multi-select → bulk publish / unpublish / add to playlist; the import
  modal (P0.5 files, P0.6 zip with a 1.5 s poll while `pending|uploaded|processing`,
  then "fetch covers" and "look up" buttons for the finished job).
- Detail: metadata, edit (only changed fields; blank → `null`), publish toggle,
  delete, play.
- Home rail: widget key `music` (`components/widget/registry.ts` →
  `MusicWidget`): the five newest **published** playable tracks, queued from the
  clicked row; renders nothing on a query error and an empty state on an empty
  list. Its `PlaylistWidget` wrapper still carries fixture `SAMPLE_TRACKS` as a
  prop default (always overridden here).

*Acceptance criteria.*
- Given a signed-out visitor on any `/library/music*` path, then they are
  redirected to `/login`.
- Given a fresh owner with no tracks, then Library and the widget show their
  empty states.

### P1 — next

- **P1.1 Show what the passes found.** The detail view shows `release_year`,
  `genre`, `lookup_status` (`no_match` styled as ordinary, `failed` as an
  error) and `lookup_note`, and offers per-track **Fetch cover** and **Look up**
  (the two per-track routes have no UI caller today).
  *AC:* given a track with `lookup_status = no_match` and a note, then the note
  renders without error styling.
- **P1.2 Resume an import after reload.** The import modal reads `GET
  /tracks/imports` and re-attaches to a job still `pending|uploaded|processing`
  (the reason that route exists; nothing calls it).
  *AC:* given an import in `processing`, when the page reloads and the modal
  opens, then it shows that job's progress.
- **P1.3 `bulk` queue.** `music:import_zip`, `music:enrich_track` and
  `music:lookup_track` leave the light server's `default` queue for the
  dedicated `bulk` server SPEC-14 P1.7 specifies (§12 row 11).
- **P1.4 `user` may author music** *(Decision 2026-10-01b (D4); unbuilt — §12
  row 26)*. A music-owned migration `000N_music_user_write_grant` (the
  `0025_comic_user_write_grant` shape: `WITH grants(...)`, `ON CONFLICT DO
  NOTHING`, idempotent) grants `music:write:own` and `music:publish:own` to
  `user`, so a second approved account can create tracks, import, enrich, look
  up and keep playlists — playlists are personal, not catalogue. `:any` codes
  and `music:delete:any` stay with `editor` / `admin`. Uploading audio and
  covers also needs F009 — `user` holding `assets:write:own` (SPEC-04 §11
  row 9) — once uploads enforce that code. The down migration deletes only
  those two `role_permissions` rows.
  *AC:* given an account holding only `user`, then `POST /tracks`, `POST
  /tracks/imports` and `POST /playlists` succeed and publishing its own track
  is 200; PATCH on another member's track is still 403 *(TC-MUS-004)*.

### P2 — design for, don't build

- Playback resume and `/continue` (§11 (b)); Media Session API (lock-screen
  controls); playlist reordering (positions are already sparse); a "look up
  everything never looked up" sweep (the unused `ListTracksNeedingLookup` query
  and `music_tracks_lookup_pending_idx` were written for it — §12 row 21);
  audio duration on the track once SPEC-10 §11 row 4 stores `duration_ms`.

## 6. Data model

All four tables are **tenant-scoped**: `tenant_id uuid NOT NULL DEFAULT
current_setting('app.current_tenant')::uuid REFERENCES organizations(id)`,
`<table>_tenant_idx`, `ENABLE` + `FORCE ROW LEVEL SECURITY`, policy
`tenant_isolation` (USING and WITH CHECK). `0022` created `music_tracks` that
way from birth; `0038` and `0041` add the column in three steps (add nullable →
set default → set NOT NULL) because a single `ADD COLUMN … DEFAULT
current_setting(…)` fails during a migration, where the GUC is unset. Owner FKs
are the sanctioned `users(id) ON DELETE CASCADE` identity anchor; asset columns
carry **no** cross-module FK — validated through `mediaapi`, reaped through
`media:asset_deleted`.

| Table | Created by | Columns (beyond `id uuid PK DEFAULT uuid_generate_v4()`, `tenant_id`, `created_at`, `updated_at`) | Constraints and indexes |
|---|---|---|---|
| `music_tracks` | `0022_music_core`; `0039` adds the catalogue columns | `owner_user_id` FK users; `title text NOT NULL`; `artist`, `album`, `description text`; `audio_asset_id`, `cover_asset_id uuid`; `status text NOT NULL DEFAULT 'draft'`; `release_year int`; `genre text`; `mb_recording_id`, `mb_release_id uuid`; `lookup_status text NOT NULL DEFAULT 'none'`; `lookup_note text`; `lookup_at timestamptz` | `CHECK (char_length(title) BETWEEN 1 AND 200)`; `CHECK (status IN ('draft','published'))`; `CHECK (lookup_status IN ('none','pending','matched','no_match','failed'))`; `music_tracks_release_year_check` (NULL or 1860–2200). Indexes: `music_tracks_status_updated_idx (status, updated_at DESC, id DESC)` (published list), `music_tracks_owner_idx (owner_user_id)`, `music_tracks_tenant_idx`, partial `music_tracks_lookup_pending_idx (owner_user_id) WHERE lookup_status IN ('none','pending')` (unused — §12 row 21). |
| `music_imports` | `0038_music_imports` | `owner_user_id` FK users; `status text NOT NULL DEFAULT 'pending'`; `upload_ref text`; `total`, `succeeded`, `failed int NOT NULL DEFAULT 0`; `report jsonb NOT NULL DEFAULT '[]'`; `error text` | `CHECK (status IN ('pending','uploaded','processing','done','failed'))`. Indexes: `music_imports_owner_idx (owner_user_id, created_at DESC)`, `music_imports_tenant_idx`. |
| `music_playlists` | `0041_music_playlists` | `owner_user_id` FK users; `name text NOT NULL`; `description text` | `CHECK (char_length(btrim(name)) BETWEEN 1 AND 120)`; `UNIQUE INDEX music_playlists_owner_name_idx (owner_user_id, lower(btrim(name)))`; `music_playlists_tenant_idx`. |
| `music_playlist_tracks` | `0041_music_playlists` | `playlist_id` FK `music_playlists ON DELETE CASCADE`; `track_id` FK `music_tracks ON DELETE CASCADE`; `position integer NOT NULL` (sparse, append = max+1); `added_at timestamptz` (no `created_at`/`updated_at`) | `PRIMARY KEY (playlist_id, track_id)`; `music_playlist_tracks_order_idx (playlist_id, position)`, `music_playlist_tracks_track_idx (track_id)`, `music_playlist_tracks_tenant_idx`. No owner column: reachable only through its playlist. |

The `report` column is written through a `::text::jsonb` cast so sqlc types the
parameter as a Go string (a `[]byte` goes out as `bytea` under
`QueryExecModeExec` and jsonb rejects it). Every UPDATE sets `updated_at =
now()` explicitly, including the lookup marks — so the track list order (keyed
on `updated_at`) shifts when a pass writes; a cursor walk running during a sweep
can skip a track that moved ahead of the cursor. Accepted at n=1.

**Permissions seeded.** `0022` inserts `music:read`, `music:write:own`,
`music:publish:own`, `music:write:any`, `music:publish:any`, `music:delete:any`
(`ON CONFLICT DO NOTHING` over `0003`'s `music:read`, `music:write:own`,
`music:write:any`, `music:publish`, `music:delete:any`) and grants as listed in
§5. `music:publish:own` and `music:publish` are required by no route (§12
row 20).

**Takeout** (SPEC-03 P1.7, interface unbuilt): `music_tracks` → JSON rows plus
the audio and cover originals through media's export; `music_playlists` +
`music_playlist_tracks` → JSON, one object per playlist with its ordered track
ids; `music_imports` **excluded** — a receipt of a past job; the tracks it
created are exported as tracks.

## 7. API summary

All paths are under `/api/v1`, all `security: [{bearerAuth: []}]`, all behind
`authTenant`. Every operation carries `x-required-permission` per the specs
README AuthZ encoding (owner-or routes: `{owner_or: <code>}`). Every
`music:write:own` row below is open to `user` once P1.4 lands (`creator` and up
on `HEAD`).

| Method | Path | Permission | Request | Success | Errors |
|---|---|---|---|---|---|
| GET | `/tracks?cursor=&limit=` | `music:read` | — | 200 `{items: Track[], next_cursor?}` | 400 `music/invalid-cursor` · 422 `music/validation` |
| GET | `/tracks/mine?cursor=&limit=` | `music:write:own` | — | 200 `{items: Track[], next_cursor?}` | 400 `music/invalid-cursor` · 403 · 422 `music/validation` |
| POST | `/tracks` | `music:write:own` | `TrackCreate` | 201 Track | 403 · 422 `music/validation` / `music/invalid-audio-asset` / `music/invalid-cover-asset` |
| GET | `/tracks/{id}` | `music:read` | — | 200 Track | 404 `music/not-found` |
| PATCH | `/tracks/{id}` | owner or `music:write:any` | `TrackPatch` | 200 Track | 403 · 404 `music/not-found` · 422 (as POST) |
| DELETE | `/tracks/{id}` | owner or `music:delete:any` | — | 204 | 403 · 404 `music/not-found` |
| POST | `/tracks/{id}/publish` | owner or `music:publish:any` | — | 200 Track | 403 · 404 · 422 `music/not-publishable` |
| POST | `/tracks/{id}/unpublish` | owner or `music:publish:any` | — | 200 Track | 403 · 404 |
| POST | `/tracks/bulk-status` | `music:write:own` | `{track_ids: uuid[1..500], status}` | 200 `{changed, requested, status}` | 403 · 422 `music/validation` |
| POST | `/tracks/{id}/enrich` | owner or `music:write:any` | — | 202 `{queued: 1}` | 403 · 404 · 422 `music/validation` (no audio) |
| POST | `/tracks/{id}/lookup` | owner or `music:write:any` | — | 202 `{queued: 1}` | 403 · 404 · 503 `music/lookup-disabled` |
| POST | `/tracks/imports` | `music:write:own` | — | 201 MusicImport | 403 |
| GET | `/tracks/imports?limit=` | `music:write:own` | — | 200 `{items: MusicImport[]}` (newest first) | 403 · 422 `music/validation` |
| GET | `/tracks/imports/{id}` | `music:write:own` | — | 200 MusicImport | 403 · 404 `music/import-not-found` |
| PUT | `/tracks/imports/{id}/upload` | `music:write:own` | raw `application/zip` | 202 MusicImport | 403 · 404 `music/import-not-found` · 409 `music/import-already-uploaded` · 422 `music/validation` (over 4 GiB) |
| POST | `/tracks/imports/{id}/enrich` | `music:write:own` | — | 202 `{queued}` | 403 · 404 `music/import-not-found` |
| POST | `/tracks/imports/{id}/lookup` | `music:write:own` | — | 202 `{queued, skipped}` | 403 · 404 `music/import-not-found` · 503 `music/lookup-disabled` |
| GET | `/playlists` | `music:read` | — | 200 `{items: Playlist[]}` | — |
| POST | `/playlists` | `music:write:own` | `{name, description?}` | 201 Playlist | 403 · 409 `music/playlist-exists` · 422 `music/invalid-playlist` |
| GET | `/playlists/{id}` | `music:read` | — | 200 `Playlist & {tracks: Track[]}` | 404 `music/playlist-not-found` |
| PATCH | `/playlists/{id}` | `music:write:own` | `{name?, description?}` | 200 Playlist | 403 · 404 · 409 · 422 `music/invalid-playlist` |
| DELETE | `/playlists/{id}` | `music:write:own` | — | 204 | 403 · 404 `music/playlist-not-found` |
| POST | `/playlists/{id}/tracks` | `music:write:own` | `{track_ids: uuid[1..500]}` | 200 `{added, requested}` | 403 · 404 · 422 `music/validation` |
| DELETE | `/playlists/{id}/tracks/{trackId}` | `music:write:own` | — | 204 | 403 · 404 `music/playlist-not-found` |

**Shapes.** `MusicImport` = `{id, status, total, succeeded, failed, report:
[{name, ok, track_id?, title?, error?}], error|null, created_at, updated_at}`
(`report` is re-emitted as parsed JSON; an unreadable blob degrades to `[]`).
`Playlist` = `{id, name, description|null, track_count, created_at,
updated_at}`. Playlist detail's `tracks` and an import's `report` are named
arrays of a composite read and stay named (specs README Pagination).

**Pagination.** Track lists: keyset on (`updated_at DESC`, `id DESC`), opaque
base64 cursor; `limit` default **30**, max **50**. Import list: no cursor;
`limit` default **10**, max **50**. Both lenient per the specs README rule —
missing, non-integer or < 1 → the default; above the max → clamped to the max;
never a Problem. Playlists are not paginated. A malformed cursor is 400
`music/invalid-cursor`; any other body or parameter shape failure is 422
`music/validation`. Malformed JSON stays 400 `about:blank` (`server.Decode`,
shared by every module).

**Problem types** (each registered in `frontend/src/lib/problems.ts`):
`music/not-found`, `music/validation`, `music/invalid-cursor`,
`music/invalid-audio-asset`, `music/invalid-cover-asset`,
`music/not-publishable`, `music/import-not-found`,
`music/import-already-uploaded`, `music/lookup-disabled`,
`music/playlist-not-found`, `music/playlist-exists`, `music/invalid-playlist`.

## 8. Events and tasks

The worker's **light server** (`cmd/worker/main.go`, `lightMux`) runs every music
task: Concurrency 4, queues `thumbnail` weight 3 / `default` weight 1 — the same
pool and queue as `notify:dispatch`. The heavy (transcode) and image servers are
never used by a music task; the covers music ingests are processed by media's
`media:process_image` on the image server.

| Name | Kind | Payload | Enqueued / emitted by | Queue · options | Registered |
|---|---|---|---|---|---|
| `music:track_published` | event | `{track_id, owner_user_id, title}` | `Service.Publish`, `BulkSetStatus` (API) | consumer edge on `default` | `Subscribe(musicapi.EventTrackPublished, notifyapi.TaskOnTrackPublished)` in **both** `cmd/api` and `cmd/worker`; one consumer: `notify:on_track_published` (bell "<title> is published", `href /library/music/{id}`, `dedup_key` = track id). Not projected into the stream since `0040`. |
| `music:on_asset_deleted` | task (consumer) | `{asset_id, owner_user_id}` | `media:asset_deleted` fan-out | `default` | `Subscribe(media.EventAssetDeleted, musicapi.TaskOnAssetDeleted)` in both binaries |
| `music:import_zip` | task | `{import_id, owner_id}` | `SaveImportZip` (API) | `default` · `Timeout(6h)` · `MaxRetry(0)` | `RegisterTasks` on the light mux |
| `music:enrich_track` | task | `{track_id, owner_id}` | `EnqueueEnrich`, `EnqueueEnrichForImport` (API) | `default` · `Timeout(10m)` · `MaxRetry(1)` | same |
| `music:lookup_track` | task | `{track_id, owner_id}` | `EnqueueLookup`, `EnqueueLookupForImport` (API) | `default` · `Timeout(5m)` · `MaxRetry(2)` | same |

**Drift with [events.md](../../reference/events.md)** (as of 2026-10-01): the
three tasks `music:import_zip`, `music:enrich_track` and `music:lookup_track` are
**absent** from its Tasks table; the `music:on_asset_deleted` row omits "subscribes
to `media:asset_deleted`" (the movie row has it) and does not say it runs
unscoped today (§12 row 1); the `music:track_published` and
`notify:on_track_published` rows match. The `cmd/worker` comment next to
`musicMod.RegisterTasks` names two of the four tasks. Music also *causes*
`media:asset_ready` for every asset it ingests (audio at import, covers at
enrich/lookup), with `origin='upload'` today (§12 row 17).

## 9. Configuration

| Variable | Default | Read by | Effect |
|---|---|---|---|
| `MUSICBRAINZ_ENABLED` | `false` | `platform/config` → `music.LookupConfig.Enabled` in both binaries | Lookup is active only when this is `true` **and** the contact is non-blank. |
| `MUSICBRAINZ_CONTACT` | `""` | same | Email or URL placed in the User-Agent. No default by design. |
| `MUSICBRAINZ_BASE_URL` | `https://musicbrainz.org/ws/2` | same | Point at a local mirror for volume or tests. |
| `COVERART_BASE_URL` | `https://coverartarchive.org` | same | Cover Art Archive base. |
| `IMAGE_CONCURRENCY` | `3` | `cmd/worker` | Image-server pool that processes ingested covers (P0.7, P0.8 wait on it). |

`.env.example` documents the four music variables with lookup off. Both
binaries must agree on `MUSICBRAINZ_ENABLED`: the API decides 503-or-enqueue,
the worker decides whether to call out (§12 row 10). The worker image installs
`ffmpeg` (which ships `ffprobe`) — a hard dependency for P0.6 and P0.7; the
`api` image has none and runs no music probe. Redis (Dragonfly) carries the
throttle slot. The import, enrichment and lookup limits are constants in
`import.go`, `enrich.go`, `lookup.go` and `musicbrainz.go`, not configuration:
4 GiB / 2000 entries / 512 MiB / 50× ratio / 6 h; 16 MiB cover / 90 s cover
wait / 10 min; 500 per sweep / 5 min; score 88 / 1100 ms slot / 30 s throttle
wait / 15 s HTTP / 8 MiB cover.

## 10. Success metrics (n=1 honest)

- Leading: the owner's real library (≥ 1 zip of ≥ 100 tracks) imports in one
  job with `failed` counting only genuinely unreadable files, checked against
  the report.
- Leading: on that library, the share of tracks with an artist after import +
  enrich, and the share with a cover after enrich (read from `music_tracks`).
- Guardrail: in a sample of 20 `matched` lookups, zero wrong albums; zero
  MusicBrainz 429/503 responses in worker logs over a full-library sweep.
- Lagging: seeking works on every supported container in Chrome and Firefox
  (manual), and the owner uses playlists (≥ 1 with ≥ 10 tracks) after a month.

## 11. Open questions

(a) — whether `user` may author music — was decided on 2026-10-01: yes
(Decision 2026-10-01b (D4), P1.4); the remaining questions keep their letters
so citations hold.

- **(b) Resume and `/continue` for tracks.** SPEC-10 (Decision 2026-09-30,
  Audio) makes audio *assets* playable with progress. Should the music player
  save progress through media's progress API on the track's audio asset (tracks
  then resume and join `/continue` as media items), or should music own
  `music.listen_progress` as D-20 sketched?
- **(c) Genre model vs D-22.** D-22 says genres are a closed per-module
  enumeration in `genre TEXT[]`; the lookup stores one free-text MusicBrainz
  folksonomy tag in `genre text`. Ratify the divergence for music, or map tags
  onto a seed list?
- **(d) Bulk publish and the bell.** Publishing 300 imported tracks queues 300
  `notify:on_track_published` notifications to their own owner. Should a bulk
  publish emit one summary event (or none), and is a self-notification for a
  self-publish wanted at all?
- **(e) Who can play a published track?** `/assets/{id}/original` is
  owner-only, so a published track is listed to tenant members who cannot play
  it. Use `mediaapi.SignedURL` (built for this, uncalled) for published tracks,
  open `/original` to tenant members for assets referenced by a published
  track, or drop "published" from the music UI until there is a second user?

## 12. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top of it change no
code: `git diff --stat 99b5a0b HEAD -- backend frontend shared` is empty). The
spec text above is the target; this section lists every place the shipped code
still diverges from it. Rows are ordered by severity: data loss or silently
wrong data first, then integrity, authorisation, contract, UX, hygiene; row 25,
found last, is appended (AuthZ) rather than renumbering the others, and so is
row 26 (P1, added by Decision 2026-10-01b). A row
closes when the code matches the requirement it cites and its
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) row is regraded
on a named test. Paths are relative to `backend/internal/modules/music/` unless
they start with `backend/`, `frontend/` or `shared/`. Proposed test ids are
`TC-MUS-NNN` (no case document exists yet). Every row's source is "Found while
writing SPEC-15, 2026-10-01" unless it names another.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.4 consumer tenancy; P0.1 | `music:on_asset_deleted` opens the payload owner's tenant scope before its UPDATEs. | `module.go` `handleAssetDeleted` discards `owner_user_id`; `service.go` `HandleAssetDeleted` calls `repo.NullAudioByAsset` / `NullCoverByAsset` on the pool with no transaction in the context (`platform/db.Conn` falls back to `pool.Exec`). Under `portal_app` with FORCE RLS the policy's `current_setting('app.current_tenant')` errors, so the task fails every retry: a deleted audio asset leaves a **published track pointing at nothing**. The unit tests use a fake repo and cannot see it. `comic`, `movie` and `story` consumers have the same shape. | **backend:** parse `owner_user_id`; run both UPDATEs inside one `runInTenant(ctx, owner, …)` (the worker already passes `RunInTenant`); unparseable owner → log and drop. The same fix in the comic, movie and story consumers (their specs' rows). **test:** TC-MUS-041 — an RLS test (`platform/db`, `RLS_TEST_APP_URL`) that runs the consumer as `portal_app`. | — |
| 2 | P0.6 upload once | Only a `pending` job accepts an upload; otherwise 409 `music/import-already-uploaded`, nothing stored or enqueued. | `import.go` `SaveImportZip` checks ownership only; `query/imports.sql` `SetMusicImportUpload` has no status predicate. A second `PUT …/upload` on a `processing` or `done` job stores a new zip and enqueues a second `music:import_zip`, **creating every track again**. | **backend:** `SetMusicImportUpload … WHERE id = $id AND status = 'pending'`, `:one` → no row = `ErrImportAlreadyUploaded`, checked **before** `store.Put`; map to 409. **openapi:** 409 on `uploadMusicImportZip`. **frontend:** `problems.ts` slug. **test:** TC-MUS-063. | — |
| 3 | P0.3 bulk publish rule | Bulk publish applies the single route's publishability rule per track and skips failures; `changed` counts moved tracks. | `playlists.go` `BulkSetStatus` calls `repo.SetStatus(id, published)` for every owned id with no audio check, so tracks **without audio** (or with a foreign / unready asset) become published and emit `music:track_published`. | **backend:** for `published`, run `validateAudioAsset` per owned track (or one SQL filter + one `mediaapi.AssetStatuses` batch) and skip failures. **test:** TC-MUS-034. | — |
| 4 | P0.3 event after commit; bulk partial failure | `music:track_published` is published only after the request transaction commits; a mid-loop failure publishes nothing. | `service.go` `emitPublished` runs inside `Publish` and inside `BulkSetStatus`'s loop, i.e. before `RequireTenant` commits. A failed `SetStatus` mid-loop returns the error → `writePlaylistErr` → 500 → the transaction rolls back, yet the events for the earlier tracks are already enqueued; the code comment ("the caller is told what did change") is not what happens. | **backend:** collect events and publish them from an after-commit hook (the SPEC-07 §11 row 1 fix shape); on error return the Problem only. **test:** TC-MUS-035 (rollback publishes nothing), TC-MUS-036 (bulk mid-loop failure: no events, no changes). | SPEC-07 §11 row 1 pattern; CC-5 |
| 5 | P0.2, P0.9 `null` clears description | `PATCH {description: null}` clears a track's and a playlist's description. | Track: `handler.go` `UpdateTrack` decodes `Description *string` (null and absent are both nil) and `query/music.sql` `UpdateTrack` uses `COALESCE(sqlc.narg('description'), description)`, so null is a no-op — and the detail form sends `null` to clear (`MusicDetailView.tsx` `save`), so **a description can never be removed**. Playlist: `playlists_handler.go` `UpdatePlaylist` passes `body.Description != nil` as "set", false for `null`, despite its comment. `shared/openapi.yaml` `TrackPatch` and `lib/music.ts` both promise null clears. | **backend:** decode `description` as `json.RawMessage` with `parseRawOptStr` and a `set_description` CASE in `UpdateTrack`; same in `UpdatePlaylist`. **test:** TC-MUS-014, TC-MUS-093. | — |
| 6 | P0.2 field types | A non-string `artist`/`album` is 422 `music/validation`, never a silent clear. | `handler.go` `parseRawOptStr` returns `(nil, true)` when the value is not a JSON string, so `{artist: 123}` **clears** the artist. | **backend:** return a parse error → 422 `music/validation`. **test:** TC-MUS-015. | — |
| 7 | P0.2 clearing audio | Clearing `audio_asset_id` on a published track returns it to `draft` in the same UPDATE. | `query/music.sql` `UpdateTrack` sets `audio_asset_id` and leaves `status`; a published track with no audio results (the state `NullAudioByAsset` exists to prevent). | **backend:** `status = CASE WHEN @set_audio AND sqlc.narg('audio_asset_id') IS NULL THEN 'draft' ELSE status END`. **test:** TC-MUS-016. | — |
| 8 | P0.7 atomic gap-fill | Enrichment writes through a COALESCE against the stored row; a value committed after the read survives. | `enrich.go` `EnrichTrack` reads the track, builds `enrichPatch`, then writes through the user-facing `UpdateTrack`, whose `CASE WHEN set_artist THEN $artist` overwrites — a PATCH that lands during the up-to-90 s cover wait is **overwritten**. | **backend:** a dedicated `EnrichTrack` query: `artist = COALESCE(NULLIF(artist,''), $artist)`, same for album, `cover_asset_id = COALESCE(cover_asset_id, $cover)`; keep `enrichPatch` for deciding whether to write. **test:** TC-MUS-073. | — |
| 9 | P0.6 Windows entry names | The filename fallback uses the entry's base name after `\` → `/` normalisation. | `import.go` `importOne` uses `path.Base(f.Name)` on the raw name; only `audioEntries` normalises. For a Windows-made zip without tags, `Album\01 - Artist - Title.mp3` yields artist `Album\01`, title `Artist - Title`, and the report's `name` carries the folder. | **backend:** normalise in `importOne` (one helper shared with `audioEntries`). **test:** TC-MUS-065. | — |
| 10 | P0.8 enable mismatch | A worker with lookups disabled records `failed` with a note instead of leaving `pending`. | `lookup.go` `LookupTrack` returns nil when `!s.LookupEnabled()`, after the API already set `lookup_status = 'pending'`: the track is **pending forever** when the API has the variables and the worker does not. | **backend:** record `failed` with note "lookup disabled on the worker" and return nil. **test:** TC-MUS-087. | — |
| 11 | P1.3 queue placement | Long music tasks do not share a 4-slot pool with `notify:dispatch`. | All four tasks register on the light mux and enqueue to `default`; `import_zip` may hold a slot for 6 h, `enrich_track` up to ~2.5 min (90 s cover wait + ffmpeg), `lookup_track` up to 5 min (throttle waits). A 300-track enrichment sweep delays every notification behind it. | **backend:** the `bulk` server of SPEC-14 §11 row 8 (F095), `Concurrency: 1`; move `import_zip` there, and `enrich_track` / `lookup_track` there or to their own small pool. **test:** TC-MUS-066 (task options assert the queue). | SPEC-14 §11 row 8 (F095) |
| 12 | P0.7, P0.8 elevated callers | `music:write:any` holders may enrich or look up another owner's track; the task carries the track owner. | The routes admit owner **or** `music:write:any` (`WriteTrackMW`), but `enrich.go` `EnqueueEnrich` and `lookup.go` `EnqueueLookup` return `ErrNotFound` when `track.OwnerID != caller` — a moderator gets 404 while `shared/openapi.yaml` says "Owner, or `music:write:any`". | **backend:** drop the owner comparison (the middleware already decided) and enqueue with `track.OwnerID`. **test:** TC-MUS-075, TC-MUS-085. | — |
| 13 | P0.9 add order | New playlist tracks append in request order. | `repository/playlists_adapter.go` `AddTracks` iterates `OwnedTrackIDs`' result, whose query (`query/playlists.sql` `OwnedTrackIDs`, `id = ANY(...)`) has no ORDER BY — positions follow heap order, not the user's selection. | **backend:** iterate the request list and skip ids absent from the owned set. **test:** TC-MUS-094. | — |
| 14 | §7 shape failures | Shape failures are 422 `music/validation`; one slug per miss. | `handler.go` `CreateTrack`/`UpdateTrack` answer a malformed asset id with `server.BadRequest` (400 `about:blank`); `playlists_handler.go` `parseIDList` answers empty, > 500 and malformed `track_ids` with 400 `about:blank`; `BulkStatus` answers a bad status with `music/invalid-playlist`; `GetImport` on another owner's job answers `music/not-found` (via `writeMusicErr`) while a malformed import id answers `music/import-not-found`. | **backend:** 422 `music/validation` for all of the above; `ErrImportNotFound` mapped to `music/import-not-found` everywhere. **test:** TC-MUS-122. | README Pagination/Errors |
| 15 | §7 envelopes | Every list answers `{items}` (`GET /tracks`, `/tracks/mine` add `next_cursor`). | `handler.go` `writeTrackList` writes `{tracks}`; `import_handler.go` `ListImports` writes `{imports}`; `playlists_handler.go` `ListPlaylists` writes `{playlists}`. `shared/openapi.yaml` `TrackList` requires `[tracks]` and `listMusicImports` `[imports]`. Readers: `frontend/src/lib/music.ts` `listTracks`, `listMyTracks`, `listPlaylists`, `fetchAllTracks`; `MusicIndexView.tsx` and `MusicWidget.tsx` read `.tracks`. (The specs README names `{imports}` and `{playlists}` but not `{tracks}`.) | **backend:** rename the three keys. **openapi:** `TrackList` → `required: [items]`; import and playlist lists likewise. **frontend:** `TracksPage.items` and every reader. **test:** TC-MUS-120. | Decision 2026-09-30 (Envelopes) |
| 16 | §7 `limit` | Missing / invalid / < 1 → default; above max → clamped. | `service.go` `list`: `if limit <= 0 \|\| limit > maxLimit { limit = defaultLimit }` (`?limit=500` → 30); `import.go` `ListImports` the same with 10/50. `lib/music.ts` `TRACKS_MAX_PAGE` documents the reset. | **backend:** `server.Limit(r, 30, 50)` and `server.Limit(r, 10, 50)` in the handlers. **openapi:** describe `limit` as defaulted and clamped. **test:** TC-MUS-121. | Decision 2026-10-01 (limit) |
| 17 | P0.6, P0.7, P0.8 `origin = import` | Assets music ingests carry `origin='import'`, so the bell is not flooded. | `mediaapi.Ingest` has no `origin` and always records `upload` (SPEC-04 §11 row 10); the zip importer, enrichment and lookup all ingest through it, and `completeAudio` publishes `media:asset_ready` per asset, which notify suppresses only for `origin='import'` — so a 300-track zip can put 300 entries in the bell. | **backend:** pass `import` from `importOne`, `ingestCover` (once SPEC-04 row 10 adds the parameter). **test:** TC-MUS-067. | F038; SPEC-04 §11 row 10; SPEC-05 §11 row 8 |
| 18 | P0.5 empty MIME | A file accepted by extension uploads with the content type its extension maps to. | `BulkImportModal.tsx` `pick` admits a file by extension when `file.type` is empty, but `lib/music.ts` `uploadAudioAsset` throws "Chỉ chấp nhận tệp âm thanh." unless `file.type` starts with `audio/` — such `.wma`/`.opus` files always fail. | **frontend:** derive the type from the extension (mirror `importAudioExt`) when `file.type` is empty, and send it as `content_type`. **test:** TC-MUS-052. | — |
| 19 | P0.5 parity | One rule set; a shared fixture table proves both implementations. | `frontend/src/lib/music.ts` `metaFromFilename` vs `import.go` `titleFromFilename`: the JS regex allows any whitespace (`\s*`) before the separator, Go only spaces (`TrimLeft(…, " ")`); the JS extension strip (`/\.[^.]+$/`) leaves a trailing `.` that Go's `filepath.Ext` removes; a blank first part becomes artist `""` in JS (sent as `""` by `BulkImportModal`) but nil in Go. Only Go has tests (`import_test.go`). | **backend:** accept tabs like the JS port (or vice versa — pick one in the fixture). **frontend:** match the trailing-dot and blank-artist rules. **test:** TC-MUS-050 (one fixture table, run by `import_test.go` and a new `frontend/src/lib/music.test.ts`). | — |
| 20 | §6 permissions | Every seeded music code is required by some route, or removed. | `music:publish:own` (`0022`) and `music:publish` (`0003`) are seeded and granted (`creator`, `editor`) but no route requires them: the owner publishes through `PublishMW` (owner or `music:publish:any`) without any code. | **backend:** require `music:publish:own` for the owner leg (`RequirePermission` chained before `PublishMW`, as SPEC-16 P0.4 / SPEC-17 P0.5 specify) — no longer a lock-out for `user`, since row 26 grants it (Decision 2026-10-01b (D4)); `music:publish` (`0003`, two segments, `editor`) satisfies `music:publish:any` and may stay. **test:** TC-MUS-002. | — |
| 21 | P2; §6 hygiene | No dead queries or indexes; bounded free-text columns; an index for the owner list. | `query/music.sql` `ListTracksNeedingLookup` has no caller and `music_tracks_lookup_pending_idx` serves only it; `ListOwnTracks` orders by `(updated_at, id)` with only `music_tracks_owner_idx (owner_user_id)`; `artist`, `album`, `description`, `genre` and `music_playlists.description` have no length bound (titles do). | **migration:** `music_tracks_owner_updated_idx (owner_user_id, updated_at DESC, id DESC)`; length CHECKs (`NOT VALID` then `VALIDATE`); drop the unused index or build the P2 sweep. **backend:** delete the query or use it. **test:** TC-MUS-003. | — |
| 22 | §7 OpenAPI | Every route declared with its real status codes, `security`, and `x-required-permission`. | `shared/openapi.yaml` has no `/playlists*` and no `/tracks/bulk-status`; the seven `/tracks/imports*` and `/tracks/{id}/enrich|lookup` operations declare no `security`; `uploadMusicImportZip` documents 400 for "not a zip, or over a limit" (the handler answers 422, and "not a zip" is detected by the worker, not the upload); `enrichTrack` documents 400 for "no audio" (422); no list documents 400 `music/invalid-cursor`; `listMyTracks` omits 403; no operation carries `x-required-permission`; `/tracks/{id}` reuses the `AssetID` parameter. | **openapi:** add the eight missing operations and their schemas; fix the codes; add `security` and the annotations (with the cross-cutting retrofit). Commit regenerated `api.gen.go` + `types.gen.ts`. **test:** TC-MUS-124. | ADR-10; README AuthZ |
| 23 | §7 Problem types | Every emitted music slug is in `problems.ts`. | `frontend/src/lib/problems.ts` registers `music/lookup-disabled`, `music/playlist-not-found`, `music/playlist-exists`, `music/invalid-playlist`; missing: `music/not-found`, `music/validation`, `music/invalid-cursor`, `music/invalid-audio-asset`, `music/invalid-cover-asset`, `music/not-publishable`, `music/import-not-found` (and `music/import-already-uploaded` with row 2). | **frontend:** add them to `ProblemType` and `PROBLEM_MESSAGES`. **test:** TC-MUS-123. | README Errors |
| 24 | P1.1, P1.2 | Lookup results and per-track passes are visible; an import survives a reload. | No `.tsx` reads `lookup_status`, `lookup_note`, `release_year` or `genre`; `lib/music.ts` `enrichTrack` and `lookupTrack` have no caller; nothing calls `GET /tracks/imports` (`lib/music.ts` has no `listImports`). | **frontend:** as P1.1/P1.2 specify. **test:** TC-MUS-111…113. | — |
| 25 | P0.2 / §7 owner-guarded routes (CC-3) | A track the caller may not see is 404 `music/not-found` on every route, byte-identical to a missing one; never 403. | `PATCH`, `DELETE`, `publish`, `unpublish`, `enrich` and `lookup` on `/tracks/{id}` sit behind `RequireOwnerOrPermission` (`backend/internal/modules/account/middleware/rbac.go`) with `cmd/api`'s `byTrack` extractor, which resolves **any** track in the tenant: another member's draft answers **403** `about:blank` (confirming it exists) and a missing or malformed id **404 `about:blank`**, not `music/not-found`. Latent while each user has a personal organisation (RLS hides other tenants' rows, so they read as missing). `http_test.go` mounts the module without the guards, so its 404 assertions do not see this. | **backend:** the `byTrack` extractor resolves only rows the caller may read (published or own) and maps the miss to `music/not-found` (a stranger's draft is then a miss; a published track a non-owner lacks the elevated code for stays 403, since its existence is public). **test:** TC-MUS-017 over the real guard. | SPEC-14 F119 pattern; CC-3 |
| 26 | P1.4 `user` authoring grant | `user` holds `music:write:own` and `music:publish:own` (a music-owned grant migration); `:any` and delete-any unchanged. | `backend/db/migrations/0022_music_core.up.sql` grants both codes to `creator` only and no later migration widens them, so a `user` gets 403 from every `music:write:own` route (`module.go` `m.perm("music:write:own")` on `/tracks`, `/tracks/mine`, `/tracks/bulk-status`, `/tracks/imports*`, `/playlists*` writes); `MusicIndexView.tsx`'s **Mine** tab already handles that 403. | **migration:** `000N_music_user_write_grant` (`ls backend/db/migrations \| tail -2` for the number). **test:** TC-MUS-004. Lands with or after F009 (SPEC-04 §11 row 9); row 20 then requires `music:publish:own`. | Decision 2026-10-01b (D4) |

**Already matching on HEAD.**
- Tenancy: all four tables carry `tenant_id`, the index and the FORCE RLS
  `tenant_isolation` policy (`0022`, `0038`, `0041`); import, enrich and lookup
  open the payload owner's scope before any query, keep one scope per zip entry,
  and copy bytes outside any transaction (`enrich.go` `spoolAudio`).
- Track rules: title trimmed and 1–200 runes; audio/cover validated as ready,
  right kind, owned; drafts 404 to strangers; repeat DELETE 404 (`:execrows`);
  publish requires a ready owned audio asset (422 `music/not-publishable`);
  `music:track_published` subscribed in both binaries with its one notify
  consumer; the asset-deleted query unpublishes on audio loss.
- Zip import: three-step flow, 4 GiB / 2000 entries / 512 MiB / 50× guards,
  `MaxRetry(0)` + 6 h timeout, upload-visibility wait, `__MACOSX` / `._` /
  backslash filtering and path-sorted order, ffprobe tags with filename fallback,
  `done` vs `failed` semantics, zip deleted afterwards, report shape.
- Enrichment: `attached_pic` disposition check, `-c copy` extraction, 16 MiB
  cover cap, 90 s ready wait, `enrichPatch` fills artist/album/cover only when
  blank and never the title.
- Lookup: off unless both variables are set; 503 `music/lookup-disabled` naming
  them; Redis `SET NX PX 1100` global slot with local fallback; contact
  User-Agent; score floor 88 with early stop; earliest dated release; year
  bounds; most-voted tag; CAA 404 = no art; 500-per-sweep cap reported as
  `skipped`; SQL COALESCE gap-fill in `SetTrackLookupResult`.
- Playlists: case-insensitive unique names via `ON CONFLICT DO NOTHING` (409, not
  500); owner predicates in every query; idempotent add with `{added,
  requested}`; foreign ids dropped; cascades.
- Playback: `/assets/{id}/original` is `http.ServeContent` over the lazy
  `objectReader` (`media/objectreader_test.go`); one app-wide `<audio>` in
  `MasterBase`; shuffle/repeat; bounded play-all with a truncation flag.
- Frontend: the four routes resolve through the template registry; `/library/:path*`
  is in the matcher; `metaFromFilename` follows the Go rules for every case the
  Go tests cover; the zip modal polls every 1.5 s and offers enrich / lookup.

**Test evidence to add or fix.**
- No test covers playlists, `BulkSetStatus`, `RunImport`, `SaveImportZip`, the
  import / enrich / lookup HTTP routes, `EnqueueEnrich*` or `EnqueueLookup*`
  (TC-MUS-034…036, TC-MUS-060…066, TC-MUS-075, TC-MUS-085, TC-MUS-090…095).
- The throttle (`MBClient.throttle` with Redis — two clients, assert ≥ 1.1 s
  spacing) and the User-Agent header (assert it in the `httptest` handler) are
  untested; `musicbrainz_test.go` builds the client with no Redis (TC-MUS-082,
  TC-MUS-083).
- `SetTrackLookupResult`'s COALESCE is the gap-fill guarantee and lives in SQL;
  only an integration test against Postgres proves it (TC-MUS-084).
- `music_test.go` names several tests after the movie vertical they were copied
  from (`TestPublishRequiresAVideoAsset`, `TestCreateRejectsAForeignVideoAsset`,
  `TestCreateRejectsAnImageAsTheVideo`, `TestCreateRejectsAVideoAsThePoster`)
  though they exercise audio and covers; rename when touched.
- The frontend has no music test; `metaFromFilename` is a pure rule and belongs
  in vitest (TC-MUS-050). Count the module's tests with
  `find backend/internal/modules/music -name '*_test.go' | wc -l`.

## 13. Out of scope

- The media module's own `/original` state and size rules, `duration_ms` for
  audio, and audio in `/continue` — SPEC-04 §11 rows 4–6 and 16, SPEC-10 §11
  rows 4, 5, 8.
- The bell's rendering of `work.published` notifications — SPEC-05.
- Movie and story verticals (backlog P2 line 28), which mirror this module's
  track shape but have no spec.
- Social sharing of music, reels audio (`D-37`), and any licensed catalogue.
