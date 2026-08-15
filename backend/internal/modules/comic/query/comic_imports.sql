-- comic import-job queries (SPEC-02 P1.7). sqlc input only.

-- name: CreateImport :one
INSERT INTO comic_imports (comic_id, chapter_id, owner_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: CreateComicImport :one
-- Comic-level (multi-chapter) job: chapter_id NULL — the worker creates chapters.
INSERT INTO comic_imports (comic_id, owner_user_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetImport :one
SELECT * FROM comic_imports WHERE id = $1;

-- name: SetImportUpload :one
UPDATE comic_imports
SET upload_ref = $2, status = 'uploaded', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: StartImport :exec
UPDATE comic_imports
SET status = 'processing', total = $2, updated_at = now()
WHERE id = $1;

-- name: UpdateImportProgress :exec
-- report is cast ::jsonb — under QueryExecModeExec pgx sends []byte untyped, so the
-- explicit cast is what makes Postgres parse it as json (same as the journal stream).
UPDATE comic_imports
SET succeeded = $2, failed = $3, report = sqlc.arg('report')::jsonb, updated_at = now()
WHERE id = $1;

-- name: FinishImport :exec
UPDATE comic_imports
SET status = $2, succeeded = $3, failed = $4, report = sqlc.arg('report')::jsonb, error = sqlc.narg('error'), updated_at = now()
WHERE id = $1;
