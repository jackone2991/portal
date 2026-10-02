# Test Cases — SPEC-03 Platform Ops

**Status:** current · **Last verified:** 2026-09-30

**Spec:** [SPEC-03](../product/specs/SPEC-03-platform-ops.md) · **Module:** `ops`
**Prefix:** `TC-OPS-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md) · **Risk:** R2 (irreplaceable data loss)

### Endpoints / tasks under test

| Method | Path / Task | Perm |
|---|---|---|
| GET | `/api/v1/ops/status` | `ops:read` (admin) |
| POST | `/api/v1/me/export` | `takeout:write:own` (P1.7) |
| GET | `/api/v1/me/export/{id}` | `takeout:read:own` (P1.7) |
| GET | `/api/v1/me/export/{id}/download` | `takeout:read:own` (P1.7) |
| UI GET/HEAD | `https://api.<domain>/admin/queues` (asynqmon) | `queues:read` (admin, P1.6) |
| UI non-GET | `https://api.<domain>/admin/queues/*` (asynqmon mutations) | `queues:write` (admin, P1.6) |
| task | `notify:on_backup_failed` (P0.6) | — |
| task | `ops:backup_database` (nightly, periodic) | — |
| task | `ops:takeout`, `ops:purge_exports` (P1) | — |
| make | `make restore-drill` | operator |

### Preconditions

- Accounts `owner`,`userA`,`admin`,`guest`. Worker image has `postgresql18-client` (major version equal to the PG 18 server).
- Storage (MinIO dev / R2 prod) reachable; `backups/pg/` prefix writable.
- Problem types: `ops/export-not-found`, `ops/export-not-ready`, `ops/export-expired`, `ops/export-in-progress`.
- Events: `ops:backup_completed`, `ops:backup_failed`, `ops:export_ready`.

---

## P0.2 — Nightly backup

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-001 | Nightly run produces off-VPS dump | Functional | P0(S1) | trigger `ops:backup_database` (scheduled `0 3 * * *` UTC, `default` queue) | dated dump `backups/pg/<yyyy-mm-dd>.dump` (UTC date) off-VPS; `ops_backup_runs` row records size + duration; status `ok` | ☐ |
| TC-OPS-002 | pg_dump bypasses PgBouncer, uses owner DSN | Contract | P0 | inspect connection; unset `BACKUP_DATABASE_URL` and run | dump uses `BACKUP_DATABASE_URL` **directly** to Postgres as `portal` (not PgBouncer, never `DATABASE_URL`); unset → run `failed` with `BACKUP_DATABASE_URL is not configured` | ☐ |
| TC-OPS-003 | Failed run handled + recovers | Reliability | P0(S1) | make DB unreachable during run | exactly one `ops:backup_failed` (MaxRetry 0); row says why; audit `ops.backup.failed` with `actor_kind='system'`, `target_kind='ops_backup_run'`; **next night runs normally** (no wedged state) | ☐ |
| TC-OPS-003a | Abandoned running row reaped | Reliability | P0 | leave a `running` row older than 2 h; run the task | the old row becomes `failed` (`error='abandoned'`, `finished_at` set) | ☐ |
| TC-OPS-003b | Error string sanitised | Security | P1 | fail pg_dump with a DSN-bearing error | `error` is one line, ≤ 500 chars, no DSN or command line (row, `/ops/status`, event) | ☐ |
| TC-OPS-004 | Retention keep/prune (property) | Data-integrity | P0(S1) | run keep/prune against synthetic date lists built from `ok` rows | keeps 7 most recent dump dates ∪ latest ok dump of each of the 4 most recent ISO weeks (UTC) ∪ LATEST target; prunes rest; covers a week whose Sunday run failed, a year boundary, duplicate same-day keys; objects with no ok row untouched | ☐ [AUTO] |
| TC-OPS-005 | LATEST.json manifest + sha256 | Data-integrity | P0(S1) | after successful run | `LATEST.json {storage_key, sha256, size_bytes, finished_at}`; sha256 matches stored object | ☐ |
| TC-OPS-006 | Spooled to a temp file | Contract | P1 | inspect impl | pg_dump stdout written to a temp file with sha256 teed in the same pass, `Seek(0)`, then `Storage.Put(tmp)`; temp file removed on exit | ☐ |
| TC-OPS-007 | Audit + events registered | Contract | P1 | check events.md + audit | `ops:backup_completed/failed` + `ops.backup.completed/failed` registered | ☐ (CC-5) |

