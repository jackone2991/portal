# SPEC-03 — Platform Ops: Backup/Restore, Queue Console, Takeout

**Status:** current, rev 2 (ADR-03 and ADR-05 folded in) · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `ops` (owns the `ops:*` prefix) · **Depends on:** nothing hard on data — rides (or, if it lands first, introduces) SPEC-04 P0.3's shared periodic-scheduler convention; failure alerts: notify consumer `notify:on_backup_failed` (owned here, P0.6) now that SPEC-05 is live; staleness remains pull-only via P0.5 until a freshness-check task is added (P2)
**Upstream:** brief 09 (folded into this spec, then deleted — `git show ea100d8:docs/product/briefs/09-platform-ops.md`) · **Refs:** [ADR-01](README.md#adr-01) (observability stays deferred), [ADR-03](#adr-03) and [ADR-05](#adr-05) (folded in below, § Decision records), [ADR-04](SPEC-04-media-image-pipeline.md#adr-04), 2026-07 backlog §7 (archived; `git show 8d382d2^:docs/product/backlog.md`), feature-inventory `D-25` (audit types)
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
is why the build order places 09 P0 as a hard gate immediately before SPEC-12
(SPEC-10 is the burst-filler).

## 2. Goals

1. Nightly automated Postgres backups to off-VPS storage (R2 prod / MinIO dev),
   with retention.
2. A **tested** restore procedure — a backup that has never been restored is a
   hope, not a property.
3. Backup failure or staleness is impossible to miss (event + queryable status).
4. The solo operator inspects/retries/purges Asynq jobs from the browser, no SSH.
   *(P1)*
5. The owner exports their data per module into one archive in open formats. *(P1)*

## 3. Non-goals

- **The 5-service observability stack** (Loki/Prometheus/Tempo/Grafana) — stays
  deferred per ADR-01; the queue console is in-process, $0.
- **Media-file backup via pg_dump** — object storage has its own durability story
  (P0.3); never stream buckets through the database job.
- **Cross-region DR, standby replicas** — single-VPS envelope (ADR-03); off-site
  *copies*, not HA.
- **GDPR-grade takeout formats** — this is the owner's own data leaving politely.
- Backing up `.env`/Traefik config automatically — runbook guidance only (P2).

## 4. User stories

- As the owner, I run `make restore-drill` on a fresh dev stack with
  `RESTORE_S3_*` pointing at the prod bucket (P0.4 step 0), and my prod ledger
  from last night is queryable — before I ever need it in anger. *(primary)*
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
`query/`, `repository/`, own `sqlc.yaml` block), migration `000N_ops_backup_runs`
(next free number), wired into `cmd/api` (status endpoint) and `cmd/worker`
(backup task), depguard block added. `ops` tables are **system-scoped** (no
`user_id`) except P1.7's exports.

Migration `000N_ops_backup_runs` seeds all four permission rows up front —
`ops:read`, `queues:read` → `admin`; `takeout:read:own`, `takeout:write:own` →
`user` (unseeded codes 403; seeding early is harmless). The `ops_exports` table
may still ship in a later P1 migration. `queues:write` is added by P1.6's own
migration (0012 is consumed and stays at four codes).

The ops surfaces require `admin` (or superadmin via `*`). A `creator`-provisioned
owner gets 403 by design; the runbook's first step verifies the owner holds
`admin`.

### P0.2 — Nightly `ops:backup_database`

On the shared periodic runner (SPEC-04 P0.3's convention — no OS cron),
nightly at 03:00 UTC (`0 3 * * *`, a cron spec, not `@every`), enqueued on the
`default` queue served by the worker's light server (matches
[events.md](../../reference/events.md)). If dump duration makes that queue's
weight-1 slot contended, move the task to a dedicated `ops` queue in the same PR
that registers it.

If this lands before SPEC-04 P0.3, ops introduces the shared periodic
scheduler itself — what is borrowed is the convention (Asynq periodic, never
OS cron), not code.

**Task options:** `asynq.MaxRetry(0)` (a failed night is recorded once and
retried by the next night), `asynq.Timeout(2h)`, `asynq.Unique(23h)`. At start,
the task marks any `running` row older than the timeout as `failed`
(`error='abandoned'`, `finished_at=now()`), so a worker crash never leaves a row
in `running` forever.

