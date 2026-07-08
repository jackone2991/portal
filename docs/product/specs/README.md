# Portal — Detailed Feature Specs (PRD level)

**Language policy:** English only, per owner decision 2026-07-07. This folder is
exempt from the `doc/en` / `doc/vi` mirror rule; no Vietnamese mirrors are maintained.

This folder holds the **detailed, implementation-ready specs** for the features
outlined in `doc/*/feature/` (the brainstorm-level folder from 2026-07-07). The
`feature/` folder answers *what and why in brief*; this folder answers *exactly what
to build, how to know it's done, and what to decide before starting*.

## Documents

| Spec | Feature | Module | Depends on |
|------|---------|--------|------------|
| [SPEC-01](SPEC-01-media-image-pipeline.md) | Media image pipeline + asset management | `media` | — |
| [SPEC-02](SPEC-02-comic-vertical.md) | Comic vertical, end-to-end | `comic` | SPEC-01 |
| [SPEC-03](SPEC-03-finance-ledger.md) | Finance ledger (Money-Lover-class) | `bank` | ADR-08; SPEC-01 (P1 receipts only) |
| [SPEC-04](SPEC-04-notification-module.md) | Notification module (life-stream backbone) | `notify` (new) | — (consumes `media:asset_ready`; unblocks account password reset) |

The positioning decision (life-OS pivot) and the parking lot are **not** specs; they
remain in `feature/00-life-os-pivot.md` (→ ADR-08) and `feature/04-deferred.md`.

## Conventions binding on all three specs

- **Migrations**: next free numeric sequence, `000N_<owning-module>_<desc>.up/down.sql`.
  0003–0007 were consumed as of 2026-07-06 (`0007_media_assets`); **verify against
  the repo** before assigning numbers — they are written as `000N_*` placeholders here.
- **Module boundaries**: cross-module access only via the other module's `api/`
  package; coupling via Asynq events `<module>:<event>`; no cross-module JOINs or FKs.
- **Errors**: RFC 7807 `Problem` on every non-2xx (D-7). Error `type` URIs given in
  each spec are also the i18n keys.
- **AuthZ**: every endpoint lists its required permission (grammar
  `<resource>:<action>[:<scope>]`), enforced via `RequirePermission`; the OpenAPI
  `x-required-permission` extension should be set per D-29's direction.
- **API contract**: every endpoint added here must land in `shared/openapi.yaml`.
  Known pre-existing drift (missing `/auth/register`, stale `/auth/callback`) should
  be fixed in the same or an earlier PR so the spec file doesn't rot further.
- **Money**: integer minor units, never floats. VND exponent = 0.
- **Frontend**: RSC-first (D-33), TanStack owns server state (D-32), performance
  budgets from `frontend.md` §8 apply to all new pages.

## Suggested implementation order

SPEC-01 → SPEC-02 → SPEC-03, matching the dependency arrows. SPEC-03 has no hard
dependency on the other two and can be parallelized if desired; only its P1 receipt
attachment needs SPEC-01.

**SPEC-04 (notification)** has no hard dependency and can land any time, but its payoff
compounds: it unblocks account password reset immediately and becomes the consumer for
every producer the other specs add (`media:asset_ready`, later finance reminders). Per the
[gap audit](../analysis/gap-audit-2026-07.md) it is priority #2 — slot it right after
SPEC-01 so `media:asset_ready` has a consumer as soon as it's emitted.
