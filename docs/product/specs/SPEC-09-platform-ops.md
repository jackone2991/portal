# SPEC-09 — Platform Ops: Backup/Restore, Queue Console, Takeout

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `ops` (owns the `ops:*` prefix) · **Depends on:** nothing hard on data — rides (or, if it lands first, introduces) SPEC-01 P0.3's shared periodic-scheduler convention; failure alerts: notify consumer `notify:on_backup_failed` (owned here, P0.6) now that SPEC-04 is live; staleness remains pull-only via P0.5 until a freshness-check task is added (P2)
**Upstream:** [briefs/09-platform-ops.md](../briefs/09-platform-ops.md) · **Refs:** [ADR-01](../../adr/01-v1-scope-cut.md) (observability stays deferred), [ADR-03](../../adr/03-single-vps-topology.md), [ADR-04](../../adr/04-storage-tier-budget.md), 2026-07 backlog §7 (archived; `git show 8d382d2^:docs/product/backlog.md`), feature-inventory `D-25` (audit types)
**Downstream consumers:** every module holding irreplaceable data (bank, journal, media, people); notify consumes `ops:backup_failed` through `notify:on_backup_failed` (P0.6, unbuilt); `ops:export_ready` is a *future* notify consumer (needs its own `notify:on_*` task + type row first)

---

## 1. Problem statement

A self-hosted life OS asks the owner to move financial history and memories onto
hardware they operate. That deal is only honest with three properties the stack
currently lacks: **backups that run themselves**, **a restore path that has
actually been exercised**, and **visibility into the async machinery** (today a
failed transcode is discoverable only by tailing worker logs). A 2026-07 docs
audit found **no backup/DR doc exists anywhere**. The vision's credo — owned end
to end — also implies data can *leave*: ownership without an export path is a
promise, not a property.

Sequencing pressure: SPEC-03 starts accruing months of hand-entered ledger data
the day it lands. This spec's P0 should land **before** that data exists, which
is why the build order places 09 P0 as a hard gate immediately before SPEC-03
(SPEC-07 is the burst-filler).

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

On the shared periodic runner (SPEC-01 P0.3's convention — no OS cron),
nightly at 03:00 UTC (`0 3 * * *`, a cron spec, not `@every`), enqueued on the
`default` queue served by the worker's light server (matches
[events.md](../../reference/events.md)). If dump duration makes that queue's
weight-1 slot contended, move the task to a dedicated `ops` queue in the same PR
that registers it.

If this lands before SPEC-01 P0.3, ops introduces the shared periodic
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

SPEC-04 is live, so `ops:backup_failed` gets a push path: a notify consumer
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
  server's `default` queue, SPEC-04 P0.2 step 4) fanning out through each wired module's `api/`
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
  consumer). The export inherits SPEC-01 P0.5's private-archive sensitivity: it
  bundles original photos with intact EXIF/GPS.
  **Download: proxy only (decided).** Because the archive carries EXIF/GPS
  originals, the SPEC-01 P0.5 rule applies. S3/R2 presigns are replayable until
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
  history — SPEC-04 §3) and other modules' derived/system rows.

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
  failed") once SPEC-04 is live.
