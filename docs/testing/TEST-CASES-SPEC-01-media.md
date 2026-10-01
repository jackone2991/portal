# Test Cases — SPEC-01 Media Image Pipeline + Asset Management

**Status:** current · **Last verified:** never

**Spec:** [SPEC-01](../product/specs/SPEC-01-media-image-pipeline.md) · **Module:** `media`
**Prefix:** `TC-MEDIA-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| POST | `/api/v1/assets/{id}/complete` | `assets:write:own` (magic-byte sniff + HEAD size) |
| DELETE | `/api/v1/assets/{id}` | owner or `assets:delete:any` |
| GET | `/api/v1/assets/{id}/original` | authenticated, owner-scoped load (no `:any`/`*` bypass) |
| GET | `/api/v1/assets/{id}/variants/{variant}` | public-ish (unauthenticated) |
| PATCH | `/api/v1/assets/{id}` | owner or `assets:write:any` (P1.1, `{title}`) |
| POST | `/api/v1/assets` | `assets:write:own` (records `original_filename`, `title`, `origin='upload'`) |
| GET | `/api/v1/assets?kind=&status=&cursor=&limit=` | `assets:read:own` |

### Preconditions (all cases)

- Stack up (`make up`), worker running, MinIO reachable, migrations current.
- Accounts `owner` (creator), `userA`, `userB`, `admin`, `guest` per plan §4.
- Fixtures loaded per plan §5 (`12mp.jpg`, `orientation6_gps.jpg`, `transparent.png`,
  `photo.webp`, `animated.gif`, `sample.heic`, `corrupt.bin`, `8001x8000.jpg`,
  `webtoon_704x18000.jpg`,
  `oversize_51mb.jpg`, `short_2s.mp4`, `audio_only.mp4`).
- Problem types: `media/unsupported-format`, `media/file-too-large`,
  `media/asset-not-found`, `media/asset-not-ready`.

**Status legend:** `☐ Not run · ✅ PASS · ❌ FAIL · ⛔ BLOCKED · ➖ N/A`

---

## P0.1 — Image ingest

| ID | Scenario | Type | Pri | Steps (concise) | Expected | Status |
|----|----------|------|-----|-----------------|----------|--------|
| TC-MEDIA-001 | JPEG happy path → ready + variants | Functional | P0 | POST `/assets`{kind:image}; PUT bytes; POST `/complete`; poll `/assets/{id}` | status `uploading→processing→ready`; `thumb`+`medium` variant rows in DB & storage; source original checksum unchanged; ≤10 s p95 | ☐ |
| TC-MEDIA-002 | Accept `image/png` | Functional | P0 | same with `transparent.png` | reaches `ready`; transparency preserved in WebP variants | ☐ |
| TC-MEDIA-003 | Accept `image/webp` | Functional | P0 | same with `photo.webp` | reaches `ready` | ☐ |
| TC-MEDIA-004 | Sniff by magic bytes, not extension | Negative | P0 | upload `corrupt.bin` renamed `.jpg` (jpg magic + garbage body) then valid image renamed `.txt` | decision follows magic bytes, not filename/Content-Type | ☐ |
| TC-MEDIA-005 | HEIC rejected at `/complete` | Negative | P0 | upload `sample.heic`; POST `/complete` | 422 `media/unsupported-format`; **detail explicitly names HEIC + suggests convert-to-JPEG**; uploaded object deleted; asset `status=failed`; never enqueued | ☐ |
| TC-MEDIA-006 | Unknown magic bytes rejected | Negative | P0 | `/complete` on object whose magic matches no accepted type | 422 `media/unsupported-format`; object deleted; asset `failed` | ☐ |
| TC-MEDIA-007 | Oversize > 50 MB via HEAD re-check | Boundary/Neg | P0 | upload `oversize_51mb.jpg` bypassing presign policy; `/complete` | 422 `media/file-too-large`; object deleted; asset `failed`; never enqueued | ☐ |
| TC-MEDIA-008 | 50 MB boundary accepted | Boundary | P1 | upload exactly ≤50 MB image | accepted, processes normally | ☐ |
| TC-MEDIA-009 | Animated GIF rejected (worker) | Negative | P0 | upload `animated.gif`; `/complete` (passes sniff) | worker ffprobe detects frame>1 → `status=failed`, `error_message` names animated input; worker does **not** crash; queue continues | ☐ |
| TC-MEDIA-010 | Area over budget rejected | Boundary/Neg | P0 | upload `8001x8000.jpg` (> 64 MP) | `status=failed`, human-readable `error_message`; no crash; queue continues | ☐ |
| TC-MEDIA-011 | EXIF orientation baked into variants | Functional | P0 | upload `orientation6_gps.jpg`; after ready, fetch both variants | both display **upright**; `exiftool` shows **no** metadata on variants | ☐ |
| TC-MEDIA-012 | Original preserved byte-identical (EXIF+GPS intact) | Data-integrity | P0(S1) | download original of TC-MEDIA-011 asset | checksum == uploaded; `exiftool` shows full EXIF incl GPS retained on original | ☐ |
| TC-MEDIA-013 | No upscaling | Boundary | P0 | upload 800 px-wide image | `medium`=800 px (not upscaled), `thumb`=320 px | ☐ |
| TC-MEDIA-014 | Corrupt file → failed, no hang | Negative | P0 | upload `corrupt.bin` (valid sniff, undecodable) | `status=failed` w/ reason; never stuck in `processing` | ☐ |
| TC-MEDIA-015 | Image/heavy concurrency caps | Reliability | P0(S1) | enqueue 5 image tasks + 2 transcodes simultaneously | ≤ `IMAGE_CONCURRENCY` image tasks and 1 transcode run at once (assert via Asynq inspector / worker logs); no OOM | ☐ [MANUAL] |
| TC-MEDIA-016 | Variant serving + cache headers | Functional | P0 | GET `/assets/{id}/variants/thumb` (unauthenticated) | 200 WebP content-type + `Cache-Control: private, max-age=600` (`public, max-age=86400` for a public asset), no `immutable`; served without auth | ☐ |
| TC-MEDIA-017 | Variant 404 for missing/deleting asset | Negative | P0 | GET variant of deleted/nonexistent id | 404 `media/asset-not-found` | ☐ |
| TC-MEDIA-018 | Variant enum validation | Boundary | P1 | GET `/variants/foo` (not in thumb\|medium\|poster) | 4xx (not 500) | ☐ |
| TC-MEDIA-019 | Lifecycle has no `uploaded` state | Functional | P1 | observe status transitions | only `uploading→processing→ready\|failed\|deleting`; never `uploaded` | ☐ |
| TC-MEDIA-020 | Tall webtoon strip admitted | Boundary | P0 | upload `webtoon_704x18000.jpg` | reaches `ready`; `medium` ≤ 16,000 px tall | ☐ |

## P0.2 — Video poster thumbnail

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-030 | Video gets poster on transcode | Functional | P0 | upload a normal video; wait for `ready` + one thumbnail-task cycle | `media:thumbnail` enqueued after `ready`; `poster` variant row exists; renders in library grid + as Vidstack poster | ☐ |
| TC-MEDIA-031 | Short 2 s video poster (seek clamp) | Boundary | P0 | upload `short_2s.mp4` | poster extraction succeeds (seek clamped inside file) | ☐ |
| TC-MEDIA-032 | Audio-only container → skip poster | Negative | P0 | upload `audio_only.mp4` (0 video streams) | poster step **skipped with warning**; no crash; asset still reaches `ready` | ☐ |
| TC-MEDIA-033 | Poster failure non-fatal | Reliability | P0 | induce poster-gen failure (e.g. unreadable frame) | video stays `ready`; poster absent; warning logged; asset NOT failed | ☐ |

## P0.3 — Delete asset + janitor

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-040 | Delete ready video removes all objects | Functional | P0(S1) | DELETE `/assets/{id}` on ready video | HLS URL, variant URLs, original-download all 404 from the API origin (cache bypassed); `mc ls` shows **no** objects under asset prefix; DB rows gone | ☐ |
| TC-MEDIA-041 | Delete is idempotent | Idempotency | P0 | DELETE same id twice | 2nd call → 404 (never 500) | ☐ (CC-8) |
| TC-MEDIA-042 | Cross-owner delete blocked | AuthZ | P0(S1) | userB DELETE userA's asset (no `assets:delete:any`, not `*`) | 403 (not 404); nothing deleted | ☐ (CC-3) |
| TC-MEDIA-043 | Admin `assets:delete:any` allowed | AuthZ | P1 | admin DELETE userA's asset | 204; deleted | ☐ |
| TC-MEDIA-044 | `deleting` excluded from listings | Functional | P0 | set asset `deleting`; GET `/assets` | asset absent from all list filters while deleting | ☐ |
| TC-MEDIA-045 | Janitor finishes stuck delete | Reliability | P0(S1) | simulate storage outage mid-delete; run janitor after recovery (>15 min grace) | asset finishes deleting; never reappears in listings meanwhile | ☐ [MANUAL] |
| TC-MEDIA-046 | Janitor sweeps abandoned uploads | Reliability | P1 | create upload session, never complete; age >24 h; run janitor | marked `failed` `error_message='upload abandoned'` | ☐ [MANUAL] |
| TC-MEDIA-047 | Stuck purge logs after 5 attempts | Reliability | P1 | force 5 consecutive purge failures for one asset | error-level log emitted (the "stuck" signal) | ☐ [MANUAL] |
| TC-MEDIA-048 | `media:asset_deleted` emitted after row gone | Integration | P0 | delete an asset; observe bus | exactly one `media:asset_deleted {asset_id, owner_user_id}` after row removed; registered in events.md | ☐ (CC-5) |
| TC-MEDIA-049 | Delete survives purge failure | Reliability | P0 | induce storage failure during DELETE; DELETE again while `deleting` | both calls 204; janitor completes the purge after the grace | ☐ |
| TC-MEDIA-050 | Delete while worker in flight | Reliability | P0(S1) | DELETE a `processing` image/video (and one with a poster task queued); let the worker finish | no objects under its prefixes; no variant rows; no `media:asset_ready` | ☐ |
| TC-MEDIA-051 | Janitor on the shared scheduler | Integration | P1 | start `cmd/worker` | `media:purge_orphans` registered `@every 1h` on the single `asynq.Scheduler` | ☐ |

## P0.4 — Library page

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-060 | Grid renders owner assets | Functional | P0 | open `/library/media` as owner with mixed assets | grid shows poster/thumb, title, kind badge, status badge, created date | ☐ |
| TC-MEDIA-061 | Filter kind=image/video/all | Functional | P0 | apply each kind filter | list matches filter | ☐ |
| TC-MEDIA-062 | Filter status; processing includes uploading | Functional | P0 | filter status=processing with a fresh `uploading` asset present | freshly-created `uploading` asset **appears** under processing filter (never invisible under every filter) | ☐ |
| TC-MEDIA-063 | Cursor pagination stable | Functional | P0 | 100+ assets; page via cursor | response `{items, next_cursor?}` (no `assets` key — specs README Pagination retrofit); order `created_at DESC, id DESC`; no dupes/gaps | ☐ (CC-4) |
| TC-MEDIA-064 | LCP < 2.5 s (100 assets) | Performance | P0 | measure first-page LCP; thumb variants only | < 2.5 s; grid uses thumb, never originals | ☐ [MANUAL] |
| TC-MEDIA-065 | Optimistic delete, no full refetch | Frontend | P0 | delete via confirm dialog | card disappears via TanStack cache mutation (no full refetch); rollback on error | ☐ (CC-9) |
| TC-MEDIA-066 | Image lightbox + download original | Frontend | P0 | open image → lightbox | shows `medium`; Download-original action present & works | ☐ |
| TC-MEDIA-067 | Failed asset shows error_message | Functional | P1 | expand a `failed` asset | `error_message` on hover/expand; only delete offered | ☐ |
| TC-MEDIA-068 | Empty state links to /upload | Frontend | P1 | new user, no assets | empty state with link to `/upload` | ☐ |
| TC-MEDIA-069 | List owner-isolation | AuthZ | P0(S1) | userA GET `/assets` | none of userB's assets | ☐ (CC-3) |
| TC-MEDIA-070 | `limit` lenient: default and clamp | Boundary | P0 | GET `/assets` with no `limit`, `limit=abc`, `limit=0`, `limit=500` over 120 assets | 50, 50, 50, then **100** items (clamped to the max, owner decision 2026-10-01); never a Problem | ☐ (CC-4) |

## P0.5 — Download original

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-080 | Owner download byte-identical | Data-integrity | P0(S1) | GET `/assets/{id}/original` as owner | checksum == uploaded; `Content-Disposition: inline; filename="<original>"`; `Accept-Ranges: bytes`; content-type = `assets.mime_type` (sniffed for images); EXIF intact | ☐ |
| TC-MEDIA-081 | Fallback filename when original_filename null | Functional | P1 | asset predating migration | filename falls back to `{asset_id}.{ext}` from sniffed type/source_key | ☐ |
| TC-MEDIA-082 | Cross-owner original blocked | AuthZ | P0(S1) | userB, editor (`assets:read:any`) and `*` superadmin each GET userA's original | 403 (or 404 when RLS hides the row); no bytes streamed | ☐ (CC-3) |
| TC-MEDIA-083 | Original never via public variant scheme | Security | P0(S1) | attempt to reach original through variant/HLS URL pattern | not reachable; only owner-authenticated proxy serves it | ☐ |
| TC-MEDIA-084 | processing/failed original still downloadable | Functional | P0 | asset failed worker-side (object still exists) | original downloadable (archival guarantee for accepted uploads) | ☐ |
| TC-MEDIA-085 | Purged-at-complete original → 404 | Negative | P0 | asset rejected at `/complete` (file-too-large/unsupported, object purged) | 404 `media/asset-not-found` | ☐ |
| TC-MEDIA-086 | Still-uploading original → 409 | Negative | P0 | GET original while `uploading` | 409 `media/asset-not-ready` | ☐ |
| TC-MEDIA-087 | Abandoned-upload original → 404 | Negative | P0 | GET original of an asset `failed` with `upload abandoned` | 404 `media/asset-not-found`; no bytes streamed | ☐ |

## P0.6 — Event fan-out prerequisite

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-090 | `platform/events` publish→consume | Integration | P0(S1) | publish `media:asset_deleted`; observe registered consumers (comic reap + stream removal) | each subscribed consumer gets its **own** Asynq task; both fire | ☐ (CC-5) |
| TC-MEDIA-091 | Emitting-binary registers edges (regression) | Integration | P0(S1) | emit an api-side event (delete) | api's publisher has the subscription edge; task actually enqueued (guards the "empty routing table" bug) | ☐ (CC-5) |
| TC-MEDIA-092 | Publish only after commit | Integration | P0 | roll back a delete tx | no event emitted | ☐ (CC-5) |
| TC-MEDIA-093 | Unregistered event name never enqueued as task | Reliability | P1 | inspect worker | raw event name is never a task type; one handler per task type (no ServeMux panic) | ☐ |
| TC-MEDIA-094 | Publish fan-out cardinality | Integration | P0 | unit: two subscriptions; none; failing enqueuer | one task per subscriber; nil + nothing enqueued; enqueue error propagated | ☐ |

## P1 — nice to have

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-100 | PATCH title (inline rename) | Functional | P1 | PATCH `/assets/{id}`{title} as owner; as another `user`; with `""`; with `null` | owner: title updated, library reflects; other user: 403; `""`: 422; `null`: reset to `original_filename` | ☐ [P1] |
| TC-MEDIA-101 | `media:asset_ready` emitted on ready | Integration | P1 | asset reaches ready | one `media:asset_ready {asset_id,kind,owner_user_id,title,origin}`; title falls back to original_filename | ☐ [P1] |
| TC-MEDIA-102 | Import-origin flood suppression flag | Integration | P1 | `mediaapi.Ingest(..., origin="import")` (P1.3) | payload carries `origin='import'`; consumers can suppress | ☐ [P1] |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-MEDIA-110 | All non-2xx are RFC-7807 | Contract | P0 | exercise each error path | `application/problem+json` + stable `type` URI; no bare 500 | ☐ (CC-1) |
| TC-MEDIA-111 | Every Problem type has i18n key | Contract | P1 | grep `frontend/src/lib/problems.ts` | each media `type` URI present | ☐ (CC-1) |
| TC-MEDIA-112 | Handler↔OpenAPI drift | Contract | P1 | compare `shared/openapi.yaml` media paths vs live routes | paths/params/perms match; note deviations | ☐ (CC-7) |
| TC-MEDIA-113 | Unauthenticated → 401 | AuthZ | P0 | guest hits authed media endpoints | 401 | ☐ |
| TC-MEDIA-114 | Migration up/down clean | Contract | P1 | `make migrate` then `make migrate-down` for media_variants | applies + rolls back cleanly; `assets_status_chk` includes `deleting`; indexes present | ☐ |
