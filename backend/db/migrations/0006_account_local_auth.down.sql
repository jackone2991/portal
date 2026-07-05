-- Down: restore the OIDC-era shape. NOTE: re-adding NOT NULL on oidc_subject
-- will FAIL if any local (password-only) users exist with a NULL subject —
-- delete or backfill them first. This is intentional: rolling back local auth
-- is a schema regression, not a routine step.

CREATE TABLE user_oidc_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_oidc_roles_user_idx ON user_oidc_roles(user_id);

ALTER TABLE users ALTER COLUMN oidc_subject SET NOT NULL;

ALTER TABLE users DROP COLUMN password_updated_at;
ALTER TABLE users DROP COLUMN password_hash;
