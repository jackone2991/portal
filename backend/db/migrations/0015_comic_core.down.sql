-- Reverse 0015_comic_core.
DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code LIKE 'comics:%'
);
DELETE FROM permissions WHERE code LIKE 'comics:%';

DROP TABLE IF EXISTS comic_reading_progress;
DROP TABLE IF EXISTS comic_pages;
DROP TABLE IF EXISTS comic_chapters;
DROP TABLE IF EXISTS comics;
