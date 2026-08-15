-- Best-effort: restore NOT NULL (fails if comic-level jobs with NULL chapter_id exist).
ALTER TABLE comic_imports ALTER COLUMN chapter_id SET NOT NULL;
