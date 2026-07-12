DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('ops:read', 'queues:read', 'takeout:read:own', 'takeout:write:own')
);
DELETE FROM permissions WHERE code IN ('ops:read', 'queues:read', 'takeout:read:own', 'takeout:write:own');
DROP TABLE IF EXISTS ops_backup_runs;
