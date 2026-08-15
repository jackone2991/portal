-- 0024_comic_reading_direction: reader redesign R3 (SPEC-02 §5). Reading direction
-- is a property of the *work* (manga = 'rtl'), owner-set — it drives the reader's
-- paged navigation order and its default mode. Not a per-user preference (those are
-- client-side, D-32). Default 'vertical' = the shipped webtoon experience.
ALTER TABLE comics
  ADD COLUMN reading_direction text NOT NULL DEFAULT 'vertical'
    CHECK (reading_direction IN ('ltr', 'rtl', 'vertical'));
