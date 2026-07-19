-- Down: drop the music vertical + its permission catalog.
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code LIKE 'music:%');
DELETE FROM permissions WHERE code LIKE 'music:%';
DROP TABLE IF EXISTS music_tracks;
