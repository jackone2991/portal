ALTER TABLE comic_sync_sources DROP CONSTRAINT comic_sync_sources_last_status_check;
ALTER TABLE comic_sync_sources ADD CONSTRAINT comic_sync_sources_last_status_check
    CHECK (last_status IN ('idle', 'syncing', 'done', 'failed'));
