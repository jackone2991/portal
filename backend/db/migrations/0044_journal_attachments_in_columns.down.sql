-- 0044 down: put the Attachment back into the body the way the old composer
-- wrote it, so a rollback restores the old bodies rather than losing the
-- pictures.
--
-- The first Attachment is re-encoded as `![photo](asset:<uuid>)` appended after
-- a blank line (lib/attachments.ts composeBody joined text and link with "\n\n";
-- a body that is only the link has no leading blank line). Only the FIRST id is
-- re-encoded — the body form only ever had room for one. The column is left as
-- it is: the pre-T1 code never reads or writes it, and keeping it means a
-- re-run of 0044 up finds nothing to move (the id is already present) while
-- any further ids survive the round trip.
UPDATE journal_entries
SET body_md = CASE
        WHEN btrim(body_md) = '' THEN '![photo](asset:' || asset_ids[1]::text || ')'
        ELSE body_md || E'\n\n![photo](asset:' || asset_ids[1]::text || ')'
    END
WHERE cardinality(asset_ids) > 0;

-- Restore the 0011 CHECK. Every row with an Attachment now has a non-empty body
-- (the link). Two rows would fail it and have to be dealt with by hand before
-- rolling back: one with neither text nor Attachment — possible only after T4's
-- asset-deleted consumer strips the last Attachment from an Attachment-only
-- Entry — and one whose text was within ~55 characters of the 20 000 limit,
-- which the appended link pushes over.
ALTER TABLE journal_entries DROP CONSTRAINT journal_entries_body_md_check;
ALTER TABLE journal_entries ADD CONSTRAINT journal_entries_body_md_check
    CHECK (char_length(body_md) BETWEEN 1 AND 20000);
