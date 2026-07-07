# ADR-01: v1 scope cut — what fits in 2 weeks / 1 dev / $100/mo / 1 VPS

**Status:** Accepted
**Date:** 2026-05-24
**Deciders:** kirito

> **Update (2026-07-06):** v1 shipped — the demo loop below is closed and committed; `MILESTONE_CHECKS.md` at repo root is the living status tracker. As-built deltas:
> - Step 1 is superseded by [ADR-06](./06-local-auth-model.md): sign-in is Portal-owned local password auth (`POST /api/v1/auth/login`); Authentik/OIDC is removed from code and compose. The OIDC-specific deliverable rows below (`user_oidc_roles` group sync, `amr`/`acr`/`auth_time` claims) are retired with it.
> - Step 4: dev persists to MinIO; R2 applies to deployed environments only — see ADR-04's Update (2026-06-06).
> - The [D-34] refresh-and-return route was replaced by the `portal_session` middleware gate + `SessionKeeper` client-side silent refresh.
> - CI landed in Phase 6 with `sqlc-drift`; an openapi handler-drift check is still open — `shared/openapi.yaml` currently drifts (missing `/auth/register`, stale `/auth/callback`).
> - As-built routes mount under plain `/api/v1` with no `/t/{tenant}` prefix; the tenant URL contract stays deferred to Phase 1.

## Context

`feature.md` describes 12 phases and 40 settled decisions covering identity, multi-tenancy, media, four content verticals, personal finance, notifications, social, search, marketing site, advanced social (reels/live/audio rooms), creator economy, marketplace, and ML safety. The decisions are individually sound; collectively they describe a platform that would take a small team a year or more to ship.

The stated constraint envelope is:

- 1 developer
- 2 weeks to v1
- ≤ $100/month infrastructure budget
- single VPS

A version of Portal that tries to honour every Phase 0 deliverable in 2 weeks will run out of time around Phase 0 step 8 (out of 14) and ship nothing. A version that picks one coherent slice, ships it, and treats the rest as a backlog can produce a *running* artefact at the end of the sprint.

This ADR makes the cut explicit so it's a decision, not a drift.

## Decision

**v1 ships Phase 0 (foundation wiring) plus a vertical slice of Phase 2 (one video upload happy path) and nothing else.** Everything in Phases 1, 3–12 is deferred. The phase ordering in `feature.md` is unchanged; the scope of what counts as "v1" is the only thing this ADR moves.

Concretely, v1 = the smallest demo that proves the architecture works end-to-end:

1. A user signs in via Authentik (OIDC).
2. They land on the Next.js home page authenticated.
3. They upload an mp4 via the UI.
4. The upload is persisted to R2 (see [ADR-04](./04-storage-tier-budget.md)).
5. The worker picks up the transcode task, produces an HLS ladder, and updates `assets.status = ready`.
6. The user plays the video back in the browser using Vidstack.
7. They sign out; the session is revocable via the existing two-channel mechanism.

That's the entire v1 demo loop. No tenants. No movies/music/stories/comics CRUD. No bank. No social. No notifications. No mediamtx. No LiveKit. No observability stack. No file-gated permissions. No policy bundles. No marketplace.

## Options considered

### Option A — Honour Phase 0 in full, defer everything else

| Dimension | Assessment |
| --- | --- |
| Complexity | High — 14 deliverables in Phase 0 alone |
| Cost | $30–60/mo |
| Scalability | N/A (foundation only) |
| Team familiarity | Solo dev knows this stack |

**Pros:** Phase 0 is the "spec-correct" sprint; every piece set up here pays dividends in every later phase.
**Cons:** 14 deliverables in 2 weeks for 1 dev is ~2 hours each including testing — unrealistic when several (CI workflows, frontend conventions doc, migration audit, RFC 7807 retrofit, observability stack) are multi-hour items. The most likely outcome is "Phase 0 partially done, no running demo."

### Option B — Phase 0 minimum + Phase 2 vertical slice  *(chosen)*

| Dimension | Assessment |
| --- | --- |
| Complexity | Medium — cut Phase 0 from 14 items to ~8 |
| Cost | $30–60/mo |
| Scalability | Single-user demo; multi-tenant deferred |
| Team familiarity | Solo dev knows this stack |

**Pros:** Produces a running demo end-of-sprint. Forces the wiring gap to close on Day 3. Surfaces the integration bugs (cookie flags, CORS, oapi-codegen handler shape, sqlc adapter signatures) that are the actual risk.
**Cons:** Skips the migration audit ([D-18]), the observability stack ([D-8]), the frontend conventions doc ([D-32]/[D-33]), CI workflows ([D-9]), the OpenAPI cross-module schema retrofit ([D-29]). All of these have to land later; some will hurt to retrofit.

### Option C — Skip Phase 0, hand-write a thin auth layer + media demo

| Dimension | Assessment |
| --- | --- |
| Complexity | Low for v1 |
| Cost | $30/mo |
| Scalability | Throwaway — would need full rewrite |
| Team familiarity | Solo dev knows this stack |

**Pros:** Fastest path to a running demo.
**Cons:** Throws away the existing account module (which is already written), the OpenAPI spec, the module boundary discipline, and the modular monolith layout. Builds technical debt the rest of the year is paying off. Only correct if v1 is a *throwaway prototype*; if it's the seed of the real product, this is wrong.

## Trade-off analysis

