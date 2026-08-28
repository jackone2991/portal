-- people module queries (SPEC-08). sqlc input only — regenerate with `make sqlc`.
-- Owner-scoped. Birthday is month/day + optional year; the notices table is the
-- daily scan's outbox/dedup (P0.4).

-- ══ Persons ═════════════════════════════════════════════════════════════

-- jsonb params are cast text->jsonb so sqlc types them as Go strings. The pool
-- runs QueryExecModeExec (platform/db): pgx picks the wire OID from the Go type
-- without describing params, so a []byte goes out as bytea and the jsonb column
-- rejects it with SQLSTATE 22P02. The cast in SQL alone does NOT fix it — only
-- the Go param type does; the cast is here to make sqlc emit `string`.
-- name: CreatePerson :one
INSERT INTO people_persons (
    user_id, display_name, relationship, birth_month, birth_day, birth_year, birth_calendar, contact, note_md,
    circle, linked_user_id
) VALUES (
    $1, $2, sqlc.narg('relationship'), sqlc.narg('birth_month'), sqlc.narg('birth_day'),
    sqlc.narg('birth_year'), COALESCE(sqlc.narg('birth_calendar'), 'solar'), COALESCE(sqlc.narg('contact')::text::jsonb, '{}'::jsonb), sqlc.narg('note_md'),
    COALESCE(sqlc.narg('circle'), 'other'), sqlc.narg('linked_user_id')
)
-- ON CONFLICT rather than letting 0035's (user_id, linked_user_id) unique raise:
-- a violation aborts the request's tenant transaction, so the 409 the handler
-- writes would be replaced by a 500 at COMMIT. No rows means already added.
ON CONFLICT (user_id, linked_user_id) WHERE linked_user_id IS NOT NULL DO NOTHING
RETURNING *;

-- name: GetPerson :one
SELECT * FROM people_persons WHERE id = $1 AND user_id = $2;

-- name: ListPeople :many
-- circle is an optional filter: NULL means every circle (the People page's
-- default), a value means one section (what a section's manage link opens).
SELECT * FROM people_persons
WHERE user_id = @user_id
  AND (sqlc.narg('circle')::text IS NULL OR circle = sqlc.narg('circle')::text)
  AND ( @cursor_name::text IS NULL
        OR display_name > @cursor_name::text
        OR (display_name = @cursor_name::text AND id > @cursor_id::uuid) )
ORDER BY display_name, id
LIMIT @lim::int;

-- name: UpdatePerson :one
-- Partial update. Birthday travels as a group: set_birthday rewrites all four
-- columns (to the given values, or NULL to clear). relationship/note clear via
-- their own set flags; display_name/contact use COALESCE (leave when NULL).
UPDATE people_persons SET
    display_name   = COALESCE(sqlc.narg('display_name'), display_name),
    relationship   = CASE WHEN @set_relationship::boolean THEN sqlc.narg('relationship') ELSE relationship END,
    note_md        = CASE WHEN @set_note::boolean THEN sqlc.narg('note_md') ELSE note_md END,
    contact        = COALESCE(sqlc.narg('contact')::text::jsonb, contact),
    circle         = COALESCE(sqlc.narg('circle'), circle),
    birth_month    = CASE WHEN @set_birthday::boolean THEN sqlc.narg('birth_month') ELSE birth_month END,
    birth_day      = CASE WHEN @set_birthday::boolean THEN sqlc.narg('birth_day') ELSE birth_day END,
    birth_year     = CASE WHEN @set_birthday::boolean THEN sqlc.narg('birth_year') ELSE birth_year END,
    birth_calendar = CASE WHEN @set_birthday::boolean THEN COALESCE(sqlc.narg('birth_calendar'), 'solar') ELSE birth_calendar END,
    updated_at     = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: ListLinkedUserIDs :many
-- Portal accounts already in the caller's registry. The suggestion list
-- subtracts these, so someone you added stops being suggested.
SELECT linked_user_id FROM people_persons
WHERE user_id = $1 AND linked_user_id IS NOT NULL;

-- name: DeletePerson :one
DELETE FROM people_persons WHERE id = $1 AND user_id = $2 RETURNING id;

-- name: ListSolarBirthdays :many
-- The caller's people that have a (solar) birthday — for the upcoming endpoint.
SELECT id, display_name, birth_month, birth_day, birth_year
FROM people_persons
WHERE user_id = $1 AND birth_month IS NOT NULL AND birth_calendar = 'solar';

-- name: AllSolarBirthdays :many
-- Every user's people with a solar birthday — the daily scan iterates this.
SELECT id, user_id, display_name, birth_month, birth_day, birth_year
FROM people_persons
WHERE birth_month IS NOT NULL AND birth_calendar = 'solar';

-- ══ Birthday notices — outbox/dedup (P0.4) ══════════════════════════════

-- name: InsertNotice :exec
-- Reserve the (person, year, threshold) slot; a conflict means it already fired.
INSERT INTO people_birthday_notices (person_id, year, threshold)
VALUES ($1, $2, $3)
ON CONFLICT (person_id, year, threshold) DO NOTHING;

-- name: PendingNotices :many
-- Notices written but not yet published (fresh + prior-failed) with their person.
SELECT n.id, n.threshold, n.year, p.id AS person_id, p.user_id, p.display_name
FROM people_birthday_notices n
JOIN people_persons p ON p.id = n.person_id
WHERE n.emitted_at IS NULL;

-- name: MarkNoticeEmitted :exec
UPDATE people_birthday_notices SET emitted_at = now() WHERE id = $1;

-- name: DeleteFutureNotices :exec
-- On a birthday edit/clear, drop current + future notices so a corrected date
-- can fire fresh this year (P0.2). Past years' history stays.
DELETE FROM people_birthday_notices WHERE person_id = $1 AND year >= $2;
