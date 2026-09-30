-- 0045_journal_location_in_columns: an Entry's Location lives in three columns,
-- not in a magic link inside the body (SPEC-12 T3, #12).
--
-- 0044 moved the Attachment out of the body; this moves the other half. Until
-- now the composer wrote the place as `[<name>](geo:<lat>,<lon>)` at the end of
-- the markdown and the feed parsed it back out. After this migration every
-- body — old and new — is plain markdown, and the frontend's encoder/decoder
-- module is deleted rather than kept as a fallback.

-- ── 1. Columns + CHECK ────────────────────────────────────────────────────
-- Four decimal places (~11 m) is more than a journal needs and is what the
-- picker produces. All three or none: a Location is a name AND a point; the
-- name must have something in it once trimmed; the point must be on Earth.
-- The IS NOT NULL tests on the coordinates are load-bearing: `NULL BETWEEN`
-- is NULL, and a CHECK that evaluates to NULL passes — without them a name
-- with no point would be accepted and read back as a place at the equator.
ALTER TABLE journal_entries
    ADD COLUMN location_name text,
    ADD COLUMN location_lat  numeric(7,4),
    ADD COLUMN location_lon  numeric(7,4),
    ADD CONSTRAINT journal_entries_location_check CHECK (
        (location_name IS NULL AND location_lat IS NULL AND location_lon IS NULL)
        OR (
            location_name IS NOT NULL AND btrim(location_name) <> ''
            AND location_lat IS NOT NULL AND location_lat BETWEEN -90 AND 90
            AND location_lon IS NOT NULL AND location_lon BETWEEN -180 AND 180
        )
    );

-- ── 2. Backfill, then assert the post-condition ───────────────────────────
-- The regex is the one the frontend used to write and read the link —
-- LOCATION_RE in lib/attachments.ts, which this same change deletes; see
-- `git show 1e1f12a:frontend/src/lib/attachments.ts` — copied verbatim. Only
-- the FIRST match moves: the composer only ever wrote one, the decoder only
-- ever read one.
--
-- The link text is the name (the encoder substituted "Location" for an empty
-- one; the same fallback applies here so the CHECK holds). A point the CHECK
-- would refuse cannot have come from the picker; should one exist, the block
-- RAISES naming the row rather than guessing — dropping the place would make
-- the down path unable to restore it, and storing it is impossible. The DDL
-- above rolls back with it; golang-migrate then marks 45 dirty, so after the
-- body is fixed the operator runs `migrate force 44` and `up` again.
--
-- Stripping mirrors what the decoder did to render the text: the link becomes
-- a space, runs of spaces and of blank lines collapse, the ends are trimmed. A
-- body that was only the link becomes '' — legal since 0044 — and such an
-- Entry has no text and no Attachment; SPEC-12 keeps it rather than deleting
-- it (Further Notes).
--
-- The block ends by RAISING if any body still matches, so a half-migrated
-- database is impossible. On an empty database (CI) that is a trivial pass.
DO $$
DECLARE
    geo_re    CONSTANT text := '\[([^\]]*)\]\(geo:(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)\)';
    r         RECORD;
    m         text[];
    link_name text;
    lat       numeric;
    lon       numeric;
    new_body  text;
    leftover  int;
BEGIN
    FOR r IN
        SELECT id, body_md FROM journal_entries WHERE body_md ~ geo_re
    LOOP
        m         := regexp_match(r.body_md, geo_re);  -- first match: {name, lat, lon}
        link_name := COALESCE(NULLIF(btrim(m[1]), ''), 'Location');
        lat       := m[2]::numeric;
        lon       := m[3]::numeric;
        IF lat NOT BETWEEN -90 AND 90 OR lon NOT BETWEEN -180 AND 180 THEN
            RAISE EXCEPTION '0045: journal_entries row % carries a point off the Earth (%, %) — fix the body by hand, then re-run',
                r.id, lat, lon;
        END IF;

        new_body := regexp_replace(r.body_md, geo_re, ' ');            -- first match only
        new_body := regexp_replace(new_body, '[ \t]{2,}', ' ', 'g');    -- space runs it left
        new_body := regexp_replace(new_body, E'\\n{3,}', E'\n\n', 'g'); -- blank lines it left
        new_body := regexp_replace(new_body, '^\s+|\s+$', '', 'g');    -- trim both ends

        UPDATE journal_entries
        SET body_md = new_body, location_name = link_name, location_lat = lat, location_lon = lon
        WHERE id = r.id;
    END LOOP;

    SELECT count(*) INTO leftover FROM journal_entries WHERE body_md ~ geo_re;
    IF leftover > 0 THEN
        RAISE EXCEPTION '0045: % journal_entries row(s) still carry a [..](geo:..) link after backfill', leftover;
    END IF;
END $$;
