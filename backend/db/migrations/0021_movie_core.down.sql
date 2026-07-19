-- Down: drop the movie vertical + its permission catalog.
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code LIKE 'movies:%');
DELETE FROM permissions WHERE code LIKE 'movies:%';
DROP TABLE IF EXISTS movies;
