-- 0003_account_rbac: hierarchical RBAC.
-- Owning module: account. Tables: roles (adjacency-list hierarchy via parent_id),
-- permissions, role_permissions, user_roles, user_oidc_roles.
--
-- Effective permissions = recursive walk of roles.parent_id, unioned across a
-- user's user_roles (+ user_oidc_roles once OIDC group sync writes to it, per D-26).

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
    -- Self-reference guard. Multi-hop cycle detection is enforced in app code.
    CONSTRAINT roles_no_self_parent CHECK (parent_id IS DISTINCT FROM id)
);

CREATE INDEX roles_parent_idx ON roles(parent_id);

-- ── permissions ───────────────────────────────────────────────────
CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        TEXT UNIQUE NOT NULL,                -- 'movies:read', 'assets:delete:own'
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Grammar: <resource>:<action>[:<scope>]   (lowercase, alphanumerics, '-', '_', '*')
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

-- ── user_roles (Portal-managed assignments) ───────────────────────
CREATE TABLE user_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by  UUID REFERENCES users(id) ON DELETE SET NULL,
    expires_at  TIMESTAMPTZ,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_roles_user_idx ON user_roles(user_id);

-- ── user_oidc_roles (synced from Authentik groups, per D-26) ───────
-- Reconciled on every OIDC callback via OIDC_GROUP_ROLE_MAP. Kept separate from
-- user_roles so Portal-managed grants and IdP-synced grants don't clobber each
-- other. Effective permissions union both (query update lands with the sync code).
CREATE TABLE user_oidc_roles (
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    synced_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX user_oidc_roles_user_idx ON user_oidc_roles(user_id);

-- ── seed: permission catalog ──────────────────────────────────────
-- Convention: <resource>:<action>[:<scope>]  (scope = 'own' | 'any')
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
--   superadmin → admin → moderator → editor → creator → user → guest
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
WITH grants(role_code, perm_code) AS (VALUES
    -- guest: public reads
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
