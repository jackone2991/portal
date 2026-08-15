-- Revert: remove the base `user` role's own-comic write/publish grants (0015 keeps
-- them on `creator`).
DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE code = 'user')
  AND permission_id IN (SELECT id FROM permissions WHERE code IN ('comics:write:own', 'comics:publish:own'));
