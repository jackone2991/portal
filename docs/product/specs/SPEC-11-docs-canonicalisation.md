# SPEC-11 — Docs Canonicalisation: retire 79 stale assertions and gate the corpus in CI

**Status:** executed 2026-09-11 on branch `docs/architecture-cleanup` (rev 1 drafted the same day as "SPEC-10"; renumbered on filing because SPEC-10 is the ledger expansion) · **Last verified:** 2026-09-11
**Module:** none — the documentation corpus, plus one CI job and four check scripts
**Depends on:** nothing hard. Branched from the `bank` feature branch at `66709d1` (see Implementation Decisions)
**Upstream:** design session 2026-09-11 (37 settled decisions) and a `/grill-with-docs` pass on `docs/README.md` the same day (12 more)
**Refs:** [ADR-09](../../adr/09-docs-architecture.md) (the tree this supersedes in part) · [ADR-11](../../adr/11-docs-canonicalisation.md) (written *by* this spec, stage one) · [STYLE.md](../../STYLE.md) (§ADR shape amended) · [analysis/remaining-work-2026-08-25.md](../analysis/remaining-work-2026-08-25.md) (prior audit, triaged by stage three into [backlog.md](../backlog.md))
**Downstream consumers:** every agent session (the project instructions are loaded unconditionally); CI; anyone onboarding

## As executed — deltas from the plan below

The plan is kept as written; this section is its fact layer. Commits `b54654e` … on `docs/architecture-cleanup`.

| Plan said | Found / done |
|---|---|
| 69 broken relative links | `scripts/check-links.sh` found **95**; all fixed. |
| "the live environment file is absent from this checkout" | `.env` **exists** and points `DATABASE_URL` at `portal_app` — RLS is enforced on this deployment. `.env.example` still seeds the superuser; that is backlog P0 #1. The connecting role was not changed, as decided. |
| ADR-00 dated 2026-07 | First commit `755dfa8`, 2026-05-24 → `product/analysis/architecture-review-2026-05-24.md`. |
| `checklist.md` "never existed" | It did; deleted in `efb8a70`. Cited so. |
| One script, three assertions, one job | Four scripts (`check-links.sh`, `check-doc-headers.sh`, `check-retired-names.sh`, `check-matrix-evidence.py`) as four steps of one blocking `link-check` job. The header check the plan deferred ("needs a forty-document backfill first") was done: 41 files backfilled, `Last verified: never` made a legal value. |
| Retired-name check allow-lists the audit directory, ADR-11 and the working note | Also skips the Decision / Options / Trade-offs layers of every ADR (verbatim history may name what it knew) and accepts a retirement cue on the same line (`deleted in`, `then …`, `renamed`, `§`-citation). First run caught a third agent-instruction file, `.continue/rules/CONTINUE.md`, still calling `MILESTONE_CHECKS.md` "live status". |
| Matrix: 47 rows | 46 P0 rows + P1 rows re-graded: 21 ✅ · 18 ⚠ · 7 ✖; two rows added (SPEC-10 phase 1, CC-10 tenant/RLS) and CC-11. |
| Backlog: one line per finding heading | Done, plus a Closed list so the audit can be ticked off. Three audit claims did not survive re-check (login lockout exists since 2026-07-05; music is not half-built; the media worker has its tenant scope) and are recorded as such. |
| Module contract gains the codegen step | Done, plus depguard block, Asynq-server choice, events-registry step (§8) and the RLS-from-birth rule (§6). |
| Working note migrates into the backlog and shrinks to a pointer | The note is the operator's file (`Note`, untracked by this work); the backlog carries everything it needed to. |
| Credential's location kept out of pushed material | The deleting commit `b54654e` names the file; the backlog and ADR-11 cite the commit, not the path. Rotation remains the operator's, backlog P0 #2. |

Tickets 1–15 of the ticket split all closed; ticket 9 was done in one pass (it fit), ticket 11 needed no human because `.env` was present, and 12 gained the CI assertion the plan filed as a work item.

---

