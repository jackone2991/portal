-- 0005_platform_audit: append-only security audit log.
-- Owning module: platform (moved out of account per D-25). Records login/logout,
-- role grant/revoke, permission changes, refresh-token reuse, etc. Best-effort
-- writes — a failed insert must never block the user request.

CREATE TABLE audit_log (
    id          BIGSERIAL PRIMARY KEY,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    actor_kind  TEXT NOT NULL DEFAULT 'user',         -- 'user' | 'system' | 'service'
    action      TEXT NOT NULL,                        -- 'auth.login', 'rbac.role.granted', etc.
    target_kind TEXT,                                 -- 'user' | 'role' | 'asset' | ...
    target_id   TEXT,                                 -- stringified ID of target
    metadata    JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip          INET,
    user_agent  TEXT
);

CREATE INDEX audit_log_occurred_idx ON audit_log(occurred_at DESC);
CREATE INDEX audit_log_actor_idx ON audit_log(actor_id, occurred_at DESC);
CREATE INDEX audit_log_action_idx ON audit_log(action, occurred_at DESC);
