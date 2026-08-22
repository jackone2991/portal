# Portal — Documentation

Design corpus for Portal: a self-hosted **life OS** — one digital identity with
facets for money, time, learning, social, and entertainment (see
[product/vision.md](product/vision.md)). Go modular-monolith backend, Next.js 15
frontend, Docker Compose behind Traefik.

**Living status truth is the code, not a document.** There is no status-tracker file
(`MILESTONE_CHECKS.md` was deleted in `f11cf3f`) — check whether a module is
constructed and mounted in `backend/cmd/api/main.go` and whether its `repository/`
dir is populated. Trust that over any "open work" section in these documents; see
[`/CLAUDE.md`](../CLAUDE.md) § "Current status" for the full set of signals. Session
and repo conventions: [`/CLAUDE.md`](../CLAUDE.md). Module contract:
[`/backend/MODULES.md`](../backend/MODULES.md).

**Language:** English only ([ADR-09](adr/09-docs-architecture.md)). The former
Vietnamese mirror is frozen in [archive/vi-2026-07/](archive/).

## Map

| Section | Answers | Start with |
|---|---|---|
| [product/](product/README.md) | Why does this exist, what are we building, in what order | [vision.md](product/vision.md) |
| [adr/](adr/README.md) | What did we decide and why (00–10) | [adr/README.md](adr/README.md) |
| [architecture/](architecture/README.md) | How is it designed | [diagrams.md](architecture/diagrams.md) |
| [guides/](guides/getting-started.md) | How do I run and work on it | [getting-started.md](guides/getting-started.md) |
| [reference/](reference/README.md) | Lookup: events, contracts, canonical sources | [events.md](reference/events.md) |

## Reading order for a new contributor

1. [product/vision.md](product/vision.md) — what Portal is (life OS) and the v1
   envelope (1 dev · single VPS · ≤$100/mo).
2. [adr/README.md](adr/README.md) → skim ADR-01 (scope cut), ADR-06 (local auth),
   ADR-08 (life-OS pivot).
3. [architecture/diagrams.md](architecture/diagrams.md) — the visual map.
4. [`/backend/MODULES.md`](../backend/MODULES.md) — module boundaries **before
   writing any backend code**.
5. [product/specs/](product/specs/README.md) — what's being built right now.

## Genre rules (what goes where)

- A **decision** with alternatives and consequences → `adr/` (numbered, immutable
  once accepted; supersede, don't edit history).
- A **brief** (brainstorm-level "what & why") → `product/briefs/`.
- An implementation-ready **spec/PRD** → `product/specs/`.
- A **design document** for how a system works → `architecture/` (post-v1 designs
  go in `architecture/deferred/`).
- A **how-to** for humans working on the repo → `guides/`.
- A **registry or pointer** you look up rather than read → `reference/`.
- Anything no longer maintained → `archive/` with a date-stamped folder.

Conventions for writing any of these: [STYLE.md](STYLE.md).