## Problem Statement

Whoever opens this repository — a human contributor, or an agent loading the project instructions at the start of every session — is reading a documentation corpus that asserts things the code contradicts, in enough places that no document can be trusted without re-deriving it from the code first.

Measured against the working tree on 2026-09-11:

- **79 false statements across the 11 architecture decision records.** No ADR is entirely accurate. The worst three are the OpenAPI-direction record (12), the single-VPS topology record (11, wrong from its first line) and the original architecture review (11). Examples: the topology record lists Postgres and PgBouncer as "as-built" compose services (both commented out since 2026-08-21; Postgres runs on the host cluster) and states that Mailpit "never shipped" (it is a live service); the OpenAPI record's Context says "no generated code exists" and describes the spec as "~730 lines, 12 paths" (the generated code is committed and the spec is 5,137 lines across 111 paths).
- **69 broken relative markdown links.** The dominant class is a single mechanical error repeated across four decision records, left behind when the docs tree was restructured and files moved up two directory levels without their links following.
- **Three different counts of the repo's test files in three documents** (14, 20; actual 30), and **three different module counts inside one document** (13, 12, 12; actual 14) — in the file that is loaded into every agent session unconditionally.
- **Four documents assert an archive directory that was deleted two months ago**, one of them instructing the reader never to edit a path that no longer exists.
- **A deleted status-tracker file is still cited as authoritative in eight live documents**, including the style guide, which offers citing it as the *exemplary* way to describe implementation state.
- **Two decision records still carry `Status: proposed`** while the decisions they describe have demonstrably shipped.
- **The documentation index declares five sections; seven exist on disk.** The two undeclared ones hold the entire test corpus and the operational runbooks.

The cost is not cosmetic. A reader who follows the documentation configures the wrong things. An agent that trusts the project instructions' counts, or the decision records' topology descriptions, plans against a system that does not exist. And the corpus has already proved it can absorb a correct diagnosis without acting on it: a comprehensive, accurate, line-referenced audit was written on 2026-08-25 and then sat unread for 17 days — in a directory that neither the documentation index nor the docs-architecture decision record declares exists.

## Solution

Make every living document true, make every immutable document honestly labelled, and put a blocking CI gate under the two classes of defect that generated most of the damage — so that the corpus cannot silently rot back.

Six principles generate all of the work:

1. **One owner per fact.** The project instructions own implementation status; the documentation index owns genre-to-directory routing; the style guide owns how to write a document; the module contract owns module conventions. No fact is explained twice.
2. **No archive-in-place.** A document that is no longer maintained is deleted, and the retirement note cites the deleting commit. Git history is the archive.
3. **Decision records are no longer immutable**, but they are corrected in tiers: statements of *fact* (current-state descriptions, paths, action-item checkboxes) are rewritten; the decision narrative, "Options considered" and "Trade-offs" are preserved. This reverses a rule the style guide marks `binding` — a rule that had already been violated in practice by one record being rewritten in place.
4. **The only marker for "checked against the code" is the `Last verified` date field** that the style guide already defines. No new changelog mechanism.
5. **A count in prose is replaced by the command that produces it** — already mandated by the style guide's own linking rules, and violated by the counts above.
6. **Point-in-time audits stay immutable.** An audit is labelled, never edited; and from now on an accepted audit must produce work items or be closed with a reason.

The work is sequenced in three stages with a review checkpoint after the mechanical stage, because the tiered rewrite in stage two is a judgement call on every one of 79 statements.

## User Stories

