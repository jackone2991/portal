-- 0020_platform_rls_enable: tenant_id + FORCE RLS on the 17 tenant-scoped tables
-- (ADR-07 Phase 1, Increment 2c). This is the data-isolation layer.
--
-- ┌───────────────────────────────────────────────────────────────────────────┐
-- │ ⚠️  DO NOT APPLY until worker tenant-scoping (Increment "1b") has landed.   │
-- │                                                                             │
-- │ tenant_id's DEFAULT current_setting('app.current_tenant')::uuid fires on   │
-- │ EVERY INSERT that omits tenant_id. current_setting() with the GUC unset    │
-- │ ERRORS — and a column-DEFAULT error is NOT bypassed by the superuser.      │
-- │ The API sets the GUC (RequireTenant, Increment 2a), but the WORKER does    │
-- │ not yet: its event consumers + scan insert into stream_items,              │
-- │ media_asset_variants, notifications, people_birthday_notices. Applying     │
-- │ this before the worker opens BeginTenantScope will break those async       │
-- │ paths. Sequence: 1b (worker) → live-verify 2a/1b → THEN this migration.    │
-- │                                                                             │
-- │ Enforcement itself stays INERT until the cutover (a binary's DATABASE_URL  │
-- │ → portal_app): the superuser `portal` bypasses even FORCE RLS. So reads/   │
-- │ writes on EXISTING rows keep working; only the INSERT-default needs the    │
-- │ GUC (hence the 1b prerequisite).                                           │
-- └───────────────────────────────────────────────────────────────────────────┘
--
-- 16 tables get tenant_id NOT NULL + a uniform tenant_isolation policy. The 17th,
-- bank_categories, keeps tenant_id NULLABLE: its NULL-user rows are the shared
-- global seed taxonomy (0014), deliberately visible across tenants — its policy
-- reads own-tenant OR NULL, writes own-tenant only. (Chosen over per-tenant seed
-- fan-out: simpler, preserves the shared-seed model; the isolation test special-
-- cases this one table.)

-- ─────────────────────────────────────────────────────────────────────────────
-- Phase 1 — add nullable tenant_id everywhere (no runtime effect yet).
-- ─────────────────────────────────────────────────────────────────────────────
ALTER TABLE assets                  ADD COLUMN tenant_id UUID;
ALTER TABLE media_asset_variants    ADD COLUMN tenant_id UUID;
ALTER TABLE media_playback_progress ADD COLUMN tenant_id UUID;
ALTER TABLE notifications           ADD COLUMN tenant_id UUID;
ALTER TABLE journal_entries         ADD COLUMN tenant_id UUID;
ALTER TABLE stream_items            ADD COLUMN tenant_id UUID;
ALTER TABLE bank_accounts           ADD COLUMN tenant_id UUID;
ALTER TABLE bank_categories         ADD COLUMN tenant_id UUID;
ALTER TABLE bank_import_batches     ADD COLUMN tenant_id UUID;
ALTER TABLE bank_transactions       ADD COLUMN tenant_id UUID;
ALTER TABLE bank_budgets            ADD COLUMN tenant_id UUID;
ALTER TABLE comics                  ADD COLUMN tenant_id UUID;
ALTER TABLE comic_chapters          ADD COLUMN tenant_id UUID;
ALTER TABLE comic_pages             ADD COLUMN tenant_id UUID;
ALTER TABLE comic_reading_progress  ADD COLUMN tenant_id UUID;
ALTER TABLE people_persons          ADD COLUMN tenant_id UUID;
ALTER TABLE people_birthday_notices ADD COLUMN tenant_id UUID;

-- ─────────────────────────────────────────────────────────────────────────────
-- Phase 2 — backfill direct-owner tables from the owner's personal org (0018).
-- Owner column differs: assets.owner_id, comics.owner_user_id, else user_id.
-- ─────────────────────────────────────────────────────────────────────────────
UPDATE assets a                  SET tenant_id = o.id FROM organizations o WHERE o.owner_id = a.owner_id       AND o.kind = 'personal';
UPDATE media_playback_progress t SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE notifications t           SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE journal_entries t         SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE stream_items t            SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE bank_accounts t           SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE bank_import_batches t     SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE bank_transactions t       SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE bank_budgets t            SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE comics t                  SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.owner_user_id  AND o.kind = 'personal';
UPDATE comic_reading_progress t  SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
UPDATE people_persons t          SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal';
-- bank_categories: only USER rows (user_id NOT NULL); NULL-user seed rows stay NULL.
UPDATE bank_categories t         SET tenant_id = o.id FROM organizations o WHERE o.owner_id = t.user_id        AND o.kind = 'personal' AND t.user_id IS NOT NULL;

-- ─────────────────────────────────────────────────────────────────────────────
-- Phase 3 — backfill child tables from their (now-populated) parents.
-- ─────────────────────────────────────────────────────────────────────────────
UPDATE media_asset_variants v    SET tenant_id = a.tenant_id  FROM assets a          WHERE a.id  = v.asset_id;
UPDATE comic_chapters c          SET tenant_id = co.tenant_id FROM comics co         WHERE co.id = c.comic_id;
UPDATE comic_pages p             SET tenant_id = ch.tenant_id FROM comic_chapters ch WHERE ch.id = p.chapter_id;
UPDATE people_birthday_notices n SET tenant_id = pp.tenant_id FROM people_persons pp WHERE pp.id = n.person_id;

