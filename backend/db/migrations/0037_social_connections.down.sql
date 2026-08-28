DROP TABLE IF EXISTS social_connections;
DROP FUNCTION IF EXISTS app_current_user();
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN ('social:read:own', 'social:write:own')
);
DELETE FROM permissions WHERE code IN ('social:read:own', 'social:write:own');
