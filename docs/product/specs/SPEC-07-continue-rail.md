# SPEC-07 — Playback Resume + Continue Rail (D-20 execution)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-09-30
**Module:** `media`; aggregator mounts in `cmd/api` · **Depends on:** nothing hard for P0; soft on SPEC-01 P0 (`assets.title`/`original_filename`, the `deleting` status, the `poster` variant); SPEC-01 P0.6 (`platform/events` fan-out) for P1.5's event; the comic leg (P1.6) plugs in with SPEC-02's `comic_reading_progress`
**Upstream:** [briefs/07-continue-rail.md](../briefs/07-continue-rail.md) · **Refs:** feature-inventory `D-20` (continue aggregator shape), SPEC-02 P0.4 (progress-beacon convention), frontend.md
**Downstream consumers:** SPEC-06 P0.4 (continue widget); SPEC-04 is a *future* consumer of P1.5's event (needs a `notify:on_*` task first)

---

## 1. Problem statement

Portal plays HLS video end to end, but every session restarts at 0:00. The gap
between "works in a demo" and "used nightly" is exactly
resume-where-you-left-off — Jellyfin and Plex prove it is *the* retention surface
for a personal media server. `D-20` already decided the aggregator fan-out
(`GET /continue` fanning out to `<module>api.Continue`); this spec revises
D-20's item schema (P0.3). With two content
surfaces imminent (video now, SPEC-02 comics with its own progress table), the
aggregator is at its cheapest right now.

## 2. Goals

1. Reopening a video resumes within ±10 s of where playback stopped — across
   devices (same account).
2. One `GET /api/v1/continue` returns the cross-module in-progress list — video
   today; comics join via P1.6 once SPEC-02 P0.4 exists, with **no
   response-shape changes** at that point.
3. ≥95%-watched emits the first "watched X" life-stream fact (P1.5).

## 3. Non-goals

- **Movie catalog/metadata CRUD** — the [briefs/04-deferred.md](../briefs/04-deferred.md)
  movie-vertical row stays parked; this spec hangs off existing media `assets` rows only.
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

### P0.2 — Beacon

`PUT /api/v1/assets/{id}/progress {position_ms}` — **authenticated
(`RequireAuth`), owner-scoped by construction**: the row is keyed by the
caller's own user id, exactly like SPEC-02 P0.4's comic progress writes
*(2026-07-10 — the drafted `media:progress:own` permission repeated the
module-prefix-as-resource pattern and added a code with nothing extra to
protect; dropped for parity with the existing progress convention)*. Upsert,
last-write-wins: `INSERT … ON CONFLICT (user_id, asset_id) DO UPDATE SET
position_ms = EXCLUDED.position_ms, updated_at = now()` — the rail's
`updated_at DESC` order depends on it (specs README updated_at convention). Server clamps `position_ms` into `[0, duration_ms]` (lower
bound only when duration is NULL — P0.1). Asset-level failures follow this
table, which applies to both the PUT and the GET:

| Asset state | Status | Problem type |
|---|---|---|
| Unknown id, malformed id, `deleting`, or owned by another user (asset visibility is owner-only at v1; no role or permission bypass) | 404 | `media/asset-not-found` |
| Owned, not a video | 404 | `media/asset-not-playable` (404 keeps existence hidden beyond ownership) |
| Owned video in `uploading`, `processing` or `failed` (no HLS output to resume) | 409 | `media/asset-not-ready` |

Until SPEC-01 P0 ships the `deleting` status, that clause is a no-op (no asset
can be in that state yet). No row is written on any non-2xx.
*(Code follow-up: on HEAD the handler maps `ErrForbidden` to a 403, `GetProgress`
maps `deleting` to `media/asset-not-playable`, and neither checks the asset
status; all three change to match this table.)*

`GET /api/v1/assets/{id}/progress` — same auth/owner-scoped construction as
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
plus on pause and `pagehide` — mirroring SPEC-02 P0.4's convention.
**Fire-and-forget**: save failures never surface to the player or block
playback.

**Transport rule** *(binding on every progress save, SPEC-02 P0.4 included)*:
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
  `media/asset-not-found` (never 403).
- Given an owned image asset, when PUT or GET progress is called, then 404
  `media/asset-not-playable`.
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
| `module` | string ∈ {`media`, `movie`, `music`, `story`, `comic`} | `media` today; the comic leg (P1.6) uses `comic` |
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
- Given only media wired, then the endpoint returns video items and no errors.
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
which mounts Vidstack. This spec creates the route, adds the key to
`templates/types.ts` and the v1 manifest, and adds it to frontend.md §2's
`views` list (all shipped on HEAD except the frontend.md line).
`mediaapi.Continue` (P0.3) builds each item's `href` to this exact path,
`/library/media/{id}`.

**Media deep-link rule** *(binding on every spec that links to a media asset;
SPEC-04 P0.4 and SPEC-06 P0.2 cite it)*: a **video or audio** asset links to
`/library/media/{id}` (this detail player); an **image** asset links to
`/library/media?open={id}`. SPEC-01 P0.4's grid reads `open` and opens its
lightbox for that id, or shows a not-found toast. The player route 404s a
non-video asset (P0.2), so no producer may send an image there. Every producer
or consumer that builds a media href uses this kind-aware rule: notify's
`media.asset_ready` `data.href`, the stream render mapping, and continue items.
*(Code follow-up: on HEAD the three have diverged — `media_progress.sql` builds
`/library/media/{id}`, `journal/stream.go` builds `/library/media`, and notify
builds `/library/{id}`; the grid does not read `open` yet.)*

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
  `/library/media/{id}` mounts the player at the saved position.

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
  SPEC-06 maps this event to a "Watched <title>" card; notify attaches only when
  a `notify:on_*` task for it is specced in SPEC-04 §7, since subscribing a
  task type with no handler makes Asynq fail every such task).
- **P1.6 Comic leg** — if SPEC-02 P0.4 (`comic_reading_progress`) has shipped,
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
                 -- identity-anchor exception (SPEC-04 §6 precedent)
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
| PUT | `/api/v1/assets/{id}/progress` | authenticated (`RequireAuth`; owner-scoped upsert — P0.2) | `{position_ms}`; fire-and-forget client |
| GET | `/api/v1/assets/{id}/progress` | authenticated (`RequireAuth`; owner-scoped — P0.2) | `{position_ms, progress_pct (null if no duration), completed_at, updated_at}`; same status table as PUT (P0.2); 200 with `position_ms` 0 / `updated_at` null when no row exists |
| GET | `/api/v1/continue?limit=` | authenticated (`RequireAuth`) | cmd/api aggregator; module-agnostic items; `limit` default 10, max 50, invalid → default (P0.3) |

Problem types: `media/asset-not-found`, `media/asset-not-ready` (both declared in SPEC-01 §7), `media/asset-not-playable` (new here; register it in `problems.ts` per the specs README Errors DoD — *code follow-up: missing from `problems.ts` on HEAD*).

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
