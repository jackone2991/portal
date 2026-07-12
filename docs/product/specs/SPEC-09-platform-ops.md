# SPEC-09 — Platform Ops: Backup/Restore, Queue Console, Takeout

**Status:** ready to build, rev 1 · **Drafted:** 2026-07-10
**Module:** `ops` (new — not scaffolded; owns the `ops:*` prefix) · **Depends on:** nothing hard on data — rides (or, if it lands first, introduces) SPEC-01 P0.3's shared periodic-scheduler convention; failure alerts get delivery when SPEC-04 lands (degrade to logs + status endpoint until then)
**Upstream:** [briefs/09-platform-ops.md](../briefs/09-platform-ops.md) · **Refs:** [ADR-01](../../adr/01-v1-scope-cut.md) (observability stays deferred), [ADR-03](../../adr/03-single-vps-topology.md), [ADR-04](../../adr/04-storage-tier-budget.md), backlog §7, feature-inventory `D-25` (audit types)
**Downstream consumers:** every module holding irreplaceable data (bank, journal, media, people); SPEC-04 (backup-failure alerts, later)

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
is why the [briefs build order](../briefs/README.md) lists it as a
dependency-free burst-filler.

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

- As the owner, I run `make restore-drill` on a fresh dev stack and my prod
  ledger from last night is queryable — before I ever need it in anger. *(primary)*
- As the owner, a transcode dies permanently and I see it in `/admin/queues` and
  retry it from the browser. *(P1)*
- As the owner, I download `portal-export-2026-07.tar.gz` and my journal, ledger
  CSVs, and original photos are inside, in open formats. *(P1)*
- Edge: as the owner, the nightly backup silently stopped a week ago — the status
  endpoint has been screaming `stale` since hour 26, and the failure event is in
  the audit log.

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
may still ship in a later P1 migration.

### P0.2 — Nightly `ops:backup_database`

