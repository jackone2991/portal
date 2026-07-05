# Portal — Architecture Review & ADRs

Living evaluation of the Portal architecture against its stated constraints. Each ADR follows the standard *context → decision → options → trade-offs → consequences → action items* shape.

## How this set was produced

These ADRs were written as an **evaluation pass** over the existing design corpus, not from scratch. The inputs were:

- [`CLAUDE.md`](../../../CLAUDE.md) — the authoritative project README and per-module conventions.
- [`backend/MODULES.md`](../../../backend/MODULES.md) — module-boundary contract.
- `feature.md` — the full feature spec with 40 decisions logged (`D-1` … `D-40`) across 12 phases.
- `diagrams.md` — Mermaid system landscape, module map, request flow, OIDC sequence, asset pipeline, roadmap.
- `archivetech.md` — a competing RBAC vision (policy-bundles + file-gated permissions) that conflicts with `feature.md` and `CLAUDE.md`.
- `docker-compose.yml`, `Makefile`, `shared/openapi.yaml`, `traefik/*` — actual infra.

## The framing constraint

Every ADR below is evaluated against this hard envelope:

| Constraint | Value |
| --- | --- |
| Team | **1 developer** |
| Time to v1 | **2 weeks** |
| Infrastructure budget | **≤ $100 / month** |
| Deployment target | **Single VPS** |
| Modality | Self-hosted, open-source-friendly |

The existing design corpus assumes none of these explicitly. Most of the 40 decisions are sound in the abstract — but several are **scope-incompatible** with the constraint envelope and must be deferred. The first two ADRs make that explicit; the rest stand or fall on whether they survive the cut.

## Index

| ADR | Status | Subject |
| --- | --- | --- |
| [00-architecture-review](./00-architecture-review.md) | Accepted | Executive review — what's load-bearing, what's at risk, where the design conflicts with itself |
| [01-v1-scope-cut](./01-v1-scope-cut.md) | Proposed | Cut feature.md's 12 phases down to a v1 that fits 2 weeks / 1 dev / $100 / 1 VPS |
| [02-rbac-model-reconciliation](./02-rbac-model-reconciliation.md) | Proposed | Reconcile archivetech.md's policy-bundle RBAC with feature.md/CLAUDE.md's role-hierarchy RBAC |
| [03-single-vps-topology](./03-single-vps-topology.md) | Proposed | Compose service set, resource budget, and what to disable for v1 |
| [04-storage-tier-budget](./04-storage-tier-budget.md) | Proposed | MinIO origin + R2 edge vs R2-only vs MinIO-only under the budget ceiling |
| [05-phase0-wiring-order](./05-phase0-wiring-order.md) | Proposed | Critical-path order for closing the existing wiring gap in `cmd/api/main.go` |
| [06-local-auth-model](./06-local-auth-model.md) | Proposed | Switch login from OIDC/Authentik to Portal-owned local password auth (supersedes ADR-05's OIDC login) |

## Diagrams

- [`diagrams/system-landscape.md`](./diagrams/system-landscape.md) — v1-scoped landscape (sparser than `diagrams.md`'s long-horizon version).
- [`diagrams/request-flow.md`](./diagrams/request-flow.md) — authenticated request middleware chain with the v1 cut applied.

## Conventions

- ADR file names: `NN-kebab-case-subject.md`. Numbers are sequential and **never reused** once published.
- Status moves `Proposed → Accepted → Deprecated`. To revise an Accepted ADR, write a new ADR that **supersedes** the old one and update the old one's status. Don't edit history.
- Mermaid for diagrams (matches the project convention in `diagrams.md`); GitHub renders them natively.
- Cross-reference decisions in `feature.md` by their `D-N` id wherever the ADR is restating or refining a settled decision.
