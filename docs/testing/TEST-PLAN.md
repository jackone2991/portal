# Portal v1 — Master Test Plan

**Version:** 1.0 · **Author:** QA Lead · **Date:** 2026-07-12
**Status:** Baseline · **Applies to:** Portal v1 (SPEC-01 … SPEC-09)

---

## 1. Introduction & purpose

Portal is a self-hosted **life-OS monorepo** (media / comic / finance / journal /
people / stream / ops) built as a Go modular-monolith backend + Next.js 15
frontend. This plan defines **what** we test, **how**, in **which environments**,
and the **criteria** by which a build is judged shippable for v1.

The requirement baseline is the nine implementation-ready specs in
[docs/product/specs/](../product/specs/). Each spec states acceptance criteria
(AC); this plan turns every AC into one or more executable test cases in the
companion `TEST-CASES-SPEC-*.md` files, and tracks coverage in
[TRACEABILITY-MATRIX.md](TRACEABILITY-MATRIX.md).

## 2. Scope

### 2.1 In scope

| Area | Modules | Spec |
|------|---------|------|
| Media image pipeline + asset lifecycle | `media` | SPEC-01 |
| Comic vertical (create→publish→read→resume) | `comic` | SPEC-02 |
| Finance ledger (accounts/txn/transfer/category/budget/dashboard) | `bank` | SPEC-03 |
| Notification store + dispatch + email + password reset | `notify`, `account` | SPEC-04 |
| Journal write path | `journal` | SPEC-05 |
| Life-stream home (projection + read + widgets) | `journal`/stream + home | SPEC-06 |
| Playback resume + continue rail | `media` + `cmd/api` aggregator | SPEC-07 |
| People registry + birthday scan | `people` | SPEC-08 |
| Platform ops (backup/restore/queue/takeout) | `ops` | SPEC-09 |
| Cross-cutting: auth/RBAC, RFC-7807 errors, events fan-out, cursor pagination | account, `platform/*` | all specs' "Conventions" (README) |

### 2.2 Out of scope (v1)

- Deferred modules/features: bank real-institution integration, social graph,
  marketplace, creator economy, ML safety, LiveKit/mediamtx, the 5-service
  observability stack (per [ADR-01](../adr/01-v1-scope-cut.md)).
- Multi-tenant / household RLS, cross-user data sharing.
- Load/stress at scale beyond the n=1 single-VPS envelope (perf budgets in §8 are
  single-user targets, not concurrency benchmarks).
- Penetration testing / formal security audit (basic security cases **are** in
  scope — see §3 "Security & abuse").

## 3. Test strategy & levels

Testing is layered; a defect is cheapest to catch at the lowest level that can
express it.

| Level | Owner | Tooling | What it covers |
|-------|-------|---------|----------------|
| **L1 Unit** | Dev | `go test`, `vitest` | Pure logic: RBAC matcher, Money/exponent math, `nextOccurrence` (TZ/Feb-29), cursor codec, retention keep/prune, budget roll-up, password hashing. |
| **L2 Integration (backend)** | Dev/QA | `go test` w/ Postgres+Dragonfly+MinIO (docker), Asynq inspector | Repository↔DB (sqlc), transactional projection writes, event fan-out (publish→consume→row), idempotency under redelivery, migration up/down. |
| **L3 API / Contract** | QA | `curl`/Postman/REST-client, `openapi` diff | Every endpoint: status codes, RFC-7807 Problem bodies, permission enforcement, cursor pagination, request validation, spec↔handler drift. |
| **L4 E2E / System** | QA | Browser + Playwright (MCP available), `make up` full stack | User journeys across modules (upload→transcode→play→resume→stream card; create comic→publish→read; log expense→dashboard→stream). |
| **L5 Frontend / Component** | QA | Playwright, manual, `tsc`/`vitest` | RSC-vs-client boundaries, optimistic mutations + rollback, empty/error states, markdown sanitization, fixture-removal grep tests, LCP/CLS budgets. |
| **NFR** | QA | see §8 | Performance budgets, reliability (async durability, OOM guard), data integrity (reconciliation, byte-identical originals, backup/restore), security/abuse. |

### 3.1 Test types applied per feature

