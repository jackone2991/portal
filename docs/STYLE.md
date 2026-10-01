# Documentation Style Guide

**Status:** current · **Last verified:** 2026-10-01

Applies to everything under `docs/`. Kept deliberately short — rules that don't get
followed are worse than no rules.

## Language

English only ([ADR-09](product/specs/SPEC-06-docs-canonicalisation.md#adr-09)).

## Every document starts with a status header

First lines after the title, so staleness is visible before content is trusted:

```
**Status:** draft | current | superseded by X | historical
**Last verified:** YYYY-MM-DD   ← the date the content was checked against the repo,
                                   not the date it was edited
```

`Last verified` is the **only** mark that a document has been checked against the
code ([ADR-11](product/specs/SPEC-06-docs-canonicalisation.md#adr-11)). No changelogs, no per-section
"update notes": correct the text, bump the date. The date means the **whole
file** was checked; a file nobody has checked as a whole says
`**Last verified:** never` — an honest `never` beats a date that means "last
edited". There is no per-section variant — and so no per-record one: a decision
record inside a spec is covered by that spec's header. CI checks that the field is present
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

- Folders and files: `kebab-case.md`.
- Decision records have IDs, not files: `ADR-NN`, two-digit, monotonic, never
  reused (`00` is retired — see ADR-11). The next new record is **ADR-12**.
- Specs: `SPEC-NN-kebab-title.md`, numbered by build priority — platform and
  configuration first, then features in dependency order (renumbered on
  2026-10-01; the old→new table is [specs/README.md § Renumbering](product/specs/README.md#renumbering-2026-10-01)).
- Audits in `product/analysis/`: `topic-YYYY-MM-DD.md` — the one place a date
  belongs in a filename, because the date *is* the identity of an audit. The
  genre is called *audit* everywhere, whatever the file calls itself.
- Otherwise no spaces and no dates in filenames; dates live in status headers.

## Linking

- Relative links only, within the repo. Link to a file, not a folder, when a
  specific document is meant. CI checks every relative link resolves.
- Cite decisions by ID: `[D-27]` for feature-inventory decisions,
  `ADR-06` for decision records. IDs are stable even when files move; a linked
  `ADR-NN` points at its record's anchor —
  `[ADR-06](product/specs/SPEC-01-account-identity-admin.md#adr-06)` — and
  [specs/README.md § Decision records](product/specs/README.md#decision-records)
  says which file holds each ID.
- When a claim depends on repo state (migration numbers, endpoint existence,
  how many of something there are), write the **command that answers it**
  rather than the number: `find backend -name '*_test.go' | wc -l` stays true;
  "14 test files" was wrong within a month.
- A document that no longer exists is cited by its **deleting commit**
  (`` `MILESTONE_CHECKS.md` (deleted in `f11cf3f`) ``), never linked. CI
  (`scripts/check-retired-names.sh`) refuses a retired name on a line that
  carries no such cue — "deleted in", "then `…`", "renamed", "retired", a
  `§` citation into a renamed document. Inside a decision record only the fact
  layer is checked: the narrative spans between the `adr-narrative` markers
  (see § Decision records) may name what they knew.

## Decision records (binding)

There is no `adr/` folder (folded into the specs on 2026-10-01 —
[ADR-09](product/specs/SPEC-06-docs-canonicalisation.md#adr-09) § Consequences).
A decision record lives **inside the spec that owns its subject**, or in
[product/specs/README.md](product/specs/README.md) when it governs the corpus
as a whole (scope, positioning, the API contract). The index of every record —
ID → file — is [specs/README.md § Decision records](product/specs/README.md#decision-records);
a new record gets a row there.

**When to write one.** A choice that (a) is expensive to reverse, (b) crosses
module boundaries, or (c) contradicts a previous record or the v1 scope cut.
Day-to-day feature decisions belong in
[product/feature-inventory.md](product/feature-inventory.md) as `D-N` entries;
specs cite both kinds by ID. A spec that starts accumulating rationale with
alternatives grows a record; a record that starts specifying endpoints moves
that into the spec body.

**Placement and shape.** An unnumbered `## Decision records` section after the
spec's last numbered section (existing `§N` numbers never shift). Each record:

```
<a id="adr-12"></a>
### ADR-12 — Title
**Decided:** YYYY-MM-DD · **Status:** accepted[, amended by ADR-NN][, executed][, superseded by ADR-NN]
#### Context
#### Decision
#### Options considered
#### Trade-offs
#### Consequences
#### Action items
```

The anchor is fixed (`adr-` + the two-digit number) and never changes, so a
link to `…#adr-12` survives the record moving to another spec. The shape
`context → decision → options considered → trade-offs → consequences → action
items` is binding; a record may add an *As built* section to its fact layer.

**Corrected in place, by layer** ([ADR-11](product/specs/SPEC-06-docs-canonicalisation.md#adr-11)):

- **Fact layer** — Context as "the state this was decided against", *As built*,
  Consequences as what actually followed, action items — is rewritten to be
  true, and the spec's `Last verified` is bumped. Executed action items may be
  compressed into one sentence each.
- **Narrative layer** — Decision, *Options considered*, *Trade-offs* — is kept
  verbatim. It records what was known at the time. It sits between two HTML
  comments, each alone on its line — the opening `adr-narrative` comment
  before the Decision heading, the closing `/adr-narrative` one after the
  Trade-offs (`<!-- adr-narrative -->` … `<!-- /adr-narrative -->`); the
  retired-names check skips exactly those spans. A record whose whole point is
  naming retired things (ADR-11) is wrapped entire.

A decision that is *reversed* gets a **new record** (the next free ID) that
supersedes the old one; the old one's Status line gains `superseded by ADR-NN`
with a link, and its body stays. `D-26.r1`-style revision notes remain the
house pattern for feature-inventory decisions.

## Formatting

- Markdown, no HTML unless unavoidable — the decision-record anchor and
  narrative markers are the standing exceptions. Mermaid for diagrams (house standard,
  see architecture/diagrams.md).
- Tables for mappings and inventories; prose for reasoning. Acceptance criteria in
  Given/When/Then or checklists.
- Line length: don't fight it; wrap around ~90–100 chars for reviewable diffs.

## Lifecycle

- New doc → `draft` in the right section (which section: [README.md](README.md)
  § Genre rules).
- Content merged/actioned → `current` (a decision record's own Status line says
  `accepted`).
- Replaced → header gains `superseded by <link>`.
- No longer maintained → **delete it**, and cite the deleting commit wherever it
  is still mentioned. Git history is the archive; there is no `archive/` folder,
  and there will not be one.
