# Architecture Decision Records

**Status:** current · **Last verified:** 2026-07-07

One record per significant decision. Accepted ADRs are immutable — supersede or
add a dated revision note (house pattern: `D-26.r1`-style), never rewrite history.
Shape is binding: **context → decision → options considered → trade-offs →
consequences → action items** ([STYLE.md](../STYLE.md)).

The v1 framing constraint every ADR inherits: **1 dev · 2-week bursts · ≤$100/mo ·
single VPS** (ADR-01).

## Index

| ADR | Title | Status | One line |
|---|---|---|---|
| [00](00-architecture-review.md) | Architecture review (2026-05-24) | accepted | Findings that motivated ADRs 01–05 |
| [01](01-v1-scope-cut.md) | v1 scope cut | accepted, amended by 08 | What v1 is — and everything it is not |
| [02](02-rbac-model-reconciliation.md) | RBAC model reconciliation | accepted | Role hierarchy is canonical for v1; policy bundles layer on top later |
| [03](03-single-vps-topology.md) | Single-VPS topology | accepted | One VPS, compose profiles as the envelope; observability/live profiles stay off |
| [04](04-storage-tier-budget.md) | Storage tier & budget | accepted | R2 for prod, MinIO kept for local dev (presigned uploads need an S3 origin) |
| [05](05-phase0-wiring-order.md) | Phase 0 wiring order | accepted | The critical path to a running demo — closed; kept for the shape of the work |
| [06](06-local-auth-model.md) | Local auth model | accepted | Passwords in Portal (Argon2id + JWT); Authentik/OIDC removed |
| [07](07-tenancy-rls-model.md) | Multi-tenancy / RLS model | accepted | Tenant column + RLS policies; enforced only when the app connects as `portal_app` |
| [08](08-life-os-pivot.md) | Life-OS pivot + finance ledger scope | **proposed** | Portal is a life OS; ledger in scope; "real bank" stays deferred |
| [09](09-docs-architecture.md) | Documentation architecture | **proposed** | Diátaxis-informed `docs/` tree; English canonical |
| [10](10-openapi-contract-direction.md) | OpenAPI contract direction | **proposed** | Spec-first, enforced: generate Go stubs + TS client; CI drift gate |
| [11](11-docs-canonicalisation.md) | Documentation canonicalisation | accepted | One owner per fact; ADRs corrected in place by layer; nothing archived |


## When to write an ADR

Write one when a choice (a) is expensive to reverse, (b) crosses module boundaries,
or (c) contradicts a previous ADR or the scope cut. Day-to-day feature decisions
belong in [product/feature-inventory.md](../product/feature-inventory.md) as `D-N`
entries; specs cite both kinds by ID.
