-- 0034_journal_drop_comic_chapter_stream: remove the per-chapter cards from the
-- life-stream. Same shape as 0033, same reason.
--
-- Publishing a comic emitted comic:chapter_published once PER CHAPTER, and the
-- stream projected each one. A 500-chapter title therefore posted its whole
-- table of contents into the feed; 2,516 rows had accumulated against 10 actual
-- journal entries.
--
-- The information is not lost, it moved to where it belongs: a publish now emits
-- one `comic:published` carrying the chapter count, and notify turns that into a
-- single bell entry ("<title> is published — 500 chapters"). One click, one
-- notification.
--
-- NOT REVERSIBLE, for the same reason as 0033: occurred_at was stamped at
-- projection time and cannot be reconstructed. The down migration is a no-op.

DELETE FROM stream_items WHERE source_module = 'comic' AND event_type = 'comic:chapter_published';
