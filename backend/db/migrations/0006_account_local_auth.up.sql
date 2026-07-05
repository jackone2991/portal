-- 0006_account_local_auth: Portal owns credentials (ADR-06).
-- Owning module: account.
--
-- Switches the login front-door from OIDC/Authentik to local password auth.
-- The token / refresh / RBAC / revocation / audit machinery is UNCHANGED — this
-- migration only adds the password store and retires the IdP-sync artefacts.

-- Argon2id password (PHC-format string, e.g. "$argon2id$v=19$m=65536,t=3,p=2$…").
-- Nullable: rows provisioned before a password is set (admin invite) have none.
ALTER TABLE users ADD COLUMN password_hash TEXT;
ALTER TABLE users ADD COLUMN password_updated_at TIMESTAMPTZ;

-- Local users have no OIDC subject. Existing OIDC-provisioned rows keep theirs;
-- new local rows leave it NULL. The UNIQUE constraint still holds — Postgres
-- treats NULLs as distinct, so many local users can coexist.
ALTER TABLE users ALTER COLUMN oidc_subject DROP NOT NULL;

-- Authentik group→role sync is retired (ADR-06); there are no IdP groups to
-- reconcile. Roles are Portal-managed only, via user_roles.
DROP INDEX IF EXISTS user_oidc_roles_user_idx;
DROP TABLE IF EXISTS user_oidc_roles;
