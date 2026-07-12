# SPEC-01 — Media Image Pipeline + Asset Management

**Status:** current, **rev 3** (code-verified corrections) · **Last verified:** 2026-07-10
**Module:** `media` (built + wired) · **Depends on:** nothing
**Upstream:** [briefs/01-media-image-pipeline.md](../briefs/01-media-image-pipeline.md) · **Refs:** [backlog.md](../backlog.md) §2, feature-inventory.md §3
**Downstream consumers:** SPEC-02 (comic pages), avatars (later), SPEC-03 P1 (receipts), SPEC-04 (media:asset_ready → in-app notification)
**Rev 2 origin:** external technical review 2026-07-09 — six findings; five accepted, one rejected. See §11.

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
  tracked separately in [backlog.md](../backlog.md) §2; this spec neither builds nor blocks them.
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
`status='processing'` and enqueues `media:process_image` `{asset_id}`.

**Worker task `media:process_image`.**
1. Probe with **ffprobe (header read — before any full decode)**: reject if
   animated (frame count > 1), if either dimension > **8,000 px** *(rev 2, was
   12,000 — review pt 1)*, or if decode fails → `status='failed'`, `error_message`
   set. Memory rationale: decoded RGBA at 8,000² ≈ 256 MB peak vs ≈ 576 MB at
   12,000² — the difference between "tight" and "OOM-killer bait" on a small VPS.
2. **The uploaded original is never modified** — no re-encode, no orientation
   rewrite, no metadata strip. It stays byte-for-byte as uploaded at its source
   storage key *(rev 2, review pt 3 — archival guarantee)*.
3. Generate **served variants** (WebP, quality ~80, `scale=W:-2`, never upscale),
   each with EXIF **auto-orientation baked in** and **all metadata stripped**
   (`-map_metadata -1`):
   | variant | max width | purpose |
   |---|---|---|
   | `thumb` | 320 | library grid, pickers |
   | `medium` | 1280 | comic reader, detail views, lightbox |
4. Insert `media_asset_variants` rows; set `status='ready'`.

**Resource guardrails** *(rev 2, review pt 1; mechanism corrected in rev 3)*:
`media:process_image` and `media:transcode` run on a **`heavy` queue consumed
by a second `asynq.Server` in the same worker process with its own
`Concurrency: 1–2`**. Asynq queue *weights* only bias dequeue order within one
shared worker pool — the shipped server runs `Concurrency: 4` over
transcode/thumbnail/default, so no queue name or weight can cap a queue's
parallelism; the cap requires its own server. Light tasks (thumbnail, future
notify, janitor) stay on the existing weighted server so heavy jobs never
starve them. This serializes the expensive decodes instead of letting N large
images + a transcode decode simultaneously.

**Serving variants.** `GET /api/v1/assets/{id}/variants/{variant}`
(`variant ∈ thumb|medium|poster`) streams the stored variant object with its
WebP content type and long-lived cache headers. Same **public-ish** auth stance
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
- Given a corrupt file / animated GIF / **9,000 px** image, when processed, then
  `status='failed'` with a human-readable `error_message`; the worker process does
  not crash and the queue continues.
- Given a HEIC upload, when sniffed at `/complete`, then 422 Problem
  `media/unsupported-format` whose detail explicitly names HEIC and suggests
  converting to JPEG, **the uploaded object is deleted, and the asset is
  `failed`** — never enqueued *(rev 2; sniff location corrected rev 3;
  asset/object fate specified F010)*.
- Given an 800 px-wide image, then `medium` is 800 px (no upscaling), `thumb` 320 px.
- Given 3 heavy tasks enqueued at once (2 large images + 1 transcode), then at most
  the heavy server's configured concurrency (1–2) run simultaneously — assert via
  Asynq inspector or worker logs *(rev 2)*.
