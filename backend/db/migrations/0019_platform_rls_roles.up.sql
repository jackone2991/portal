-- 0019_platform_rls_roles: the two application DB roles for RLS (ADR-07 Phase 1,
-- Increment 2). **INERT** until a binary's DATABASE_URL switches to portal_app
-- (the cutover, a later increment). The app currently connects as the superuser
-- `portal`, which bypasses RLS — so creating these roles changes nothing at runtime.
--
--   portal_app  — NOSUPERUSER NOBYPASSRLS. The API/worker connect as this once RLS
--                 is enforced. FORCE RLS applies to it because it does NOT own the
--                 tables (owned by the migration role `portal`).
--   portal_sys  — NOSUPERUSER BYPASSRLS. Cross-tenant maintenance (cmd/sysjobs),
--                 depguard-isolated; sees every tenant's rows.
--
-- Passwords are dev placeholders (matching the .env `change-me` convention). Set
-- real ones out-of-band before the cutover. DO blocks make CREATE ROLE idempotent
-- (it has no IF NOT EXISTS).

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'portal_app') THEN
        CREATE ROLE portal_app LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE
            PASSWORD 'change-me-portal-app';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'portal_sys') THEN
        CREATE ROLE portal_sys LOGIN NOSUPERUSER BYPASSRLS NOCREATEDB NOCREATEROLE
            PASSWORD 'change-me-portal-sys';
    END IF;
END
$$;

-- A fresh NOSUPERUSER role has NO privileges by default. Without these grants the
-- cutover would 'permission denied' on every query. Grant on the existing objects
-- now; ALTER DEFAULT PRIVILEGES covers objects created by later migrations (which
-- run as the owner `portal`).
GRANT USAGE ON SCHEMA public TO portal_app, portal_sys;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO portal_app, portal_sys;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO portal_app, portal_sys;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO portal_app, portal_sys;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO portal_app, portal_sys;
