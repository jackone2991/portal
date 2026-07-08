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
| [02](02-*.md) | *(see file)* | accepted | Role-based access now; policy bundles deferred |
| [03](03-*.md) | *(see file)* | accepted | |
| [04](04-*.md) | *(see file)* | accepted | |
| [05](05-*.md) | *(see file)* | accepted | |
| [06](06-local-auth-model.md) | Local auth model | accepted | Passwords in Portal (Argon2id + JWT); Authentik/OIDC removed |
| [07](07-*.md) | Multi-tenancy / RLS (design only) | accepted (deferred design) | |
| [08](08-life-os-pivot.md) | Life-OS pivot + finance ledger scope | **proposed** | Portal is a life OS; ledger in scope; "real bank" stays deferred |
| [09](09-docs-architecture.md) | Documentation architecture | **proposed** | Diátaxis-informed `docs/` tree; English canonical |
| [10](10-openapi-contract-direction.md) | OpenAPI contract direction | **proposed** | Spec-first, enforced: generate Go stubs + TS client; CI drift gate |

> Note: rows 02–05 and 07 keep their existing filenames from the migration; fill in
> their exact titles when running the move (they are unchanged in content).

## When to write an ADR

Write one when a choice (a) is expensive to reverse, (b) crosses module boundaries,
or (c) contradicts a previous ADR or the scope cut. Day-to-day feature decisions
belong in [product/feature-inventory.md](../product/feature-inventory.md) as `D-N`
entries; specs cite both kinds by ID.