-- ─────────────────────────────────────────────────────────────────────────────
-- Phase 4 — lock down + enable RLS. The DEFAULT lets existing INSERT statements
-- populate tenant_id with no query change (the churn-avoider). FK is plain here;
-- for a large prod table use ADD CONSTRAINT ... NOT VALID then VALIDATE CONSTRAINT.
--
-- The 16 NOT-NULL tables share the identical uniform policy:
--   USING/WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid)
-- ─────────────────────────────────────────────────────────────────────────────

-- assets
ALTER TABLE assets ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE assets ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE assets ADD CONSTRAINT assets_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE assets ENABLE ROW LEVEL SECURITY;
ALTER TABLE assets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON assets
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX assets_tenant_idx ON assets (tenant_id);

-- media_asset_variants
ALTER TABLE media_asset_variants ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE media_asset_variants ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE media_asset_variants ADD CONSTRAINT media_asset_variants_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE media_asset_variants ENABLE ROW LEVEL SECURITY;
ALTER TABLE media_asset_variants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON media_asset_variants
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX media_asset_variants_tenant_idx ON media_asset_variants (tenant_id);

-- media_playback_progress
ALTER TABLE media_playback_progress ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE media_playback_progress ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE media_playback_progress ADD CONSTRAINT media_playback_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE media_playback_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE media_playback_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON media_playback_progress
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX media_playback_progress_tenant_idx ON media_playback_progress (tenant_id);

-- notifications
ALTER TABLE notifications ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE notifications ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE notifications ADD CONSTRAINT notifications_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notifications
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX notifications_tenant_idx ON notifications (tenant_id);

-- journal_entries
ALTER TABLE journal_entries ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE journal_entries ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE journal_entries ADD CONSTRAINT journal_entries_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE journal_entries ENABLE ROW LEVEL SECURITY;
ALTER TABLE journal_entries FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON journal_entries
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX journal_entries_tenant_idx ON journal_entries (tenant_id);

-- stream_items
ALTER TABLE stream_items ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE stream_items ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE stream_items ADD CONSTRAINT stream_items_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE stream_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE stream_items FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON stream_items
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX stream_items_tenant_idx ON stream_items (tenant_id);

-- bank_accounts
ALTER TABLE bank_accounts ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_accounts ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bank_accounts ADD CONSTRAINT bank_accounts_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_accounts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_accounts
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_accounts_tenant_idx ON bank_accounts (tenant_id);

-- bank_import_batches
ALTER TABLE bank_import_batches ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_import_batches ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bank_import_batches ADD CONSTRAINT bank_import_batches_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_import_batches ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_import_batches FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_import_batches
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_import_batches_tenant_idx ON bank_import_batches (tenant_id);

-- bank_transactions
ALTER TABLE bank_transactions ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_transactions ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bank_transactions ADD CONSTRAINT bank_transactions_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_transactions ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_transactions FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_transactions
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_transactions_tenant_idx ON bank_transactions (tenant_id);

-- bank_budgets
ALTER TABLE bank_budgets ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_budgets ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bank_budgets ADD CONSTRAINT bank_budgets_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_budgets ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_budgets FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_budgets
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_budgets_tenant_idx ON bank_budgets (tenant_id);

-- comics
ALTER TABLE comics ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE comics ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE comics ADD CONSTRAINT comics_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE comics ENABLE ROW LEVEL SECURITY;
ALTER TABLE comics FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comics
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX comics_tenant_idx ON comics (tenant_id);

-- comic_chapters
ALTER TABLE comic_chapters ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE comic_chapters ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE comic_chapters ADD CONSTRAINT comic_chapters_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE comic_chapters ENABLE ROW LEVEL SECURITY;
ALTER TABLE comic_chapters FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comic_chapters
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX comic_chapters_tenant_idx ON comic_chapters (tenant_id);

-- comic_pages
ALTER TABLE comic_pages ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE comic_pages ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE comic_pages ADD CONSTRAINT comic_pages_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE comic_pages ENABLE ROW LEVEL SECURITY;
ALTER TABLE comic_pages FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comic_pages
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX comic_pages_tenant_idx ON comic_pages (tenant_id);

-- comic_reading_progress
ALTER TABLE comic_reading_progress ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE comic_reading_progress ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE comic_reading_progress ADD CONSTRAINT comic_reading_progress_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE comic_reading_progress ENABLE ROW LEVEL SECURITY;
ALTER TABLE comic_reading_progress FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON comic_reading_progress
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX comic_reading_progress_tenant_idx ON comic_reading_progress (tenant_id);

-- people_persons
ALTER TABLE people_persons ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE people_persons ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE people_persons ADD CONSTRAINT people_persons_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE people_persons ENABLE ROW LEVEL SECURITY;
ALTER TABLE people_persons FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON people_persons
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX people_persons_tenant_idx ON people_persons (tenant_id);

-- people_birthday_notices
ALTER TABLE people_birthday_notices ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE people_birthday_notices ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE people_birthday_notices ADD CONSTRAINT people_birthday_notices_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE people_birthday_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE people_birthday_notices FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON people_birthday_notices
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX people_birthday_notices_tenant_idx ON people_birthday_notices (tenant_id);

-- ─────────────────────────────────────────────────────────────────────────────
-- Phase 5 — bank_categories: NULLABLE tenant_id (shared NULL-user seed rows stay
-- global). Read own-tenant OR shared seeds; write own-tenant only (WITH CHECK
-- forbids forging a NULL-tenant/seed row). This is the one non-uniform policy.
-- ─────────────────────────────────────────────────────────────────────────────
ALTER TABLE bank_categories ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_categories ADD CONSTRAINT bank_categories_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_categories ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_categories FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_categories
    USING (tenant_id = current_setting('app.current_tenant')::uuid OR tenant_id IS NULL)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_categories_tenant_idx ON bank_categories (tenant_id);
