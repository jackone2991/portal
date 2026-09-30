# Feature Briefs — Life-OS

**Status:** current · **Last verified:** never

Feature briefs for the life-OS direction. Briefs 00–04 were produced by the
2026-07-07 brainstorm that captured the **pivot in positioning** (Portal is a
self-hosted **life OS**, not a Facebook clone — ratified as
[ADR-08](../../adr/08-life-os-pivot.md)); briefs 05–09 were added by the
2026-07-10 research pass. A brief is the *product* capture; when one is promoted
to build, it gets an implementation-ready spec in [../specs/](../specs/).

These briefs sit downstream of [feature-inventory.md](../feature-inventory.md)
(canonical decision log — cite `D-N` ids) and upstream of `../specs/`. This folder
is `docs/product/briefs/`; docs are **English-only** per
[ADR-09](../../adr/09-docs-architecture.md) (the former vi mirror was deleted in
`f11cf3f` — see [ADR-11](../../adr/11-docs-canonicalisation.md)).

## Build order

| # | Brief | Module | Spec | Status | Why this order |
|---|------|--------|------|--------|----------------|
| 00 | [Life-OS pivot](00-life-os-pivot.md) | — (positioning) | — | **Ratified as [ADR-08](../../adr/08-life-os-pivot.md)** | Frames everything below |
| 01 | [Media image pipeline](01-media-image-pipeline.md) | `media` | [SPEC-01](../specs/SPEC-01-media-image-pipeline.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | Shared bottleneck: unlocks comics, avatars, photos, receipts |
| — | Notification module | `notify` | [SPEC-04](../specs/SPEC-04-notification-module.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) — no brief, went straight to spec from the gap audit | Life-stream backbone; slots right after SPEC-01 (`media:asset_ready` consumer) |
| 02 | [Comic vertical](02-comic-vertical.md) | `comic` | [SPEC-02](../specs/SPEC-02-comic-vertical.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | First media → domain vertical; proves the pattern |
| 03 | [Finance ledger](03-finance-ledger.md) | `bank` | [SPEC-03](../specs/SPEC-03-finance-ledger.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | First "life" domain; Money-Lover-class, import-ready schema |
| 04 | [Deferred / parking lot](04-deferred.md) | — | — | Living list | What was consciously set aside, and re-entry conditions |
| 05 | [Journal (life-stream write path)](05-journal-life-stream.md) | `journal` | [SPEC-05](../specs/SPEC-05-journal.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | First real post type per ADR-08; zero hard deps |
| 06 | [Life-stream home](06-life-stream-home.md) | `journal` + home | [SPEC-06](../specs/SPEC-06-life-stream-home.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | The ADR-08 proof screen; needs 05 + event producers |
| 07 | [Continue rail (D-20)](07-continue-rail.md) | `media` | [SPEC-07](../specs/SPEC-07-continue-rail.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | Retention mechanic; video leg buildable today |
| 08 | [People registry](08-people-registry.md) | `people` | [SPEC-08](../specs/SPEC-08-people-registry.md) | Specced; built (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | "Mom's birthday in 3 days" — social facet at n=1 |
| 09 | [Platform ops: backup/restore](09-platform-ops.md) | `ops` | [SPEC-09](../specs/SPEC-09-platform-ops.md) | Specced; P0 built, P1.7 takeout unbuilt (see [TRACEABILITY-MATRIX](../../reference/TRACEABILITY-MATRIX.md)) | Land before SPEC-03 data accrues; no backup doc exists today |
| — | Ledger expansion | `bank` | [SPEC-10](../specs/SPEC-10-ledger-expansion.md) | Phase 1 built (debts); later phases open (no brief — design session 2026-09-11) | Extends SPEC-03 |
| — | Docs canonicalisation | — | [SPEC-11](../specs/SPEC-11-docs-canonicalisation.md) | Executed 2026-09-11 (no brief) | Docs + CI only |
| — | Journal attachments | `journal` | [SPEC-12](../specs/SPEC-12-journal-attachments.md) | Executed 2026-09-19 (no brief — issue #8) | Extends SPEC-05/06 |

Sequencing (mirrors the [specs README](../specs/README.md) "Build state and what
remains", which records what actually landed): SPEC-01 → SPEC-04 → 05 + 06 P0.1
(the `stream_items` projection must precede every producer below, SPEC-06 §1)
→ 02 → 09 P0 (backups — hard gate immediately **before** SPEC-03 ledger data
accrues) → 03 (after 09 P0 is live; in parallel with 07 as burst-filler); 09 P1
here or later, then 08 → 06 P0.2–P0.4 + P1 (read API, home, rail). ADR-10's
spec-first CI gate is in force; the `ServerInterface` retrofit is per module as
each is touched and gates nothing.

## Conventions binding on all briefs/specs

- Migrations take the **next free numeric sequence** as `000N_<owning-module>_<desc>.up/down.sql`
  (verify against `backend/db/migrations/` before writing — specs are also claiming numbers).
- Cross-module access **only** through the other module's `api/` package; cross-module
  coupling via Asynq events `<module>:<event>`, registered in
  [../../reference/events.md](../../reference/events.md) (definition of done).
- All permission checks go through the RBAC engine / `RequirePermission` — grammar
  is **strictly 2–3 segments** `<resource>:<action>[:<scope>]` (`rbac.Parse` rejects
  4-segment codes; see SPEC-04 §7's note before copying older 4-segment examples).
- `shared/openapi.yaml` is the API contract, spec-first
  ([ADR-10](../../adr/10-openapi-contract-direction.md), accepted 2026-07-11; the
  specs README "API contract" convention has the rule).
- Every new domain module **emits ≥1 bus event from day one** (ADR-08 rule).
