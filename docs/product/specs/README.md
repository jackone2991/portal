# Portal — Detailed Feature Specs (PRD level)

**Language policy:** English only, per owner decision 2026-07-07 (ratified by
[ADR-09](../../adr/09-docs-architecture.md) — the old vi mirror is a frozen archive).

This folder holds the **detailed, implementation-ready specs** for the features
captured in [../briefs/](../briefs/). A brief answers *what and why in brief*;
a spec answers *exactly what to build, how to know it's done, and what to decide
before starting*. A brief is promoted to a spec when it's queued for build
(briefs README tracks the mapping).

## Documents

| Spec | Feature | Module | Depends on |
|------|---------|--------|------------|
| [SPEC-01](SPEC-01-media-image-pipeline.md) | Media image pipeline + asset management | `media` | — |
| [SPEC-02](SPEC-02-comic-vertical.md) | Comic vertical, end-to-end | `comic` | SPEC-01 |
| [SPEC-03](SPEC-03-finance-ledger.md) | Finance ledger (Money-Lover-class) | `bank` | ADR-08; SPEC-01 (P1 receipts only) |
| [SPEC-04](SPEC-04-notification-module.md) | Notification module (life-stream backbone) | `notify` (new) | SPEC-01 P1.2 for its P0.4 consumer (rest is dependency-free; unblocks account password reset) |
| [SPEC-05](SPEC-05-journal.md) | Journal — life-stream write path | `journal` (new) | — (P1 photos need SPEC-01) |
| [SPEC-06](SPEC-06-life-stream-home.md) | Life-stream home — projection + dashboard | `journal` + home | SPEC-05; producers attach as they land |
| [SPEC-07](SPEC-07-continue-rail.md) | Playback resume + continue rail (D-20) | `media` | — (comic leg joins after SPEC-02) |
| [SPEC-08](SPEC-08-people-registry.md) | People registry — contacts + birthdays | `people` (new) | — (P1 avatars need SPEC-01) |
| [SPEC-09](SPEC-09-platform-ops.md) | Platform ops — backup/restore, queue console, takeout | `ops` (new) | — (land P0 before SPEC-03 data accrues) |

The positioning decision (life-OS pivot) and the parking lot are **not** specs;
they remain in [../briefs/00-life-os-pivot.md](../briefs/00-life-os-pivot.md)
(→ [ADR-08](../../adr/08-life-os-pivot.md)) and
[../briefs/04-deferred.md](../briefs/04-deferred.md).

## Conventions binding on all specs

- **Migrations**: next free numeric sequence, `000N_<owning-module>_<desc>.up/down.sql`.
  0001–0007 were consumed as of 2026-07-06 (`0007_media_assets`); **verify against
  the repo** before assigning numbers — specs are written as `000N_*` placeholders
  and several are claiming numbers concurrently.
- **Module boundaries**: cross-module access only via the other module's `api/`
  package; coupling via Asynq events `<module>:<event>`; no cross-module JOINs or
  FKs — with one sanctioned exception: the **identity-anchor FK** to `users(id)`
  (SPEC-04 §6, matching the `0007_media_assets` precedent).
- **Events**: every task/event name lands in
  [../../reference/events.md](../../reference/events.md) as part of definition of
  done; every new domain module emits ≥ 1 bus event from day one (ADR-08). The
  `platform/events` fan-out helper (`Publish(ctx, name, payload)` + the
  event-name→consumer-task subscription table registered in `cmd/worker`) is a
  prerequisite of the first spec to land; SPEC-01 P0 owns building it.
- **Errors**: RFC 7807 `Problem` on every non-2xx (D-7). Error `type` URIs given in
  each spec are also the i18n keys. **DoD**: every new Problem `type` URI is
  registered as an i18n message key in the frontend i18n catalog (per
  `frontend.md` §5) in the same PR that introduces the endpoint — mirroring
  the events.md registration rule; a type URI with no i18n key is a DoD
  failure.
- **AuthZ**: every endpoint lists its required permission (or an explicit
  "authenticated"), enforced via `RequirePermission`; the grammar is **strictly
  2–3 segments** `<resource>:<action>[:<scope>]` — `rbac.Parse` rejects
  4-segment codes and `AllowsCode` is fail-closed. **Canonical naming scheme**
  (reconciles the drafts; matches `rbac/permission.go`'s examples and the
  0003 catalog's domain rows (`assets:*`, `movies:*`, ...) — 0003's
  admin-plane codes (`rbac:role:*`, `system:settings:write`, `moderation:*`)
  predate this scheme and are grandfathered literal codes; they must not be
  used as templates for new modules): resource = plain noun, kebab-compound
  where needed (`assets`, `comics`, `bank-accounts`, `notification-prefs`);
  action ∈ `read | write | delete` plus sparing domain verbs (`publish`);
  scope ∈ `own | any` **only** — the matcher special-cases exactly those (a
  literal scope like `published` would only ever match its own literal grant).
  **Seeding rule**: a spec that introduces a permission also names the
  receiving role and ships the `permissions` + `role_permissions` seed rows in
  its own migration (0003's `WITH grants(...)` pattern) — an unseeded code
  403s everyone below superadmin. The OpenAPI `x-required-permission`
  extension applies per [security.md](../../architecture/security.md)'s
  target convention. The single v1 owner is provisioned `creator` or higher;
  grants to `user` are the floor.
- **API contract**: every endpoint added here must land in `shared/openapi.yaml`;
  [ADR-10](../../adr/10-openapi-contract-direction.md) (spec-first + CI drift
  gate) should land before new endpoint families so they are born under it.
- **Money**: integer minor units, never floats. VND exponent = 0. *(Known,
  deliberate divergence from D-14's `numeric(20,8)`/string-wire rule and
  D-15's double-entry internals for the v1 ledger scope — SPEC-03 §7 carries
  the rationale and the obligation to record it as a new decision entry.)*
- **Frontend**: RSC-first (D-33), TanStack owns server state (D-32), performance
  budgets from `frontend.md` §8 apply to all new pages.

## Suggested implementation order

Per the [briefs build order](../briefs/README.md): **ADR-10 codegen cutover
first** (cross-cutting gate), then

1. **SPEC-01** (shared bottleneck: unlocks comics, avatars, photos, receipts)
2. **SPEC-04** (slots right after — consumer for `media:asset_ready`; unblocks
   password reset)
3. **SPEC-05** (first life-stream write path; zero hard deps)
4. **SPEC-02** and **SPEC-09 P0** (backups — must land **before** any SPEC-03
   ledger data)
5. **SPEC-03** (parallelizable) / **SPEC-07** (burst-filler); **SPEC-09 P1**
   here or later
6. **SPEC-08**, then **SPEC-06** last (the proof screen — it consumes everything
   above and every widget degrades to an empty state until its producer lands)
