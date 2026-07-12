# Backup & Restore Runbook

**Scope:** SPEC-09 P0 — nightly Postgres backups, the freshness sentinel, and the
quarterly restore drill. Followable start-to-finish by someone who did not write
the code.

---

## 1. What is (and isn't) backed up

**Postgres is the fragile asset, and it is the only thing this backup covers.**
The database holds every pointer into object storage; losing it orphans the media
even though the bytes survive. So the nightly job dumps Postgres and nothing else.

**Object storage (media) is NOT dumped through the database — by design.**

- **Prod media lives on Cloudflare R2** (11-nines-class durability). Its
  durability story is R2's, not ours. Turn on **bucket versioning / soft-delete**
  on the R2 bucket as operator guidance (a console setting, not code) so an
  accidental delete is recoverable.
- **Dev media (MinIO bind-mount `./data/minio`) is explicitly disposable.** It
  exists to exercise the machinery; nothing there is precious.

Never "fix" backup coverage by streaming buckets through the pg_dump job — that is
an explicit non-goal (SPEC-09 §3). If media durability ever needs more, it is a
storage-lifecycle change, not a database change.

**Not automated (manual, documented here):** `.env`, Traefik config, and MinIO/R2
credentials. Keep a copy of `.env` in your password manager or a sealed secret
store — a restored database is useless without the JWT signing keys and S3 creds
to run against it.

---

## 2. The nightly backup (automatic)

A periodic Asynq task `ops:backup_database` runs **nightly at 03:00 UTC**,
scheduled on the worker's shared scheduler (there is no OS cron). Each run:

1. Opens a `ops_backup_runs` row (`running`).
2. Streams `pg_dump -Fc` — connecting **directly to Postgres, not PgBouncer** —
   through a sha256 hasher straight into object storage at
   `backups/pg/<yyyy-mm-dd>.dump`.
3. Closes the row (`ok` + size + key, or `failed` + reason).
4. On success, overwrites `backups/pg/LATEST.json`
   `{storage_key, sha256, size_bytes, finished_at}`.
5. Prunes old dumps: keeps the **7 most recent daily** dumps + the **4 most
   recent weekly** (Sunday) dumps; never deletes the current dump or `LATEST.json`.
6. Emits `ops:backup_completed` / `ops:backup_failed` and writes an audit record
   (`ops.backup.completed` / `ops.backup.failed`).

A failure is terminal for that run (never a wedged `running`); the next night runs
normally.

### Configuration

`BACKUP_DATABASE_URL` (in `.env`) is the DSN pg_dump uses. It **MUST connect to
Postgres directly** (`postgres:5432`), not through PgBouncer — a transaction
pooler breaks the session semantics pg_dump relies on. Example:

```
BACKUP_DATABASE_URL=postgres://portal:change-me@postgres:5432/portal?sslmode=disable
```

If it is empty, the task self-reports a `failed` run (visible on `/ops/status`)
rather than crashing the worker.

The worker image ships `postgresql17-client` (pg_dump + pg_restore), major-version
pinned to the Postgres 17 server.

---

## 3. The freshness sentinel

`GET /api/v1/ops/status` (permission `ops:read`, admin-tier) returns:

```json
{
  "last_success_at": "2026-07-12T03:00:11Z",
  "hours_since_success": 5.4,
  "state": "ok",
  "last_run": { "id": "…", "status": "ok", "started_at": "…", "finished_at": "…",
                "size_bytes": 1048576, "storage_key": "backups/pg/2026-07-12.dump",
                "error": null }
}
```

`state`, first match wins:

- **`failed`** — the most recent *completed* run failed (something is actively
  broken; wins even if an older success is still fresh).
- **`stale`** — no success within 26 h, **including never-ran** (then
  `last_success_at` and `hours_since_success` are `null`).
- **`ok`** — otherwise.

A `running` run is not completed and never changes `state`.

**Check it every morning.** If `state` is anything but `ok`, read `last_run.error`
and the worker logs. `/healthz` is deliberately separate (unauthenticated liveness)
— backup staleness must not restart-loop a healthy API.

---

## 4. Restore drill (quarterly — DO THIS)

A backup that has never been restored is a hope, not a property. Run the drill on
a fresh dev stack once per quarter (a calendar practice, not automation) and after
any change to the backup path.

```bash
make restore-drill
```

### What it does

1. Reads `backups/pg/LATEST.json` from storage (**the manifest — never
   latest-by-key-listing**, which a partial upload could poison).
2. Downloads the dump and **verifies its sha256** against the manifest.
3. `pg_restore`s into a throwaway scratch DB (`portal_restore_check`); the restore
   must exit 0.
4. Sanity-checks the restored DB (each meaningful without knowing the dump's
   history):
   - `schema_migrations.version` ≤ the checked-out repo's latest migration number;
   - `users` and `assets` return plausible counts;
   - the 7 system roles seeded by `0003_account_rbac` are present.
5. Drops the scratch DB and prints `RESTORE DRILL PASSED`.

It consults **no** application table for selection — a fresh dev stack has no
`ops_backup_runs` to read; the manifest is the single source of truth.

### Prerequisites

`aws` (S3-compatible CLI), `jq`, `postgresql17-client` (`pg_restore` + `psql`),
and `sha256sum` or `shasum` on the host. The scratch-DB connection must reach
Postgres **directly** (not PgBouncer).

### Connection knobs

The script auto-reads `S3_*` and `POSTGRES_*` from `.env`. Because the drill runs
from the host (not inside a container), point it at Postgres with:

```bash
# defaults shown; override if your Postgres isn't on localhost:5432
RESTORE_PGHOST=localhost RESTORE_PGPORT=5432 make restore-drill
```

If Postgres is only reachable inside the compose network, either publish its port
or run the drill from a container on the `internal` network. `S3_ENDPOINT` in dev
is the MinIO container URL — expose it or run against a host-reachable endpoint.

### Manual restore into production (real disaster)

The drill restores into a scratch DB. To restore **for real**:

1. Stop the API + worker (`make down` or scale to 0) so nothing writes.
2. Download + verify the dump exactly as the drill does (or copy it out of the
   scratch flow).
3. `pg_restore --clean --if-exists --no-owner --no-privileges -d <prod-dsn>
   <dump>` into the live database (take a fresh dump first if the DB is reachable).
4. Confirm `schema_migrations.version` matches the running code's expected head;
   run `make migrate` if the code is ahead of the dump.
5. Restore `.env` (JWT keys, S3 creds) from your secret store, bring the stack up,
   and hit `/ops/status` + `/healthz`.

---

## 5. Success metrics (n=1 honest)

- 7/7 mornings in the first week show a fresh dated dump off-VPS;
  `hours_since_success` never exceeds 26 outside induced failures.
- One restore drill executed and documented before this spec closes; quarterly
  thereafter, calendar-verified.
