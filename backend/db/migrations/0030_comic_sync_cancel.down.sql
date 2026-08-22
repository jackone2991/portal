-- Rollback of 0030. The CHECK re-added below only permits the pre-0030 status set,
-- so any row still holding 'cancelled' has to be moved first — otherwise ADD
-- CONSTRAINT fails and the whole rollback aborts, which it did after any single use
-- of the Stop button.
--
-- 'idle' is the closest pre-0030 meaning: 0030's up migration introduced 'cancelled'
-- precisely so that stopping a sync would NOT read as an error, so folding these
-- rows into 'failed' would invert that intent. Chapters already imported are
-- untouched either way.
UPDATE comic_sync_sources SET last_status = 'idle' WHERE last_status = 'cancelled';

ALTER TABLE comic_sync_sources DROP CONSTRAINT comic_sync_sources_last_status_check;
ALTER TABLE comic_sync_sources ADD CONSTRAINT comic_sync_sources_last_status_check
    CHECK (last_status IN ('idle', 'syncing', 'done', 'failed'));
