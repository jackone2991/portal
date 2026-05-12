-- 0002_rbac: hierarchical RBAC + auth machinery.
--
-- Tables added:
--   roles               — role catalog with parent_id for hierarchy
--   permissions         — permission registry (resource:action[:scope])
--   role_permissions    — role ↔ permission grants
--   user_roles          — user ↔ role assignments (auditable, expirable)
--   refresh_tokens      — hashed refresh tokens with rotation chain
--   audit_log           — security-sensitive events
--
-- Columns added to users:
--   token_version       — bump to invalidate all outstanding access tokens
--   disabled_at         — soft-disable (auth checks reject if not null)

-- ── users: revocation + lifecycle columns ─────────────────────────
ALTER TABLE users
    ADD COLUMN token_version INTEGER NOT NULL DEFAULT 1,
    ADD COLUMN disabled_at   TIMESTAMPTZ;

-- ── roles ─────────────────────────────────────────────────────────
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        TEXT UNIQUE NOT NULL,                -- 'user', 'admin', etc.
    name        TEXT NOT NULL,                       -- human-readable label
    description TEXT,
    parent_id   UUID REFERENCES roles(id) ON DELETE SET NULL,
    is_system   BOOLEAN NOT NULL DEFAULT false,      -- system roles cannot be deleted
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Self-reference guard. Cycle detection across multiple hops is enforced in app code.
    CONSTRAINT roles_no_self_parent CHECK (parent_id IS DISTINCT FROM id)
);

CREATE INDEX roles_parent_idx ON roles(parent_id);

-- ── permissions ───────────────────────────────────────────────────
CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        TEXT UNIQUE NOT NULL,                -- 'movies:read', 'assets:delete:own'
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Permission code grammar: <resource>:<action>[:<scope>]   (lowercase, alphanumerics, '-', '_', '*')
    CONSTRAINT permissions_code_format CHECK (
        code ~ '^[a-z0-9_*-]+(:[a-z0-9_*-]+){1,2}$' OR code = '*'
    )
);

-- ── role_permissions ──────────────────────────────────────────────
CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by    UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (role_id, permission_id)
);

-- ── user_roles ────────────────────────────────────────────────────
CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_roles_user_idx ON user_roles(user_id);

-- ── refresh_tokens ────────────────────────────────────────────────
-- Tokens are stored hashed (SHA-256). The plaintext is only ever returned to
-- the client at issue time and never persisted.
--
-- Rotation chain: when a token is exchanged for a new one, the old row's
-- replaced_by_id is set, and revoked_at is stamped. Presenting an already-
-- replaced token is treated as theft — the entire chain is revoked.
CREATE TABLE refresh_tokens (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      BYTEA NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ,
    revoke_reason   TEXT,
    replaced_by_id  UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    parent_id       UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL,
    -- Audit: bind token to client environment so anomalous reuse can be detected.
    issued_ip       INET,
    issued_user_agent TEXT
);

CREATE UNIQUE INDEX refresh_tokens_hash_idx ON refresh_tokens(token_hash);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens(user_id) WHERE revoked_at IS NULL;
CREATE INDEX refresh_tokens_expiry_idx ON refresh_tokens(expires_at) WHERE revoked_at IS NULL;

-- ── audit_log ─────────────────────────────────────────────────────
-- Append-only. Records every security-sensitive event: login, logout,
-- role grant/revoke, permission change, refresh-token reuse, etc.
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

-- ── seed: permission catalog ──────────────────────────────────────
-- Convention: <resource>:<action>[:<scope>]
--   scope = 'own' (only resources owned by actor) or 'any' (organization-wide)
INSERT INTO permissions (code, description) VALUES
    -- meta
    ('*',                       'Wildcard — all permissions (superadmin only)'),
    -- profile / self
    ('profile:read',            'Read own profile'),
    ('profile:write',           'Update own profile'),
    -- assets (uploads)
    ('assets:read:own',         'Read own assets'),
    ('assets:read:any',         'Read any asset'),
    ('assets:write:own',        'Upload + edit own assets'),
    ('assets:write:any',        'Edit any asset'),
    ('assets:delete:own',       'Delete own assets'),
    ('assets:delete:any',       'Delete any asset'),
    -- movies / music / stories: same shape
    ('movies:read',             'Read public movies'),
    ('movies:write:own',        'Create + edit own movies'),
    ('movies:write:any',        'Edit any movie'),
    ('movies:publish',          'Publish movies (visible to public)'),
    ('movies:delete:any',       'Delete any movie'),
    ('music:read',              'Read public music'),
    ('music:write:own',         'Create + edit own music'),
    ('music:write:any',         'Edit any music'),
    ('music:publish',           'Publish music'),
    ('music:delete:any',        'Delete any music'),
    ('stories:read',            'Read public stories'),
    ('stories:write:own',       'Create + edit own stories'),
    ('stories:write:any',       'Edit any story'),
    ('stories:publish',         'Publish stories'),
    ('stories:delete:any',      'Delete any story'),
    -- comments
    ('comments:write',          'Post comments'),
    ('comments:delete:own',     'Delete own comments'),
    ('comments:delete:any',     'Delete any comment (moderation)'),
    -- moderation
    ('moderation:flag',         'Flag content for review'),
    ('moderation:hide',         'Hide content from public'),
    ('moderation:ban_user',     'Ban a user account'),
    -- user / role administration
    ('users:read:any',          'List + view any user'),
    ('users:write:any',         'Edit any user (email, profile, status)'),
    ('users:delete:any',        'Delete any user'),
    ('rbac:role:read',          'View roles and permissions'),
    ('rbac:role:write',         'Create + edit roles'),
    ('rbac:role:assign',        'Assign / revoke roles on users'),
    -- audit
    ('audit:read',              'Read the audit log'),
    -- system
    ('system:settings:write',   'Modify system-wide settings');

