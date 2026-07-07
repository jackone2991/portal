# Product

**Status:** current · **Last verified:** 2026-07-07

Why Portal exists, what it is, and what gets built in what order.

| Document | Genre | Role |
|---|---|---|
| [vision.md](vision.md) | positioning | What Portal is (life OS) — the yardstick that replaced Facebook parity |
| [feature-inventory.md](feature-inventory.md) | decision log | Canonical per-module feature inventory + decisions `D-1`…`D-40` |
| [backlog.md](backlog.md) | living analysis | Gap analysis / backlog (was `missing-features.md`); ordering now defers to briefs + specs |
| [checklist.md](checklist.md) | living checklist | Phase 0–12 deliverables tied to `D-N` IDs |
| [analysis/facebook-comparison.md](analysis/facebook-comparison.md) | historical | The old parity yardstick — superseded by vision.md ([ADR-08](../adr/08-life-os-pivot.md)) |
| [briefs/](briefs/) | briefs | Brainstorm-level "what & why" per feature (00–04, from 2026-07-07) |
| [specs/](specs/README.md) | specs/PRDs | Implementation-ready: SPEC-01 media images · SPEC-02 comic · SPEC-03 finance ledger |

## The pipeline

An idea moves left to right, gaining precision and shedding ambiguity:

```
brainstorm → briefs/NN-*.md → specs/SPEC-NN-*.md → code + MILESTONE_CHECKS.md
                    ↑ decisions worth recording → ../adr/ or D-N entries
```

A brief answers *should we, and roughly what*. A spec answers *exactly what,
with acceptance criteria*. Implementation status never lives here — it lives in
[`/MILESTONE_CHECKS.md`](../../MILESTONE_CHECKS.md).

## Current build order (per ADR-08)

SPEC-01 (media image pipeline) → SPEC-02 (comic vertical) → SPEC-03 (finance
ledger) → notification module (life-stream backbone; brief to be written).
Everything consciously postponed, with re-entry conditions:
[briefs/04-deferred.md](briefs/04-deferred.md).