- **Functional / happy path** — the AC's primary scenario.
- **Negative / validation** — bad input → correct Problem type + status; no 500s.
- **Boundary** — min/max lengths, `amount ≤ 0`, dimension caps (8000 px), size cap
  (50 MB), `days` clamp (1–366), limit clamp (max 50), Feb-29, month boundaries.
- **Authorization** — owner isolation (user B never sees user A), `:own` vs `:any`
  scope, unseeded-code 403, unauthenticated 401, existence-non-leak (404 not 403).
- **Idempotency / async** — Asynq redelivery, event dedup (`ON CONFLICT`), outbox
  re-publish, retry-safe reads (POST /read twice), transfer-leg collapse.
- **Integration / cross-module** — event producer→bus→consumer→projection; soft
  cascade on `media:asset_deleted`; `/continue` aggregator fan-out.
- **Security & abuse** — enumeration-safe reset (uniform status+timing), throttles
  (per-email/per-IP/global ceiling), HTML/script sanitization, GPS/EXIF privacy
  split (variants stripped vs original private), SameSite cookies, token rotation
  reuse detection.
- **Data integrity / reliability** — money reconciliation (property test), original
  checksum, backup sha256, restore drill, no-OOM under heavy queue.

## 4. Test environments

| Env | Purpose | Bring-up | Notes |
|-----|---------|----------|-------|
| **DEV-LOCAL** | Primary functional/API/E2E | `make up` (Postgres, PgBouncer, Dragonfly, MinIO+setup, Traefik, api, worker, frontend, Mailpit) + `make migrate` | HTTPS via `*.portal.localhost` mkcert certs; `COOKIE_SECURE` may be `false` only for plain-http. Storage = MinIO bind-mount. |
| **CI** | L1/L2 gate on every PR | GitHub Actions (`.github/workflows/ci.yml`) | `go test ./...`, `vitest run`, golangci-lint, sqlc-drift + openapi-drift gates. `-race` needs cgo. |
| **DEV-STACK (prod-like)** | Restore drill, backup, email | `make up` with R2-shaped config, `make restore-drill` | SPEC-09 P0.4 restore drill runs on a **fresh** dev stack. |

**Access matrix (create these once, reuse across cases):**

| Handle | Role | Purpose |
|--------|------|---------|
| `owner` | `creator` (or higher) | the v1 single owner — has `assets:write:own`, all `bank-*`, `comics:write/publish:own`, `journal:*`, `people:*`, `notifications:*`, `stream:read:own`. |
| `userA` | `user` | baseline authenticated user; owner-isolation reference. |
| `userB` | `user` | second user — used to prove cross-owner 404/isolation. |
| `editor` | `editor` | holds `comics:write:any` / `comics:publish:any` (moderation). |
| `admin` | `admin` | holds `comics:delete:any`, `assets:delete:any`, `ops:read`, `queues:read`. |
| `guest` | unauthenticated | 401 reference. |

> Provision via `POST /auth/register` then a role grant (admin/CLI). Record each
> account's email+password in the run's secure test-data sheet (never commit).

## 5. Test data strategy

- **Fixtures (media):** curate a fixture set under a test assets folder —
  `12mp.jpg` (EXIF Orientation=6 + GPS), `transparent.png`, `photo.webp`,
  `animated.gif`, `sample.heic`, `corrupt.bin` (jpg magic + garbage),
  `9000px.jpg` (>8000 px), `oversize_51mb.jpg`, `short_2s.mp4`, `audio_only.mp4`
  (mp3 renamed), `multi_chapter_pages/` (ordered images), `chapter.zip` (valid),
  `zipbomb.zip`, `nested_dirs.zip`, `traversal.zip`.
- **Bank data:** reproducible scripts creating ≥3 accounts (TCB/checking,
  cash, Momo/ewallet), seed categories are migration-provided (26 VN categories),
  transaction sequences for reconciliation property tests, one transfer, one
  transfer+fee.
- **Time-sensitive:** birthday cases need controllable "today" — prefer a
  test-injected clock or fixtures dated relative to run day; document the TZ used
  (owner TZ default `Asia/Ho_Chi_Minh`). Feb-29 cases need a leap and non-leap year.
- **Isolation:** each functional run starts from a **known DB state** — either a
  fresh `make migrate` DB or a documented seed. Owner-isolation cases require two
  distinct users with non-overlapping data.
- **Never** commit real credentials, real financial data, or real photos with GPS.

## 6. Entry & exit criteria

### 6.1 Entry (a spec is ready to test)

