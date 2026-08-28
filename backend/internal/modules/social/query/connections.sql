-- social module queries — connections between accounts (0037). sqlc input only.
--
-- These return IDS, never names. `users` belongs to the account module and this
-- module may not join it (MODULES.md); the service resolves display names
-- through account's api/ package in one batch afterwards.
--
-- Every statement is additionally fenced by the row-level policies from 0037,
-- which key on `app.current_user`. The predicates below are therefore about
-- INTENT ("the requests waiting on me"), not about isolation: a missing
-- predicate would return fewer rows, never someone else's.

-- name: CreateRequest :one
-- Send a request. The pair unique index (either direction) is what makes a
-- duplicate impossible; the caller turns that conflict into "already connected
-- or already asked" rather than a 500.
INSERT INTO social_connections (requester_id, addressee_id)
VALUES ($1, $2)
RETURNING *;

-- name: GetConnection :one
SELECT * FROM social_connections WHERE id = $1;

-- name: FindBetween :one
-- The relationship between two accounts, whichever way it was created.
SELECT * FROM social_connections
WHERE least(requester_id, addressee_id) = least(@a::uuid, @b::uuid)
  AND greatest(requester_id, addressee_id) = greatest(@a::uuid, @b::uuid);

-- name: AcceptRequest :one
-- Only a pending row moves; re-accepting an accepted one returns no rows, which
-- keeps a double-click from rewriting responded_at.
UPDATE social_connections
SET status = 'accepted', responded_at = now()
WHERE id = $1 AND addressee_id = $2 AND status = 'pending'
RETURNING *;

-- name: DeleteConnection :one
-- Withdraw, decline, or disconnect — the same statement for all three, because
-- the row means the same thing in every case: this link no longer exists.
DELETE FROM social_connections
WHERE id = $1 AND (requester_id = $2 OR addressee_id = $2)
RETURNING *;

-- name: ListAccepted :many
SELECT * FROM social_connections
WHERE status = 'accepted' AND @me::uuid IN (requester_id, addressee_id)
ORDER BY created_at DESC, id;

-- name: ListIncoming :many
-- Requests waiting on the caller's answer.
SELECT * FROM social_connections
WHERE status = 'pending' AND addressee_id = @me::uuid
ORDER BY created_at DESC, id;

-- name: ListOutgoing :many
-- Requests the caller has sent and nobody has answered yet.
SELECT * FROM social_connections
WHERE status = 'pending' AND requester_id = @me::uuid
ORDER BY created_at DESC, id;

-- name: ListCounterpartIDs :many
-- Every account the caller already has any relationship with, pending or not —
-- what the suggestion list subtracts so it stops offering people you have
-- already asked.
-- The cast is what makes sqlc type this column as uuid rather than interface{}:
-- a bare CASE has no declared type for it to read.
SELECT (CASE WHEN requester_id = @me::uuid THEN addressee_id ELSE requester_id END)::uuid AS other_id
FROM social_connections
WHERE @me::uuid IN (requester_id, addressee_id);

-- name: CountIncoming :one
-- The header badge.
SELECT count(*) FROM social_connections
WHERE status = 'pending' AND addressee_id = $1;
