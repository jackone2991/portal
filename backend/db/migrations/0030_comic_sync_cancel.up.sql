-- 0030_comic_sync_cancel: allow a 'cancelled' sync status (SPEC-02 P1.8) so the UI's
-- "Ngừng đồng bộ" button can stop a running sync without it reading as an error.
ALTER TABLE comic_sync_sources DROP CONSTRAINT comic_sync_sources_last_status_check;
ALTER TABLE comic_sync_sources ADD CONSTRAINT comic_sync_sources_last_status_check
    CHECK (last_status IN ('idle', 'syncing', 'done', 'failed', 'cancelled'));
