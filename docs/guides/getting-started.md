# Getting Started (dev)

**Status:** current · **Last verified:** 2026-07-07 — commands verified against the
running stack on this date; if something here disagrees with the repo, trust the
repo and fix this file.

## What you're running

Go 1.24 modular monolith (`cmd/api` Chi HTTP, `cmd/worker` Asynq + FFmpeg) +
Next.js 15 frontend, behind Traefik v3 on `*.portal.localhost`, with Postgres 17
(+ PgBouncer), DragonflyDB (cache + Asynq broker), and MinIO — all via Docker
Compose. Nothing (Go, node, sqlc) needs to be installed on the host: **all builds
and codegen run in containers**.

## Bring the stack up

```bash
# From the repo root — NOT with -f docker-compose.yml:
make up            # or: docker compose up -d
```

**Why not `-f docker-compose.yml`:** `docker-compose.override.yml` mounts the local
TLS certificate. Skip it and Traefik has no `portal.localhost` cert → HTTPS fails
with "unrecognized name". This bites after every machine/stack restart.

Dev login on the seeded stack: `demo@portal.localhost` / `Passw0rd123`.

## Everyday commands

| Command | Does |
|---|---|
| `make up` | full stack via compose (+ override) |
| `make migrate` | run DB migrations (`migrate/migrate` image) |
| `make sqlc` | regenerate query code (`sqlc/sqlc` image) — **never hand-edit `*.sql.go`** |
| `make openapi` | regenerate API stubs from `shared/openapi.yaml` |
| `make dev` | frontend dev loop |
| `make test` | test suite (in `golang:1.24` image) |

## Windows + Git Bash

Use `MSYS_NO_PATHCONV=1` and `$(pwd -W)` for any Docker volume mounts, or paths get
mangled by MSYS path conversion.

## Connection gotchas (read before debugging "weird" DB/queue errors)

- **Dev connects directly to `postgres:5432`**, not through PgBouncer — PgBouncer's
  transaction mode clashes with pgx prepared statements. Prod goes through PgBouncer.
- DragonflyDB runs with `--default_lua_flags=allow-undeclared-keys`; without it,
  Asynq's Lua scripts fail.
- Vidstack loads hls.js from a CDN → video playback needs internet even locally
  (bundling it is a known backlog item).

## Hard rules (the ones that bite)

1. **Module boundaries**: modules talk only via each other's `api/` package; no
   imports of another module's `service/handler/repository/query`; no cross-module
   JOINs. Cross-module coupling = Asynq events `<module>:<event>`
   ([reference/events.md](../reference/events.md)).
2. **Schema changes = migrations only**: `backend/db/migrations/000N_<owning-module>_<desc>.up/down.sql`,
   single numeric sequence — check the next free number against the repo.
3. **Never hand-edit generated files**: `internal/modules/*/repository/*.sql.go`,
   `internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`.
4. **All permission checks** via the RBAC engine / `RequirePermission`
   (grammar `<resource>:<action>[:<scope>]`) — no ad-hoc checks.
5. **Commits**: only when the owner asks; messages end with `Co-Authored-By: Claude`.

## Where to read next

New here? Follow the reading order in [../README.md](../README.md). Building a
feature? Its spec is in [../product/specs/](../product/specs/README.md); check
[`/MILESTONE_CHECKS.md`](../../MILESTONE_CHECKS.md) for what's actually real before
trusting any doc's status claims.
