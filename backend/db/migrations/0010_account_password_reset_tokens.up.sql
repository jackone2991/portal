-- 0010_account_password_reset_tokens: single-use, hashed-at-rest password-reset
-- tokens (SPEC-04 P0.3). Account-owned (per-module migration ownership — it can't
-- ride the notify migration). Mirrors ADR-06's refresh-token construction:
-- ≥256-bit CSPRNG raw token, SHA-256 hash at rest, looked up by hash in
-- constant time, short TTL, consumed on use.

CREATE TABLE password_reset_tokens (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,         -- SHA-256 of the raw token
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens (user_id);
