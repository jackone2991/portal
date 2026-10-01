# Product

**Status:** current · **Last verified:** 2026-09-11

Why Portal exists, what it is, and what gets built in what order.

| Document | Genre | Role |
|---|---|---|
| [vision.md](vision.md) | positioning | What Portal is (life OS) — the yardstick that replaced Facebook parity |
| [feature-inventory.md](feature-inventory.md) | decision log | Canonical per-module feature inventory + decisions `D-1`…`D-41` |
| [backlog.md](backlog.md) | living backlog | Triaged open work, P0 first; every accepted audit feeds it (ADR-11). Replaced the 2026-07 gap analysis on 2026-09-11. § Deferred is the parking lot, with re-entry conditions |
| [analysis/](analysis/) | audits | Dated, immutable point-in-time reviews; a superseded, fully triaged audit is deleted (git history keeps it). Today: [spec-gap-fix-worklog-2026-09-30.md](analysis/spec-gap-fix-worklog-2026-09-30.md). Read the newest before trusting anything else here |
| [specs/](specs/README.md) | specs/PRDs | Implementation-ready SPEC-01…12 (`ls specs/`) |

## The pipeline

An idea moves left to right, gaining precision and shedding ambiguity:

```
brainstorm → specs/SPEC-NN-*.md → code (status: /CLAUDE.md § Current status)
      ↑ decisions worth recording → ../adr/ or D-N entries
```

A spec answers *exactly what, with acceptance criteria*; its opening sections
(problem, goals, non-goals) carry the *should we, and roughly what* that a
separate brief used to hold. The briefs of 2026-07 were folded into their specs
and deleted (`git show ea100d8:docs/product/briefs/`). Implementation status
never lives here — it lives in the code, described once in `/CLAUDE.md`
§ Current status (ADR-11); what is still open lives in [backlog.md](backlog.md).

## Current build order (per ADR-08)

SPEC-01 (media image pipeline) → SPEC-02 (comic vertical) → SPEC-03 (finance
ledger) → SPEC-04 (notifications) → 05/06 (journal + life stream) → 07 → 08
(people) → 09 (ops) → SPEC-10 (ledger expansion, phased) — all through SPEC-10
phase 1 have shipped; `ls specs/` is the list, [backlog.md](backlog.md) says what
is left in each. Everything consciously postponed, with re-entry conditions:
[backlog.md § Deferred](backlog.md).
