-- Down: remove RLS + tenant_id from all 17 tables. Order per table: drop the
-- policy (it depends on the column), disable RLS, then drop the column — which
-- cascades the FK, index, DEFAULT and NOT NULL with it.

DROP POLICY IF EXISTS tenant_isolation ON assets;
ALTER TABLE assets DISABLE ROW LEVEL SECURITY;
ALTER TABLE assets DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON media_asset_variants;
ALTER TABLE media_asset_variants DISABLE ROW LEVEL SECURITY;
ALTER TABLE media_asset_variants DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON media_playback_progress;
ALTER TABLE media_playback_progress DISABLE ROW LEVEL SECURITY;
ALTER TABLE media_playback_progress DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON notifications;
ALTER TABLE notifications DISABLE ROW LEVEL SECURITY;
ALTER TABLE notifications DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON journal_entries;
ALTER TABLE journal_entries DISABLE ROW LEVEL SECURITY;
ALTER TABLE journal_entries DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON stream_items;
ALTER TABLE stream_items DISABLE ROW LEVEL SECURITY;
ALTER TABLE stream_items DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON bank_accounts;
ALTER TABLE bank_accounts DISABLE ROW LEVEL SECURITY;
ALTER TABLE bank_accounts DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON bank_categories;
ALTER TABLE bank_categories DISABLE ROW LEVEL SECURITY;
ALTER TABLE bank_categories DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON bank_import_batches;
ALTER TABLE bank_import_batches DISABLE ROW LEVEL SECURITY;
ALTER TABLE bank_import_batches DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON bank_transactions;
ALTER TABLE bank_transactions DISABLE ROW LEVEL SECURITY;
ALTER TABLE bank_transactions DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON bank_budgets;
ALTER TABLE bank_budgets DISABLE ROW LEVEL SECURITY;
ALTER TABLE bank_budgets DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON comics;
ALTER TABLE comics DISABLE ROW LEVEL SECURITY;
ALTER TABLE comics DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON comic_chapters;
ALTER TABLE comic_chapters DISABLE ROW LEVEL SECURITY;
ALTER TABLE comic_chapters DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON comic_pages;
ALTER TABLE comic_pages DISABLE ROW LEVEL SECURITY;
ALTER TABLE comic_pages DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON comic_reading_progress;
ALTER TABLE comic_reading_progress DISABLE ROW LEVEL SECURITY;
ALTER TABLE comic_reading_progress DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON people_persons;
ALTER TABLE people_persons DISABLE ROW LEVEL SECURITY;
ALTER TABLE people_persons DROP COLUMN IF EXISTS tenant_id;

DROP POLICY IF EXISTS tenant_isolation ON people_birthday_notices;
ALTER TABLE people_birthday_notices DISABLE ROW LEVEL SECURITY;
ALTER TABLE people_birthday_notices DROP COLUMN IF EXISTS tenant_id;
