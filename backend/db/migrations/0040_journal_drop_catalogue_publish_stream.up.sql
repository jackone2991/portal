-- 0040_journal_drop_catalogue_publish_stream: take the catalogue publishes out
-- of the life-stream. Third and last of the same shape as 0033 and 0034.
--
-- Publishing a track, a movie or a story is a LIBRARY event — "this is now in
-- the library" — not a moment in anyone's day. Projected into the stream it put
-- a "<title> published" card in the feed for every work, which is the same
-- mistake media:asset_ready and comic:chapter_published made, just at a smaller
-- scale because the catalogue is smaller.
--
-- The information is not lost, it moves to where the other two went: the three
-- events now fan out to notify (notify:on_{movie,track,story}_published), which
-- writes one bell entry per work with a click-through into its library.
--
-- movie:published and story:published had written nothing yet — they had no
-- render case and would have shown their raw event name — so this is mostly
-- housekeeping for them and a real cleanup for music.
--
-- NOT REVERSIBLE, as with 0033/0034: occurred_at was stamped at projection time.
-- The down migration is a deliberate no-op.

DELETE FROM stream_items
WHERE (source_module = 'music' AND event_type = 'music:track_published')
   OR (source_module = 'movie' AND event_type = 'movie:published')
   OR (source_module = 'story' AND event_type = 'story:published');
