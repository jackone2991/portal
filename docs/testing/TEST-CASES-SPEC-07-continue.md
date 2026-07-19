# Test Cases — SPEC-07 Playback Resume + Continue Rail

**Spec:** [SPEC-07](../product/specs/SPEC-07-continue-rail.md) · **Module:** `media` + `cmd/api` aggregator
**Prefix:** `TC-CONT-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md)

### Endpoints under test

| Method | Path | Perm |
|---|---|---|
| PUT | `/api/v1/assets/{id}/progress` | authenticated (owner-scoped upsert) |
| GET | `/api/v1/assets/{id}/progress` | authenticated (owner-scoped) |
| GET | `/api/v1/continue?limit=` | authenticated (aggregator, cmd/api) |

### Preconditions

- Accounts `owner`,`userA`,`userB`,`guest`. `owner` has ≥2 ready videos with known
  `duration_ms`, plus one asset with `duration_ms IS NULL` (failed/older probe).
- Problem types: `media/asset-not-found`, `media/asset-not-playable`.
- Event `media:playback_completed {asset_id, user_id, title}` (P1.5).

---

## P0.1 — Progress table + NULL-duration rule

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-CONT-001 | Beacon upserts (PK user+asset) | Functional | P0 | PUT progress twice | last-write-wins; single row PK `(user_id, asset_id)` | ☐ |
| TC-CONT-002 | NULL-duration: upsert works, pct undefined | Functional | P0 | PUT on NULL-duration asset | upsert saved (clamped ≥0); `progress_pct` null | ☐ |
| TC-CONT-003 | NULL-duration excluded from /continue | Functional | P0 | asset NULL-duration at 10:00 | not in `/continue`; resume still offers ~10:00; no completion event ever | ☐ |

## P0.2 — Beacon

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-CONT-020 | Resume within ±10 s | Functional | P0 | play to 12:34, close tab, reopen | player offers resume at ~12:34 (±10 s) | ☐ |
| TC-CONT-021 | Cross-owner beacon → 404 | AuthZ | P0(S1) | userB PUT progress on userA's asset (any role/grant) | 404; **no** row (no permission-based bypass) | ☐ (CC-3) |
| TC-CONT-022 | Clamp beyond duration | Boundary | P0 | PUT position_ms > duration | clamped to `[0, duration_ms]`, never 500 | ☐ |
| TC-CONT-023 | Clamp negative | Boundary | P0 | PUT position_ms < 0 | clamped to ≥0 | ☐ |
| TC-CONT-024 | Unknown/non-video/deleting → 404 | Negative | P0 | PUT on unknown, non-video, deleting asset | 404 | ☐ |
| TC-CONT-025 | Fire-and-forget under API outage | Reliability | P0 | API briefly down; keep playing | playback unaffected; no error UI; beacon failures silent | ☐ |
| TC-CONT-026 | sendBeacon Content-Type JSON | Frontend | P0 | inspect pagehide beacon | sends `Blob(..., {type:'application/json'})` (not text/plain) so handler parses | ☐ |
| TC-CONT-027 | GET progress read path | Functional | P0 | GET `/assets/{id}/progress` | `{position_ms, progress_pct (null if no duration), completed_at, updated_at}`; same 404s as PUT | ☐ |
| TC-CONT-028 | Cross-device continuity | Functional | P1 | stop at 43:00 desktop; open phone (same account) | resumes at 43:00 | ☐ [MANUAL] |
| TC-CONT-029 | SameSite=Strict (no CSRF) | Security | P1 | third-party page fires beacon | cookies not sent (Strict) → no cross-site write | ☐ |

## P0.3 — Aggregator `/continue`

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-CONT-040 | Media-only returns video items | Functional | P0 | GET `/continue` with only media wired | video items; no errors | ☐ |
| TC-CONT-041 | Inclusion predicate (≥30 s AND <95%) | Functional | P0(S1) | items at 20 s and at 97% | **neither** appears (accidental-click + finished drop-off) | ☐ |
| TC-CONT-042 | In-progress item appears | Functional | P0 | item at 40% watched, >30 s | appears with progress bar + href `/library/media/{id}` | ☐ |
| TC-CONT-043 | Response shape module-agnostic | Contract | P0 | inspect item schema | `{module, ref_id, title, poster_url, progress_pct, href, updated_at}`; no video special-casing (contract test) | ☐ |
| TC-CONT-044 | Sorted updated_at DESC, limit clamp | Boundary | P0 | GET `?limit=100` | sorted `updated_at DESC`; clamped to max 50 (default 10) | ☐ |
| TC-CONT-045 | Owner-scoped by construction | AuthZ | P0(S1) | userA `/continue` | only userA's in-progress items | ☐ (CC-3) |
| TC-CONT-046 | title fallback to filename | Functional | P1 | asset without title | falls back to filename from source_key | ☐ |
| TC-CONT-047 | Aggregator mounted in cmd/api | Contract | P1 | inspect wiring | aggregator in cmd/api; calls only module `api/` (boundary holds) | ☐ |

## P0.4 — Resume UX

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-CONT-060 | Player fetches progress before init | Frontend | P0 | open player | GET `/assets/{id}/progress` before Vidstack init; seeks to exact position | ☐ |
| TC-CONT-061 | Opens at 12:34, Start over → 0 | Frontend | P0 | saved 12:34 | opens at 12:34; "Start over" restarts at 0; next beacon overwrites | ☐ |
| TC-CONT-062 | 97% starts at 0 (replay) | Functional | P0 | saved progress at 97% | starts at 0 (finished content replays) | ☐ |
| TC-CONT-063 | NULL-duration resume + Start over | Functional | P1 | NULL-duration asset, saved ≥30 s | resumes at saved position (percent gate skipped); visible "Start over" | ☐ |
| TC-CONT-064 | Player route exists | Contract | P0 | GET `/library/media/{id}` | route resolves a template-manifest view mounting Vidstack | ☐ |

## P1 — Completion event

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-CONT-080 | Emit once at ≥95% crossing | Integration | P1 | cross 95% first time | `media:playback_completed {asset_id,user_id,title}` emitted once; `completed_at` latched | ☐ [P1] |
| TC-CONT-081 | No double-emit on repeat crossing | Idempotency | P1 | re-watch, cross 95% again; retry | no second emit (NULL→set latch) | ☐ [P1] |
| TC-CONT-082 | NULL-duration never emits | Integration | P1 | NULL-duration asset | no completion event ever | ☐ [P1] |
| TC-CONT-083 | Two consumers registered | Integration | P1 | inspect events.md | stream + notify both registered for `media:playback_completed` | ☐ [P1] (CC-5) |

## Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-CONT-100 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-CONT-101 | Resume survives restart | Reliability | P0 | save; `make up` restart; reopen | progress persists (DB) | ☐ |
| TC-CONT-102 | Migration up/down | Contract | P1 | migrate + down `media_playback_progress` | clean; PK, position≥0 CHECK, completed_at, index; same-module FK to assets | ☐ |
