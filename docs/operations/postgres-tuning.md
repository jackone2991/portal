# Postgres / PgBouncer / pgx tuning

**Status:** stub — the pool-sizing note promised by [ADR-03](../adr/03-single-vps-topology.md)
(step 5, "Document in `docs/operations/postgres-tuning.md`") and flagged by
[ADR-00](../adr/00-architecture-review.md) ("No mention of database connection-pool
tuning between PgBouncer transaction-pool mode and `pgx`"). **v1 does not need this
tuned** — dev connects directly and load is n=1; it becomes load-bearing at Phase 1
(tenancy/RLS routes the app through PgBouncer) and under concurrent transcode bursts.

## Two separate problems (do not conflate)

1. **pgx ⨯ PgBouncer transaction mode — a *protocol* clash.** PgBouncer in
   transaction-pool mode is incompatible with pgx's default prepared-statement
   cache (a statement prepared on one server connection may not exist on the next).
   **Resolved for v1 by connecting the app directly to `postgres:5432`, bypassing
   PgBouncer** (see [overview.md](../architecture/overview.md) §4 and
   [getting-started.md](../guides/getting-started.md)). When Phase 1 routes the app
   through PgBouncer, set the pool's `DefaultQueryExecMode = QueryExecModeExec` (or
   simple protocol) / disable the statement cache — see
   [ADR-07](../adr/07-tenancy-rls-model.md) §"pgx ⨯ PgBouncer transaction mode".

2. **Pool sizing — a *capacity* problem.** `api`, `worker`, and Asynq share one
   Postgres cluster. Without a connection budget, a transcode burst on `worker` can
   exhaust connections and starve `api` ([ADR-00](../adr/00-architecture-review.md)).
   This document is the home for that budget when it is set.

## v1 posture (today)

- **Dev:** app → `postgres:5432` **directly** (no PgBouncer); MinIO storage. Load is
  n=1 — no tuning required.
- **Prod:** app → PgBouncer, transaction-pool mode (~30 MB — [ADR-03](../adr/03-single-vps-topology.md) §resource table).

## Target settings (from ADR-03 — not yet applied)

| Setting | Target | Notes |
|---|---|---|
| `shared_buffers` | `4GB` | single-VPS envelope |
| `effective_cache_size` | `10GB` | |
| `max_connections` | `50` | PgBouncer pools below this |
| pgx pool `MaxConns` — `api` | _TBD_ | budget so worker/asynq bursts can't starve the API |
| pgx pool `MaxConns` — `worker` | _TBD_ | cap concurrent transcode DB usage |

## When to fill this in

- **Phase 1 (tenancy):** the app starts routing through PgBouncer → pin the protocol
  exec mode and the real per-binary `MaxConns` numbers.
- **First observed contention:** a transcode burst measurably raises API latency →
  set the worker/api pool split and re-measure.

Until then this file exists so the gap is *tracked, not silently open*.
