# SPEC-01 — Media Image Pipeline + Asset Management

**Status:** current, **rev 4** (spec-gap fixes, code-verified) · **Last verified:** 2026-10-01
**Module:** `media` (built + wired) · **Depends on:** nothing
**Upstream:** [briefs/01-media-image-pipeline.md](../briefs/01-media-image-pipeline.md) · **Refs:** 2026-07 backlog §2 (archived; `git show 8d382d2^:docs/product/backlog.md`), feature-inventory.md §3
**Downstream consumers:** SPEC-02, SPEC-03 P1 (receipts), SPEC-04 P0.4 (`media:asset_ready` → in-app notification), SPEC-05/SPEC-12 (entry photos), SPEC-06 (`media:asset_deleted`; playback via SPEC-07), SPEC-07, SPEC-08 P1.7 (avatars), SPEC-09 (P0.3 scheduler; P1.7 mediaapi ExportProvider)
**Rev 2 origin:** external technical review 2026-07-09 — six findings; five accepted, one rejected. See §12.

---

## 1. Problem statement

The media `assets` schema already permits `kind = image`, but the worker pipeline only
handles video: uploading an image today either fails or strands the asset in a
non-`ready` state. `worker.HandleThumbnail` is a stub, so video assets have no
posters. There is no way to delete an asset (rows and storage grow forever on a
≤$100/mo VPS budget), no way to retrieve an uploaded original (untenable for a
life-OS personal archive), and the only asset listing is the small one embedded in
`/upload`. Image support is the shared bottleneck for the entire entertainment axis
and several scattered backlog items (avatar upload, photos, receipts).

## 2. Goals

1. An image uploaded through the existing flow reaches `ready` with web-optimized
   variants in < 10 s for a 12 MP JPEG on the dev VPS class.
2. Every video asset transcoded after this ships shows a real poster frame.
3. An owner can delete any of their assets; DB rows and **all** storage objects are
   gone afterwards — and can retrieve the byte-identical original of anything they
   uploaded (archival guarantee).
4. A `/library/media` page lists every asset the user owns with filter + pagination.
5. Zero regressions to the existing video path, and **zero memory-related worker
   deaths**: media processing must not be able to OOM the box (rev 2).

## 3. Non-goals

- Multi-rendition HLS ladder, playback ACL, presigned direct-to-bucket upload —
  tracked separately in the 2026-07 backlog §2 (archived;
  `git show 8d382d2^:docs/product/backlog.md`); this spec neither builds nor blocks them.
- Audio asset kind (P2 placeholder only).
- Image *editing* (crop, rotate-on-demand, filters). Auto-orientation of **served
  variants** is in scope (§5 P0.1); user-driven editing is not.
- Animated images. Animated GIF/WebP inputs are **rejected** at v1 — detected
  by the worker's ffprobe step (frame count > 1), which marks the asset
  `failed` with an explicit animated-input message; unlike HEIC's magic
  bytes, animation is not cheaply detectable at `/complete` *(rev 3 — one
  rejection surface, the worker)*. Treating them as video is a future decision.
- **HEIC/HEIF (iPhone default format) — rejected at v1** *(rev 2, review pt 2)*.
  Rationale: ffmpeg HEIC decode depends on build flags (libheif/HEVC) — verifying
  and maintaining that in the worker image is out of the v1 envelope. The rejection
  must be **explicit and helpful**: HEIC magic bytes are detected and get a
  dedicated error detail telling the user to convert/export as JPEG on-device.
  **Re-entry condition** (recorded in [briefs/04](../briefs/04-deferred.md)): the
  moment dogfooding involves an iPhone user, HEIC ingest becomes P0 (likely a
  `libheif`-based pre-step, not ffmpeg).
- De-duplication of identical uploads (content hashing) — future consideration.

## 4. User stories

- As a comic creator, I upload page images and get web-optimized variants so readers
  load fast on mobile. *(primary — feeds SPEC-02)*
- As an asset owner, I delete an asset and it disappears from listings, playback,
  and object storage, so disk/R2 usage doesn't grow forever.
- As the owner of a personal archive, I download the **byte-identical original** of
  any photo I uploaded — with its capture date and location metadata intact —
  because a life OS that silently degrades my originals is destroying my data.
  *(rev 2, review pt 3)*
- As a user, I browse everything I've uploaded in one place, filter by kind/status,
  and page through it.
- As a user, when my phone photo is sideways (EXIF orientation), the displayed
  image is upright everywhere.
- As an iPhone user uploading a HEIC photo, I get a clear message telling me to
  convert to JPEG — not a generic "unsupported format". *(rev 2)*
- As a user, when I upload a corrupt file, I see the asset marked failed with a
  reason — the system never hangs in `processing`.

## 5. Requirements

### P0.1 — Image ingest

**Behavior** *(rev 3 — aligned to the shipped upload flow)*. The existing
two-step flow (POST `/assets` → presigned browser→bucket PUT → POST
`/assets/{id}/complete`) accepts `image/jpeg`, `image/png`, `image/webp`; the
asset row is created with `kind='image'`, `status='uploading'` at session
start — the shipped lifecycle is `uploading → processing → ready|failed`;
there is no `uploaded` state. Because bytes go straight to the bucket, the API
never sees the file body: **content sniffing happens at `/complete`**, via a
ranged GET of the object's first bytes (magic bytes decide — never file
extension or client-declared Content-Type) plus a HEAD for the true size.
HEIC/HEIF magic bytes are recognized specifically and rejected with a
convert-on-device hint *(rev 2)*. **On any unsupported-format detection at
`/complete`** (HEIC/HEIF, or a body whose magic bytes match no accepted type):
return 422 Problem `media/unsupported-format`, **delete the uploaded object**,
and set `status='failed'` with an `error_message` naming the detected format
(HEIC gets the convert-to-JPEG hint) — mirroring the file-too-large handling
*(F010)*. The 50 MB cap is enforced belt-and-braces:
`content-length-range` condition on the presigned PUT policy where supported,
re-checked via HEAD at `/complete` (on violation: object deleted, asset
`failed`, Problem `media/file-too-large`). On acceptance `/complete` sets
`status='processing'` and enqueues `media:process_image`
`{asset_id, source_key, owner_user_id}` (the events.md registry payload).

**Sniffed type is stored** *(rev 4)*. For `kind='image'`, `/complete`
overwrites `assets.mime_type` with the sniffed type (`image/jpeg|png|webp`).
The kind itself comes from the content type declared at session start, so a
body whose magic bytes match no accepted image type — including a video or
other non-image file sent with an image content type — follows the
unsupported-format rule above (422, object deleted, `failed`). Video and audio
kinds are not sniffed at v1; their `mime_type` stays client-declared. *(Code
follow-up: the shipped `completeImage` discards the sniffed type, so
`mime_type` is still the declared one.)*

