# ADR-11 — Documentation Canonicalisation (ADRs are corrected in place; nothing is archived)

**Status:** **accepted** 2026-09-11
**Last verified:** 2026-09-11
**Relates to:** [ADR-09](09-docs-architecture.md) (the tree this refines) · [STYLE.md](../STYLE.md) (the rules this changes) · [docs/README.md](../README.md) (the genre map this extends)
**Affects:** every file under `docs/`, `/CLAUDE.md`, `backend/MODULES.md`

## Context

An audit on 2026-09-11 read all eleven ADRs against the code and found **79
statements that are no longer true**. Not one ADR was wholly accurate. The
pattern is the same in every file: the *decision* still holds, but the
description of the world it was made in has rotted — services that were
removed, files that were deleted, counts that drifted, action items ticked in
code but never in the document.

Two rules in [STYLE.md](../STYLE.md) produced this, and they were in tension
with each other:

- *"Accepted ADRs are immutable: correct them by a dated addendum note"* — so
  wrong facts stayed in the body and corrections piled up underneath, until a
  reader had to know which paragraph won. [ADR-07](07-tenancy-rls-model.md)
  had already broken this rule out of necessity ("used to read … Both were
  false"), which is the honest signal that the rule did not fit the repo.
- *"Replaced → move to `archive/`"* — but `docs/archive/` was deleted in
  `f11cf3f` (2026-07-19, 33 files), and four documents still described it as
  "frozen, never update". An archive nobody maintains is a broken link with
  extra steps.

Beneath both: no document owned any given fact. Implementation status lived in
`MILESTONE_CHECKS.md` (deleted in `f11cf3f`), in `CLAUDE.md`, in each ADR's
action items, and in the specs — so when one moved, the others did not.

Separately, the audit at
[analysis/remaining-work-2026-08-25.md](../product/analysis/remaining-work-2026-08-25.md)
had already listed most of these defects, with line numbers, seventeen days
earlier — and sat unread in a folder that neither `docs/README.md` nor ADR-09
declared. A finding that generates no work is a finding that will be made
again.

## Decision

1. **Every fact has exactly one living owner.**

   | Fact | Owner |
   |---|---|
   | What is implemented, wired, tested | `/CLAUDE.md` § Current status |
   | Which folder a document belongs in | `docs/README.md` (genre rules) |
   | How a document is written | `docs/STYLE.md` |
   | The contract between backend modules | `backend/MODULES.md` |

   Every other document *points* at the owner for that fact. It does not
   restate it.

2. **ADRs are corrected in place, by layer.** The immutability rule is
   withdrawn. Within an ADR:

   - **Fact clauses** — descriptions of current state, file paths, counts,
     action-item checkboxes — are **rewritten** to be true.
   - **Decision narrative** — *Options considered*, *Trade-offs*, and the
     reasoning that led to the choice — is **kept verbatim**. That layer records
     what was known when the decision was made; editing it would be rewriting
     history.

   The line between the two is judgement, not mechanism. When unsure, keep.

3. **Nothing is archived in place. Unmaintained documents are deleted**, and
   the deleting commit is cited wherever the document is still mentioned. Git
   history is the archive; a folder named `archive/` is a promise to maintain
   what it holds, and that promise was not kept.

4. **`**Last verified:** YYYY-MM-DD`** — a field STYLE.md already required —
   is the only mark that a document has been checked against the code. No
   changelog, no "Update note" convention, no per-section stamps. One field,
   already defined, already in the header.

5. **Counts in prose are replaced by the command that produces them.**
   STYLE.md already forbade rotting numbers; `CLAUDE.md` said "13 modules",
   "12 modules" and "14 test files" in the same file while the disk held 14
   and 30. The rule was right and unenforced.

6. **`product/analysis/` stays immutable** — a dated audit is a record of what
   was found, and editing its body destroys that. But an audit is not filed
   and forgotten: when accepted it must either generate backlog lines or be
   closed with a reason (see [docs/README.md](../README.md) § analysis).

## Options considered

- **Keep immutability; add one dated addendum per correction.** Rejected:
  this is what produced 79 wrong statements. Addenda accumulate, the body
  stays false, and a reader who trusts the first paragraph is misled.
- **Supersede each stale ADR with a new one.** Rejected: eleven new ADRs
  whose only content is "the previous one was right but the facts moved" is
  ceremony, and doubles the file count the next audit has to read.
- **Keep `archive/` and repopulate it.** Rejected: the folder was deleted two
  months ago and nobody noticed. An archive that is not read is not an
  archive.
- **Write a `CONTEXT.md` glossary for the documentation domain.** Rejected:
  the genre rules in `docs/README.md` *are* that glossary. Adding a fourth
  source of "what goes where" while reconciling three is self-inflicted.
- **Move status truth from files into code (a `/status` endpoint, a generated
  page).** Not decided here — that was settled in `f11cf3f` when
  `MILESTONE_CHECKS.md` was deleted in favour of "verify against the code".
  This ADR only names `/CLAUDE.md` as where that policy is written down.

## Trade-offs

- **Lost:** the guarantee that an ADR reads today as it did on acceptance.
  Mitigated by keeping the decision narrative verbatim and by git history,
  which has always been the real record.
- **Lost:** a place to park documents that are "not quite dead". Accepted
  deliberately — that is the state in which they rot.
- **Gained:** a reader can trust the first paragraph of any ADR again.
- **Gained:** one field (`Last verified`) answers "has anyone checked this
  lately", instead of a reader reconstructing it from addenda dates.
- **Cost:** every future audit must edit ADRs, not append to them. That is
  more work per finding and less work per reader; readers outnumber auditors.

## Consequences

- ADR numbering: `00` is retired permanently.
  [00-architecture-review.md](00-architecture-review.md) was never a decision
  record — it is a dated review of the architecture as found on 2026-07-07,
  all eleven of its false statements are descriptions of that day, and by the
  genre rules it belongs in `analysis/`. It moves to
  `product/analysis/architecture-review-2026-07.md` (body untouched, status
  `historical`) as an action item below. STYLE.md says numbers are never
  reused, so the next ADR is 12.
- `docs/README.md` gains rules for `testing/`, `operations/` and `analysis/`
  (three folders that existed on disk without a declared genre), a reading-
  order line that puts the newest audit **before** every other document, and
  the deletion convention.
- `STYLE.md` loses its *Genre discipline* section (a second, shorter, already
  diverging copy of the README's genre map), the archive references, and the
  immutability clause.
- A relative-link check runs in CI as its own job, blocking
  (`scripts/check-links.sh`, job `link-check`). The audit counted 69 broken
  relative links by hand; the script found 95 — the 26 it missed are what a
  rule that is not checked looks like.

## Action items

- [x] Delete `docs/testing/SESSION-HANDOFF-2026-07-12.md` (committed with
      credentials; `b54654e`). Rotation is the operator's call — P0 in backlog.
- [x] This document.
- [x] `docs/README.md`, `STYLE.md`, `CLAUDE.md` per Consequences.
- [x] Move `guides/backup-restore.md` and `guides/rls-cutover.md` to
      `operations/`; `testing/TRACEABILITY-MATRIX.md` to `reference/`.
- [x] Fix the broken relative links (95 → 0), then enable the link-check job.
- [ ] Rewrite the fact layer of ADR-01…10 (ADR-10 and ADR-03 first — they
      carry 23 of the 79 — then review before the rest).
- [ ] Move ADR-00 to `analysis/`.
- [ ] Re-grade `TRACEABILITY-MATRIX.md` against the `_test.go` files that
      exist, with an `Evidence` column (`file:func`) — a ✅ with no evidence is
      what produced 47 rows at 100%.
- [ ] Replace the archived `product/backlog.md` with a live one triaged from
      the 2026-08-25 audit; RLS `.env.example` default and credential rotation
      lead it.
- [ ] `backend/MODULES.md` § 8: add the OpenAPI step the new-module checklist
      is missing (a module that follows the checklist today still fails CI).
