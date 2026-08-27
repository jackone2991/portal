-- Tenant control-plane queries (ADR-07 Phase 1). These tables are NOT
-- tenant-scoped (no RLS), so they run on the pool directly during tenant
-- resolution — before any BeginTenantScope tx exists.

-- name: GetOrganizationByID :one
SELECT id, kind, slug, name, owner_id, created_at, updated_at
FROM organizations
WHERE id = $1;

-- name: GetOrganizationBySlug :one
SELECT id, kind, slug, name, owner_id, created_at, updated_at
FROM organizations
WHERE slug = $1;

-- name: GetPersonalOrgForUser :one
SELECT id, kind, slug, name, owner_id, created_at, updated_at
FROM organizations
WHERE owner_id = $1 AND kind = 'personal';

-- name: ListOrganizationsForUser :many
SELECT o.id, o.kind, o.slug, o.name, o.owner_id, o.created_at, o.updated_at
FROM organizations o
JOIN organization_memberships m ON m.org_id = o.id
WHERE m.user_id = $1
ORDER BY o.created_at;

-- name: IsMember :one
SELECT EXISTS (
    SELECT 1 FROM organization_memberships WHERE org_id = $1 AND user_id = $2
);

-- CreatePersonalOrg is idempotent: the unique partial index
-- organizations_personal_owner_idx (0018) makes a concurrent second call a
-- no-op (ON CONFLICT DO NOTHING → zero rows → caller re-fetches).
--
-- $1 is pinned ::uuid in BOTH uses on purpose. The pool runs QueryExecModeExec
-- (platform/db), so pgx does not describe parameters and Postgres has to infer
-- $1's type from the statement itself. A bare `$1::text` for the slug pinned the
-- inference to text, and the same $1 then hit the uuid `owner_id` column —
-- `column "owner_id" is of type uuid but expression is of type text`
-- (SQLSTATE 42804), which made every first request by a new user 500 with
-- "could not resolve tenant". Casting uuid->text for the slug keeps one
-- unambiguous param type.
-- name: CreatePersonalOrg :one
INSERT INTO organizations (kind, slug, name, owner_id)
VALUES ('personal', 'personal-' || sqlc.arg('owner_id')::uuid::text,
        sqlc.arg('name'), sqlc.arg('owner_id')::uuid)
ON CONFLICT DO NOTHING
RETURNING id, kind, slug, name, owner_id, created_at, updated_at;

-- name: CreateMembership :exec
INSERT INTO organization_memberships (org_id, user_id, role)
VALUES ($1, $2, $3)
ON CONFLICT DO NOTHING;

-- ListAllOrganizationIDs returns every tenant, for the worker's cross-tenant
-- periodic sweeps. `organizations` is deliberately NOT an RLS-protected table
-- (ADR-07: the tenant registry itself must be readable to resolve a scope), so
-- this stays correct after the portal_app cutover — which is what lets a sweep
-- iterate tenants instead of needing BYPASSRLS.
-- name: ListAllOrganizationIDs :many
SELECT id FROM organizations ORDER BY created_at;
