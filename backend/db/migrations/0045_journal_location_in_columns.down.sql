-- 0045 down: put the Location back into the body the way the old composer
-- wrote it, then drop the columns — a rollback restores the old bodies rather
-- than losing the places.
--
-- The link is `[<name>](geo:<lat>,<lon>)` appended after a blank line
-- (composeBody in the deleted lib/attachments.ts joined text and link with
-- "\n\n"; a body that is only the link has no leading blank line). The name is
-- re-encoded exactly as encodeLocation did: square brackets stripped (they
-- would break the link form), trimmed, "Location" if nothing is left.
-- numeric(7,4)::text renders four decimals ("21.0285"), which the decoder's
-- regex accepts.
--
-- What comes back is the four-decimal value 0045 stored, not the digits the
-- old body had: a body that read `geo:21.02857,105.85064` returns as
-- `geo:21.0286,105.8506`. SPEC-12 settled on four places (~11 m), so this is
-- the precision the schema has, not a loss the rollback introduces.
--
-- Two rows would fail the 0044 length bound and have to be dealt with by hand
-- before rolling back: one whose text was within ~60 characters of the 20 000
-- limit, which the appended link pushes over, and — if 0044 is rolled back
-- next — one with neither text nor Attachment nor Location.
UPDATE journal_entries
SET body_md = CASE
        WHEN btrim(body_md) = '' THEN ''
        ELSE body_md || E'\n\n'
    END
    || '[' || COALESCE(NULLIF(btrim(regexp_replace(location_name, '[\[\]]', '', 'g')), ''), 'Location')
    || '](geo:' || location_lat::text || ',' || location_lon::text || ')'
WHERE location_name IS NOT NULL;

ALTER TABLE journal_entries
    DROP CONSTRAINT journal_entries_location_check,
    DROP COLUMN location_name,
    DROP COLUMN location_lat,
    DROP COLUMN location_lon;