- The spec's endpoints exist in `shared/openapi.yaml` and the handlers are mounted
  (`GET /healthz` green; the route returns non-404).
- Migrations applied (`make migrate`); `go test ./...` green on the build.
- Test environment up; fixtures loaded; the six test accounts provisioned.

### 6.2 Exit / sign-off (per spec)

- **100%** of `P0`/`Critical` cases executed; **100% PASS** or an accepted,
  written deviation (defect with agreed disposition).
- **≥90%** of `High` cases PASS; no open `Critical`/`High` defect without a
  workaround.
- Traceability matrix shows every P0 AC mapped to ≥1 executed case.
- Non-functional budgets (§8) met for the spec's user-facing routes.

### 6.3 Suspension / resume

- Suspend a spec's run if a `Blocker` defect prevents >30% of its cases (e.g.
  module not mounted, migration failure, worker crash-loop). Resume when fixed.

## 7. Defect management

**Severity** (impact) — **Priority** (fix urgency) are tracked separately.

| Severity | Definition | Examples |
|----------|-----------|----------|
| **S1 Critical** | Data loss/corruption, security breach, module down, money math wrong, backup/restore broken. | Reconciliation off; original not byte-identical; user A reads user B's finance; reset token reusable; stream event never projected. |
| **S2 Major** | Core AC fails, no workaround. | Publish allows empty chapter; transfer double-counts in totals; delete leaves storage objects; 500 instead of Problem. |
| **S3 Minor** | AC fails with workaround, or non-primary path. | Wrong Problem `type` URI; missing i18n key; cursor off-by-one recoverable by refresh. |
| **S4 Trivial** | Cosmetic, copy, non-blocking. | Badge count flicker; empty-state wording. |

**Defect record must include:** id, title, spec+AC ref, TC id, severity, priority,
build/commit, environment, steps to reproduce, expected vs actual, evidence
(response body / log / screenshot), suspected area.

## 8. Non-functional targets (from specs §8 / frontend.md §8)

| Metric | Target | Source |
|--------|--------|--------|
| Image processing p95 (12 MP) | < 10 s | SPEC-01 §8 |
| Worker/API OOM kills w/ image processing live | **0** | SPEC-01 §8 |
| Heavy-queue concurrent decodes | ≤ configured (1–2) | SPEC-01 P0.1 |
| Library first page LCP (100 assets) | < 2.5 s | SPEC-01 P0.4 |
| Comic reader time-to-first-page (Fast 3G) | < 2 s | SPEC-02 §8 |
| Comic reader CLS | < 0.1 | SPEC-02 P0.3 |
| Quick-add expense (dialog→saved) | < 10 s | SPEC-03 §9 |
| Journal composer open→saved | < 10 s | SPEC-05 §8 |
| Home LCP (50-item stream) | < 2.5 s | SPEC-06 §8 |
| Video resume accuracy | ± 10 s of true position | SPEC-07 §8 |
| Password-reset email (dev, Mailpit) | < 5 s | SPEC-04 §8 |
| `media:asset_ready` → bell (poll) | ≤ 60 s; (SSE P1.2) < 10 s p95 | SPEC-04 §8 |
| Backup freshness | `hours_since_success` never > 26 (outside induced) | SPEC-09 §8 |
| Money on the wire | integer minor units, **never** floats/strings | SPEC-03 §7 (D-41) |

## 9. Cross-cutting conventions to assert everywhere (from specs README)

These are **global invariants**; each spec file references them rather than
repeating. A violation is at least S3, often S2/S1:

- **CC-1 RFC-7807:** every non-2xx returns `application/problem+json` with a stable
  `type` URI (which is also the i18n key). No bare 500 for expected errors.
- **CC-2 Permission grammar:** codes are 2–3 segments `<resource>:<action>[:<scope>]`;
  `rbac.Parse` rejects 4-segment; `AllowsCode` is fail-closed (even `*` returns
  false on malformed required code). A 2-seg grant satisfies bare/`:any` but **not**
  `:own`; `:own` matches only `:own`; ownership compare is the caller's job.
- **CC-3 Owner isolation:** a foreign id is **404, never 403** (existence never
  leaks). List endpoints never return another user's rows.
- **CC-4 Cursor pagination:** ordered per the spec (`occurred_at DESC, id DESC` etc.),
  stable across pages, no dupes/gaps under inserts/edits; `next_cursor` in body.
