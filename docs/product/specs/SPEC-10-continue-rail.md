# SPEC-10 — Playback Resume + Continue Rail (D-20 execution)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `media`; aggregator mounts in `cmd/api` · **Depends on:** nothing hard for P0; soft on SPEC-04 P0 (`assets.title`/`original_filename`, the `deleting` status, the `poster` variant); SPEC-04 P0.6 (`platform/events` fan-out) for P1.5's event; the comic leg (P1.6) plugs in with SPEC-14's `comic_reading_progress`
**Upstream:** brief 07 (folded into this spec, then deleted — `git show ea100d8:docs/product/briefs/07-continue-rail.md`) · **Refs:** feature-inventory `D-20` (continue aggregator shape), SPEC-14 P0.4 (progress-beacon convention), frontend.md
**Downstream consumers:** SPEC-09 P0.4 (continue widget); SPEC-05 is a *future* consumer of P1.5's event (needs a `notify:on_*` task first)

---

## 1. Problem statement

Portal plays HLS video end to end, but every session restarts at 0:00. The gap
between "works in a demo" and "used nightly" is exactly
resume-where-you-left-off — Jellyfin and Plex prove it is *the* retention surface
for a personal media server. `D-20` already decided the aggregator fan-out
(`GET /continue` fanning out to `<module>api.Continue`); this spec revises
D-20's item schema (P0.3). With two content
surfaces imminent (video now, SPEC-14 comics with its own progress table), the
aggregator is at its cheapest right now.

## 2. Goals

1. Reopening a video or audio asset resumes within ±10 s of where playback
   stopped — across devices (same account).
2. One `GET /api/v1/continue` returns the cross-module in-progress list — video
   and audio today; comics join via P1.6 once SPEC-14 P0.4 exists, with **no
   response-shape changes** at that point.
3. ≥95%-watched emits the first "watched X" life-stream fact (P1.5).

## 3. Non-goals

- **Movie catalog/metadata CRUD** — the movie vertical is its own spec,
  [SPEC-16](SPEC-16-movie-vertical.md) (backend built, `0021_movie_core`; its
  frontend is SPEC-16 P1.1). This spec hangs off existing media `assets` rows
  only: a movie resumes through its video asset (SPEC-16 P0.6), and a `movie`
  leg of `/continue` is SPEC-16 P1.2.
- **Cross-device conflict resolution** beyond last-write-wins on the upsert.
- **Per-position bus events** — beacons are deliberately *not* events (too
  chatty); only the P1.5 completion fact hits the bus.
- Watch-history UI — the table will hold the data (P2); no page at v1.

## 4. User stories

- As the owner, I stop a movie at minute 43 on the desktop and resume at minute
  43 on my phone.
- As the owner, the home rail shows the things I'm mid-way through, with progress
  bars, and clicking one drops me back in.
- As the owner, a video I finished disappears from the continue rail instead of
  cluttering it at 99%.
- Edge: as the owner, a 20-second accidental click never pollutes my continue
  rail.

## 5. Requirements

### P0.1 — Progress table

`media_playback_progress` (§6) — media-module migration, next free number.
PK `(user_id, asset_id)`; `position_ms`; percent-complete **derives** from
`assets.duration_ms` *(verified 2026-07-10: the column exists in migration
0007 — and note the table is named `assets` — but is **nullable**)*.

**NULL-duration rule** *(2026-07-10 — previously undefined and silently
assumed non-NULL)*: for an asset with `duration_ms IS NULL` (failed/older
probe), the beacon still upserts (clamped only to `≥ 0`), resume still works
from the saved position, but `progress_pct` is undefined — the item is
**excluded from `/continue`** (its predicate needs a percentage) and the P1.5
completion event is never emitted for it. Backfilling `duration_ms` for such
assets via re-probe is a janitor-level nicety, not P0.

**Completion ratio** *(binding on P0.2, P0.3, P0.4 and P1.5)*: the completion
ratio is the integer `floor(position_ms * 100 / duration_ms)`. `progress_pct`
in every response, the `/continue` predicate, the P0.4 resume gate and the P1.5
latch all use this one value, so an item leaves `/continue` exactly when P1.5
fires. `duration_ms <= 0` is treated as NULL (the NULL-duration rule applies).

**Display title** *(binding on P0.3 and P1.5)*: the first non-empty of
`assets.title`, `assets.original_filename`, `'Untitled video'`. Never derive a
title from `assets.source_key`: it is always `uploads/{id}/original{ext}`, so
every item would read "original.mp4".

**Playable kinds** *(owner decision 2026-09-30; binding on P0.2–P0.4 and
P1.5)*: `video` and `audio` assets are playable; every other kind (`image`,
documents) is not. A video plays from its HLS output
(`/api/v1/assets/{id}/hls/index.m3u8`) and may have a `poster` variant. An
audio asset has neither: it is marked `ready` at `/complete` with no transcode
step and plays from its stored original, `GET /api/v1/assets/{id}/original`
(SPEC-04 P0.5 — owner-authenticated, widened for `shared` assets by SPEC-04
P0.8, `inline`, Range-capable via
`http.ServeContent`, so seeking and resume work). Audio resumes, saves
progress and completes exactly like video. Its `duration_ms` is probed from the
original at `/complete` (the same ffprobe call the video poster step uses), so
audio has a completion ratio and appears in `/continue`; an audio asset whose
probe failed falls under the NULL-duration rule like any video. *(Code
follow-up: HEAD's `completeAudio` stores no `duration_ms`, so every audio
asset is NULL-duration — it resumes but never reaches `/continue` or P1.5.)*

