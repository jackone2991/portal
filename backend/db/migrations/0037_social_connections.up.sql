-- 0037_social_connections: the social layer's first real object — a mutual link
-- between two accounts on this Portal.
--
-- NOT tenant-scoped. Every other domain table hangs off one tenant, but a
-- connection is by definition between two of them, so a `tenant_id` column would
-- have to pick a side and the other party could never see the row. Connections
-- sit at the same level as `users` itself: global, and isolated per USER rather
-- than per tenant.
--
-- That isolation is still enforced by the database, not by remembering to write
-- the right WHERE clause. The policies below read `app.current_user`, the same
-- GUC the media ACL (0032) uses, so a query that forgets its predicate returns
-- nothing instead of someone else's friend list.
--
-- Status is deliberately two values. 'declined' is not a state worth keeping: a
-- declined request is deleted, so the pair is free to try again later and
-- nobody's refusal is stored as a permanent record.

CREATE TABLE social_connections (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    requester_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at  TIMESTAMPTZ,
    CONSTRAINT social_connections_not_self CHECK (requester_id <> addressee_id)
);

-- One relationship per pair, whichever way round it was created: without this,
-- A→B and B→A are two rows and "are we connected?" has two answers.
CREATE UNIQUE INDEX social_connections_pair_idx ON social_connections
    (least(requester_id, addressee_id), greatest(requester_id, addressee_id));

CREATE INDEX social_connections_addressee_idx ON social_connections (addressee_id, status);
CREATE INDEX social_connections_requester_idx ON social_connections (requester_id, status);

-- app_current_user() keeps the policies readable. STABLE, not IMMUTABLE: the
-- value changes per transaction. The two-argument current_setting with NULLIF
-- means an unset GUC yields NULL and every policy is simply false, rather than
-- raising — an anonymous or unscoped connection sees no connections at all.
CREATE FUNCTION app_current_user() RETURNS uuid
    LANGUAGE sql STABLE AS
$$ SELECT NULLIF(current_setting('app.current_user', true), '')::uuid $$;

ALTER TABLE social_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE social_connections FORCE ROW LEVEL SECURITY;

-- Read: both parties. A pending request must be visible to the person being
-- asked, or they could never answer it.
CREATE POLICY connection_select ON social_connections FOR SELECT
    USING (app_current_user() IN (requester_id, addressee_id));

-- Send: only as yourself. You cannot forge a request from someone else.
CREATE POLICY connection_insert ON social_connections FOR INSERT
    WITH CHECK (requester_id = app_current_user());

-- Answer: only the addressee, and they cannot rewrite who it is between.
CREATE POLICY connection_update ON social_connections FOR UPDATE
    USING (addressee_id = app_current_user())
    WITH CHECK (addressee_id = app_current_user());

-- Withdraw / decline / disconnect: either party, at any time.
CREATE POLICY connection_delete ON social_connections FOR DELETE
    USING (app_current_user() IN (requester_id, addressee_id));

-- ── permissions ─────────────────────────────────────────────────────────
INSERT INTO permissions (code, description) VALUES
    ('social:read:own',  'See your own connections and requests'),
    ('social:write:own', 'Send, accept and remove your own connections')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (
    VALUES ('user', 'social:read:own'), ('user', 'social:write:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
