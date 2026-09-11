-- 0032_media_asset_acl: per-user ACL on media, on top of tenant isolation.
--
-- 0020 gave every media row a tenant fence. That is the right outer boundary,
-- but inside a shared org it is not a boundary at all: any member could read any
-- other member's asset by id. This migration narrows the rule to
--
--     you may read an asset if it is PUBLIC,
--     or it is yours,
--     or you are an admin of the tenant that owns it.
--
-- Three session GUCs carry the actor (all set by platform/db BeginScope):
--
--     app.current_tenant   the active org
--     app.current_user     the acting user; empty for a backend job
--     app.tenant_admin     'on' for a tenant owner/admin AND for trusted
--                          backend scopes (the worker), which must process
--                          every member's media
--
-- Every read of a GUC here uses the two-argument current_setting(..., true)
-- wrapped in NULLIF, so an UNSET or EMPTY variable yields NULL and the
-- comparison is simply false. The one-argument form used by 0020 RAISES on an
-- unset variable, which the media handlers turned into an indistinguishable
-- 404 — a request with no tenant context must fall through to "you may only see
-- public rows", not explode. The tenant_id column DEFAULT keeps the strict
-- one-argument form on purpose: a write with no tenant context must still fail
-- loudly rather than land somewhere arbitrary.

-- ─────────────────────────────────────────────────────────────────────────────
-- visibility: private by default. Everything that already exists stays private,
-- including the ~219k imported comic pages — sharing is an explicit act.
-- ─────────────────────────────────────────────────────────────────────────────
ALTER TABLE assets ADD COLUMN visibility TEXT NOT NULL DEFAULT 'private'
    CHECK (visibility IN ('private', 'public'));

-- ─────────────────────────────────────────────────────────────────────────────
-- assets — replace the uniform tenant policy with per-command ACL policies.
-- Split by command so the read rule (which admits public rows) can never widen
-- a write: a public asset is readable by anyone, writable only by its owner.
-- ─────────────────────────────────────────────────────────────────────────────
DROP POLICY tenant_isolation ON assets;

CREATE POLICY asset_select ON assets FOR SELECT
    USING (
        visibility = 'public'
        OR (
            tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
            AND (
                owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
                OR current_setting('app.tenant_admin', true) = 'on'
            )
        )
    );

CREATE POLICY asset_insert ON assets FOR INSERT
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND (
            owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
            OR current_setting('app.tenant_admin', true) = 'on'
        )
    );

CREATE POLICY asset_update ON assets FOR UPDATE
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND (
            owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
            OR current_setting('app.tenant_admin', true) = 'on'
        )
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND (
            owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
            OR current_setting('app.tenant_admin', true) = 'on'
        )
    );

CREATE POLICY asset_delete ON assets FOR DELETE
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND (
            owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
            OR current_setting('app.tenant_admin', true) = 'on'
        )
    );

-- ─────────────────────────────────────────────────────────────────────────────
-- media_asset_variants — a variant is a rendition of its asset and must be
-- exactly as visible as the asset, never more.
--
-- The rule is RESTATED here against assets' columns rather than delegating to
-- assets' own policy through a bare EXISTS. Whether a policy's subquery is
-- itself row-filtered is a subtlety no security boundary should rest on; spelled
-- out, this predicate is correct either way. The cost is duplication: change the
-- asset rule above and you must change it here too.
-- ─────────────────────────────────────────────────────────────────────────────
DROP POLICY tenant_isolation ON media_asset_variants;

CREATE POLICY variant_select ON media_asset_variants FOR SELECT
    USING (
        EXISTS (
            SELECT 1 FROM assets a
            WHERE a.id = media_asset_variants.asset_id
              AND (
                  a.visibility = 'public'
                  OR (
                      a.tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
                      AND (
                          a.owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
                          OR current_setting('app.tenant_admin', true) = 'on'
                      )
                  )
              )
        )
    );

-- Writes are the worker's (variant rows are produced by the image/transcode
-- pipeline), so they need the tenant fence plus write access to the parent.
CREATE POLICY variant_write ON media_asset_variants FOR INSERT
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM assets a
            WHERE a.id = media_asset_variants.asset_id
              AND a.tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
              AND (
                  a.owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
                  OR current_setting('app.tenant_admin', true) = 'on'
              )
        )
    );

CREATE POLICY variant_update ON media_asset_variants FOR UPDATE
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM assets a
            WHERE a.id = media_asset_variants.asset_id
              AND (
                  a.owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
                  OR current_setting('app.tenant_admin', true) = 'on'
              )
        )
    )
    WITH CHECK (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
    );

CREATE POLICY variant_delete ON media_asset_variants FOR DELETE
    USING (
        tenant_id = NULLIF(current_setting('app.current_tenant', true), '')::uuid
        AND EXISTS (
            SELECT 1 FROM assets a
            WHERE a.id = media_asset_variants.asset_id
              AND (
                  a.owner_id = NULLIF(current_setting('app.current_user', true), '')::uuid
                  OR current_setting('app.tenant_admin', true) = 'on'
              )
        )
    );
