-- Down: drop the story vertical + its permission catalog (chapters cascade).
DELETE FROM role_permissions WHERE permission_id IN (SELECT id FROM permissions WHERE code LIKE 'stories:%');
DELETE FROM permissions WHERE code LIKE 'stories:%';
DROP TABLE IF EXISTS story_chapters;
DROP TABLE IF EXISTS stories;
