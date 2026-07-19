-- Down: drop the app roles. DROP OWNED BY first revokes every grant + default
-- privilege referencing the role (they own no objects — tables are owned by
-- `portal`), so DROP ROLE then succeeds. Guarded so it is safe if never applied.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'portal_app') THEN
        EXECUTE 'DROP OWNED BY portal_app';
        EXECUTE 'DROP ROLE portal_app';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'portal_sys') THEN
        EXECUTE 'DROP OWNED BY portal_sys';
        EXECUTE 'DROP ROLE portal_sys';
    END IF;
END
$$;