1. Insert an `ops_backup_runs` row (`running`).
2. `pg_dump -Fc` spooled to a worker temp file (sha256 teed in the same pass),
   then uploaded via `platform/storage.Storage.Put` to
   `backups/pg/<yyyy-mm-dd>.dump` (R2 prod / MinIO dev — **same code path both**,
   bucket differs; locked from the brief: dev exercising the machinery is the
   point of the drill). `Put` needs a seekable body so the SDK can sign the
   payload; a straight stdout → `Put` pipe fails (§10). **Dump connects directly
   to Postgres, not through PgBouncer** — pg_dump needs session semantics a
   transaction pooler breaks. The DSN is `BACKUP_DATABASE_URL`
   (`platform/config` → `cfg.BackupDatabaseURL`; documented in `.env.example`).
   It connects directly to Postgres, never through a pooler, as the schema-owner
   role `portal`, which bypasses RLS so every tenant's rows are dumped.
   `DATABASE_URL` (`portal_app`, NOBYPASSRLS) must never be reused: pg_dump under
   RLS either errors or silently omits tenants. If the setting is unset, the run
   fails with `error="BACKUP_DATABASE_URL is not configured"`, so P0.5 reads
   `failed`.
3. Finish the row (`ok` + `size_bytes` + `storage_key`, or `failed` + `error`).
   `error` is a single line truncated to 500 chars; the DSN and the command line
   are never interpolated into it (it is returned by P0.5 and published in
   `ops:backup_failed`).
4. On success, overwrite `backups/pg/LATEST.json` `{storage_key, sha256,
   size_bytes, finished_at}` — sha256 computed while spooling. Retention
   (step 5) must never delete it or its target.
5. **Retention** after a successful run. Input = `ops_backup_runs` rows with
   `status='ok'` and non-null `storage_key`, de-duplicated by key (never a
   storage listing). All dates are UTC (operator-facing retention dates, not a
   user-facing day boundary, so the specs README Timezone rule does not apply): a dump's date is the `<yyyy-mm-dd>` in
   its key, which is `started_at` in UTC. Keep = the 7 most recent dump dates ∪,
   for each of the 4 most recent ISO weeks (Mon–Sun, UTC) that contain an ok
   dump, the latest ok dump in that week ∪ the current `LATEST.json` target.
   Delete every other key from storage. `Storage.Delete` of a missing key is a
   no-op, so rows are never modified and re-running prune is idempotent.
   `backups/pg/LATEST.json` is never a candidate. Objects with no ok row are left
   alone: after a disaster restore the ledger is older than the bucket, and
   sweeping would delete the newest dumps.
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
run failed keeps no weekly anchor.)*

The worker image carries the Postgres client whose major version equals the
server's (currently `postgresql18-client`; locked from the brief: smallest
moving-part count). An older pg_dump refuses to dump a newer server. Bump the
client in the same change that moves the server major.

**Acceptance criteria.**
- Given a nightly run, then a dated dump exists off-VPS and the run row records
  size + duration.
- Given a failed run (e.g. DB unreachable), then `ops:backup_failed` is emitted,
  the row says why, and the next night runs normally (no wedged state).
- Given a DB outage, then exactly one `ops:backup_failed` is emitted for that
  night (no Asynq retries).
- Given a `running` row older than 2 h, then the next run marks it `failed`
  (`error='abandoned'`).
- Given 12 daily dumps accumulated, then pruning leaves exactly the retention set
  (property test on the keep/prune function against synthetic date lists,
  covering weeks whose Sunday run failed, inputs spanning a year boundary, and
  duplicate same-day keys).
- Given a successful run, then `LATEST.json` points at the new dump and its
  sha256 matches the stored object.

### P0.3 — Media durability posture (decided here, documented in the runbook)

Prod media lives on R2 (11-nines-class durability) — **the database is the
fragile asset**, and it holds every pointer into storage. The dev MinIO
bind-mount is explicitly disposable. The runbook states this so nobody "fixes"
backup coverage by dumping buckets nightly (non-goal). Object-storage lifecycle
(versioning/soft-delete on the R2 bucket) is noted as operator guidance, not
code.

### P0.4 — Restore drill

`make restore-drill`: runs **on a fresh dev stack, where no `ops_backup_runs`
table exists to consult** — dump selection cannot depend on the database.
*(2026-07-10 — the original "latest `ok` dump" + "migrations table matches the
dump's expected head" checks were unimplementable: nothing recorded an
expected head, and comparing the restored migrations table against itself is
a tautology.)* Mechanics:

