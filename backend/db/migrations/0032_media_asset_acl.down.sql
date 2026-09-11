-- Revert 0032: back to the uniform tenant-only policy from 0020.
--
-- Note the widening: after this runs, every member of a shared org can read
-- every other member's media again, and `visibility` is gone so nothing is
-- public. Only roll back if 0032 itself is the problem.

DROP POLICY IF EXISTS variant_delete ON media_asset_variants;
DROP POLICY IF EXISTS variant_update ON media_asset_variants;
DROP POLICY IF EXISTS variant_write  ON media_asset_variants;
DROP POLICY IF EXISTS variant_select ON media_asset_variants;

CREATE POLICY tenant_isolation ON media_asset_variants
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

DROP POLICY IF EXISTS asset_delete ON assets;
DROP POLICY IF EXISTS asset_update ON assets;
DROP POLICY IF EXISTS asset_insert ON assets;
DROP POLICY IF EXISTS asset_select ON assets;

CREATE POLICY tenant_isolation ON assets
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);

ALTER TABLE assets DROP COLUMN visibility;
