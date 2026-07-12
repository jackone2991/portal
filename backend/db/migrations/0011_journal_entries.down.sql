DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('journal:read:own', 'journal:write:own', 'journal:delete:own')
);
DELETE FROM permissions WHERE code IN ('journal:read:own', 'journal:write:own', 'journal:delete:own');
DROP TABLE IF EXISTS journal_entries;
