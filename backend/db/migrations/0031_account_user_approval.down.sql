-- Down: drop the approval gate. Every account becomes usable again by virtue of
-- the column disappearing — the login/middleware checks that read it go with the
-- code, so there is no half-state to clean up.

DROP INDEX IF EXISTS users_approval_pending_idx;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_approval_status_check;

ALTER TABLE users
    DROP COLUMN IF EXISTS approved_by,
    DROP COLUMN IF EXISTS approved_at,
    DROP COLUMN IF EXISTS approval_note,
    DROP COLUMN IF EXISTS approval_status;

-- role_permissions rows cascade from the permission delete.
DELETE FROM permissions WHERE code = 'users:approve';
