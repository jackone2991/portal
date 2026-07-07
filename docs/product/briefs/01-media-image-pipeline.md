# 01 — Media: Image Asset Kind + Pipeline Completion

**Module:** `media` (built + wired) · **Effort:** ~days · **Depends on:** nothing.
**Unlocks:** comic pages (spec 02), avatars, photos, finance receipt attachments.

## Problem statement

The media schema already allows `image` kind, but the worker pipeline only handles
video; `worker.HandleThumbnail` is a stub; there is no `DELETE /assets`, and the only
asset list is the small one on `/upload`. Image support is the **shared bottleneck**
for the entertainment axis (comic = a sequence-of-images reader) and for several P2
items scattered in the backlog (avatar upload, photos).

## Goals

- An image uploaded through the existing upload flow reaches `ready` with derivatives.
- A video asset gets a real poster thumbnail (stub eliminated).
- A user can delete an asset and storage is actually purged.
- A media library page exists beyond `/upload`.

## Non-goals

- Multi-rendition HLS ladder, playback access control, presigned direct upload —
  all remain separate backlog items (missing-features §2), untouched here.
- Audio kind (P2 here, see Requirements).
- Any image *editing* (crop/rotate) — consumers do client-side crop before upload if needed.

## User stories

- As a comic creator, I upload page images and get web-optimized derivatives so the
  reader loads fast on mobile.
- As the owner of an asset, I delete it and it disappears from listing, playback,
  and object storage, so my VPS disk/R2 bill doesn't grow forever.
- As a user, I browse everything I've uploaded in one library page, filtered by
  kind and status.

## Requirements

### P0 — must have

1. **Image ingest**: upload accepts `image/jpeg|png|webp`; worker task
   (`media:process_image`) generates derivatives — suggested set: `thumb` (~320w),
   `medium` (~1280w), plus original; strips EXIF; marks asset `ready`.
   - [ ] Given a 12 MP JPEG upload, when the worker finishes, the asset is `ready`
         with 2 derivatives + original in storage.
   - [ ] EXIF GPS/location data is absent from all stored variants.
   - [ ] A corrupt/oversized file marks the asset `failed` with an error message,
         never crashes the worker.
2. **Video thumbnail**: `HandleThumbnail` implemented — extract a frame (e.g. at 10%
   duration) via ffmpeg, store as the asset's poster.
   - [ ] Every newly transcoded video shows a poster in the library and on Vidstack.
3. **Delete**: `DELETE /api/v1/assets/{id}` — permission `media:asset:delete:own`
   (admin wildcard covers the rest); deletes DB row(s) and purges all storage keys
   (original, renditions, variants).
   - [ ] After delete, the HLS URL and all variant URLs return 404/403.
   - [ ] Delete is idempotent (second call → 404, no 500).
4. **Library page**: `/library/media` — grid with poster/thumb, kind + status filter,
   pagination; row actions: open, delete.

### P1 — nice to have

5. Rename / metadata edit (`PATCH /assets/{id}`: title).
6. **Emit `media:asset_ready`** on the bus when any asset reaches `ready`
   (moves up from P3 — it is the first life-stream event producer).

### P2 — future considerations (design for, don't build)

7. Audio kind (schema already allows it).
8. Bulk upload (multi-file / zip) — spec 02 has a concrete consumer for zip.

## Data model sketch

New table (next free migration, `000N_media_variants`):

```
media_asset_variants(
  id uuid pk, asset_id uuid → media_assets (same module: FK OK),
  variant text check in ('thumb','medium','original','poster'),
  storage_key text, width int, height int, size_bytes bigint,
  created_at timestamptz
)
```

Video poster is a `poster` variant on the video asset — one mechanism for both kinds.

## API sketch (add to `shared/openapi.yaml`)

```
DELETE /api/v1/assets/{id}
PATCH  /api/v1/assets/{id}            {title}
GET    /api/v1/assets?kind=&status=&page=   (extend existing list)
```

## Open questions

- **(engineering, non-blocking)** Derivative generation via ffmpeg (already in the
  worker image, zero new deps) vs a Go imaging lib (sharper resizes, new dep).
  Recommendation: ffmpeg first; swap later behind the task handler.
- **(engineering, non-blocking)** Max accepted dimensions/size for images
  (suggest 50 MB / 12k px, validated at upload).