## P0.4 — Restore drill

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-020 | Drill passes on fresh dev stack | Functional | P0(S1) | `make restore-drill` on a **fresh** stack (no `ops_backup_runs` table) against a **real** nightly dump | restore exits 0; sanity checks pass | ☐ [MANUAL] |
| TC-OPS-021 | Selection via manifest, not key-listing | Reliability | P0 | drill reads LATEST.json | selects from manifest (never latest-by-key-listing that a partial upload could poison) | ☐ |
| TC-OPS-021a | Explicit backup source | Functional | P0 | set `RESTORE_S3_*` to prod R2 (read-only token) | drill restores last night's prod dump; the stack's `S3_*` unchanged | ☐ [MANUAL] |
| TC-OPS-022 | sha256 verified before restore | Data-integrity | P0(S1) | drill downloads dump | verifies sha256 matches manifest before pg_restore | ☐ |
| TC-OPS-023 | Sanity checks meaningful | Functional | P0 | after restore into scratch DB | `schema_migrations.dirty = false` and version ≤ repo latest; `users` count ≥ 1 and `assets` count succeeds; 7 system roles present; any failure → non-zero exit `RESTORE DRILL FAILED: <check>` | ☐ |
| TC-OPS-024 | Runbook followable by a stranger | Usability | P0 | someone who didn't write it follows `docs/operations/backup-restore.md` | completes start-to-finish | ☐ [MANUAL] |

## P0.5 — Freshness sentinel

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-040 | stale after 26 h | Functional | P0 | last success 27 h ago, no completed since; GET `/ops/status` | `state=stale`; a fresh success → `ok` | ☐ |
| TC-OPS-041 | failed precedence over recent success | Functional | P0(S1) | success 2 h ago then a failed run | `state=failed` (precedence — wins even with recent success) | ☐ |
| TC-OPS-042 | Zero runs ever → stale null | Boundary | P0 | brand-new instance, no runs | `state=stale`, `last_success_at:null`, `hours_since_success:null`, `last_run:null` (never `ok`) | ☐ |
| TC-OPS-042a | Response shape | Contract | P0 | GET `/ops/status` with runs present | `last_success_at` RFC3339; `hours_since_success` fractional, unrounded; `last_run` = most recent row by `started_at` with `{id, status, started_at, finished_at, size_bytes, storage_key, error}` | ☐ |
| TC-OPS-043 | running doesn't change state | Functional | P1 | during a running backup | state reflects last **completed**, not the running one | ☐ |
| TC-OPS-044 | Non-admin → 403 | AuthZ | P0(S1) | userA GET `/ops/status` | 403 | ☐ (CC-2) |
| TC-OPS-045 | Unauthenticated → 401 | AuthZ | P0 | guest GET `/ops/status` | 401 | ☐ |
| TC-OPS-046 | /healthz untouched | Contract | P1 | GET `/healthz` | still unauthenticated liveness; backup metadata not folded in | ☐ |

## P0.6 — Failure alert delivery

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-050 | Failed backup notifies admins | Functional | P0 | fail a run | `notify:on_backup_failed` delivers one notification per admin holder (sanitised error) | ☐ (unbuilt) |

## P0.3 — Media durability posture (doc)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-060 | Runbook states DB is fragile asset | Doc | P1 | read runbook | states media on R2 (durable), DB is fragile; MinIO bind-mount disposable; no bucket-in-pg_dump | ☐ |

