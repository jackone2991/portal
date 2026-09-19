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

-- Restore the 0011 CHECK — NOT VALID, so the rollback completes on a database
-- that holds rows the old bound cannot describe, and the old write rule still
-- applies to every new write. Three kinds of row can be in that state: a
-- link-only body whose Asset was already gone when 0044 up ran (the link was
-- dropped, so the row is empty with nothing to re-encode — the backfill test
-- seeds exactly this row and proved a VALID constraint fails here), an
-- Attachment-only Entry whose last photo T4's asset-deleted consumer stripped,
-- and a body within ~55 characters of the 20 000 limit that the appended link
-- pushes over. After dealing with those by hand:
--   ALTER TABLE journal_entries VALIDATE CONSTRAINT journal_entries_body_md_check;
ALTER TABLE journal_entries DROP CONSTRAINT journal_entries_body_md_check;
ALTER TABLE journal_entries ADD CONSTRAINT journal_entries_body_md_check
    CHECK (char_length(body_md) BETWEEN 1 AND 20000) NOT VALID;
