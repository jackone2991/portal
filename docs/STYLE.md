# Documentation Style Guide

Applies to everything under `docs/`. Kept deliberately short — rules that don't get
followed are worse than no rules.

## Language

English only ([ADR-09](adr/09-docs-architecture.md)). `docs/archive/vi-2026-07/`
is frozen history — never edit it.

## Every document starts with a status header

First lines after the title, so staleness is visible before content is trusted:

```
**Status:** draft | current | accepted (ADRs) | superseded by X | historical
**Last verified:** YYYY-MM-DD   ← the date the content was checked against the repo,
                                   not the date it was edited
```

Documents describing implementation state must defer to `/MILESTONE_CHECKS.md`
rather than restate it ("see MILESTONE_CHECKS for status" beats a table that rots).

## Naming

- Folders and files: `kebab-case.md`. ADRs: `NN-kebab-title.md`, two-digit,
  monotonic, never reused.
- Specs: `SPEC-NN-kebab-title.md`, numbered by intended build order.
- Briefs: `NN-kebab-title.md` within `product/briefs/`.
- No spaces, no dates in filenames (dates live in status headers); exception:
  archive folders are date-stamped (`vi-2026-07`).

## Linking

- Relative links only, within the repo. Link to a file, not a folder, when a
  specific document is meant.
- Cite decisions by ID: `[D-27]` for feature-inventory decisions,
  `ADR-06` for architecture decisions. IDs are stable even when files move.
- When a claim depends on repo state (migration numbers, endpoint existence),
  write it as verifiable ("next free sequence — verify against the repo") rather
  than asserting a number that will rot.

## Genre discipline

One document, one genre (decision / brief / spec / design / guide / reference).
If a spec starts accumulating decision rationale with alternatives, extract an ADR
and link it. If an ADR starts specifying endpoints, move that into a spec.

## ADR shape (binding)

`context → decision → options considered → trade-offs → consequences → action
items`. Accepted ADRs are immutable: correct them by a dated addendum note or a
superseding ADR (`D-26.r1`-style revision notes are the existing house pattern —
keep it).

## Formatting

- Markdown, no HTML unless unavoidable. Mermaid for diagrams (house standard,
  see architecture/diagrams.md).
- Tables for mappings and inventories; prose for reasoning. Acceptance criteria in
  Given/When/Then or checklists.
- Line length: don't fight it; wrap around ~90–100 chars for reviewable diffs.

## Lifecycle

- New doc → `draft` in the right section.
- Content merged/actioned → `current` (or `accepted` for ADRs).
- Replaced → header gains `superseded by <link>`; move to `archive/` only when it
  no longer needs to be discoverable in place.