## P1 — Queue console

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-080 | Queue console gated | AuthZ | P1 | admin vs non-admin vs guest GET `https://api.<domain>/admin/queues` | admin 200 (RequireAuth + `queues:read`); non-admin 403; unauthenticated 401 | ☐ [P1] |
| TC-OPS-081 | Dead-letter inspect + retry | Functional | P1 | fail a task to DLQ; retry from browser as admin | job inspectable; retry re-enters `pending`; assets survive CSP/proxy | ☐ [P1] |
| TC-OPS-082 | Read-only caller cannot mutate | AuthZ | P1 | caller with only `queues:read` POSTs a retry | 403 (read-only handler; `queues:write` required) | ☐ [P1] |
| TC-OPS-083 | Cross-origin mutation blocked | Security | P1 | POST from a same-site sibling (e.g. `minio.`) with the admin cookie | 403 (`Sec-Fetch-Site`/`Origin` check) | ☐ [P1] |

## P1 — Owner takeout

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-100 | Export produces per-module archive | Functional | P1 | POST `/me/export` → 202 {export_id}; wait; GET `/me/export/{id}` | `portal-export-<yyyy-mm>.tar.gz` w/ journal md, bank CSVs, media originals+metadata, people JSON, account profile+audit | ☐ [P1] |
| TC-OPS-101 | Fan-out via ExportProvider only | Contract | P1 | inspect impl | never touches another module's tables — providers only (boundary) | ☐ [P1] |
| TC-OPS-102 | Download via owner-scoped proxy only | Security | P1 | GET `/api/v1/me/export/{id}/download` | streams via `Storage.Get` with `Content-Disposition: attachment`; never a presigned URL; pending/running → 409 `ops/export-not-ready` | ☐ [P1] |
| TC-OPS-103 | Cross-owner export blocked | AuthZ | P1(S1) | userA GET userB's export and its download | 404 `ops/export-not-found` | ☐ [P1] (CC-3) |
| TC-OPS-103a | One in-flight export per user | Functional | P1 | POST twice while the first is pending/running | second → 409 `ops/export-in-progress` carrying the existing `export_id` | ☐ [P1] |
| TC-OPS-103b | Provider failure fails the export | Reliability | P1 | make one provider error | export `failed` with `error='<provider>: <msg>'`; no archive object remains | ☐ [P1] |
| TC-OPS-103c | Deleted owner's archive purged | Data-integrity | P1 | delete a user with a ready export; run `ops:purge_exports` | `user_id` set NULL; archive object and row gone after the purge | ☐ [P1] |
| TC-OPS-104 | Expired export → 410 | Negative | P1 | export older than 7 days (purged) | `GET` → 410 `ops/export-expired` (never `ready` with dead URL); `ops:purge_exports` set status `expired` | ☐ [P1] |
| TC-OPS-105 | notifications table excluded | Contract | P1 | inspect export contents | `notifications` (delivery store) + derived/system rows excluded by design | ☐ [P1] |

## P0.1 / Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-120 | Permission seeding | AuthZ | P0 | inspect grants | `ops:read`,`queues:read`→admin (0012); `queues:write`→admin (P1.6 migration); `takeout:read/write:own`→user | ☐ (CC-2) |
| TC-OPS-121 | ops table scoping (runs system-scoped, exports per user) | Contract | P1 | inspect schema | `ops_backup_runs` has no user_id; `ops_exports` has nullable user_id `ON DELETE SET NULL` | ☐ |
| TC-OPS-121a | Run-row state constraint | Data-integrity | P1 | try to write `ok` without key/size, `failed` without error, `running` with `finished_at` | each rejected by `ops_backup_runs_state_chk` | ☐ |
| TC-OPS-122 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-OPS-123 | Migration up/down | Contract | P1 | migrate + down `ops_backup_runs` | clean; status CHECKs; started_at index | ☐ |