Option A's failure mode is "no demo at end of sprint, lots of half-finished plumbing." Option C's failure mode is "demo works, can't extend it." Option B's failure mode is "demo works, missing some Phase 0 niceties that need a Phase 0.5 sprint." Of the three, Option B's failure mode is the cheapest to recover from: the missing pieces (CI workflows, frontend conventions doc, observability profile) can each be added in a half-day sprint without touching application code.

The cut to ~8 Phase 0 deliverables is:

| Phase 0 deliverable (from feature.md) | v1? | Reason |
| --- | --- | --- |
| Wire `cmd/api/main.go` | **Yes** | The actual blocker. |
| `make sqlc` for account block + commit decision | **Yes** | Required before adapters compile. |
| Repository adapters for account interfaces | **Yes** | Required to construct the module. |
| Migration `0001` audit (split into 0001/0002/0003/0005) | **Yes** | Cheap to do *now* before data exists; impossible later. [D-18] |
| `users.locale` + `users.timezone` columns | **Yes** | One-line addition during the audit; needed by frontend on day one. |
| Move `audit/` → `platform/audit/` + rename event | **Yes** | Cheap during the migration audit; expensive after audit_log has rows. [D-25] |
| Surface `amr`/`acr`/`auth_time` claims into context | **Yes** | One file change; lets [D-27]/[D-28] land later without rewriting middleware. |
| `user_oidc_roles` table | **Yes** | Lands in `0003_account_rbac`; OIDC group sync writes to it on first login. [D-26] |
| RFC 7807 `Problem` adoption in OpenAPI | **Partial** | Add the schema; retrofit handlers as they're written, not in a sweep. |
| Reserve `notify:*` Asynq prefix | **Yes** | Documentation-only; one line in MODULES.md §5.2. |
| OpenAPI cross-module schemas (Money, PaginatedResult, TenantContext, ContinuingItem) | **No** | Money/Continue/TenantContext aren't needed until bank/Phase 4/tenant ships. Add when first needed. |
| URL versioning + RFC 9745 doc | **No** | `/api/v1/` is already in place; the doc is paperwork that can land in week 3. |
| Frontend server-only API client + refresh-and-return route | **Yes** | Without this, RSC pages can't authenticate against the API. [D-34] |
| Frontend conventions doc (Zustand/TanStack/RHF boundary) | **No** | Solo dev; a doc for an audience of one is paperwork. Add when a second contributor appears. [D-32]/[D-33] |
| CI workflows (lint + test + drift + roundtrip + build + security) | **Partial** | Ship the drift check (`sqlc-drift`, `openapi-drift`) only. Skip multi-arch builds, security scan, integration matrix until week 3. [D-9] |

For Phase 2 the v1 cut is: one queue priority, libx264 only, no hardware encoder paths, no per-user/per-tenant quotas, no backpressure, no dead-letter queue UI (failed transcodes get logged loudly and the operator fishes them out by hand). The single happy-path flow proves the architecture; the polish lands in Phase 2.5.

## Consequences

**What becomes easier:**

- The 2-week sprint has a single, demonstrable success criterion: end-of-sprint demo runs the 7 steps above. Easy to test, easy to know if it's done.
- The wiring gap (the actual blocker) gets closed on Day 1–3 because everything else depends on it.
- Solo-dev cognitive load drops — only the modules touched by the demo loop need to be understood deeply in week 1.

**What becomes harder:**

- Deferring the migration audit *would* be cheaper now than later, so doing it under v1 (rather than punting to Phase 0.5) costs a day. Worth it.
- Skipping the frontend conventions doc means the first contributor (whoever they are) lands without a state-boundary contract. Accept this — the doc can land when needed.
- Skipping the observability stack means the v1 demo runs blind. For a single-user demo this is fine; production deploy should add `--profile observability` before any external users.
- Skipping multi-tenant means the URL prefix `/t/{tenant}/...` isn't exercised in v1. Defer the URL contract until Phase 1; do NOT hard-code the v1 demo paths in a way that's incompatible with the prefix later. Use `/api/v1/healthz` only; protected routes should already live under `/t/me/api/v1/...` even with a hard-coded `me` tenant.

**What we'll need to revisit:**

- After v1 ships, run a Phase 0.5 sprint to close the skipped Phase 0 items (CI workflows in full, frontend conventions doc, observability profile setup) before Phase 1 (tenancy) begins.
- Pick up Phase 1 (tenancy + RLS) as the next sprint; the `me` synthetic tenant carries forward unchanged. [D-23]/[D-24]
- The RBAC schism ([ADR-02](./02-rbac-model-reconciliation.md)) should be resolved BEFORE Phase 1 so the role + policy migrations land in one pass.

## Action items

1. [x] Pin this ADR (`Accepted`) before writing any code for the 2-week sprint. *(done 2026-07-06 — status flipped; v1 was built under this cut)*
2. [ ] Open a tracking issue/todo with the 7-step demo as the literal acceptance criterion. → acceptance tracked in `MILESTONE_CHECKS.md`
3. [ ] Add a `v1-out-of-scope` label/section in the issue tracker for everything in §3 of the executive review — keeps the deferred work visible without inviting scope creep.
4. [ ] Add `# v1 scope: see doc/en/architecture/01-v1-scope-cut.md` as a comment at the top of `cmd/api/main.go` so future-you doesn't reach for a non-v1 module.
5. [ ] At sprint end, write a one-page "what we cut, what we shipped, what we learned" retrospective; promote any cut-but-needed items into the Phase 0.5 backlog.
