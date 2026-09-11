# Architecture Decision Records

**Status:** current · **Last verified:** 2026-09-11

One record per significant decision. An accepted ADR is **corrected in place, by
layer** ([ADR-11](11-docs-canonicalisation.md)): its fact clauses — Context as
"the state this was decided against", Consequences as what actually followed,
action-item boxes — are kept true and `Last verified` is bumped; its *Options
considered* and *Trade-offs* are kept verbatim. A reversed decision gets a new
ADR that supersedes the old one. Shape is binding: **context → decision →
options considered → trade-offs → consequences → action items**
([STYLE.md](../STYLE.md)). Numbers are never reused; `00` is retired — that
file was a review, and lives in
[product/analysis/](../product/analysis/architecture-review-2026-05-24.md).

The v1 framing constraint every ADR inherits: **1 dev · 2-week bursts · ≤$100/mo ·
single VPS** (ADR-01).

## Index

| ADR | Title | Status | One line |
|---|---|---|---|
| [01](01-v1-scope-cut.md) | v1 scope cut | accepted, amended by 08 | What v1 is — and everything it is not |
| [02](02-rbac-model-reconciliation.md) | RBAC model reconciliation | accepted | Role hierarchy is canonical for v1; policy bundles layer on top later |
| [03](03-single-vps-topology.md) | Single-VPS topology | accepted | One VPS, compose profiles as the envelope; observability/live profiles stay off |
| [04](04-storage-tier-budget.md) | Storage tier & budget | accepted | R2 for prod, MinIO kept for local dev (presigned uploads need an S3 origin) |
| [05](05-phase0-wiring-order.md) | Phase 0 wiring order | accepted | The critical path to a running demo — closed; kept for the shape of the work |
| [06](06-local-auth-model.md) | Local auth model | accepted | Passwords in Portal (Argon2id + JWT); Authentik/OIDC removed |
| [07](07-tenancy-rls-model.md) | Multi-tenancy / RLS model | accepted, executed | Tenant column + RLS policies; enforced only when the app connects as `portal_app` |
| [08](08-life-os-pivot.md) | Life-OS pivot + finance ledger scope | accepted | Portal is a life OS; ledger in scope; "real bank" stays deferred |
| [09](09-docs-architecture.md) | Documentation architecture | accepted, amended by 11 | Diátaxis-informed `docs/` tree; English canonical |
| [10](10-openapi-contract-direction.md) | OpenAPI contract direction | accepted | Spec-first, enforced: generate Go stubs + TS client; CI drift gate |
| [11](11-docs-canonicalisation.md) | Documentation canonicalisation | accepted | One owner per fact; ADRs corrected in place by layer; nothing archived |


## When to write an ADR

Write one when a choice (a) is expensive to reverse, (b) crosses module boundaries,
or (c) contradicts a previous ADR or the scope cut. Day-to-day feature decisions
belong in [product/feature-inventory.md](../product/feature-inventory.md) as `D-N`
entries; specs cite both kinds by ID.
