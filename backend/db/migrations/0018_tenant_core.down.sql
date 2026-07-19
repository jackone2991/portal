-- Down: drop the tenancy control-plane tables (memberships first — FK to orgs).
DROP TABLE IF EXISTS organization_memberships;
DROP TABLE IF EXISTS organizations;
