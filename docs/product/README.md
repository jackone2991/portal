# Product

**Status:** current · **Last verified:** 2026-09-11

Why Portal exists, what it is, and what gets built in what order.

| Document | Genre | Role |
|---|---|---|
| [vision.md](vision.md) | positioning | What Portal is (life OS) — the yardstick that replaced Facebook parity |
| [feature-inventory.md](feature-inventory.md) | decision log | Canonical per-module feature inventory + decisions `D-1`…`D-41` |
| [backlog.md](backlog.md) | living backlog | Triaged open work, P0 first; every accepted audit feeds it (ADR-11). Replaced the 2026-07 gap analysis on 2026-09-11 |
| [analysis/facebook-comparison.md](analysis/facebook-comparison.md) | historical | The old parity yardstick — superseded by vision.md ([ADR-08](../adr/08-life-os-pivot.md)) |
| [analysis/](analysis/) | audits | Dated, immutable point-in-time reviews — newest first: `remaining-work-2026-08-25.md`, `spec-gap-fix-worklog-2026-07-11.md`, `architecture-review-2026-05-24.md`. Read the newest before trusting anything else here |
| [briefs/](briefs/) | briefs | Brainstorm-level "what & why" per feature (00–04, from 2026-07-07) |
| [specs/](specs/README.md) | specs/PRDs | Implementation-ready SPEC-01…10 (`ls specs/`) |

## The pipeline

An idea moves left to right, gaining precision and shedding ambiguity:

```
brainstorm → briefs/NN-*.md → specs/SPEC-NN-*.md → code (status: /CLAUDE.md § Current status)
                    ↑ decisions worth recording → ../adr/ or D-N entries
```

A brief answers *should we, and roughly what*. A spec answers *exactly what,
with acceptance criteria*. Implementation status never lives here — it lives in
the code, described once in `/CLAUDE.md` § Current status (ADR-11); what is
still open lives in [backlog.md](backlog.md).

## Current build order (per ADR-08)

SPEC-01 (media image pipeline) → SPEC-02 (comic vertical) → SPEC-03 (finance
ledger) → SPEC-04 (notifications) → 05/06 (journal + life stream) → 07 → 08
(people) → 09 (ops) → SPEC-10 (ledger expansion, phased) — all through SPEC-10
phase 1 have shipped; `ls specs/` is the list, [backlog.md](backlog.md) says what
is left in each. Everything consciously postponed, with re-entry conditions:
[briefs/04-deferred.md](briefs/04-deferred.md).