On the shared periodic runner (SPEC-01 P0.3's convention — no OS cron), nightly:

If this lands before SPEC-01 P0.3, ops introduces the shared periodic
scheduler itself — what is borrowed is the convention (Asynq periodic, never
OS cron), not code.

1. Insert a `backup_runs` row (`running`).
2. `pg_dump -Fc` streamed through `platform/storage` to
   `backups/pg/<yyyy-mm-dd>.dump` (R2 prod / MinIO dev — **same code path both**,
   bucket differs; locked from the brief: dev exercising the machinery is the
   point of the drill). **Dump connects directly to Postgres, not through
   PgBouncer** — pg_dump needs session semantics a transaction pooler breaks.
3. Finish the row (`ok` + `size_bytes` + `storage_key`, or `failed` + `error`).
4. On success, overwrite `backups/pg/LATEST.json` `{storage_key, sha256,
   size_bytes, finished_at}` — sha256 computed by teeing the pg_dump stream
   through a hasher during upload. Retention (step 5) must never delete it or
   its target.
5. **Retention** after a successful run: keep the last **7 daily** dumps and **4
   weekly** (the most recent Sunday dump per week); prune the rest from storage.
6. Emit `ops:backup_completed {run_id, size_bytes}` / `ops:backup_failed
   {run_id, error}` (register in [events.md](../../reference/events.md)); write
   audit events `ops.backup.completed` / `ops.backup.failed` per D-25.

The worker image gains `postgresql-client` (locked from the brief: smallest
moving-part count; pin the client's **major version to Postgres 17**).

**Acceptance criteria.**
- Given a nightly run, then a dated dump exists off-VPS and the run row records
  size + duration.
- Given a failed run (e.g. DB unreachable), then `ops:backup_failed` is emitted,
  the row says why, and the next night runs normally (no wedged state).
- Given 12 daily dumps accumulated, then pruning leaves exactly the retention set
  (property test on the keep/prune function against synthetic date lists).
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

`make restore-drill`: runs **on a fresh dev stack, where no `backup_runs`
table exists to consult** — dump selection cannot depend on the database.
*(2026-07-10 — the original "latest `ok` dump" + "migrations table matches the
dump's expected head" checks were unimplementable: nothing recorded an
expected head, and comparing the restored migrations table against itself is
a tautology.)* Mechanics:

1. **Selection via manifest**: after every successful run, P0.2's task
   overwrites a small `backups/pg/LATEST.json` `{storage_key, sha256,
   size_bytes, finished_at}`. The drill reads the manifest — never
   latest-by-key-listing, which a failed partial upload could poison.
2. Download, verify `sha256`, `pg_restore` into a scratch database
   (`portal_restore_check` or a throwaway container); the restore must exit 0.
3. **Sanity checks** (each meaningful without knowing the dump's history):
   the restored migration version is `≤` the checked-out repo's latest
   migration number and high enough to contain the tables checked next;
   `users` and `assets` (the media table's real name — not `media_assets`)
   return plausible row counts; spot-query one known row.

Documented as a runbook in `docs/guides/backup-restore.md` (this spec's PR
ships the doc). Quarterly execution is a calendar practice, not automation.

**Acceptance criteria.**
- The drill passes against a **real nightly dump** before this spec closes —
  landing the code without one exercised restore does not count as done.
- The runbook is followable start-to-finish by someone who didn't write it (the
  honest test of a runbook).

### P0.5 — Freshness sentinel

`GET /api/v1/ops/status` — permission `ops:read` (admin-tier; the ops
migration seeds it and grants to `admin` — the `*` wildcard also covers it).
Returns `{last_success_at, hours_since_success, state, last_run: {...}}`.

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
  `hours_since_success: null`.
- Given a non-admin caller, then 403; unauthenticated, 401.

### P1 — nice to have

- **P1.6 Queue console**: mount `hibiken/asynqmon`'s `http.Handler` at
  `/admin/queues` inside `cmd/api`, gated `RequireAuth` +
  `RequirePermission("queues:read")` (seeded and granted to `admin` alongside
  `ops:read` — see P0.1; wired via the documented
  `Engine()` exception). Dead-lettered jobs are inspectable and retryable from
  the browser. Configure the handler's base path; verify its inline assets
  survive the current CSP/proxy setup.
- **P1.7 Owner takeout**: `POST /api/v1/me/export` (permission
  `takeout:write:own` — canonical verb; seeded to `user` with
  `takeout:read:own` — see P0.1) → 202 `{export_id}`; enqueues
  `ops:takeout` (default queue) fanning out through each wired module's `api/`
  **`ExportProvider`** interface — `accountapi` (profile + audit trail JSON),
  `mediaapi` (metadata JSON + originals), journal (markdown files), bank (CSV
  per account + categories/budgets), people (JSON) as they land. Produces one
  `portal-export-<yyyy-mm>.tar.gz` in storage (`exports/<user_id>/<id>.tar.gz`);
  `GET /api/v1/me/export/{id}` (`takeout:read:own`) returns status + a download
  URL served through an authenticated owner-scoped proxy (matching SPEC-01
  P0.5), or — if presigning R2 directly — a single-use, opaque key with an
  explicit TTL of ≤5 min; emits `ops:export_ready` (future notify consumer).
  The export inherits SPEC-01 P0.5's private-archive sensitivity: it bundles
  original photos with intact EXIF/GPS.
  **Boundary rule: the fan-out never touches another module's tables — providers
  only.** **Prune mechanics (2026-07-10 — previously asserted with no owner):**
  nightly janitor task `ops:purge_exports` (default queue, registered in
  events.md) deletes archives older than 7 days from storage and sets the row's
  status to `expired`; `GET` on an expired export returns 410 Problem
  `ops/export-expired` — never `ready` with a dead URL. Excluded by design: the
  `notifications` table (a delivery store, not history — SPEC-04 §3) and other
  modules' derived/system rows.

### P2 — future considerations (design for, don't build)

- **Backup encryption at rest** (age/GPG) — required before any *second* user's
  data arrives; key handling is the hard part, design then.
- **`ops:job_dead` event** on dead-letter archive → notify ("your transcode
  failed") once SPEC-04 is live.
- **Config/secret backup guidance** (`.env`, Traefik, MinIO creds) in the
  runbook — documented manual step, not automated.

## 6. Data model — migration `000N_ops_backup_runs`

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
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
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

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/ops/status` | `ops:read` (admin) | P0.5 freshness sentinel |
| POST | `/api/v1/me/export` | `takeout:write:own` | P1.7; 202 `{export_id}` |
| GET | `/api/v1/me/export/{id}` | `takeout:read:own` | P1.7; status + download URL; 410 when expired |
| — | `/admin/queues` (mounted UI) | `queues:read` (admin) | P1.6; asynqmon, not OpenAPI |

Problem types: `ops/export-not-found`, `ops/export-expired`.
Tasks/events owned: `ops:backup_database`, `ops:takeout` (P1),
`ops:purge_exports` (P1), `ops:backup_completed`, `ops:backup_failed`,
`ops:export_ready` — all registered in
[events.md](../../reference/events.md).

## 8. Success metrics (n=1 honest)

- Leading: 7/7 mornings in the first week show a fresh dated dump off-VPS;
  `hours_since_success` never exceeds 26 outside induced failures.
- Leading: one restore drill executed and documented before the spec closes;
  quarterly thereafter (calendar-verified, not vibes).
- Lagging: a takeout archive opens and contains the expected per-module formats
  (spot-check against live data). *(P1)*

## 9. Timeline & phasing

1. Scaffold + migration + sqlc (½ day)
2. Backup task + streaming + retention + manifest + events + audit (1.5 days)
3. Restore drill + `make` target + runbook doc (1 day)
4. Status endpoint + OpenAPI (½ day)
5. P1: queue console (1 day); takeout fan-out + providers for wired modules +
   download endpoint (3–4 days, grows as modules land)
P0 ≈ 3.5–4 dev-days (brief's estimate holds); P1 adds ~5.

## 10. Open questions

- **(resolved)** `pg_dump` location: `postgresql-client` in the worker image,
  major-version-pinned to PG 17 (P0.2).
- **(resolved)** Dev backups: same code path as prod, MinIO bucket (P0.2).
- **(resolved)** Sentinel placement: separate authenticated endpoint, not
  `/healthz` (P0.5).
- **(resolved 2026-07-10)** Streaming vs temp-file for `pg_dump` output:
  **stream** — `platform/storage.Store.Put(ctx, key, body io.Reader,
  contentType)` already takes an `io.Reader` (verified in `storage.go`/`s3.go`);
  pipe `pg_dump` stdout straight into it.
- **(product, non-blocking)** Takeout formats are proposed per module in P1.7 —
  lock each at that module's provider implementation.
