-- 0044_journal_attachments_in_columns: an Entry's Attachments live in
-- `asset_ids`, not in a magic link inside the body (SPEC-12 T1, #10).
--
-- Until now the composer smuggled the Attachment into the markdown body as
-- `![photo](asset:<uuid>)` because the service refused the `asset_ids` column
-- it has carried since 0011. This migration makes the column live and takes the
-- cheat out of every existing body in the same step, so "the body is plain
-- markdown" holds for all rows — not only new ones — and the frontend never
-- needs the decoder as a fallback.
--
-- The Location half (`[name](geo:lat,lon)`) is NOT touched here; SPEC-12 T3
-- moves it into its own columns with its own migration.

-- ── 1. The body CHECK keeps only the length bound ─────────────────────────
-- An Attachment-only Entry has an empty body. The rule "an Entry is text, or at
-- least one Attachment" is a service-level WRITE rule, not a CHECK: the
-- asset-deleted consumer (T4) must be able to strip the last Attachment from
-- such an Entry and leave it standing, and a CHECK would refuse exactly that.
ALTER TABLE journal_entries DROP CONSTRAINT journal_entries_body_md_check;
ALTER TABLE journal_entries ADD CONSTRAINT journal_entries_body_md_check
    CHECK (char_length(body_md) <= 20000);

-- ── 2. Backfill, then assert the post-condition ───────────────────────────
-- The regex is the one the frontend used to write the link (lib/attachments.ts
-- PHOTO_RE), copied verbatim. Only the FIRST match moves — the body form only
-- ever held one Attachment; a second was silently dropped by the composer.
--
-- The id is checked against the media module's `assets` table. This is a
-- migration, not module code, so reading across the boundary is licensed here
-- (issue #10) and nowhere else: an Attachment whose Asset no longer exists is
-- DROPPED rather than backfilled, so no row ever points at a picture that is
-- gone. The link is stripped from the body either way.
--
-- Stripping mirrors what the decoder did to render the text: remove the link,
-- collapse the blank lines it leaves behind, trim the ends. A body that was
-- only the link becomes '' — legal under the relaxed CHECK.
--
-- The block ends by RAISING if any body still matches, which makes a
-- half-migrated database impossible: either every row is clean or the whole
-- transaction rolls back. On an empty database (CI) that is a trivial pass.
DO $$
DECLARE
    photo_re CONSTANT text := '!\[[^\]]*\]\(asset:([0-9a-fA-F-]{36})\)';
    uuid_re  CONSTANT text := '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$';
    r        RECORD;
    captured text;
    aid      uuid;
    new_ids  uuid[];
    new_body text;
    leftover int;
BEGIN
    FOR r IN
        SELECT id, body_md, asset_ids FROM journal_entries WHERE body_md ~ photo_re
    LOOP
        captured := substring(r.body_md FROM photo_re);
        new_ids  := r.asset_ids;
        -- 36 hex-or-dash characters is what the frontend regex accepted; only a
        -- well-formed uuid can be cast and looked up.
        IF captured ~ uuid_re THEN
            aid := captured::uuid;
            IF EXISTS (SELECT 1 FROM assets a WHERE a.id = aid)
               AND NOT (aid = ANY (new_ids)) THEN
                new_ids := new_ids || aid;
            END IF;
        END IF;

        new_body := regexp_replace(r.body_md, photo_re, '');          -- first match only
        new_body := regexp_replace(new_body, E'\\n{3,}', E'\n\n', 'g'); -- blank lines it left
        new_body := regexp_replace(new_body, '^\s+|\s+$', '', 'g');    -- trim both ends

        UPDATE journal_entries
        SET body_md = new_body, asset_ids = new_ids
        WHERE id = r.id;
    END LOOP;

    SELECT count(*) INTO leftover FROM journal_entries WHERE body_md ~ photo_re;
    IF leftover > 0 THEN
        RAISE EXCEPTION '0044: % journal_entries row(s) still carry a ![..](asset:..) link after backfill', leftover;
    END IF;
END $$;
