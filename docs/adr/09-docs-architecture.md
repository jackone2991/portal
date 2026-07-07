# ADR-09 — Documentation Architecture

**Status:** proposed (drafted 2026-07-07)
**Supersedes:** the "docs are bilingual" convention (CLAUDE.md / project rules) and the `doc/en`+`doc/vi` layout.

## Context

The documentation tree grew organically: a flat `doc/en/` mixing genres — a
decision log (`feature.md`), design specs (`frontend.md`, `authoration.md`), gap
analyses (`missing-features.md`, `facebook-comparison.md`), deferred designs
(`archivetech*.md`), diagrams, plus ADRs nested underneath — mirrored 1:1 into
`doc/vi/`. Three pressures broke it:

1. **Mirror tax.** Every edit costs double; drift between mirrors had begun. On
   2026-07-07 the owner switched working language to English only.
2. **Genre confusion.** Normative (ADRs, module contract), aspirational (long-horizon
   specs), historical (comparisons), and living (backlog) documents are visually
   indistinguishable; the project already needs a standing warning ("trust
   MILESTONE_CHECKS over stale doc sections").
3. **New genres arrived** (brainstorm briefs, implementation-ready specs) with no
   structural home.

## Decision

Adopt a **Diátaxis-informed** tree rooted at `docs/` (the ecosystem-standard root),
with genre-separated sections and an explicit lifecycle:

`docs/{adr, product{vision, feature-inventory, backlog, checklist, analysis,
briefs, specs}, architecture{…, deferred/}, guides, reference, archive}` — full
mapping in [MIGRATION.md](../MIGRATION.md).

Policies bundled into this decision:

- **English is canonical.** The Vietnamese mirror is frozen at
  `docs/archive/vi-2026-07/` and never updated. The bilingual rule in CLAUDE.md is
  replaced by a pointer to this ADR.
- **Living status stays out of docs/**: `MILESTONE_CHECKS.md` remains at the repo
  root as the single status truth; documents defer to it instead of restating state.
- **Canonical-source rule**: contracts live next to what they govern
  (`backend/MODULES.md`, `shared/openapi.yaml`); `docs/reference/` points at them
  rather than copying them.
- Every document carries a status header (`STYLE.md`); ADRs are immutable once
  accepted (supersede or add dated revision notes).

## Options considered

- **A. Keep `doc/en`+`doc/vi`, just add subfolders.** Rejected: keeps the mirror
  tax that the owner has already abandoned in practice; drift becomes silent lying.
- **B. Diátaxis-informed `docs/`, English canonical** *(chosen)*: matches the
  actual genres present; standard root; one language, one truth per fact.
- **C. Wiki (GitHub wiki / Notion).** Rejected: splits docs from code review and
  version history; violates the repo-as-single-source habit the project relies on.
- **D. Strict Diátaxis (tutorials/how-to/reference/explanation only).** Rejected as
  a straitjacket: a design-heavy pre-1.0 solo project is dominated by decisions,
  briefs, and specs — genres strict Diátaxis has no first-class home for. We keep
  its *separation principle*, not its exact four boxes.

## Trade-offs

- One-time link breakage across the repo; mitigated by the migration script + grep
  checklist in MIGRATION.md, and by citing decisions via stable IDs (`D-N`, `ADR-N`)
  going forward.
- Vietnamese-speaking future contributors lose maintained VI docs; accepted —
  the archive remains readable, and code/API-level naming was always English.
- Renames (`authoration.md` → `security.md`, `archivetech*` → descriptive names)
  trade grep-ability of old names for legibility; the mapping table preserves the
  trail.

## Consequences

- CLAUDE.md and the project instructions must be updated: bilingual rule removed,
  `doc/*` paths → `docs/*` (action item — instructions currently contradict this ADR
  until edited).
- The standing "trust MILESTONE_CHECKS" warning gets structural support: genres and
  status headers make staleness visible.
- New-document authors have exactly one correct location per genre
  (`docs/README.md` "Genre rules"), ending ad-hoc placement.
- Optional follow-up: CI link check (`lychee`) over `docs/` to keep the tree honest.

## Action items

- [ ] Accept this ADR (owner).
- [ ] Run `migrate-docs.sh` on a branch; drop in the bundle's new files.
- [ ] Fix inbound links (MIGRATION.md step 3) and update CLAUDE.md / project
      instructions (language rule + paths).
- [ ] Add `docs/` link-check to CI (optional, P3).
