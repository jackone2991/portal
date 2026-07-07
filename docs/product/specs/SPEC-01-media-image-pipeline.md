# SPEC-01 — Media Image Pipeline + Asset Management

**Module:** `media` (built + wired) · **Status:** ready to build · **Depends on:** nothing
**Upstream:** `feature/01-media-image-pipeline.md` · **Refs:** missing-features §2, feature.md §3
**Downstream consumers:** SPEC-02 (comic pages), avatars (later), SPEC-03 P1 (receipts)

---

## 1. Problem statement

The `media_assets` schema already permits `kind = image`, but the worker pipeline only
handles video: uploading an image today either fails or strands the asset in a
non-`ready` state. `worker.HandleThumbnail` is a stub, so video assets have no
posters. There is no way to delete an asset (rows and storage grow forever on a
≤$100/mo VPS budget), and the only asset listing is the small one embedded in
`/upload`. Image support is the shared bottleneck for the entire entertainment axis
and several scattered backlog items (avatar upload, photos, receipts).

## 2. Goals

1. An image uploaded through the existing flow reaches `ready` with web-optimized
   variants in < 10 s for a 12 MP JPEG on the dev VPS class.
2. Every video asset transcoded after this ships shows a real poster frame.
3. An owner can delete any of their assets; DB rows and **all** storage objects are
   gone afterwards.
4. A `/library/media` page lists every asset the user owns with filter + pagination.
5. Zero regressions to the existing video path (upload → HLS → Vidstack).

## 3. Non-goals

- Multi-rendition HLS ladder, playback ACL, presigned direct-to-bucket upload —
  tracked separately in missing-features §2; this spec neither builds nor blocks them.
- Audio asset kind (P2 placeholder only).
- Image *editing* (crop, rotate-on-demand, filters). Auto-orientation on ingest is
  in scope (§6.1); user-driven editing is not.
- Animated images. Animated GIF/WebP inputs are **rejected** at v1 (Problem
  `media/unsupported-format`); treating them as video is a future decision.
- De-duplication of identical uploads (content hashing) — future consideration.

## 4. User stories

- As a comic creator, I upload page images and get web-optimized variants so readers
  load fast on mobile. *(primary — feeds SPEC-02)*
- As an asset owner, I delete an asset and it disappears from listings, playback,
  and object storage, so disk/R2 usage doesn't grow forever.
- As a user, I browse everything I've uploaded in one place, filter by kind/status,
  and page through it.
- As a user, when my phone photo is sideways (EXIF orientation), the stored image
  displays upright everywhere.
- As a user, when I upload a corrupt file, I see the asset marked failed with a
  reason — the system never hangs in `processing`.

## 5. Requirements

### P0.1 — Image ingest

**Behavior.** The existing upload endpoint accepts `image/jpeg`, `image/png`,
`image/webp`. MIME is determined by **content sniffing** (magic bytes), never by
file extension or client-declared Content-Type alone. On success the asset row is
created with `kind='image'`, `status='uploaded'`, and task `media:process_image`
`{asset_id}` is enqueued.

**Worker task `media:process_image`.**
1. Download original from storage.
2. Probe with ffprobe: reject if animated (frame count > 1), if either dimension
   > 12,000 px, or if decode fails → `status='failed'`, `error_message` set.
3. Apply EXIF auto-orientation, then **strip all metadata** (`-map_metadata -1`).
4. Generate variants (all WebP, quality ~80, `scale=W:-2`, never upscale):
   | variant | max width | purpose |
   |---|---|---|
   | `thumb` | 320 | library grid, pickers |
   | `medium` | 1280 | comic reader, detail views |
   | `original` | — | stored as-is post-orientation/strip |
5. Insert `media_asset_variants` rows; set `status='ready'`.

Implementation note: use ffmpeg (already in the worker image — zero new deps);
the task handler is the seam if a Go imaging lib is ever swapped in.

**Acceptance criteria.**
- Given a 12 MP JPEG upload, when the worker completes, then the asset is `ready`
  with `thumb` + `medium` + `original` variants present in storage and DB.
- Given a JPEG with EXIF Orientation=6 and GPS tags, when processed, then all
  variants display upright and `exiftool` shows no GPS/metadata on any variant.
- Given a PNG with transparency, when processed, then transparency is preserved in
  WebP variants.
- Given a corrupt file / animated GIF / 15,000 px image, when processed, then
  `status='failed'` with a human-readable `error_message`; the worker process does
  not crash and the queue continues.
- Given an 800 px-wide image, when processed, then the `medium` variant is 800 px
  (no upscaling) and `thumb` is 320 px.
- Upload size limit: files > 50 MB are rejected at the API with Problem
  `media/file-too-large` before any storage write.

### P0.2 — Video poster thumbnail (kill the stub)

**Behavior.** `worker.HandleThumbnail` implemented: after (or as the final step of)
transcode, probe duration, seek to `min(10% of duration, 10s)`, extract one frame,
scale to 640 w, store as WebP, insert a `poster` variant row on the video asset.

**Acceptance criteria.**
- Given a newly uploaded video, when transcode completes, then a `poster` variant
  exists and renders in the library grid and as the Vidstack poster.
- Given a 2 s video, when thumbnailed, then extraction succeeds (seek point is
  clamped inside the file).
- Poster failure does **not** fail the asset: video remains `ready`, poster is
  absent, a warning is logged. (Playback > cosmetics.)