- Given a > 50 MB object at `/complete` (presign policy bypassed or unsupported),
  then Problem `media/file-too-large`, the object is deleted from storage, and the
  asset is `failed` — never enqueued *(rev 3 — the API cannot reject "before any
  storage write" under presigned direct upload)*.

### P0.2 — Video poster thumbnail (kill the stub)

**Behavior.** `worker.HandleThumbnail` implemented: after (or as the final step of)
transcode, probe duration, seek to `min(10% of duration, 10s)`, extract one frame,
scale to 640 w, store as WebP, insert a `poster` variant row on the video asset.
**If ffprobe reports zero video streams** (e.g. an `.mp3` renamed `.mp4` — an
audio-only container), **skip poster generation entirely and log a warning**; the
asset's playability is unaffected *(rev 2, review pt 5)*.

**Acceptance criteria.**
- Given a newly uploaded video, when transcode completes, then a `poster` variant
  exists and renders in the library grid and as the Vidstack poster.
- Given a 2 s video, when thumbnailed, then extraction succeeds (seek point clamped
  inside the file).
- Given an audio-only container (0 video streams), when the poster step runs, then
  it is skipped with a warning, no crash, and the asset still reaches `ready` *(rev 2)*.
- Poster failure does **not** fail the asset: video remains `ready`, poster absent,
  warning logged. (Playback > cosmetics.)

### P0.3 — Delete asset

**Endpoint.** `DELETE /api/v1/assets/{id}` — enforced via
`RequireOwnerOrPermission(engine, "assets:delete:any", extractAssetOwner)`
*(rev 3: a plain `RequirePermission("assets:delete:own")` would 403 admin's
0003-seeded `assets:delete:any` because the matcher never lets `:any` satisfy
`:own`; `assets:delete:own` remains the catalog entry documenting the owner
capability, and the middleware's owner branch admits owners)*.

**Behavior.** In order: (1) authorize ownership; (2) set `status='deleting'`
(excluded from all listings); (3) delete every storage object for the asset —
source original, HLS playlist + segments, all variants (by the asset's storage key
prefix `assets/{id}/` if the layout allows, else enumerate known keys); (4) delete
variant rows + asset row; (5) once the asset row is gone (via this path **or** the
janitor's), emit `media:asset_deleted` `{asset_id, owner_user_id}` (registry:
`docs/reference/events.md`) so domain modules that hold the id with no cross-module FK
— e.g. comic — can drop dangling references (SPEC-02 P0.6). Best-effort, like all
cross-module events; a dropped event is recoverable by a consumer-side reconcile.

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
(the "a purge is stuck" signal on a single-operator box). Cost note: this is an
indexed scan (`assets_deleting_idx`, §6) over a near-empty set — trigger cadence
is about *definedness*, not CPU.

**Shared periodic scheduler** *(F002 — single registration point)*: `cmd/worker`
stands up **one** `asynq.Scheduler` (`PeriodicTaskManager`) as the single place
all periodic tasks register. `media:purge_orphans` is the first entry; later
specs register their periodic tasks here too — `ops:backup_database` (SPEC-09),
`people:scan_birthdays` (SPEC-05), `notify:purge_old` (SPEC-04), account's
`PurgeExpiredRefreshTokens` — rather than each standing up its own scheduler or
reaching for OS cron. This is the runner the other specs "borrow the convention
of (Asynq periodic, never OS cron), not the code."

**Acceptance criteria.**
- Given a ready video asset, when deleted, then its HLS URL, variant URLs, **and
  original-download URL** return 404/403 and `mc ls` shows no objects under its prefix.
- Given an already-deleted id, when DELETE is called again, then 404 (idempotent,
  never 500).
- Given user B calling DELETE on user A's asset without wildcard perms, then 403
  and nothing is deleted.
- Given a storage outage mid-delete, when the janitor's next hourly run happens
  after recovery, then the asset finishes deleting; it never reappears in listings
  meanwhile; a stuck purge logs at error level after 5 attempts *(rev 2)*.

### P0.4 — Library page

**Route.** `/library/media` (RSC catalogue shell + client islands per D-33). The
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
expanded view)*, delete (confirm dialog → optimistic removal). Extend
`GET /api/v1/assets` with `?kind=&status=` and cursor pagination if not already
present. `failed` assets show `error_message` on hover/expand and offer only delete.

**Acceptance criteria.**
- Given 100 assets, when the page loads, then the first page renders < 2.5 s LCP
  (thumb variants only — never originals in the grid).
- Given a delete confirmation, when it succeeds, then the card disappears without a
  full refetch (TanStack cache mutation).
- Given an image lightbox, then a Download-original action is present and works *(rev 2)*.
- Empty state links to `/upload`.

### P0.5 — Download original *(new in rev 2, review pt 3)*

**Endpoint.** `GET /api/v1/assets/{id}/original` — permission
`assets:read:own` *(rev 3: seeded to `user` by 0003)*.

**Behavior.** Streams the source object with
`Content-Disposition: attachment; filename="<original filename>"` and the sniffed
content type. When `original_filename` is null (assets predating the §6
migration), fall back to `{asset_id}.{ext}` where `ext` is derived from the
sniffed content type (or the `source_key` extension) *(F039)*. **Owner-authenticated
proxy only — the original must never be reachable through the public-ish
variant/HLS URL scheme**, because (unlike the stripped variants) it retains full
EXIF including GPS. This asymmetry is the point: variants are safe to be
semi-public, originals are private archive.

**Acceptance criteria.**
- Given the owner requests the original, then the downloaded file's checksum equals
  the uploaded file's checksum (byte-identical, EXIF intact).
- Given user B (no wildcard perms) requests user A's original, then 403/404.
- Given an asset in `processing`/`failed` whose source object still exists
  (worker-side failure: corrupt/animated/oversized-dimensions), the original is
  still downloadable. Given an asset rejected at `/complete` with its object
  purged (file-too-large, unsupported-format), then 404 Problem
  `media/asset-not-found` — the archival guarantee applies only to accepted
  uploads.
- Given an asset still `uploading` (browser PUT unfinished or abandoned), then
  409 Problem `media/asset-not-ready` — the source object may be partial
  *(rev 3 — this is the declared use of that Problem type)*.

### P0.6 — Event fan-out (`platform/events`) *(prerequisite; first producer owns it)*

**Deliverable** *(F007)*: build `platform/events` — a
`Publish(ctx, name, payload)` helper plus an **event-name → consumer-task
subscription table** registered in `cmd/worker`, so a published event
(`<module>:<event>`) fans out to every subscribed Asynq task (events.md
"Delivery mechanics"). SPEC-01 is the first producer (`media:asset_ready`,
`media:asset_deleted`) and therefore owns constructing it; **every later
multi-consumer stream/notify feature gates on this** (SPEC-04 bell, SPEC-06
stream, …) — events.md warns that publishing an event name with no registered
subscription panics `cmd/worker`, so the registry must exist before the first
fan-out event ships. No consumer is required for SPEC-01 itself to land.

### P1 — nice to have

- **P1.1 Metadata edit**: `PATCH /api/v1/assets/{id}` `{title}` — permission
  `assets:write:own` *(rev 3: seeded to `creator` by 0003)*; inline rename in
  the library. Operates on the `title` column §6 adds.
- **P1.2 Event emit**: publish `media:asset_ready` `{asset_id, kind, owner_user_id,
  title, origin}` — via the `platform/events` publisher (events.md "Delivery
  mechanics"; built as the P0.6 prerequisite) — whenever any asset reaches
  `ready`. `title` falls back to `original_filename` when unset. `origin` is
  **read from `assets.origin`** (§6), set at row creation — `'upload'` by the
  upload-session endpoint, `'import'` by the mediaapi batch-create SPEC-02 P1.7
  uses — so the worker task payload (`{asset_id}`) need not carry it and
  consumers can suppress bulk floods (the bell/stream must not get 300 items for
  one chapter import) *(rev 3; persistence F009)*. mediaapi asset listings also
  expose `origin` (SPEC-06 P1.6 backfill needs it). First life-stream producer;
  no consumer required to ship.

### P2 — future considerations (design for, don't build)

- Audio kind: schema allows it; keep the task-per-kind dispatch shape so
  `media:process_audio` slots in. (The variant enum will need e.g. `waveform` —
  see the deliberate migration-cost note in §6.)
- Bulk upload (multi-file / zip): SPEC-02 P1.7 is the concrete consumer; API shape
  should not preclude a batch-create of assets.
- Content-hash dedup: nullable `content_sha256` column decision stays open.
- HEIC ingest — deferred with an explicit re-entry condition (§3).

## 6. Data model

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

-- Back the P0.3 janitor's "indexed scan" and the P0.4 cursor keyset (F040):
CREATE INDEX assets_deleting_idx ON assets (updated_at) WHERE status = 'deleting';
CREATE INDEX assets_owner_cursor_idx ON assets (owner_id, created_at DESC, id DESC);
```

`'deleting'` is required by P0.3. `title` / `original_filename` are required by
P0.4 (grid title), P0.5 (`Content-Disposition` filename), P1.1 (rename) and
P1.2 (event `title`) — the shipped table stores neither (the upload filename
today only picks a storage-key extension). The upload-session endpoint starts
recording `original_filename`; `title` defaults to it. `origin` is required by
P1.2 (the `media:asset_ready` payload's flood guard): set at row creation —
`'upload'` by the upload-session endpoint, `'import'` by the mediaapi
batch-create SPEC-02 P1.7 uses — and exposed on mediaapi asset listings
(SPEC-06 P1.6 backfill needs it). The two indexes back the P0.3 janitor scan
(`assets_deleting_idx`) and the P0.4 keyset (`assets_owner_cursor_idx`, replacing
reliance on `assets_owner_idx` for the library cursor). Queries in
`query/media_variants.sql`; regenerate via `make sqlc` — never hand-edit
`*.sql.go`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| POST | `/api/v1/assets/{id}/complete` | `assets:write:own` | modified: magic-byte sniff + HEAD size re-check; new Problems `media/unsupported-format` (422), `media/file-too-large` |
| DELETE | `/api/v1/assets/{id}` | owner, or `assets:delete:any` (RequireOwnerOrPermission) | 204; idempotent 404 |
| GET | `/api/v1/assets/{id}/original` | `assets:read:own` | rev 2; attachment stream; never public |
| GET | `/api/v1/assets/{id}/variants/{variant}` | public-ish / unauthenticated (same as `/hls/*`) | `variant ∈ thumb\|medium\|poster`; streamed w/ content type + cache headers; 404 `media/asset-not-found` |
| PATCH | `/api/v1/assets/{id}` | `assets:write:own` | P1; `{title}` |
| GET | `/api/v1/assets` | `assets:read:own` | extend: `?kind=&status=&cursor=` |

*(Rev 3: all codes are the 0003-seeded catalog entries — no new permission
rows needed. The earlier 4-segment `media:asset:*:own` drafts are rejected by
`rbac.Parse`: wired through `RequirePermission` they panic at server start
(`MustParse` on the required code), and any dynamic `AllowsCode` check fails
closed — returning false even for a `*` superadmin grant.)*

Problem types: `media/unsupported-format` (detail names HEIC when detected),
`media/file-too-large`, `media/asset-not-found`, `media/asset-not-ready`.

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
   registry in `cmd/worker` (P0.6) (½ day)
3. Shared `asynq.Scheduler` single registration point in `cmd/worker` (P0.3) — the
   janitor and all future periodic tasks register here (¼ day)
4. `media:process_image` worker + ingest wiring + heavy-queue config (1 day)
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
- **(engineering, non-blocking)** Heavy-queue concurrency 1 vs 2: default **1** if
  the VPS has < 4 GB RAM, else 2; one config constant, decided at deploy.
- **(engineering, non-blocking)** WebP quality 80 vs 85 for `medium` — eyeball on
  real comic pages during SPEC-02.
- **(product, non-blocking)** Should `/upload` merge into `/library/media` later?
  Note for the frontend IA pass.

## 11. Revision history

| Rev | Date | Change |
|---|---|---|
| r1 | 2026-07-07 | Initial spec from brief 01. |
| r2 | 2026-07-09 | External review integrated — 6 findings: **(1)** OOM guard: dimension cap 12,000→8,000 px + shared low-concurrency heavy queue *(accepted)*; **(2)** HEIC: explicit v1 non-goal with detected-and-helpful rejection + re-entry condition *(accepted)*; **(3)** originals preserved byte-identical (EXIF intact), metadata stripped on variants only, new authenticated download endpoint P0.5 *(accepted, sharpened — review asked for a download button; the EXIF split and private-URL requirement follow from it)*; **(4)** janitor mechanics defined: hourly periodic task, 15-min grace, error-log after 5 failures *(accepted; CPU-cost rationale trimmed)*; **(5)** audio-only-container branch in poster step *(accepted)*; **(6)** drop variant CHECK constraint *(rejected — DB integrity kept; migration cost accepted deliberately)*. |
| r3 | 2026-07-10 | Code-verified corrections from the multi-lens spec review: **(1)** table is `assets`, not `media_assets` — DDL/prose fixed, `assets` gains `title`/`original_filename` (P0.4/P0.5/P1.1/P1.2 were unimplementable without them); **(2)** ingest aligned to the shipped presigned-PUT + `/complete` lifecycle (`uploading→processing→ready\|failed`; no `uploaded` state) — sniffing/size checks moved to `/complete`, animated rejection stays worker-side; **(3)** permission codes reconciled to the seeded 3-segment catalog (`assets:read/write/delete:own`) — 4-segment drafts are unparseable; **(4)** heavy-queue guardrail mechanism corrected: second `asynq.Server` with own concurrency (weights cannot cap parallelism); **(5)** `media:asset_ready` payload gains `origin` (import-flood suppression for SPEC-02 P1.7 consumers); **(6)** list pagination resolved to cursor; status filter covers `uploading`; `media/asset-not-ready` given its use (P0.5 on `uploading`). |
