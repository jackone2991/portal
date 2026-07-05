-- 0002_account_users: the users table (identity core).
-- Owning module: account. Roles/RBAC land in 0003; refresh tokens in 0004.
--
-- token_version + disabled_at are folded into the CREATE here (previously an
-- ALTER in the old 0002_account_rbac). locale + timezone are added for the
-- frontend (BCP-47 / IANA). `role` is a legacy denormalized label kept because
-- GetUserAuthSnapshot still selects it — authorization is via user_roles (0003),
-- NOT this column.

CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    oidc_subject    TEXT UNIQUE NOT NULL,          -- Authentik 'sub' claim
    email           TEXT UNIQUE NOT NULL,
    display_name    TEXT NOT NULL,
    avatar_url      TEXT,
    role            TEXT NOT NULL DEFAULT 'user',  -- legacy label; real authz = user_roles (0003)
    locale          TEXT NOT NULL DEFAULT 'en-US', -- BCP-47; frontend locale
    timezone        TEXT NOT NULL DEFAULT 'UTC',   -- IANA tz; frontend date/time formatting
    token_version   INTEGER NOT NULL DEFAULT 1,    -- bump to invalidate all access tokens (logout-all)
    disabled_at     TIMESTAMPTZ,                   -- soft-disable; auth rejects when not null
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
