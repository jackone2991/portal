# SPEC-07 — Playback Resume + Continue Rail (D-20 execution)

**Status:** ready to build, rev 1 · **Drafted:** 2026-07-10
**Module:** `media` (built + wired); aggregator mounts in `cmd/api` · **Depends on:** nothing hard (the video leg runs on today's stack); the comic leg plugs in when SPEC-02 ships `comic_reading_progress`
**Upstream:** [briefs/07-continue-rail.md](../briefs/07-continue-rail.md) · **Refs:** feature-inventory `D-20` (continue aggregator shape), SPEC-02 P0.4 (progress-beacon convention), frontend.md
**Downstream consumers:** SPEC-06 P0.4 (continue widget), SPEC-04 (open type registry, via P1.5's event)

---

## 1. Problem statement

Portal plays HLS video end to end, but every session restarts at 0:00. The gap
between "works in a demo" and "used nightly" is exactly
resume-where-you-left-off — Jellyfin and Plex prove it is *the* retention surface
for a personal media server. `D-20` already decided the aggregator shape
(`GET /continue` fanning out to `<module>api.Continue`); with two content
surfaces imminent (video now, SPEC-02 comics with its own progress table), the
aggregator is at its cheapest right now.

## 2. Goals

1. Reopening a video resumes within ±10 s of where playback stopped — across
   devices (same account).
2. One `GET /api/v1/continue` returns the cross-module in-progress list — video
   today, comics automatically when SPEC-02 lands, with **no response-shape
   changes** at that point.
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

### P0.2 — Beacon

`PUT /api/v1/assets/{id}/progress {position_ms}` — **authenticated
(`RequireAuth`), owner-scoped by construction**: the row is keyed by the
caller's own user id, exactly like SPEC-02 P0.4's comic progress writes
*(2026-07-10 — the drafted `media:progress:own` permission repeated the
module-prefix-as-resource pattern and added a code with nothing extra to
protect; dropped for parity with the existing progress convention)*. Upsert,
last-write-wins. Server clamps `position_ms` into `[0, duration_ms]` (lower
bound only when duration is NULL — P0.1) and 404s assets that are unknown,
non-video, `deleting` (until SPEC-01 P0 ships the status this clause is a
no-op — no asset can be in that state yet), **or not owned by the caller**
(asset visibility is owner-only at v1; the prose previously omitted the
cross-owner case its own AC tested).

`GET /api/v1/assets/{id}/progress` — same auth/owner-scoped construction as
the PUT above, returns `{position_ms, progress_pct (null when duration_ms IS
NULL), completed_at, updated_at}` for the caller's own row, 404ing under the
identical conditions as the PUT. This is the read path P0.4 fetches before
initializing Vidstack — `/continue`'s `progress_pct` is a rounded percentage,
not precise enough to seek by.

Client: throttled from the Vidstack player — every ~10 s while position advances,
plus on pause and `pagehide` (via `navigator.sendBeacon`/keepalive fetch) —
mirroring SPEC-02 P0.4's convention. **Fire-and-forget**: beacon failures never
surface to the player or block playback. The `sendBeacon` call sends a
`Blob([JSON.stringify(body)], {type: 'application/json'})`, not a bare string —
`sendBeacon` defaults to `Content-Type: text/plain`, which the handler cannot
parse as JSON — or uses a keepalive `fetch`, which can set the header
directly. A `pagehide` beacon fired after the access token has expired is
dropped (nothing refreshes the cookie on unload); acceptable because the
~10 s/on-pause save cadence bounds the lost window to a few seconds. No CSRF
concern: cookies are `SameSite=Strict`, so a beacon from a third-party page
never carries them.

**Acceptance criteria.**
- Given playback to 12:34 then tab close, when the asset is reopened, then the
  player offers resume at ~12:34 (±10 s).
- Given a beacon for another user's asset (regardless of the caller's role or
  permission grants — there is no permission-based bypass), then 404 and no row.
- Given `position_ms` beyond the asset duration, then it is clamped, never 500.
- Given the API briefly down, then playback continues unaffected (no error UI).

### P0.3 — Aggregator `GET /api/v1/continue`

Per D-20, mounted in **`cmd/api`** (resolved from the brief: the aggregator is
cross-module by definition and the wiring layer is the documented home for such
composition — the second sanctioned instance after the `Engine()` exception; it
calls only module `api/` packages, so the boundary rule holds). Fans out to
`mediaapi.Continue(ctx, userID, limit)` today; `comicapi.Continue` joins after
SPEC-02 — the fan-out signature is the seam, and the response shape is
module-agnostic from day one:

```
[{module, ref_id, title, poster_url, progress_pct, href, updated_at}]
```

sorted `updated_at DESC`, limited (`?limit=`, default 10, max 50). Requires
authentication only — each module call is owner-scoped by construction, so no
separate permission code is needed beyond `RequireAuth`. `title` falls back to
a filename derived from `assets.source_key` until SPEC-01 P0 lands
`assets.title` (a soft dependency — see the header).

**Inclusion predicate** (applied module-side, in `mediaapi.Continue`): an item is
"in progress" iff `position_ms ≥ 30 000` **and** `progress_pct < 95` (resolves
the brief's threshold question: accidental clicks below 30 s never appear; the
completed drop-off is a P0 predicate, not a P1 afterthought).

**Acceptance criteria.**
- Given only media wired, then the endpoint returns video items and no errors.
- Given items at 20 s watched and at 97% watched, then neither appears.
- Given comicapi joining later, then the response shape is unchanged (contract
  test on the item schema — no video special-casing).
- Given a saved position of 10:00 on an asset with `duration_ms IS NULL`, then
  the item does not appear in `/continue`, reopening still offers resume at
  ~10:00, and no `media:playback_completed` event is ever emitted for it.

### P0.4 — Resume UX

**Playback host:** the per-asset player lives at
`app/(app)/library/media/[id]/page.tsx`, resolving a template-manifest view
(`TemplateManifest.views`, frontend/src/templates conventions) that mounts
Vidstack — this route does not exist yet and is created as part of this spec.
`mediaapi.Continue` (P0.3) builds each item's `href` to this exact path,
`/library/media/{id}`.

Before initializing Vidstack, the player fetches `GET
/api/v1/assets/{id}/progress` (P0.2) to obtain the exact saved `position_ms` —
`/continue`'s `progress_pct` is a rounded percentage, not enough to seek by.
Vidstack starts at the saved position above the 30 s threshold and below
95% — the percent gate is skipped when `progress_pct` is undefined (NULL
`duration_ms`, P0.1), in which case any saved position ≥ 30 s resumes, with a
visible "Start over" affordance; otherwise starts at 0.

**Acceptance criteria.**
- Given saved progress at 12:34, then playback opens at 12:34 and "Start over"
  restarts at 0 (and the next beacon overwrites the old position).
- Given progress at 97%, then playback starts at 0 (finished content replays).

### P1 — nice to have

- **P1.5 Completion event**: `media:playback_completed`
  `{asset_id, user_id, title}` (`title` falls back to `original_filename` per
  SPEC-01 P1.2) emitted **once** when progress first crosses ≥95%
  (undefined-duration assets never emit — P0.1) — latched via a nullable
  `completed_at` on the progress row (emit only on the NULL→set transition, so
  retries and repeat crossings never double-emit). Published via
  `platform/events` (events.md "Delivery mechanics" — two consumers are
  registered). Consumers: stream (SPEC-06 — maps this event to a "Watched
  <title>" card), notify (SPEC-04 open type registry) — none required to ship.

### P2 — future considerations (design for, don't build)

- **Comic leg** — arrives with SPEC-02's `comic_reading_progress`; only
  `comicapi.Continue` and one line in the aggregator's fan-out list.
- **Watched-history page** — `completed_at` + `updated_at` already hold the data;
  UI later.

## 6. Data model — migration `000N_media_playback_progress`

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
| GET | `/api/v1/assets/{id}/progress` | authenticated (`RequireAuth`; owner-scoped — P0.2) | `{position_ms, progress_pct (null if no duration), completed_at, updated_at}`; same 404s as PUT |
| GET | `/api/v1/continue?limit=` | authenticated (`RequireAuth`) | cmd/api aggregator; module-agnostic items |

Problem types: `media/asset-not-found` (shared with SPEC-01 — first spec to ship defines it), `media/asset-not-playable`.

## 8. Success metrics (n=1 honest)

- Leading: resume offered on reopen within ±10 s of true position, verified
  cross-device (desktop → phone) during the first dogfood week.
- Leading: zero player jank or errors attributable to beacons (fire-and-forget
  verified under an API outage).
- Lagging: the owner actually resumes (≥ 3 resumes/week once a multi-session
  video exists) — the retention claim this spec is built on.

## 9. Timeline & phasing

1. Migration + queries + `mediaapi.Continue` (½ day)
2. Beacon endpoint + Vidstack throttle/pagehide wiring + resume UX (1.5 days)
3. `cmd/api` aggregator + OpenAPI + contract test (1 day)
4. P1.5 completion latch + event (½ day)
P0 ≈ 3 dev-days; P1 adds ½. Matches the brief's ~4.

## 10. Open questions

- **(resolved)** Aggregator home: `cmd/api` composition (P0.3).
- **(resolved)** Resume threshold: ignore < 30 s and ≥ 95% (P0.3/P0.4).
- **(resolved 2026-07-10)** Duration column: `assets.duration_ms` exists
  (migration 0007) but is nullable — handled by P0.1's NULL-duration rule.
- **(product, non-blocking)** Should "Start over" also clear the progress row, or
  just play from 0 and let the next beacon overwrite? Recommendation: the latter
  (simpler; identical outcome after ~10 s of playback).
