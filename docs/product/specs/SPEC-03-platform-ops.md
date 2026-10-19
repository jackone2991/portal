# SPEC-03 — Platform Ops: Backup/Restore, Queue Console, Takeout

**Status:** current, rev 3 (Decisions 2026-10-19 (C1), (C2), (C3), (C5), (C6): [ADR-18](#adr-18) backups, ADR-03 re-affirmed, no media backup, P0.7 encryption; nine owner questions open in §10); rev 2 folded ADR-03 and ADR-05 in and moved ADR-01, ADR-10 and ADR-16 in from the specs README (2026-10-02) · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `ops` (owns the `ops:*` prefix) · **Depends on:** nothing hard on data — rides (or, if it lands first, introduces) SPEC-04 P0.3's shared periodic-scheduler convention; failure alerts: notify consumer `notify:on_backup_failed` (owned here, P0.6) now that SPEC-05 is live; staleness remains pull-only via P0.5 until a freshness-check task is added (P2)
**Upstream:** brief 09 (folded into this spec, then deleted — `git show ea100d8:docs/product/briefs/09-platform-ops.md`) · **Refs:** [ADR-01](#adr-01) (observability stays deferred), [ADR-03](#adr-03) and [ADR-05](#adr-05) (folded in below, § Decision records, with [ADR-01](#adr-01), [ADR-10](#adr-10), [ADR-16](#adr-16) and, since 2026-10-19, [ADR-18](#adr-18)), [ADR-04](SPEC-04-media-image-pipeline.md#adr-04), 2026-07 backlog §7 (archived; `git show 8d382d2^:docs/product/backlog.md`), feature-inventory `D-25` (audit types)
**Downstream consumers:** every module holding irreplaceable data (bank, journal, media, people); notify consumes `ops:backup_failed` through `notify:on_backup_failed` (P0.6, unbuilt); `ops:export_ready` is a *future* notify consumer (needs its own `notify:on_*` task + type row first)

---

## 1. Problem statement

A self-hosted life OS asks the owner to move financial history and memories onto
hardware they operate. That deal is only honest with three properties the stack
currently lacks: **backups that run themselves**, **a restore path that has
actually been exercised**, and **visibility into the async machinery** (today a
failed transcode is discoverable only by tailing worker logs). A 2026-07 docs
audit found **no backup/DR doc exists anywhere**, and the 2026-07 backlog §7
named the pieces without anything owning them — today a single disk failure ends
the life-OS thesis. The vision's credo — owned end
to end — also implies data can *leave*: ownership without an export path is a
promise, not a property.

Sequencing pressure: SPEC-12 starts accruing months of hand-entered ledger data
the day it lands. This spec's P0 should land **before** that data exists, which
is why the build order places SPEC-03 P0 as a hard gate immediately before SPEC-12
(SPEC-10 is the burst-filler). The gate was met in order (`0012` preceded
`0014`) but not in substance: the P0.4 drill has never passed, and the owner's
review of 2026-10-19 found the backups themselves unfit — in the media bucket
under the media key, unencrypted, without the roles a restore needs
([ADR-18](#adr-18)).

## 2. Goals

1. Nightly automated backups of the Postgres cluster — its globals and the
   portal database — encrypted, into a separate locked bucket off the VPS (R2
   `portal-backups` in production; a MinIO bucket of the same name in dev,
   which shares the dev machine's disk), with retention, plus an encrypted copy
   of the deployment's `.env` ([ADR-18](#adr-18); P0.2, P0.7). "Off-VPS" is a
   production property: production is the rented VPS with R2 ([ADR-03](#adr-03),
   re-affirmed 2026-10-19 (C1)).
2. A **tested** restore procedure — a backup that has never been restored is a
   hope, not a property.
3. Backup failure or staleness is impossible to miss (event + queryable status).
4. The solo operator inspects/retries/purges Asynq jobs from the browser, no SSH.
   *(P1)*
5. The owner exports their data per module into one archive in open formats. *(P1)*

## 3. Non-goals

- **The 5-service observability stack** (Loki/Prometheus/Tempo/Grafana) — stays
  deferred per ADR-01; the queue console is in-process, $0.
- **Any media backup** *(Decision 2026-10-19 (C3))* — originals and variants
  rely on the object store's durability alone: no bucket streamed through the
  database job, no cross-bucket copy ([ADR-04](SPEC-04-media-image-pipeline.md#adr-04)
  decision item 5, dropped by its 2026-10-19 amendment), no versioning or
  soft-delete. P0.3 says why.
- **Cross-region DR, standby replicas** — single-VPS envelope (ADR-03); off-site
  *copies*, not HA.
- **GDPR-grade takeout formats** — this is the owner's own data leaving politely.
- Backing up Traefik and compose config — both live in git; only `.env`, which
  does not, is copied (P0.7).

## 4. User stories

- As the owner, I run `make restore-drill` with a read-only token for the
  backup bucket and my offline age identity (P0.4 step 0), and last night's
  ledger is queryable as `portal_app` in a throwaway Postgres 18 container —
  before I ever need it in anger. *(primary)*
- As the owner, a transcode dies permanently and I see it in
  `https://api.<domain>/admin/queues` and retry it from the browser. *(P1)*
- As the owner, I download `portal-export-2026-07.tar.gz` and my journal, ledger
  CSVs, and original photos are inside, in open formats. *(P1)*
- Edge: as the owner, the scheduler silently stopped a week ago (no runs at
  all) — the status endpoint has read `stale` since hour 26.
- Edge: as the owner, last night's run failed — the status endpoint reads
  `failed` (P0.5 precedence), and `ops.backup.failed` is in the audit log.

## 5. Requirements

### P0.1 — Module scaffold

`internal/modules/ops/` per MODULES.md §8 (module.go, `api/`, `service/`,
`query/`, `repository/`, own `sqlc.yaml` block), migration `0012_ops_backup_runs`,
wired into `cmd/api` (status endpoint) and `cmd/worker` (backup task), depguard
block added. `ops` tables are **system-scoped** (no `user_id`) except P1.7's
exports.

Migration `0012_ops_backup_runs` seeded all four permission rows up front —
`ops:read`, `queues:read` → `admin`; `takeout:read:own`, `takeout:write:own` →
`user` (unseeded codes 403; seeding early is harmless). The `ops_exports` table
ships in a later P1 migration. `queues:write` is added by P1.6's own migration
(0012 is applied and stays at four codes).

The ops surfaces require `admin` (or superadmin via `*`). The operator is the
instance's Superadmin — the account `BOOTSTRAP_SUPERADMIN_EMAIL` names, which
[SPEC-01](SPEC-01-account-identity-admin.md) P0.12 pre-creates (Decision
2026-10-02 (A2); until that ships, the first registrant on an empty database
or the same variable) — and holds `*`; any other account needs `admin`. The
runbook's first step verifies it.

### P0.2 — Nightly `ops:backup_database`

On the shared periodic runner (SPEC-04 P0.3's convention — no OS cron),
nightly at 03:00 UTC (`0 3 * * *`, a cron spec, not `@every`), enqueued on the
`default` queue served by the worker's light server (matches
[events.md](../../reference/events.md)). If dump duration makes that queue's
weight-1 slot contended, move the task to a dedicated `ops` queue in the same PR
that registers it. *(The schedule — 03:00 UTC is 10:00 in `Asia/Ho_Chi_Minh` —
the RPO and the retention below are open: §10 Q-C4.)*

If this lands before SPEC-04 P0.3, ops introduces the shared periodic
scheduler itself — what is borrowed is the convention (Asynq periodic, never
OS cron), not code.

**Task options:** `asynq.MaxRetry(0)` (a failed night is recorded once and
retried by the next night), `asynq.Timeout(2h)`, `asynq.Unique(23h)`. At start,
the task marks any `running` row older than the timeout as `failed`
(`error='abandoned'`, `finished_at=now()`), so a worker crash never leaves a row
in `running` forever.

**What a backup set is, and where it goes** *(Decisions 2026-10-19 (C2), (C5),
(C6); [ADR-18](#adr-18))*: one run writes one **set** of three objects to the
**backup bucket** `portal-backups` — never the media bucket — each written
once under a key unique to the run, each age-encrypted (P0.7):

| Object | Key | Content |
|---|---|---|
| globals | `backups/pg/<yyyy-mm-dd>/<run_id>.globals.sql.age` | `pg_dumpall --globals-only`: every role of the cluster with its attributes and password hash, role memberships |
| dump | `backups/pg/<yyyy-mm-dd>/<run_id>.dump.age` | `pg_dump -Fc` of the portal database, **with** owners and privileges |
| config | `backups/config/<yyyy-mm-dd>/<run_id>.env.age` | the deployment's `.env` (P0.7) |

`<yyyy-mm-dd>` is the run's `started_at` in UTC. The manifest
`backups/LATEST.json` is the one object outside the locked prefixes (it is
overwritten nightly and holds no data). The bucket is reached through its own
`platform/storage` instance built from `BACKUP_S3_ENDPOINT`, `BACKUP_S3_REGION`,
`BACKUP_S3_BUCKET`, `BACKUP_S3_ACCESS_KEY`, `BACKUP_S3_SECRET_KEY` and
`BACKUP_S3_USE_PATH_STYLE` — a key scoped to that bucket, held by the `worker`
container only (compose gives each service the variables its binary reads, not
the whole `.env`) — R2 in production, a MinIO bucket of the same name in dev,
**same code path both** (locked from the brief: dev exercising the machinery is
the point of the drill). An unset `BACKUP_S3_BUCKET` fails the run
(`BACKUP_S3_BUCKET is not configured`); there is no fallback to `S3_*`. The
bucket is **locked**: S3 Object Lock on MinIO, R2 bucket lock rules on
`backups/pg/` and `backups/config/` in production, retaining every object for
`BACKUP_LOCK_DAYS` (30 days proposed; settled with retention in §10 Q-C4), so
not even the worker's key can delete or overwrite a young backup. In dev the
bucket shares the machine's disk with the database: it guards against a bad
migration or a mistaken delete, not against losing the machine.

1. Insert an `ops_backup_runs` row (`running`).
2. Produce and upload the set. `pg_dumpall --globals-only` and then
   `pg_dump -Fc` — no `--no-owner`, no `--no-privileges`, so ownership and every
   `GRANT` (`0019`'s grants and default privileges to `portal_app` and
   `portal_sys`; a future `portal_acl`-owned `SECURITY DEFINER` function) come
   back with a restore (C6) — each piped through age encryption into a worker
   temp file, with sha256 teed over the **ciphertext** in the same pass, then
   uploaded via `platform/storage.Storage.Put` under the set's key; the config
   copy likewise (P0.7). Nothing reaches the bucket unencrypted, and no
   plaintext dump touches the worker's disk. `Put` needs a seekable body so the
   SDK can sign the payload; a straight stdout → `Put` pipe fails (§10). A failed `Put` deletes nothing: S3
   `PutObject` is atomic, so there is never a partial object to remove, and a
   later run's failure never touches an earlier run's objects. A run that fails
   part-way leaves the objects it already uploaded unreferenced (no ok row, so
   retention never sees them; the lock ages them out of protection, and they
   stay until the operator removes them — at most one set per failed night).
   **Both dumps connect directly to Postgres, not through PgBouncer** — pg_dump
   needs session semantics a transaction pooler breaks. The DSN is
   `BACKUP_DATABASE_URL` (`platform/config` → `cfg.BackupDatabaseURL`;
   documented in `.env.example`). It connects directly to Postgres, never
   through a pooler, as the superuser and schema owner `portal`, which bypasses
   RLS so every tenant's rows are dumped and can read `pg_authid` so the role
   password hashes are in the globals. `DATABASE_URL` (`portal_app`,
   NOBYPASSRLS) must never be reused: pg_dump under RLS either errors or
   silently omits tenants. If the setting is unset, the run fails with
   `error="BACKUP_DATABASE_URL is not configured"`, so P0.5 reads `failed`.
3. Finish the row (`ok` + `size_bytes` + `storage_key`, or `failed` + `error`).
   `size_bytes` is the stored (encrypted) dump object's; `storage_key` is the
   dump's key, and the set's other two keys derive from its
   `<yyyy-mm-dd>/<run_id>` stem, so the row keeps one key. `error` is a single
   line truncated to 500 chars; the DSN and the command line are never
   interpolated into it (it is returned by P0.5 and published in
   `ops:backup_failed`).
4. On success, overwrite the manifest `backups/LATEST.json` `{storage_key,
   sha256, size_bytes, globals_key, globals_sha256, config_key, config_sha256,
   encryption: "age", recipient, finished_at}` — every sha256 computed while
   spooling, over the ciphertext as stored; `recipient` is the public key the set
   was encrypted to (P0.7). Retention (step 5) must never delete it or its
   target.
5. **Retention** after a successful run. Input = `ops_backup_runs` rows with
   `status='ok'` and non-null `storage_key`, de-duplicated by key (never a
   storage listing). All dates are UTC (operator-facing retention dates, not a
   user-facing day boundary, so the specs README Timezone rule does not apply):
   a set's date is the `<yyyy-mm-dd>` in its key, which is `started_at` in UTC.
   Keep = for each of the 7 most recent dump dates, that date's latest ok set ∪,
   for each of the 4 most recent ISO weeks (Mon–Sun, UTC) that contain an ok
   dump, the latest ok set in that week ∪ the current `LATEST.json` target.
   Every other set's three objects are deleted from storage **only once its lock
   has expired** — its run's `finished_at` older than `BACKUP_LOCK_DAYS`; a
   younger candidate is skipped and deleted by the first successful run after
   its lock expires, so for the first `BACKUP_LOCK_DAYS` every set is kept
   whatever the tiers say. `Storage.Delete` of a missing key is a no-op, so rows
   are never modified and re-running prune is idempotent. `backups/LATEST.json`
   is never a candidate. Objects with no ok row are left alone: after a
   disaster restore the ledger is older than the bucket, and sweeping would
   delete the newest dumps. *(Whether retention stays app-side or moves to a
   bucket lifecycle rule is part of §10 Q-C4; either way nothing is deleted
   before its lock expires.)*
6. Emit `ops:backup_completed {run_id, size_bytes}` / `ops:backup_failed
   {run_id, error}` (register in [events.md](../../reference/events.md)); write
   audit events `ops.backup.completed` / `ops.backup.failed` per D-25. Audit rows
   use `actor_kind='system'`, `actor_id=NULL`, `target_kind='ops_backup_run'`,
   `target_id=<run id>`. Register both types in the D-25 audit-type catalogue
   (feature-inventory D-25 and the module-map audit-types table).

*(Code follow-up: on HEAD the scheduler registers the task with no
`MaxRetry`/`Timeout`/`Unique` options and there is no abandoned-row sweep; the
failure `error` is truncated to 1,000 chars, not 500; and retention (`keep` in
`ops/retention.go`) keeps the 4 most recent **Sunday** dumps rather than the
latest ok dump of each of the 4 most recent ISO weeks, so a week whose Sunday
run failed keeps no weekly anchor. Everything the 2026-10-19 decisions add is
unbuilt — HEAD writes one plaintext `pg_dump -Fc --no-owner --no-privileges`
to `backups/pg/<yyyy-mm-dd>.dump` in the media bucket with the media key, its
manifest at `backups/pg/LATEST.json`, and deletes that key after a failed
upload: §11 rows 12–16.)*

The worker image carries the Postgres client whose major version equals the
server's (currently `postgresql18-client`, which provides `pg_dump`,
`pg_dumpall` and `pg_restore`; locked from the brief: smallest moving-part
count). An older pg_dump refuses to dump a newer server. Bump the client in the
same change that moves the server major.

**Acceptance criteria.**
- Given a nightly run, then a set — globals, dump and config copy, each
  age-encrypted — exists in the backup bucket (off-VPS in production) and the
  run row records size + duration.
- Given `pg_dump` or `pg_dumpall` fails while the application database is
  reachable (e.g. `BACKUP_DATABASE_URL` names a host or role the server
  refuses), then exactly one `ops:backup_failed` is emitted for that night (no
  Asynq retries), the row says why, and the next night runs normally (no wedged
  state).
- Given the database is unreachable when the task starts, then no run row can
  be opened and nothing is emitted or audited (both need the run's row); the
  task fails once (`MaxRetry(0)`, archived — visible in the queue console), the
  next night runs normally, and P0.5 reads `stale` once 26 h pass without a
  success.
- Given a `running` row older than 2 h, then the next run marks it `failed`
  (`error='abandoned'`).
- Given an ok run earlier the same UTC day and a later run whose upload fails,
  then the earlier set and the manifest's target are untouched.
- Given 12 daily dumps accumulated, then the keep/prune selector returns exactly
  the retention set (property test on the pure function against synthetic date
  lists, covering weeks whose Sunday run failed, inputs spanning a year
  boundary, and two ok sets on one date); and given prune candidates younger and
  older than `BACKUP_LOCK_DAYS`, only the older are deleted.
- Given the worker's backup key, then a delete or overwrite of an object younger
  than the lock is refused by the store (R2) or leaves the locked version in
  place (MinIO).
- Given `BACKUP_S3_BUCKET` unset, then the run fails with that message and
  nothing is written to the media bucket.
- Given a set, then its globals restore `portal_app` and `portal_sys` with their
  attributes, and its dump carries their `GRANT`s (`pg_restore --list` shows the
  ACL entries).
- Given a successful run, then `LATEST.json` points at the new set and each
  sha256 matches the stored object.

### P0.3 — Media durability posture: no media backup (decided here, documented in the runbook)

*(Decision 2026-10-19 (C3); [ADR-04](SPEC-04-media-image-pipeline.md#adr-04)
amended the same day.)* Prod media lives on R2 (11-nines-class durability) —
**the database is the fragile asset**, and it holds every pointer into storage;
it is backed up (P0.2). **Media is not backed up at all**: no nightly bucket
dump (non-goal), no cross-bucket copy (ADR-04's weekly export, never built, is
dropped), and no bucket versioning or soft-delete — a versioned or
soft-deleting bucket would keep a deleted User's originals past
[ADR-13](SPEC-01-account-identity-admin.md#adr-13)'s purge, which is meant to
be irreversible. The owner accepted the cost: an original's loss risk is the
provider's durability, and an operator's mistaken delete on the media bucket
cannot be undone. The dev MinIO bind mount is no different — and dev is not
meant to hold irreplaceable data ([ADR-03](#adr-03), re-affirmed 2026-10-19
(C1)): until the production deployment exists (backlog #30), whatever real
media sits on the dev machine has no copy anywhere. The runbook states all this
so nobody "fixes" backup coverage by dumping buckets nightly or by switching
versioning on.

### P0.4 — Restore drill

`make restore-drill`: runs **on a fresh machine or dev stack, where no
`ops_backup_runs` table exists to consult** — dump selection cannot depend on
the database — and needs no Postgres of the stack's: it restores into a
throwaway container of its own *(Decision 2026-10-19 (C6),
[ADR-18](#adr-18))*. *(2026-07-10 — the original "latest `ok` dump" +
"migrations table matches the dump's expected head" checks were
unimplementable: nothing recorded an expected head, and comparing the restored
migrations table against itself is a tautology.)* Mechanics:

0. **Source** *(Decision 2026-10-19 (C2))*: the drill reads the backup bucket
   with a **read-only token** — `RESTORE_S3_ENDPOINT`, `RESTORE_S3_REGION`,
   `RESTORE_S3_BUCKET`, `RESTORE_S3_USE_PATH_STYLE`, `RESTORE_S3_ACCESS_KEY`,
   `RESTORE_S3_SECRET_KEY`, exported in the operator's shell or in the env file
   the script reads — and the offline age identity at `AGE_IDENTITY_FILE`
   (P0.7). There is **no fallback**: not to the stack's `S3_*` (the media bucket
   and its key) and not to `BACKUP_S3_*` (the worker's write key); an unset value
   fails the drill with `RESTORE DRILL FAILED: backup source not configured`.
   The token may read and list the bucket and nothing else — in production an
   R2 API token scoped read-only to `portal-backups`, in dev a MinIO user with a
   read-only policy on the bucket of the same name. *(This replaces the
   2026-09-30 design, whose `RESTORE_S3_*` each fell back to the stack's
   `S3_*`.)*
1. **Selection via manifest**: after every successful run, P0.2's task
   overwrites the small `backups/LATEST.json` (P0.2 step 4). The drill reads the
   manifest — never latest-by-key-listing, which a failed partial upload could
   poison.
2. **Download, verify, decrypt, restore.** Download the set's three objects and
   verify each `sha256` against the manifest (over the ciphertext); decrypt the
   globals and the dump with `AGE_IDENTITY_FILE`, and the config copy to
   nowhere (proving it is readable — the drill needs nothing from it). Start a
   **throwaway `postgres:18` container** (`RESTORE_PG_IMAGE`; the host
   cluster's major, the pattern CI's `backend` job uses) with no volume, removed
   on exit, its bootstrap superuser named `drill_admin` so no role in the
   globals collides with it. Apply the globals first (`psql -f`), then
   `pg_restore --create --dbname postgres` with owners and privileges; both
   must exit 0 with no ignored error. The drill never restores into the host
   cluster: there every role in the globals already exists, and applying them
   would collide — or, worse, revert role passwords to the night's values.
3. **Sanity checks** (each meaningful without knowing the dump's history):
   (a) `schema_migrations.dirty = false` and `version` ≤ the checked-out repo's
   latest migration; (b) `count(*) FROM users` ≥ 1 and
   `SELECT count(*) FROM assets` succeeds (the media table's real name — not
   `media_assets`); (c) the 7 system roles seeded by `0003` are present;
   (d) **as `portal_app`, not only as the superuser** *(C6)*: after
   `SET ROLE portal_app` (the role came back with the globals), an unscoped
   `SELECT count(*)` of a FORCE-RLS table in which the superuser sees rows is
   refused (a policy is evaluated only against rows, so an empty table proves
   nothing), and with `app.current_tenant` set to the oldest organisation's id
   (`organizations` has no RLS) `SELECT count(*)` succeeds on every table with
   `FORCE ROW LEVEL SECURITY` — proving the roles, their grants and the policies
   all came back. Any failure → non-zero exit with
   `RESTORE DRILL FAILED: <check>`. *(Code follow-up: §11 rows 2–3 — the
   shipped `restore-drill.sh` needs the compose `postgres` container removed on
   2026-08-21, so it cannot run at all; it reads the media `S3_*` set at a
   hard-coded `http://minio:9000`, never decrypts or applies globals, checks
   neither `dirty` nor `users ≥ 1` and never connects as `portal_app`.)*

Documented as a runbook in `docs/operations/backup-restore.md` (this spec's PR
ships the doc). Quarterly execution is a calendar practice, not automation
*(cadence, automation and an on-demand backup are open — §10 Q-C8)*.

**Acceptance criteria.**
- The drill passes against a **real nightly set** before this spec closes —
  landing the code without one exercised restore does not count as done.
- The runbook is followable start-to-finish by someone who didn't write it (the
  honest test of a runbook).
- Given `RESTORE_S3_*` holding a read-only token on the production backup bucket
  and `AGE_IDENTITY_FILE` the offline identity, the drill restores last night's
  set into its throwaway container — no stack Postgres, no `S3_*`, no
  `BACKUP_S3_*` involved.
- Given any `RESTORE_S3_*` value unset, then the drill fails
  `RESTORE DRILL FAILED: backup source not configured` and reads nothing.
- Given a missing or wrong `AGE_IDENTITY_FILE`, then the drill fails
  `RESTORE DRILL FAILED: decrypt` before anything is restored.
- Given a restored set, then `portal_app` reads every FORCE-RLS table inside a
  tenant scope and is refused outside one.

### P0.5 — Freshness sentinel

`GET /api/v1/ops/status` — permission `ops:read` (admin-tier; the ops
migration seeds it and grants to `admin` — the `*` wildcard also covers it).
Returns `{last_success_at: RFC3339|null, hours_since_success: number|null
(fractional hours, unrounded), state: "ok"|"stale"|"failed", last_run: {id,
status, started_at, finished_at|null, size_bytes|null, storage_key|null,
error|null} | null}`. `last_run` is the most recent row by `started_at` (any
status), or null when there are zero runs.

**`state` semantics (2026-07-10 — precedence and the zero-runs case were
undefined):** evaluated in order, first match wins —

1. `failed` — the most recent **completed** run has `status='failed'`
   (something is actively broken; wins even if an older success is < 26 h old).
2. `stale` — no successful run within 26 h, **including never-ran**
   (`last_success_at: null`, `hours_since_success: null`) — a brand-new or
   silently-dead scheduler must read as a failure state, not `ok`.
3. `ok` — otherwise.

A `running` run doesn't change state (it isn't completed yet).

**Resolved from the brief's "or fold into `/healthz`": a separate authenticated
endpoint, `/healthz` untouched.** Two reasons: `/healthz` is unauthenticated
liveness — backup metadata doesn't belong on it; and coupling backup staleness
into liveness invites an orchestrator to restart-loop a healthy API over a
cron-shaped problem.

**Acceptance criteria.**
- Given last success 27 h ago and no completed run since, then `state=stale`;
  a fresh success → `ok`.
- Given a success 2 h ago followed by a failed run, then `state=failed`
  (precedence).
- Given zero runs ever, then `state=stale` with `last_success_at: null`,
  `hours_since_success: null`, `last_run: null`.
- Given a non-admin caller, then 403; unauthenticated, 401.

### P0.6 — Failure alert delivery (`notify:on_backup_failed`)

SPEC-05 is live, so `ops:backup_failed` gets a push path: a notify consumer
task `notify:on_backup_failed` (owned by this spec, implemented in the notify
module's worker handlers, subscribed in `cmd/worker/main.go` — the emitting
binary — per the specs README **Events** convention) delivers a notification to
every `admin` holder. It needs its own notification type row. Staleness from a
dead scheduler emits no event and stays pull-only via P0.5 until a
freshness-check task is added (P2). *(Unbuilt as of 2026-09-30.)*

**Acceptance criteria.**
- Given a failed run, then each admin receives one notification naming the run
  (the sanitised `error`, never a DSN).

### P0.7 — Backup encryption and the config copy

*(Decision 2026-10-19 (C5), [ADR-18](#adr-18); promoted from P2 the same day.
Unbuilt — §11 row 14.)*

- **age, to a public recipient.** Every object of a set — globals, dump and
  config copy — is encrypted with age (X25519) to the public key in
  `BACKUP_AGE_RECIPIENT` (`age1…`) as the worker produces it: the dump's stdout
  → age → temp file, with sha256 teed over the ciphertext (P0.2 step 2).
  Nothing reaches the bucket unencrypted, and no plaintext dump reaches the
  worker's disk (the `.env` it copies is a read-only mount). The worker holds only
  the recipient: it can write backups it cannot read. An unset or unparsable
  recipient fails the run (`BACKUP_AGE_RECIPIENT is not configured`) before
  anything is uploaded — there is no plaintext fallback.
- **The identity lives offline.** The matching private identity
  (`AGE-SECRET-KEY-1…`) is kept in the owner's password manager and on paper,
  never on a server and never in `.env`. The drill reads it from
  `AGE_IDENTITY_FILE` on the operator's machine for the length of a run (P0.4
  step 2); a real restore does the same.
- **The config copy.** Each night the worker also encrypts the deployment's
  `.env` — mounted read-only into the `worker` container only, at
  `BACKUP_CONFIG_PATH` — and uploads it as the set's config object
  (`backups/config/<yyyy-mm-dd>/<run_id>.env.age`), so a restored database has
  the JWT keys, the S3 keys and the DSN passwords that match it. An unreadable
  path fails the run (`BACKUP_CONFIG_PATH is not readable`). Traefik and compose
  config are in git and are not copied.
- **The manifest records it.** `LATEST.json` carries `encryption: "age"` and
  `recipient` (the public key, so a restore knows which identity it needs after
  a rotation) beside the set's keys; every `sha256` covers the ciphertext as
  stored (P0.2 step 4).
- **Key custody** is a runbook section
  ([backup-restore.md](../../operations/backup-restore.md) § 2): generate,
  store, prove, rotate. A rotation is a new pair and a new recipient on the
  worker; the old identity is kept as long as any set encrypted to it is
  retained. **Old `AUDIT_PII_KEY`s are kept the same way** once that key exists
  ([SPEC-01](SPEC-01-account-identity-admin.md) P0.16,
  [ADR-14](SPEC-01-account-identity-admin.md#adr-14)): a restored dump's `pii`
  columns open only with the key that sealed them.
- **Accepted risk:** losing the private identity — every copy — loses every
  backup. There is no escrow and no recovery path; it is the price of a server
  that cannot read its own backups.

**Acceptance criteria.**
- Given a set, then each of its objects begins with the age header, decrypts
  with the identity matching `BACKUP_AGE_RECIPIENT`, and no plaintext dump
  exists on the worker's disk after the run.
- Given `BACKUP_AGE_RECIPIENT` unset or invalid, then the run fails with its
  name and uploads nothing.
- Given a set, then its config object decrypts to the `.env` the worker reads at
  `BACKUP_CONFIG_PATH`; given that path unreadable, then the run fails with its
  name.
- Given a successful run, then `LATEST.json` names `encryption: "age"` and the
  recipient, and each sha256 is the stored ciphertext's.
- The runbook's key-custody section is followable by someone who did not write
  it: they generate a pair, set the recipient, store the identity in both
  places, and decrypt a set with it.

### P1 — nice to have

- **P1.6 Queue console**: mount `hibiken/asynqmon`'s `http.Handler` at
  `/admin/queues` inside `cmd/api` (wired via the documented `Engine()`
  exception). Served at `https://api.<domain>/admin/queues` (the API origin, not
  a Next route — on the app origin `/admin/*` belongs to Next). The Next admin
  nav links to that absolute URL; no Next page is added. Two asynqmon handlers
  are built: `asynqmon.New(asynqmon.Options{RootPath: "/admin/queues",
  RedisConnOpt: ..., ReadOnly: true})` for callers holding only `queues:read`,
  and one with `ReadOnly: false` for `queues:write` holders. The mount picks the
  handler per request after `RequireAuth`. GET/HEAD under `/admin/queues`
  require `queues:read` (seeded to `admin` by 0012 — see P0.1). Every
  non-GET/HEAD request requires `queues:write`, a new admin-only code seeded
  (`permissions` + `role_permissions` → `admin`) by P1.6's own migration
  `000N_ops_queues_write`. Non-GET requests are rejected with 403 unless
  `Sec-Fetch-Site: same-origin` (or `Origin` equals the API origin), because
  the session cookies' `SameSite=Strict` does not separate same-site siblings
  such as `minio.`, `mail.` and `traefik.` — SameSite is per site, not per
  origin. **Portal's wrapper answers every refusal, before asynqmon sees the
  request**: a non-GET/HEAD from a caller without `queues:write`, or one failing
  the same-origin check, gets the 403 `account/forbidden` Problem that
  `RequirePermission` writes. asynqmon's own `ReadOnly` mode answers **405**
  `text/plain` (`restrictToReadOnly`, asynqmon v0.7.2), so the read-only handler
  is a second line, never the answer a caller sees. Dead-lettered jobs are
  inspectable and retryable from the browser. Configure the handler's base
  path; verify its inline assets survive the current CSP/proxy setup. *(Who may
  see and mutate the console, and whether payloads are redacted, is open —
  §10 Q-C10.)*

  **Acceptance criteria.**
  - An admin retries a dead-lettered task and it re-enters `pending`.
  - A caller with only `queues:read` POSTing a retry → 403 `account/forbidden`
    Problem from Portal's wrapper (never asynqmon's 405).
  - A cross-origin POST with the admin cookie → 403 Problem.
  - A non-admin GET `/admin/queues` → 403; unauthenticated → 401.
- **P1.7 Owner takeout**: `POST /api/v1/me/export` (permission
  `takeout:write:own` — canonical verb; seeded to `user` with
  `takeout:read:own` — see P0.1) → 202 `{export_id}`; enqueues
  `ops:takeout` (a dedicated **`bulk`** queue served by its own `asynq.Server`,
  Concurrency 1 — a multi-GB export must not starve notify on the light
  server's `default` queue, SPEC-05 P0.2 step 4) fanning out through each wired module's `api/`
  **`ExportProvider`** interface — account (profile + audit trail), media
  (metadata + originals + playback progress), journal (markdown + attachments),
  bank (CSV per account + categories/budgets), people (persons +
  interactions), comic (metadata + reading progress), movie/music/story
  (catalogue metadata the owner authored), notify (prefs only; notification
  history excluded as derived), social (if user-authored rows exist, else a
  stated exclusion). The list is non-exhaustive: the specs README **Takeout**
  convention governs new modules. Produces one
  `portal-export-<yyyy-mm>.tar.gz` in storage (`exports/<user_id>/<id>.tar.gz`);
  `GET /api/v1/me/export/{id}` (`takeout:read:own`) returns status + a
  `download_url`; emits `ops:export_ready {export_id, user_id}` (future notify
  consumer). The export inherits SPEC-04 P0.5's private-archive sensitivity: it
  bundles original photos with intact EXIF/GPS.
  **Download: proxy only (decided).** Because the archive carries EXIF/GPS
  originals, the SPEC-04 P0.5 rule applies. S3/R2 presigns are replayable until
  expiry and cannot be single-use. `download_url` =
  `/api/v1/me/export/{id}/download` (§7).
  **Creation limits.** At most one export per user in `pending|running`.
  Another POST → 409 `ops/export-in-progress`, whose body carries the existing
  `export_id`. The tar.gz is written to a worker temp file (the same
  seekable-body constraint as P0.2), then uploaded with `Storage.Put`, and the
  file is removed on exit. Worker scratch disk must hold one archive. Past
  ~5 GiB, a `Storage.PutMultipart` (`feature/s3/manager.Uploader`) is the P2
  path. `ops:takeout` options: `asynq.Timeout(2h)`, `asynq.MaxRetry(3)`.
  **Provider contract.** `type ExportProvider interface { Name() string;
  Export(ctx, userID uuid.UUID, w ExportWriter) error }`, with
  `ExportWriter.Add(path string, size int64, r io.Reader) error` streaming one
  entry under `<Name()>/` (originals are streamed, never buffered in memory).
  Any provider error → export `failed` with `error='<provider>: <msg>'` and the
  partial archive deleted.
  **Boundary rule: the fan-out never touches another module's tables — providers
  only.** **Prune mechanics (2026-07-10 — previously asserted with no owner):**
  nightly janitor task `ops:purge_exports` (default queue, registered in
  events.md) runs **once per tenant** through `cmd/worker`'s `forEachTenant`
  (one committed scope per tenant, as `media:purge_orphans` does —
  `ops_exports` is tenant-scoped under FORCE RLS, so an unscoped read as
  `portal_app` errors) and deletes from storage (a) archives older than 7 days, setting the
  row's status to `expired`, and (b) immediately, any row whose
  `user_id IS NULL` (owner deleted, §6), deleting its `storage_key` object and
  then the row. The purge stays row-driven, no storage listing is needed, and a
  deleted user's EXIF/GPS originals never outlive one janitor cycle *(how
  branch (b) meets [ADR-13](SPEC-01-account-identity-admin.md#adr-13)'s delete
  order, which removes the owner's tenant last, is open — §10 Q-C13)*. `GET` on an
  expired export returns 410 Problem `ops/export-expired` — never `ready` with a
  dead URL. Excluded by design: the `notifications` table (a delivery store, not
  history — SPEC-05 §3) and other modules' derived/system rows.

  **Acceptance criteria.**
  - Given a POST, then 202 `{export_id}`, and the export reaches `ready` with an
    archive holding one `<provider>/` directory per wired provider.
  - Given an export in `pending|running`, then a second POST → 409
    `ops/export-in-progress` carrying the existing `export_id`.
  - Given a provider that errors, then the export is `failed` with
    `error='<provider>: <msg>'` and no archive object remains.
  - Given `ready`, then the download streams the archive with
    `Content-Disposition: attachment`; while `pending|running` → 409
    `ops/export-not-ready`; another user's id → 404 `ops/export-not-found`.
  - Given an export older than 7 days, then after `ops:purge_exports` the object
    is gone and `GET` → 410 `ops/export-expired`.
  - Given a user with a ready export is deleted, then after the next purge the
    archive object is gone.
  - Given a ready export, then `ops:export_ready {export_id, user_id}` is
    emitted once.

### P2 — future considerations (design for, don't build)

- *(Backup encryption and the `.env` copy were here until 2026-10-19; Decision
  2026-10-19 (C5) promoted both to P0.7.)*
- **`ops:job_dead` event** on dead-letter archive → notify ("your transcode
  failed") once SPEC-05 is live.
- **Postgres connection budget** — the pool-sizing note [ADR-03](#adr-03)
  promised (folded in from the `operations/postgres-tuning.md` stub, deleted —
  `git show ea100d8:docs/operations/postgres-tuning.md`). Two separate problems,
  not to be conflated:
  1. *Protocol* — pgx's prepared-statement cache breaks under PgBouncer
     transaction pooling (a statement prepared on one server connection may not
     exist on the next). If a transaction-mode pooler is ever put in front of
     the app, set `DefaultQueryExecMode = QueryExecModeExec` (or the simple
     protocol) / disable the statement cache — [ADR-07](SPEC-01-account-identity-admin.md#adr-07)
     § "pgx ⨯ PgBouncer transaction mode".
  2. *Capacity* — `api`, `worker` and Asynq share one cluster; without a
     connection budget a transcode burst can exhaust connections and starve
     `api`. Server targets (ADR-03): `shared_buffers = 4GB`,
     `effective_cache_size = 10GB`, `max_connections = 50`. Per-binary pgx
     `MaxConns` for `api` and `worker` are still TBD.

  Not needed at n=1. Set the numbers when a pooler is introduced, or on the
  first measured contention (a transcode burst that raises API latency) — then
  set the `worker`/`api` split and re-measure.

## 6. Data model — migration `0012_ops_backup_runs`

**Tenancy** (specs README convention, ADR-07). `ops_backup_runs` is a system table, exempt (no `tenant_id`). `ops_exports` (P1.7, unbuilt) holds per-owner rows and is tenant-scoped when built: `tenant_id` + index + FORCE RLS + `tenant_isolation` policy in its creating migration.

**One key per run.** `storage_key` holds the set's dump key; the globals and
config keys derive from its `<yyyy-mm-dd>/<run_id>` stem (P0.2), so
[ADR-18](#adr-18) needs no column change.

```sql
CREATE TABLE ops_backup_runs (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  started_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz,
  status      text NOT NULL DEFAULT 'running'
                CHECK (status IN ('running','ok','failed')),
  size_bytes  bigint,
  storage_key text,
  error       text
);
CREATE INDEX ON ops_backup_runs (started_at DESC);

-- P1.7 (may ship in a later migration with that phase)
CREATE TABLE ops_exports (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid REFERENCES users(id) ON DELETE SET NULL,
                -- NULL = owner deleted; object purged by the next ops:purge_exports
  status      text NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending','running','ready','failed','expired')),
                -- 'expired' set by ops:purge_exports (P1.7) — a pruned export
                -- must never keep reporting 'ready' with a dead URL
  storage_key text,
  size_bytes  bigint,
  error       text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  finished_at timestamptz
);
CREATE INDEX ON ops_exports (user_id, created_at DESC);
```

The status CHECK alone admits states the prose rules out (`ok` without a key or
size, `failed` without an error, `running` with `finished_at`). Follow-up
migration `000N_ops_backup_runs_state_chk` (0012 is consumed):

```sql
ALTER TABLE ops_backup_runs ADD CONSTRAINT ops_backup_runs_state_chk CHECK (
  (status = 'running' AND finished_at IS NULL)
  OR (status = 'ok' AND finished_at IS NOT NULL AND size_bytes IS NOT NULL
      AND storage_key IS NOT NULL)
  OR (status = 'failed' AND finished_at IS NOT NULL AND error IS NOT NULL)
) NOT VALID;
ALTER TABLE ops_backup_runs VALIDATE CONSTRAINT ops_backup_runs_state_chk;
```

Retention never modifies rows (P0.2 step 5), so `ok` rows keep their
`storage_key`.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/ops/status` | `ops:read` (admin) | P0.5 freshness sentinel |
| POST | `/api/v1/me/export` | `takeout:write:own` | P1.7; 202 `{export_id}`; 409 `ops/export-in-progress` (body carries the existing `export_id`) |
| GET | `/api/v1/me/export/{id}` | `takeout:read:own` | P1.7; status + `download_url`; 404 `ops/export-not-found`; 410 when expired |
| GET | `/api/v1/me/export/{id}/download` | `takeout:read:own` | P1.7; streams via `Storage.Get` with `Content-Disposition: attachment; filename="portal-export-<yyyy-mm>.tar.gz"`; 404 `ops/export-not-found` if absent or not the caller's; 409 `ops/export-not-ready` while pending/running; 410 `ops/export-expired` |
| GET/HEAD | `/admin/queues` (mounted UI, API origin) | `queues:read` (admin) | P1.6; asynqmon read-only handler, not OpenAPI |
| non-GET | `/admin/queues/*` (mounted UI, API origin) | `queues:write` (admin) | P1.6; mutations (retry, delete, archive, pause); same-origin check; not OpenAPI |

Problem types: `ops/export-not-found`, `ops/export-not-ready`,
`ops/export-expired`, `ops/export-in-progress`.
Annotate each operation per the specs README AuthZ **OpenAPI encoding**. `/admin/queues` is not an OpenAPI operation.
Tasks/events owned: `ops:backup_database`, `ops:takeout` (P1),
`ops:purge_exports` (P1), `ops:backup_completed`, `ops:backup_failed`,
`ops:export_ready`, and the notify consumer `notify:on_backup_failed` (P0.6) —
all registered in [events.md](../../reference/events.md).

## 8. Success metrics (n=1 honest)

- Leading: 7/7 mornings in the first week show a fresh dated dump off-VPS;
  `hours_since_success` never exceeds 26 outside induced failures.
- Leading: one restore drill executed and documented before the spec closes;
  quarterly thereafter (calendar-verified, not vibes).
- Lagging: a takeout archive opens and contains the expected per-module formats
  (spot-check against live data). *(P1)*

## 9. Timeline & phasing

1. Scaffold + migration + sqlc (½ day)
2. Backup task + dump + upload + retention + manifest + events + audit +
   `BACKUP_DATABASE_URL` config (1.5 days)
3. Restore drill + `make` target + runbook doc (1 day)
4. Status endpoint + OpenAPI (½ day)
5. P1: queue console (1 day); takeout fan-out + providers for wired modules +
   download endpoint (3–4 days, grows as modules land)
6. [ADR-18](#adr-18) rework (2026-10-19): backup bucket + lock + per-service
   env, globals + owner-preserving dump, age + config copy, the
   throwaway-container drill — §11 rows 2–3, 12–16; not estimated by the
   review, size it when scheduled.
P0 ≈ 3.5–4 dev-days as first estimated (brief's estimate held), before item 6;
P1 adds ~5.

## 10. Open questions

- **(resolved)** `pg_dump` location: the Postgres client in the worker image,
  major version equal to the server's (currently `postgresql18-client`; P0.2) —
  not a separate backup sidecar: fewest moving parts.
- **(resolved)** Dev backups: same code path as prod, MinIO bucket (P0.2) —
  since 2026-10-19 a locked `portal-backups` bucket of its own, not the media
  bucket ([ADR-18](#adr-18)).
- **(resolved)** Sentinel placement: separate authenticated endpoint, not
  `/healthz` (P0.5).
- **(resolved, corrected 2026-09-30)** Streaming vs temp-file for `pg_dump`
  output: **spool to a temp file.** `platform/storage.Storage.Put` needs a
  seekable body so the SDK can sign the payload. Run `pg_dump -Fc` with
  `cmd.Stdout = io.MultiWriter(tmp, sha256)`, check exit 0, `Seek(0)`, then
  `Put(tmp)`, and remove the file on exit. Worker scratch disk must hold one
  dump. Above ~5 GiB (the single-PutObject cap), add a multipart Put (P2).
  *(Since 2026-10-19 the stdout goes through age first — the temp file and the
  sha256 see only ciphertext, P0.7.)*
- **(product, non-blocking)** Takeout formats are proposed per module in P1.7 —
  lock each at that module's provider implementation *(scope and formats are
  now an owner question — Q-C11)*.

**Open — the owner review of 2026-10-19.** The review raised fourteen points;
the owner answered points 1, 2, 3, 5 and 6 (Decisions 2026-10-19 (C1)–(C3),
(C5), (C6) — [ADR-03](#adr-03), [ADR-18](#adr-18),
[ADR-04](SPEC-04-media-image-pipeline.md#adr-04)). The other nine stay open
here, numbered after the point they came from, so the next round can close
them; the specs README routes them. Text that waits on an answer is named
with it. The review's mechanical items M11, M13 and M19–M21 depend on Q-C9,
Q-C11 and Q-C12 and are corrected when those close.

- **Q-C4 (open — needs clarification) RPO, RTO, schedule and retention.**
  Recommended: RPO 24 h; RTO the same day, ≤ 4 h; run at local night
  (`CRON_TZ=Asia/Ho_Chi_Minh` — today's `0 3 * * *` UTC is 10:00 there); a
  catch-up run at worker start when the last success is older than 24 h;
  retention either 7 daily + 4 weekly kept app-side (P0.2 step 5, as today) or a
  bucket lifecycle rule, ≤ 90 days either way — "no backup copy older than 90
  days", in line with ADR-13's and ADR-14's 90-day horizons. The answer also
  fixes [ADR-18](#adr-18)'s lock length (`BACKUP_LOCK_DAYS`, 30 proposed) and
  whether pruning stays app-side. *Owner's reply so far:* "back up daily, only
  the data on that server (not what is on R2); MinIO in dev" — awaiting
  confirmation of the numbers. Waits on it: P0.2 (schedule, step 5), P0.5's
  26 h threshold, §8, ADR-18's N, D-10's RPO/RTO table in
  [feature-inventory.md](../feature-inventory.md).
- **Q-C7 (open) What a restore undoes.** Restoring a dump rolls back
  `users.token_version`, revoked refresh tokens and every disable, approval and
  password change since the dump — a revoked session or a reset password can
  come back. (a) Leave it: the runbook's warning only. (b) A mandatory
  post-restore reset: bump every `token_version`, revoke every refresh token,
  re-apply the deletes recorded in the live `deleted_users`, re-run the
  identity sweep (SPEC-01 P0.16), list the Users whose state differs from the
  live database. (c) (b) plus a deletion journal kept outside the database, so
  deletes survive when the live database is lost too. Waits on it: the
  runbook's restore warning (§ 4), ADR-13's resurrection note.
- **Q-C8 (open) Drill cadence and automation; an on-demand backup.** (a) Manual,
  quarterly, each run recorded in this spec; (b) a scheduled GitHub Actions
  restore — after encryption, and it would need the read-only token and the
  age identity as CI secrets, against ADR-18's offline identity; (c) a nightly
  `pg_restore --list` check of the new set, plus (a). Separately: an on-demand
  backup — `make backup-now` or `POST /ops/backups` (Superadmin) — replacing
  `cmd/opsenqueue` (§11 row 18). Waits on it: P0.4's cadence sentence, §8,
  backlog #32.
- **Q-C9 (open) Failure-alert audience, channel and liveness.** P0.6 says every
  `admin` holder: admins, or the Superadmins (ADR-17's audience for admin
  notices)? In-app only, or email forced regardless of preferences? An opt-in
  `BACKUP_HEARTBEAT_URL` pinged on each success (a dead-man's switch for a dead
  scheduler)? An `/admin` banner while `/ops/status` is not `ok`? Waits on it:
  P0.6 and its AC, §11 row 7, and the header's "freshness-check task (P2)",
  which no P2 bullet defines.
- **Q-C10 (open) Queue console exposure; Dragonfly durability.** (a) As P1.6
  specifies (`queues:read` read-only, `queues:write` mutations, both `admin`);
  (b) admin read with redacted payloads, `queues:write` granted to no role
  (Superadmin via `*`), every mutation audited; (c) Superadmin only. And
  Dragonfly, which has no volume in `docker-compose.yml`: (i) disposable —
  queued tasks and caches may be lost on a recreate, as today; (ii) a volume
  and snapshots. Waits on it: P1.6, §11 rows 1 and 8, ADR-03's Dragonfly note
  and action item 1.
- **Q-C11 (open) Takeout scope and formats.** (a) The owner's own data only, in
  the formats P1.7 lists; (b) (a) plus the owner's own audit trail, decrypted
  (ADR-14 makes identity data readable by Superadmins only); (c) everything the
  owner can read, shared items included. Waits on it: P1.7's provider list,
  TC-OPS-100.
- **Q-C12 (open) Takeout shape and limits.** (a) As P1.7 specifies (one
  `tar.gz`, a single `Put` to ~5 GiB, a 7-day expiry); (b) a small data
  archive plus optional originals in parts of ≤ 4 GiB, a new export replacing
  the previous one, a bell notice when ready, no archive password, and
  `GET /me/exports` to list them; (c) a streaming tar download with no stored
  archive. Waits on it: P1.7, §6 `ops_exports`, §7.
- **Q-C13 (open) Takeout and user deletion.** (a) ops implements
  `PurgeOwnerData` (ADR-13's registry) for `ops_exports` and its objects;
  (b) `ops_exports` becomes a global table with per-user RLS, outliving the
  owner's tenant. And export-before-delete: (i) the delete dialog nudges the
  User to export first; (ii) an admin `takeout:write:any` to export on
  someone's behalf. Waits on it: P1.7's purge branch (b), §6, SPEC-01 P0.10's
  registry.
- **Q-C14 (open) ADR-10's `ServerInterface` adoption.** (a) Keep retrofitting
  as each module is touched; (b) a scheduled cutover; (c) amend
  [ADR-10](#adr-10): drop the `ServerInterface` adoption and add
  response-conformance tests instead. Waits on it: ADR-10's action items,
  backlog #6.

## Decision records

The platform decisions this spec operates inside. They were standalone ADRs
under `docs/adr/` until the 2026-10-01 fold; the `ADR-NN` IDs are unchanged and
each record keeps a fixed anchor. ADR-01 (the v1 scope cut and its single-VPS
envelope), ADR-10 (the OpenAPI contract) and ADR-16 (the list API contract,
whose clamp lives in `platform/server`) were held in the specs README until
2026-10-02, when every record moved into a spec; they moved verbatim. Decision, Options considered and Trade-offs
are verbatim (ADR-11 rule 2 — [SPEC-06](SPEC-06-docs-canonicalisation.md#adr-11));
the fact layer was re-checked against the repo on 2026-10-01. ADR-18 (backups)
was written here on 2026-10-19, from Decisions 2026-10-19 (C2), (C5) and (C6);
the same day's C1 re-affirmed ADR-03 in a note after its narrative.

<a id="adr-01"></a>
### ADR-01 — v1 scope cut: what fits in 2 weeks / 1 dev / $100/mo / 1 VPS

**Decided:** 2026-05-24 · **Status:** accepted; amended by [ADR-08](SPEC-09-life-stream-home.md#adr-08) (scope since widened); executed (the demo loop shipped 2026-07-06)

Deciders: kirito.

#### Context

*As found on 2026-05-24. The cut this record made was executed and closed: the
demo loop shipped on 2026-07-06 and is the regression baseline. What has been
built since is not tracked here — [`/CLAUDE.md`](../../../CLAUDE.md) § Current
status owns that.*

[`feature-inventory.md`](../feature-inventory.md) (then `feature.md`) described 12 phases and 40 settled decisions covering identity, multi-tenancy, media, four content verticals, personal finance, notifications, social, search, marketing site, advanced social (reels/live/audio rooms), creator economy, marketplace, and ML safety. The decisions are individually sound; collectively they describe a platform that would take a small team a year or more to ship.

The stated constraint envelope was:

- 1 developer
- 2 weeks to v1
- ≤ $100/month infrastructure budget
- single VPS

A version of Portal that tried to honour every Phase 0 deliverable in 2 weeks would run out of time around Phase 0 step 8 (out of 14) and ship nothing. A version that picked one coherent slice, shipped it, and treated the rest as a backlog could produce a *running* artefact at the end of the sprint.

This record made the cut explicit so it was a decision, not a drift.

<!-- adr-narrative -->
#### Decision

**v1 ships Phase 0 (foundation wiring) plus a vertical slice of Phase 2 (one video upload happy path) and nothing else.** Everything in Phases 1, 3–12 was deferred by this cut. The phase ordering in `feature.md` was unchanged; the scope of what counted as "v1" was the only thing this ADR moved.

Concretely, v1 = the smallest demo that proves the architecture works end-to-end. As built (two steps differ from the 2026-05-24 text: sign-in was to be Authentik/OIDC and the upload was to go straight to R2):

1. A user signs in with a local password ([ADR-06](SPEC-01-account-identity-admin.md#adr-06): `POST /api/v1/auth/login`; Authentik/OIDC never shipped).
2. They land on the Next.js home page authenticated.
3. They upload an mp4 via the UI.
4. The upload is persisted to MinIO in dev and R2 in deployed environments ([ADR-04](SPEC-04-media-image-pipeline.md#adr-04)).
5. The worker picks up the transcode task, produces an HLS ladder, and updates `assets.status = ready`.
6. The user plays the video back in the browser using Vidstack.
7. They sign out; the session is revocable via the existing two-channel mechanism.

That was the entire v1 demo loop. At the time of the cut: no tenants, no movies/music/stories/comics CRUD, no bank, no social, no notifications, no mediamtx, no LiveKit, no observability stack, no file-gated permissions, no policy bundles, no marketplace. Of those, tenants, the four verticals, bank, notifications and a first social slice have since shipped under ADR-08; the rest remain out.

#### Options considered

##### Option A — Honour Phase 0 in full, defer everything else

| Dimension | Assessment |
| --- | --- |
| Complexity | High — 14 deliverables in Phase 0 alone |
| Cost | $30–60/mo |
| Scalability | N/A (foundation only) |
| Team familiarity | Solo dev knows this stack |

**Pros:** Phase 0 is the "spec-correct" sprint; every piece set up here pays dividends in every later phase.
**Cons:** 14 deliverables in 2 weeks for 1 dev is ~2 hours each including testing — unrealistic when several (CI workflows, frontend conventions doc, migration audit, RFC 7807 retrofit, observability stack) are multi-hour items. The most likely outcome is "Phase 0 partially done, no running demo."

##### Option B — Phase 0 minimum + Phase 2 vertical slice  *(chosen)*

| Dimension | Assessment |
| --- | --- |
| Complexity | Medium — cut Phase 0 from 14 items to ~8 |
| Cost | $30–60/mo |
| Scalability | Single-user demo; multi-tenant deferred |
| Team familiarity | Solo dev knows this stack |

**Pros:** Produces a running demo end-of-sprint. Forces the wiring gap to close on Day 3. Surfaces the integration bugs (cookie flags, CORS, oapi-codegen handler shape, sqlc adapter signatures) that are the actual risk.
**Cons:** Skips the migration audit ([D-18]), the observability stack ([D-8]), the frontend conventions doc ([D-32]/[D-33]), CI workflows ([D-9]), the OpenAPI cross-module schema retrofit ([D-29]). All of these have to land later; some will hurt to retrofit.

##### Option C — Skip Phase 0, hand-write a thin auth layer + media demo

| Dimension | Assessment |
| --- | --- |
| Complexity | Low for v1 |
| Cost | $30/mo |
| Scalability | Throwaway — would need full rewrite |
| Team familiarity | Solo dev knows this stack |

**Pros:** Fastest path to a running demo.
**Cons:** Throws away the existing account module (which is already written), the OpenAPI spec, the module boundary discipline, and the modular monolith layout. Builds technical debt the rest of the year is paying off. Only correct if v1 is a *throwaway prototype*; if it's the seed of the real product, this is wrong.

#### Trade-offs

Option A's failure mode is "no demo at end of sprint, lots of half-finished plumbing." Option C's failure mode is "demo works, can't extend it." Option B's failure mode is "demo works, missing some Phase 0 niceties that need a Phase 0.5 sprint." Of the three, Option B's failure mode is the cheapest to recover from: the missing pieces (CI workflows, frontend conventions doc, observability profile) can each be added in a half-day sprint without touching application code.

The cut to ~8 Phase 0 deliverables is:

| Phase 0 deliverable (from feature.md) | v1? | Reason |
| --- | --- | --- |
| Wire `cmd/api/main.go` | **Yes** | The actual blocker. |
| `make sqlc` for account block + commit decision | **Yes** | Required before adapters compile. |
| Repository adapters for account interfaces | **Yes** | Required to construct the module. |
| Migration `0001` audit (split into 0001/0002/0003/0005) | **Yes** | Cheap to do *now* before data exists; impossible later. [D-18] |
| `users.locale` + `users.timezone` columns | **Yes** | One-line addition during the audit; needed by frontend on day one. |
| Move `audit/` → `platform/audit/` + rename event | **Yes** | Cheap during the migration audit; expensive after audit_log has rows. [D-25] |
| Surface `amr`/`acr`/`auth_time` claims into context | **Yes** | One file change; lets [D-27]/[D-28] land later without rewriting middleware. |
| `user_oidc_roles` table | **Yes** | Lands in `0003_account_rbac`; OIDC group sync writes to it on first login. [D-26] |
| RFC 7807 `Problem` adoption in OpenAPI | **Partial** | Add the schema; retrofit handlers as they're written, not in a sweep. |
| Reserve `notify:*` Asynq prefix | **Yes** | Documentation-only; one line in MODULES.md §5.2. |
| OpenAPI cross-module schemas (Money, PaginatedResult, TenantContext, ContinuingItem) | **No** | Money/Continue/TenantContext aren't needed until bank/Phase 4/tenant ships. Add when first needed. |
| URL versioning + RFC 9745 doc | **No** | `/api/v1/` is already in place; the doc is paperwork that can land in week 3. |
| Frontend server-only API client + refresh-and-return route | **Yes** | Without this, RSC pages can't authenticate against the API. [D-34] |
| Frontend conventions doc (Zustand/TanStack/RHF boundary) | **No** | Solo dev; a doc for an audience of one is paperwork. Add when a second contributor appears. [D-32]/[D-33] |
| CI workflows (lint + test + drift + roundtrip + build + security) | **Partial** | Ship the drift check (`sqlc-drift`, `openapi-drift`) only. Skip multi-arch builds, security scan, integration matrix until week 3. [D-9] |

For Phase 2 the v1 cut is: one queue priority, libx264 only, no hardware encoder paths, no per-user/per-tenant quotas, no backpressure, no dead-letter queue UI (failed transcodes get logged loudly and the operator fishes them out by hand). The single happy-path flow proves the architecture; the polish lands in Phase 2.5.
<!-- /adr-narrative -->

#### Consequences

**What became easier:**

- The 2-week sprint had a single, demonstrable success criterion: the 7 steps above. It ran, and it still runs — it is the regression baseline every later change is checked against.
- The wiring gap (the actual blocker) closed first because everything else depended on it.
- Solo-dev cognitive load dropped — only the modules touched by the demo loop needed to be understood deeply in week 1.

**What became harder, and how it resolved:**

- The migration audit was done under v1 rather than punted; it cost a day and was worth it.
- The frontend conventions doc was skipped for v1 and later written as [`frontend/CLAUDE.md`](../../../frontend/CLAUDE.md) ([D-32]/[D-33]/[D-34]).
- The observability stack was skipped and is still absent ([ADR-03](SPEC-03-platform-ops.md#adr-03)).
- The `/t/{tenant}/...` URL prefix was never adopted. Tenancy landed ([ADR-07](SPEC-01-account-identity-admin.md#adr-07)) with routes staying under plain `/api/v1`; the tenant is resolved by middleware (`RequireTenant` in `cmd/api/main.go`), not by the path.
- The [D-34] refresh-and-return route was replaced by the `portal_session` middleware gate plus `SessionKeeper` client-side silent refresh.

**What we said we'd revisit:**

- A Phase 0.5 sprint for the skipped items never ran as such; CI landed in Phase 6 (the `backend`, `lint`, `openapi`, `frontend` and `link-check` jobs in `.github/workflows/ci.yml` — there is no `sqlc-drift` job and never was, sqlc output is not committed), the conventions doc landed with the frontend work, observability has not.
- Phase 1 (tenancy + RLS) landed per ADR-07; the `me` synthetic tenant was not carried forward — personal organisations are real rows.
- The RBAC schism was resolved by [ADR-02](SPEC-01-account-identity-admin.md#adr-02) before tenancy, as intended.

#### Action items

1. [x] Pin this record (`Accepted`) before writing any code for the 2-week sprint (2026-07-06 status flip; v1 was built under this cut).
2. [x] Acceptance criterion tracked and met — the loop shipped 2026-07-06. (It was tracked in `MILESTONE_CHECKS.md`, deleted in `f11cf3f`; status now lives in code, see `/CLAUDE.md`.)
3. [x] ~~A `v1-out-of-scope` label in the issue tracker~~ — won't do (closed 2026-10-19): GitHub Issues on `jackone2991/portal` use the five triage labels of [triage-labels.md](../../agents/triage-labels.md), and the deferred list lives in [backlog.md § Deferred](../backlog.md) (and `/CLAUDE.md` § "Still deferred"); a label would be a second copy of it.
4. [x] Scope comment at the top of `backend/cmd/api/main.go`. It still cites the file this record was folded from (`docs/adr/01-v1-scope-cut.md`, retired by the 2026-10-01 fold); it is corrected to `docs/product/specs/SPEC-03-platform-ops.md#adr-01` when that file is next touched.
5. [x] ~~Sprint-end retrospective~~ — won't do (closed 2026-10-19): the sprint closed on 2026-07-06 and a retrospective written now would be reconstruction, not record. The nearest thing was the 2026-07-11 spec-gap worklog (deleted — `git show ea100d8:docs/product/analysis/spec-gap-fix-worklog-2026-07-11.md`).

<a id="adr-03"></a>
### ADR-03 — Single-VPS topology and compose-profile envelope for v1

**Decided:** 2026-05-24 · **Status:** accepted; re-affirmed 2026-10-19 (Decision 2026-10-19 (C1): production is the rented VPS with R2 this record decides, the Windows machine the stack runs on is development; the note follows the Trade-off analysis)

Deciders: kirito. Affects: [docker-compose.yml](../../../docker-compose.yml),
[Makefile](../../../Makefile), [D-8] (observability), [D-36]/[D-39] (live +
calls) in [feature-inventory.md](../feature-inventory.md).

#### Context

*As found on 2026-05-24. Two things changed after the decision and are
reflected below: [ADR-06](SPEC-01-account-identity-admin.md#adr-06) (2026-07-05) removed
Authentik before it ever shipped, and on 2026-08-21 Postgres moved out of
compose onto the host cluster. The sizing analysis in this section and in
Trade-offs rests on Authentik; it is kept as written because it is why CCX23
was chosen.*

`docker-compose.yml` then brought up Traefik + Postgres + PgBouncer + Dragonfly + MinIO + API + Worker + Frontend. The corpus implied additional services landing progressively: Authentik (OIDC), Mailpit (dev email), a 5-service observability stack ([D-8]: Loki, Promtail, Prometheus, Tempo, Grafana, GlitchTip), mediamtx (live ingest, [D-36]), LiveKit (group calls, [D-39]), and FFmpeg-bound workers under bursty load.

The constraint envelope is **single VPS, ≤ $100/mo**. At reasonable VPS prices, this translates to:

| Tier | Example (Hetzner) | vCPU | RAM | Disk | ~$/mo |
| --- | --- | --- | --- | --- | --- |
| Bare minimum | CCX13 | 2 dedicated | 8 GB | 80 GB SSD | ~$13 |
| Recommended v1 | CCX23 | 4 dedicated | 16 GB | 160 GB SSD | ~$30 |
| With headroom | CCX33 | 8 dedicated | 32 GB | 240 GB SSD | ~$60 |
| Ceiling | CCX43 | 16 dedicated | 64 GB | 360 GB SSD | ~$120 |

Cloudflare R2 storage is ~$0.015/GB-month with no egress fees inside Cloudflare's network. 100 GB of stored HLS = $1.50/mo; bandwidth to viewers is free at the edge. So infra budget is dominated by the VPS itself.

Once Authentik (~1 GB resident), Postgres (~500 MB shared_buffers + work), Dragonfly (~256 MB allocated, scales with cache), MinIO (~150 MB), Traefik (~50 MB), API + Worker (~300 MB combined idle), Frontend Next.js SSR (~250 MB), and a transcode burst (FFmpeg can spike 1–2 GB for 1080p+ content) are stacked, the **floor is ~3.5 GB RAM idle, ~6 GB under transcode**. CCX13 (8 GB) is too tight; CCX23 (16 GB) is the right v1 tier.

If the observability stack ([D-8]) is brought up, add ~1.1 GB. If LiveKit + mediamtx + coturn are brought up, add ~500 MB idle plus bursty CPU/bandwidth. CCX23 cannot host both observability AND live streaming AND a transcode burst simultaneously.

<!-- adr-narrative -->
#### Decision

**v1 runs on Hetzner CCX23 (4 vCPU / 16 GB / 160 GB) or equivalent (~$30/mo). The following are explicitly *out* for v1** — and none of them exists in `docker-compose.yml`: the file has no `profiles:` key at all, so "off" means "not written", not "behind a flag":

- observability (Loki/Prometheus/Tempo/Grafana/GlitchTip) — defer until traffic justifies it. v1 runs with stdout JSON logs.
- live (mediamtx) — live streaming is Phase 10. Not v1.
- calls (LiveKit + coturn) — voice/video is Phase 12. Not v1.

**The v1 service set, as built** (`grep -E '^  [a-z][a-z0-9_-]+:$' docker-compose.yml`; as decided it also carried `authentik-server` + `authentik-worker`, which ADR-06 removed before they shipped, and `postgres` + `pgbouncer`, which moved to the host on 2026-08-21):

| Service | Role | RAM (idle) | Notes |
| --- | --- | --- | --- |
| `traefik` | TLS terminator + reverse proxy | ~50 MB | Single edge; routes by Host + path |
| *(host)* Postgres 18 | Database | — | **Not in compose.** Runs on the host cluster, reached at `host.docker.internal:5432`; `make up` does not start it. `postgres`/`pgbouncer` are commented out in the compose file with the rollback recipe; the `postgres_data` volume is retained. |
| `dragonfly` | Redis-compatible cache + Asynq broker | ~256 MB | `--default_lua_flags=allow-undeclared-keys` (Asynq's Lua needs it); **no `--maxmemory` cap** (action item 1) |
| `minio` + `minio-setup` | Dev S3 origin, bind-mounted at `./data/minio` | ~150 MB | Dev only; prod is R2 ([ADR-04](SPEC-04-media-image-pipeline.md#adr-04)) |
| `mailpit` | Dev SMTP sink + web UI (`mail.${APP_DOMAIN}`) | ~30 MB | `SMTP_HOST=mailpit` in `.env.example`; prod points `SMTP_*` at a real relay and drops it. Shipped for Portal's own mail (password reset, notify), not for Authentik. |
| `api` | Go HTTP server (`cmd/api`) | ~150 MB | Single replica |
| `worker` | Asynq consumer (`cmd/worker`) | ~150 MB idle, 1–2 GB during transcode | Three servers; the heavy pool is `heavyConcurrency = 1` (a const, not an env knob) |
| `scraper` | Python comic scraper (FastAPI + headless Chrome) | Chrome-sized | Not in the original decision; see `scraper/README.md` |
| `frontend` | Next.js SSR | ~250 MB | Single replica |

Headroom on a 16 GB VPS is ample for v1 and gives room to add observability without resizing.

**Cloudflare R2** is the only off-VPS dependency (storage origin; see [ADR-04](SPEC-04-media-image-pipeline.md#adr-04)). DNS via Cloudflare is assumed (free tier sufficient).

**Storage** on the VPS itself: Dragonfly snapshots and the MinIO bind-mount live on the box (Postgres data lives with the host cluster). Dev uploads land in MinIO; deployed environments upload straight to R2 (ADR-04), which is what saves the disk that would otherwise hold replicated assets.

#### Options considered

##### Option A — CCX13 (2 vCPU / 8 GB) at ~$13/mo

| Dimension | Assessment |
| --- | --- |
| Cost | Best — under $20/mo |
| Headroom | None — Authentik + Postgres + transcode burst will OOM |
| Future-proofing | Forces a migration to a bigger VPS within months |

**Pros:** Cheapest possible. Fits a hobbyist who never transcodes >720p.
**Cons:** Authentik alone is 1 GB resident; one 1080p transcode and the kernel kills something. Not viable for the 7-step demo.

##### Option B — CCX23 (4 vCPU / 16 GB) at ~$30/mo  *(chosen for v1)*

| Dimension | Assessment |
| --- | --- |
| Cost | $30/mo leaves $70 budget for R2, DNS, future paid tiers |
| Headroom | Comfortable idle; one concurrent transcode survives |
| Future-proofing | Can add observability profile without resize; live streaming would force a resize |

**Pros:** Right-sized for v1 + Phase 0.5 expansion. Cheap to upgrade in-place to CCX33 if needed.
**Cons:** Cannot run multiple concurrent transcodes; `TRANSCODE_CONCURRENCY=1` is a hard floor.

##### Option C — CCX33 (8 vCPU / 32 GB) at ~$60/mo

| Dimension | Assessment |
| --- | --- |
| Cost | $60/mo + ~$20 R2/Cloudflare = ~$80; still under budget |
| Headroom | Comfortable with observability + 2-3 concurrent transcodes |
| Future-proofing | Runway through Phase 5 (bank) before resize |

**Pros:** Plenty of room; no resize until Phase 7 (social).
**Cons:** Pays for capacity v1 doesn't use. Start smaller; upgrade in-place when needed.

##### Option D — Split across two cheap VPSes (one app, one DB/storage)

| Dimension | Assessment |
| --- | --- |
| Cost | ~$26 (2 × CCX13) |
| Headroom | DB on dedicated box; app on the other |
| Operational complexity | Higher — private network, certs, monitoring across two hosts |

**Pros:** Cheaper than CCX23 by ~$4.
**Cons:** Violates the "single VPS" constraint stated upfront. Adds ops complexity for marginal savings. Skip.

#### Trade-off analysis

The pivotal question is the **memory pressure from Authentik plus a transcode burst**. Without Authentik, an 8 GB VPS would do. With Authentik, 16 GB is the floor. The alternative (skipping Authentik in favour of a hand-rolled local password store) trades ~1 GB of RAM for 3 days of solo-dev time writing password storage + reset flow + email templates + lockout logic; the time is more valuable than the RAM.

Cloudflare R2 saving the VPS disk is the second-largest decision. Storing assets locally on the VPS means provisioning ≥240 GB for any meaningful library, which forces CCX33 minimum and a backup strategy (R2 replication or rsync). Sending uploads directly to R2 sidesteps both — see [ADR-04](SPEC-04-media-image-pipeline.md#adr-04).

Disabling the observability profile for v1 is the cheapest call in this ADR. Loki + Prometheus + Tempo + Grafana + GlitchTip cost 5 services and ~1.1 GB for telemetry no one is reading in week 1. `docker compose logs api worker` covers the demo loop.
<!-- /adr-narrative -->

**Re-affirmed 2026-10-19 (C1):** the owner's review of this spec asked whether
the machine the stack actually runs on had become production. It has not
(Decision 2026-10-19 (C1)): **production is the rented VPS with Cloudflare R2**
that the Decision above and [ADR-04](SPEC-04-media-image-pipeline.md#adr-04)
describe. The owner's Windows machine — Postgres 18 on the host at
`G:\data\postgres\18`, MinIO on a bind mount, Docker Desktop — is the
**development** environment. Two things follow. The production compose overlay
and `docs/operations/deployment.md` are written before more real data accrues
on the development machine ([backlog.md](../backlog.md) #30). And a backup
leaves the box only in production: a development backup bucket shares the
machine's disk ([ADR-18](#adr-18)).

#### Consequences

**As built — development only (re-checked 2026-10-19, C1):**

- The stack runs on one Windows machine under Docker Desktop: Postgres 18 on the
  host (`G:\data\postgres\18`, reached at `host.docker.internal:5432`), MinIO
  bind-mounted at `./data/minio` (or `MINIO_DATA_DIR`), the rest in
  `docker-compose.yml`. That is the development environment.
- Production — the VPS of the Decision above with R2 — is the **target**: no
  production overlay or deployment guide exists in the repo.
  `docker-compose.yml`'s header names a production overlay file that was never
  written ([SPEC-03](#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01)
  §11 row 17); writing it is backlog #30.

**What became easier:**

- Running the stack is `make up` (`docker compose up -d`, no profile flags — there are no profiles to forget). There is no deploy script beyond the Makefile, and no production deployment yet.
- Cost ceiling is predictable: $30/mo VPS + ~$5/mo R2 + Cloudflare free tier = ~$35/mo, well under budget — the production target's figure, not a running bill.
- Mailpit in the stack from day one means every mail path (password reset, notification email) is testable end-to-end in dev.

**What became harder:**

- No observability — when the demo breaks at the customer's site, the only diagnostics are container logs (`docker compose logs api worker`). Still true on 2026-10-01.
- Heavy-queue concurrency of 1 means a slow source video blocks the transcode queue. Acceptable for v1 (single demo user); becomes a real bottleneck under multi-tenant usage. The per-tenant quota wiring [D-13] has not landed.
- Postgres on the host means `make up` does not give you a database; the host cluster must be running and reachable at `host.docker.internal:5432`, and tuning lives with the host, not in compose.

**What we'll need to revisit:**

- When live streaming lands, mediamtx + concurrent transcodes will push the VPS over 16 GB. Plan the CCX33 upgrade (or split to a media-dedicated VPS) ahead of that sprint.
- The backup strategy [D-10] shipped as the `ops` module — this spec (§5 P0.2 `ops:backup_database` + retention, P0.4 restore drill, runbook [backup-restore.md](../../operations/backup-restore.md)), reshaped on 2026-10-19 by [ADR-18](#adr-18). Dragonfly is not part of it, and the Decision's "Dragonfly snapshots … live on the box" does not hold: `docker-compose.yml` gives `dragonfly` no volume and no snapshot flags, so nothing it holds — queued Asynq tasks included — is guaranteed to survive the container being recreated. Whether that stays so is SPEC-03 §10 Q-C10.
- Observability was to land with tenancy so per-tenant latency is measurable from day one [D-8]. Tenancy landed ([ADR-07](SPEC-01-account-identity-admin.md#adr-07)); observability did not.

#### Action items

1. [ ] Cap Dragonfly memory before it competes with FFmpeg. Shipped command is `["--logtostderr", "--default_lua_flags=allow-undeclared-keys"]` (Asynq needs the Lua flag); `--maxmemory` is still absent.
2. [x] ~~Add `authentik-server`, `authentik-worker`, and `mailpit` services.~~ Obsolete per ADR-06: Authentik dropped. `mailpit` shipped on its own merits.
3. [ ] Document the out-of-scope services in `docker-compose.yml` with a one-line comment pointing at this record (`docs/product/specs/SPEC-03-platform-ops.md#adr-03`). Not done; the file has no such comment, and its header instead names a production overlay that does not exist — SPEC-03 §11 row 17 fixes both in one edit.
4. [x] ~~`make deploy-v1`~~ — moot (closed 2026-10-19): with no `profiles:` in the file, plain `make up` cannot bring up anything it shouldn't; the production deployment the target stood for is backlog #30.
5. [x] Postgres tuning values are recorded once, in §5 P2 "Postgres connection budget" above (the `operations/postgres-tuning.md` stub that first held them was folded in there and deleted). They now apply to the host cluster; there is no PgBouncer.
6. [ ] `docs/operations/deployment.md` — still absent. The VPS sizing rationale lives only in this record. Since Decision 2026-10-19 (C1) it ships with the production compose overlay, before more real data accrues on the development machine ([backlog.md](../backlog.md) #30, P0).
7. [x] Transcode concurrency is 1 — as a compile-time constant (`heavyConcurrency` in `cmd/worker/main.go`), which is the OOM guard [SPEC-04](SPEC-04-media-image-pipeline.md) P0.1 relies on. Image processing has the env knob (`IMAGE_CONCURRENCY`, default 3, in `.env.example`). `MAX_CONCURRENT_TRANSCODES_PER_USER` does not exist; per-user limits wait on [D-13].

<a id="adr-05"></a>
### ADR-05 — Phase 0 wiring order — the critical path to a running demo

**Decided:** 2026-05-24 · **Status:** accepted, executed (closed 2026-07-06)

Deciders: kirito. Affects: [cmd/api/main.go](../../../backend/cmd/api/main.go),
[cmd/worker/main.go](../../../backend/cmd/worker/main.go),
[backend/internal/modules/account/module.go](../../../backend/internal/modules/account/module.go),
[backend/sqlc.yaml](../../../backend/sqlc.yaml),
[backend/db/migrations/](../../../backend/db/migrations/).

#### Context

*As found on 2026-05-24. This record is a sequencing plan; it ran, and every
milestone closed by 2026-07-06. It is kept for the shape of the work — the
migrations → sqlc → adapters → construction order still applies to every new
module (`backend/MODULES.md` § 8). Three milestones were delivered differently
from the text below: 0.4 (OIDC) shipped as local password auth per
[ADR-06](SPEC-01-account-identity-admin.md#adr-06); 0.5's refresh-and-return route became the
`SessionKeeper` client-side silent refresh with Next.js middleware gating on the
`portal_session` cookie, and no `server-only` API client was written; 0.6's CI
landed later and larger (see Consequences). Milestone text is left as planned.*

CLAUDE.md stated the blocker plainly at the time:

> `cmd/api/main.go` still has a `TODO: mount OpenAPI-generated handlers` comment and does not yet call `account.New(...)` or any module's `MountHTTP`. The account module assembles its handler internally inside `backend/internal/modules/account/module.go`; the API binary just hasn't been taught to construct it. Wiring is deferred until repository adapters land.

> `internal/modules/*/repository/` directories exist but are empty. The interfaces consumed by the account module (`AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`) need adapters around the sqlc-generated code once `make sqlc` runs.

Every v1 deliverable depended on closing this gap. The 2-week sprint could not afford a wrong sequence — re-doing migrations after sqlc generation has run, for instance, costs the rest of a day.

[ADR-01](#adr-01)'s v1 cut kept 8 Phase 0 items. This record put them in execution order.

<!-- adr-narrative -->
#### Decision

**Phase 0 lands in 5 strictly-sequenced milestones over Days 1–6 of the sprint, before any feature work begins.** Each milestone ends with a concrete check the developer can run.

##### Milestone 0.1 — Migration tree audit (Day 1, ~4 hours)

Before sqlc runs and freezes the schema, split `0001` per [D-18]:

```
0001_platform_init.up.sql          extensions (uuid-ossp, citext, pg_trgm), common helper functions
0002_account_users.up.sql          users (no role col; +locale +timezone +token_version +disabled_at)
0003_account_rbac.up.sql           roles, role_parents, permissions, role_permissions, user_roles,
                                   user_oidc_roles (per [D-26])
0004_account_sessions.up.sql       refresh_tokens (with parent_id, replaced_by_id, revoked_at)
0005_platform_audit.up.sql         audit_log (moved from account; per [D-25])
```

Each `up.sql` has a matching `down.sql`. The `assets` table (was in old `0001`) and any media tables are deferred — they don't ship in Milestone 0.x.

**Check:** `make migrate && make migrate-down && make migrate` runs clean. Document this as the v1 acceptance for migrations.

##### Milestone 0.2 — sqlc generation + repository adapters (Day 2, ~6 hours)

1. Run `make sqlc` for the account block. Generated code lands in `backend/internal/modules/account/repository/`.
2. Write adapters that implement the interfaces account already declares:
   - `AuthSnapshotFetcher` — wraps `GetUserAuthSnapshot` (returns `id`, `token_version`, `disabled_at`).
   - `RefreshStore` — wraps `InsertRefreshToken`, `GetRefreshToken`, `RevokeRefreshTokenChain` (recursive CTE for theft detection).
   - `PermissionFetcher` — wraps `GetEffectivePermissions` (recursive role-ancestor walk).
   - `EventStore` — wraps `InsertAuditEvent`. Now in `platform/audit/`, not `account/audit/` (per Milestone 0.1 / [D-25]).
   - `UserUpserter` — wraps `UpsertOidcUser`, `SyncOidcRoles`. *(As shipped: the OIDC upsert was retired with ADR-06; local-auth queries took its place.)*

Adapters are 1:1 with sqlc-generated functions; no business logic. They live in `backend/internal/modules/account/repository/adapter.go` (one file, alphabetical).

**Check:** `go build ./...` succeeds across all packages. No `// TODO: adapter` comments left in account.

##### Milestone 0.3 — Construct the account module in `cmd/api/main.go` (Day 3, ~6 hours)

The wiring sequence in `cmd/api/main.go`:

```go
func main() {
    cfg := config.MustLoad()                                    // env loader
    logger := platformlog.New(cfg.Env)                          // stdout JSON in v1
    pgPool := db.MustOpen(ctx, cfg.DatabaseURL)                 // pgxpool
    cache := cache.NewDragonfly(cfg.RedisURL)                   // Dragonfly client
    asynqClient := jobs.NewClient(cfg.RedisURL)                 // Asynq producer

    auditLogger := audit.NewLogger(pgPool, logger)              // platform/audit (per [D-25])

    accountMod, err := account.New(account.Deps{
        DB:                 pgPool,
        Cache:              cache,
        Audit:              auditLogger,
        OIDCIssuerURL:      cfg.OIDCIssuerURL,
        OIDCClientID:       cfg.OIDCClientID,
        OIDCClientSecret:   cfg.OIDCClientSecret,
        OIDCRedirectURL:    cfg.OIDCRedirectURL,
        JWTSigningKeys:     cfg.JWTSigningKeys,                 // comma-separated, rotating kid
        CookieDomain:       cfg.CookieDomain,
        CookieSecure:       cfg.CookieSecure,
        BootstrapAdminSubs: cfg.BootstrapAdminOIDCSubjects,     // per [D-26]
        OIDCGroupRoleMap:   cfg.OIDCGroupRoleMap,
    })
    must(err)

    r := chi.NewRouter()
    r.Use(middleware.RealIP, middleware.RequestID, middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(corsMiddleware(cfg))                                  // configured per env
    r.Use(ratelimit.Middleware(cache))

    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/healthz", healthz(pgPool, cache))
        accountMod.MountHTTP(r)                                 // mounts /auth/*, /me/*
        // v1 stops here. Future modules append their MountHTTP under r.
    })

    server := &http.Server{Addr: ":8080", Handler: r}
    logger.Info("api listening", "addr", server.Addr)
    must(server.ListenAndServe())
}
```

*(Planning sketch. As shipped the OIDC `Deps` fields are gone (ADR-06), and every module under `internal/modules/` — `ls -d backend/internal/modules/*/` — is constructed and mounted under `/api/v1` the same way.)*

The same shape applies to `cmd/worker/main.go` with each module's `RegisterTasks(mux)`. (As shipped, account has no `RegisterTasks` — it enqueues into `notify:*` instead — and media splits into `RegisterHeavyTasks` / `RegisterImageTasks` / `RegisterLightTasks`, one per Asynq server; see `/CLAUDE.md` § Job queue.)

**Check:** `make up && go run ./cmd/api` (or `make dev`) starts. `curl http://localhost:8080/api/v1/healthz` returns 200 with `{"status":"ok","db":true,"cache":true}`.

##### Milestone 0.4 — OIDC end-to-end (Day 4, ~6 hours)

With Authentik running in compose (per [ADR-03](#adr-03)), the OIDC handshake from `diagrams.md` §5 must work:

1. Configure Authentik provider for Portal: client ID + secret, redirect URI `https://${APP_DOMAIN}/api/v1/auth/callback`, allow `openid profile email groups` scopes.
2. Set `OIDC_GROUP_ROLE_MAP=portal-admins:admin` and create a `portal-admins` group in Authentik.
3. Create your own user in Authentik, add to `portal-admins`, set `BOOTSTRAP_ADMIN_OIDC_SUBJECTS=<your-sub>`.
4. Browser flow: visit `${APP_DOMAIN}` → frontend redirects to `/api/v1/auth/login` → 302 to Authentik → log in → callback → `users` row created, `user_oidc_roles` populated, access + refresh cookies set → redirect to `/`.
5. `curl -b cookies.txt https://${APP_DOMAIN}/api/v1/me` returns the user payload.
6. `curl -b cookies.txt -X POST https://${APP_DOMAIN}/api/v1/auth/logout-all` bumps `token_version`; the next `/me` returns 401.

**Check:** above 6 steps work without manual SQL.

##### Milestone 0.5 — Frontend server-only API client + RSC auth handoff (Day 5–6, ~10 hours)

Per [D-34]:

1. Create `frontend/src/lib/api-server.ts` with `import "server-only"`. Wraps `fetch` to read `cookies()` and inject `Cookie:` on outgoing API calls.
2. Create `frontend/src/lib/api-client.ts` (no server-only guard) for client-component fetches; uses `credentials: 'include'`.
3. Create `frontend/src/app/auth/refresh-and-return/route.ts` — receives `return_to=<path>` query param, calls `/api/v1/auth/refresh`, redirects to `return_to`.
4. Create a `<Sign in>` button on the index page that links to `/api/v1/auth/login`.
5. Create `/account/page.tsx` (RSC) that fetches `/api/v1/me` and renders the user. On 401, throws `redirect('/auth/refresh-and-return?return_to=/account')`.

**Check:** unauth user clicks Sign in, lands at Authentik, logs in, returns to `/account`, sees their email. Refreshing after access token expiry triggers the refresh-and-return flow once and lands back on `/account`.

##### Milestone 0.6 — Reserve naming + minimal CI (parallel, ~3 hours)

These happen alongside but don't gate the milestones above:

- Add `notify:*` Asynq prefix reservation note to `backend/MODULES.md` §5.2 (per [D-1]).
- Add a minimal `.github/workflows/ci.yml` with just two jobs: `sqlc-drift` (`make sqlc && git diff --exit-code`) and `openapi-drift` (`make openapi && git diff --exit-code`). Skip lint/test/security for v1. Drift detection alone catches the most expensive class of bugs.
- Add `# v1 scope: ADR-01` comment header to `cmd/api/main.go`.

#### Options considered

##### Option A — sqlc first, then migrations  *(rejected)*

Generates code against an unaudited schema. Splitting migrations afterwards forces a regeneration that may rename functions/types, breaking adapters mid-sprint. Order matters — migrations are the contract sqlc reads.

##### Option B — Construct modules before sqlc adapters exist  *(rejected)*

The account module's `New(Deps)` constructor requires the adapters as inputs. Stubbing them with no-ops to get the binary running is tempting but creates a "construct then re-wire" double-pass. Worse, it hides the adapter shape bugs (interface mismatches between what sqlc generates and what the account module wants) behind a green build.

##### Option C — Skip Authentik in v1, hand-roll password auth  *(rejected)*

Saves ~1 GB RAM and 1 day of Authentik config. Costs 3 days of password storage + reset flow + email templates + lockout logic + recovery codes. Net loss; auth surface is exactly where security regressions cost the most. Authentik in compose is the right call for v1 even though it's heavy.

##### Option D — Defer the migration split; rename inside one mega-migration  *(rejected)*

Tempting because there's no prod data yet. Costs nothing now, but introduces a "this migration is actually three migrations" cognitive tax forever. The split is cheap *only* before sqlc runs against it. After, it's expensive. Pay the cheap version. [D-18]

#### Trade-off analysis

The order milestones 0.1 → 0.2 → 0.3 is non-negotiable: each depends on the previous (migrations → sqlc → adapters → module construction). Milestones 0.4 (OIDC) and 0.5 (frontend RSC auth) are parallel-ish — OIDC ends at "can curl /me with cookies", frontend starts at "browser does what curl just did". You could swap the order, but doing OIDC first gives you a working backend before touching the frontend, which is easier to debug because every problem is in one place at a time.

Milestone 0.6 (naming + CI) is small enough to drop in any gap, but doing it before Milestone 0.4 means the drift checks catch any sqlc or openapi regressions caused by 0.1–0.3 before they compound.

Total budget for Phase 0: ~35 hours, ~Days 1–6 of the sprint. That leaves Days 7–10 for the Phase 2 vertical slice (upload → transcode → playback), Days 11–12 for bugfixes + the deploy script, Days 13–14 for buffer.
<!-- /adr-narrative -->

#### Consequences

What happened (checked 2026-10-01):

- The "wire it" panic ended on schedule. Every module since has attached to `r.Route("/api/v1", ...)` exactly as account did; `cmd/api/main.go` mounts every module on disk and the `TODO: mount` comment is gone.
- Migrations landed as planned for `0001_platform_init` … `0005_platform_audit`, then kept going: `ls backend/db/migrations | wc -l` (90 files, `0001`–`0045` at last check). Media tables shipped in `0007`, tenancy in `0018`–`0020`, and so on — the `<seq>_<module>_<desc>` naming from Milestone 0.1 held throughout.
- `repository/` directories are populated in every module: the committed `db.go` / `models.go` / `querier.go` and the hand-written `adapter.go`, plus the per-query `*.sql.go` that `make sqlc` regenerates and `.gitignore` excludes. An empty one is the signal a module is inert.
- The 7-step demo from ADR-01 ran on 2026-07-06 and is the regression baseline.
- **CI** landed later and larger than Milestone 0.6's two drift jobs, and different: `backend` (sqlc generate → migrate a throwaway Postgres → build → vet → test `-race`), `lint` (depguard module boundaries), `openapi` (parse + regenerate-and-diff, [ADR-10](#adr-10)), `frontend` (typecheck + build), `link-check` ([ADR-11](SPEC-06-docs-canonicalisation.md#adr-11)). There is **no `sqlc-drift` job and never was** — sqlc output is not committed, so there is nothing to diff; the `backend` job regenerates it and builds.
- The Authentik wildcard never had to be played: ADR-06 removed it before Day 4.
- Milestone 0.5's `frontend/src/lib/api-server.ts` (`server-only`) was never written; the frontend fetches from client components through TanStack Query (`frontend/CLAUDE.md`, [D-32]/[D-33]) and `SessionKeeper` keeps the session alive ([D-34]).
- Revisited: the Zustand/TanStack/RHF boundary doc ([D-32]) and RSC decision tree ([D-33]) landed as [`frontend/CLAUDE.md`](../../../frontend/CLAUDE.md); of the CI jobs Milestone 0.6 skipped, lint and test landed, security scan and multi-arch build did not.

#### Action items

All executed; the plan is closed. Milestone tracking ran in
`MILESTONE_CHECKS.md` (deleted in `f11cf3f` once the milestones closed — status now lives in code, see
`/CLAUDE.md` § Current status), together with the recorded milestone-check
commands. `.env.example` was populated on Day 0 (its `OIDC_*` /
`BOOTSTRAP_ADMIN_OIDC_SUBJECTS` entries were dropped with ADR-06;
`BOOTSTRAP_SUPERADMIN_EMAIL` took the bootstrap role). The Day 4 Authentik
afternoon became moot with ADR-06. The full 7-step demo ran at the end of
Milestone 0.5 (2026-07-06).

<a id="adr-10"></a>
### ADR-10 — OpenAPI Contract Direction (spec-first, enforced)

**Decided:** 2026-07-11 (drafted 2026-07-08, from the 2026-07-08 gap audit) · **Status:** accepted

Relates to backlog §9 as it stood in 2026-07 (that file was replaced on
2026-09-11; `git log --follow -- docs/product/backlog.md`) · the **API
contract** convention of the [specs README](README.md#conventions-binding-on-all-specs), which is the one statement of the rule ·
[ADR-09](SPEC-06-docs-canonicalisation.md#adr-09)'s canonical-source rule ·
[`backend/MODULES.md`](../../../backend/MODULES.md) § 8 (states the rule since
2026-09-11).

#### Context

*The state this decision was made against, as the 2026-07-08 audit found it.
Where it stands now is under Consequences and Action items.*

`shared/openapi.yaml` (OpenAPI 3.1, then ~730 lines, 12 paths) declared itself
the source of truth — its `info` block said *"Server stubs (Go) and the
TypeScript client are both generated from it,"* and CLAUDE.md repeated the rule.
On disk that claim was aspirational, not real:

- **No generated code existed.** `backend/internal/handler/api.gen.go` and
  `frontend/src/lib/types.gen.ts` were `.gitignore`d and had never been
  generated or committed — `internal/handler/` did not exist. `make openapi`
  was fully wired (oapi-codegen `chi-server` + `openapi-typescript`, config in
  `backend/oapi-codegen.yaml`) but had never been run in anger.
- **All 12 handlers were hand-written plain-chi** (6 account, 6 media) with zero
  `ServerInterface` — `grep -rn ServerInterface backend/` returned nothing. The
  hand-written `frontend/src/lib/api-client.ts` carried a TODO to switch to the
  generated types.
- **The contract already lied.** Handlers emitted the legacy `{code, message}`
  error body via a local `writeErr`, but the spec mandated RFC 7807 `Problem`.
  The auth-path drift (`/auth/register`, retired `/auth/callback`) had been fixed
  by hand; the error-shape drift persisted.
- **CI did not gate drift — of anything.** The `openapi` job only asserted the
  YAML parsed and had `openapi`/`info`/`paths` keys (a ~10-line Python check): no
  lint, no codegen, no spec-vs-handler comparison. (The audit also credited the
  `backend` job with an sqlc drift gate to mirror. It had none: sqlc output is
  `.gitignore`d and regenerated on every build, so there was nothing to diff.
  The *pattern* — regenerate, then `git diff --exit-code` — was still the right
  one; it just had no precedent in this repo.)

The forcing function: **SPEC-04/14/12 (and SPEC-05) were about to add ~30
endpoints** across media/comic/bank/notify onto a contract nothing
machine-checked. Each spec's Definition of Done said *"fix the drift in the same
or an earlier PR"* — but there was no mechanism to enforce that, so it would
rot. Two facts made timing decisive: the generated side was **greenfield**
(nothing committed to reconcile or delete), and the handler count was at its
**all-time low** (12, two wired modules). This was the cheapest the decision
would ever be; every week of deferral raised the price.

The underlying decision (backlog §9) had never actually been made: *adopt
oapi-codegen/openapi-typescript, or drop the spec as source of truth.* It is
expensive to reverse and touches every module — hence an ADR.

<!-- adr-narrative -->
#### Decision

**Keep `shared/openapi.yaml` as the single source of truth and make that real and
enforced (spec→code).** Concretely:

1. **Go: adopt oapi-codegen `chi-server` + `models`.** Handlers implement the
   generated `ServerInterface`; **bodies stay hand-written** — cookie/throttle/
   audit logic in the account handlers is unchanged, only signatures and
   request/response types come from generation. Routes register via the generated
   mux instead of ad-hoc `r.Post(...)`.
2. **TS: adopt `openapi-typescript`.** `api-client.ts` consumes `types.gen.ts`;
   the frontend gets end-to-end typed API access.
3. **Commit the generated artifacts** (remove `api.gen.go` + `types.gen.ts` from
   `.gitignore`). CI drift becomes the sqlc pattern: `make openapi` then
   `git diff --exit-code`. Committing means PRs show the contract surface changing —
   that diff *is* the contract review.
4. **Pin the toolchain.** Add oapi-codegen via a `tools.go` / `go.mod` tool
   directive (it is currently unpinned — no `tools.go`, absent from `go.mod`, so
   the drift gate would be non-reproducible without this).
5. **Replace the CI parse-check** with (a) a real lint (`redocly lint` or
   `vacuum`) and (b) the regenerate-and-diff drift gate for **both** Go and TS.
6. **Resolve the RFC 7807 drift as part of the cutover** — a module-wide
   `writeErr` → `Problem` helper, so the error contract stops lying.

Scope: do the cutover **now** on the two wired modules (account, media), before
SPEC-04. Thereafter each spec adds its paths spec-first, and an endpoint missing
from the spec fails CI — the specs' DoD becomes mechanical, not aspirational.

#### Options considered

- **A. Spec→code, enforced** *(chosen)*: makes the declared intent real, reuses
  the working sqlc drift-gate pattern, gives the frontend typed access for free,
  and closes the door on ~30 endpoints of future drift at the point of minimum cost.
- **B. Code→spec (swaggo annotations; generate the spec from handlers).** Rejected:
  inverts the source of truth the project has declared three times; imposes a
  per-handler annotation burden forever; swaggo's OpenAPI 3.1 support lags; the
  annotations drift from behavior just as easily as a hand-kept spec.
- **C. Drop the spec as source of truth (code-first, hand-written TS client).**
  Rejected: abandons a principle stated in the spec, CLAUDE.md, and MODULES.md;
  discards a ~730-line asset; lets frontend types drift silently into runtime bugs.
  Saves work now; the frontend repays it with interest. Cheapest today, most
  expensive across the four verticals.
- **D. Spec-first-lite: generate only the TS client, keep Go hand-written, add a
  custom route-set diff in CI.** Rejected: the path-presence check is bespoke
  tooling that catches *missing endpoints* but not *schema drift* (request/response
  body shape) — and body-shape is exactly where ledger/finance correctness bugs
  hide. oapi-codegen gives full-shape conformance for less long-run maintenance.

#### Trade-offs

- **Rewriting 12 handlers to the generated interface + fixing the error shape is
  real work now (~1 dev-day).** Accepted: one-time, at the all-time-low handler
  count, and it forecloses drift across the four incoming specs.
- **oapi-codegen `chi-server`'s interface is somewhat rigid.** The complex auth
  handlers keep hand-written bodies but must match generated signatures. Mitigated:
  chi-server generates routing + types, not logic — the throttle/cookie/audit code
  is untouched.
- **Committing generated code adds diff noise on contract changes.** Accepted —
  that noise is the point; it surfaces contract changes in review.
- **One more pinned tool in the build.** Accepted: mirrors sqlc; reproducibility
  is the whole reason for the gate.
- **The spec still covers only ~2 of ~7 modules.** Accepted: the specs backfill
  their own paths as they land; enforcement from now prevents the gap widening.
<!-- /adr-narrative -->

#### Consequences

What followed, checked against the tree on 2026-10-01. The working rule itself
is the **API contract** convention of the [specs README](README.md#conventions-binding-on-all-specs); this list records how it came to be
true and where it still is not.

- **The drift gate runs, and is required.** `api.gen.go` and `types.gen.ts`
  (`wc -l backend/internal/handler/api.gen.go frontend/src/lib/types.gen.ts`)
  have been committed since `6160f8e` (2026-07-12); `.gitignore` excludes sqlc
  output only; the `openapi` job runs `make openapi` then
  `git diff --exit-code`. From `edadf28` (2026-07-08) to `a30b887` (2026-09-11)
  the job never reached that step — `frontend/pnpm-lock.yaml` had been deleted
  while `setup-node` still cached on it — so for two months the gate was true on
  paper only. `a30b887` committed the lockfile; since then the job runs to its
  diff and is green on `main` (`gh run list --workflow ci.yml --branch main`),
  and branch protection on `main` requires it ("OpenAPI contract (parse +
  codegen drift gate)" — `gh api repos/jackone2991/portal/branches/main/protection`).
- **Spec-first held for every module that came after.** The spec is
  `wc -l shared/openapi.yaml` lines (5,187 at last check) and
  `python3 -c "import yaml;print(len(yaml.safe_load(open('shared/openapi.yaml'))['paths']))"`
  paths (111), tagged for every wired module — account/admin, media, movies,
  music, stories, comic, bank, journal, notifications, layout, ops, people,
  social, tenant, platform. The "~2 of ~7 modules" gap closed.
- **The error contract stopped lying — but by a different route.**
  `internal/platform/server.Problem` became the single RFC 7807 writer on
  2026-08-25; the four surviving `writeErr`/`writeError` shims (account, media,
  notify, tenant) delegate to it, and `schemas/Error` is deprecated with no
  referent (the **Errors** convention of the [specs README](README.md#conventions-binding-on-all-specs)). This landed without the
  `ServerInterface` cutover it was scoped under.
- **No handler implements the generated `ServerInterface`.** `api.gen.go` is
  its only referent in the tree (`grep -rln ServerInterface backend`). Every
  handler is still hand-written plain-chi, so the gate proves the generated
  code matches the spec, **not** that handler behaviour does (the **API
  contract** convention of the [specs README](README.md#conventions-binding-on-all-specs)).
- **The frontend is not typed end-to-end.** `types.gen.ts` is committed, but
  `grep -rln 'from "./types.gen"' frontend/src` finds one importer
  (`lib/comic-sync.ts`); `api-client.ts` still opens with "Once `make openapi`
  runs …" and the other `lib/*.ts` modules hand-declare their types. The typed
  client the decision promised is available and unused.
- **backlog §9 closed** (the either/or is resolved). The specs' DoD is
  enforceable for *path presence and schema shape*; handler conformance stays
  a review question.
- The spec's own `info`-block claim ("stubs and client are generated from it")
  is true. CLAUDE.md's "don't hand-edit generated files" protects real files.
- **`backend/MODULES.md` §8 gained its step on 2026-09-11** — two months after
  this record said it would; until then a new module that followed the
  checklist failed the `openapi` job. ADR-09's canonical-source rule holds: the
  contract stays at `shared/openapi.yaml`; `docs/reference/` points at it.

#### Action items

- [x] Accepted 2026-07-11; generated files un-ignored and committed (`6160f8e`); CI regenerate-and-diff for Go + TS (`openapi` job, `6160f8e`; reachable since `a30b887`); `Problem` error helper (`internal/platform/server`, 2026-08-25); every spec since SPEC-04 added its paths spec-first; `backend/MODULES.md` §8 and CLAUDE.md state the rule (2026-09-11).
- [ ] Pin oapi-codegen for **local** runs. CI pins `oapi-codegen@v2.7.2`
      (`.github/workflows/ci.yml`) and `openapi-typescript ^7.4.0`
      (`frontend/package.json`), but `backend/go.mod` has no `tool` directive
      and there is no `tools.go`, so `make openapi` on a developer machine uses
      whatever version is on `PATH` — and a version skew produces a diff the
      gate rejects.
- [ ] Refactor handlers onto the generated `ServerInterface` — **none done**,
      account and media included. Retrofit per module as each is touched. (The
      "cutover PR before SPEC-04" as scoped never landed; only the gate did.)
- [ ] Replace the parse-check with a real lint (`redocly lint` / `vacuum`) —
      the job still only checks that the YAML parses.
- [ ] Make the frontend consume `types.gen.ts` beyond `comic-sync.ts`, or
      strike the "typed client" claim from the spec's `info` block.

<a id="adr-16"></a>
### ADR-16 — List API contract: `{items[, next_cursor]}`, keyset cursors, a lenient `limit`

**Decided:** 2026-09-30, extended 2026-10-01 and 2026-10-02 · **Status:** accepted (Decision 2026-09-30 (Envelopes); Decision 2026-10-01 (e) and (g); Decision 2026-10-02 (A5)); partly built — `platform/server.Limit` already clamps, the retrofits are not done

Deciders: kirito. Relates to [ADR-10](#adr-10) (the contract this shapes) and
[D-29] (`PaginatedResult<T>`, whose 2026-09-30 and 2026-10-01 updates in
[feature-inventory.md](../feature-inventory.md) record the same decisions).
The rule is the **Pagination** convention of the [specs README](README.md#conventions-binding-on-all-specs), which is its one statement;
the admin exception is [SPEC-01](SPEC-01-account-identity-admin.md) P0.8, and
every spec §7 applies it. This record holds the decision and points there for
the contract.

#### Context

*The state this was decided against, 2026-09-30 to 2026-10-02 (`main` @
`99b5a0b`). Spec numbers are today's.*

- **The envelope existed only on paper.** D-29 specified
  `PaginatedResult<T>` as `{items, next_cursor}`, but the shipped lists answer
  resource-named keys — `{assets}`, `{comics}`, `{transactions}`,
  `{people}`, `{upcoming}`, `{accounts}`, the music, movie, story, social and
  admin-user lists among them.
- **The specs disagreed on envelopes** (the 2026-09-30 worklog): the
  Pagination convention grandfathered existing endpoints' limits, not their
  envelopes; SPEC-11 kept `{people}` / `{upcoming}` as a deliberate
  exception; SPEC-04, SPEC-14 and SPEC-12 called `{assets}`, `{comics}` and
  `{transactions}` a pending retrofit. Whether a non-paginated list
  (`/bank/accounts`, chapter pages, `/bank/debts`,
  `/people/upcoming-birthdays`) wore the envelope at all was unstated.
- **The specs disagreed three ways on `limit`** (the 2026-10-01 verification):
  clamp to the max (SPEC-09, SPEC-10, TC-NOTIFY-009); fall back to the
  default (SPEC-04, SPEC-07, SPEC-11); silent (SPEC-14, SPEC-12, SPEC-05). On
  `HEAD` every list service fell back to the default on both sides; only
  `/continue` clamped.
- **One list pages by offset.** `GET /admin/users` answers `limit` +
  `offset` with a `total` and per-status `counts`.

A response shape changed later breaks every client, and the rule binds every
module — criteria (a) and (b) for a record.

<!-- adr-narrative -->
#### Decision

1. **Every collection response is `{items}`.** A cursor-paginated list
   answers `{items, next_cursor}`, `next_cursor` absent or null on the last
   page; a non-paginated list answers `{items}` too (2026-10-01 (g)), so a
   list that later gains pagination or metadata changes no client. Top-level
   fields that are not the list may sit beside `items`; a resource-named list
   key may not. A single resource or a composite read keeps its arrays as
   named fields.
2. **Retrofitted, not grandfathered.** Endpoints that shipped before the rule
   move to `{items}`; only their declared limits are kept.
3. **Keyset cursors.** Lists page by an opaque base64 keyset
   `?cursor=&limit=` whose ordering key ends in `id`; a malformed cursor is
   400 `<module>/invalid-cursor`. A new endpoint that states no limits uses
   default 30, max 50.
4. **`limit` is lenient and never an error** (2026-10-01 (e)): missing,
   non-integer or < 1 gives the endpoint's default; above its max clamps to
   the max. No Problem type is ever emitted for `limit`.
5. **One named exception** (A5): the admin-only `GET /admin/users` keeps
   `limit` + `offset` with `total` and per-status `counts`. It wears the
   `{items}` envelope, and it is not a precedent: a new list is keyset-paged.

#### Options considered

Envelopes, from the 2026-09-30 worklog ("pick one policy in the README and
align the four specs"):

- **Grandfather shipped envelopes**, keeping SPEC-11's exception. Not chosen.
- **Retrofit every list to `{items}`.** *Chosen.*

Non-paginated lists (2026-10-01 (g)): the question asked the owner to confirm
the reading that `{items}` applies to them too; the alternative it implied —
the envelope for paginated lists only — was not taken.

Out-of-range `limit` (2026-10-01 (e)), as written:

- **Lenient and clamping** — the proposed rule: what a client asking for 500
  means, matching SPEC-09, SPEC-10 and TC-NOTIFY-009, at one line in each of
  seven services plus their tests. *Chosen.*
- **Fall back to the default everywhere** — the zero-code alternative, which
  would have flipped SPEC-09, SPEC-10, TC-NOTIFY-009 and the `/continue`
  handler instead.

Admin paging, the former SPEC-01 §10 Q2: **keep offset** for this one
operator list (*chosen*), or **move to a cursor and drop `total`**.

#### Trade-offs

- **Every shipped list is a retrofit.** Each one moves its handler,
  `shared/openapi.yaml` and its frontend readers in one PR; the payoff is
  that no list ever changes shape again.
- **A wrapper on lists that never page.** `{items}` costs one level of
  nesting where a bare array would do.
- **Leniency hides a client mistake.** A client asking for 500 silently gets
  the max; the stricter answer would have been a 4xx for a request whose
  meaning is clear.
- **Keyset gives no totals.** No page numbers and no "how many"; the one list
  that needs them, an approval queue at household scale, is the named
  exception.
<!-- /adr-narrative -->

#### Consequences

*Partly built: `platform/server.Limit(r, def, max)` implements Decision 4;
the rest is the target, true when the rows in Action items close.*

- **The contract** is the **Pagination** convention of the
  [specs README](README.md#conventions-binding-on-all-specs); every spec §7
  states its shape, default and max, `<module>/invalid-cursor` and
  `<module>/validation`; SPEC-01 P0.8 states the exception.
- **The shipped divergences** are the "Envelopes retrofit to `{items}`" and
  "`limit` clamps instead of resetting" cross-cutting gaps in the [specs README](README.md#implementation-gaps-index-2026-10-01), plus the
  one list no spec owns (people `{suggestions}`), which retrofits the same
  way.

#### Action items

1. [ ] Envelopes: SPEC-04 §11 row 12 · SPEC-14 §11 row 12 · SPEC-12 §12
   row 9 · SPEC-11 §11 rows 9, 11 · SPEC-13 §8 row 3 · SPEC-01 §11 row 18 ·
   SPEC-15 §12 row 15 · SPEC-16 §11 row 8 · SPEC-17 §11 row 9 · SPEC-18 §11
   row 4; and people `{suggestions}`.
2. [ ] `limit` clamps: SPEC-04 §11 row 18 · SPEC-14 §11 row 17 · SPEC-12 §12
   row 17 · SPEC-05 §11 row 22 · SPEC-07 §11 row 8 · SPEC-09 §11 row 8 ·
   SPEC-11 §11 row 15 · SPEC-01 §11 row 19 · SPEC-15 §12 row 16 · SPEC-16
   §11 row 9 · SPEC-17 §11 row 10; and people `{suggestions}`.

<a id="adr-18"></a>
### ADR-18 — Backups: a separate locked bucket, age-encrypted, cluster globals included

**Decided:** 2026-10-19 · **Status:** accepted (Decisions 2026-10-19 (C2), (C5), (C6)); not built

Deciders: kirito. Relates to [ADR-03](#adr-03) (the topology a backup has to
leave; re-affirmed the same day, C1), [ADR-04](SPEC-04-media-image-pipeline.md#adr-04)
(the media bucket, which since its 2026-10-19 amendment (C3) has no copy of its
own), [ADR-07](SPEC-01-account-identity-admin.md#adr-07) (`portal_app` and the
other roles a restore must bring back), [ADR-13](SPEC-01-account-identity-admin.md#adr-13)
(a restore resurrects deleted Users) and [ADR-14](SPEC-01-account-identity-admin.md#adr-14)
(`AUDIT_PII_KEY`, which old dumps need). The requirements it produces are P0.2
(the set, the backup bucket, retention past the lock), P0.4 (the drill) and
P0.7 (encryption and the config copy) above; this record holds the reasoning
and does not repeat their contracts.

#### Context

*The state this was decided against, 2026-10-19 (`main` @ `99b5a0b`; the docs
commits on top change no code).*

- **Dumps share the media bucket and its key.** `cmd/worker/main.go` hands
  `ops.New` the same `store` media uses (`S3_BUCKET`, `portal-media` in
  `.env.example`), so every dump sits at `backups/pg/<yyyy-mm-dd>.dump` beside
  every asset, and the one `S3_ACCESS_KEY` / `S3_SECRET_KEY` pair that uploads
  photos can read, overwrite and delete every backup.
- **Every container holds every secret.** `docker-compose.yml` gives `api`,
  `worker`, `scraper` and `frontend` the whole `.env` (`env_file: .env`), so the
  internet-facing `api` and `frontend` also hold `BACKUP_DATABASE_URL` and
  `MIGRATE_DATABASE_URL` — the superuser `portal` DSN — although only the
  worker's backup task and the host's `make migrate` use them.
- **Dumps are plaintext.** `pg_dump -Fc` compresses; nothing encrypts. P2 asked
  for encryption "before any *second* user's data arrives", and approval-gated
  registration ([SPEC-01](SPEC-01-account-identity-admin.md) P0.1) already
  admits second users.
- **A dump cannot bring the application back.** `ops/backup.go`
  `dumpToStorage` runs `pg_dump -Fc --no-owner --no-privileges` of one
  database. Roles are cluster objects, so `portal_app` and `portal_sys`
  (created by `0019`) are in no dump, and the two flags drop every owner and
  every `GRANT` — `0019`'s grants and default privileges to those two roles
  among them. A database restored from it gives the API's `portal_app`
  "permission denied" on every query; [ADR-12](SPEC-01-account-identity-admin.md#adr-12)'s
  planned `SECURITY DEFINER` functions, which only work while owned by
  `portal_acl`, would come back owned by whoever ran the restore.
- **The drill cannot run.** `backend/scripts/restore-drill.sh` restores inside
  the compose `postgres` container (`portal-postgres-1`), commented out since
  Postgres moved to the host on 2026-08-21; it reads the store with the media
  `S3_*` set at a hard-coded `http://minio:9000`, and it checks only as the
  superuser, which passes RLS and every missing grant. The 2026-09-30
  `RESTORE_S3_*` design fell back to `S3_*` when unset — to the media key. No
  passing run is recorded ([backlog.md](../backlog.md) #32).
- **Where the data is.** Production — the VPS with R2 of ADR-03 and ADR-04 —
  does not exist yet. The stack runs on the owner's Windows machine (host
  Postgres 18, MinIO bind mount, Docker Desktop), which ADR-03's 2026-10-19
  note names the development environment; a development backup bucket shares
  that machine's disk.

The owner's review of this spec on 2026-10-19 settled three of its points
together: C2 (where backups go and who can delete them), C5 (encryption) and C6
(what a backup contains). They change the compose file, the worker's
configuration, the roles every restore depends on and every operator
procedure, and they are expensive to reverse once sets exist in the new shape —
criteria (a) and (b) for a record.

<!-- adr-narrative -->
#### Decision

1. **A bucket of its own** (C2). Backups go to a separate bucket,
   `portal-backups` — an R2 bucket in production, a MinIO bucket of the same
   name in development — never the media bucket. Same `platform/storage`
   client, a second instance.
2. **Credentials only the worker holds** (C2). `BACKUP_S3_*` is a key scoped to
   that bucket, and it and `BACKUP_DATABASE_URL` reach the `worker` container
   only: compose gives each service the variables its binary reads, not the
   whole `.env`. The `api`, `scraper` and `frontend` hold neither.
3. **Immutable for N days** (C2). The bucket is locked — S3 Object Lock on
   MinIO, R2 bucket lock rules in production — for N days, so not even the
   worker's key can delete or overwrite a young backup. N is proposed at 30
   days and settled together with retention (SPEC-03 §10 Q-C4). Keys are
   therefore written once and never reused, and the application's pruning
   deletes only what the lock has released.
4. **A read-only token for restore** (C2). The drill reads the same bucket with
   a separate read-only token — never the worker's key, never `S3_*`. There is
   no fallback.
5. **Encrypted with age** (C5). Every backup object is encrypted to an age
   public key as it is written; the worker holds only that recipient. The
   private identity lives offline — a password manager and a paper copy — and
   only the drill or a real restore uses it. The manifest records the
   encryption, and its sha256 covers the ciphertext.
6. **The configuration comes too** (C5). Each night the worker also writes an
   age-encrypted copy of the deployment's `.env` beside the dump, so a restored
   database has the keys to run.
7. **Cluster globals and owners** (C6). Each backup set is
   `pg_dumpall --globals-only` — every role with its password hash, role
   memberships — plus `pg_dump -Fc` of the portal database **with** owners and
   privileges. A restore applies the globals first, then `pg_restore`.
8. **The drill restores into a throwaway Postgres** (C6) — `postgres:18`, the
   host cluster's major, the pattern CI uses — globals then dump, and it checks
   the result as `portal_app` inside a tenant scope, not only as the superuser.
   On a live cluster the role names would collide; a throwaway container has
   none of them.
9. **Keys outlive the backups that need them** (C5). Every age identity, and
   every `AUDIT_PII_KEY` once that key exists, is kept for as long as any backup
   that needs it is retained.

#### Options considered

The review's chosen options are marked; the alternatives are stated as this
record weighs them.

Where backups go and who can delete them (C2):

- **Keep them in the media bucket under the media key.** Rejected: whoever can
  upload a photo can delete every backup, and a compromised `api` holds that
  key.
- **A separate bucket with worker-only credentials** *(chosen, C2 (b))*, together
  with **immutability on it** *(chosen, C2 (c))*: the separate key narrows who
  can delete; the lock removes deletion for N days even from that key.
- A further option written for the case where the Windows machine is
  production (C2 (d), conditional on C1 (b)) — moot, because the owner chose
  C1 (a).

Encryption (C5):

- **Rely on the provider's encryption at rest.** Rejected: it protects the
  provider's disks, not a backup read with a valid key.
- **A symmetric key held by the worker.** Rejected: the machine being backed up
  would also hold what decrypts every backup.
- **age to a public recipient, the identity offline** *(chosen, C5 (b), promoted
  from P2 to P0)*: the server can write backups it cannot read.

What a set contains (C6):

- **Keep `--no-owner --no-privileges` and re-create the roles and grants by
  hand**, re-running `0019`'s block after a restore. Rejected: `0019` seeds
  placeholder passwords, covers only its own two roles and grants, and cannot
  restore a function's owner.
- **Dump with owners and privileges, re-create the roles by hand.** Rejected:
  the restore then depends on an operator reproducing every role, attribute and
  membership correctly, under pressure.
- **`pg_dumpall --globals-only` beside an owner-preserving `pg_dump`**
  *(chosen, C6 (c))*: the roles and their hashes travel with the data, and a
  restore is two mechanical steps.

#### Trade-offs

- **Lose the identity, lose every backup.** Accepted explicitly: a backup the
  server cannot read is also one nobody can read without the offline key. Two
  copies, in a password manager and on paper, are the mitigation, and every
  drill proves the key still opens the latest set.
- **The lock costs space and flexibility.** Every set is kept at least N days
  whatever the retention tiers say, and a mistaken backup cannot be removed
  early — which is the point of the lock.
- **Two stores, two lock dialects.** MinIO's Object Lock is versioned (a plain
  delete adds a marker and the locked version stays); R2's lock refuses delete
  and overwrite outright. The code relies only on what both honour: write a key
  once, delete only after the lock expires.
- **The worker reads the whole `.env`** to copy it — the api's JWT keys
  included. It is the one application container with no public route
  (`networks: [internal]`, no Traefik router).
- **A longer compose file.** Per-service variable lists must be kept in step
  with `platform/config` and the scraper; a missed variable fails at start-up
  rather than silently.
- **Globals carry the night's passwords.** Restoring them into a new cluster
  resets every role password to its value at dump time — matched by the same
  night's `.env` copy. Restoring them into a cluster that survived would revert
  rotations, so a real restore into a live cluster skips them.
- **Development backups stay on the development machine.** A MinIO bucket on
  the same disk as the database protects against a bad migration or a mistaken
  delete, not against losing the machine — one more reason C1 asks for the
  production deployment before more real data accrues.
<!-- /adr-narrative -->

#### Consequences

*Nothing below is built; the facts are the target, true when the rows in
Action items close.*

- **A set is three objects under one stem** (P0.2): globals, dump and config,
  keyed `backups/pg/<yyyy-mm-dd>/<run_id>.globals.sql.age`,
  `….dump.age` and `backups/config/<yyyy-mm-dd>/<run_id>.env.age`, the manifest
  at `backups/LATEST.json`. `ops_backup_runs.storage_key` keeps the dump's key and
  the others derive from it, so no migration is needed.
- **New configuration**, described in P0.2, P0.4 and P0.7 and the runbook
  (`.env.example` gains it with the code): on the worker,
  `BACKUP_S3_ENDPOINT`, `BACKUP_S3_REGION`, `BACKUP_S3_BUCKET`,
  `BACKUP_S3_ACCESS_KEY`, `BACKUP_S3_SECRET_KEY`, `BACKUP_S3_USE_PATH_STYLE`,
  `BACKUP_LOCK_DAYS`, `BACKUP_AGE_RECIPIENT` and `BACKUP_CONFIG_PATH`; for the
  drill, on the operator's machine and never the server's, `RESTORE_S3_ENDPOINT`,
  `RESTORE_S3_REGION`, `RESTORE_S3_BUCKET`, `RESTORE_S3_USE_PATH_STYLE`,
  `RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY`, `AGE_IDENTITY_FILE` and
  `RESTORE_PG_IMAGE`. The drill's `PG_CONTAINER`, `SCRATCH_DB` and
  `RESTORE_PGUSER` go with the stack container they addressed.
- **The runbook changes with this record**:
  [backup-restore.md](../../operations/backup-restore.md) describes the target
  storage, the key custody, the restore order and the throwaway-container
  drill, and says where `HEAD` still differs.
- **Drafting choices for the owner to confirm.** C2, C5 and C6 settled the
  shape; these details were chosen conservatively when the record was written
  and stand until the owner says otherwise: N = 30 days, revisited with Q-C4;
  `BACKUP_LOCK_DAYS` must equal the bucket's configured retention; the lock is
  in compliance mode on MinIO (no bypass, even for the MinIO root user) and two
  R2 rules, on `backups/pg/` and `backups/config/`; the manifest sits outside
  both, at `backups/LATEST.json`; each run's keys carry its id, so a same-day
  rerun never overwrites; an unset bucket, recipient or config path fails the
  run — never a fallback to `S3_*`, never plaintext, never a set without its
  config copy; the `.env` reaches the worker as a read-only mount at
  `BACKUP_CONFIG_PATH`; the throwaway container's bootstrap superuser is
  `drill_admin`, a name no dump holds; a run failing part-way leaves its
  uploaded objects unreferenced, for the operator to remove; the plaintext
  `backups/pg/*.dump` objects already in the media bucket are deleted by hand
  once the first set in the new shape has passed a drill.
- **Gap rows.** SPEC-03 §11 rows 2–3 (rewritten) and rows 12–15 (new); the same
  review appended rows 16–18; the specs README gaps index counts them.

#### Action items

1. [ ] The backup bucket, worker-only credentials, the lock, per-run keys and
   pruning past the lock — SPEC-03 §11 row 12.
2. [ ] Per-service variables in compose — SPEC-03 §11 row 13.
3. [ ] age encryption and the `.env` copy — SPEC-03 §11 row 14.
4. [ ] Cluster globals and an owner-preserving dump — SPEC-03 §11 row 15.
5. [ ] The drill: the read-only source, decryption, the throwaway
   `postgres:18`, the `portal_app` checks — SPEC-03 §11 rows 2–3; then one
   recorded pass against a real nightly set (P0.4's AC; backlog #32).
6. [ ] Create the production `portal-backups` bucket, its lock rules and the
   read-only token with the production deployment (backlog #30); the
   development bucket comes from `minio-setup` (row 12).
7. [x] The runbook: storage target, encryption and key custody, the globals and
   the restore order, the throwaway-container drill —
   [backup-restore.md](../../operations/backup-restore.md), 2026-10-19 (it
   describes the target; the rows above make it true).

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top changed no code).
The spec text above is the target; each row below is a place where the shipped
code still diverges from it, verified against `HEAD`. Rows are ordered by
severity — **Sec** (security, including the restore path), **Data** (silent
loss of backups or runs), **Func** (a requirement or AC does not hold),
**Contract** (API/OpenAPI surface), **Hyg** (hygiene — schema, and since
2026-10-19 config comments and dev tooling), **P1** (unbuilt nice-to-have). Paths are relative to the repo root. A row closes when the code
matches the spec and the SPEC-03 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded
with the named test (or, for the drill, the recorded run) as evidence. Rows
12–18 were appended on 2026-10-19 — by Decisions 2026-10-19 (C1), (C2), (C5) and
(C6) ([ADR-18](#adr-18)) and by mechanical items of the same owner review —
instead of renumbering rows other documents cite, so the severity order
restarts at row 12; rows 1–3 and 11 were rewritten in place that day.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 · Sec | P1.6 Queue console — read/write split + CSRF | Two asynqmon handlers: `ReadOnly: true` for `queues:read` holders, `ReadOnly: false` for `queues:write` holders, picked per request after `RequireAuth`; every non-GET/HEAD request needs `queues:write` (seeded to `admin` by P1.6's own migration) and `Sec-Fetch-Site: same-origin` or an `Origin` equal to the API origin, else a 403 `account/forbidden` Problem written by Portal's wrapper before asynqmon sees the request (asynqmon's `ReadOnly` answers 405 `text/plain`, so it is never the refusal a caller gets). | `backend/cmd/api/main.go` builds one `asynqmon.New(asynqmon.Options{RootPath: "/admin/queues", RedisConnOpt: asynqRedis})` (`ReadOnly` unset, so writable) and mounts it for every method behind `RequireAuth` + `RequirePermission(engine, "queues:read")`. No migration seeds `queues:write` and no same-origin check exists, so any `queues:read` holder can retry, delete, archive or pause, and a same-site sibling (`minio.`, `mail.`, `traefik.`) can drive it with the admin's cookie. The console also displays task payloads, which today carry plaintext reset URLs (SPEC-05 §11 row 1). | **migration:** `000N_ops_queues_write` seeds `queues:write` and grants it to `admin` (0012 stays at four codes). **backend:** build both handlers; a small middleware serves the writable one only to `queues:write` holders, requires `queues:write` for non-GET/HEAD (through `RequirePermission`, so the refusal is its 403 `account/forbidden` Problem), and answers the same 403 Problem to a non-GET without `Sec-Fetch-Site: same-origin` (or a matching `Origin`) — both before the request reaches asynqmon, whose read-only mode would answer 405 (`restrictToReadOnly`, v0.7.2). **test:** httptest over the mount — TC-OPS-080, TC-OPS-081, TC-OPS-082 (asserts 403 Problem, not 405), TC-OPS-083. | F087; the 2026-10-19 review (M7) |
| 2 · Sec | P0.4 step 0 Restore drill reads the backup bucket with a read-only token | `RESTORE_S3_ENDPOINT`, `RESTORE_S3_REGION`, `RESTORE_S3_BUCKET`, `RESTORE_S3_USE_PATH_STYLE`, `RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY` — a read-only token on `portal-backups` — and `AGE_IDENTITY_FILE`, from the operator's shell or the env file the script reads; **no fallback** to `S3_*` (the media bucket and key) or `BACKUP_S3_*` (the worker's write key); an unset value → `RESTORE DRILL FAILED: backup source not configured` (Decision 2026-10-19 (C2), [ADR-18](#adr-18); replaces the 2026-09-30 fallback design). | `backend/scripts/restore-drill.sh` `mc()` hard-codes `mc alias set d http://minio:9000 …` and reads only the media `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` (defaulting to `portal-media` / `portal` / `change-me`; not even `S3_ENDPOINT`) through a `minio/mc` container on the compose network, so it can only ever reach the dev MinIO's media bucket, with the media key. | **backend (script):** read the six `RESTORE_S3_*` values and `AGE_IDENTITY_FILE` — never `S3_*` or `BACKUP_S3_*` — fail when any is unset, pass the endpoint and token to `mc alias set`, keep `RESTORE_NET` so a dev MinIO endpoint (`http://minio:9000`) resolves; list them in `.env.example` as operator-side; keep [backup-restore.md](../../operations/backup-restore.md) in step. **config:** a read-only MinIO user on `portal-backups` (`minio-setup`, with row 12); the R2 read-only token with the production deployment (backlog #30). **test:** TC-OPS-021a, recorded as a drill run. | F084; Decision 2026-10-19 (C2) |
| 3 · Sec | P0.4 steps 2–3 Restore into a throwaway Postgres and check as `portal_app` | After the sha256 checks, decrypt with `AGE_IDENTITY_FILE`; start a throwaway `postgres:18` (`RESTORE_PG_IMAGE`; no volume, removed on exit, bootstrap superuser `drill_admin`); apply the globals, then `pg_restore --create` **with** owners and privileges, both exit 0 with no ignored error; checks (a) `schema_migrations.dirty = false` and `version` ≤ the repo's latest migration, (b) `count(*) FROM users` ≥ 1 and `SELECT count(*) FROM assets` succeeds, (c) the 7 system roles from `0003`, (d) after `SET ROLE portal_app` an unscoped read of a FORCE-RLS table the superuser sees rows in is refused and, scoped to the oldest organisation, every FORCE-RLS table answers `SELECT count(*)`. Any failure exits non-zero with `RESTORE DRILL FAILED: <check>` (Decision 2026-10-19 (C6), [ADR-18](#adr-18)). | `restore-drill.sh` requires `PG_CONTAINER` (default `portal-postgres-1`) — the compose `postgres` service, commented out since 2026-08-21 — so on `HEAD`'s topology it exits 2 at its precondition and has never run since (the review's M2). Inside that container it would `pg_restore --no-owner --no-privileges` into `portal_restore_check` as the superuser, never decrypt, never apply globals and never connect as `portal_app` (M3). Step 4 checks `version ≤ REPO_LATEST` and `system roles ≥ 7`, and only that `users`/`assets` returned some value; it never reads `dirty`, never enforces `users ≥ 1`, and prints `!! …` messages. Manifest, download, `pg_restore` and `mc` failures exit through `set -e` with no `RESTORE DRILL FAILED:` prefix. | **backend (script):** `age -d -i "$AGE_IDENTITY_FILE"` for globals and dump (the config copy to `/dev/null`); `docker run` the throwaway container instead of `docker exec` into a stack one; `psql -f` the globals, then `pg_restore --create --dbname postgres`; `SELECT version, dirty FROM schema_migrations`; enforce `dirty = f` and `users ≥ 1`; check (d) through `SET ROLE portal_app` and `set_config('app.current_tenant', …)`; route every failure (including a `trap … ERR`) through one helper that prints `RESTORE DRILL FAILED: <check>`; drop `PG_CONTAINER`, `SCRATCH_DB` and `RESTORE_PGUSER`. **test:** TC-OPS-020, TC-OPS-023, TC-OPS-025, TC-OPS-026, TC-OPS-027. | F179; Decision 2026-10-19 (C6); the 2026-10-19 review (M2, M3) |
| 4 · Sec | P0.2 step 3 Sanitised `error` | `error` is a single line truncated to 500 chars; the DSN and the command line never appear in it (it is returned by P0.5 and published in `ops:backup_failed`). | `backend/internal/modules/ops/backup.go` `BackupDatabase` stores `truncate(err.Error(), 1000)`, and `dumpToStorage` wraps `pg_dump`'s stderr tail (`tail(stderr.Bytes(), 400)`) into that error. The stored, returned (`/ops/status` `last_run.error`), published and audited string is therefore multi-line, up to 1,000 bytes, may carry host/role/database names, and `truncate` can split a UTF-8 rune. | **backend:** one sanitiser — first line only (newlines collapsed), capped at 500 characters on a rune boundary, never interpolating the DSN or the command line; the full stderr goes to the worker log only; use it for the row, the event and the audit metadata. **test:** unit test on the sanitiser — TC-OPS-003b. | F178 |
| 5 · Data | P0.2 Task options + abandoned-row sweep | `asynq.MaxRetry(0)`, `asynq.Timeout(2h)`, `asynq.Unique(23h)`; at start, any `running` row older than the timeout becomes `failed` (`error='abandoned'`, `finished_at=now()`). | `backend/cmd/worker/main.go` registers `scheduler.Register("0 3 * * *", asynq.NewTask(opsapi.TaskBackupDatabase, nil), asynq.Queue("default"))`, so Asynq's defaults apply (25 retries, 30-min timeout, no uniqueness): a dump of a growing database is killed at 30 min and retried. `BackupDatabase` returns an error when `FinishBackupRunOK` fails, which re-runs the whole dump under those retries. No query in `backend/internal/modules/ops/query/ops.sql` reaps a stale `running` row, so a worker crash leaves one forever. | **backend:** add the three options to the `Register` call; add a `ReapAbandonedBackupRuns` query (`UPDATE ops_backup_runs SET status = 'failed', error = 'abandoned', finished_at = now() WHERE status = 'running' AND started_at < now() - INTERVAL '2 hours'`) and call it first in `BackupDatabase`. **test:** TC-OPS-003, TC-OPS-003a. | F086 |
| 6 · Data | P0.2 step 5 Retention set | Keep = the 7 most recent dump dates ∪, for each of the 4 most recent ISO weeks (Mon–Sun, UTC) that contain an ok dump, the latest ok dump of that week ∪ the current `LATEST.json` target. | `backend/internal/modules/ops/retention.go` `keep` keeps the 7 newest dumps plus the 4 newest **Sunday** dumps, so a week whose Sunday run failed keeps no weekly anchor. `backup.go` `pruneRetention` protects only `currentKey`, which is the `LATEST.json` target only when `writeManifest` succeeded. (The input — ok rows de-duplicated by key, never a listing — is already right.) | **backend:** weekly anchor = latest dump per `Date.ISOWeek()` over the 4 most recent ISO weeks that contain one; add the manifest's actual target to the keep set. **test:** rewrite `TestKeepRetention` and add the property test the AC names (failed Sunday, year boundary, duplicate same-day keys) — TC-OPS-004. | F085 |
| 7 · Func | P0.6 Failure alert delivery | A notify consumer `notify:on_backup_failed`, subscribed in `cmd/worker/main.go`, delivers one notification per failed run to every `admin` holder, naming the run and its sanitised `error`; it needs its own notification type. | `backup.go` `emitFailed` publishes `ops:backup_failed`, but `cmd/worker/main.go` has no `publisher.Subscribe` for it, `notify/api` has no `notify:on_backup_failed` task, and no notification type exists. | **backend:** `notifyapi.TaskOnBackupFailed`; a type in `notify/types.go` + `notify/README.md`; a handler that resolves the `admin` holders through an `account/api` read port and dispatches one intent each with `dedup_key = run_id` and a relative `data.href` (SPEC-05 P0.4 contract); `publisher.Subscribe(opsapi.EventBackupFailed, notifyapi.TaskOnBackupFailed, asynq.Queue("default"))`; register the task in events.md. **test:** TC-OPS-050. | F091 |
| 8 · Func | P1.6 Console reachable from the app | The Next admin nav links to the absolute `https://api.<domain>/admin/queues`; no Next page is added. | No link to `/admin/queues` anywhere in `frontend/src`. | **frontend:** an admin-nav entry built from the API base URL, shown to `queues:read` holders. **test:** TC-OPS-080. | F184 |
| 9 · Contract | §7 OpenAPI encoding | Per the specs README AuthZ OpenAPI encoding, `/ops/status` carries `x-required-permission: ops:read`. | `shared/openapi.yaml` `getOpsStatus` declares `bearerAuth` but no `x-required-permission` (no operation in the file carries one — the gap is repo-wide). | **openapi:** add the extension. **test:** the spec-first CI gate. | found while verifying 2026-10-01 |
| 10 · Hyg | §6 Run-row state constraint | Follow-up migration `000N_ops_backup_runs_state_chk` (`running` ⇒ no `finished_at`; `ok` ⇒ `finished_at`, `size_bytes`, `storage_key`; `failed` ⇒ `finished_at`, `error`), added `NOT VALID` then validated. | `backend/db/migrations/0012_ops_backup_runs.up.sql` has only the `status IN (…)` CHECK. | **migration:** as §6 states. Row 5's reap writes `finished_at` and `error`, so it satisfies the constraint. **test:** TC-OPS-121a. | F183 |
| 11 · P1 | P1.7 Owner takeout | `POST /api/v1/me/export`, `GET /api/v1/me/export/{id}`, `GET …/download`; `ops:takeout` on a dedicated `bulk` queue and server; the `ExportProvider` contract in each module's `api/`; `ops:purge_exports`, run once per tenant through `forEachTenant` (`ops_exports` is FORCE-RLS); a tenant-scoped `ops_exports`. | Not built: no `ops_exports` table, no `/me/export` routes, no `ExportProvider` in any module's `api/`, no `bulk` server, no `ops:takeout` / `ops:purge_exports` (the takeout permission codes are already seeded by `0012`). | Build per P1.7, §6 and §7 — `ops:purge_exports` wired in `cmd/worker` through `forEachTenant`, like `media:purge_orphans`; the `bulk` server is shared with SPEC-05 §11 row 19. Scope, shape and the delete interplay are open (§10 Q-C11…Q-C13). **test:** TC-OPS-100…105, TC-OPS-103a…103c. | F033 · F088 · F089 · F095 · F180; the 2026-10-19 review (M12) |
| 12 · Sec | P0.2 Backups in their own locked bucket | Every set goes to `BACKUP_S3_BUCKET` (`portal-backups`) through a second `storage.NewS3` built from `BACKUP_S3_*` — never `S3_*`; unset bucket → run `failed` (`BACKUP_S3_BUCKET is not configured`). Keys are written once and carry the run id (`backups/pg/<yyyy-mm-dd>/<run_id>.{globals.sql,dump}.age`, `backups/config/<yyyy-mm-dd>/<run_id>.env.age`); the manifest is the unlocked `backups/LATEST.json`. The bucket is locked for `BACKUP_LOCK_DAYS` (30 proposed, §10 Q-C4) — MinIO Object Lock in dev, R2 lock rules on `backups/pg/` and `backups/config/` in production — and retention deletes a set only once its run's `finished_at` is older than that (Decision 2026-10-19 (C2), [ADR-18](#adr-18)). | `backend/cmd/worker/main.go` passes `ops.New` the media `store` (`S3_BUCKET`, `S3_ACCESS_KEY` / `S3_SECRET_KEY`); `backend/internal/modules/ops/retention.go` `dumpKeyForDate` writes `backups/pg/<yyyy-mm-dd>.dump`, one key per UTC day that a same-day rerun overwrites; `LatestManifestKey` is `backups/pg/LATEST.json`; `backup.go` `pruneRetention` deletes every non-kept key at once; `platform/config` has no `BACKUP_S3_*` or `BACKUP_LOCK_DAYS`; `docker-compose.yml` `minio-setup` creates only `S3_BUCKET`, unlocked. | **backend:** `BACKUP_S3_*` and `BACKUP_LOCK_DAYS` in `platform/config`, read by the worker only; `cmd/worker` builds the backup store for `ops.Deps.Store`; per-run keys and the derived set keys; the manifest key; `pruneRetention` skips candidates younger than the lock. **config:** `minio-setup` creates `portal-backups` with object lock (`mc mb --with-lock`) and a default compliance retention of `BACKUP_LOCK_DAYS`, a worker-only MinIO user limited to that bucket and a read-only one for the drill (row 2); the production bucket, rules and token with the production deployment (backlog #30). **test:** TC-OPS-009, TC-OPS-010; `TestKeepRetention` gains a lock-age case (TC-OPS-004). | Decision 2026-10-19 (C2) |
| 13 · Sec | P0.2 Only the worker holds backup secrets | Each compose service receives the variables its binary reads, not the whole `.env`: `BACKUP_DATABASE_URL`, `BACKUP_S3_*`, `BACKUP_LOCK_DAYS`, `BACKUP_AGE_RECIPIENT` and the read-only `.env` mount (P0.7) reach `worker` only; `MIGRATE_DATABASE_URL` reaches no container (`make migrate` runs on the host) (Decision 2026-10-19 (C2), [ADR-18](#adr-18)). | `docker-compose.yml` sets `env_file: .env` on `api`, `worker`, `scraper` and `frontend`, so all four hold `BACKUP_DATABASE_URL` and `MIGRATE_DATABASE_URL` — the superuser `portal` DSN — as well as the JWT keys, the S3 keys and every other secret, the internet-facing `api` and `frontend` included. | **config (compose):** replace the four `env_file: .env` lines with per-service `environment:` lists interpolated from `.env` (or one env file per service); mount `.env` read-only into `worker` alone; a comment naming this row. **test:** TC-OPS-011 (`docker compose config` shows each secret on its service only). | Decision 2026-10-19 (C2); the 2026-10-19 review (M5) |
| 14 · Sec | P0.7 Backups age-encrypted; nightly `.env` copy | Every object of a set is encrypted to `BACKUP_AGE_RECIPIENT` as it is produced — plaintext never touches disk or the bucket; an unset or unparsable recipient fails the run before any upload; the worker uploads `BACKUP_CONFIG_PATH` encrypted as the set's config object (unreadable → run fails); the manifest gains `globals_key`, `globals_sha256`, `config_key`, `config_sha256`, `encryption: "age"` and `recipient`, every sha256 over ciphertext (Decision 2026-10-19 (C5), [ADR-18](#adr-18)). | `backup.go` `dumpToStorage` spools `pg_dump`'s plaintext output to the temp file and uploads it as is; `Manifest` is `{storage_key, sha256, size_bytes, finished_at}`; `backend/go.mod` has no age dependency, `platform/config` no `BACKUP_AGE_RECIPIENT` or `BACKUP_CONFIG_PATH`, and nothing copies `.env`. | **backend:** age (X25519 recipient, `filippo.io/age`) between `pg_dumpall` / `pg_dump` stdout and the temp-file + sha256 tee; the config copy; the manifest fields; both variables in `platform/config`. **config (compose):** the read-only `.env` mount on `worker` (row 13). **docs:** the runbook's key custody (written 2026-10-19). **test:** TC-OPS-070, TC-OPS-071, TC-OPS-072, TC-OPS-005 (manifest), TC-OPS-027 (the drill decrypts). | Decision 2026-10-19 (C5) |
| 15 · Data | P0.2 step 2 Cluster globals and an owner-preserving dump | Each set holds `pg_dumpall --globals-only` (every role with attributes and password hash, memberships) and `pg_dump -Fc` **without** `--no-owner --no-privileges`, both through `BACKUP_DATABASE_URL` (Decision 2026-10-19 (C6), [ADR-18](#adr-18)). | `backup.go` `dumpToStorage` runs only `pg_dump -Fc --no-owner --no-privileges --dbname …`: no roles, no owners, no `GRANT`s. Restored, the database has none of `0019`'s grants, so `portal_app` is refused on every query — and on a new cluster `portal_app` and `portal_sys` do not exist at all. The drill never notices (row 3). | **backend:** drop both flags; a `pg_dumpall --globals-only` step through the same DSN before the dump, encrypted and uploaded as the set's globals object (row 14); confirm the worker image's `postgresql18-client` provides `pg_dumpall`. **test:** TC-OPS-012; TC-OPS-026 (the drill restores both). | Decision 2026-10-19 (C6) |
| 16 · Data | P0.2 step 2 A failed upload deletes nothing | A failed `Put` leaves the store as it was: S3 `PutObject` is atomic, so there is no partial object to remove, and an earlier run's objects and the manifest's target are never touched by a later run's failure. | `backup.go` `dumpToStorage` calls `Store.Delete(ctx, key)` after a failed `Put` ("no partial object left behind"). `platform/storage` `Put` is a single `PutObject`, so nothing partial exists; and because the key is the UTC date (`dumpKeyForDate`), the Delete removes the dump an earlier run stored the same day — possibly the one `LATEST.json` points at — and leaves that run's ok row naming a missing object. | **backend:** remove the Delete; row 12's per-run keys then make the collision impossible as well. **test:** TC-OPS-008 (a store that fails the second `Put` of a day: the first dump and the manifest target survive). | the 2026-10-19 review (M1) |
| 17 · Hyg | [ADR-03](#adr-03) (re-affirmed 2026-10-19) The compose header tells the truth about production | `docker-compose.yml`'s header describes what the file is — the development stack — names no production file that does not exist, and points at ADR-03/ADR-04 for the production target; the production overlay and `docs/operations/deployment.md` are backlog #30 (Decision 2026-10-19 (C1)). | The header's second line promises a production overlay file that does not exist, its object-storage paragraph describes that file and cites the retired `doc/en/architecture/` tree for R2, and its "Infrastructure only" example starts `postgres pgbouncer`, both commented out since 2026-08-21. ADR-03 action item 3's comment pointing at ADR-03 is also missing. | **config (compose):** rewrite the header — the development stack; production is ADR-03/ADR-04's target with no overlay yet (backlog #30); the storage paragraph points at ADR-04; drop `postgres pgbouncer` from the example; a one-line pointer at ADR-03 (its action item 3). When #30 writes the overlay, the header names it. **test:** none (a comment). | Decision 2026-10-19 (C1) |
| 18 · Hyg | P0.4 `cmd/opsenqueue` reaches the configured broker | The dev helper that enqueues one `ops:backup_database` for the drill connects through `ASYNQ_REDIS_URL` (`asynq.ParseRedisURI`, as `cmd/worker` does), not a literal address. | `backend/cmd/opsenqueue/main.go` builds `asynq.RedisClientOpt{Addr: "dragonfly:6379", DB: 1}` and reads no environment, so it works only inside the compose network and only while `.env` keeps `.env.example`'s `redis://dragonfly:6379/1`. | **backend:** parse `ASYNQ_REDIS_URL`, failing with its name when unset. Whether an on-demand backup replaces the helper is §10 Q-C8. **test:** none — a dev helper; checked by hand with a non-default URL. | the 2026-10-19 review (M18) |

**Already matching on `HEAD`** (verified 2026-10-01):

- `0012` creates `ops_backup_runs` (a system table, no `tenant_id`) and seeds the four permission codes (`ops:read`, `queues:read` → `admin`; `takeout:read:own`, `takeout:write:own` → `user`).
- The nightly task is registered at `0 3 * * *` on a UTC scheduler, on the light server's `default` queue.
- `pg_dump -Fc` connects through `BACKUP_DATABASE_URL` (unset → a `failed` run with that message), spools to a temp file while teeing sha256 and uploads with `Storage.Put`; the worker image carries `postgresql18-client`. (The bucket, the flags, the missing globals and encryption, and the delete after a failed upload are rows 12–16.)
- `LATEST.json` `{storage_key, sha256, size_bytes, finished_at}` is overwritten after each successful run; retention reads ok rows de-duplicated by key (never a storage listing) and never deletes the manifest.
- `ops:backup_completed` / `ops:backup_failed` are emitted and audited with `actor_kind='system'`, `target_kind='ops_backup_run'`, `target_id=<run id>`.
- `GET /ops/status` returns the full P0.5 shape with the `failed` → `stale` → `ok` precedence, behind `RequireAuth` + `ops:read`.
- The drill selects via `LATEST.json` and verifies sha256 before `pg_restore`; `make restore-drill` exists. (Where it restores, and that it cannot run on `HEAD`'s topology, is row 3.)
- The console is mounted at `/admin/queues` on the API origin with `RootPath` equal to the mount path.

**Test evidence to add or fix:**

- `backend/internal/modules/ops/retention_test.go: TestKeepRetention` pins the superseded Sunday-anchored rule (its fixtures assert "4 Sundays"); rewrite it for ISO-week anchors and add the property test (TC-OPS-004).
- New: TC-OPS-003 / TC-OPS-003a / TC-OPS-003c (task options, abandoned-row reap, the database down at start), TC-OPS-003b (error sanitiser), TC-OPS-005 (manifest fields and sha256 over the stored ciphertext), TC-OPS-008 (a failed upload deletes nothing), TC-OPS-009…012 (backup bucket, lock and pruning past it, per-service secrets, globals and owners), TC-OPS-050 (admin alert), TC-OPS-070…072 (encryption, config copy, key custody), TC-OPS-080…083 (console split and CSRF; TC-OPS-082 asserts the wrapper's 403, not asynqmon's 405), TC-OPS-121a (state constraint).
- The drill is a script, not a `_test.go`: TC-OPS-020, TC-OPS-021a, TC-OPS-023 and TC-OPS-025…027 close with a recorded run (date, set key, output) linked from the matrix row, including one run against a real nightly set (the P0.4 AC).
- `backend/internal/modules/ops/state_test.go: TestComputeState, TestHoursSince` stay valid as they are.
