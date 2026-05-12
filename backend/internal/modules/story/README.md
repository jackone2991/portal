# Story module

Owns: long-form written stories with chapters, authors, reading progress, bookmarks.

## Talks to

- `media/api` for cover images and optional audio narration assets
- `account/api` for author identity + permission checks

## Tables (planned)

`stories`, `story_chapters`, `story_authors`, `story_reading_progress`, `story_bookmarks`.

## Open work

- Migration `0008_story_init.up.sql`
- CRUD + permissions (`stories:read`, `stories:write:own`, `stories:publish`, `stories:delete:any`)
- Postgres FTS on `tsvector` with Vietnamese `unaccent` configuration
- Chapter pagination + per-chapter reading-progress tracking
