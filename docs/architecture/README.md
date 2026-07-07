# Architecture

**Status:** current · **Last verified:** 2026-07-07

How Portal is designed. Decisions live in [../adr/](../adr/README.md); this section
holds the design documents themselves.

| Document | Role |
|---|---|
| [diagrams.md](diagrams.md) | Mermaid map: system landscape, module boundaries, request/login flows, media pipeline, roadmap |
| [security.md](security.md) | Security spec: authentication, tokens, revocation, authorization (RBAC/RLS), threat model *(was `authoration.md`)* |
| [frontend.md](frontend.md) | Next.js 15 frontend: versioned templates, routing, state, auth handoff, page inventory, budgets |
| [deferred/access-policies.md](deferred/access-policies.md) | Post-v1: policy-bundle / user-group / file-gated access control *(was `archivetech.md`)* |
| [deferred/multi-tenant-backend.md](deferred/multi-tenant-backend.md) | Post-v1: RLS-scoped multi-tenant backend, milestones M0–M5 *(was `archivetech-backend.md`)* |

## Canonical sources that live outside this folder (on purpose)

- **Module boundaries & event/task registry conventions:**
  [`/backend/MODULES.md`](../../backend/MODULES.md) — next to the code it governs.
- **API contract:** [`/shared/openapi.yaml`](../../shared/openapi.yaml).
- **Implementation status:** [`/MILESTONE_CHECKS.md`](../../MILESTONE_CHECKS.md).

## `deferred/` semantics

Documents here are **designs, not commitments** — explicitly out of the v1 envelope
(ADR-01) but written down so future work starts from thinking, not memory. Their
migration numbers and phase plans are indicative; re-verify against the repo when
scheduled.
