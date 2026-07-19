# Test Cases — SPEC-09 Platform Ops

**Spec:** [SPEC-09](../product/specs/SPEC-09-platform-ops.md) · **Module:** `ops`
**Prefix:** `TC-OPS-` · **Plan:** [TEST-PLAN.md](TEST-PLAN.md) · **Risk:** R2 (irreplaceable data loss)

### Endpoints / tasks under test

| Method | Path / Task | Perm |
|---|---|---|
| GET | `/api/v1/ops/status` | `ops:read` (admin) |
| POST | `/api/v1/me/export` | `takeout:write:own` (P1.7) |
| GET | `/api/v1/me/export/{id}` | `takeout:read:own` (P1.7) |
| UI | `/admin/queues` (asynqmon) | `queues:read` (admin, P1.6) |
| task | `ops:backup_database` (nightly, periodic) | — |
| task | `ops:takeout`, `ops:purge_exports` (P1) | — |
| make | `make restore-drill` | operator |

### Preconditions

- Accounts `owner`,`userA`,`admin`,`guest`. Worker image has `postgresql-client` (PG 17).
- Storage (MinIO dev / R2 prod) reachable; `backups/pg/` prefix writable.
- Problem types: `ops/export-not-found`, `ops/export-expired`.
- Events: `ops:backup_completed`, `ops:backup_failed`, `ops:export_ready`.

---

## P0.2 — Nightly backup

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-001 | Nightly run produces off-VPS dump | Functional | P0(S1) | trigger `ops:backup_database` | dated dump `backups/pg/<yyyy-mm-dd>.dump` off-VPS; `backup_runs` row records size + duration; status `ok` | ☐ |
| TC-OPS-002 | pg_dump bypasses PgBouncer | Contract | P0 | inspect connection | dump connects **directly** to Postgres (session semantics), not PgBouncer | ☐ |
| TC-OPS-003 | Failed run handled + recovers | Reliability | P0(S1) | make DB unreachable during run | `ops:backup_failed` emitted; row says why; audit `ops.backup.failed`; **next night runs normally** (no wedged state) | ☐ |
| TC-OPS-004 | Retention keep/prune (property) | Data-integrity | P0(S1) | run keep/prune against synthetic 12+ daily dates | keeps last 7 daily + 4 weekly (most recent Sunday/week); prunes rest; never deletes LATEST target | ☐ [AUTO] |
| TC-OPS-005 | LATEST.json manifest + sha256 | Data-integrity | P0(S1) | after successful run | `LATEST.json {storage_key, sha256, size_bytes, finished_at}`; sha256 matches stored object | ☐ |
| TC-OPS-006 | Streamed (no temp file) | Contract | P1 | inspect impl | pg_dump stdout piped straight into `storage.Put(io.Reader)`; sha256 teed during upload | ☐ |
| TC-OPS-007 | Audit + events registered | Contract | P1 | check events.md + audit | `ops:backup_completed/failed` + `ops.backup.completed/failed` registered | ☐ (CC-5) |

## P0.4 — Restore drill

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-020 | Drill passes on fresh dev stack | Functional | P0(S1) | `make restore-drill` on a **fresh** stack (no backup_runs table) against a **real** nightly dump | restore exits 0; sanity checks pass | ☐ [MANUAL] |
| TC-OPS-021 | Selection via manifest, not key-listing | Reliability | P0 | drill reads LATEST.json | selects from manifest (never latest-by-key-listing that a partial upload could poison) | ☐ |
| TC-OPS-022 | sha256 verified before restore | Data-integrity | P0(S1) | drill downloads dump | verifies sha256 matches manifest before pg_restore | ☐ |
| TC-OPS-023 | Sanity checks meaningful | Functional | P0 | after restore into scratch DB | migration version ≤ repo latest & high enough for checked tables; `users` + `assets` plausible counts; one known row spot-query | ☐ |
| TC-OPS-024 | Runbook followable by a stranger | Usability | P0 | someone who didn't write it follows `docs/guides/backup-restore.md` | completes start-to-finish | ☐ [MANUAL] |

