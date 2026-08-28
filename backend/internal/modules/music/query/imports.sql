-- Bulk track import jobs (0038). The client creates a job, PUTs the zip, then
-- polls this row for status + the per-file report.

-- name: CreateMusicImport :one
INSERT INTO music_imports (owner_user_id) VALUES ($1)
RETURNING id, owner_user_id, status, upload_ref, total, succeeded, failed, report, error, created_at, updated_at;

-- name: GetMusicImport :one
SELECT id, owner_user_id, status, upload_ref, total, succeeded, failed, report, error, created_at, updated_at
FROM music_imports WHERE id = $1;

-- name: SetMusicImportUpload :one
-- Marks the zip stored. `uploaded` (not `processing`) because the worker has not
-- picked the job up yet — the client showing "đang xử lý" before anything is
-- running would be a lie it has to take back.
UPDATE music_imports
SET upload_ref = sqlc.arg('upload_ref'),
    status     = 'uploaded',
    updated_at = now()
WHERE id = sqlc.arg('id')
RETURNING id, owner_user_id, status, upload_ref, total, succeeded, failed, report, error, created_at, updated_at;

-- name: StartMusicImport :exec
UPDATE music_imports
SET status = 'processing', total = sqlc.arg('total'), updated_at = now()
WHERE id = sqlc.arg('id');

-- name: FinishMusicImport :exec
-- report is cast text->jsonb so sqlc types the param as a Go string: the pool runs
-- QueryExecModeExec, where pgx picks the wire OID from the Go type without
-- describing params, and a []byte goes out as bytea which jsonb rejects (22P02).
UPDATE music_imports
SET status     = sqlc.arg('status')::text,
    succeeded  = sqlc.arg('succeeded'),
    failed     = sqlc.arg('failed'),
    report     = sqlc.arg('report')::text::jsonb,
    error      = sqlc.narg('error'),
    updated_at = now()
WHERE id = sqlc.arg('id');

-- name: ListMusicImports :many
-- Recent jobs for the owner, so the UI can show an import that is still running
-- after a page reload.
SELECT id, owner_user_id, status, upload_ref, total, succeeded, failed, report, error, created_at, updated_at
FROM music_imports
WHERE owner_user_id = $1
ORDER BY created_at DESC
LIMIT $2;
