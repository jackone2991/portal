# Feature Briefs — Life-OS

Feature briefs for the life-OS direction. Briefs 00–04 were produced by the
2026-07-07 brainstorm that captured the **pivot in positioning** (Portal is a
self-hosted **life OS**, not a Facebook clone — ratified as
[ADR-08](../../adr/08-life-os-pivot.md)); briefs 05–09 were added by the
2026-07-10 research pass. A brief is the *product* capture; when one is promoted
to build, it gets an implementation-ready spec in [../specs/](../specs/).

These briefs sit downstream of [feature-inventory.md](../feature-inventory.md)
(canonical decision log — cite `D-N` ids) and upstream of `../specs/`. This folder
is `docs/product/briefs/`; docs are **English-only** per
[ADR-09](../../adr/09-docs-architecture.md) (the old `doc/vi` mirror is a frozen
archive — never update it).

## Build order

| # | Brief | Module | Spec | Status | Why this order |
|---|------|--------|------|--------|----------------|
| 00 | [Life-OS pivot](00-life-os-pivot.md) | — (positioning) | — | **Ratified as [ADR-08](../../adr/08-life-os-pivot.md)** | Frames everything below |
| 01 | [Media image pipeline](01-media-image-pipeline.md) | `media` | [SPEC-01](../specs/SPEC-01-media-image-pipeline.md) | Specced, unbuilt | Shared bottleneck: unlocks comics, avatars, photos, receipts |
| — | Notification module | `notify` | [SPEC-04](../specs/SPEC-04-notification-module.md) | Specced, unbuilt (no brief — went straight to spec from the gap audit) | Life-stream backbone; slots right after SPEC-01 (`media:asset_ready` consumer) |
| 02 | [Comic vertical](02-comic-vertical.md) | `comic` | [SPEC-02](../specs/SPEC-02-comic-vertical.md) | Specced, unbuilt | First media → domain vertical; proves the pattern |
| 03 | [Finance ledger](03-finance-ledger.md) | `bank` | [SPEC-03](../specs/SPEC-03-finance-ledger.md) | Specced, unbuilt | First "life" domain; Money-Lover-class, import-ready schema |
| 04 | [Deferred / parking lot](04-deferred.md) | — | — | Living list | What was consciously set aside, and re-entry conditions |
| 05 | [Journal (life-stream write path)](05-journal-life-stream.md) | `journal` | [SPEC-05](../specs/SPEC-05-journal.md) | Specced 2026-07-10, unbuilt | First real post type per ADR-08; zero hard deps |
| 06 | [Life-stream home](06-life-stream-home.md) | `journal` + home | [SPEC-06](../specs/SPEC-06-life-stream-home.md) | Specced 2026-07-10, unbuilt | The ADR-08 proof screen; needs 05 + event producers |
| 07 | [Continue rail (D-20)](07-continue-rail.md) | `media` | [SPEC-07](../specs/SPEC-07-continue-rail.md) | Specced 2026-07-10, unbuilt | Retention mechanic; video leg buildable today |
| 08 | [People registry](08-people-registry.md) | `people` | [SPEC-08](../specs/SPEC-08-people-registry.md) | Specced 2026-07-10, unbuilt | "Mom's birthday in 3 days" — social facet at n=1 |
| 09 | [Platform ops: backup/restore](09-platform-ops.md) | `ops` | [SPEC-09](../specs/SPEC-09-platform-ops.md) | Specced 2026-07-10, unbuilt | Land before SPEC-03 data accrues; no backup doc exists today |

Suggested sequencing: **ADR-10 codegen cutover first** (cross-cutting gate — every
brief above adds endpoints that should be born under it), then SPEC-01 → SPEC-04 →
05 → 02 → 09 P0 (backups — must land **before** SPEC-03 ledger data accrues) →
03 (parallelizable) with 07 as burst-filler (dependency-free); 09 P1 here or
later, then 08 → 06.

## Conventions binding on all briefs/specs

- Migrations take the **next free numeric sequence** as `000N_<owning-module>_<desc>.up/down.sql`
  (verify against `backend/db/migrations/` before writing — specs are also claiming numbers).
- Cross-module access **only** through the other module's `api/` package; cross-module
  coupling via Asynq events `<module>:<event>`, registered in
  [../../reference/events.md](../../reference/events.md) (definition of done).
- All permission checks go through the RBAC engine / `RequirePermission` — grammar
  is **strictly 2–3 segments** `<resource>:<action>[:<scope>]` (`rbac.Parse` rejects
  4-segment codes; see SPEC-04 §7's note before copying older 4-segment examples).
- `shared/openapi.yaml` is the API contract; [ADR-10](../../adr/10-openapi-contract-direction.md)
  (spec-first + CI drift gate) should land before new endpoint families.
- Every new domain module **emits ≥1 bus event from day one** (ADR-08 rule).