- **Config/secret backup guidance** (`.env`, Traefik, MinIO creds) in the
  runbook — documented manual step, not automated.

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
  major version equal to the server's (currently `postgresql18-client`; P0.2).
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

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the docs commits on top changed no code).
The spec text above is the target; each row below is a place where the shipped
code still diverges from it, verified against `HEAD`. Rows are ordered by
severity — **Sec** (security, including the restore path), **Data** (silent
loss of backups or runs), **Func** (a requirement or AC does not hold),
**Contract** (API/OpenAPI surface), **Hyg** (schema hygiene), **P1** (unbuilt
nice-to-have). Paths are relative to the repo root. A row closes when the code
matches the spec and the SPEC-09 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded
with the named test (or, for the drill, the recorded run) as evidence.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 · Sec | P1.6 Queue console — read/write split + CSRF | Two asynqmon handlers: `ReadOnly: true` for `queues:read` holders, `ReadOnly: false` for `queues:write` holders, picked per request after `RequireAuth`; every non-GET/HEAD request needs `queues:write` (seeded to `admin` by P1.6's own migration) and `Sec-Fetch-Site: same-origin` or an `Origin` equal to the API origin, else 403. | `backend/cmd/api/main.go` builds one `asynqmon.New(asynqmon.Options{RootPath: "/admin/queues", RedisConnOpt: asynqRedis})` (`ReadOnly` unset, so writable) and mounts it for every method behind `RequireAuth` + `RequirePermission(engine, "queues:read")`. No migration seeds `queues:write` and no same-origin check exists, so any `queues:read` holder can retry, delete, archive or pause, and a same-site sibling (`minio.`, `mail.`, `traefik.`) can drive it with the admin's cookie. The console also displays task payloads, which today carry plaintext reset URLs (SPEC-04 §11 row 1). | **migration:** `000N_ops_queues_write` seeds `queues:write` and grants it to `admin` (0012 stays at four codes). **backend:** build both handlers; a small middleware serves the writable one only to `queues:write` holders, requires `queues:write` for non-GET/HEAD, and answers 403 to a non-GET without `Sec-Fetch-Site: same-origin` (or a matching `Origin`). **test:** httptest over the mount — TC-OPS-080, TC-OPS-081, TC-OPS-082, TC-OPS-083. | F087 |
| 2 · Sec | P0.4 step 0 Restore drill reads an explicit backup source | `RESTORE_S3_ENDPOINT`, `RESTORE_S3_BUCKET`, `RESTORE_S3_ACCESS_KEY`, `RESTORE_S3_SECRET_KEY`, each falling back to the stack's `S3_*` only when unset, so a dev stack can restore last night's prod dump. | `backend/scripts/restore-drill.sh` `mc()` hard-codes `mc alias set d http://minio:9000 …` and reads only `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` (not even `S3_ENDPOINT`) through a `minio/mc` container on the compose network, so it can only ever reach the disposable dev MinIO. | **backend (script):** resolve the four `RESTORE_S3_*` values with the `S3_*` fallback (including `S3_ENDPOINT`) and pass the endpoint to `mc alias set`; document them in `.env.example` and keep [backup-restore.md](../../operations/backup-restore.md) in step. **test:** TC-OPS-021a, recorded as a drill run against prod R2. | F084 |
| 3 · Sec | P0.4 step 3 Drill sanity checks | (a) `schema_migrations.dirty = false` and `version` ≤ the repo's latest migration; (b) `count(*) FROM users` ≥ 1 and `SELECT count(*) FROM assets` succeeds; (c) the 7 system roles from `0003` are present. Any failure exits non-zero with `RESTORE DRILL FAILED: <check>`. | `restore-drill.sh` step 4 checks `version ≤ REPO_LATEST` and `system roles ≥ 7`, and only that `users`/`assets` returned some value; it never reads `dirty`, never enforces `users ≥ 1`, and prints `!! …` messages. Manifest, download, `pg_restore` and `mc` failures exit through `set -e` with no `RESTORE DRILL FAILED:` prefix. | **backend (script):** `SELECT version, dirty FROM schema_migrations`; enforce `dirty = f` and `users ≥ 1`; route every failure (including a `trap … ERR`) through one helper that prints `RESTORE DRILL FAILED: <check>`. **test:** TC-OPS-023, TC-OPS-020. | F179 |
| 4 · Sec | P0.2 step 3 Sanitised `error` | `error` is a single line truncated to 500 chars; the DSN and the command line never appear in it (it is returned by P0.5 and published in `ops:backup_failed`). | `backend/internal/modules/ops/backup.go` `BackupDatabase` stores `truncate(err.Error(), 1000)`, and `dumpToStorage` wraps `pg_dump`'s stderr tail (`tail(stderr.Bytes(), 400)`) into that error. The stored, returned (`/ops/status` `last_run.error`), published and audited string is therefore multi-line, up to 1,000 bytes, may carry host/role/database names, and `truncate` can split a UTF-8 rune. | **backend:** one sanitiser — first line only (newlines collapsed), capped at 500 characters on a rune boundary, never interpolating the DSN or the command line; the full stderr goes to the worker log only; use it for the row, the event and the audit metadata. **test:** unit test on the sanitiser — TC-OPS-003b. | F178 |
| 5 · Data | P0.2 Task options + abandoned-row sweep | `asynq.MaxRetry(0)`, `asynq.Timeout(2h)`, `asynq.Unique(23h)`; at start, any `running` row older than the timeout becomes `failed` (`error='abandoned'`, `finished_at=now()`). | `backend/cmd/worker/main.go` registers `scheduler.Register("0 3 * * *", asynq.NewTask(opsapi.TaskBackupDatabase, nil), asynq.Queue("default"))`, so Asynq's defaults apply (25 retries, 30-min timeout, no uniqueness): a dump of a growing database is killed at 30 min and retried. `BackupDatabase` returns an error when `FinishBackupRunOK` fails, which re-runs the whole dump under those retries. No query in `backend/internal/modules/ops/query/ops.sql` reaps a stale `running` row, so a worker crash leaves one forever. | **backend:** add the three options to the `Register` call; add a `ReapAbandonedBackupRuns` query (`UPDATE ops_backup_runs SET status = 'failed', error = 'abandoned', finished_at = now() WHERE status = 'running' AND started_at < now() - INTERVAL '2 hours'`) and call it first in `BackupDatabase`. **test:** TC-OPS-003, TC-OPS-003a. | F086 |
| 6 · Data | P0.2 step 5 Retention set | Keep = the 7 most recent dump dates ∪, for each of the 4 most recent ISO weeks (Mon–Sun, UTC) that contain an ok dump, the latest ok dump of that week ∪ the current `LATEST.json` target. | `backend/internal/modules/ops/retention.go` `keep` keeps the 7 newest dumps plus the 4 newest **Sunday** dumps, so a week whose Sunday run failed keeps no weekly anchor. `backup.go` `pruneRetention` protects only `currentKey`, which is the `LATEST.json` target only when `writeManifest` succeeded. (The input — ok rows de-duplicated by key, never a listing — is already right.) | **backend:** weekly anchor = latest dump per `Date.ISOWeek()` over the 4 most recent ISO weeks that contain one; add the manifest's actual target to the keep set. **test:** rewrite `TestKeepRetention` and add the property test the AC names (failed Sunday, year boundary, duplicate same-day keys) — TC-OPS-004. | F085 |
| 7 · Func | P0.6 Failure alert delivery | A notify consumer `notify:on_backup_failed`, subscribed in `cmd/worker/main.go`, delivers one notification per failed run to every `admin` holder, naming the run and its sanitised `error`; it needs its own notification type. | `backup.go` `emitFailed` publishes `ops:backup_failed`, but `cmd/worker/main.go` has no `publisher.Subscribe` for it, `notify/api` has no `notify:on_backup_failed` task, and no notification type exists. | **backend:** `notifyapi.TaskOnBackupFailed`; a type in `notify/types.go` + `notify/README.md`; a handler that resolves the `admin` holders through an `account/api` read port and dispatches one intent each with `dedup_key = run_id` and a relative `data.href` (SPEC-04 P0.4 contract); `publisher.Subscribe(opsapi.EventBackupFailed, notifyapi.TaskOnBackupFailed, asynq.Queue("default"))`; register the task in events.md. **test:** TC-OPS-050. | F091 |
| 8 · Func | P1.6 Console reachable from the app | The Next admin nav links to the absolute `https://api.<domain>/admin/queues`; no Next page is added. | No link to `/admin/queues` anywhere in `frontend/src`. | **frontend:** an admin-nav entry built from the API base URL, shown to `queues:read` holders. **test:** TC-OPS-080. | F184 |
| 9 · Contract | §7 OpenAPI encoding | Per the specs README AuthZ OpenAPI encoding, `/ops/status` carries `x-required-permission: ops:read`. | `shared/openapi.yaml` `getOpsStatus` declares `bearerAuth` but no `x-required-permission` (no operation in the file carries one — the gap is repo-wide). | **openapi:** add the extension. **test:** the spec-first CI gate. | found while verifying 2026-10-01 |
| 10 · Hyg | §6 Run-row state constraint | Follow-up migration `000N_ops_backup_runs_state_chk` (`running` ⇒ no `finished_at`; `ok` ⇒ `finished_at`, `size_bytes`, `storage_key`; `failed` ⇒ `finished_at`, `error`), added `NOT VALID` then validated. | `backend/db/migrations/0012_ops_backup_runs.up.sql` has only the `status IN (…)` CHECK. | **migration:** as §6 states. Row 5's reap writes `finished_at` and `error`, so it satisfies the constraint. **test:** TC-OPS-121a. | F183 |
| 11 · P1 | P1.7 Owner takeout | `POST /api/v1/me/export`, `GET /api/v1/me/export/{id}`, `GET …/download`; `ops:takeout` on a dedicated `bulk` queue and server; the `ExportProvider` contract in each module's `api/`; `ops:purge_exports`; a tenant-scoped `ops_exports`. | Not built: no `ops_exports` table, no `/me/export` routes, no `ExportProvider` in any module's `api/`, no `bulk` server, no `ops:takeout` / `ops:purge_exports` (the takeout permission codes are already seeded by `0012`). | Build per P1.7, §6 and §7; the `bulk` server is shared with SPEC-04 §11 row 19. **test:** TC-OPS-100…105, TC-OPS-103a…103c. | F033 · F088 · F089 · F095 · F180 |

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
