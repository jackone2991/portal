-- 0012_ops_backup_runs: the ops module's backup-run ledger (SPEC-09 §6) + its
-- permission seeds. System-scoped (no user_id) — backups are a platform concern.
-- The ops_exports table (P1.7 takeout) ships in a later migration with that phase.

CREATE TABLE ops_backup_runs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    status      TEXT NOT NULL DEFAULT 'running'
                  CHECK (status IN ('running', 'ok', 'failed')),
    size_bytes  BIGINT,
    storage_key TEXT,
    error       TEXT
);
CREATE INDEX ops_backup_runs_started_idx ON ops_backup_runs (started_at DESC);

-- ── seed: ops permission catalog + grants (P0.1) ──
-- ops:read / queues:read → admin; takeout:read/write:own → user. Seeding early
-- (before P1.6/P1.7 land) is harmless; an unseeded code 403s everyone.
INSERT INTO permissions (code, description) VALUES
    ('ops:read',          'Read platform ops status (backups, queues)'),
    ('queues:read',       'Inspect the Asynq queue console'),
    ('takeout:read:own',  'Read own data-export status + download'),
    ('takeout:write:own', 'Request own data export')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('admin', 'ops:read'),
    ('admin', 'queues:read'),
    ('user',  'takeout:read:own'),
    ('user',  'takeout:write:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