### P0.3 — Delete asset

**Endpoint.** `DELETE /api/v1/assets/{id}` — permission `media:asset:delete:own`
(wildcard grants cover admin).

**Behavior.** In order: (1) authorize ownership; (2) set `status='deleting'` (excluded
from all listings); (3) delete every storage object for the asset — original, HLS
playlist + segments, all variants (delete by the asset's storage key prefix
`assets/{id}/` if the layout allows, else enumerate known keys); (4) delete variant
rows + asset row. If storage purge partially fails, leave the row in `deleting` and
retry via a janitor task `media:purge_orphans` (also P0 — it is what makes delete
trustworthy).

**Acceptance criteria.**
- Given a ready video asset, when deleted, then its HLS URL and all variant URLs
  return 404/403 and `mc ls` (MinIO) shows no objects under its prefix.
- Given an already-deleted id, when DELETE is called again, then 404 (idempotent,
  never 500).
- Given user B calling DELETE on user A's asset without wildcard perms, then 403
  and nothing is deleted.
- Given a storage outage mid-delete, when the janitor runs after recovery, then the
  asset finishes deleting; it never reappears in listings meanwhile.

### P0.4 — Library page

**Route.** `/library/media` (RSC catalogue shell + client islands per D-33).

**Behavior.** Grid of the user's assets: poster/thumb, title, kind badge, status
badge, created date. Filters: kind (all/video/image), status (ready/processing/failed).
Pagination (cursor or page — match the existing list endpoint's style). Row actions:
open (video → player, image → lightbox showing `medium`), delete (confirm dialog →
optimistic removal). Extend `GET /api/v1/assets` with `?kind=&status=` and pagination
if not already present. `failed` assets show `error_message` on hover/expand and
offer only delete.

**Acceptance criteria.**
- Given 100 assets, when the page loads, then the first page renders < 2.5 s LCP
  (thumb variants only — never originals in the grid).
- Given a delete confirmation, when it succeeds, then the card disappears without a
  full refetch (TanStack cache mutation).
- Empty state links to `/upload`.

### P1 — nice to have

- **P1.1 Metadata edit**: `PATCH /api/v1/assets/{id}` `{title}` — permission
  `media:asset:update:own`; inline rename in the library.
- **P1.2 Event emit**: publish `media:asset_ready` `{asset_id, kind, owner_user_id,
  title}` on the bus whenever any asset reaches `ready`. First life-stream producer;
  no consumer required to ship (the notification module consumes later).

### P2 — future considerations (design for, don't build)

- Audio kind: schema allows it; keep the task-per-kind dispatch shape so
  `media:process_audio` slots in.
- Bulk upload (multi-file / zip): SPEC-02 P1.7 is the concrete consumer; API shape
  should not preclude a batch-create of assets.
- Content-hash dedup: leave a nullable `content_sha256` column *decision* open — not
  added now, but don't design the storage layout to prevent it.

## 6. Data model

New table — migration `000N_media_variants` (take the next free number):

```sql
CREATE TABLE media_asset_variants (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id    uuid NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
  variant     text NOT NULL CHECK (variant IN ('thumb','medium','original','poster')),
  storage_key text NOT NULL,
  width       int  NOT NULL,
  height      int  NOT NULL,
  size_bytes  bigint NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (asset_id, variant)
);
```

Same-module FK is allowed (boundary rule forbids only cross-module FKs). If
`media_assets.status` lacks `'deleting'`, extend the check constraint in the same
migration. Queries in `query/media_variants.sql`; regenerate via `make sqlc` — never
hand-edit `*.sql.go`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| DELETE | `/api/v1/assets/{id}` | `media:asset:delete:own` | 204; idempotent 404 |
| PATCH | `/api/v1/assets/{id}` | `media:asset:update:own` | P1; `{title}` |
| GET | `/api/v1/assets` | `media:asset:read:own` | extend: `?kind=&status=&page=` |

Problem types: `media/unsupported-format`, `media/file-too-large`,
`media/asset-not-found`, `media/asset-not-ready`.

## 8. Success metrics (n=1 honest)

- Leading: p95 image processing time < 10 s (12 MP input); image ingest failure
  rate < 2% excluding deliberately invalid files.
- Leading: comic page loads in SPEC-02 use `medium` variants 100% of the time
  (verified by storage access pattern / URL audit).
- Lagging: storage usage stops ratcheting — deletes reclaim space (check MinIO
  metrics after a delete pass).

## 9. Timeline & phasing

1. Migration + variants queries + sqlc (½ day)
2. `media:process_image` worker + ingest wiring (1 day)
3. `HandleThumbnail` (½ day)
4. DELETE + janitor (1 day)
5. Library page (1 day)
6. P1 items (½ day)
Total ≈ 4–5 dev-days inside the v1 envelope.

## 10. Open questions

- **(engineering, non-blocking)** Storage key layout: is everything already under
  a per-asset prefix (enables prefix delete)? Verify in `platform/storage` before
  building P0.3; if not, enumerate keys from DB rows instead.
- **(engineering, non-blocking)** WebP quality 80 vs 85 for `medium` — eyeball on
  real comic pages during SPEC-02; constant lives in one place.
- **(product, non-blocking)** Should `/upload` merge into `/library/media` later?
  Out of scope here; note for the frontend IA pass.