-- ── seed: role hierarchy ──────────────────────────────────────────
-- Hierarchy (child inherits parent's permissions):
--
--   superadmin
--   └── admin
--       └── moderator
--           └── editor
--               └── creator
--                   └── user
--                       └── guest
--
-- Inserted child→parent in two passes so parent_id can be set after creation.

INSERT INTO roles (code, name, description, is_system) VALUES
    ('guest',      'Guest',       'Anonymous visitor. Public reads only.',                 true),
    ('user',       'User',        'Authenticated user with profile and comments.',          true),
    ('creator',    'Creator',     'Can upload and manage their own media.',                 true),
    ('editor',     'Editor',      'Can curate and edit any content (publish library).',     true),
    ('moderator',  'Moderator',   'Can hide content, delete comments, ban users.',          true),
    ('admin',      'Admin',       'Manages users, roles, system settings.',                 true),
    ('superadmin', 'Super Admin', 'Unrestricted access. Use sparingly.',                    true);

UPDATE roles SET parent_id = (SELECT id FROM roles WHERE code = 'guest')      WHERE code = 'user';
UPDATE roles SET parent_id = (SELECT id FROM roles WHERE code = 'user')       WHERE code = 'creator';
UPDATE roles SET parent_id = (SELECT id FROM roles WHERE code = 'creator')    WHERE code = 'editor';
UPDATE roles SET parent_id = (SELECT id FROM roles WHERE code = 'editor')     WHERE code = 'moderator';
UPDATE roles SET parent_id = (SELECT id FROM roles WHERE code = 'moderator')  WHERE code = 'admin';
UPDATE roles SET parent_id = (SELECT id FROM roles WHERE code = 'admin')      WHERE code = 'superadmin';

-- ── seed: role → permission grants ────────────────────────────────
-- Helper macro: grant(role_code, perm_codes...)
WITH grants(role_code, perm_code) AS (VALUES
    -- guest: nothing extra; public reads happen without a role check
    ('guest',     'movies:read'),
    ('guest',     'music:read'),
    ('guest',     'stories:read'),

    -- user: profile + comments + own asset reads
    ('user',      'profile:read'),
    ('user',      'profile:write'),
    ('user',      'assets:read:own'),
    ('user',      'comments:write'),
    ('user',      'comments:delete:own'),

    -- creator: own uploads + own publish-pipeline
    ('creator',   'assets:write:own'),
    ('creator',   'assets:delete:own'),
    ('creator',   'movies:write:own'),
    ('creator',   'music:write:own'),
    ('creator',   'stories:write:own'),

    -- editor: write/publish any content + read any asset
    ('editor',    'assets:read:any'),
    ('editor',    'assets:write:any'),
    ('editor',    'movies:write:any'),
    ('editor',    'movies:publish'),
    ('editor',    'music:write:any'),
    ('editor',    'music:publish'),
    ('editor',    'stories:write:any'),
    ('editor',    'stories:publish'),

    -- moderator: moderation + delete comments
    ('moderator', 'comments:delete:any'),
    ('moderator', 'moderation:flag'),
    ('moderator', 'moderation:hide'),
    ('moderator', 'moderation:ban_user'),

    -- admin: user/role administration + delete content + audit
    ('admin',     'users:read:any'),
    ('admin',     'users:write:any'),
    ('admin',     'users:delete:any'),
    ('admin',     'assets:delete:any'),
    ('admin',     'movies:delete:any'),
    ('admin',     'music:delete:any'),
    ('admin',     'stories:delete:any'),
    ('admin',     'rbac:role:read'),
    ('admin',     'rbac:role:write'),
    ('admin',     'rbac:role:assign'),
    ('admin',     'audit:read'),
    ('admin',     'system:settings:write'),

    -- superadmin: wildcard (covers everything, including future perms)
    ('superadmin', '*')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
