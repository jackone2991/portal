-- 0004_account_sessions: refresh tokens with rotation-chain theft detection.
-- Owning module: account.
--
-- Tokens are stored hashed (SHA-256); plaintext is returned to the client once at
-- issue time and never persisted. On rotation the old row's replaced_by_id is set
-- and revoked_at is stamped; presenting an already-replaced token is treated as
-- theft and the whole chain is revoked (see RevokeRefreshTokenChain).

CREATE TABLE refresh_tokens (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash        BYTEA NOT NULL,
    expires_at        TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at        TIMESTAMPTZ,
    revoke_reason     TEXT,
    replaced_by_id    UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    parent_id         UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    -- Bind token to client environment so anomalous reuse can be detected.
    issued_ip         INET,
    issued_user_agent TEXT
);

CREATE UNIQUE INDEX refresh_tokens_hash_idx ON refresh_tokens(token_hash);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_expiry_idx ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;
