DROP TABLE IF EXISTS media_asset_variants;
DROP INDEX IF EXISTS assets_owner_cursor_idx;
DROP INDEX IF EXISTS assets_deleting_idx;
ALTER TABLE assets DROP COLUMN IF EXISTS origin;
ALTER TABLE assets DROP COLUMN IF EXISTS original_filename;
ALTER TABLE assets DROP COLUMN IF EXISTS title;
ALTER TABLE assets DROP CONSTRAINT assets_status_chk;
ALTER TABLE assets ADD CONSTRAINT assets_status_chk
    CHECK (status IN ('uploading', 'processing', 'ready', 'failed'));