0. **Source**: the drill reads from an explicit backup source
   (`RESTORE_S3_ENDPOINT`, `RESTORE_S3_BUCKET`, `RESTORE_S3_ACCESS_KEY`,
   `RESTORE_S3_SECRET_KEY`), each falling back to the stack's `S3_*` only when
   unset. The stack's single `S3_*` set points at the disposable dev MinIO, so
   without this the drill could never reach last night's prod dump. The prod
   drill uses a read-only R2 token scoped to `backups/pg/*`.
1. **Selection via manifest**: after every successful run, P0.2's task
   overwrites a small `backups/pg/LATEST.json` `{storage_key, sha256,
   size_bytes, finished_at}`. The drill reads the manifest — never
   latest-by-key-listing, which a failed partial upload could poison.
2. Download, verify `sha256`, `pg_restore` into a scratch database
   (`portal_restore_check` or a throwaway container); the restore must exit 0.
3. **Sanity checks** (each meaningful without knowing the dump's history):
   (a) `schema_migrations.dirty = false` and `version` ≤ the checked-out repo's
   latest migration; (b) `count(*) FROM users` ≥ 1 and
   `SELECT count(*) FROM assets` succeeds (the media table's real name — not
   `media_assets`); (c) the 7 system roles seeded by `0003` are present. Any
   failure → non-zero exit with `RESTORE DRILL FAILED: <check>`. *(Code
   follow-up: the shipped `restore-drill.sh` checks neither `dirty` nor
   `users ≥ 1`, and does not read `RESTORE_S3_*` (step 0) — it always talks to
   `http://minio:9000`.)*

Documented as a runbook in `docs/operations/backup-restore.md` (this spec's PR
ships the doc). Quarterly execution is a calendar practice, not automation.

**Acceptance criteria.**
- The drill passes against a **real nightly dump** before this spec closes —
  landing the code without one exercised restore does not count as done.
- The runbook is followable start-to-finish by someone who didn't write it (the
  honest test of a runbook).
- Given `RESTORE_S3_*` pointing at prod R2, the drill restores last night's prod
  dump without changing the stack's `S3_*`.

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
  SameSite=Lax does not separate same-site siblings such as `minio.`, `mail.`
  and `traefik.`. Dead-lettered jobs are inspectable and retryable from the
  browser. Configure the handler's base path; verify its inline assets survive
  the current CSP/proxy setup.

  **Acceptance criteria.**
  - An admin retries a dead-lettered task and it re-enters `pending`.
  - A caller with only `queues:read` POSTing a retry → 403.
  - A cross-origin POST with the admin cookie → 403.
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
  events.md) deletes from storage (a) archives older than 7 days, setting the
  row's status to `expired`, and (b) immediately, any row whose
  `user_id IS NULL` (owner deleted, §6), deleting its `storage_key` object and
  then the row. The purge stays row-driven, no storage listing is needed, and a
  deleted user's EXIF/GPS originals never outlive one janitor cycle. `GET` on an
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

- **Backup encryption at rest** (age/GPG) — required before any *second* user's
  data arrives; key handling is the hard part, design then.
