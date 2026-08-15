-- 0027_comic_import_comic_level: allow comic-level zip imports (SPEC-02 P1.7,
-- multi-chapter). A comic-level job has chapter_id NULL — the worker groups the
-- zip's images by their top-level folder and creates one chapter per folder.
ALTER TABLE comic_imports ALTER COLUMN chapter_id DROP NOT NULL;
