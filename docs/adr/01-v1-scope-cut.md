# ADR-01: v1 scope cut — what fits in 2 weeks / 1 dev / $100/mo / 1 VPS

**Status:** **accepted** 2026-05-24 · scope since widened by [ADR-08](./08-life-os-pivot.md)
**Last verified:** 2026-09-11
**Deciders:** kirito

## Context

*As found on 2026-05-24. The cut this ADR made was executed and closed: the
demo loop shipped on 2026-07-06 and is the regression baseline. What has been
built since is not tracked here — [`/CLAUDE.md`](../../CLAUDE.md) § Current
status owns that.*

[`feature-inventory.md`](../product/feature-inventory.md) (then `feature.md`) described 12 phases and 40 settled decisions covering identity, multi-tenancy, media, four content verticals, personal finance, notifications, social, search, marketing site, advanced social (reels/live/audio rooms), creator economy, marketplace, and ML safety. The decisions are individually sound; collectively they describe a platform that would take a small team a year or more to ship.

The stated constraint envelope was:

- 1 developer
- 2 weeks to v1
- ≤ $100/month infrastructure budget
- single VPS

A version of Portal that tried to honour every Phase 0 deliverable in 2 weeks would run out of time around Phase 0 step 8 (out of 14) and ship nothing. A version that picked one coherent slice, shipped it, and treated the rest as a backlog could produce a *running* artefact at the end of the sprint.

This ADR made the cut explicit so it was a decision, not a drift.

## Decision

**v1 ships Phase 0 (foundation wiring) plus a vertical slice of Phase 2 (one video upload happy path) and nothing else.** Everything in Phases 1, 3–12 was deferred by this cut. The phase ordering in `feature.md` was unchanged; the scope of what counted as "v1" was the only thing this ADR moved.

Concretely, v1 = the smallest demo that proves the architecture works end-to-end. As built (two steps differ from the 2026-05-24 text: sign-in was to be Authentik/OIDC and the upload was to go straight to R2):

1. A user signs in with a local password ([ADR-06](./06-local-auth-model.md): `POST /api/v1/auth/login`; Authentik/OIDC never shipped).
2. They land on the Next.js home page authenticated.
3. They upload an mp4 via the UI.
4. The upload is persisted to MinIO in dev and R2 in deployed environments ([ADR-04](./04-storage-tier-budget.md)).
5. The worker picks up the transcode task, produces an HLS ladder, and updates `assets.status = ready`.
6. The user plays the video back in the browser using Vidstack.
7. They sign out; the session is revocable via the existing two-channel mechanism.

That was the entire v1 demo loop. At the time of the cut: no tenants, no movies/music/stories/comics CRUD, no bank, no social, no notifications, no mediamtx, no LiveKit, no observability stack, no file-gated permissions, no policy bundles, no marketplace. Of those, tenants, the four verticals, bank, notifications and a first social slice have since shipped under ADR-08; the rest remain out.

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

**What became easier:**

- The 2-week sprint had a single, demonstrable success criterion: the 7 steps above. It ran, and it still runs — it is the regression baseline every later change is checked against.
- The wiring gap (the actual blocker) closed first because everything else depended on it.
- Solo-dev cognitive load dropped — only the modules touched by the demo loop needed to be understood deeply in week 1.

**What became harder, and how it resolved:**

- The migration audit was done under v1 rather than punted; it cost a day and was worth it.
- The frontend conventions doc was skipped for v1 and later written as [`frontend/CLAUDE.md`](../../frontend/CLAUDE.md) ([D-32]/[D-33]/[D-34]).
- The observability stack was skipped and is still absent (ADR-03).
- The `/t/{tenant}/...` URL prefix was never adopted. Tenancy landed ([ADR-07](./07-tenancy-rls-model.md)) with routes staying under plain `/api/v1`; the tenant is resolved by middleware (`RequireTenant` in `cmd/api/main.go`), not by the path.
- The [D-34] refresh-and-return route was replaced by the `portal_session` middleware gate plus `SessionKeeper` client-side silent refresh.

**What we said we'd revisit:**

- A Phase 0.5 sprint for the skipped items never ran as such; CI landed in Phase 6 (the `backend`, `lint`, `openapi`, `frontend` and `link-check` jobs in `.github/workflows/ci.yml` — there is no `sqlc-drift` job and never was, sqlc output is not committed), the conventions doc landed with the frontend work, observability has not.
- Phase 1 (tenancy + RLS) landed per ADR-07; the `me` synthetic tenant was not carried forward — personal organisations are real rows.
- The RBAC schism was resolved by [ADR-02](./02-rbac-model-reconciliation.md) before tenancy, as intended.

## Action items

1. [x] Pin this ADR (`Accepted`) before writing any code for the 2-week sprint (2026-07-06 status flip; v1 was built under this cut).
2. [x] Acceptance criterion tracked and met — the loop shipped 2026-07-06. (It was tracked in `MILESTONE_CHECKS.md`, deleted in `f11cf3f`; status now lives in code, see `/CLAUDE.md`.)
3. [ ] A `v1-out-of-scope` label/section in an issue tracker — there is no issue tracker; the deferred list lives in `/CLAUDE.md` § "Still deferred".
4. [x] Scope comment at the top of `cmd/api/main.go` (path corrected 2026-09-11 to `docs/adr/01-v1-scope-cut.md`).
5. [ ] Sprint-end retrospective — never written. The nearest thing is [analysis/spec-gap-fix-worklog-2026-07-11.md](../product/analysis/spec-gap-fix-worklog-2026-07-11.md).
