# Documentation Style Guide

**Status:** current · **Last verified:** 2026-09-11

Applies to everything under `docs/`. Kept deliberately short — rules that don't get
followed are worse than no rules.

## Language

English only ([ADR-09](adr/09-docs-architecture.md)).

## Every document starts with a status header

First lines after the title, so staleness is visible before content is trusted:

```
**Status:** draft | current | accepted (ADRs) | superseded by X | historical
**Last verified:** YYYY-MM-DD   ← the date the content was checked against the repo,
                                   not the date it was edited
```

`Last verified` is the **only** mark that a document has been checked against the
code ([ADR-11](adr/11-docs-canonicalisation.md)). No changelogs, no per-section
"update notes": correct the text, bump the date. The date means the **whole
file** was checked; a file nobody has checked as a whole says
`**Last verified:** never` — an honest `never` beats a date that means "last
edited". There is no per-section variant. CI checks that the field is present
(`scripts/check-doc-headers.sh`); it does not judge the value.

The four docs checks — links, headers, retired names, and the traceability
matrix's evidence cells (`scripts/check-matrix-evidence.py`) — run together as
the `link-check` job. Each is presence-only by design: a machine can prove a
citation resolves, not that a sentence is true.

Audits (`product/analysis/`) additionally carry `**Triaged:** YYYY-MM-DD →
backlog.md` once their findings have been turned into backlog lines. Header
lines and link targets are the only things ever edited in an audit.

Documents describing implementation state must defer to [`/CLAUDE.md`](../CLAUDE.md)
§ "Current status" rather than restate it — a pointer beats a table that rots.

## Naming

- Folders and files: `kebab-case.md`. ADRs: `NN-kebab-title.md`, two-digit,
  monotonic, never reused (`00` is retired — see ADR-11).
- Specs: `SPEC-NN-kebab-title.md`, numbered by intended build order.
- Briefs: `NN-kebab-title.md` within `product/briefs/`.
- Audits in `product/analysis/`: `topic-YYYY-MM-DD.md` — the one place a date
  belongs in a filename, because the date *is* the identity of an audit. The
  genre is called *audit* everywhere, whatever the file calls itself.
- Otherwise no spaces and no dates in filenames; dates live in status headers.

## Linking

- Relative links only, within the repo. Link to a file, not a folder, when a
  specific document is meant. CI checks every relative link resolves.
- Cite decisions by ID: `[D-27]` for feature-inventory decisions,
  `ADR-06` for architecture decisions. IDs are stable even when files move.
- When a claim depends on repo state (migration numbers, endpoint existence,
  how many of something there are), write the **command that answers it**
  rather than the number: `find backend -name '*_test.go' | wc -l` stays true;
  "14 test files" was wrong within a month.
- A document that no longer exists is cited by its **deleting commit**
  (`` `MILESTONE_CHECKS.md` (deleted in `f11cf3f`) ``), never linked. CI
  (`scripts/check-retired-names.sh`) refuses a retired name on a line that
  carries no such cue — "deleted in", "then `…`", "renamed", "retired", a
  `§` citation into a renamed document. Inside an ADR only the fact layer is
  checked; Decision, Options and Trade-offs may name what they knew.

## ADR shape (binding)

`context → decision → options considered → trade-offs → consequences → action
items`.

An accepted ADR is **corrected in place, by layer** ([ADR-11](adr/11-docs-canonicalisation.md)):

- **Fact clauses** — what currently exists, paths, counts, action-item boxes —
  are rewritten to be true, and `Last verified` is bumped.
- **Decision narrative** — *Options considered*, *Trade-offs*, the reasoning
  behind the choice — is kept verbatim. It records what was known at the time.

A decision that is *reversed* gets a new ADR that supersedes the old one; the old
one's header gains `superseded by <link>`. `D-26.r1`-style revision notes remain
the house pattern for feature-inventory decisions.

## Formatting

- Markdown, no HTML unless unavoidable. Mermaid for diagrams (house standard,
  see architecture/diagrams.md).
- Tables for mappings and inventories; prose for reasoning. Acceptance criteria in
  Given/When/Then or checklists.
- Line length: don't fight it; wrap around ~90–100 chars for reviewable diffs.

## Lifecycle

- New doc → `draft` in the right section (which section: [README.md](README.md)
  § Genre rules).
- Content merged/actioned → `current` (or `accepted` for ADRs).
- Replaced → header gains `superseded by <link>`.
- No longer maintained → **delete it**, and cite the deleting commit wherever it
  is still mentioned. Git history is the archive; there is no `archive/` folder,
  and there will not be one.
