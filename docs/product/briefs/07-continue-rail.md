# 07 — Playback Resume + Continue Rail (D-20 execution)

**Module:** `media` (built + wired) · **Effort:** ~4 days · **Depends on:** nothing (video leg works on today's stack); the comic leg plugs in when SPEC-02 ships its progress table.
**Unlocks:** the retention mechanic of every personal media server (Jellyfin/Plex's most-used surface); the continue widget in brief 06's rail.
**Provenance:** promotion of cataloged decision `D-20` (continue aggregator), justified by the closed video loop — researched 2026-07-10. **Spec:** [SPEC-07](../specs/SPEC-07-continue-rail.md).

## Problem statement

Portal plays HLS video end to end, but every session restarts at 0:00. The gap
between "works in a demo" and "used nightly" is exactly resume-where-you-left-off —
Jellyfin and Plex prove it is *the* retention surface for a personal media server.
`D-20` already decided the aggregator shape (`GET /continue` fanning out to
`<module>api.Continue`); with two content surfaces imminent (video now, SPEC-02
comics with its own `comic_reading_progress`), the aggregator is at its cheapest
right now.

## Goals

- Reopening a video resumes within a few seconds of where playback stopped.
- One `GET /api/v1/continue` returns the cross-module in-progress list (video today,
  comics automatically when SPEC-02 lands).
- ≥95%-watched emits the first "watched X" life-stream fact.

## Non-goals

- Movie catalog/metadata CRUD — the [04-deferred](04-deferred.md) movie-vertical row
  stays parked (re-entry: after SPEC-02 proves the pattern); this hangs off existing
  assets only.
- Cross-device sync conflicts beyond last-write-wins.
- Bus traffic per beacon — position writes are deliberately **not** events (too
  chatty); only the completion fact hits the bus.

## User stories

- As the owner, I stop a movie at minute 43 on the desktop and resume at minute 43
  on my phone.
- As the owner, the home rail shows the three things I'm mid-way through, with
  progress bars, and clicking one drops me back in.

## Requirements

### P0 — must have

1. **Progress table** `media_playback_progress` (media module migration, next free
   number) — PK `(user_id, asset_id)`, `position_ms`, `updated_at`; percent-complete
   derives from the asset's existing `duration_ms`.
2. **Beacon**: `PUT /api/v1/assets/{id}/progress {position_ms}` — throttled from the
   Vidstack player (~10 s + on pause/pagehide, mirroring SPEC-02 P0.4's convention);
   RBAC: authenticated owner-scoped upsert (`media:progress:own` — 3-segment grammar).
   - [ ] Given playback to 12:34 then tab close, when reopened, then the player
         offers resume at ~12:34 (±10 s).
   - [ ] Beacons never block playback (fire-and-forget, no error surfacing).
3. **Aggregator** `GET /api/v1/continue` per D-20: fans out through module `api/`
   packages (`mediaapi.Continue(userID, limit)` today; `comicapi` joins later);
   returns `[{module, ref_id, title, poster_url, progress_pct, href}]`, most-recent
   first.
   - [ ] With only media wired, the endpoint returns video items and no errors.
4. **Resume UX**: Vidstack starts at the saved position (with a small "start over"
   affordance).

### P1 — nice to have

5. **`media:playback_completed`** emitted once when progress first crosses ≥95%
   `{asset_id, user_id}` — register in events.md; consumers: stream (brief 06),
   notify (SPEC-04, open type registry).
6. Completed items drop off `/continue` automatically (predicate: <95%).

### P2 — future considerations (design for, don't build)

7. Comic leg — arrives with SPEC-02's `comic_reading_progress`; the aggregator's
   fan-out signature is the seam, don't special-case video in its response shape.
8. "Watched history" page — the table already holds it; UI later.

## Data model sketch

```
media_playback_progress(
  user_id uuid not null, asset_id uuid not null references media_assets(id) on delete cascade,
  position_ms bigint not null check (position_ms >= 0),
  updated_at timestamptz not null default now(),
  primary key (user_id, asset_id)
)
```

## API sketch (add to `shared/openapi.yaml`)

```
PUT /api/v1/assets/{id}/progress    {position_ms}
GET /api/v1/continue?limit=
```

## Open questions

- **(engineering, non-blocking)** Where `GET /continue` mounts: `cmd/api` composes
  module `Continue()` calls (like the `Engine()` exception) vs a thin aggregator in
  `media`. Recommendation: `cmd/api` composition — the aggregator is cross-module by
  definition and the wiring layer is the documented home for that.
- **(product, non-blocking)** Resume threshold: ignore progress <2% or <30 s
  (accidental clicks)? Recommendation: yes, ignore below 30 s.
