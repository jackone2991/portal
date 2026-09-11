# ADR-09 — Documentation Architecture

**Status:** **accepted** — drafted 2026-07-07, the tree it describes shipped the same day; three of its bundled policies were replaced by [ADR-11](11-docs-canonicalisation.md) on 2026-09-11 (marked below)
**Last verified:** 2026-09-11
**Supersedes:** the "docs are bilingual" convention (CLAUDE.md / project rules) and the `doc/en`+`doc/vi` layout.

## Context

*As found on 2026-07-07. The restructure ran that day; its own migration map
(`MIGRATION.md`) and the frozen Vietnamese mirror it created were deleted twelve
days later in `f11cf3f`, and the status tracker it deferred to went with them.
ADR-11 records what replaced each.*

The documentation tree had grown organically: a flat `doc/en/` (retired) mixing genres — a
decision log (`feature.md`, then; now `product/feature-inventory.md`), design specs (`frontend.md`, `authoration.md` — renamed `security.md`), gap
analyses (`missing-features.md` — deleted; `facebook-comparison.md`), deferred designs
(`archivetech*.md` — renamed under `architecture/deferred/`), diagrams, plus ADRs nested underneath — mirrored 1:1 into
`doc/vi/` (retired). Three pressures broke it:

1. **Mirror tax.** Every edit cost double; drift between mirrors had begun. On
   2026-07-07 the owner switched working language to English only.
2. **Genre confusion.** Normative (ADRs, module contract), aspirational (long-horizon
   specs), historical (comparisons), and living (backlog) documents were visually
   indistinguishable; the project already needed a standing warning ("trust
   `MILESTONE_CHECKS.md` over stale doc sections" — a file since deleted in `f11cf3f`).
3. **New genres arrived** (brainstorm briefs, implementation-ready specs) with no
   structural home.

## Decision

Adopt a **Diátaxis-informed** tree rooted at `docs/` (the ecosystem-standard root),
with genre-separated sections and an explicit lifecycle.

As built (`ls docs/`): `adr/`, `product/{vision, feature-inventory, backlog,
analysis/, briefs/, specs/}`, `architecture/{…, deferred/}`, `guides/`,
`operations/`, `reference/`, `testing/`. The 2026-07-07 text also listed
`product/checklist` and `archive/`: the checklist was deleted in `efb8a70`, the
archive in `f11cf3f`; `operations/` and `testing/` existed on disk without being
declared here until [docs/README.md](../README.md) gained their genre rules
(ADR-11). The full old→new mapping was `MIGRATION.md` (deleted in `f11cf3f`).

Policies bundled into this decision:

- **English is canonical.** *Standing.* The bilingual rule in CLAUDE.md is
  replaced by a pointer to this ADR. (The frozen Vietnamese mirror at
  `docs/archive/vi-2026-07/` that this bullet created was deleted in `f11cf3f`;
  git history holds it.)
- **Living status stays out of `docs/`.** *Standing in spirit, changed in
  mechanism:* `MILESTONE_CHECKS.md` was deleted in `f11cf3f`; status is now
  verified against the code, with `/CLAUDE.md` § Current status as the one
  written owner (ADR-11 rule 1). Documents still defer instead of restating.
- **Canonical-source rule**: contracts live next to what they govern
  (`backend/MODULES.md`, `shared/openapi.yaml`); `docs/reference/` points at them
  rather than copying them. *Standing.*
- Every document carries a status header (`STYLE.md`). *Standing.* ~~ADRs are
  immutable once accepted (supersede or add dated revision notes).~~ *Withdrawn
  by ADR-11:* ADRs are corrected in place by layer; `Last verified` is the only
  freshness mark. ADR-07 had already broken the immutability rule out of
  necessity before it was withdrawn.

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
  checklist in `MIGRATION.md` (deleted in `f11cf3f`), and by citing decisions via stable IDs (`D-N`, `ADR-N`)
  going forward.
- Vietnamese-speaking future contributors lose maintained VI docs; accepted —
  the archive remains readable, and code/API-level naming was always English.
- Renames (`authoration.md` → `security.md`, `archivetech*` → descriptive names)
  trade grep-ability of old names for legibility; the mapping table preserves the
  trail.

## Consequences

- CLAUDE.md and the project instructions were updated: bilingual rule removed,
  `doc/*` paths → `docs/*`. (One `doc/en/…` link in `account/README.md` and
  the scope comment in `cmd/api/main.go` survived until the 2026-09-11 sweep,
  ADR-11.)
- The standing "trust MILESTONE_CHECKS" warning got structural support and then
  lost its object; the warning is now "verify against the code" in `/CLAUDE.md`.
- New-document authors have exactly one correct location per genre
  (`docs/README.md` "Genre rules"). Two folders (`operations/`, `testing/`)
  were created without one for two months — the rules now cover them.
- The optional CI link check became mandatory and blocking:
  `scripts/check-links.sh` in the `link-check` job (ADR-11). It found 95
  broken relative links when first run.

## Action items

- [x] Accept this ADR — executed 2026-07-07; status field corrected 2026-09-11.
- [x] `migrate-docs.sh` run; bundle files dropped in (2026-07-07).
- [x] Inbound links fixed (the last two on 2026-09-11) and CLAUDE.md / project
      instructions updated (language rule + paths).
- [x] `docs/` link-check in CI — landed as a blocking job under ADR-11, not as
      the optional P3 `lychee` pass proposed here.
