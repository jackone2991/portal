-- 0029_comic_sync_batch: overall scrape progress on a sync source (SPEC-02 P1.8).
-- A whole-comic sync now scrapes in batches (each batch = a separate comic_imports
-- job), so per-import counters no longer represent the sync as a whole. These
-- columns track the sync's chapter-level progress; a per-chapter failure summary is
-- kept in the existing last_error.
ALTER TABLE comic_sync_sources ADD COLUMN total_chapters   INT NOT NULL DEFAULT 0;
ALTER TABLE comic_sync_sources ADD COLUMN scraped_chapters INT NOT NULL DEFAULT 0;
