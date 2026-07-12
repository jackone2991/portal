-- ops module queries (SPEC-09 P0). sqlc input only — regenerate with `make sqlc`;
-- never hand-edit the *.sql.go output. ops_backup_runs is system-scoped (no
-- user_id): backups are a platform concern, one ledger for the whole install.

-- name: InsertBackupRun :one
-- Open a run in the 'running' state at the start of the nightly backup task. The
-- returned id threads through the finish/emit steps.
INSERT INTO ops_backup_runs DEFAULT VALUES
RETURNING id;

-- name: FinishBackupRunOK :exec
-- Close a run as succeeded: stamp finished_at, size, and the storage key of the
-- dump. Only ever applied to the run's own 'running' row.
UPDATE ops_backup_runs
SET status      = 'ok',
    finished_at = now(),
    size_bytes  = $2,
    storage_key = $3
WHERE id = $1;

-- name: FinishBackupRunFailed :exec
-- Close a run as failed with a human-readable reason. The next night's run is
-- unaffected — a failed row is terminal, never a wedged 'running' (P0.2 AC).
UPDATE ops_backup_runs
SET status      = 'failed',
    finished_at = now(),
    error       = $2
WHERE id = $1;

-- name: GetLastRun :one
-- The most recently started run of any status — the `last_run` block on
-- /ops/status (P0.5).
SELECT * FROM ops_backup_runs
ORDER BY started_at DESC
LIMIT 1;

-- name: GetLastSuccessfulRun :one
-- The most recent succeeded run — drives last_success_at / hours_since_success.
SELECT * FROM ops_backup_runs
WHERE status = 'ok'
ORDER BY finished_at DESC
LIMIT 1;

-- name: GetLastCompletedRun :one
-- The most recent COMPLETED run (ok or failed; a still-'running' row is not
-- completed). Its status decides the `failed` precedence branch of the sentinel.
SELECT * FROM ops_backup_runs
WHERE status IN ('ok', 'failed')
ORDER BY finished_at DESC
LIMIT 1;

-- name: ListSuccessfulRuns :many
-- Every succeeded run's storage key + completion time, newest first — the
-- candidate set the retention keep/prune pass runs over (P0.2 step 5).
SELECT storage_key, finished_at FROM ops_backup_runs
WHERE status = 'ok' AND storage_key IS NOT NULL
ORDER BY finished_at DESC;
