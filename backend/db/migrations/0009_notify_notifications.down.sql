DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code IN (
        'notifications:read:own', 'notifications:write:own',
        'notification-prefs:read:own', 'notification-prefs:write:own',
        'push-subscriptions:write:own', 'push-subscriptions:delete:own'
    )
);
DELETE FROM permissions WHERE code IN (
    'notifications:read:own', 'notifications:write:own',
    'notification-prefs:read:own', 'notification-prefs:write:own',
    'push-subscriptions:write:own', 'push-subscriptions:delete:own'
);
DROP TABLE IF EXISTS web_push_subscriptions;
DROP TABLE IF EXISTS notification_preferences;
DROP TABLE IF EXISTS notifications;
