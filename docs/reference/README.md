# Reference

**Status:** current · **Last verified:** 2026-07-07

Lookup material. This section **points at** canonical sources rather than copying
them — a copy is a lie waiting to happen.

| What you need | Canonical source |
|---|---|
| Module layout, boundaries, task/event naming rules | [`/backend/MODULES.md`](../../backend/MODULES.md) |
| API contract | [`/shared/openapi.yaml`](../../shared/openapi.yaml) — note the known drift (~30 live routes were absent from the spec — closed 2026-08-25; `/auth/register` was never missing and `/auth/callback` was already gone); see `product/backlog.md` §9 |
| Implementation status | **No tracker file.** `MILESTONE_CHECKS.md` was deleted in `f11cf3f`. Verify status against the code: a module is live iff it is constructed *and* `MountHTTP`'d in `backend/cmd/api/main.go`. The most recent audit is [remaining-work-2026-08-25.md](../product/analysis/remaining-work-2026-08-25.md). |
| Session / repo conventions | [`/CLAUDE.md`](../../CLAUDE.md) |
| Asynq events & tasks registry | [events.md](events.md) *(lives here — it spans modules, so no module owns it)* |
| Permission grammar | `<resource>:<action>[:<scope>]`, wildcard rules — spec in [architecture/security.md](../architecture/security.md) |
| Decision IDs | `D-N` → [product/feature-inventory.md](../product/feature-inventory.md) · `ADR-N` → [adr/](../adr/README.md) |