- **`ops:job_dead` event** on dead-letter archive → notify ("your transcode
  failed") once SPEC-05 is live.
- **Config/secret backup guidance** (`.env`, Traefik, MinIO creds) in the
  runbook — documented manual step, not automated.
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

## 6. Data model — migration `000N_ops_backup_runs`

**Tenancy** (specs README convention, ADR-07). `ops_backup_runs` is a system table, exempt (no `tenant_id`). `ops_exports` (P1.7, unbuilt) holds per-owner rows and is tenant-scoped when built: `tenant_id` + index + FORCE RLS + `tenant_isolation` policy in its creating migration.

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
P0 ≈ 3.5–4 dev-days (brief's estimate holds); P1 adds ~5.

## 10. Open questions

- **(resolved)** `pg_dump` location: the Postgres client in the worker image,
  major version equal to the server's (currently `postgresql18-client`; P0.2) —
  not a separate backup sidecar: fewest moving parts.
- **(resolved)** Dev backups: same code path as prod, MinIO bucket (P0.2).
- **(resolved)** Sentinel placement: separate authenticated endpoint, not
  `/healthz` (P0.5).
- **(resolved, corrected 2026-09-30)** Streaming vs temp-file for `pg_dump`
  output: **spool to a temp file.** `platform/storage.Storage.Put` needs a
  seekable body so the SDK can sign the payload. Run `pg_dump -Fc` with
  `cmd.Stdout = io.MultiWriter(tmp, sha256)`, check exit 0, `Seek(0)`, then
  `Put(tmp)`, and remove the file on exit. Worker scratch disk must hold one
  dump. Above ~5 GiB (the single-PutObject cap), add a multipart Put (P2).
- **(product, non-blocking)** Takeout formats are proposed per module in P1.7 —
  lock each at that module's provider implementation.

## Decision records

The platform decisions this spec operates inside. They were standalone ADRs
under `docs/adr/` until the 2026-10-01 fold; the `ADR-NN` IDs are unchanged and
each record keeps a fixed anchor. Decision, Options considered and Trade-offs
are verbatim (ADR-11 rule 2 — [SPEC-06](SPEC-06-docs-canonicalisation.md#adr-11));
the fact layer was re-checked against the repo on 2026-10-01.

<a id="adr-03"></a>
### ADR-03 — Single-VPS topology and compose-profile envelope for v1

**Decided:** 2026-05-24 · **Status:** accepted

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

#### Consequences

**What became easier:**

- Deploying is `make up` (`docker compose up -d`, no profile flags — there are no profiles to forget). There is no deploy script beyond the Makefile.
- Cost ceiling is predictable: $30/mo VPS + ~$5/mo R2 + Cloudflare free tier = ~$35/mo, well under budget.
- Mailpit in the stack from day one means every mail path (password reset, notification email) is testable end-to-end in dev.

**What became harder:**

- No observability — when the demo breaks at the customer's site, the only diagnostics are container logs (`docker compose logs api worker`). Still true on 2026-10-01.
- Heavy-queue concurrency of 1 means a slow source video blocks the transcode queue. Acceptable for v1 (single demo user); becomes a real bottleneck under multi-tenant usage. The per-tenant quota wiring [D-13] has not landed.
- Postgres on the host means `make up` does not give you a database; the host cluster must be running and reachable at `host.docker.internal:5432`, and tuning lives with the host, not in compose.

**What we'll need to revisit:**

- When live streaming lands, mediamtx + concurrent transcodes will push the VPS over 16 GB. Plan the CCX33 upgrade (or split to a media-dedicated VPS) ahead of that sprint.
- The backup strategy [D-10] shipped as the `ops` module — this spec (§5 P0.2 `ops:backup_database` + retention, P0.4 restore drill, runbook [backup-restore.md](../../operations/backup-restore.md)). Dragonfly snapshots are not part of it.
- Observability was to land with tenancy so per-tenant latency is measurable from day one [D-8]. Tenancy landed ([ADR-07](SPEC-01-account-identity-admin.md#adr-07)); observability did not.

#### Action items

1. [ ] Cap Dragonfly memory before it competes with FFmpeg. Shipped command is `["--logtostderr", "--default_lua_flags=allow-undeclared-keys"]` (Asynq needs the Lua flag); `--maxmemory` is still absent.
2. [x] ~~Add `authentik-server`, `authentik-worker`, and `mailpit` services.~~ Obsolete per ADR-06: Authentik dropped. `mailpit` shipped on its own merits.
3. [ ] Document the out-of-scope services in `docker-compose.yml` with a one-line comment pointing at this record (`docs/product/specs/SPEC-03-platform-ops.md#adr-03`). Not done; the file has no such comment.
4. [ ] `make deploy-v1` — not done, and moot: with no `profiles:` in the file, plain `make up` cannot bring up anything it shouldn't.
5. [x] Postgres tuning values are recorded once, in §5 P2 "Postgres connection budget" above (the `operations/postgres-tuning.md` stub that first held them was folded in there and deleted). They now apply to the host cluster; there is no PgBouncer.
6. [ ] `docs/operations/deployment.md` — still absent. The VPS sizing rationale lives only in this record.
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

[ADR-01](README.md#adr-01)'s v1 cut kept 8 Phase 0 items. This record put them in execution order.

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
- **CI** landed later and larger than Milestone 0.6's two drift jobs, and different: `backend` (sqlc generate → migrate a throwaway Postgres → build → vet → test `-race`), `lint` (depguard module boundaries), `openapi` (parse + regenerate-and-diff, [ADR-10](README.md#adr-10)), `frontend` (typecheck + build), `link-check` ([ADR-11](SPEC-06-docs-canonicalisation.md#adr-11)). There is **no `sqlc-drift` job and never was** — sqlc output is not committed, so there is nothing to diff; the `backend` job regenerates it and builds.
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

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top changed no code).
The spec text above is the target; each row below is a place where the shipped
code still diverges from it, verified against `HEAD`. Rows are ordered by
severity — **Sec** (security, including the restore path), **Data** (silent
loss of backups or runs), **Func** (a requirement or AC does not hold),
**Contract** (API/OpenAPI surface), **Hyg** (schema hygiene), **P1** (unbuilt
nice-to-have). Paths are relative to the repo root. A row closes when the code
matches the spec and the SPEC-03 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded
with the named test (or, for the drill, the recorded run) as evidence.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 · Sec | P1.6 Queue console — read/write split + CSRF | Two asynqmon handlers: `ReadOnly: true` for `queues:read` holders, `ReadOnly: false` for `queues:write` holders, picked per request after `RequireAuth`; every non-GET/HEAD request needs `queues:write` (seeded to `admin` by P1.6's own migration) and `Sec-Fetch-Site: same-origin` or an `Origin` equal to the API origin, else 403. | `backend/cmd/api/main.go` builds one `asynqmon.New(asynqmon.Options{RootPath: "/admin/queues", RedisConnOpt: asynqRedis})` (`ReadOnly` unset, so writable) and mounts it for every method behind `RequireAuth` + `RequirePermission(engine, "queues:read")`. No migration seeds `queues:write` and no same-origin check exists, so any `queues:read` holder can retry, delete, archive or pause, and a same-site sibling (`minio.`, `mail.`, `traefik.`) can drive it with the admin's cookie. The console also displays task payloads, which today carry plaintext reset URLs (SPEC-05 §11 row 1). | **migration:** `000N_ops_queues_write` seeds `queues:write` and grants it to `admin` (0012 stays at four codes). **backend:** build both handlers; a small middleware serves the writable one only to `queues:write` holders, requires `queues:write` for non-GET/HEAD, and answers 403 to a non-GET without `Sec-Fetch-Site: same-origin` (or a matching `Origin`). **test:** httptest over the mount — TC-OPS-080, TC-OPS-081, TC-OPS-082, TC-OPS-083. | F087 |
| 2 · Sec | P0.4 step 0 Restore drill reads an explicit backup source | `RESTORE_S3_ENDPOINT`, `RESTORE_S3_BUCKET`, `RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY`, each falling back to the stack's `S3_*` only when unset, so a dev stack can restore last night's prod dump. | `backend/scripts/restore-drill.sh` `mc()` hard-codes `mc alias set d http://minio:9000 …` and reads only `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` (not even `S3_ENDPOINT`) through a `minio/mc` container on the compose network, so it can only ever reach the disposable dev MinIO. | **backend (script):** resolve the four `RESTORE_S3_*` values with the `S3_*` fallback (including `S3_ENDPOINT`) and pass the endpoint to `mc alias set`; document them in `.env.example` and keep [backup-restore.md](../../operations/backup-restore.md) in step. **test:** TC-OPS-021a, recorded as a drill run against prod R2. | F084 |
| 3 · Sec | P0.4 step 3 Drill sanity checks | (a) `schema_migrations.dirty = false` and `version` ≤ the repo's latest migration; (b) `count(*) FROM users` ≥ 1 and `SELECT count(*) FROM assets` succeeds; (c) the 7 system roles from `0003` are present. Any failure exits non-zero with `RESTORE DRILL FAILED: <check>`. | `restore-drill.sh` step 4 checks `version ≤ REPO_LATEST` and `system roles ≥ 7`, and only that `users`/`assets` returned some value; it never reads `dirty`, never enforces `users ≥ 1`, and prints `!! …` messages. Manifest, download, `pg_restore` and `mc` failures exit through `set -e` with no `RESTORE DRILL FAILED:` prefix. | **backend (script):** `SELECT version, dirty FROM schema_migrations`; enforce `dirty = f` and `users ≥ 1`; route every failure (including a `trap … ERR`) through one helper that prints `RESTORE DRILL FAILED: <check>`. **test:** TC-OPS-023, TC-OPS-020. | F179 |
| 4 · Sec | P0.2 step 3 Sanitised `error` | `error` is a single line truncated to 500 chars; the DSN and the command line never appear in it (it is returned by P0.5 and published in `ops:backup_failed`). | `backend/internal/modules/ops/backup.go` `BackupDatabase` stores `truncate(err.Error(), 1000)`, and `dumpToStorage` wraps `pg_dump`'s stderr tail (`tail(stderr.Bytes(), 400)`) into that error. The stored, returned (`/ops/status` `last_run.error`), published and audited string is therefore multi-line, up to 1,000 bytes, may carry host/role/database names, and `truncate` can split a UTF-8 rune. | **backend:** one sanitiser — first line only (newlines collapsed), capped at 500 characters on a rune boundary, never interpolating the DSN or the command line; the full stderr goes to the worker log only; use it for the row, the event and the audit metadata. **test:** unit test on the sanitiser — TC-OPS-003b. | F178 |
| 5 · Data | P0.2 Task options + abandoned-row sweep | `asynq.MaxRetry(0)`, `asynq.Timeout(2h)`, `asynq.Unique(23h)`; at start, any `running` row older than the timeout becomes `failed` (`error='abandoned'`, `finished_at=now()`). | `backend/cmd/worker/main.go` registers `scheduler.Register("0 3 * * *", asynq.NewTask(opsapi.TaskBackupDatabase, nil), asynq.Queue("default"))`, so Asynq's defaults apply (25 retries, 30-min timeout, no uniqueness): a dump of a growing database is killed at 30 min and retried. `BackupDatabase` returns an error when `FinishBackupRunOK` fails, which re-runs the whole dump under those retries. No query in `backend/internal/modules/ops/query/ops.sql` reaps a stale `running` row, so a worker crash leaves one forever. | **backend:** add the three options to the `Register` call; add a `ReapAbandonedBackupRuns` query (`UPDATE ops_backup_runs SET status = 'failed', error = 'abandoned', finished_at = now() WHERE status = 'running' AND started_at < now() - INTERVAL '2 hours'`) and call it first in `BackupDatabase`. **test:** TC-OPS-003, TC-OPS-003a. | F086 |
| 6 · Data | P0.2 step 5 Retention set | Keep = the 7 most recent dump dates ∪, for each of the 4 most recent ISO weeks (Mon–Sun, UTC) that contain an ok dump, the latest ok dump of that week ∪ the current `LATEST.json` target. | `backend/internal/modules/ops/retention.go` `keep` keeps the 7 newest dumps plus the 4 newest **Sunday** dumps, so a week whose Sunday run failed keeps no weekly anchor. `backup.go` `pruneRetention` protects only `currentKey`, which is the `LATEST.json` target only when `writeManifest` succeeded. (The input — ok rows de-duplicated by key, never a listing — is already right.) | **backend:** weekly anchor = latest dump per `Date.ISOWeek()` over the 4 most recent ISO weeks that contain one; add the manifest's actual target to the keep set. **test:** rewrite `TestKeepRetention` and add the property test the AC names (failed Sunday, year boundary, duplicate same-day keys) — TC-OPS-004. | F085 |
| 7 · Func | P0.6 Failure alert delivery | A notify consumer `notify:on_backup_failed`, subscribed in `cmd/worker/main.go`, delivers one notification per failed run to every `admin` holder, naming the run and its sanitised `error`; it needs its own notification type. | `backup.go` `emitFailed` publishes `ops:backup_failed`, but `cmd/worker/main.go` has no `publisher.Subscribe` for it, `notify/api` has no `notify:on_backup_failed` task, and no notification type exists. | **backend:** `notifyapi.TaskOnBackupFailed`; a type in `notify/types.go` + `notify/README.md`; a handler that resolves the `admin` holders through an `account/api` read port and dispatches one intent each with `dedup_key = run_id` and a relative `data.href` (SPEC-05 P0.4 contract); `publisher.Subscribe(opsapi.EventBackupFailed, notifyapi.TaskOnBackupFailed, asynq.Queue("default"))`; register the task in events.md. **test:** TC-OPS-050. | F091 |
| 8 · Func | P1.6 Console reachable from the app | The Next admin nav links to the absolute `https://api.<domain>/admin/queues`; no Next page is added. | No link to `/admin/queues` anywhere in `frontend/src`. | **frontend:** an admin-nav entry built from the API base URL, shown to `queues:read` holders. **test:** TC-OPS-080. | F184 |
| 9 · Contract | §7 OpenAPI encoding | Per the specs README AuthZ OpenAPI encoding, `/ops/status` carries `x-required-permission: ops:read`. | `shared/openapi.yaml` `getOpsStatus` declares `bearerAuth` but no `x-required-permission` (no operation in the file carries one — the gap is repo-wide). | **openapi:** add the extension. **test:** the spec-first CI gate. | found while verifying 2026-10-01 |
| 10 · Hyg | §6 Run-row state constraint | Follow-up migration `000N_ops_backup_runs_state_chk` (`running` ⇒ no `finished_at`; `ok` ⇒ `finished_at`, `size_bytes`, `storage_key`; `failed` ⇒ `finished_at`, `error`), added `NOT VALID` then validated. | `backend/db/migrations/0012_ops_backup_runs.up.sql` has only the `status IN (…)` CHECK. | **migration:** as §6 states. Row 5's reap writes `finished_at` and `error`, so it satisfies the constraint. **test:** TC-OPS-121a. | F183 |
| 11 · P1 | P1.7 Owner takeout | `POST /api/v1/me/export`, `GET /api/v1/me/export/{id}`, `GET …/download`; `ops:takeout` on a dedicated `bulk` queue and server; the `ExportProvider` contract in each module's `api/`; `ops:purge_exports`; a tenant-scoped `ops_exports`. | Not built: no `ops_exports` table, no `/me/export` routes, no `ExportProvider` in any module's `api/`, no `bulk` server, no `ops:takeout` / `ops:purge_exports` (the takeout permission codes are already seeded by `0012`). | Build per P1.7, §6 and §7; the `bulk` server is shared with SPEC-05 §11 row 19. **test:** TC-OPS-100…105, TC-OPS-103a…103c. | F033 · F088 · F089 · F095 · F180 |

**Already matching on `HEAD`** (verified 2026-10-01):

- `0012` creates `ops_backup_runs` (a system table, no `tenant_id`) and seeds the four permission codes (`ops:read`, `queues:read` → `admin`; `takeout:read:own`, `takeout:write:own` → `user`).
- The nightly task is registered at `0 3 * * *` on a UTC scheduler, on the light server's `default` queue.
- `pg_dump -Fc` connects through `BACKUP_DATABASE_URL` (unset → a `failed` run with that message), spools to a temp file while teeing sha256, uploads with `Storage.Put`, and removes a partial object on upload failure; the worker image carries `postgresql18-client`.
- `LATEST.json` `{storage_key, sha256, size_bytes, finished_at}` is overwritten after each successful run; retention reads ok rows de-duplicated by key (never a storage listing) and never deletes the manifest.
- `ops:backup_completed` / `ops:backup_failed` are emitted and audited with `actor_kind='system'`, `target_kind='ops_backup_run'`, `target_id=<run id>`.
- `GET /ops/status` returns the full P0.5 shape with the `failed` → `stale` → `ok` precedence, behind `RequireAuth` + `ops:read`.
- The drill selects via `LATEST.json`, verifies sha256 before `pg_restore` into a throwaway database, and checks the migration version and the 7 system roles; `make restore-drill` exists.
- The console is mounted at `/admin/queues` on the API origin with `RootPath` equal to the mount path.

**Test evidence to add or fix:**

- `backend/internal/modules/ops/retention_test.go: TestKeepRetention` pins the superseded Sunday-anchored rule (its fixtures assert "4 Sundays"); rewrite it for ISO-week anchors and add the property test (TC-OPS-004).
- New: TC-OPS-003 / TC-OPS-003a (task options, abandoned-row reap), TC-OPS-003b (error sanitiser), TC-OPS-005 (manifest sha256 matches the stored object), TC-OPS-050 (admin alert), TC-OPS-080…083 (console split and CSRF), TC-OPS-121a (state constraint).
- The drill is a script, not a `_test.go`: TC-OPS-020, TC-OPS-021a and TC-OPS-023 close with a recorded run (date, dump key, output) linked from the matrix row, including one run against a real nightly prod dump (the P0.4 AC).
- `backend/internal/modules/ops/state_test.go: TestComputeState, TestHoursSince` stay valid as they are.