1. As an agent starting a session, I want the project instructions' factual claims to be true, so that I plan against the system that exists rather than the one described.
2. As an agent starting a session, I want counts in the instructions replaced by the commands that produce them, so that a number can never be stale.
3. As an agent asked to add a module, I want the module contract's checklist to be complete, so that following it does not leave me failing CI.
4. As an agent exploring an unfamiliar area, I want the documentation index to list every section that exists, so that I do not miss the directory holding the test corpus or the runbooks.
5. As an agent reading a decision record, I want its current-state descriptions to match the code, so that I do not implement against a topology that was dismantled.
6. As an agent reading a decision record, I want the decision narrative and rejected alternatives preserved, so that I can tell why a path was not taken and avoid re-proposing it.
7. As an agent following a link in any document, I want the link to resolve, so that a citation is a path to evidence rather than a dead end.
8. As a new contributor, I want a single reading order that starts with what is true, so that my first hour is not spent discovering that documents disagree.
9. As a new contributor, I want the most recent audit surfaced in the reading order, so that I read the corpus's own list of known lies before trusting any part of it.
10. As a new contributor, I want the genre rules to tell me where a document belongs, so that I do not put a runbook in the how-to section.
11. As a maintainer, I want every document to have exactly one section it could belong to, so that routing decisions stop being re-litigated.
12. As a maintainer, I want retired documents deleted rather than archived in place, so that a stale document cannot be found and mistaken for a live one.
13. As a maintainer, I want the retirement convention to cite the deleting commit, so that "git history is the archive" is findable rather than theoretical.
14. As a maintainer, I want the backlog file to be the live backlog, so that the name the instructions advertise does not open onto a document banner-marked "do not plan from this file".
15. As a maintainer, I want the 2026-08-25 audit triaged into work items, so that an accurate diagnosis produces action instead of sitting unread for a second month.
16. As a maintainer, I want a standing rule that an accepted audit must generate work or be closed with a reason, so that this failure mode does not recur.
17. As a maintainer, I want a blocking CI gate on link integrity, so that the 69 links I fix by hand stay fixed.
18. As a maintainer, I want a blocking CI gate on retired names, so that the class of defect that produced most of the 79 false statements — plain-prose references to deleted things — cannot be reintroduced.
19. As a maintainer, I want the docs gate to be a separate CI job from the Go lint job, so that a documentation failure is legible as one and does not hide inside module-boundary enforcement.
20. As a reviewer, I want the two-tier rewrite applied consistently, so that I can tell at review time which sentences were retrofitted and which are original.
21. As a reviewer, I want the tiered rewrite demonstrated on the two worst records first, so that I can correct the cut line before it is applied to nine more files.
22. As a future reader, I want each canonicalised record to carry the date it was last checked against the code, so that I know how much to trust it without re-deriving it.
23. As a future reader, I want a decision record explaining why records in this repo are corrected in place, so that the reversal of the immutability rule is not a mystery.
24. As a future reader of an archived audit, I want it labelled rather than edited, so that it remains a faithful snapshot of what was believed at the time.
25. As a reader of the traceability matrix, I want the coverage column to reflect real tests, so that "100% covered" stops meaning "nobody has run anything".
26. As a reader of the traceability matrix, I want each covered row to name the test that covers it, so that I can verify the claim without re-auditing the suite.
27. As a maintainer of the traceability matrix, I want a CI assertion that every named test exists and every covered row names one, so that the matrix cannot drift back to uniform green.
28. As an operator, I want the runbooks in the operations section rather than the contributor how-to section, so that operating the deployed system and working on the repo are not the same shelf.
29. As an operator, I want the record of the original architecture review moved to the audit section, so that a point-in-time review is not mistaken for a standing decision.
30. As an operator, I want the true state of row-level-security enforcement stated in exactly one place, so that five documents cannot disagree about whether tenant isolation is active.
31. As an operator, I want the environment template to seed a configuration whose security posture matches what the decision records claim, so that a new environment does not silently come up with decorative isolation.
32. As an operator, I want the committed development credential removed and rotated, so that a secret in a public repository stops being a live secret.
33. As a security-conscious maintainer, I want the precise location of that credential kept out of anything that gets pushed, so that remediation does not advertise the problem further.
34. As an agent regenerating the API contract, I want the existing codegen drift gate to cover the contract edit this work requires, so that a documentation change cannot leave generated code stale.
35. As the developer, I want this work split into tickets with declared blocking edges, so that I can pick it up across several sessions without re-deriving the plan.

## Implementation Decisions

