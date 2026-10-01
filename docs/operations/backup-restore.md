# Backup & Restore Runbook

**Status:** current · **Last verified:** 2026-09-30

**Scope:** SPEC-03 P0 — nightly Postgres backups, the freshness sentinel, and the
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
an explicit non-goal (SPEC-03 §3). If media durability ever needs more, it is a
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
2. Runs `pg_dump -Fc` — connecting **directly to Postgres, not PgBouncer** —
   spooling it to a worker temp file with sha256 computed in the same pass, then
   uploads the file to object storage at `backups/pg/<yyyy-mm-dd>.dump` (the
   storage client needs a seekable body, so a straight pipe cannot work). The
   worker's scratch disk must hold one dump.
3. Closes the row (`ok` + size + key, or `failed` + reason).
4. On success, overwrites `backups/pg/LATEST.json`
   `{storage_key, sha256, size_bytes, finished_at}`.
5. Prunes old dumps, working only from the `ok` rows in `ops_backup_runs` (never
   a bucket listing; all dates UTC): keeps the **7 most recent** dump dates plus,
   for each of the **4 most recent ISO weeks** with an ok dump, that week's
   latest dump; never deletes the `LATEST.json` target or `LATEST.json` itself.
   Objects with no ok row are left alone (after a disaster restore the ledger is
   older than the bucket). See SPEC-03 P0.2 step 5. *(The shipped selector
   still keeps the 4 most recent Sunday dumps as the weekly set, not the latest
   dump of each ISO week — code follow-up.)*
6. Emits `ops:backup_completed` / `ops:backup_failed` and writes an audit record
   (`ops.backup.completed` / `ops.backup.failed`).

A failure is terminal for that run (never a wedged `running`); the next night runs
normally.

### Configuration

`BACKUP_DATABASE_URL` (in `.env`) is the DSN pg_dump uses. It **MUST connect to
Postgres directly** (port 5432), never through PgBouncer or any other pooler — a
transaction pooler breaks the session semantics pg_dump relies on. It connects as
the schema-owner role `portal`, which bypasses RLS so every tenant's rows are
dumped. Never reuse `DATABASE_URL` (`portal_app`, NOBYPASSRLS): under RLS pg_dump
either errors or silently omits tenants. Under the host-Postgres topology the
host is `host.docker.internal`, matching `.env.example`:

```
BACKUP_DATABASE_URL=postgres://portal:change-me@host.docker.internal:5432/portal?sslmode=disable
```

If it is empty, the task self-reports a `failed` run
(`BACKUP_DATABASE_URL is not configured`, visible on `/ops/status`) rather than
crashing the worker.

The worker image ships `postgresql18-client` (pg_dump + pg_restore), whose major
version equals the Postgres 18 server's. An older pg_dump refuses to dump a newer
server, so bump the client in the same change that moves the server major.

---

## 3. The freshness sentinel

**First, verify the owner account holds the `admin` role** (or superadmin). The
ops surfaces (`/ops/status`, `/admin/queues`) require `admin`; an owner
provisioned as `creator` gets 403 by design (SPEC-03 P0.1).

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
   - `schema_migrations.dirty = false` and `version` ≤ the checked-out repo's
     latest migration number;
   - `count(*) FROM users` ≥ 1, and `SELECT count(*) FROM assets` succeeds;
   - the 7 system roles seeded by `0003_account_rbac` are present.

   Any failed check exits non-zero. SPEC-03 P0.4 step 3 is the contract
   (`RESTORE DRILL FAILED: <check>`); the shipped script does not yet check
   `dirty` or `users ≥ 1`.
5. Drops the scratch DB and prints `RESTORE DRILL PASSED`.

It consults **no** application table for selection — a fresh dev stack has no
`ops_backup_runs` to read; the manifest is the single source of truth.

### Prerequisites

The drill runs docker-first (`backend/scripts/restore-drill.sh`): object storage
is read with a one-off `minio/mc` container on the compose network, and
`pg_restore`/`psql` run inside the running Postgres container. The host needs
`docker`, `python3` (to parse the manifest) and `sha256sum` or `shasum`; no
host-installed Postgres client is required. When you restore with a host client
instead, it must be `postgresql18-client` (the server's major version). The
scratch-DB connection must reach Postgres **directly** (not PgBouncer).

### Connection knobs

The script auto-reads `S3_BUCKET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY` and
`POSTGRES_USER` from `.env`, and takes `PG_CONTAINER` (default
`portal-postgres-1`), `RESTORE_NET` (default `portal_internal`) and `SCRATCH_DB`
(default `portal_restore_check`) from the environment.

**Backup source.** The stack's single `S3_*` set points at the disposable dev
MinIO, so it cannot reach last night's prod dump. SPEC-03 P0.4 step 0 defines an
explicit source: `RESTORE_S3_ENDPOINT`, `RESTORE_S3_BUCKET`,
`RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY`, each falling back to `S3_*`
only when unset. The prod drill uses a read-only R2 token scoped to
`backups/pg/*`. The shipped script does not read `RESTORE_S3_*` yet (it always
talks to `http://minio:9000`); until it does, a prod drill means copying
`LATEST.json` and its dump from R2 into the dev MinIO bucket first.

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