## P0.5 — Freshness sentinel

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-040 | stale after 26 h | Functional | P0 | last success 27 h ago, no completed since; GET `/ops/status` | `state=stale`; a fresh success → `ok` | ☐ |
| TC-OPS-041 | failed precedence over recent success | Functional | P0(S1) | success 2 h ago then a failed run | `state=failed` (precedence — wins even with recent success) | ☐ |
| TC-OPS-042 | Zero runs ever → stale null | Boundary | P0 | brand-new instance, no runs | `state=stale`, `last_success_at:null`, `hours_since_success:null` (never `ok`) | ☐ |
| TC-OPS-043 | running doesn't change state | Functional | P1 | during a running backup | state reflects last **completed**, not the running one | ☐ |
| TC-OPS-044 | Non-admin → 403 | AuthZ | P0(S1) | userA GET `/ops/status` | 403 | ☐ (CC-2) |
| TC-OPS-045 | Unauthenticated → 401 | AuthZ | P0 | guest GET `/ops/status` | 401 | ☐ |
| TC-OPS-046 | /healthz untouched | Contract | P1 | GET `/healthz` | still unauthenticated liveness; backup metadata not folded in | ☐ |

## P0.3 — Media durability posture (doc)

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-060 | Runbook states DB is fragile asset | Doc | P1 | read runbook | states media on R2 (durable), DB is fragile; MinIO bind-mount disposable; no bucket-in-pg_dump | ☐ |

## P1 — Queue console

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-080 | Queue console gated | AuthZ | P1 | admin vs non-admin GET `/admin/queues` | admin 200 (RequireAuth + `queues:read`); non-admin 403 | ☐ [P1] |
| TC-OPS-081 | Dead-letter inspect + retry | Functional | P1 | fail a task to DLQ; retry from browser | job inspectable + retryable; assets survive CSP/proxy | ☐ [P1] |

## P1 — Owner takeout

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-100 | Export produces per-module archive | Functional | P1 | POST `/me/export` → 202 {export_id}; wait; GET `/me/export/{id}` | `portal-export-<yyyy-mm>.tar.gz` w/ journal md, bank CSVs, media originals+metadata, people JSON, account profile+audit | ☐ [P1] |
| TC-OPS-101 | Fan-out via ExportProvider only | Contract | P1 | inspect impl | never touches another module's tables — providers only (boundary) | ☐ [P1] |
| TC-OPS-102 | Download via owner-scoped proxy / TTL key | Security | P1 | GET download URL | authenticated owner-scoped proxy, or ≤5-min single-use opaque key (bundles GPS originals — private-archive sensitivity) | ☐ [P1] |
| TC-OPS-103 | Cross-owner export blocked | AuthZ | P1(S1) | userA GET userB's export | 404 | ☐ [P1] (CC-3) |
| TC-OPS-104 | Expired export → 410 | Negative | P1 | export older than 7 days (purged) | `GET` → 410 `ops/export-expired` (never `ready` with dead URL); `ops:purge_exports` set status `expired` | ☐ [P1] |
| TC-OPS-105 | notifications table excluded | Contract | P1 | inspect export contents | `notifications` (delivery store) + derived/system rows excluded by design | ☐ [P1] |

## P0.1 / Cross-cutting / contract

| ID | Scenario | Type | Pri | Steps | Expected | Status |
|----|----------|------|-----|-------|----------|--------|
| TC-OPS-120 | Permission seeding | AuthZ | P0 | inspect grants | `ops:read`,`queues:read`→admin; `takeout:read/write:own`→user | ☐ (CC-2) |
| TC-OPS-121 | ops tables system-scoped | Contract | P1 | inspect schema | `ops_backup_runs` has no user_id; `ops_exports` has user_id | ☐ |
| TC-OPS-122 | All non-2xx RFC-7807 | Contract | P0 | error paths | Problem+json + stable type | ☐ (CC-1) |
| TC-OPS-123 | Migration up/down | Contract | P1 | migrate + down `ops_backup_runs` | clean; status CHECKs; started_at index | ☐ |