- **CC-5 Events:** every task/event name is registered in
  [events.md](../reference/events.md); publish is **after commit** (rollback →
  no emit); fan-out via `platform/events` (one Asynq task per consumer);
  **each emitting binary registers its own subscription edges** (api + worker) —
  an unregistered edge silently enqueues nothing. Redelivery is idempotent.
- **CC-6 Money:** integer minor units end-to-end (VND exponent 0); no floats anywhere.
- **CC-7 Migration-only schema:** DDL only via `backend/db/migrations`; generated
  `*.sql.go`/`api.gen.go`/`types.gen.ts` never hand-edited; CI drift gates green.
- **CC-8 Idempotent deletes:** DELETE on an already-gone id → 404, never 500.
- **CC-9 Frontend state ownership (D-32):** server state in TanStack Query, never
  Zustand; mutations optimistic with rollback; no fixture data on shipped routes.

## 10. Risk-based prioritization

Test effort concentrates where failure is most expensive on a single-operator,
irreplaceable-data box.

| Rank | Risk | Where | Mitigating tests |
|------|------|-------|------------------|
| R1 | **Money incorrectness** (transfer double-count, reconciliation drift) | SPEC-03 | reconciliation property test, transfer/fee leg-predicate, direction/kind guard |
| R2 | **Irreplaceable data loss** (backup never restored, delete strands storage) | SPEC-09, SPEC-01 | restore drill against real dump, sha256 verify, storage-gone-after-delete, janitor |
| R3 | **Cross-user data leak** (finance/journal/people are private) | all | owner-isolation 404 suite per module |
| R4 | **Stream/event correctness** (event never projected, dup items, resurrect) | SPEC-06 + producers | fan-out integration, idempotency, transfer collapse, birthday notice_id keying |
| R5 | **Account takeover** (reset token reuse, enumeration, session revocation) | SPEC-04, account | token single-use/expiry, enumeration-safe timing, `token_version` bump |
| R6 | **Worker OOM / stuck queue** | SPEC-01 | heavy-queue concurrency cap, dimension/size caps, stuck-purge error log |
| R7 | **Data-loss on capture** (journal/people not saved) | SPEC-05, SPEC-08 | persistence-across-restart, optimistic-then-refetch survives |

## 11. Automation vs manual

- **Automate (regression-critical):** L1 unit suites (already partly present:
  `rbac`, `password`, `media/service`, `people`, `journal`), L2 integration for
  event fan-out + projection idempotency + reconciliation property tests, L3 API
  contract (a runnable REST collection), the drift gates.
- **Manual (first pass / exploratory):** E2E journeys, frontend states, reader/CLS,
  Mailpit checks, restore drill, LCP measurements. Promote stable manual E2E to
  Playwright once the flows settle.
- Each test case file marks candidates with **[AUTO]** (should be automated) vs
  **[MANUAL]** where automation is impractical at v1.

## 12. Schedule / phasing (suggested execution order)

Mirrors the specs' dependency order so producers are tested before consumers.

1. **Foundations:** auth/RBAC conventions, `platform/events` fan-out (CC-2, CC-5).
2. **SPEC-01** media (shared bottleneck) → **SPEC-07** progress/continue.
3. **SPEC-05** journal → **SPEC-06** stream projection & home (consumes producers).
4. **SPEC-02** comic → **SPEC-03** bank (largest) → **SPEC-08** people.
5. **SPEC-04** notify (email/reset/bell) → **SPEC-09** ops (backup/restore/takeout).
6. **Full regression** of cross-module E2E + NFR budgets before v1 sign-off.

## 13. Assumptions & dependencies

- The stack runs via `make up`; migrations current; the six test accounts exist.
- Mailpit is reachable in dev for email cases (SPEC-04).
- Some P1/P2 features may be unbuilt — their cases read `➖ N/A` until landed; this
  is tracked, not a defect (see README "spec vs as-built").
- A controllable clock (or run-relative fixtures) is available for time cases.
- OpenAPI is **not** guaranteed to match handlers (known drift) — contract cases
  verify both directions.

## 14. Deliverables

- This plan + nine `TEST-CASES-SPEC-*.md` + the traceability matrix.
- A per-run execution report (Status columns filled, defects linked, NFR readings).
- Sign-off record per spec against §6.2.