**Branching.** The work branches from the current feature branch, not from the default branch. The contract file that must be edited has already been rewritten by ~4,700 lines on the feature branch; branching from the default branch would guarantee a conflict in a CI-gated file plus its two generated artifacts.

**Commit shape.** The removal of the committed credential is the first commit and stands alone, so that it can be pushed immediately and independently of the rest.

**Stage one — mechanical.** A new decision record is written *first*, declaring the canonicalisation policy that stage two then executes: the end of record immutability, the delete-don't-archive retirement policy, the one-owner-per-fact assignment, and `Last verified` as the freshness marker. It takes the next free number; the retired number of the relocated architecture review is not reused. Then: the documentation index gains the two undeclared sections, three new genre rules (test document, runbook, audit), the delete-don't-archive rule, and a reading-order entry pointing at the newest audit; its duplicated status-signal prose collapses to a single pointer. The style guide loses its genre list entirely (it duplicated and diverged from the index's), and its language, naming, lifecycle and record-shape sections are corrected. The project instructions lose two false claims and have both count assertions replaced by commands. Two further documents drop the deleted-tracker citation. The archived audit is labelled, not edited. Three documents are relocated to the sections their genre now assigns them, and every inbound reference is repointed — including one inside the API contract, which requires running codegen and committing the regenerated artifacts.

**Stage two — the 79 statements, with a checkpoint.** The two worst records are canonicalised first and reviewed before the remaining nine are touched, because the boundary between "statement of fact" and "decision narrative" is a judgement call rather than a rule. Four cross-cutting classes are fixed in one sweep each: the deleted status tracker, the claim that CI gates sqlc drift (it never has — only the OpenAPI codegen is drift-gated), the claim that Postgres and PgBouncer run in compose, and a directory-depth error in four records' reference headers. Two records have their status flipped from proposed to accepted.

**The original architecture review is reclassified, not rewritten.** It is a point-in-time review, not a decision, which is why all eleven of its false statements are current-state descriptions. It moves to the audit section with a historical status and its body preserved verbatim, and its number is permanently retired.

**Row-level security is documented, not changed.** Five sources disagree about whether tenant isolation is enforced; the live environment file is absent from this checkout, so the question cannot be settled from the repo. One finding stands regardless: the environment template seeds a superuser connection, so every newly created environment gets non-enforcing RLS whatever the current machine does. This work states the true position in one place and files the cutover as a P0 work item; it does not change the database role or run the cutover, because changing the connecting role can break queries that depend on superuser rights and needs to be exercised against real data.

**Stage three.** The traceability matrix's coverage column is rebuilt from evidence — covered only where a named test demonstrably exercises the requirement, partial where the module has tests that do not, gap where it has none — and gains an evidence column naming the test for every covered row. The archived backlog is deleted and a live backlog written in its place from a full triage of the 2026-08-25 audit, one line per finding heading, with the credential rotation and the RLS cutover as the leading P0 items. The module contract gains the API-contract rule and the codegen step its checklist is missing — a gap that currently lets a new module pass all seven documented steps and still fail CI.

**Citations are repointed, not translated.** Where a dead link has a successor document, the link is repointed; where it does not, the link is removed, the name is kept in prose, and the deleting commit is cited. Section-number citations into a document that has since switched to decision-ID citations are left as written, because translating them means guessing which decision the record meant — that is rewriting a decision, not repairing a pointer.

## Testing Decisions

**What a good test is here.** The unit under test is the documentation corpus, and the only externally observable behaviour it has is whether its claims resolve against the repository. A good assertion is therefore mechanical and content-agnostic: it checks that a reference points at something that exists, never that a sentence is worded a particular way. Nothing asserts on prose.

**One new seam: a single script behind a single blocking CI job.** Three assertions live in that one script rather than in three jobs, so that the next invariant has one place to go and the corpus has one red/green signal.

- **Link resolution.** Every relative markdown link in the repo resolves. Reference material vendored from a third-party template is excluded; fragments are not checked; no network access, so external URLs are out of scope.
- **Retired-name check.** A deny-list of names for deleted things must not appear in any *living* document, with the audit directory, the new policy record and the local working note allow-listed. This assertion exists because the link checker structurally cannot catch this class: most of the 79 false statements are plain prose or inline-code mentions, not markdown links. A repo full of references to deleted files passes a link checker cleanly.
- **Matrix evidence.** Every evidence cell in the traceability matrix names a test file that exists, and every row marked covered has a non-empty evidence cell. Without this, the 47 rows rebuilt by hand in stage three drift back to uniform green within months — which is exactly how they got there the first time.

**Existing seams are reused, not duplicated.** The OpenAPI codegen drift gate already covers the contract edit and its regenerated artifacts; nothing is added there. The backend test job is unchanged. The docs job is deliberately *not* folded into the Go lint job, which exists to enforce module boundaries via depguard.

**Ordering.** The gate is enabled only after the existing 69 link failures are repaired; enabling it first would fail CI on pre-existing breakage and train everyone to ignore it.

**Prior art.** The OpenAPI job is the closest existing pattern: a generated artifact is rebuilt and compared, and the build fails on drift. The docs job follows the same shape — derive the truth from the repo, compare against what is written down, fail on divergence.

**Deliberately not asserted yet.** A "every document carries a status header" check is a real invariant that the style guide already requires, but enabling it demands a backfill across roughly forty documents first. It is filed as a work item rather than bundled here.

## Out of Scope

- Running the row-level-security cutover, or changing the database connection role.
- Purging git history, or rotating the exposed credential on the operator's behalf. The removal from the working tree and the work item are in scope; the rotation itself is a human action on infrastructure outside this repo.
- Introducing a separate domain-glossary file. The documentation index's genre rules already are the glossary for this domain; adding a fourth glossary while repairing drift between three would be self-defeating.
- Translating section-number citations into decision-ID citations inside record bodies.
- Reassessing what the test suite actually covers beyond the evidence the existing tests provide. The coverage column is rebuilt from what can be demonstrated, not from a fresh test-gap analysis.
- Any change to the vendored third-party template material.
- Creating index files for directories that hold one or three documents purely to satisfy a structural check.
- Anchor and external-URL checking in the docs gate.
- A decision record for the earlier move of status truth from a tracker file to the code; that was decided two months ago and is already documented in two live places.

## Further Notes

**This repository is public.** The exposed development credential should be treated as compromised rather than "verified if still in use", and its precise location must stay out of anything pushed — including this spec. One already-pushed document points at it; that pointer should be removed once rotation is done. Removing the file from the working tree is the most time-sensitive item in this plan and should not wait for the rest.

**The path-level worksheet lives in the repo's local working note**, which carries the full per-file, per-line detail: exact line numbers for each false statement, the per-record breakdown of all 79, the inbound-reference lists for each relocation, and the deny-list contents. This spec deliberately stays at the level of decisions so that it does not rot as files move; the worksheet is the executable detail and is expected to rot, which is why it should migrate into the live backlog at the end of stage three and shrink to a pointer.

**Provenance.** The 79-statement count comes from a per-record audit against the working tree; the 69-link count from a repo-wide relative-link sweep excluding vendored material. Both were run on 2026-09-11 and both should be re-run at the start of the work, since the feature branch continues to move.

**Two facts corrected during planning**, recorded so they are not re-derived: the local-auth record's refresh-token TTL is correct as written (24h, matching the shipped default) — an earlier suspicion that it claimed 30 days was wrong; and the 2026-08-25 audit's list of documents citing the deleted status tracker is itself incomplete, missing the style guide.

**Tracker.** This spec could not be published to the GitHub issue tracker: the authenticated account has `pull` access only on this repository (no `push`, no `triage`), so neither the `ready-for-agent` label nor an issue with labels can be created. The spec therefore lives here, which is where the genre rules this spec establishes would route it anyway. To move it onto the tracker, authenticate an account with write access and run the setup flow to establish the triage-label vocabulary.
