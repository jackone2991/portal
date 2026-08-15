-- 0025_comic_user_write_grant: life-OS direction — every authenticated user creates
-- and publishes their OWN comics, not just the `creator` role. 0015 granted
-- comics:write:own / comics:publish:own to `creator`; this widens them to the base
-- `user` role (the role hierarchy means creator/editor/… still inherit). Moderation
-- (:any) and delete stay creator+/admin-only. Idempotent.
WITH grants(role_code, perm_code) AS (VALUES
    ('user', 'comics:write:own'),
    ('user', 'comics:publish:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;