### P0.2 — Beacon

`PUT /api/v1/assets/{id}/progress {position_ms}` — **authenticated
(`RequireAuth`), caller-scoped by construction**: the row is keyed by the
caller's own user id, exactly like SPEC-14 P0.4's comic progress writes
*(2026-07-10 — the drafted `media:progress:own` permission repeated the
module-prefix-as-resource pattern and added a code with nothing extra to
protect; dropped for parity with the existing progress convention)*. Upsert,
last-write-wins: `INSERT … ON CONFLICT (user_id, asset_id) DO UPDATE SET
position_ms = EXCLUDED.position_ms, updated_at = now()` — the rail's
`updated_at DESC` order depends on it (specs README updated_at convention). Server clamps `position_ms` into `[0, duration_ms]` (lower
bound only when duration is NULL — P0.1). **Who may keep a row** *(Decision
2026-10-02b (B13), audience revised by B14 and B15 —
[ADR-12](SPEC-01-account-identity-admin.md#adr-12); unbuilt — §11 row 15)*:
anyone SPEC-04 P0.8 lets play the asset — its owner, or, while the asset is
`shared` (a published track's audio, a published movie's video), a User who
shares a group with the owner in its tenant (SPEC-01 P0.18, B15) or the
owner's friend in the same or an actively linked tenant. A tenant admin
as such is not admitted. Each keeps their **own** row, `(user_id, asset_id)`
with their own id; nobody reads or writes another user's row, and the owner's
position is never shared. **The row lives in the reader's own tenant**: it is
written in the request's scope, whose `app.current_tenant` is the reader's
personal organisation, so a friend's row on an asset in another tenant
carries the friend's `tenant_id`, not the asset's, and `media_playback_progress`'s
`tenant_isolation` policy is satisfied without any exception — only the read
of the asset row crosses the fence, through SPEC-04 P0.8. If tenant selection
lands (`D-23`), the progress routes keep writing into the reader's personal
organisation rather than the active tenant, because the primary key is
`(user_id, asset_id)` across tenants and a second row cannot exist. When the
reader loses access (unpublish, unfriend, unlink), the row is kept but
unreachable: the routes answer 404 and `/continue`'s join to `assets` drops
it, until access returns.
"Owned" in the table below means "playable to the caller" in that sense.
Asset-level failures follow this table, which applies to both the PUT and the
GET:

| Asset state | Status | Problem type |
|---|---|---|
| Unknown id, malformed id, `deleting`, or another user's asset the caller may not play (SPEC-04 P0.8: not `shared`, or the caller outside its ADR-12 audience; no role, permission or tenant-admin bypass) | 404 | `media/asset-not-found` |
| Owned, kind neither `video` nor `audio` (an image or document) | 404 | `media/asset-not-playable` (404 keeps existence hidden beyond ownership) |
| Owned video or audio in `uploading`, `processing` or `failed` (nothing playable yet) | 409 | `media/asset-not-ready` |

Until SPEC-04 P0 ships the `deleting` status, that clause is a no-op (no asset
can be in that state yet). No row is written on any non-2xx.
*(Code follow-up: on HEAD the handler maps `ErrForbidden` to a 403, `GetProgress`
maps `deleting` to `media/asset-not-playable`, and neither checks the asset
status; all three change to match this table. Both `PutProgress` and
`GetProgress` also reject every non-video kind (`asset.Kind != "video"` →
`ErrNotPlayable`), so audio answers 404 `media/asset-not-playable`; they
accept `audio` too.)*

`GET /api/v1/assets/{id}/progress` — same auth/caller-scoped construction as
the PUT above, returns `{position_ms, progress_pct (null when duration_ms IS
NULL or ≤ 0), completed_at, updated_at}` for the caller's own row, failing per
the table above. When the asset passes every check but the caller has no row
(first open), it returns 200 `{position_ms: 0, progress_pct: 0 (null when
duration_ms IS NULL or ≤ 0), completed_at: null, updated_at: null}`. 404 is
reserved for asset-level failures, so the player never has to treat a 404 as
"start from 0". This is the read path P0.4 fetches before initializing
Vidstack — `/continue`'s `progress_pct` is an integer percentage (P0.1), not
precise enough to seek by. *(Code follow-up: on HEAD a missing row surfaces as
404 `media/asset-not-found`, and `shared/openapi.yaml` `PlaybackProgress`
still requires `updated_at`; it becomes `required: [position_ms]` with
`updated_at` and `progress_pct` nullable.)*

Client: throttled from the Vidstack player — every ~10 s while position advances,
plus on pause and `pagehide` — mirroring SPEC-14 P0.4's convention.
**Fire-and-forget**: save failures never surface to the player or block
playback.

**Transport rule** *(binding on every progress save, SPEC-14 P0.4 included)*:
all saves, including `pagehide`, use
`fetch(url, {method: 'PUT', keepalive: true, credentials: 'include', headers: {'Content-Type': 'application/json'}, body})`.
Never use `navigator.sendBeacon`: it can only POST, and this route is
PUT-only, so every beacon save is a 405 that fire-and-forget hides (and
`sendBeacon` returns `true` once queued, so a keepalive fallback behind it
never runs). The API is on another origin, so the `pagehide` save is a
best-effort preflighted CORS request; the ~10 s and on-pause saves are the
durable path. Do not relax the handler to accept `text/plain`. A `pagehide`
save fired after the access token has expired is dropped (nothing refreshes
the cookie on unload); acceptable because the ~10 s/on-pause cadence bounds
the lost window to a few seconds. No CSRF concern: cookies are
`SameSite=Strict`, so a request from a third-party page never carries them.
*(Code follow-up: the shipped `MediaDetailView.tsx` and `lib/comic.ts` still
call `navigator.sendBeacon` first; both switch to the keepalive `PUT`.)*

**Acceptance criteria.**
- Given playback to 12:34 then tab close, when the asset is reopened, then the
  player starts at ~12:34 (±10 s) with a visible "Start over" control.
- Given a beacon for another user's asset (regardless of the caller's role or
  permission grants — there is no permission-based bypass), then 404
  `media/asset-not-found` (never 403) and no row.
- Given another user's asset, when GET progress is called, then 404
  `media/asset-not-found` (never 403) — unless SPEC-04 P0.8 lets the caller
  play it.
- Given owner C and M who shares a group with C in C's tenant, and C's `shared` video (a
  published movie), when M PUTs `position_ms` 600 000 and GETs it back, then
  both answer 200, M's row holds 600 000, and C's GET still returns C's own
  position; given the asset back to `private`, then M's PUT and GET answer 404
  and M's row is kept. *(TC-CONT-103)*
- Given C's `shared` audio and C's friend F in an actively linked tenant, when
  F PUTs a position, then F's row carries F's own `tenant_id` and appears in
  F's `/continue`; when C and F disconnect or the link drops, then F's PUT and
  GET answer 404, the item leaves F's `/continue`, and the row is kept.
  *(TC-CONT-104)*
- Given an owned image asset, when PUT or GET progress is called, then 404
  `media/asset-not-playable`.
- Given an owned `ready` audio asset, when PUT progress is called with
  `position_ms` 90 000 and GET progress follows, then both answer 200 and GET
  returns `position_ms` 90 000.
- Given an owned video still `processing`, when PUT or GET progress is called,
  then 409 `media/asset-not-ready` and no row.
- Given a playable owned video never played, when GET progress is called, then
  200 with `position_ms` 0 and `updated_at` null, and playback starts at 0.
- Given `duration_ms IS NULL` and a saved row, when GET progress is called,
  then `progress_pct` is null and `position_ms` is returned.
- Given `position_ms` beyond the asset duration, then it is clamped, never 500.
- Given the API briefly down, then playback continues unaffected (no error UI).
- Given a `pagehide` save against the cross-origin API, then the API logs a
  `PUT` 2xx, never a `POST` 405.

### P0.3 — Aggregator `GET /api/v1/continue`

D-20 decided the fan-out; the item schema is revised here (D-20's
`position`/`duration` become `progress_pct`; the exact seek position comes from
GET progress, P0.2). Mounted in **`cmd/api`** (resolved from the brief: the aggregator is
cross-module by definition and the wiring layer is the documented home for such
composition — the second sanctioned instance after the `Engine()` exception; it
calls only module `api/` packages, so the boundary rule holds). Fans out to
`mediaapi.Continue(ctx, userID, limit)` today; `comicapi.Continue` joins in
P1.6 — the fan-out signature is the seam, and the response shape is
module-agnostic from day one:

```
{items: [{module, ref_id, title, poster_url, progress_pct, href, updated_at}]}
```

The item matches `shared/openapi.yaml` `components.schemas.ContinueItem`:

| Field | Type | Meaning |
|---|---|---|
| `module` | string ∈ {`media`, `movie`, `music`, `story`, `comic`} | `media` today — music tracks included, as items of their audio asset (SPEC-15 P0.10, Decision 2026-10-02b (B2)), so `music` is never emitted; the comic leg (P1.6) uses `comic` |
| `ref_id` | uuid | the asset id for `media`; the comic id for `comic` |
| `title` | string, non-null | the display title (P0.1); the comic leg uses the comic title |
| `poster_url` | string \| null | `/api/v1/assets/{ref_id}/variants/poster` only when a `poster` variant exists, else null; the comic leg uses its cover's `thumb` variant |
| `progress_pct` | integer 0–100 | the completion ratio (P0.1) |
| `href` | relative app path | `/library/media/{ref_id}` (P0.4 deep-link rule); the comic leg (P1.6) uses its reader path |
| `updated_at` | RFC 3339 timestamp | the progress row's `updated_at` |

The shared Go type lives in a platform package (e.g. `platform/continueitem.Item`),
and every `<module>api.Continue` returns it, so `comicapi` never imports
`mediaapi`. *(Code follow-up: on HEAD the type is `mediaapi.ContinueItem`,
`poster_url` is always set even when no poster exists, the title falls back
to `source_key`, and `progress_pct` is cast with `::int`, which rounds instead
of flooring.)*

`?limit=` is an integer, default 10. Values above 50 are clamped to 50; a
missing, non-integer or `< 1` value falls back to the default 10 (never an
error). Each module's `Continue` is called with `limit`; the aggregator merges
the lists, sorts by `updated_at DESC`, and truncates to `limit`. Requires
authentication only — each module call is owner-scoped by construction, so no
separate permission code is needed beyond `RequireAuth`.

**Inclusion predicate** (applied module-side, in `mediaapi.Continue`): an item is
"in progress" iff `position_ms ≥ 30 000` **and** the completion ratio (P0.1)
is `< 95` (resolves
the brief's threshold question: accidental clicks below 30 s never appear; the
completed drop-off is a P0 predicate, not a P1 afterthought).

**Acceptance criteria.**
- Given only media wired, then the endpoint returns video and audio items and
  no errors; an audio item's `poster_url` is null.
- Given items at 20 s watched and at 97% watched, then neither appears.
- Given 12 in-progress items and no `?limit=`, then 10 items sorted by
  `updated_at DESC`.
- Given `?limit=100`, then at most 50 items; given `?limit=abc` or
  `?limit=0`, then 200 with the default 10.
- Given an asset with no title and no `original_filename`, then its item's
  `title` is "Untitled video", never "original.mp4".
- Given comicapi joining later, then the response shape is unchanged (contract
  test on the item schema — no video special-casing).
- Given a saved position of 10:00 on an asset with `duration_ms IS NULL`, then
  the item does not appear in `/continue`, reopening still starts at ~10:00,
  and no `media:playback_completed` event is ever emitted for it.

### P0.4 — Resume UX

**Playback host:** the per-asset player lives at
`app/(app)/library/media/[id]/page.tsx` (an "RSC shell", specs README
Frontend), resolving `TemplateManifest.views.libraryMediaDetail`
(`ComponentType<{ id: string }>`; v1 → `views/library/media/MediaDetailView.tsx`),
which mounts Vidstack for both playable kinds: a video with its HLS source and
video layout, an audio asset with `src` = `/api/v1/assets/{id}/original`
(typed by `assets.mime_type`) and the audio layout — no poster, no HLS. Any
other kind renders the not-playable state. This spec creates the route, adds the key to
`templates/types.ts` and the v1 manifest, and adds it to frontend.md §2's
`views` list (all shipped on HEAD; frontend.md lists the key since the
2026-09-30 docs pass).
`mediaapi.Continue` (P0.3) builds each item's `href` to this exact path,
`/library/media/{id}`.

**Media deep-link rule** *(binding on every spec that links to a media asset;
SPEC-05 P0.4 and SPEC-09 P0.2 cite it)*: a **video or audio** asset links to
`/library/media/{id}` (this detail player); an **image** asset links to
`/library/media?open={id}`. SPEC-04 P0.4's grid reads `open` and opens its
lightbox for that id, or shows a not-found toast. The player route accepts
video and audio and answers `media/asset-not-playable` for every other kind
(P0.2), so no producer may send an image there. Every producer
or consumer that builds a media href uses this kind-aware rule: notify's
`media.asset_ready` `data.href`, the stream render mapping, and continue items.
*(Code follow-up: on HEAD the three have diverged — `media_progress.sql` builds
`/library/media/{id}`, `journal/stream.go` builds `/library/media`, and notify
builds `/library/{id}`; the grid does not read `open` yet. `MediaDetailView`
feeds audio the empty `hls_url` (the server builds it for video only) under
`DefaultVideoLayout`, so an audio asset opened here never plays; it switches
to the `/original` source and the audio layout.)*

Before initializing Vidstack, the player fetches `GET
/api/v1/assets/{id}/progress` (P0.2) to obtain the exact saved `position_ms` —
`/continue`'s `progress_pct` is an integer percentage, not enough to seek by.
Vidstack starts at the saved position when it is at or above 30 s
(`position_ms ≥ 30 000`) and the completion ratio (P0.1) is below 95 — the percent gate is skipped when `progress_pct` is undefined (NULL
`duration_ms`, P0.1), in which case any saved position ≥ 30 s resumes, with a
visible "Start over" affordance; otherwise starts at 0.

**Acceptance criteria.**
- Given saved progress at 12:34, then playback opens at 12:34 and "Start over"
  restarts at 0 (and the next beacon overwrites the old position).
- Given progress at 97%, then playback starts at 0 (finished content replays).
- Given an in-progress item on `/continue`, when its `href` is opened, then
  `/library/media/{id}` mounts the player at the saved position — for an audio
  item too, playing from `/original`.
- Given an audio asset's `media.asset_ready` notification, when it is opened,
  then it lands on `/library/media/{id}`, never the image lightbox.

### P1 — nice to have

- **P1.5 Completion event**: `media:playback_completed`
  `{asset_id, user_id, title, completed_at}` (`title` is the display title,
  P0.1; `completed_at` is the latched value returned by step 2's UPDATE —
  `RETURNING completed_at` — so the stream card's `occurred_at` is the moment
  of completion, not the consumer's processing time) emitted
  **once** when the completion ratio (P0.1) first reaches ≥ 95
  (undefined-duration assets never emit — P0.1) — latched via a nullable
  `completed_at` on the progress row (emit only on the NULL→set transition, so
  repeat crossings never re-emit). The beacon runs in **one transaction**:
  (1) upsert the position (P0.2's `INSERT … ON CONFLICT (user_id, asset_id) DO
  UPDATE SET position_ms = EXCLUDED.position_ms, updated_at = now()`); (2) if
  the ratio is ≥ 95, run `UPDATE media_playback_progress SET completed_at =
  now() WHERE user_id = $1 AND asset_id = $2 AND completed_at IS NULL RETURNING
  completed_at` — the row lock serialises concurrent beacons, so only one gets a row back;
  (3) only when step 2 returned a row, call `events.Publish` before COMMIT; if
  Publish returns an error, roll back (the position save is lost too, and the
  next beacon retries both). Never discard the Publish error. Because Publish
  enqueues per subscriber and is not atomic, a retry or a crash between Publish
  and COMMIT may deliver the event twice; consumers dedupe on
  (`asset_id`, `user_id`), which the payload already carries. *(Code follow-up:
  on HEAD `PutProgress` reads `completed_at`, publishes outside any
  transaction, discards the Publish error with `_ =`, then upserts — a failed
  Publish loses the event for good and two concurrent beacons can both emit.)*
  Published via
  `platform/events` (events.md "Delivery mechanics"; v1 consumer: stream only —
  SPEC-09 maps this event to a "Watched <title>" card; notify attaches only when
  a `notify:on_*` task for it is specced in SPEC-05 §7, since subscribing a
  task type with no handler makes Asynq fail every such task).
- **P1.6 Comic leg** — if SPEC-14 P0.4 (`comic_reading_progress`) has shipped,
  implement `comicapi.Continue(ctx, userID, limit)` returning the P0.3 item
  (`module='comic'`,
  `href=/library/comic/{comic_id}/read/{chapter_id}?page={page_position}`) and
  add it to the aggregator fan-out in `cmd/api` `handleContinue` (½ day). Not
  built on HEAD: `handleContinue` fans out to media only.

### P2 — future considerations (design for, don't build)

- **Watched-history page** — the table holds only the first completion per
  asset (`completed_at` is set once; `updated_at` moves with every beacon), which
  is enough for a "watched once" list. Per-watch history, including rewatches,
  needs an append-only `media_playback_completions` table later; UI later.

## 6. Data model — migration `000N_media_playback_progress`

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `media_playback_progress` (`tenant_id` + index + `tenant_isolation` policy, added by `0020_platform_rls_enable`). The DDL below predates ADR-07 and omits the columns.

```sql
CREATE TABLE media_playback_progress (
  user_id      uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
                 -- identity-anchor exception (SPEC-05 §6 precedent)
  asset_id     uuid NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
                 -- the media table is named `assets` (migration 0007)
                 -- same-module FK: progress dies with the asset
  position_ms  bigint NOT NULL CHECK (position_ms >= 0),
  completed_at timestamptz,   -- P1.5 latch: set once at first ≥95% crossing
  updated_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, asset_id)
);
CREATE INDEX ON media_playback_progress (user_id, updated_at DESC);
```

Queries in `query/media_progress.sql`; regenerate via `make sqlc`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| PUT | `/api/v1/assets/{id}/progress` | authenticated (`RequireAuth`; caller-keyed upsert on an asset the caller may play — P0.2, SPEC-04 P0.8) | `{position_ms}`; fire-and-forget client |
| GET | `/api/v1/assets/{id}/progress` | authenticated (`RequireAuth`; the caller's own row on an asset the caller may play — P0.2, SPEC-04 P0.8) | `{position_ms, progress_pct (null if no duration), completed_at, updated_at}`; same status table as PUT (P0.2); 200 with `position_ms` 0 / `updated_at` null when no row exists |
| GET | `/api/v1/continue?limit=` | authenticated (`RequireAuth`) | cmd/api aggregator; module-agnostic items; `limit` default 10, max 50, invalid → default (P0.3) |

Problem types: `media/asset-not-found`, `media/asset-not-ready` (both declared in SPEC-04 §7), `media/asset-not-playable` (new here; register it in `problems.ts` per the specs README Errors DoD — *code follow-up: missing from `problems.ts` on HEAD*).

`/continue` is a non-paginated list and returns `{items}` (specs README Pagination). Annotate each operation per the specs README AuthZ **OpenAPI encoding**. The three rows are authenticated-only: `security: [{bearerAuth: []}]`, no `x-required-permission`.

## 8. Success metrics (n=1 honest)

- Leading: resume offered on reopen within ±10 s of true position, verified
  cross-device (desktop → phone) during the first dogfood week.
- Leading: zero player jank or errors attributable to beacons (fire-and-forget
  verified under an API outage).
- Lagging: the owner actually resumes (≥ 3 resumes/week once a multi-session
  video exists) — the retention claim this spec is built on.

## 9. Timeline & phasing

1. Migration + queries + `mediaapi.Continue` (½ day)
2. PUT + GET progress + `/library/media/[id]` route + `libraryMediaDetail`
   view + Vidstack throttle/pagehide wiring + resume UX (2 days)
3. `cmd/api` aggregator + OpenAPI + contract test (1 day)
4. P1.5 completion latch + event (½ day)
5. P1.6 comic leg — `comicapi.Continue` + one fan-out line (½ day)
P0 ≈ 3.5 dev-days; P1 adds ~1 (P1.5 ½ + P1.6 ½). The brief's ~4 omitted the
GET read path and the player route.

## 10. Open questions

- **(resolved)** Aggregator home: `cmd/api` composition (P0.3).
- **(resolved)** Resume threshold: ignore < 30 s and ≥ 95% (P0.3/P0.4).
- **(resolved 2026-07-10)** Duration column: `assets.duration_ms` exists
  (migration 0007) but is nullable — handled by P0.1's NULL-duration rule.
- **(product, non-blocking)** Should "Start over" also clear the progress row, or
  just play from 0 and let the next beacon overwrite? Recommendation: the latter
  (simpler; identical outcome after ~10 s of playback).

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top of it change no
code). The spec text above is the target; this section lists every place the
shipped code still diverges from it, so an implementer needs nothing but this
spec. Rows are ordered by severity: data loss first (a save or an event that is
silently dropped), then wrong answers on the wire, then shape and wiring, then
unbuilt P1; row 15 (added by Decision 2026-10-02b (B13), rewritten in place by
B14) is appended rather
than renumbering rows other documents cite. A row closes when the code matches the requirement it cites and the
SPEC-10 row of [TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md)
is regraded on a named test. File paths are relative to the repo root;
"F-ids" refer to [spec-gap-fix-worklog-2026-09-30.md](../analysis/spec-gap-fix-worklog-2026-09-30.md).

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.2 transport rule | Every save, `pagehide` included, is `fetch(url, {method: 'PUT', keepalive: true, credentials: 'include', headers: {'Content-Type': 'application/json'}, body})`; never `navigator.sendBeacon`. | `frontend/src/templates/v1/views/library/media/MediaDetailView.tsx` `sendProgressBeacon` (fired on `pagehide` and `visibilitychange`) calls `navigator.sendBeacon(url, Blob)` whenever it exists, which is a `POST` to a PUT-only route (`media/module.go` registers `r.Put("/{id}/progress")` only) → 405, hidden by fire-and-forget; the keepalive branch never runs because `sendBeacon` exists in every current browser. **The last position before the tab closes is lost on every session.** `putProgressFetch` (10 s and on-pause saves) omits `keepalive: true`. The comic reader has the same bug in `frontend/src/lib/comic.ts` (lines 118–121; SPEC-14 P0.4 cites this rule). | **frontend:** replace both helpers with one `saveProgress(assetId, positionMs)` that always sends the keepalive `PUT` above; delete the `sendBeacon` branch; fix `lib/comic.ts` in the same PR. **backend:** none (do not accept `text/plain` or `POST`). **test:** TC-CONT-026. | F001 |
| 2 | P1.5 completion latch | One transaction: upsert position; `UPDATE media_playback_progress SET completed_at = now() WHERE user_id = $1 AND asset_id = $2 AND completed_at IS NULL RETURNING completed_at`; `events.Publish` only when that returned a row, before COMMIT; a Publish error rolls back. | `backend/internal/modules/media/service.go` `PutProgress`: reads `GetPlaybackProgress` first, sets `completedAt = time.Now()` in Go, calls `s.events.Publish` **outside any transaction and discards its error** (`_ = s.events.Publish(...)`), then `UpsertPlaybackProgress` writes `completed_at` via `COALESCE` (`query/media_progress.sql`). A failed Publish still latches `completed_at`, so the event is lost for good; two concurrent beacons both read NULL and both publish. | **backend:** split `UpsertPlaybackProgress` (position + `updated_at = now()` only) from a new sqlc query `LatchPlaybackCompleted :one` (the UPDATE … `RETURNING completed_at` above); run upsert → latch → Publish inside one `pgx.Tx` (tenant scope already open on the request), return the Publish error so the handler answers 500 and the tx rolls back. **test:** TC-CONT-080/081 (Publish error → no row change and a retry emits once; two concurrent beacons → one event). | F165 |
| 3 | P1.5 payload; P0.1 display title | Payload `{asset_id, user_id, title, completed_at}`; `completed_at` is the latched DB value; `title` is the display title (`assets.title` → `original_filename` → `'Untitled video'`). | `PutProgress` publishes `{asset_id, user_id, title}` only; `title` falls back `Title` → `OriginalFilename` and can be `""`. The stream consumer (`backend/internal/modules/journal/stream.go`) therefore stamps the card with processing time. | **backend:** add `completed_at` (RFC 3339, from row 2's `RETURNING`) and apply the display-title rule (empty strings count as missing). events.md already documents the target payload. **test:** extend TC-CONT-080 to assert both fields. SPEC-09 P0.1(b) owns reading `completed_at` as `occurred_at`. | F152, F078 |
| 4 | P0.1 playable kinds (audio duration) | Audio `duration_ms` is probed from the original at `/complete`, so audio has a completion ratio, appears in `/continue` and can fire P1.5. | `service.go` `completeAudio` calls `s.repo.MarkReady(ctx, asset.ID, "", nil, nil, nil)` — no duration; its comment says duration "is reported by the browser". Every audio asset is NULL-duration. The `api` image (`backend/Dockerfile`, stage `api`) has no ffprobe; only the `worker` stage installs `ffmpeg`. | **backend:** probe the stored original and pass `durationMs` to `MarkReady` — either add `ffmpeg` to the `api` stage and reuse the ffprobe call (`worker/transcode.go` `probe`, moved to a shared helper), or keep `/complete` synchronous-ready and enqueue a worker probe task that writes `duration_ms` (the asset is NULL-duration until it lands, which the NULL-duration rule already covers). A failed probe leaves NULL. **test:** extend `TestCompleteUploadAudioReadyWithoutTranscode` to assert the stored duration; TC-CONT-040. | Decision 2026-09-30 (Audio) |
| 5 | P0.2 playable kinds (audio progress) | `video` and `audio` are playable on PUT and GET progress. | `PutProgress` returns `ErrNotPlayable` when `asset.Kind != "video"`; `GetProgress` the same → 404 `media/asset-not-playable` for audio. | **backend:** accept `kind IN ('video','audio')` in both. **test:** TC-CONT-031 (owned ready audio, PUT 90 000 then GET → 200, 90 000). | Decision 2026-09-30 (Audio) |
| 6 | P0.2 status table | Unknown, malformed, `deleting` or foreign → 404 `media/asset-not-found` (never 403); owned non-playable → 404 `media/asset-not-playable`; owned playable in `uploading`/`processing`/`failed` → 409 `media/asset-not-ready`; no row written on any non-2xx. Same table for PUT and GET. | `service.go` `owned()` returns `ErrForbidden` for a foreign asset and `handler.go` `writeProgressProblem` maps it to **403 `about:blank`**; `PutProgress` checks kind before `deleting`, so a deleting image answers not-playable; `GetProgress` maps `deleting` to `ErrNotPlayable`; neither checks `asset.Status`, so a `processing` video saves a row and answers 204/200; `writeProgressProblem` has no `ErrNotReady` case (it would fall to 500). | **backend:** in one guard used by both methods: missing/foreign/`deleting` → `ErrNotFound`; kind not playable → `ErrNotPlayable`; status ≠ `ready` → `ErrNotReady`; map `ErrNotReady` → 409 `media/asset-not-ready` in `writeProgressProblem`; drop the 403 branch. **openapi:** add `409` to `getAssetProgress` and `putAssetProgress`. **test:** TC-CONT-021, TC-CONT-024. | F077 |
| 7 | P0.2 GET first open | Asset passes every check but no row → 200 `{position_ms: 0, progress_pct: 0 (null when duration_ms IS NULL or ≤ 0), completed_at: null, updated_at: null}`; 404 is reserved for asset-level failures. | `GetProgress` returns the repository's `ErrNotFound` for a missing row → handler answers 404 `media/asset-not-found`; the handler always formats `updated_at` (never null). `shared/openapi.yaml` `PlaybackProgress` has `required: [position_ms, updated_at]` and `progress_pct` not nullable; `frontend/src/lib/media-assets.ts` types `updated_at: string`, and the player hides the 404 with `.catch(() => null)`. | **backend:** on no row return the zero value above; emit `updated_at: null`. **openapi:** `PlaybackProgress` → `required: [position_ms]`, `updated_at` and `progress_pct` `nullable: true`. **frontend:** `PlaybackProgress.updated_at: string \| null`, `progress_pct: number \| null`; stop treating 404 as "start at 0". **test:** rewrite `TestGetProgressNoRow`; TC-CONT-030. | F076 |
| 8 | P0.4 playback host (audio) | Audio mounts Vidstack with `src` = `/api/v1/assets/{id}/original` (typed by `mime_type`) and the audio layout; any non-playable kind renders the not-playable state. | `MediaDetailView.tsx` passes `src={asset.hls_url \|\| ""}` and `DefaultVideoLayout` for both kinds; the server builds `hls_url` for video only (`service.go` `hlsURL`), so audio gets an empty source and never plays. Non-playable kinds render an `<img>` of the original instead of a not-playable state. | **frontend:** branch on `asset.kind`: video → HLS + `DefaultVideoLayout`; audio → `{src: baseURL + '/api/v1/assets/' + id + '/original', type: asset.mime_type}` + `DefaultAudioLayout` (no poster); other kinds → not-playable state (and link to `/library/media?open={id}`). **test:** TC-CONT-066, TC-CONT-065 (audio item click-through). | Decision 2026-09-30 (Audio) |
| 9 | P0.4 media deep-link rule | Video/audio → `/library/media/{id}`; image → `/library/media?open={id}`; the grid reads `open` and opens its lightbox or a not-found toast. Applies to notify's `media.asset_ready` `data.href`, the stream mapping and continue items. | `backend/internal/modules/notify/service.go` (`OnAssetReady` intent, line 268) sets `href` to `/library/` + asset id for every kind; `journal/stream.go` maps `media:playback_completed` to `/library/media` (no id); `media_progress.sql` `GetContinueItems` already builds `/library/media/{id}`. `MediaIndexView.tsx` reads only `kind`/`status` from the query string, never `open`. | **backend:** notify builds the href from the payload's `kind` (already carried) with the rule; stream maps `media:playback_completed` to `/library/media/{asset_id}`. **frontend:** `MediaIndexView` reads `open`, opens the lightbox for that id or shows a not-found toast. **test:** TC-CONT-065; SPEC-09 and SPEC-05 own their mapping tests. | F018 |
| 10 | P0.1 display title (continue item) | `title` = first non-empty of `assets.title`, `assets.original_filename`, `'Untitled video'`; never derived from `source_key`. | `media_progress.sql` `GetContinueItems`: `COALESCE(a.title, a.original_filename, a.source_key)` — an untitled asset reads `uploads/{id}/original.mp4`; empty strings are not skipped. | **backend:** `COALESCE(NULLIF(a.title, ''), NULLIF(a.original_filename, ''), 'Untitled video')`; regenerate sqlc. **test:** TC-CONT-046 (DB-backed). | F078 |
| 11 | P0.1 completion ratio | One integer `floor(position_ms * 100 / duration_ms)` drives `progress_pct`, the `/continue` predicate, the resume gate and the latch; `duration_ms ≤ 0` is treated as NULL. | `GetContinueItems` computes `progress_pct` with a float division cast `::int`, which **rounds** (94.6 % is reported as 95 while the item is still listed; the predicate itself compares floats, which is equivalent to the floor). `PutProgress` clamps to `duration_ms` whenever it is non-NULL, so `duration_ms = 0` clamps every position to 0 instead of applying the NULL rule. | **backend:** `progress_pct` = `(mpp.position_ms * 100 / a.duration_ms)::int` on integers with `a.duration_ms > 0` in the WHERE; in `PutProgress` treat `DurationMs == nil` or `≤ 0` alike (clamp to `≥ 0` only, no latch). **test:** TC-CONT-041 with a 94.6 % row (reports 94, listed) and a `duration_ms = 0` row (excluded, position kept). | F161 |
| 12 | P0.3 item shape | Shared Go type in a platform package (e.g. `platform/continueitem.Item`) returned by every `<module>api.Continue`; `poster_url` is null unless a `poster` variant exists (audio is always null). | The type is `mediaapi.ContinueItem` in `backend/internal/modules/media/api/api.go` with `PosterURL string` (never null); `GetContinueItems` builds `/api/v1/assets/{id}/variants/poster` for every row; `cmd/api/main.go` `handleContinue` falls back to `[]mediaapi.ContinueItem{}`. | **backend:** move the struct to `platform/continueitem` (`PosterURL *string`), have `mediaapi.Continue` return it; in SQL `LEFT JOIN media_asset_variants v ON v.asset_id = a.id AND v.variant = 'poster'` and emit the URL only when `v.asset_id IS NOT NULL`. **openapi:** `ContinueItem` already allows null; describe `limit` as clamped/defaulted (its schema's `minimum: 1`/`maximum: 50` reads as a 4xx). **test:** TC-CONT-043 (contract test on the item schema), TC-CONT-040 (audio item `poster_url` null). | F079 |
| 13 | §7 problem types | `media/asset-not-playable` is registered in `frontend/src/lib/problems.ts`. | `problems.ts` declares `media/asset-not-found` and `media/asset-not-ready` but not `media/asset-not-playable`, although `media/handler.go` emits it. | **frontend:** add the slug to the `ProblemType` union and its message. **test:** TC-CONT-100. | F077, F031 |
| 14 | P1.6 comic leg | `comicapi.Continue(ctx, userID, limit)` returns the P0.3 item and joins the `handleContinue` fan-out. | Not built: `handleContinue` calls `mediaMod.API().Continue` only. | **backend:** implement once SPEC-14 P0.4 is on `main`; merge, sort by `updated_at DESC`, truncate in `handleContinue`. **test:** new TEST-CASES row (none exists). | TRACEABILITY-MATRIX SPEC-10 P1.6 (✖) |
| 15 | P0.2 who may keep a row (Decision 2026-10-02b (B13), audience revised by B14 and B15) | A caller SPEC-04 P0.8 lets play the asset — the owner, or the ADR-12 audience of a `shared` asset (a group co-member of the owner in its tenant — B15 — or the owner's friend in the same or an actively linked tenant) — reads and writes their own `(user_id, asset_id)` row, in their own tenant; a tenant admin as such is not admitted; anyone else gets 404 `media/asset-not-found`. | `backend/internal/modules/media/service.go` `PutProgress` and `GetProgress` both start with `owned(ctx, ownerID, assetID)`, which compares `asset.OwnerID` with the caller (the parameter named `ownerID` is the caller) and returns `ErrForbidden` → 403. The table already keys rows by the caller (`0013_media_playback_progress`, PK `(user_id, asset_id)`; `media_playback_progress` RLS is `tenant_isolation` only, and the request writes in the caller's personal organisation), and `GetContinueItems` joins `assets` under the caller's RLS — so the owner check is the only obstacle once SPEC-04 P0.8's policies admit the asset row. | **backend:** the shared guard of row 6 asks SPEC-04 P0.8's playable-read rule (SPEC-04 §11 row 20 — `app_can_read_shared`) instead of `owned()`; every query keeps the caller's id as `user_id` and the request's own tenant. Land with or after row 6 and SPEC-04 §11 row 20. **test:** TC-CONT-103, TC-CONT-104. | Decision 2026-10-02b (B13), B14; ADR-12 |

**Already matching on HEAD.**
- `0013_media_playback_progress` matches §6 (PK `(user_id, asset_id)`,
  `position_ms ≥ 0` CHECK, nullable `completed_at`); `0020_platform_rls_enable`
  adds `tenant_id` and the `tenant_isolation` policy.
- The upsert sets `updated_at = now()` on conflict, so the rail orders by the
  latest save.
- PUT and GET progress sit behind `RequireAuth` and key the row on the
  caller's id; the handler clamps a negative `position_ms` to 0 and the service
  clamps above `duration_ms` (never 500).
- `GET /api/v1/continue` lives in `cmd/api` (`handleContinue`), answers
  `{items}`, defaults `limit` to 10, clamps to 50, falls back to 10 on a
  missing, non-integer or `< 1` value, and the predicate is `position_ms ≥
  30 000`, ratio `< 95`, `duration_ms IS NOT NULL`, ordered `updated_at DESC`.
- The `/library/media/[id]` route, `TemplateManifest.views.libraryMediaDetail`
  and the v1 manifest entry exist; continue items link to `/library/media/{id}`.
- The player fetches GET progress before seeking, resumes at `≥ 30 s` and
  `< 95` (any `≥ 30 s` when `progress_pct` is null), shows "Start over", and
  saves every ~10 s and on pause.
- Repeated ≥ 95 % crossings in sequence emit `media:playback_completed` once.

**Test evidence to add or fix.**
- `backend/internal/modules/media/service_test.go` `TestGetProgressNoRow`
  asserts `ErrNotFound` for a missing row — superseded; rewrite to expect the
  200 zero value (TC-CONT-030).
- `TestPutProgressGuards` asserts `ErrForbidden` for a foreign asset — the
  target is 404 `media/asset-not-found` (TC-CONT-021); add an owned `ready`
  audio (accepted, TC-CONT-031), a `processing` video (`ErrNotReady`, no row,
  TC-CONT-024), a `deleting` image (not found) and `duration_ms = 0`.
- `TestPutProgressCompletionLatch` covers sequential crossings only; add a
  Publish error (rollback, retry emits once), concurrent beacons and the
  `completed_at`/title payload (TC-CONT-080/081).
- `TestContinueItemsPredicate` runs on an in-memory fake, so it cannot see the
  SQL rounding, title or poster bugs; add a DB-backed query test for
  TC-CONT-041/043/046.
- No HTTP-level progress test exists; add one asserting the problem type for
  every row of the P0.2 status table (TC-CONT-024, TC-CONT-100).
- Frontend: TC-CONT-026 (keepalive `PUT`, no `sendBeacon`), TC-CONT-066 (audio
  from `/original`) and TC-CONT-065 have no test file.
- TC-CONT-083 ("Two consumers registered", stream + notify) is stale against
  P1.5 (v1 consumer: stream only); rewrite it to assert stream only.
