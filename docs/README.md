# Portal — Documentation

**Status:** current · **Last verified:** 2026-10-01

Design corpus for Portal: a self-hosted **life OS** — one digital identity with
facets for money, time, learning, social, and entertainment (see
[product/vision.md](product/vision.md)). Go modular-monolith backend, Next.js 15
frontend, Docker Compose behind Traefik.

**Implementation status lives in the code, and is described in one place:**
[`/CLAUDE.md`](../CLAUDE.md) § "Current status". Module contract:
[`/backend/MODULES.md`](../backend/MODULES.md). Which fact lives where is
[ADR-11](product/specs/SPEC-06-docs-canonicalisation.md#adr-11)'s decision.

**Language:** English only ([ADR-09](product/specs/SPEC-06-docs-canonicalisation.md#adr-09)).

## Map

| Section | Answers | Start with |
|---|---|---|
| [product/](product/README.md) | Why does this exist, what are we building, in what order | [vision.md](product/vision.md) |
| Decision records (`ADR-NN`) — no folder of their own; each lives in the spec that owns its subject | What did we decide and why | [specs/README.md § Decision records](product/specs/README.md#decision-records) (the ID → file index) |
| [architecture/](architecture/README.md) | How is it designed | [diagrams.md](architecture/diagrams.md) |
| [guides/](guides/) | How do I get set up to work on it | [getting-started.md](guides/getting-started.md) |
| [operations/](operations/) | How do I run the deployed system | [backup-restore.md](operations/backup-restore.md) |
| [reference/](reference/README.md) | Lookup: events, contracts, canonical sources — and **what is proven** by a test | [events.md](reference/events.md) · [TRACEABILITY-MATRIX.md](reference/TRACEABILITY-MATRIX.md) |
| [testing/](testing/README.md) | What we **intend** to prove, per spec (test plan, case documents) | [TEST-PLAN.md](testing/TEST-PLAN.md) |
| [agents/](agents/) | Where the engineering skills find the issue tracker, triage labels and domain-doc rules | [issue-tracker.md](agents/issue-tracker.md) |

**Outside `docs/`** — contracts live next to what they govern ([ADR-09](product/specs/SPEC-06-docs-canonicalisation.md#adr-09)); this is the list:

| File | Governs |
|---|---|
| [`/CLAUDE.md`](../CLAUDE.md) | The repo as a whole: stack, boundaries, **implementation status** (the one written owner) |
| [`/CONTEXT.md`](../CONTEXT.md) | The product glossary — one canonical term per concept, with the words to avoid. Not a spec: no implementation detail lives there |
| [`/backend/MODULES.md`](../backend/MODULES.md) | Backend module contract; the new-module checklist |
| `backend/internal/modules/<name>/README.md` | What that module owns, talks to, emits and subscribes. **Not status** — open work is in [product/backlog.md](product/backlog.md) |
| [`/frontend/CLAUDE.md`](../frontend/CLAUDE.md) · [`/frontend/src/templates/README.md`](../frontend/src/templates/README.md) | Frontend conventions; the version-switched template tree |
| [`/shared/openapi.yaml`](../shared/openapi.yaml) | The API contract ([ADR-10](product/specs/SPEC-03-platform-ops.md#adr-10)) |
| [`/scraper/README.md`](../scraper/README.md) | The comic scraper service |
| [`/.design-sync/NOTES.md`](../.design-sync/NOTES.md) | Design-sync harness facts and capture techniques |
| [`/.continue/rules/CONTINUE.md`](../.continue/rules/CONTINUE.md) | Orientation for the Continue IDE agent — a pointer to the files above, owns nothing |

## Reading order for a new contributor

0. [product/backlog.md](product/backlog.md) — the open work, P0 first, **before
   trusting any other document here**. It is the **single priority list**: it
   ranks everything, including the spec↔code divergences whose detail lives in
   each spec's "Implementation gaps" section (indexed in
   [product/specs/README.md](product/specs/README.md)). The audits
   ([product/analysis/](product/analysis/)) are the evidence behind both.
1. [product/vision.md](product/vision.md) — what Portal is (life OS) and the v1
   envelope (1 dev · single VPS · ≤$100/mo).
2. The decision records ([index](product/specs/README.md#decision-records)) →
   skim [ADR-01](product/specs/SPEC-03-platform-ops.md#adr-01) (scope cut) in SPEC-03,
   [ADR-08](product/specs/SPEC-09-life-stream-home.md#adr-08) (life-OS pivot) in SPEC-09,
   [ADR-06](product/specs/SPEC-01-account-identity-admin.md#adr-06) (local auth)
   in SPEC-01, and [ADR-11](product/specs/SPEC-06-docs-canonicalisation.md#adr-11) (how these
   documents are kept true) in SPEC-06.
3. [architecture/diagrams.md](architecture/diagrams.md) — the visual map.
4. [`/backend/MODULES.md`](../backend/MODULES.md) — module boundaries **before
   writing any backend code**.
5. [product/specs/](product/specs/README.md) — what's being built right now.

## Genre rules (what goes where)

One document, one genre. The one deliberate mix: a spec carries the decision
records for its subject. If a spec starts accumulating decision rationale with
alternatives, give it a record under its `## Decision records`; if a record
starts specifying endpoints, move that into the spec body.

- A **decision** with alternatives and consequences → a **decision record**
  (`ADR-NN`, never reused; the next is ADR-18) under `## Decision records` in
  the spec that owns its subject — always a spec, never
  [product/specs/README.md](product/specs/README.md), which keeps only the
  [index](product/specs/README.md#decision-records), where it gets a row.
  There is no `adr/` folder: it was folded into the specs on 2026-10-01
  ([ADR-09](product/specs/SPEC-06-docs-canonicalisation.md#adr-09) § Consequences). Corrected in
  place by layer ([ADR-11](product/specs/SPEC-06-docs-canonicalisation.md#adr-11)); shape, anchor
  and markers: [STYLE.md § Decision records](STYLE.md#decision-records-binding).
- An implementation-ready **spec/PRD** → `product/specs/`. The brainstorm-level
  "what & why" is the spec's opening sections, not a separate brief (the
  `product/briefs/` folder was folded into the specs and deleted after
  `ea100d8`).
- An **audit** — a dated point-in-time review of the repo against its documents
  (the folder is `product/analysis/`; "review", "worklog" and "analysis" are
  the same genre, and *audit* is its name) → `product/analysis/`, filename
  `topic-YYYY-MM-DD.md`. Its **body is never edited**; only the header lines
  (Status, Last verified, labels) and the *targets* of its relative links may
  change, so a moved document does not leave it pointing at nothing. It is
  superseded by a newer audit, not revised. An audit is **triaged in the same
  session that writes it**: every finding becomes a line in
  [product/backlog.md](product/backlog.md) or is closed there with a reason,
  and the audit's header gains `**Triaged:** YYYY-MM-DD → backlog.md`. A
  finding that generates no work will be found again; an audit that sits
  untriaged is how 17 days were lost in 2026-08.
- A **design document** for how a system works → `architecture/` (post-v1 designs
  go in `architecture/deferred/`).
- A **how-to for working on the repo** (setup, tooling, conventions) → `guides/`.
- A **runbook for operating the deployed system** (backup, restore, cutover,
  tuning) → `operations/`.
- A **test plan, a per-spec test-case document, or a dated test run** —
  what we *intend* to prove → `testing/`. A dated manual run that exercises
  one spec is a section of that spec instead (SPEC-08 § Manual run). What
  *is* proven, by which `_test.go` function, is the traceability matrix in
  `reference/`.
- A **registry or pointer** you look up rather than read → `reference/`.
- **Configuration the engineering skills read** (which issue tracker, which
  triage labels, where domain docs live) → `agents/`. Written once by
  `/setup-matt-pocock-skills`, edited by hand after; never a place for content
  a human reads to learn the system.
- **Implementation status** has exactly one written owner,
  [`/CLAUDE.md`](../CLAUDE.md) § Current status, and one list of what is
  still open, [product/backlog.md](product/backlog.md). No other document —
  decision-record action items, module READMEs, specs — carries a status claim it does not
  cite one of those two for.
- Anything **no longer maintained → delete it**. Git history is the archive.
  Wherever the document is still mentioned, cite the deleting commit
  (`deleted in f11cf3f`) so the reader can find it without a broken link.

Conventions for writing any of these: [STYLE.md](STYLE.md).
