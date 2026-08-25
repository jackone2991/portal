# Story module

Owns: long-form written stories with chapters, authors, reading progress, bookmarks.

## Talks to

- `media/api` for cover images and optional audio narration assets
- `account/api` for author identity + permission checks

## Tables (planned)

`stories`, `story_chapters`, `story_authors`, `story_reading_progress`, `story_bookmarks`.

## Open work

The migration is `0023_story_core` (not `0008_story_init`) and CRUD + chapters +
reorder are live. Genuinely open:

- **No frontend.** `views/library/novel/NovelDetailView.tsx` is a 26-line static
  placeholder and the index route it links to does not exist.
- **Reading progress** — comic has `comic_reading_progress`; story has no
  equivalent.
- **Search** — Postgres FTS + `unaccent` (D-2). Not started in any module.
- **No SPEC.**
