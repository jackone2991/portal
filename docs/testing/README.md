# Portal — Test Documentation

This tree is the **QA source of truth** for the Portal v1 platform. It is written
to be executable later by a tester who did not build the features — every test
case carries preconditions, steps, test data, and an unambiguous expected result.

**Language:** English only, matching the repo doc policy ([ADR-09](../adr/09-docs-architecture.md)).

## Layout

| File | What it is |
|------|-----------|
| [TEST-PLAN.md](TEST-PLAN.md) | Master test plan — scope, strategy, levels, environments, entry/exit criteria, risk model, defect workflow, roles, schedule. **Read this first.** |
| [TRACEABILITY-MATRIX.md](TRACEABILITY-MATRIX.md) | Requirement (`SPEC-NN Px.y`) ↔ test-case-ID map. Coverage gate. |
| [TEST-CASES-SPEC-01-media.md](TEST-CASES-SPEC-01-media.md) | `media` — image pipeline, poster, delete/janitor, library, download-original, events |
| [TEST-CASES-SPEC-02-comic.md](TEST-CASES-SPEC-02-comic.md) | `comic` — CRUD, publish/RBAC, reader, progress, asset-deletion coupling, zip import |
| [TEST-CASES-SPEC-03-bank.md](TEST-CASES-SPEC-03-bank.md) | `bank` — accounts, transactions, transfers, categories, budgets, dashboard, events |
| [TEST-CASES-SPEC-04-notify.md](TEST-CASES-SPEC-04-notify.md) | `notify` — store/read, dispatch fan-out, email + password reset, abuse controls, bell |
| [TEST-CASES-SPEC-05-journal.md](TEST-CASES-SPEC-05-journal.md) | `journal` — entries CRUD, ordering, event emit, composer, sanitization |
| [TEST-CASES-SPEC-06-stream.md](TEST-CASES-SPEC-06-stream.md) | `journal`/stream — projection consumers, read API, home replacement, widget rail |
| [TEST-CASES-SPEC-07-continue.md](TEST-CASES-SPEC-07-continue.md) | `media` — playback progress beacon, `/continue` aggregator, resume UX, completion event |
| [TEST-CASES-SPEC-08-people.md](TEST-CASES-SPEC-08-people.md) | `people` — CRUD, birthday validation, upcoming endpoint, scan/dedup/outbox |
| [TEST-CASES-SPEC-09-ops.md](TEST-CASES-SPEC-09-ops.md) | `ops` — nightly backup, retention, restore drill, freshness sentinel, queue console, takeout |

## How to use

1. Read [TEST-PLAN.md](TEST-PLAN.md) §4 (environments) and §7 (test data) and set
   up the environment + fixtures once.
2. Pick a spec file. Each is grouped by requirement (`P0.1`, `P0.2`, …). Execute
   top-to-bottom — earlier cases seed data that later cases reuse.
3. Record the result in the **Status** column of each case (`PASS` / `FAIL` /
   `BLOCKED` / `N/A`), the build/commit under test, and link any defect id.
4. A spec is **signed off** when every `P0`/`Critical` case is `PASS` (or has an
   accepted, documented deviation) — see [TEST-PLAN.md](TEST-PLAN.md) §6.

## Status legend (used in every case's Status column)

`☐ Not run` · `✅ PASS` · `❌ FAIL` · `⛔ BLOCKED` · `➖ N/A (feature not built)`

## Scope note — spec vs as-built

These cases are written against the **specs** as the requirement source of truth.
Several `P1`/`P2` items (email/reset, web-push, SSE, zip import, takeout, queue
console, nightly backup) may not be implemented yet — those cases are tagged
`[P1]`/`[P2]` and are expected to read `➖ N/A` until their feature lands.
Handlers are hand-written and can drift from `shared/openapi.yaml` (see
[CLAUDE.md](../../CLAUDE.md) "Known drift") — every spec file therefore includes a
**contract/drift** check section.
