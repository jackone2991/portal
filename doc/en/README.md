# Portal — Documentation Index

This tree is the **design corpus** for Portal: specs, decision records, diagrams, and analyses.
Live implementation status lives in [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md); operational
how-to and repo conventions live in [CLAUDE.md](../../CLAUDE.md); the backend module contract is
[backend/MODULES.md](../../backend/MODULES.md).

## Start here

New contributor? Read in this order:

1. [architecture/README.md](architecture/README.md) — ADR index and the v1 framing constraint (1 dev · 2 weeks · ≤ $100/mo · 1 VPS).
2. [architecture/01-v1-scope-cut.md](architecture/01-v1-scope-cut.md) — what v1 actually is (and everything it is not).
3. [architecture/06-local-auth-model.md](architecture/06-local-auth-model.md) — the current auth model (local passwords; Authentik/OIDC removed).
4. [feature.md](feature.md) — the canonical decision log `D-1`…`D-40` and full feature inventory.
5. [diagrams.md](diagrams.md) — the visual architecture map.
6. [backend/MODULES.md](../../backend/MODULES.md) — module boundaries before you write any backend code.

## All documents

| File | Role | Status | One line |
| --- | --- | --- | --- |
| [feature.md](feature.md) | Decision log | Current | Per-module feature inventory, phased roadmap, open questions, and decisions `D-1`…`D-40`. |
| [checklist.md](checklist.md) | Checklist | Current | Whole-project work checklist: every Phase 0–12 deliverable as a checkbox, tied to `D-N` IDs and current status. |
| [diagrams.md](diagrams.md) | Diagrams | Current | Mermaid map: system landscape, module boundaries, request/login flows, media pipeline, roadmap. |
| [authoration.md](authoration.md) | Spec | Current | Security spec: authentication, tokens, revocation, authorization (RBAC/RLS), threat model. |
| [frontend.md](frontend.md) | Spec | Current | Next.js 15 frontend: versioned template layer, routing, state, auth handoff, page inventory. |
| [archivetech.md](archivetech.md) | Spec | Deferred design (post-v1) | Policy-bundle / user-group / file-gated access control — layers on top of roles per ADR-02. |
| [archivetech-backend.md](archivetech-backend.md) | Spec | Deferred design (post-v1) | Multi-tenant Go backend: RLS-scoped transactions, tenancy across Asynq/storage/cache, milestones M0–M5. |
| [facebook-comparison.md](facebook-comparison.md) | Analysis | Snapshot (2026-07-06) | Feature-by-feature comparison of Portal against Facebook (the Olympus UI's yardstick). |
| [missing-features.md](missing-features.md) | Analysis | Snapshot (2026-07-06) | Post-v1 gap analysis against feature.md — the backlog for choosing what to build next. |
| [architecture/README.md](architecture/README.md) | Index | Index | ADR set index, authoring conventions, and the 1 dev / 2 weeks / $100/mo / 1 VPS envelope. |
| [architecture/00-architecture-review.md](architecture/00-architecture-review.md) | Review / analysis | Accepted ADR | Dated (2026-05-24) architecture review whose findings motivated ADRs 01–05. |
| [architecture/01-v1-scope-cut.md](architecture/01-v1-scope-cut.md) | ADR | Accepted ADR | v1 = Phase 0 wiring + one video-upload happy path; bank/social/marketplace deferred. |
| [architecture/02-rbac-model-reconciliation.md](architecture/02-rbac-model-reconciliation.md) | ADR | Accepted ADR | Role hierarchy is canonical; archivetech policy bundles become a later additive layer. |
| [architecture/03-single-vps-topology.md](architecture/03-single-vps-topology.md) | ADR | Accepted ADR | Single-VPS hosting envelope: v1 service set, disabled compose profiles, RAM/cost budget. |
| [architecture/04-storage-tier-budget.md](architecture/04-storage-tier-budget.md) | ADR | Accepted ADR | R2-only storage in deployed environments; MinIO bind-mount for local dev (2026-06-06 update). |
| [architecture/05-phase0-wiring-order.md](architecture/05-phase0-wiring-order.md) | ADR | Accepted ADR | Strict ordering of the Phase 0 wiring gap: migrations → sqlc → adapters → wiring → auth → frontend. |
| [architecture/06-local-auth-model.md](architecture/06-local-auth-model.md) | ADR | Accepted ADR | Portal owns credentials (Argon2id); Authentik/OIDC dropped; token/RBAC/revocation machinery reused. |
| [architecture/diagrams/system-landscape.md](architecture/diagrams/system-landscape.md) | Diagrams | Current (v1-scoped) | What the deploy actually starts, plus the sign-in and upload → transcode → playback happy paths. |
| [architecture/diagrams/request-flow.md](architecture/diagrams/request-flow.md) | Diagrams | Current (v1-scoped) | Authenticated request sequence through `RequireAuth` → `RequirePermission` → handler. |

---

`doc/vi/` is the Vietnamese mirror of this tree and may lag behind `doc/en/`.
