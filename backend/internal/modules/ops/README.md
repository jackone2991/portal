# ops module

Platform operations (SPEC-09 P0): self-running Postgres backups, a tested
restore path, and a freshness sentinel. Owns the `ops:*` prefix.

## Owns these tables

`ops_backup_runs` (migration `0012_ops_backup_runs`) — the backup-run ledger.
**System-scoped**: no `user_id`, one ledger for the whole install (backups are a
platform concern). The `0012` migration also seeds the ops permission catalog:
`ops:read` / `queues:read` → `admin`, `takeout:read:own` / `takeout:write:own` →
`user` (the last three are for P1 features not yet built; an unseeded code 403s).

## Public surface (`api/`)

`internal/modules/ops/api` exposes only the cross-module contract:

- `TaskBackupDatabase = "ops:backup_database"` — the nightly task type.
- `EventBackupCompleted` / `EventBackupFailed` (+ their payload structs) — emit-only.

Everything else (service, handler, repository, the backup task body) is private.

## What runs where

- **`cmd/api`** mounts `GET /ops/status` (P0.5 freshness sentinel) under
  `RequireAuth` + `RequirePermission("ops:read")`.
- **`cmd/worker`** registers the `ops:backup_database` handler on the light mux and
  schedules it nightly on the shared periodic scheduler (no OS cron — SPEC-01
  P0.3 convention).

## The nightly backup (P0.2)

1. Insert a `running` row.
2. `pg_dump -Fc` connecting **directly to Postgres** (`BACKUP_DATABASE_URL`, NOT
   PgBouncer — a transaction pooler breaks pg_dump's session semantics), streamed
   through a sha256 hasher (`io.TeeReader`) straight into `platform/storage` at
   `backups/pg/<yyyy-mm-dd>.dump` — one pass, no temp file.
3. Close the row (`ok` + size + key, or `failed` + reason). A failure is terminal,
   never a wedged `running`; the next night just runs again.
4. On success, overwrite `backups/pg/LATEST.json`
   `{storage_key, sha256, size_bytes, finished_at}` — the restore drill selects
   from this manifest, never by listing keys (a partial upload can't poison it).
5. **Retention**: keep the 7 most recent daily dumps + the 4 most recent weekly
   anchors (Sunday dumps); prune the rest. The current dump and the manifest are
   never pruned. The keep/prune selection is a pure, unit-tested function.

## Restore drill (P0.4)

`make restore-drill` reads `LATEST.json`, downloads + verifies the sha256,
`pg_restore`s into a scratch DB, and sanity-checks it. Runbook:
[docs/guides/backup-restore.md](../../../../docs/guides/backup-restore.md).

## Freshness sentinel (P0.5)

`GET /ops/status` → `{last_success_at, hours_since_success, state, last_run}`.
`state` precedence, first match wins:

1. `failed` — the most recent COMPLETED run failed.
2. `stale` — no success within 26 h, **including never-ran** (`last_success_at`
   and `hours_since_success` both null).
3. `ok` — otherwise.

A `running` run is not completed and never changes state.

## Emits (docs/reference/events.md)

| Kind | Name | Payload | Consumers |
|---|---|---|---|
| Task | `ops:backup_database` | — (nightly sweep) | ops (self) |
| Event | `ops:backup_completed` | `{run_id, size_bytes}` | — (audit + `/ops/status`; notify later) |
| Event | `ops:backup_failed` | `{run_id, error}` | — (same) |

## Audit event types (owned; D-25 taxonomy)

`ops.backup.completed` · `ops.backup.failed` (actor kind `system`).

## Talks to

- `platform/storage` — dump upload + retention prune.
- `platform/events` — emit-only `ops:backup_*`.
- `platform/audit` — `ops.backup.*` records.

Never imports another module's internals. The API server bridges account's
`RequireAuth` / `RequirePermission` in through `Deps` (ops never imports
`account/rbac`).

## Deferred (do not build under P0)

P1.6 queue console (`/admin/queues` asynqmon), P1.7 owner takeout
(`ops:takeout` / `ops:purge_exports` / the `ops_exports` table). P2: backup
encryption at rest, `ops:job_dead`.
