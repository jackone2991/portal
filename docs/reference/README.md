# Reference

**Status:** current · **Last verified:** 2026-09-11

Lookup material. This section **points at** canonical sources rather than copying
them — a copy is a lie waiting to happen.

| What you need | Canonical source |
|---|---|
| Module layout, boundaries, task/event naming rules | [`/backend/MODULES.md`](../../backend/MODULES.md) |
| API contract | [`/shared/openapi.yaml`](../../shared/openapi.yaml) — CI regenerates and diffs the codegen ([ADR-10](../adr/10-openapi-contract-direction.md)); what the gate does not prove (handler conformance) is [backlog](../product/backlog.md) P1 |
| Implementation status | [`/CLAUDE.md`](../../CLAUDE.md) § Current status — the one written owner (ADR-11). Verify against the code: a module is live iff it is constructed *and* `MountHTTP`'d in `backend/cmd/api/main.go`. Open work: [product/backlog.md](../product/backlog.md). Newest audit: [remaining-work-2026-08-25.md](../product/analysis/remaining-work-2026-08-25.md). |
| Session / repo conventions | [`/CLAUDE.md`](../../CLAUDE.md) |
| Asynq events & tasks registry | [events.md](events.md) *(lives here — it spans modules, so no module owns it)* |
| Permission grammar | `<resource>:<action>[:<scope>]`, wildcard rules — spec in [architecture/security.md](../architecture/security.md) |
| Decision IDs | `D-N` → [product/feature-inventory.md](../product/feature-inventory.md) · `ADR-N` → [adr/](../adr/README.md) |
| Requirement → test evidence | [TRACEABILITY-MATRIX.md](TRACEABILITY-MATRIX.md) — graded on named `_test.go` functions, re-checked 2026-09-11 |
