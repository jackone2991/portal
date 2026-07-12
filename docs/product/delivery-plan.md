# Portal — Delivery Plan (spec set SPEC-01…09)

**Author:** PO working plan · **Date:** 2026-07-11 · **Horizon:** post‑v1 expansion (the v1 demo loop is already closed — see [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md)).
**Scope source:** the 9 implementation-ready specs in [specs/](specs/) and their declared dependencies / effort. This plan sequences them; it does not re-decide them.
**Companion:** the verified fix pass that made these specs build-ready — [analysis/spec-gap-fix-worklog-2026-07-11.md](analysis/spec-gap-fix-worklog-2026-07-11.md).

> **Decisions locked (2026-07-11):** ✅ **Foundation gate first** — accept + execute ADR-10, then `platform/events` + shared scheduler, before feature endpoints. ✅ **Life‑OS milestone order** (M1→M8 as below; comic/SPEC-02 stays at M6). Still open: email transport for prod (decision §7.3), the D-41 money-model entry (§7.4), cadence (§7.5).

---

## 1. Framing (read first)

- **This is a ~10–12 week solo program, not a sprint.** The 9 specs total **≈ 45.5 P0 dev-days** of *design-only* effort (the specs' own §9/§10 numbers). With the real overhead a single dev carries — review, bugfix, dogfooding, ops, contract/i18n/events DoD — plan on **~1.3×**, i.e. **≈ 60 effective dev-days ≈ 3 months** of sustainable solo pace. P1 tails add ~13 more.
- **One developer ⇒ milestones are serial.** The specs say "parallelizable" in a few places; that assumes >1 engineer. Here the critical path *is* the schedule.
- **Deliver a usable increment every milestone.** Each milestone below closes with something the owner can actually use that day, and a "dogfood before you extend" gate. Several specs make this explicit (SPEC-03: *do not start P1 before the first reconciliation succeeds*).
- **The product is a life‑OS ([ADR-08](../adr/08-life-os-pivot.md)), not a media portal.** That biases priority toward life facets (money, journal, people, data-safety) over media-content verticals (comic). Reflected in the ordering — and called out as a decision in §7.

---

## 2. Critical path (dependency-honest ordering)

```
M0  Foundation gate ─────────────┐  (ADR-10 codegen + platform/events + shared scheduler)
                                 ▼
M1  SPEC-01 Media images ──┬─────────────┬──────────────┬────────────┐
                           ▼             ▼              ▼            ▼
M2 SPEC-04 Notify+reset   M6 SPEC-02   M5b SPEC-03    M5a SPEC-07  M7 SPEC-08
   (consumes asset_ready)    Comic       receipts(P1)   comic leg     avatars(P1)
                                 (needs M6)
M3  SPEC-05 Journal ──────────────────────────────────────────────► M8 SPEC-06 (LAST)
M4  SPEC-09 P0 Backups  ── MUST precede ──►  M5b SPEC-03 (ledger data)   ▲
                                                                          │
   every producer above feeds M6 SPEC-06's stream/widgets ───────────────┘
```

**Hard rules encoded above:** SPEC-01 unlocks everything with images · SPEC-04 P0.4 consumes `media:asset_ready` (needs SPEC-01 P1.2 or a coordinated one-line emit) · **SPEC-09 P0 backups land BEFORE SPEC-03 accrues irreplaceable ledger data** · SPEC-05 before SPEC-06 · **SPEC-06 is last** (it consumes every producer; each widget degrades to an empty state until its producer ships).

---

## 3. Foundation gate — M0 (do before any feature spec)

These are cross-cutting prerequisites the specs assume exist. The review confirmed **none of them exist yet**. They are cheap and unblock everything.

| Item | Why it gates | Effort | Where it's specced |
|------|--------------|:--:|--------|
| **Execute ADR-10** — spec-first: commit `make openapi` output + CI drift gate so `shared/openapi.yaml` ↔ handlers can't diverge | Every new endpoint family should be *born* under the contract. ADR-10 is still **"proposed"** — accept + implement it first, or accumulate drift on 9 specs' worth of new endpoints. | ~1.5–2 d | [ADR-10](../adr/10-openapi-contract-direction.md), README "API contract" |
| **`platform/events`** fan-out helper (`Publish` + event‑name→consumer‑task subscription registry in `cmd/worker`) | The multi-consumer stream/notify architecture depends on it; naive subscribe-by-task-type panics `cmd/worker` once an event gets a 2nd consumer. Owned by **SPEC-01 P0.6**. | ½ d | SPEC-01 §9.2 |
| **Shared `asynq.Scheduler`** single registration point in `cmd/worker` | Backups, birthday scan, notify/journal janitors, refresh-token purge all ride it. Owned by **SPEC-01 P0.3**. | ¼ d | SPEC-01 §9.3 |
| **Convention plumbing** (land as first module uses them): RBAC seed-row migration pattern (`WITH grants(...)`), i18n Problem-type catalog stub (`frontend.md §5`), `middleware.ts` auth-gate matcher discipline, template-registry (`TemplateManifest.views`) discipline | The spec-gap review found each of these silently assumed. Establish the pattern once in M1 so M2+ copy it. | folded into M1 | README Conventions |

> M0's first three items are literally SPEC-01's opening phases, so **M0 folds into the front of M1** — but treat "contract gate accepted" as an explicit go/no-go before writing endpoint code.

---

## 4. Milestones

Each: **Outcome** (what the owner can do) · **Effort (P0 dev-days)** · **Key risks** · **Dogfood gate**.
Recurring Definition of Done is in §6 — every milestone inherits it.

### M1 — Media images are real  ·  SPEC-01  ·  ~5.75 d
- **Outcome:** upload an image → variants (thumb/medium/poster) rendered → library grid with download + delete → orphan janitor. Unblocks **avatars, journal photos, receipts, comic pages** — the shared bottleneck.
- **Includes M0** (events + scheduler + contract gate).
- **Risks:** heavy-queue concurrency vs image throughput (already tuned in spec); HEIC/unsupported handling; the RBAC `RequireOwnerOrPermission` DELETE pattern (set the house style here — every later module copies it).
- **Dogfood gate:** upload 20 real photos, delete some, confirm variants + library before moving on.

### M2 — Notifications backbone + password reset ships  ·  SPEC-04  ·  ~6 d
- **Outcome:** in-app bell (real, fixture removed) + email channel via Mailpit + **`forgot/reset-password` actually works** (today it's admin/CLI only — a real product gap). First `media:asset_ready` consumer.
- **Risks:** email transport choice for prod (decision §7); abuse controls (per-email/IP throttle + global ceiling) are load-bearing for a public `forgot` endpoint — don't skip.
- **Dogfood gate:** reset your own password end-to-end through Mailpit.

### M3 — First life-stream write path  ·  SPEC-05 journal  ·  ~4.5 d
- **Outcome:** create/read journal entries; composer wired on home; the first **real post type** replaces part of the fixture feed. Emits `journal:entry_created` (emit-only; projection maintained transactionally for M8).
- **Risks:** the emit-only-vs-projection boundary (fixed in the review — keep projection writes transactional, bus event for future external consumers).
- **Dogfood gate:** journal for 5 consecutive days; it should feel lighter than a note app.

### M4 — Data safety before money  ·  SPEC-09 P0  ·  ~3.75 d  ·  ⚠ ordering-critical
- **Outcome:** **backups that run themselves** (nightly `pg_dump` → storage, retention, `LATEST.json` manifest) + a **rehearsed restore drill** (`make` target + runbook) + `/ops/status`. This is the honesty tax of asking the owner to put finances on self-hosted hardware.
- **Why here:** must land **before SPEC-03 accrues irreplaceable ledger data**. Non-negotiable sequencing.
- **Risks:** the restore drill is the actual deliverable — a backup you've never restored is not a backup. Budget the full restore rehearsal.
- **Dogfood gate:** wipe a throwaway DB and restore it from the manifest. If that's painful, stop and fix before M5.

### M5 — Money + Continue rail  ·  SPEC-03 (8.5 d) + SPEC-07 (3 d)  ·  ~11.5 d
- **M5a SPEC-07 (do first, 3 d):** resume-where-you-left-off for video (the missing GET progress read-path was added in the fix pass). The nightly-retention surface; small and high-value. Comic leg joins after M6.
- **M5b SPEC-03 (8.5 d — the largest spec):** Money-Lover-class ledger — accounts, transactions, hierarchical categories, monthly budgets, inter-account transfers, import-ready schema. Requires the **D-41 money-model decision entry** (integer minor units vs D-14) recorded first.
- **Risks:** category delete/reassign matrix and the budget roll-up are where it grew past estimate; transfer paired-leg semantics (counterparty normalization was fixed in review). **Reconciliation is the acceptance test** — spec says do not start any P1 until month-end reconciliation closes within rounding.
- **Dogfood gate:** log real transactions for 20 of 30 days; reconcile against a real account.

### M6 — Comic vertical (reference implementation)  ·  SPEC-02  ·  ~4.5 d P0 (+2 P1)
- **Outcome:** comic reader + vertical progress + library replacing placeholders + zip chapter import. Doubles as the **canonical `media → domain vertical` pattern** (migration → query → repo → service/handler → MountHTTP → real view) that movie/music/story later copy.
- **Priority note:** for a life‑OS owner this is lower-value than M3/M5/M7; its main worth is as the template. Candidate to defer (decision §7).
- **Dogfood gate:** import one real chapter zip, read it on mobile viewport.

### M7 — People registry (contacts + birthdays)  ·  SPEC-08  ·  ~4.5 d
- **Outcome:** store people + birthdays; `nextOccurrence` (TZ + Feb-29 + no-year); daily scan → `people:birthday_upcoming`. Makes brief 00's flagship "mom's birthday in 3 days" finally possible.
- **Risks:** timezone/leap-day correctness (table-driven tests are in-spec); avatars (P1) need M1.
- **Dogfood gate:** enter real family birthdays; confirm the upcoming list + emitted notice.

### M8 — Life-stream home (the proof screen)  ·  SPEC-06  ·  ~5 d  ·  LAST
- **Outcome:** the real `/` — a merged stream projection + widget rail (finance, birthdays, continue, notifications) replacing every fixture. Consumes **all** producers above; each widget was degrading to an empty state until its producer shipped, so this is where the life-OS thesis becomes visible.
- **Risks:** projection idempotency under at-least-once delivery (backfill flood-guard + transfer-collapse fixed in review); it's a synthesis milestone — its quality reflects everything before it.
- **Dogfood gate:** open `/` after a week of real use across modules; it should read like *your* day.

---

## 5. Effort & calendar summary

| Milestone | Spec(s) | P0 dev-days | P1 tail |
|-----------|---------|:--:|:--:|
| M0/M1 | SPEC-01 (+foundation) | 5.75 | +0.5 |
| M2 | SPEC-04 | 6 | +2 |
| M3 | SPEC-05 | 4.5 | +1 |
| M4 | SPEC-09 P0 | 3.75 | +5 (takeout) |
| M5a | SPEC-07 | 3 | +0.5 |
| M5b | SPEC-03 | 8.5 | — |
| M6 | SPEC-02 | 4.5 | +2 |
| M7 | SPEC-08 | 4.5 | +1 |
| M8 | SPEC-06 | 5 | +1.5 |
| **Total P0** | | **≈ 45.5 d** | **≈ +13.5 d** |

**Realistic calendar (solo, ~1.3× overhead):** ≈ 60 effective dev-days ≈ **~12 working weeks / ~3 months** for the full P0 set. First usable increment (M1) inside **~1.5 weeks**; password reset (M2) by **~week 3**; data-safe money tracking (through M5) by **~week 8**.

---

## 6. Definition of Done — applies to every milestone

1. **Contract:** endpoints in `shared/openapi.yaml`; codegen runs; CI drift gate green (once M0 lands).
2. **Events:** every task/event registered in [reference/events.md](../reference/events.md).
3. **i18n:** every new Problem `type` URI has an i18n key in the frontend catalog (same PR).
4. **Migrations:** real sequential `000N_<module>_*.up/down.sql` (assign at build time — specs use `000N` placeholders and several claim numbers concurrently); up→down→up roundtrip passes (CI job exists).
5. **RBAC:** every endpoint names its permission; the introducing spec ships the `permissions` + `role_permissions` seed rows in its own migration.
6. **Tests:** unit + contract; module isolation respected (depguard/CI).
7. **Tracker:** update [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md).
8. **Dogfood:** the milestone's gate above is met before starting the next.

---

## 7. Decisions needed before Sprint 1

1. ✅ **DECIDED (2026-07-11) — accept + execute ADR-10 first.** Cross-cutting gate; ~2 d up front, before feature endpoints.
2. ✅ **DECIDED (2026-07-11) — life‑OS order; comic (SPEC-02) stays at M6.** Money/journal/people/data-safety ahead of the media-content vertical.
3. **Email transport for prod (SPEC-04 §10).** Mailpit is the dev choice; prod = SMTP vs an API provider (budget/${free-tier} implications). Needed before M2 ends.
4. **Record D-41** (money-model divergence: integer minor units vs D-14) — a prerequisite of SPEC-03 step 1; decide the decision-entry wording before M5b.
5. **Cadence assumption:** confirm single developer, sustainable pace (drives the ~3-month calendar). If more capacity exists, M2/M3 and M5a/M7 can parallelize.

---

## 8. Recommended Sprint 1 (first ~2 weeks)

**Goal: the foundation gate + the shared bottleneck, shipped and dogfooded.**
1. Accept ADR-10; commit codegen output + CI drift gate. *(decision 1)*
2. SPEC-01 P0.6 `platform/events` + P0.3 shared `asynq.Scheduler`.
3. SPEC-01 image pipeline: migration → `media:process_image` → variants → library page → DELETE (`RequireOwnerOrPermission` — set the house pattern) → janitor.
4. Establish the convention plumbing (RBAC seed migration, i18n Problem stub, matcher + template-registry discipline) so M2+ copy it.
5. **Exit criteria:** upload/download/delete real images; contract gate green; events + scheduler live. → unblocks M2–M8.

---

## 9. Top risks

| Risk | Milestone | Mitigation |
|------|:--:|-----------|
| Data loss before backups exist | M4 vs M5 | Hard-ordered: SPEC-09 P0 **before** SPEC-03. Restore drill is the deliverable, not the dump. |
| Contract drift across 9 specs | M0 | Execute ADR-10 first; CI gate. |
| `cmd/worker` panic on 2nd consumer of an event | M2/M8 | `platform/events` fan-out (M0) before any multi-consumer event. |
| Scope creep on the biggest spec (SPEC-03, 8.5 d) | M5b | Reconciliation is the gate; no P1 until it closes within rounding. |
| Solo-pace fatigue / 3-month horizon | all | Ship a usable increment every milestone; dogfood gates keep motivation and catch model errors early. |
| Estimates are design-only | all | ~1.3× overhead already applied in §5; re-baseline after M1 actuals. |

---

*Living document — re-baseline effort against actuals after M1. Sequencing rules in §2 are load-bearing; the milestone **contents** are fixed by the specs, only their **order/priority** is a PO lever (see §7).*
