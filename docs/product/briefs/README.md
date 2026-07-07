# Feature Specs — Life-OS Pivot (2026-07-07)

Feature specifications produced by the brainstorm on 2026-07-07. They capture the
**pivot in positioning**: Portal is not a Facebook clone measured by feature parity —
it is a **self-hosted "life OS"**: one digital identity with facets for money, time,
learning, social, and entertainment. First two axes to build: **money** and
**entertainment (video / image / comic)**.

These specs sit downstream of [feature.md](../feature.md) (canonical decision log)
and supersede the *ordering* (not the content) of
[missing-features.md — Suggested next order](../missing-features.md).
Placement in the repo: `doc/en/feature/` with the `doc/vi/feature/` mirror (bilingual rule).

## Build order

| # | Spec | Module | Status | Why this order |
|---|------|--------|--------|----------------|
| 00 | [Life-OS pivot](00-life-os-pivot.md) | — (positioning) | Needs ADR-08 | Frames everything below |
| 01 | [Media image pipeline](01-media-image-pipeline.md) | `media` | Not started | Shared bottleneck: unlocks comics, avatars, photos, receipts |
| 02 | [Comic vertical](02-comic-vertical.md) | `comic` | Skeleton | First media → domain vertical; proves the pattern |
| 03 | [Finance ledger](03-finance-ledger.md) | `bank` | No code | First "life" domain; Money-Lover-class, import-ready schema |
| 04 | [Deferred / parking lot](04-deferred.md) | — | — | What was consciously set aside, and re-entry conditions |

## Conventions binding on all specs

- Migrations take the **next free numeric sequence** as `000N_<owning-module>_<desc>.up/down.sql`
  (0003–0007 are consumed through `0007_media_assets` — verify against the repo before writing).
- Cross-module access **only** through the other module's `api/` package; cross-module
  coupling via Asynq events `<module>:<event>`. No cross-module JOINs or FKs.
- All permission checks go through the RBAC engine / `RequirePermission`
  (grammar `<resource>:<action>[:<scope>]`).
- `shared/openapi.yaml` is the API contract; note §9 of missing-features.md — the
  auth paths in the spec must be fixed before or alongside new endpoints.
- Every doc here has a `vi/` mirror; keep the pair in sync.
