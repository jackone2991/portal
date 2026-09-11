# Portal — Documentation

**Status:** current · **Last verified:** 2026-09-11

Design corpus for Portal: a self-hosted **life OS** — one digital identity with
facets for money, time, learning, social, and entertainment (see
[product/vision.md](product/vision.md)). Go modular-monolith backend, Next.js 15
frontend, Docker Compose behind Traefik.

**Implementation status lives in the code, and is described in one place:**
[`/CLAUDE.md`](../CLAUDE.md) § "Current status". Module contract:
[`/backend/MODULES.md`](../backend/MODULES.md). Which fact lives where is
[ADR-11](adr/11-docs-canonicalisation.md)'s decision.

**Language:** English only ([ADR-09](adr/09-docs-architecture.md)).

## Map

| Section | Answers | Start with |
|---|---|---|
| [product/](product/README.md) | Why does this exist, what are we building, in what order | [vision.md](product/vision.md) |
| [adr/](adr/README.md) | What did we decide and why (01–11) | [adr/README.md](adr/README.md) |
| [architecture/](architecture/README.md) | How is it designed | [diagrams.md](architecture/diagrams.md) |
| [guides/](guides/getting-started.md) | How do I get set up to work on it | [getting-started.md](guides/getting-started.md) |
| [operations/](operations/postgres-tuning.md) | How do I run the deployed system | [backup-restore.md](operations/backup-restore.md) |
| [reference/](reference/README.md) | Lookup: events, contracts, canonical sources | [events.md](reference/events.md) |
| [testing/](testing/README.md) | What is tested, and how each spec is proven | [TEST-PLAN.md](testing/TEST-PLAN.md) |

## Reading order for a new contributor

0. The **newest audit** in [product/analysis/](product/analysis/) — read it
   **before trusting any other document here**. An audit is a dated list of what
   was found wrong; whatever it names is stale until its backlog line is closed.
1. [product/vision.md](product/vision.md) — what Portal is (life OS) and the v1
   envelope (1 dev · single VPS · ≤$100/mo).
2. [adr/README.md](adr/README.md) → skim ADR-01 (scope cut), ADR-06 (local auth),
   ADR-08 (life-OS pivot), ADR-11 (how these documents are kept true).
3. [architecture/diagrams.md](architecture/diagrams.md) — the visual map.
4. [`/backend/MODULES.md`](../backend/MODULES.md) — module boundaries **before
   writing any backend code**.
5. [product/specs/](product/specs/README.md) — what's being built right now.

## Genre rules (what goes where)

One document, one genre. If a spec starts accumulating decision rationale with
alternatives, extract an ADR and link it; if an ADR starts specifying endpoints,
move that into a spec.

- A **decision** with alternatives and consequences → `adr/` (numbered, never
  reused; corrected in place by layer — see [ADR-11](adr/11-docs-canonicalisation.md)).
- A **brief** (brainstorm-level "what & why") → `product/briefs/`.
- An implementation-ready **spec/PRD** → `product/specs/`.
- A **dated point-in-time audit** → `product/analysis/`. Never updated;
  superseded by a newer one. When an audit is accepted it must either produce
  lines in [product/backlog.md](product/backlog.md) or be closed with a reason —
  a finding that generates no work will be found again.
- A **design document** for how a system works → `architecture/` (post-v1 designs
  go in `architecture/deferred/`).
- A **how-to for working on the repo** (setup, tooling, conventions) → `guides/`.
- A **runbook for operating the deployed system** (backup, restore, cutover,
  tuning) → `operations/`.
- A **test plan, a per-spec test-case document, or a dated test run** → `testing/`.
- A **registry or pointer** you look up rather than read → `reference/`.
- Anything **no longer maintained → delete it**. Git history is the archive.
  Wherever the document is still mentioned, cite the deleting commit
  (`deleted in f11cf3f`) so the reader can find it without a broken link.

Conventions for writing any of these: [STYLE.md](STYLE.md).
