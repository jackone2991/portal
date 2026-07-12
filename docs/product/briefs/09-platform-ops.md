# 09 — Platform Ops: Backup/Restore, Queue Console, Takeout

**Module:** `ops` (new — not scaffolded; owns the `ops:*` prefix) · **Effort:** ~4 days P0, +5 P1 · **Depends on:** nothing hard; failure alerts get delivery when SPEC-04 lands (degrade to logs + `/healthz` until then).
**Unlocks:** the right to hold irreplaceable data. SPEC-03 puts months of hand-entered finance on one VPS and media holds memories — today a single disk failure ends the life-OS thesis.
**Provenance:** a 2026-07 docs audit found **no backup/DR doc exists anywhere**; backlog §7 names pieces but nothing owns them — researched 2026-07-10. **Spec:** [SPEC-09](../specs/SPEC-09-platform-ops.md).

## Problem statement

A self-hosted life OS asks the owner to move their financial history and memories
onto hardware they operate. That deal is only honest with three properties the
stack currently lacks: **backups that run themselves**, **a restore path that has
actually been exercised**, and **visibility into the async machinery** (today a
failed transcode is discoverable only by tailing worker logs). The vision's credo —
owned end to end — also implies data can *leave*: ownership without an export path
is a promise, not a property.

## Goals

- Nightly automated Postgres backups to off-VPS storage (R2), with retention.
- A **tested** restore procedure — a backup that has never been restored is a hope.
- Backup failure/staleness is impossible to miss (event + health signal).
- The solo operator can inspect/retry/purge Asynq jobs without SSH.
- The owner can export their data per module into one archive.

## Non-goals

- The 5-service observability stack (Loki/Prometheus/Tempo/Grafana) — stays in
  [04-deferred](04-deferred.md) per ADR-01; the queue console is in-process, $0.
- Media-file backup via pg_dump — object storage is handled by its own mechanism
  (see P0.3); don't stream buckets through the database job.
- Cross-region DR, standby replicas — single-VPS envelope; off-site copies, not HA.
- GDPR-grade takeout formats — this is the owner's own data leaving politely.

## User stories

- As the owner, I `make restore-drill` on a fresh dev stack and my prod ledger from
  last night is queryable — before I ever need it in anger.
- As the owner, a transcode dies permanently and I see it in `/admin/queues` and
  retry it from the browser.
- As the owner, I download `portal-export-2026-07.tar.gz` and my journal, ledger
  CSVs, and original photos are inside, in open formats.

## Requirements

### P0 — must have (backup discipline)

1. **Module scaffold** `ops` per MODULES.md §8; migration `000N_ops_backup_runs`
   (id, started_at, finished_at, status, size_bytes, storage_key, error).
2. **Nightly `ops:backup_database`** on the shared periodic runner (SPEC-01 P0.3
   convention — no OS cron): `pg_dump -Fc` streamed through `platform/storage` to
   `backups/pg/<date>.dump` (R2 prod / MinIO dev). Retention: keep 7 daily + 4
   weekly, prune the rest. Emits `ops:backup_completed` / `ops:backup_failed`
   (register in events.md; audit types `ops.backup.completed/.failed` per D-25).
   - [ ] Given a nightly run, a dated dump exists off-VPS and a `backup_runs` row
         records size + duration.
   - [ ] Given a failed run, `ops:backup_failed` is emitted and the run row says why.
3. **Media durability note, decided here**: prod media lives on R2 (11-nines-class
   durability) — the *database* is the fragile asset. Dev MinIO bind-mount is
   explicitly disposable. Document this in the runbook so nobody "fixes" it by
   dumping buckets nightly.
4. **Restore drill**: `make restore-drill` — downloads the latest dump, restores
   into a scratch database, runs a row-count sanity check; documented runbook in
   `docs/guides/`. Run it once per quarter (calendar note, not automation).
   - [ ] The drill passes against a real nightly dump before this brief closes.
5. **Freshness sentinel**: `/healthz` (or a tiny `GET /api/v1/ops/status`,
   `ops:read` admin permission) reports hours-since-last-successful-backup; >26 h
   is a failure state.

### P1 — nice to have (visibility + takeout)

6. **Queue console**: mount `hibiken/asynqmon`'s handler at `/admin/queues` inside
   `cmd/api`, gated `RequireAuth` + `RequirePermission("queues:read")` (admin roles;
   `*` covers it) via the documented `Engine()` exception. Dead-lettered jobs are
   inspectable and retryable from the browser.
7. **Owner takeout**: `POST /api/v1/me/export` enqueues `ops:takeout` (default
   queue) fanning out through each wired module's `api/` `ExportProvider` —
   `accountapi` (profile, audit trail JSON), `mediaapi` (metadata JSON + originals),
   journal/bank/people as they land; produces one archive in storage + a signed
   status/download endpoint; emits `ops:export_ready` (future notify consumer).
   Boundary rule: the fan-out never touches another module's tables — providers only.

### P2 — future considerations (design for, don't build)

8. Backup encryption at rest (age/GPG) before any second user's data arrives.
9. `ops:job_dead` event on dead-letter archive → notify ("your transcode failed").
10. Config/secret backup guidance (.env, Traefik) in the runbook — not automated.

## Data model sketch

```
ops_backup_runs(
  id uuid pk, started_at timestamptz not null, finished_at timestamptz,
  status text not null check (status in ('running','ok','failed')),
  size_bytes bigint, storage_key text, error text
)
```

## API sketch (add to `shared/openapi.yaml`)

```
GET  /api/v1/ops/status                  (P0.5 — or fold into /healthz)
POST /api/v1/me/export                   (P1)
GET  /api/v1/me/export/{id}              (P1 — status + download URL)
```

## Open questions

- **(engineering, non-blocking)** `pg_dump` from the worker container needs the
  postgres client tools in the worker image — add to Dockerfile, or run backups in
  a tiny sidecar? Recommendation: add `postgresql-client` to the worker image
  (smallest moving-part count).
- **(engineering, non-blocking)** Backup MinIO-in-dev too, or only prod R2 paths?
  Recommendation: same code path both (bucket name differs) — dev exercises the
  machinery, which is the point of the drill.
- **(product, non-blocking)** Takeout formats per module: journal → markdown files,
  bank → CSV per account, media → originals. Lock per-module at implementation.
