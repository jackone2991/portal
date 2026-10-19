# Backup & Restore Runbook

**Status:** current · **Last verified:** 2026-09-30

**Scope:** SPEC-03 P0 — nightly backups of the Postgres cluster and of the
deployment's `.env`, the freshness sentinel, and the quarterly restore drill.
Followable start-to-finish by someone who did not write the code.

> **Target, not yet `HEAD`.** Since 2026-10-19 this runbook describes the
> backups [ADR-18](../product/specs/SPEC-03-platform-ops.md#adr-18) decides
> (Decisions 2026-10-19 (C2), (C5), (C6)): a separate locked bucket, age
> encryption, cluster globals, a drill in a throwaway container. The code does
> not do that yet ([SPEC-03](../product/specs/SPEC-03-platform-ops.md) §11
> rows 2–3, 12–16). On `HEAD` the worker writes one **plaintext**
> `pg_dump -Fc --no-owner --no-privileges` to `backups/pg/<yyyy-mm-dd>.dump`
> in the **media** bucket with the media key, its manifest at
> `backups/pg/LATEST.json`; every container holds `BACKUP_DATABASE_URL`; and
> `make restore-drill` cannot run, because it needs the compose Postgres
> container that was removed on 2026-08-21 (§ 4 "Today on `HEAD`").

---

## 1. What is (and isn't) backed up

**Backed up, nightly:**

- **The Postgres cluster's globals** — every role with its attributes and
  password hash, and role memberships (`pg_dumpall --globals-only`). Without
  them a restored database has no `portal_app` to serve the API.
- **The portal database, with owners and grants** (`pg_dump -Fc`). Postgres is
  the fragile asset: it holds every pointer into object storage, so losing it
  orphans the media even though the bytes survive.
- **The deployment's `.env`** — a restored database is useless without the JWT
  signing keys, the S3 keys and the DSN passwords that match it.

Each is encrypted before it leaves the worker (§ 2 "Encryption and key
custody"). Traefik and compose config are not copied: they live in git.

**Media is not backed up at all — by decision** (Decision 2026-10-19 (C3);
SPEC-03 P0.3; [ADR-04](../product/specs/SPEC-04-media-image-pipeline.md#adr-04),
amended the same day):

- **Prod media lives on Cloudflare R2** (11-nines-class durability). Its
  durability is R2's and that is the whole story: there is no cross-bucket
  copy. **Do not turn on bucket versioning or soft-delete** on the media bucket
  — a deleted User's originals would then survive the purge that deleting a
  User must perform ([ADR-13](../product/specs/SPEC-01-account-identity-admin.md#adr-13)).
  The accepted price: an operator's mistaken delete on the media bucket cannot
  be undone.
- **Dev media** (the MinIO bind mount, `./data/minio` or `MINIO_DATA_DIR`) has
  no copy either.

Never "fix" backup coverage by streaming buckets through the backup job — that
is an explicit non-goal (SPEC-03 §3).

**Where this runs.** Production is a rented VPS with R2
([ADR-03](../product/specs/SPEC-03-platform-ops.md#adr-03), re-affirmed
2026-10-19 (C1)); its compose overlay and deployment guide are not written yet
([backlog.md](../product/backlog.md) #30). Today the stack runs on the
development machine — Postgres 18 on the host at `G:\data\postgres\18`, MinIO
on a bind mount, Docker Desktop — and its backup bucket is a MinIO bucket on
the **same disk**: it protects against a bad migration or a mistaken delete,
not against losing the machine. Do not let irreplaceable data accrue there
before production exists.

---

## 2. The nightly backup (automatic)

A periodic Asynq task `ops:backup_database` runs **nightly at 03:00 UTC**
(10:00 in `Asia/Ho_Chi_Minh` — the schedule and retention are open, SPEC-03 §10
Q-C4), scheduled on the worker's shared scheduler (there is no OS cron). Each
run writes one **set**:

1. Opens an `ops_backup_runs` row (`running`).
2. Runs `pg_dumpall --globals-only` and `pg_dump -Fc` — both connecting
   **directly to Postgres, not PgBouncer**, the dump keeping owners and
   privileges — and encrypts each output with age as it streams, spooling the
   ciphertext to a worker temp file with sha256 computed in the same pass;
   then encrypts the `.env` likewise. It uploads the three files to the backup
   bucket under keys unique to the run (the storage client needs a seekable
   body, so a straight pipe cannot work). The worker's scratch disk must hold
   one encrypted dump. A failed upload deletes nothing.
3. Closes the row (`ok` + size + the dump's key, or `failed` + reason).
4. On success, overwrites the manifest `backups/LATEST.json`
   `{storage_key, sha256, size_bytes, globals_key, globals_sha256, config_key,
   config_sha256, encryption, recipient, finished_at}` — every sha256 over the
   ciphertext.
5. Prunes old sets, working only from the `ok` rows in `ops_backup_runs` (never
   a bucket listing; all dates UTC): keeps, for each of the **7 most recent**
   dump dates, that date's latest set, plus, for each of the **4 most recent ISO
   weeks** with an ok dump, that week's latest set; never deletes the manifest
   or its target; and deletes nothing **younger than the lock**
   (`BACKUP_LOCK_DAYS`) — a candidate waits for the first run after its lock
   expires, so every set of the last `BACKUP_LOCK_DAYS` days is kept. Objects
   with no ok row are left alone (after a disaster restore the ledger is older
   than the bucket). See SPEC-03 P0.2 step 5. *(`HEAD` keeps the 4 most recent
   Sunday dumps as the weekly set, prunes at once, and knows no lock — §11
   rows 6 and 12.)*
6. Emits `ops:backup_completed` / `ops:backup_failed` and writes an audit record
   (`ops.backup.completed` / `ops.backup.failed`).

A failure is terminal for that run (never a wedged `running`); the next night runs
normally. If the database itself is down when the task starts, no row can be
opened: the task fails once, is archived, and `/ops/status` reads `stale` once
26 h pass without a success.

### Where it goes: the backup bucket

Backups go to their own bucket, **`portal-backups`** — never the media bucket —
under a key that only the worker holds (Decision 2026-10-19 (C2)):

| Object | Key |
|---|---|
| globals | `backups/pg/<yyyy-mm-dd>/<run_id>.globals.sql.age` |
| dump | `backups/pg/<yyyy-mm-dd>/<run_id>.dump.age` |
| `.env` copy | `backups/config/<yyyy-mm-dd>/<run_id>.env.age` |
| manifest | `backups/LATEST.json` (unlocked; overwritten nightly) |

The bucket is **locked**: every object under `backups/pg/` and
`backups/config/` is retained for `BACKUP_LOCK_DAYS` (30 days proposed; settled
with SPEC-03 §10 Q-C4) and cannot be deleted or overwritten before then — not
by the worker's key, not by you. That is the point: a compromised server cannot
destroy the backups it wrote.

**Setting it up** (once per environment):

- **Development (MinIO).** `minio-setup` will create the bucket locked
  (SPEC-03 §11 row 12); until it does, create it by hand with `mc` against the
  dev MinIO: `mc mb --with-lock local/portal-backups`, then
  `mc retention set --default compliance 30d local/portal-backups` (compliance
  mode: even the MinIO root user cannot shorten it). Add two MinIO users with
  bucket policies limited to `portal-backups`: one for the worker (put, get,
  list, delete) and one read-only for the drill (get, list).
- **Production (R2).** Create the bucket `portal-backups`; under its settings add
  two bucket lock rules, on the prefixes `backups/pg/` and `backups/config/`,
  each retaining objects for `BACKUP_LOCK_DAYS` days (the manifest's
  `backups/LATEST.json` stays outside both). Create two R2 API tokens scoped to
  this bucket only: *Object Read & Write* for the worker, *Object Read only*
  for the drill. Record them where the deployment guide (backlog #30) says.

### Configuration

The worker reads these, and **only the worker container receives them**:
compose passes each service the variables its binary reads, not the whole
`.env` (SPEC-03 §11 row 13 — on `HEAD` every container gets all of `.env`).
Apart from `BACKUP_DATABASE_URL`, they are not in `.env.example` yet; they land
there with the code.

| Variable | What it is |
|---|---|
| `BACKUP_DATABASE_URL` | the DSN `pg_dumpall` and `pg_dump` use (below) |
| `BACKUP_S3_ENDPOINT`, `BACKUP_S3_REGION`, `BACKUP_S3_BUCKET`, `BACKUP_S3_USE_PATH_STYLE` | where the backup bucket is — same shape as `S3_*`; dev: `http://minio:9000`, `us-east-1`, `portal-backups`, `true`; R2: `https://<account>.r2.cloudflarestorage.com`, `auto`, `portal-backups`, `false` |
| `BACKUP_S3_ACCESS_KEY`, `BACKUP_S3_SECRET_KEY` | the worker's key to that bucket only — never the media key |
| `BACKUP_LOCK_DAYS` | must equal the bucket's lock retention (30 proposed) |
| `BACKUP_AGE_RECIPIENT` | the age **public** key (`age1…`) every object is encrypted to |
| `BACKUP_CONFIG_PATH` | where the worker sees the read-only mount of `.env` |

An unset `BACKUP_S3_BUCKET`, `BACKUP_AGE_RECIPIENT` or `BACKUP_CONFIG_PATH`
fails the run with that name in `error` — there is no fallback to the media
bucket and no plaintext mode.

`BACKUP_DATABASE_URL` **MUST connect to Postgres directly** (port 5432), never
through PgBouncer or any other pooler — a transaction pooler breaks the session
semantics pg_dump relies on. It connects as the superuser and schema owner
`portal`, which bypasses RLS so every tenant's rows are dumped, and which can
read the role password hashes `pg_dumpall` copies. Never reuse `DATABASE_URL`
(`portal_app`, NOBYPASSRLS): under RLS pg_dump either errors or silently omits
tenants. Under the host-Postgres topology the host is `host.docker.internal`,
matching `.env.example`:

```
BACKUP_DATABASE_URL=postgres://portal:change-me@host.docker.internal:5432/portal?sslmode=disable
```

If it is empty, the task self-reports a `failed` run
(`BACKUP_DATABASE_URL is not configured`, visible on `/ops/status`) rather than
crashing the worker.

The worker image ships `postgresql18-client` (`pg_dump`, `pg_dumpall`,
`pg_restore`), whose major version equals the Postgres 18 server's. An older
pg_dump refuses to dump a newer server, so bump the client in the same change
that moves the server major.

### Encryption and key custody

Every backup object is encrypted with [age](https://age-encryption.org) to a
public key; the server holds only that public key, so it can write backups it
cannot read (Decision 2026-10-19 (C5); SPEC-03 P0.7).

1. **Generate a pair** on a machine you trust: `age-keygen -o portal-backup.key`.
   It prints the public key (`age1…`); the file holds the private identity
   (`AGE-SECRET-KEY-1…`).
2. **Give the worker the public key**: `BACKUP_AGE_RECIPIENT=age1…`.
3. **Keep the identity offline, twice**: paste the identity into your password
   manager, and print it on paper kept somewhere safe. Then delete
   `portal-backup.key` from the disk. It must never sit on a server or in
   `.env`.
4. **Prove it**: the drill (§ 4) decrypts with the identity each time it runs —
   write it to a temporary file, point `AGE_IDENTITY_FILE` at it, delete the
   file afterwards.
5. **Rotate** by generating a new pair and changing `BACKUP_AGE_RECIPIENT`. Keep
   every old identity for as long as any set encrypted to it is retained (at
   least `BACKUP_LOCK_DAYS` and the longest retention tier); each manifest
   names the `recipient` its set used.
6. **Keep old `AUDIT_PII_KEY`s.** Once that key exists (SPEC-01 P0.16), a dump's
   `pii` columns open only with the key that sealed them: never discard an old
   one while a set that may hold rows sealed with it is retained — keep them in
   the password manager beside the identities. JWT signing keys can change
   freely (a restore under new keys only signs everyone out).

> **Accepted risk:** lose every copy of the identity and you lose every
> backup. There is no escrow and no recovery path.

---

## 3. The freshness sentinel

**First, verify you are signed in as the operator**: the instance's Superadmin
(the account `BOOTSTRAP_SUPERADMIN_EMAIL` names — SPEC-01 P0.12) holds `*`;
any other account needs the `admin` role. The ops surfaces (`/ops/status`,
`/admin/queues`) answer 403 to anyone else (SPEC-03 P0.1).

`GET /api/v1/ops/status` (permission `ops:read`, admin-tier) returns:

```json
{
  "last_success_at": "2026-07-12T03:00:11Z",
  "hours_since_success": 5.4,
  "state": "ok",
  "last_run": { "id": "…", "status": "ok", "started_at": "…", "finished_at": "…",
                "size_bytes": 1048576, "storage_key": "backups/pg/2026-07-12/<run_id>.dump.age",
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

A backup that has never been restored is a hope, not a property. Run the drill
once per quarter (a calendar practice, not automation — cadence is SPEC-03 §10
Q-C8) and after any change to the backup path. It needs no Postgres of the
stack's and can run on any machine with Docker.

```bash
make restore-drill
```

### What it does

1. Reads `backups/LATEST.json` from the backup bucket (**the manifest — never
   latest-by-key-listing**, which a partial upload could poison).
2. Downloads the set's three objects and **verifies each sha256** against the
   manifest, then decrypts the globals and the dump with your identity (and
   the `.env` copy to nowhere, proving it opens).
3. Starts a **throwaway `postgres:18` container** — the host cluster's major,
   no volume, removed on exit, its superuser named `drill_admin` so nothing in
   the globals collides with it — applies the **globals first**, then
   `pg_restore --create` with owners and grants. Both must exit 0. It never
   restores into the host cluster: there every role already exists.
4. Sanity-checks the restored database (each meaningful without knowing the
   dump's history):
   - `schema_migrations.dirty = false` and `version` ≤ the checked-out repo's
     latest migration number;
   - `count(*) FROM users` ≥ 1, and `SELECT count(*) FROM assets` succeeds;
   - the 7 system roles seeded by `0003_account_rbac` are present;
   - **as `portal_app`** (`SET ROLE portal_app`): an unscoped read of a
     row-holding table under `FORCE ROW LEVEL SECURITY` is refused, and inside
     the oldest organisation's tenant scope every table
     with `FORCE ROW LEVEL SECURITY` answers `SELECT count(*)` — the roles,
     their grants and the policies all came back.

   Any failed check exits non-zero with `RESTORE DRILL FAILED: <check>`
   (SPEC-03 P0.4 step 3).
5. Removes the container and prints `RESTORE DRILL PASSED`.

It consults **no** application table for selection — a fresh machine has no
`ops_backup_runs` to read; the manifest is the single source of truth.

### Prerequisites and knobs

The host needs `docker`, `python3` (to parse the manifest), `sha256sum` or
`shasum`, and `age`; no host-installed Postgres client. Object storage is read
with a one-off `minio/mc` container. The drill reads, from your shell or the env
file the script loads:

| Variable | What it is |
|---|---|
| `RESTORE_S3_ENDPOINT`, `RESTORE_S3_REGION`, `RESTORE_S3_BUCKET`, `RESTORE_S3_USE_PATH_STYLE` | where the backup bucket is (production: R2's endpoint; dev: `http://minio:9000`) |
| `RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY` | the **read-only** token on that bucket (§ 2) |
| `AGE_IDENTITY_FILE` | a temporary file holding your age identity |
| `RESTORE_NET` | the docker network the `mc` container joins (default `portal_internal`, so a dev `http://minio:9000` resolves) |
| `RESTORE_PG_IMAGE` | the throwaway image (default `postgres:18`) |

There is **no fallback**: not to the stack's `S3_*` (the media bucket) and not
to `BACKUP_S3_*` (the worker's write key). An unset value fails the drill with
`RESTORE DRILL FAILED: backup source not configured`. *(This replaces the
2026-09-30 design, whose `RESTORE_S3_*` fell back to `S3_*`.)*

### Today on `HEAD`

`backend/scripts/restore-drill.sh` predates all of the above (SPEC-03 §11
rows 2–3): it reads the **media** bucket at a hard-coded `http://minio:9000`
with `S3_*` from `.env`, and runs `pg_restore --no-owner --no-privileges` and
`psql` inside a Postgres container named by `PG_CONTAINER` (default
`portal-postgres-1`) — the compose service commented out on 2026-08-21, so the
script stops at its precondition. Until the rework lands, a stand-in container
should let it run against `HEAD`'s dumps (untested; no run is recorded):

```bash
docker run -d --rm --name portal-postgres-1 \
  -e POSTGRES_USER=portal -e POSTGRES_PASSWORD=drill postgres:18
until docker exec portal-postgres-1 pg_isready -U portal; do sleep 1; done
make restore-drill
docker stop portal-postgres-1
```

That proves only what a `HEAD` dump holds: the data restores, as the
superuser. It cannot prove the roles or the grants, because `HEAD`'s dumps
carry neither (SPEC-03 §11 row 15).

### Manual restore into production (real disaster)

The drill restores into a throwaway container. To restore **for real**:

1. Stop the API + worker (`make down` or scale to 0) so nothing writes.
2. Download the set (from the manifest, or the date you want), verify each
   sha256 as the drill does, and decrypt the globals, the dump and the `.env`
   copy with your identity.
3. **Globals — only into a new cluster.** If the old cluster is gone, apply the
   globals first, as a superuser: `psql -f globals.sql`. That recreates
   `portal`, `portal_app` and `portal_sys` with their passwords as of that
   night. If the cluster survived, **skip them**: the roles exist (the
   statements would collide), and applying them would revert any password
   rotated since the dump; check `\du` lists `portal_app` and `portal_sys`.
4. Restore the dump with owners and grants — never `--no-owner` or
   `--no-privileges`: into a new cluster
   `pg_restore --create -d postgres <dump>`; over an existing database
   `pg_restore --clean --if-exists -d <portal-db-dsn> <dump>`, as a superuser.
   Take a fresh dump first if the database is reachable.
5. Confirm `schema_migrations.version` matches the running code's expected head;
   run `make migrate` if the code is ahead of the dump.
6. Restore `.env` from the set's decrypted copy — its DSN passwords match the
   globals of that night — and compare it with your password manager for any
   key rotated since. Bring the stack up and hit `/ops/status` + `/healthz`.

> **Warning — a restore brings deleted Users back.** Deleting a User is meant
> to be irreversible ([SPEC-01](../product/specs/SPEC-01-account-identity-admin.md)
> P0.10, owner decision 2026-10-02 A6): their rows, files and cache keys are
> purged and only a 90-day identity snapshot (`deleted_users`) survives. A dump
> taken **before** the delete still holds the User and every row they owned, so
> restoring it resurrects them — able to sign in again, with their data — while
> the media objects that the delete removed from storage stay gone, leaving
> their `assets` rows pointing at nothing. This is an operations exception,
> not a recovery feature. The restore also replaces `deleted_users` itself, so
> the record of later deletes must be saved **before** step 4: if the live
> database is reachable, copy out the `deleted_users` rows with `deleted_at`
> after the dump's timestamp (the fresh dump of step 4 holds them too). After
> the restore, delete each of those Users again — through the admin console,
> so every module purges. The same check applies
> to identity data the 90-day audit sweep (SPEC-01 P0.16) had already
> anonymised: the next sweep run anonymises it again. What else a restore
> rolls back — sessions revoked, accounts disabled, passwords changed since the
> dump — and whether a post-restore reset becomes mandatory is open (SPEC-03
> §10 Q-C7).

---

## 5. Success metrics (n=1 honest)

- 7/7 mornings in the first week show a fresh dated set in the backup bucket
  (off-VPS in production); `hours_since_success` never exceeds 26 outside
  induced failures.
- One restore drill executed and documented before this spec closes; quarterly
  thereafter, calendar-verified.
