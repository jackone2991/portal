# ops module

Platform operations ([SPEC-03](../../../../docs/product/specs/SPEC-03-platform-ops.md)
P0): self-running Postgres backups, a restore path, a freshness sentinel, and
the queue console's permission codes. Owns the `ops:*` prefix.

This file says what the code on `HEAD` does. The target — a separate locked
backup bucket, age encryption, cluster globals, a drill in a throwaway
container — is [ADR-18](../../../../docs/product/specs/SPEC-03-platform-ops.md#adr-18)
(2026-10-19); every place `HEAD` still differs is a row in SPEC-03 §11.

## Owns these tables

`ops_backup_runs` (migration `0012_ops_backup_runs`) — the backup-run ledger.
**System-scoped**: no `user_id`, one ledger for the whole install (backups are a
platform concern). The `0012` migration also seeds the ops permission catalog:
`ops:read` / `queues:read` → `admin`, `takeout:read:own` / `takeout:write:own` →
`user` (the takeout codes are for P1.7, not yet built; an unseeded code 403s).

## Public surface (`api/`)

`internal/modules/ops/api` exposes only the cross-module contract:

- `TaskBackupDatabase = "ops:backup_database"` — the nightly task type.
- `EventBackupCompleted` / `EventBackupFailed` (+ their payload structs) — emit-only.

Everything else (service, handler, repository, the backup task body) is private.

## What runs where

- **`cmd/api`** mounts `GET /ops/status` (P0.5 freshness sentinel) under
  `RequireAuth` + `RequirePermission("ops:read")`. It also mounts the asynqmon
  queue console at `/admin/queues` (P1.6) — in `cmd/api/main.go`, not through
  this module — as one writable handler behind `queues:read`; the read/write
  split, `queues:write` and the same-origin check are unbuilt (SPEC-03 §11
  row 1).
- **`cmd/worker`** registers the `ops:backup_database` handler on the light mux and
  schedules it nightly at 03:00 UTC on the shared periodic scheduler (no OS
  cron — SPEC-04 P0.3 convention), with Asynq's default task options (§11
  row 5).
- **`cmd/opsenqueue`** is a throwaway dev helper that enqueues one
  `ops:backup_database` on demand; it hard-codes `dragonfly:6379` DB 1 rather
  than reading `ASYNQ_REDIS_URL` (§11 row 18).

## The nightly backup (P0.2), as shipped

1. Insert a `running` row.
2. `pg_dump -Fc --no-owner --no-privileges` connecting **directly to Postgres**
   (`BACKUP_DATABASE_URL`, NOT PgBouncer — a transaction pooler breaks
   pg_dump's session semantics). Its stdout is **spooled to a worker temp file**
   with a sha256 teed in the same pass (`io.MultiWriter`), then the file is
   uploaded with `Storage.Put` to `backups/pg/<yyyy-mm-dd>.dump` — the S3 client
   needs a seekable body, so a straight pipe fails. The store is the **media**
   bucket and key the worker already holds; the dump is plaintext and carries no
   roles, owners or grants (§11 rows 12, 14, 15). A failed `Put` is followed by
   a `Delete` of the same key, which can only remove an earlier same-day dump
   (§11 row 16).
3. Close the row (`ok` + size + key, or `failed` + reason). A failure is terminal,
   never a wedged `running`; the next night just runs again.
4. On success, overwrite `backups/pg/LATEST.json`
   `{storage_key, sha256, size_bytes, finished_at}` — the restore drill selects
   from this manifest, never by listing keys (a partial upload can't poison it).
5. **Retention**: keep the 7 most recent daily dumps + the 4 most recent weekly
   anchors (Sunday dumps); prune the rest. The current dump and the manifest are
   never pruned. The keep/prune selection is a pure, unit-tested function. (The
   spec anchors the weekly set on ISO weeks instead — §11 row 6.)

## Restore drill (P0.4)

`make restore-drill` (`backend/scripts/restore-drill.sh`) reads `LATEST.json`,
downloads + verifies the sha256, `pg_restore`s into a scratch DB and
sanity-checks it — inside the compose `postgres` container, which was removed on
2026-08-21, so on the host-Postgres topology it stops at its precondition (§11
rows 2–3). Runbook, with an interim stand-in:
[docs/operations/backup-restore.md](../../../../docs/operations/backup-restore.md).

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
| Event | `ops:backup_failed` | `{run_id, error}` | — (same; `notify:on_backup_failed` is SPEC-03 P0.6, unbuilt) |

## Audit event types (owned; D-25 taxonomy)

`ops.backup.completed` · `ops.backup.failed` (actor kind `system`).

## Talks to

- `platform/storage` — dump upload + retention prune.
- `platform/events` — emit-only `ops:backup_*`.
- `platform/audit` — `ops.backup.*` records.

Never imports another module's internals. The API server bridges account's
`RequireAuth` / `RequirePermission` in through `Deps` (ops never imports
`account/rbac`).

## Not built yet

P0.6 `notify:on_backup_failed`; P0.7 backup encryption and the `.env` copy, and
the rest of ADR-18 (§11 rows 12–15); P1.6's read/write split; P1.7 owner
takeout (`ops:takeout` / `ops:purge_exports` / the `ops_exports` table); P2
`ops:job_dead`.