**Worker task `media:process_image`.**
1. Probe with **ffprobe (header read — before any full decode)**: reject if
   **width × height > 64,000,000 px** (8,000² ≈ 256 MB decoded RGBA — the real
   memory guard), if either side > **30,000 px** (sanity ceiling for malformed
   input), if animated (frame count > 1), or if decode fails →
   `status='failed'`, `error_message` set. The cap is on **area**, not side
   length, so the tall webtoon strips SPEC-02 ingests (e.g. 704×18,000 ≈ 13 MP)
   are admitted while an 8,001×8,000 image is refused *(rev 4; rev 2 had a
   per-side 8,000 px cap, was 12,000 — review pt 1)*. Memory rationale: decoded
   RGBA at 8,000² ≈ 256 MB peak vs ≈ 576 MB at 12,000² — the difference between
   "tight" and "OOM-killer bait" on a small VPS.
2. **The uploaded original is never modified** — no re-encode, no orientation
   rewrite, no metadata strip. It stays byte-for-byte as uploaded at its source
   storage key *(rev 2, review pt 3 — archival guarantee)*.
3. Generate **served variants** (WebP, quality ~80), each fitting within
   **max-width × 16,000 px** (libwebp's hard limit is 16,383 px per side),
   aspect-preserving and never upscaled
   (`scale=w='min(W,iw)':h='min(16000,ih)':force_original_aspect_ratio=decrease`),
   each with EXIF **auto-orientation baked in** and **all metadata stripped**
   (`-map_metadata -1`):
   | variant | max width | purpose |
   |---|---|---|
   | `thumb` | 320 | library grid, pickers |
   | `medium` | 1280 | comic reader, detail views, lightbox |
4. Insert `media_asset_variants` rows; set `status='ready'` — in one
   transaction, under the status guard defined in P0.3 ("Delete while a worker
   is in flight").

**Resource guardrails** *(rev 2, review pt 1; mechanism corrected in rev 3;
queues split in rev 4)*: `media:transcode` runs on the **`heavy` queue** with
its own `asynq.Server` (`Concurrency: 1`). `media:process_image` runs on the
**`image` queue** with its own `asynq.Server` (`Concurrency = IMAGE_CONCURRENCY`,
default 3; set 1 on a < 4 GB VPS), so a batch comic import processes pages in
parallel while video transcode stays serial. Each server lives in the same
worker process. Asynq queue *weights* only bias dequeue order within one shared
worker pool, so no queue name or weight can cap a queue's parallelism; the cap
requires its own server. The step-1 area cap bounds each decode, so peak memory
≈ `IMAGE_CONCURRENCY` × 256 MB + one transcode. Light tasks (thumbnail, notify,
janitor) stay on the weighted default server so heavy jobs never starve them.

**Serving variants.** `GET /api/v1/assets/{id}/variants/{variant}`
(`variant ∈ thumb|medium|poster`) streams the stored variant object with its
WebP content type and `Cache-Control: private, max-age=600` — no `immutable`
and no year-long max-age, so a delete or un-share takes effect within minutes
(an asset made public via its visibility flag may use `public, max-age=86400`)
*(rev 4)*. Same **public-ish** auth stance
as `/hls/*` — **unauthenticated**: variants carry no EXIF/GPS (all metadata is
stripped, step 3 above), so they are safe to be semi-public, unlike the private
original (P0.5). Returns 404 `media/asset-not-found` for a missing or
deleting/deleted asset. Lands in `shared/openapi.yaml` (§7).

**Acceptance criteria.**
- Given a 12 MP JPEG upload, when the worker completes, then the asset is `ready`
  with `thumb` + `medium` variants in storage/DB **and the source original's
  checksum is unchanged**.
- Given a JPEG with EXIF Orientation=6 and GPS tags, when processed, then both
  variants display upright and `exiftool` shows no metadata on them — **while the
  original retains its full EXIF** (verified with `exiftool` on a downloaded copy).
- Given a PNG with transparency, when processed, then transparency is preserved in
  WebP variants.
- Given a corrupt file / animated GIF / **8,001×8,000** image (area over
  budget), when processed, then `status='failed'` with a human-readable
  `error_message`; the worker process does not crash and the queue continues.
- Given a 704×18,000 webtoon strip, when processed, then it reaches `ready` and
  `medium` is ≤ 16,000 px tall *(rev 4)*.
- Given a HEIC upload, when sniffed at `/complete`, then 422 Problem
  `media/unsupported-format` whose detail explicitly names HEIC and suggests
  converting to JPEG, **the uploaded object is deleted, and the asset is
  `failed`** — never enqueued *(rev 2; sniff location corrected rev 3;
  asset/object fate specified F010)*.
- Given an 800 px-wide image, then `medium` is 800 px (no upscaling), `thumb` 320 px.
- Given 5 image tasks + 2 transcodes enqueued at once, then at most
  `IMAGE_CONCURRENCY` image tasks and 1 transcode run simultaneously — assert via
  Asynq inspector or worker logs *(rev 2; queues split rev 4)*.
- Given a > 50 MB object at `/complete` (presign policy bypassed or unsupported),
  then Problem `media/file-too-large`, the object is deleted from storage, and the
  asset is `failed` — never enqueued *(rev 3 — the API cannot reject "before any
  storage write" under presigned direct upload)*.

### P0.2 — Video poster thumbnail (kill the stub)

**Behavior.** `worker.HandleThumbnail` implemented. On transcode success the
transcode worker sets `ready` and enqueues `media:thumbnail
{asset_id, source_key, owner_user_id}` on the `thumbnail` queue; the poster
never gates `ready` *(rev 4)*. The thumbnail task probes duration, seeks to
`min(10% of duration, 10s)`, extracts one frame, scales it to 640 w, stores
it as WebP and inserts a `poster` variant row on the video asset.
**If ffprobe reports zero video streams** (e.g. an `.mp3` renamed `.mp4` — an
audio-only container), **skip poster generation entirely and log a warning**; the
asset's playability is unaffected *(rev 2, review pt 5)*.

**Acceptance criteria.**
- Given a newly uploaded video, then within one thumbnail-task cycle after it
  reaches `ready` a `poster` variant exists and renders in the library grid and
  as the Vidstack poster.
- Given a 2 s video, when thumbnailed, then extraction succeeds (seek point clamped
  inside the file).
- Given an audio-only container (0 video streams), when the poster step runs, then
  it is skipped with a warning, no crash, and the asset still reaches `ready` *(rev 2)*.
- Poster failure does **not** fail the asset: video remains `ready`, poster absent,
  warning logged. (Playback > cosmetics.)

### P0.3 — Delete asset

**Endpoint.** `DELETE /api/v1/assets/{id}` — enforced via
`RequireOwnerOrPermission(engine, "assets:delete:any", extractAssetOwner)`
*(rev 4: `RequirePermission` never checks ownership — a plain
`RequirePermission("assets:delete:own")` would let every holder of
`assets:delete:own` delete anyone's asset. The rev-3 reason given here (that it
would 403 admin) was wrong: admin inherits `assets:delete:own` through the 0003
role parent chain. `assets:delete:own` remains the catalog entry documenting
the owner capability; the middleware's owner branch admits owners and
`assets:delete:any` (admin) or `*` admits everyone else)*.

**Behavior.** In order: (1) authorize ownership; (2) set `status='deleting', updated_at=now()` (the janitor
grace keys on this; README updated_at convention)
(excluded from all listings); (3) delete every storage object for the asset —
source original, HLS playlist + segments, all variants (by the asset's storage key
prefixes — shipped: `uploads/{id}/`, the HLS output prefix, `variants/{id}/`);
(4) delete variant rows + asset row; (5) once the asset row is gone (via this path **or** the
janitor's), emit `media:asset_deleted` `{asset_id, owner_user_id}` (registry:
`docs/reference/events.md`) so domain modules that hold the id with no cross-module FK
— e.g. comic — can drop dangling references (SPEC-02 P0.6). Best-effort, like all
cross-module events; a dropped event is recoverable by a consumer-side reconcile.

**Response** *(rev 4)*. Once step (2) commits, the delete is irrevocable. If
step (3) or (4) fails, the API still returns 204 and logs a warning; the
janitor completes the purge after the 15-min grace. A DELETE on an asset
already in `deleting` returns 204 (and retries the purge). Only a missing row
returns 404 `media/asset-not-found`. *(Code follow-up: the shipped handler
answers a step-3/4 failure with a 500 `about:blank`.)*

**Delete while a worker is in flight** *(rev 4)*. Workers finish under a
status guard inside their existing transaction: `process_image` and
`transcode` use `UPDATE assets SET status='ready' … WHERE id=$1 AND
status='processing'`; the thumbnail worker inserts its `poster` row only if
the asset exists and `status <> 'deleting'`. When the guard affects 0 rows, or
a variant insert hits the FK or a missing row, the transaction is rolled back
(no variant rows, no `media:asset_ready`). The worker then deletes every
object it uploaded for the asset and returns nil, so the task is not retried.
*(Code follow-up: the shipped `MarkAssetReady`/`MarkAssetImageReady` queries
carry no status predicate and the workers do not clean up after a lost race.)*

**Janitor `media:purge_orphans`** *(mechanics specified in rev 2, review pt 4)*:
an **Asynq periodic task, hourly**, that selects assets
`WHERE status='deleting' AND updated_at < now() - interval '15 minutes'` (grace
window so it never races an in-flight API delete) and re-runs the purge. It
**also sweeps abandoned upload sessions** —
`WHERE status='uploading' AND updated_at < now() - interval '24 hours'` —
marking them `failed` with `error_message='upload abandoned'` (then reclaimable
by the normal delete path), so partial objects and stranded rows don't
accumulate forever against Goal 3 *(F038)*. Idempotent per asset. After
**5 consecutive failed purge attempts** for the same asset, log at error level
(the "a purge is stuck" signal on a single-operator box). Cost note: these are
indexed scans (`assets_deleting_idx`, `assets_uploading_idx`, §6) over
near-empty sets — trigger cadence is about *definedness*, not CPU.

**Shared periodic scheduler** *(F002 — single registration point)*: `cmd/worker`
stands up **one** `asynq.Scheduler` (`PeriodicTaskManager`) as the single place
all periodic tasks register. `media:purge_orphans` is the first entry; later
specs register their periodic tasks here too — `ops:backup_database` and
`ops:purge_exports` (SPEC-09), `people:scan_birthdays` (SPEC-08 P0.4),
`notify:purge_old` (SPEC-04), account's
`PurgeExpiredRefreshTokens` — rather than each standing up its own scheduler or
reaching for OS cron. This is the runner the other specs "borrow the convention
of (Asynq periodic, never OS cron), not the code."

**Acceptance criteria.**
- Given a ready video asset, when deleted, then its HLS URL, variant URLs, **and
  original-download URL** return 404 from the API origin (cache bypassed) and
  `mc ls` shows no objects under its prefixes; a browser cache may still serve a
  variant for ≤ 10 min (P0.1 cache headers) *(rev 4)*.
- Given an already-deleted id, when DELETE is called again, then 404 (idempotent,
  never 500).
- Given user B calling DELETE on user A's asset without `assets:delete:any` (and
  not `*`), then 403 and nothing is deleted. 403 (not 404) is deliberate:
  `RequireOwnerOrPermission` answers 404 only for a missing row and 403 for
  another owner's asset. A row hidden by RLS surfaces as 404
  `media/asset-not-found` *(rev 4)*.
- Given a storage outage mid-delete, then the DELETE still returns 204; when the
  janitor's next hourly run happens after recovery, the asset finishes deleting;
  it never reappears in listings meanwhile; a stuck purge logs at error level
  after 5 attempts *(rev 2; 204 rev 4)*.
- Given a successful delete (API or janitor path), then exactly one
  `media:asset_deleted {asset_id, owner_user_id}` is published after the row is
  gone *(rev 4)*.
- Given an asset deleted while `processing`, or while its poster task is in
  flight, when the worker finishes, then no objects remain under its prefixes,
  no variant rows exist, and no `media:asset_ready` is emitted *(rev 4)*.
- Given an `uploading` asset older than 24 h, when the janitor runs, then it is
  marked `failed` with `error_message='upload abandoned'` *(rev 4)*.
- Given `cmd/worker` starts, then `media:purge_orphans` is registered hourly on
  the single shared `asynq.Scheduler` *(rev 4)*.

### P0.4 — Library page

**Route.** `/library/media` (RSC catalogue shell + client islands per D-33 — "RSC shell" as defined
in the specs README Frontend convention). The
page view is declared in `TemplateManifest.views`
(`frontend/src/templates/types.ts`), implemented under `templates/v1/views/...`,
and `app/(app)/library/media/page.tsx` resolves it via
`activeTemplate().views.<x>` — never a version-specific import in `app/`, so the
`v2` template switch keeps working *(F006)*.

**Behavior.** Grid of the user's assets: poster/thumb, title, kind badge, status
badge, created date. Filters: kind (all/video/image), status (all/ready/
processing/failed — the `processing` filter **includes still-`uploading`
sessions** so a freshly created asset is never invisible under every filter
*(rev 3)*). Pagination: **cursor** (`created_at DESC, id DESC` — the convention
all newer specs use; extend the existing list endpoint to it) *(rev 3 — resolves
the cursor-vs-page waffle)*. Row actions: open (video → player, image → lightbox
showing `medium`), **Download original** *(rev 2 — in the lightbox and the card's
expanded view)*, delete (confirm dialog → optimistic removal). The list
contract (`?kind=&status=&cursor=&limit=`, response shape, the server-side
`processing` → `processing,uploading` expansion) is in §7. `failed` assets show `error_message` on hover/expand and offer only delete.

**Acceptance criteria.**
- Given 100 assets, when the page loads, then the first page (≤ 50 thumbs, the
  §7 default `limit`) renders with LCP < 2.5 s (thumb variants only — never
  originals in the grid).
- Given a delete confirmation, when it succeeds, then the card disappears without a
  full refetch (TanStack cache mutation).
- Given an image lightbox, then a Download-original action is present and works *(rev 2)*.
- Empty state links to `/upload`.

### P0.5 — Download original *(new in rev 2, review pt 3)*

**Endpoint.** `GET /api/v1/assets/{id}/original` — authenticated. The handler
loads the row owner-scoped (`owner_id = caller`). There is **no
`assets:read:any` or `*` bypass**, because originals carry GPS.
`assets:read:own` (seeded to `user` by 0003) documents the capability only;
`RequirePermission` alone checks no ownership and must never be the sole gate
*(rev 4)*.

**Behavior.** Streams the source object (Range-capable, via
`http.ServeContent`) with `Content-Disposition: inline;
filename="<original filename>"` — `inline`, because the same route is the
playback source for audio (the SPEC-07 player page plays audio from it; there
is no HLS or poster for audio) — and `assets.mime_type` as the content type
(sniffed for images per P0.1, client-declared otherwise). When
`original_filename` is null (assets predating the §6
migration), fall back to `{asset_id}.{ext}` where `ext` is derived from the
sniffed content type (or the `source_key` extension) *(F039)*. **Owner-authenticated
proxy only — the original must never be reachable through the public-ish
variant/HLS URL scheme**, because (unlike the stripped variants) it retains full
EXIF including GPS. This asymmetry is the point: variants are safe to be
semi-public, originals are private archive.

**Acceptance criteria.**
- Given the owner requests the original, then the downloaded file's checksum equals
  the uploaded file's checksum (byte-identical, EXIF intact).
- Given user B (any role, including `*` and editor's `assets:read:any`)
  requests user A's original, then 403 (or 404 `media/asset-not-found` when RLS
  hides the row), and no bytes are streamed *(rev 4)*.
- Given an asset in `processing`/`failed` whose source object still exists
  (worker-side failure: corrupt/animated/oversized-dimensions), the original is
  still downloadable. Given an asset rejected at `/complete` with its object
  purged (file-too-large, unsupported-format), then 404 Problem
  `media/asset-not-found` — the archival guarantee applies only to accepted
  uploads.
- Given an asset `failed` with `error_message='upload abandoned'` (it never
  passed `/complete`, so it was never sniffed or size-checked), then 404
  `media/asset-not-found`; its object is deleted when the owner deletes the
  asset *(rev 4; code follow-up: the shipped `OriginalContent` blocks only
  `uploading`/`deleting`, so an abandoned asset's possibly-partial object is
  still streamed)*.
- Given an asset still `uploading` (browser PUT unfinished or abandoned), then
  409 Problem `media/asset-not-ready` — the source object may be partial
  *(rev 3 — this is the declared use of that Problem type)*.

### P0.6 — Event fan-out (`platform/events`) *(prerequisite; first producer owns it)*

**Deliverable** *(F007)*: build `platform/events` — a
`Publish(ctx, name, payload)` helper plus an **event-name → consumer-task
subscription table** registered in each emitting binary (`cmd/api` and/or
`cmd/worker` — the table is per-binary, see the specs README Events convention), so a published event
(`<module>:<event>`) fans out to every subscribed Asynq task (events.md
"Delivery mechanics"). SPEC-01 is the first producer (`media:asset_deleted` at
P0.3; `media:asset_ready` at P1.2) and therefore owns building the fan-out;
**every later multi-consumer stream/notify feature gates on this** (SPEC-04
bell, SPEC-06 stream, …). Asynq's `ServeMux` panics on duplicate handler
registration, so two consumers can never both handle the raw event task type;
the registry must exist before any event gains a second consumer. Publishing
with zero subscriptions is a silent no-op by design (events.md), and no
consumer is required for SPEC-01 itself to land *(rev 4)*.

**Acceptance criteria** *(rev 4)*.
- Given two subscriptions for an event, when it is published, then `Publish`
  enqueues exactly one task per subscriber.
- Given no subscription for an event, then `Publish` returns nil and enqueues
  nothing.
- Given an enqueue error, then `Publish` propagates it.

### P1 — nice to have

- **P1.1 Metadata edit**: `PATCH /api/v1/assets/{id}` `{title}` — enforced via
  `RequireOwnerOrPermission(engine, "assets:write:any", extractAssetOwner)`
  *(rev 4: `RequirePermission("assets:write:own")` checks no ownership and
  would let any holder rename anyone's asset)*. `assets:write:any` is already
  seeded to `editor` by 0003 (admin inherits it), so no new seed row is
  needed. `{title}` accepts a trimmed string of 1–200 chars; an empty string
  gets 422 `media/validation`; `{title: null}` resets it to
  `original_filename`. Inline rename in the library; operates on the `title`
  column §6 adds. *(Shipped today: the same `PATCH` accepts only
  `{visibility}` (0032), owner-only through the statement's own owner
  predicate; `{title}` is unbuilt. The visibility toggle stays owner-only —
  the `assets:write:any` bypass applies to `title` only.)*
- **P1.2 Event emit**: publish `media:asset_ready` `{asset_id, kind, owner_user_id,
  title, origin}` — via the `platform/events` publisher (events.md "Delivery
  mechanics"; built as the P0.6 prerequisite) — whenever any asset reaches
  `ready`. `title` is set to `original_filename` when the row is created (§6);
  the payload uses `COALESCE(title, original_filename, id::text)`. `origin` is
  **read from `assets.origin`** (§6), set at row creation — `'upload'` by the
  upload-session endpoint, `'import'` by P1.3 `mediaapi.Ingest` for bulk
  producers — so the worker task payload
  (`{asset_id, source_key, owner_user_id}`) need not carry it and
  consumers can suppress bulk floods (the bell must not get one item per page of
  an import of thousands of assets — the whole-comic zip import cap is
  `importMaxEntries` in `comic/import.go`, 100,000 as of 2026-09; a 9,129-image
  archive is verified) *(rev 3; persistence F009)*. mediaapi asset listings also
  expose `origin`. Its only consumer is notify (SPEC-04 P0.4); it is not
  projected into the stream since `0033` ([events.md](../../reference/events.md)).
- **P1.3 mediaapi ingest for server-side producers** *(rev 4)*:
  `mediaapi.Ingest(ctx, ownerID, filename, contentType string, data []byte,
  origin string) (uuid.UUID, error)` runs the same create → store →
  `/complete` path as a browser upload, so the magic-byte sniff, HEIC
  rejection and 50 MB cap apply unchanged, with the same errors as the HTTP
  Problem types. The row is created with the given `origin` (`import` for
  SPEC-02 P1.7 and other bulk producers; the HTTP upload-session endpoint
  always passes `upload`). Media owns it; SPEC-02 P1.7 is the first caller.
  *(Code follow-up: the shipped `Ingest` has no `origin` parameter and always
  records `'upload'`, so the P1.2 import-flood guard never fires — see the
  comment in `cmd/worker/main.go`.)*

### P2 — future considerations (design for, don't build)

- Audio kind: schema allows it; keep the task-per-kind dispatch shape so
  `media:process_audio` slots in. (The variant enum will need e.g. `waveform` —
  see the deliberate migration-cost note in §6.)
- Bulk upload (multi-file / zip) from the browser: SPEC-02 P1.7 is the concrete
  consumer; server-side producers already use P1.3 `mediaapi.Ingest`, and the
  HTTP API shape should not preclude a browser-facing batch upload.
- Content-hash dedup: nullable `content_sha256` column decision stays open.
- HEIC ingest — deferred with an explicit re-entry condition (§3).

## 6. Data model

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `assets` and `media_asset_variants` — `tenant_id` + `<t>_tenant_idx` + FORCE RLS, added by `0020_platform_rls_enable` (`0032_media_asset_acl` later widened their policies for the asset ACL). The DDL below predates ADR-07 and omits the columns.

New table — migration `000N_media_variants` (take the next free number):

```sql
CREATE TABLE media_asset_variants (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id    uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
  variant     text NOT NULL CHECK (variant IN ('thumb','medium','poster')),
  storage_key text NOT NULL,
  width       int  NOT NULL,
  height      int  NOT NULL,
  size_bytes  bigint NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (asset_id, variant)
);
```

**Rev 2 changes.** (a) `'original'` left the enum: the variants table now holds
**derived artifacts only**; the source original is the asset's own storage object
(tracked on `assets`), never re-encoded. (b) **Review pt 6 — rejected**: the
suggestion to drop the CHECK constraint in favor of app-level enums trades DB-level
integrity for avoiding a five-minute migration on a tiny table — and migrations are
this project's only schema mechanism by hard rule anyway. The flexible choice was
already made by using `text + CHECK` rather than a native Postgres `ENUM` type.
**Extending the variant set is a deliberate, small migration cost — accepted.**

Same-module FK is allowed (boundary rule forbids only cross-module FKs).

**Rev 3 — the media table is named `assets`, not `media_assets`** (verified
against `0007_media_assets.up.sql` — the *file* carries the module prefix, the
*table* does not). The same migration also amends it:

```sql
ALTER TABLE assets DROP CONSTRAINT assets_status_chk;
ALTER TABLE assets ADD CONSTRAINT assets_status_chk
  CHECK (status IN ('uploading','processing','ready','failed','deleting'));
ALTER TABLE assets
  ADD COLUMN title text,
  ADD COLUMN original_filename text,
  ADD COLUMN origin text NOT NULL DEFAULT 'upload'
    CHECK (origin IN ('upload','import'));

-- Back the P0.3 janitor's indexed scans and the P0.4 cursor keyset (F040):
CREATE INDEX assets_deleting_idx ON assets (updated_at) WHERE status = 'deleting';
CREATE INDEX assets_uploading_idx ON assets (updated_at) WHERE status = 'uploading';
CREATE INDEX assets_owner_cursor_idx ON assets (owner_id, created_at DESC, id DESC);
```

`'deleting'` is required by P0.3. `title` / `original_filename` are required by
P0.4 (grid title), P0.5 (`Content-Disposition` filename), P1.1 (rename) and
P1.2 (event `title`) — the shipped table stores neither (the upload filename
today only picks a storage-key extension). The upload-session endpoint starts
recording `original_filename`. `title` is set to `original_filename` when the
row is created (upload session or P1.3 `mediaapi.Ingest`). Rows predating this
migration may have NULL `title`/`original_filename`, so every read (grid,
`media:asset_ready`, `Content-Disposition`) uses
`COALESCE(title, original_filename, id::text)` *(rev 4)*. `origin` is required by
P1.2 (the `media:asset_ready` payload's flood guard): set at row creation —
`'upload'` by the upload-session endpoint, `'import'` by P1.3
`mediaapi.Ingest` for bulk producers — and exposed on mediaapi asset listings
(originally for SPEC-06's P1.6 backfill, since retired). The indexes back the P0.3 janitor scans
(`assets_deleting_idx`, `assets_uploading_idx`) and the P0.4 keyset
(`assets_owner_cursor_idx`, replacing reliance on `assets_owner_idx` for the
library cursor). *(Rev 4: `assets_uploading_idx` is not in the applied
`0008_media_image_pipeline`; it ships in a new `000N_media_uploading_idx`
migration — code follow-up.)* Queries in
`query/media_variants.sql`; regenerate via `make sqlc` — never hand-edit
`*.sql.go`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/assets` | `assets:write:own` | modified: records `original_filename`, sets `title = original_filename` and `origin='upload'`; response unchanged |
| POST | `/api/v1/assets/{id}/complete` | `assets:write:own` | modified: magic-byte sniff (stores the sniffed `mime_type` for images) + HEAD size re-check; new Problems `media/unsupported-format` (422), `media/file-too-large` |
| DELETE | `/api/v1/assets/{id}` | owner, or `assets:delete:any` (RequireOwnerOrPermission) | 204 once `deleting` commits (purge failures fall to the janitor; already-`deleting` → 204); 404 `media/asset-not-found` only for a missing row |
| GET | `/api/v1/assets/{id}/original` | authenticated; owner-scoped load, no `:any`/`*` bypass (`assets:read:own` documents the capability) | rev 2; inline stream, Range-capable; never public |
| GET | `/api/v1/assets/{id}/variants/{variant}` | public-ish / unauthenticated (same as `/hls/*`) | `variant ∈ thumb\|medium\|poster`; streamed w/ content type + `Cache-Control: private, max-age=600` (`public, max-age=86400` for a public asset); 404 `media/asset-not-found` |
| PATCH | `/api/v1/assets/{id}` | owner, or `assets:write:any` (RequireOwnerOrPermission) | P1.1 `{title}`; shipped `{visibility}` is owner-only (P1.1) |
| GET | `/api/v1/assets` | `assets:read:own` | `?kind=&status=&cursor=&limit=`; see Pagination below |

*(Rev 3: all codes are the 0003-seeded catalog entries — no new permission
rows needed. The earlier 4-segment `media:asset:*:own` drafts are rejected by
`rbac.Parse`: wired through `RequirePermission` they panic at server start
(`MustParse` on the required code), and any dynamic `AllowsCode` check fails
closed — returning false even for a `*` superadmin grant.)*

**`user`-tier grant (README AuthZ floor).** 0003 seeds `assets:write:own` and
`assets:delete:own` to `creator` only, but comics (0025), journal photos
(SPEC-12) and avatars (SPEC-08 P1.7) are `user` features that upload. A new
migration `000N_media_user_asset_grants` seeds `('user','assets:write:own')` and
`('user','assets:delete:own')` via the 0003 `WITH grants(...)` pattern
(`ON CONFLICT DO NOTHING`); 0008 is applied and is not edited. Only after that
migration is applied do upload-sessions and `/complete` enforce
`RequirePermission("assets:write:own")`; today they are authenticated-only.
(PATCH is gated by `RequireOwnerOrPermission`, P1.1, not by this grant.)

Annotate per the README OpenAPI encoding (`security: []` for the variant row;
`x-required-permission: {owner_or: assets:delete:any}` for DELETE,
`{owner_or: assets:write:any}` for PATCH `{title}`).

**Pagination** (README convention, with this endpoint's OpenAPI-declared
limits kept): `GET /api/v1/assets?kind=&status=&cursor=&limit=` responds
`{items: Asset[], next_cursor?}` — `limit`
default 50, max 100; an out-of-range value falls back to 50. Opaque cursor over
`(created_at, id)` DESC. `status=processing` expands server-side to
`processing,uploading`; `deleting` is never returned. `next_cursor` is present
only when another page exists. A malformed cursor is 400 `media/invalid-cursor`
and a param-shape failure 422 `media/validation`. The `{items}` envelope is
the README rule, which retrofits pre-rule endpoints (owner decision
2026-09-30); there is no `assets` key. *(Rev 4. Code follow-up: the shipped
handler and `shared/openapi.yaml` still answer `{assets: [...], next_cursor?}`
and a bad cursor with 400 `media/bad_request` ("invalid cursor"); the
retrofit renames the key to `items` in the handler, `shared/openapi.yaml` and
every frontend reader of `/assets` in one PR.)*

Problem types: `media/unsupported-format` (detail names HEIC when detected),
`media/file-too-large`, `media/asset-not-found`, `media/asset-not-ready`,
`media/invalid-cursor`, `media/validation`.

## 8. Success metrics (n=1 honest)

- Leading: p95 image processing time < 10 s (12 MP input); image ingest failure
  rate < 2% excluding deliberately invalid files.
- Leading *(rev 2)*: **zero OOM-killed worker/API processes** during the first
  dogfood month with image processing live (check `dmesg`/container restarts).
- Leading: comic page loads in SPEC-02 use `medium` variants 100% of the time.
- Lagging: storage stops ratcheting — deletes reclaim space (MinIO metrics after a
  delete pass); original downloads verify byte-identical at spot checks.

## 9. Timeline & phasing

1. Migration + variants queries + sqlc (½ day)
2. `platform/events` fan-out — `Publish` + event-name→consumer-task subscription
   registry in each emitting binary (`cmd/api` and/or `cmd/worker`) (P0.6) (½ day)
3. Shared `asynq.Scheduler` single registration point in `cmd/worker` (P0.3) — the
   janitor and all future periodic tasks register here (¼ day)
4. `media:process_image` worker + ingest wiring + `image`/`heavy` queue servers (1 day)
5. `HandleThumbnail` incl. zero-video-stream branch (½ day)
6. DELETE + janitor (periodic task on the shared scheduler) (1 day)
7. Download-original endpoint (½ day) *(rev 2)*
8. Library page incl. download action (1 day)
9. P1 items (½ day)
Total ≈ 5¾ dev-days inside the v1 envelope *(was 4–5; +½ for P0.5, +¾ for the
events + shared-scheduler foundation)*.

## 10. Open questions

- **(engineering, non-blocking)** Storage key layout: is everything already under a
  per-asset prefix (enables prefix delete)? Verify in `platform/storage` before
  building P0.3; if not, enumerate keys from DB rows.
- **(engineering, non-blocking)** `IMAGE_CONCURRENCY` sizing: the shipped
  default is **3** (peak ≈ 3 × 256 MB decode + one transcode); set **1** on a
  VPS with < 4 GB RAM. Decided at deploy from the box's RAM; the `heavy`
  (transcode) server stays at 1 *(rev 4)*.
- **(engineering, non-blocking)** WebP quality 80 vs 85 for `medium` — eyeball on
  real comic pages during SPEC-02.
- **(product, non-blocking)** Should `/upload` merge into `/library/media` later?
  Note for the frontend IA pass.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top changed no code). The spec
text above is the target; this section lists every place the shipped code still
diverges from it, so an implementer needs nothing beyond this file. Rows are
ordered by severity — data loss and integrity first, then authorization, then
contract and polish. A row closes when the code matches the target and the
SPEC-01 rows of [TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md)
are regraded. Paths are relative to `backend/internal/modules/media/` unless
stated otherwise.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.3 Response; P0.3 step (5) | Once `deleting` commits the delete is irrevocable; a step-3/4 failure still answers 204 and the janitor finishes; `media:asset_deleted` is published after the row is gone. | `handler.go` `Delete` answers any non-`ErrNotFound` error with 500 `about:blank`. The whole request runs in the `RequireTenant` transaction, which `tenant/middleware/require_tenant.go` rolls back on any status ≥ 500, so the `SetAssetStatusDeleting` write is undone while `service.go` `purgeObjects` has already removed some storage objects: the asset reappears with its original or variants gone. `purgeAsset` → `emitDeleted` publishes inside that same uncommitted transaction. | **backend:** commit `status='deleting'` before the purge (own short transaction, or return 204 so the request transaction commits); on a step-3/4 error log a warning and answer 204; publish `media:asset_deleted` only after the row delete has committed (API path and janitor). **openapi:** DELETE description states 204 on purge failure and on an already-`deleting` asset. **test:** TC-MEDIA-049, TC-MEDIA-048. | F036 |
| 2 | P0.3 Delete while a worker is in flight; P0.1 step 4 | `process_image`/`transcode` set ready with `… WHERE id=$1 AND status='processing'`; thumbnail inserts `poster` only if the asset exists and is not `deleting`; on 0 rows (or FK/missing row) roll back, delete every object the worker uploaded, return nil, emit no `media:asset_ready`. | `query/assets.sql` `MarkAssetReady` and `MarkAssetImageReady` are `:exec` with no status predicate. `worker/process_image.go` `run` inserts variants, marks ready and calls `emitAssetReady` inside the transaction (published before commit). `worker/transcode.go` `run` marks ready, then enqueues the poster and emits `media:asset_ready` unconditionally. `worker/thumbnail.go` `run` calls `InsertVariant` with no status check. None clean up uploaded objects. | **backend:** both queries gain `AND status = 'processing'` and become `:execrows`; `worker.Repo.MarkReady`/`MarkImageReady` return the row count; `InsertVariant` for `poster` becomes `INSERT … SELECT … WHERE EXISTS (SELECT 1 FROM assets WHERE id=$1 AND status <> 'deleting')`. On 0 rows: return a sentinel that rolls the transaction back, then `DeletePrefix("variants/{id}/")` (and the HLS prefix for transcode), return nil, skip the poster enqueue and `media:asset_ready`. Publish `media:asset_ready` after commit. **test:** TC-MEDIA-050. | F037 |
| 3 | User stories ("never hangs in `processing`"); §6 Tenancy | Every worker write runs in the asset owner's tenant scope (ADR-07). | `worker/transcode.go` `Handle` calls `t.repo.MarkFailed` outside `inTenant` (`process_image.go` wraps it correctly). Once the app runs as `portal_app`, a failed transcode cannot be marked `failed` and the asset stays `processing`. | **backend:** wrap the failure `MarkFailed` in `inTenant(ctx, t.run_, TaskTypeTranscode, p.OwnerUserID, …)`. **test:** a transcode failure under a tenant-scoping fake marks the asset `failed`. | found while verifying (2026-10-01) |
| 4 | P0.5 AC (abandoned upload) | An asset `failed` with `error_message='upload abandoned'` answers 404 `media/asset-not-found`; its possibly partial object is never streamed. | `service.go` `OriginalContent` (the function the `/original` route uses) rejects only `deleting` (404) and `uploading` (409). | **backend:** `OriginalContent` (and `DownloadOriginal`) return `ErrNotFound` for `status='failed' AND error_message='upload abandoned'`. **test:** TC-MEDIA-087. | F035 |
| 5 | P0.5 AC (purged at `/complete`) | An asset rejected at `/complete` (file-too-large, unsupported-format; object deleted) answers 404 `media/asset-not-found`. | `objectreader.go` `Read` turns `storage.ErrNotFound` into `io.EOF`, so `http.ServeContent` answers 200 with `Content-Length` = `size_bytes` and an empty body. | **backend:** `OriginalContent` confirms the object exists (`store.Size`, or the first ranged GET) before writing headers and returns `ErrNotFound` when it is absent. **test:** TC-MEDIA-085 against the HTTP route, not only `DownloadOriginal`. | found while verifying (2026-10-01) |
| 6 | P0.5 (byte-identical, Range-capable); P0.1 HEAD re-check | The original streams byte-identical, sized from the stored object. | `service.go` `CompleteUpload` measures the true size with `store.Size` but never stores it; `assets.size_bytes` keeps the client-declared value from `POST /assets`, and `OriginalContent` sizes every `ServeContent` response from it. A wrong declared size truncates or overruns the download. | **backend:** `/complete` writes `size_bytes` = the HEAD size for every kind (extend `MarkAssetProcessing` / add `SetAssetSize`). **test:** TC-MEDIA-080 with a declared size that differs from the body. | found while verifying (2026-10-01) |
| 7 | P0.1 Sniffed type is stored; P0.5 content type | For `kind='image'`, `/complete` overwrites `mime_type` with the sniffed `image/jpeg\|png\|webp`; `/original` serves `assets.mime_type`. | `service.go` `completeImage` calls `sniffImageType` and discards the type (`_, ok :=`); `mime_type` stays the declared content type. | **backend:** pass the sniffed type to `MarkAssetProcessing` (new `mime_type` parameter, images only). **test:** TC-MEDIA-004 asserts the stored `mime_type` after a PNG body declared `image/jpeg`. | F039 |
| 8 | P0.5 filename fallback | `original_filename` NULL → `{asset_id}.{ext}`, `ext` from the sniffed type or the `source_key` extension. | `service.go` `OriginalContent` falls back to the bare `asset.ID.String()` (no extension). `DownloadOriginal` has the right fallback but no route calls it. | **backend:** one shared filename helper used by both. **test:** TC-MEDIA-081 against `OriginalContent`. | F039 |
| 9 | §7 `user`-tier grant | Migration `000N_media_user_asset_grants` seeds `('user','assets:write:own')` and `('user','assets:delete:own')`; after it, `POST /assets` and `/complete` enforce `RequirePermission("assets:write:own")`. | `0003_account_rbac` grants both codes to `creator` only and no later migration widens them. `module.go` `MountHTTP` mounts `POST /`, `PUT /{id}/source` and `POST /{id}/complete` behind `RequireAuth` only. | **migration:** `000N_media_user_asset_grants` (0003 `WITH grants(...)` pattern, `ON CONFLICT DO NOTHING`). **backend:** add a `RequirePermission` dep to `media.Deps` and wrap the three upload routes in `assets:write:own`. **openapi:** `x-required-permission`. **test:** a `user` can upload; a role without the code gets 403. | F009 |
| 10 | P1.3 `mediaapi.Ingest(…, origin)`; P1.2 flood guard | `Ingest` takes `origin`; bulk producers pass `import`; the HTTP upload session always passes `upload`. | `api/api.go` `Ingest(ctx, ownerID, filename, contentType, data)` has no `origin`; `service.go` `Ingest` → `CreateUploadSession` hard-codes `Origin: "upload"`. Callers: `comic/import.go` `RunImport` and the music zip import. The `cmd/worker/main.go` comment records that the import-flood guard never fires. | **backend:** add `origin string` to `mediaapi.API.Ingest`, `Service.Ingest` and `CreateUploadSession` (validate `upload\|import`); the HTTP handler passes `upload`; comic and music imports pass `import`. **test:** TC-MEDIA-102 (an ingested asset carries `origin='import'`). | F038 |
| 11 | P1.2 payload `title` | `title` = `COALESCE(title, original_filename, id::text)`. | `repository/adapter.go` `LoadAssetMeta` stops at `original_filename` (empty for pre-0008 rows); `service.go` `completeAudio` publishes `asset.Title` with no fallback. | **backend:** apply the full fallback in `LoadAssetMeta` and `completeAudio`. **test:** TC-MEDIA-101 with a NULL-title row. | found while verifying (2026-10-01) |
| 12 | §7 Pagination (list envelope, Problems) | `GET /assets` answers `{items, next_cursor?}`; malformed cursor 400 `media/invalid-cursor`; param-shape failure 422 `media/validation`. | `handler.go` `List` answers `{assets: [...]}` and a bad cursor with 400 `media/bad_request` ("invalid cursor"); unknown `kind`/`status` values are passed through unvalidated (an empty page, never 422). `shared/openapi.yaml` `listAssets` declares `required: [assets]`. Frontend `lib/media-assets.ts` `ListAssetsPage.assets` and `MediaIndexView.tsx` read `page.assets`. `frontend/src/lib/problems.ts` lacks `media/invalid-cursor` and `media/validation`. | One PR: **backend** renames the key to `items`, emits `media/invalid-cursor`, and answers an unknown `kind`/`status` with 422 `media/validation`; **openapi** `required: [items]` + 400/422 responses; **frontend** readers switch to `items`; add both slugs to `problems.ts`. **test:** `TestListPaginates` asserts `items`; add a bad-cursor HTTP test (TC-MEDIA-063). | F112, F024, Decision 2026-09-30 (Envelopes) |
| 13 | §6 indexes; P0.3 janitor | `assets_uploading_idx ON assets (updated_at) WHERE status='uploading'` backs the abandoned-upload sweep. | Not in `0008_media_image_pipeline` or any later migration; `query/assets.sql` `ListAbandonedUploads` scans without it. | **migration:** new `000N_media_uploading_idx` (0008 is applied and is not edited). **test:** TC-MEDIA-114 up/down. | F115 |
| 14 | P1.1 metadata edit | `PATCH /assets/{id} {title}` (trimmed 1–200 chars; `""` → 422 `media/validation`; `null` → reset to `original_filename`) behind `RequireOwnerOrPermission(engine, "assets:write:any", extractAssetOwner)`; `{visibility}` stays owner-only; inline rename in the library. | `handler.go` `Patch` accepts only `{visibility}` (`"nothing to update"` otherwise); no `title` query; no owner-or-permission middleware on PATCH. `MediaIndexView.tsx` has no rename control. | **backend:** add `SetAssetTitle` query + handler branch; mount the owner-or-`assets:write:any` middleware for the `title` branch only (visibility keeps its owner predicate). **openapi:** `title` in the PATCH body, 422 `media/validation`. **frontend:** inline rename. **test:** TC-MEDIA-100. | §5 P1.1 inline note (rev 4) |
| 15 | P0.4 deep link (SPEC-07 P0.4 Media deep-link rule) | An image link is `/library/media?open={id}`; the grid reads `open` and opens that asset's lightbox, or shows a not-found toast. | `templates/v1/views/library/media/MediaIndexView.tsx` reads only `kind` and `status` from the search params. | **frontend:** read `open`, open the lightbox when the id is in the loaded pages (fetch `GET /assets/{id}` otherwise), toast on 404. | F018 |
| 16 | Decision 2026-09-30 (Audio) — SPEC-07 P0.1 | Audio `duration_ms` is probed from the original at `/complete` so audio can save progress and join `/continue` (requirement owned by SPEC-07). | `service.go` `completeAudio` calls `MarkReady(ctx, id, "", nil, nil, nil)` — no duration. | **backend:** ffprobe the original at `/complete` for audio (or a light follow-up task) and store `duration_ms`. Tracked with SPEC-07's gap list. | Decision 2026-09-30 (Audio) |
| 17 | §7 OpenAPI encoding | Variant row `security: []`; `x-required-permission: {owner_or: assets:delete:any}` on DELETE and `{owner_or: assets:write:any}` on PATCH `{title}`. | `shared/openapi.yaml` has no `x-required-permission` anywhere; `getAssetVariant` declares no `security`; the `completeAssetUpload` description still says images enqueue a thumbnail job. | **openapi:** add the annotations and fix the description. **test:** TC-MEDIA-112 drift check. | F025 (README OpenAPI encoding) |

**Already matching (verified on HEAD — do not redo).**
- Worker admission rule: `worker/process_image.go` `checkImageDims` (area ≤ 64 MP, side ≤ 30,000 px, animated refused) and `encodeWebP` (fit within max-width × 16,000 px, never upscaled, `-map_metadata -1`); the original is never re-encoded.
- Queues: `media:process_image` on `image` (`IMAGE_CONCURRENCY`, default 3), `media:transcode` alone on `heavy` (Concurrency 1), `media:thumbnail` on the light weighted server (`cmd/worker/main.go`).
- `/complete`: magic-byte sniff, HEIC hint, unknown format and > 50 MB each delete the object and mark the asset `failed` (`completeImage`).
- `POST /assets` records `title` = `original_filename` = the upload filename and `origin='upload'` (`CreateUploadSession`).
- Variants: `ServeVariant` 404s `deleting` assets; `Cache-Control: private, max-age=600` or `public, max-age=86400` (`handler.go` `cacheControl`).
- `/original` streams through `http.ServeContent` with `Content-Disposition: inline` and is owner-scoped with no `:any`/`*` bypass (`OriginalContent` → `owned`); `uploading` → 409.
- DELETE is gated by `RequireOwnerOrPermission(engine, "assets:delete:any", extractAssetOwner)` (`cmd/api/main.go`); a missing row is 404.
- Janitor: hourly `media:purge_orphans` on the single `asynq.Scheduler`, 15-min grace, 24 h abandoned sweep, error log after 5 consecutive failures (`PurgeOrphans`, `recordPurge`).
- Poster: separate `media:thumbnail` task, `min(10 %, 10 s)` seek, audio-only skipped, never fails the asset (`worker/thumbnail.go`).
- `platform/events` fan-out (P0.6) and the library keyset (`ListAssetsByOwnerCursor`, `processing` → `processing,uploading`, limit 50/100 with fallback).

**Test evidence to add/fix.**
- `service_test.go: TestListPaginates` reads the old `{assets}` result shape through the service; once the handler renames the key, add an HTTP test asserting `items` and 400 `media/invalid-cursor` (TC-MEDIA-063).
- `service_test.go: TestDownloadOriginal` exercises `DownloadOriginal`, which no route calls; retarget it at `OriginalContent` and add the abandoned (TC-MEDIA-087), purged-at-`/complete` (TC-MEDIA-085) and NULL-filename (TC-MEDIA-081) cases.
- `service_test.go: TestDeleteAsset` covers only the happy path; add the purge-failure → 204 + janitor case (TC-MEDIA-049) and an assertion that `media:asset_deleted` is published after commit (TC-MEDIA-048, TC-MEDIA-092).
- New worker tests for the lost race (TC-MEDIA-050): process_image, transcode and thumbnail each with the asset deleted mid-run.
- `service_test.go: TestCompleteUploadImageAccepted` should assert the stored sniffed `mime_type` and HEAD size (TC-MEDIA-004, TC-MEDIA-080).
- New: `Ingest` with `origin='import'` (TC-MEDIA-102), PATCH `{title}` (TC-MEDIA-100), grant migration + `RequirePermission` on upload (TC-MEDIA-114 and a 403 case).

## 12. Revision history

| Rev | Date | Change |
|---|---|---|
| r1 | 2026-07-07 | Initial spec from brief 01. |
| r2 | 2026-07-09 | External review integrated — 6 findings: **(1)** OOM guard: dimension cap 12,000→8,000 px + shared low-concurrency heavy queue *(accepted)*; **(2)** HEIC: explicit v1 non-goal with detected-and-helpful rejection + re-entry condition *(accepted)*; **(3)** originals preserved byte-identical (EXIF intact), metadata stripped on variants only, new authenticated download endpoint P0.5 *(accepted, sharpened — review asked for a download button; the EXIF split and private-URL requirement follow from it)*; **(4)** janitor mechanics defined: hourly periodic task, 15-min grace, error-log after 5 failures *(accepted; CPU-cost rationale trimmed)*; **(5)** audio-only-container branch in poster step *(accepted)*; **(6)** drop variant CHECK constraint *(rejected — DB integrity kept; migration cost accepted deliberately)*. |
| r3 | 2026-07-10 | Code-verified corrections from the multi-lens spec review: **(1)** table is `assets`, not `media_assets` — DDL/prose fixed, `assets` gains `title`/`original_filename` (P0.4/P0.5/P1.1/P1.2 were unimplementable without them); **(2)** ingest aligned to the shipped presigned-PUT + `/complete` lifecycle (`uploading→processing→ready\|failed`; no `uploaded` state) — sniffing/size checks moved to `/complete`, animated rejection stays worker-side; **(3)** permission codes reconciled to the seeded 3-segment catalog (`assets:read/write/delete:own`) — 4-segment drafts are unparseable; **(4)** heavy-queue guardrail mechanism corrected: second `asynq.Server` with own concurrency (weights cannot cap parallelism); **(5)** `media:asset_ready` payload gains `origin` (import-flood suppression for SPEC-02 P1.7 consumers); **(6)** list pagination resolved to cursor; status filter covers `uploading`; `media/asset-not-ready` given its use (P0.5 on `uploading`); **(7)** P0.6 `platform/events` fan-out (F007); **(8)** single shared `asynq.Scheduler` (F002); **(9)** abandoned `uploading` sweep (F038); **(10)** `assets.origin` (F009) and `assets_deleting_idx`/`assets_owner_cursor_idx` (F040); **(11)** unsupported-format → object deleted + `failed` (F010); `original_filename` fallback (F039); template-registry note (F006). |
| r4 | 2026-09-30 | Spec-gap fixes, code-verified: image cap is **area** (≤ 64 MP, ≤ 30,000 px per side) with a 16,000 px variant height clamp, admitting webtoon strips; `media:process_image` on its own `image` queue (`IMAGE_CONCURRENCY`, default 3), transcode alone on `heavy`; original download and PATCH `{title}` ownership-gated (no `:any`/`*` bypass for originals; `RequireOwnerOrPermission` + `assets:write:any` for PATCH), P0.3 gate rationale corrected; DELETE 204 once `deleting` commits; worker status guard for delete-in-flight; poster via a separate `media:thumbnail` task; sniffed `mime_type` stored; P1.3 `mediaapi.Ingest` with `origin`; `title` set at creation + COALESCE reads + PATCH validation; list contract (`limit`, `next_cursor`, status expansion); variant `Cache-Control`; `assets_uploading_idx`; `process_image` payload per events.md; scheduler list attributions; POST `/assets` row in §7; missing ACs (P0.3 events/sweep/scheduler, P0.5 abandoned, P0.6). Code follow-ups flagged inline. |
| r4 | 2026-10-01 | Added §11 implementation gaps (self-contained follow-up list); Revision history renumbered §12. |
