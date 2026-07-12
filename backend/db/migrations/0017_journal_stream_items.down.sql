-- Reverse 0017_journal_stream_items.
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code = 'stream:read:own'
);
DELETE FROM permissions WHERE code = 'stream:read:own';
DROP TABLE IF EXISTS stream_items;
