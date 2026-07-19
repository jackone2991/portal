-- 0018_tenant_core: organizations + organization_memberships (ADR-07 Phase 1,
-- Increment 1 — the tenancy control plane).
--
-- These tables are the multi-tenancy CONTROL PLANE and are NOT tenant-scoped
-- themselves (global, like `users`) — no RLS here. Adding `tenant_id` + RLS to
-- the domain tables is a later increment; nothing enforces isolation yet.
--
-- Every existing user gets a synthetic `personal` org (ADR-07 §1) so the app
-- always has a tenant to resolve from day one. `/t/me` resolves to a user's
-- personal org by owner; explicit `/t/{slug}` routing lands in a later increment.

CREATE TABLE organizations (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    kind       TEXT NOT NULL CHECK (kind IN ('org', 'household', 'personal')),
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    owner_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX organizations_owner_idx ON organizations (owner_id);

-- Exactly one personal org per user — the backfill and the signup hook both
-- rely on this to stay idempotent.
CREATE UNIQUE INDEX organizations_personal_owner_idx
    ON organizations (owner_id) WHERE kind = 'personal';

CREATE TABLE organization_memberships (
    org_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'owner',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (org_id, user_id)
);
CREATE INDEX organization_memberships_user_idx ON organization_memberships (user_id);

-- Backfill: one personal org + one owner membership per existing user.
-- Idempotent via the unique personal-owner index (ON CONFLICT DO NOTHING).
INSERT INTO organizations (kind, slug, name, owner_id)
SELECT 'personal', 'personal-' || u.id::text, u.display_name, u.id
FROM users u
ON CONFLICT DO NOTHING;

INSERT INTO organization_memberships (org_id, user_id, role)
SELECT o.id, o.owner_id, 'owner'
FROM organizations o
WHERE o.kind = 'personal'
ON CONFLICT DO NOTHING;
