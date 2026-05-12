# Comic module

Owns: comics / manga with chapter → page hierarchy. Each page is an image asset (managed by the media module).

## Talks to

- `media/api` to resolve page-image URLs (often as a batch: one chapter = N pages)
- `account/api` for author identity + permission checks

## Tables (planned)

`comics`, `comic_chapters`, `comic_pages`, `comic_authors`, `comic_reading_progress`.

## Open work

- Migration `0009_comic_init.up.sql`
- CRUD + permissions (`comics:read`, `comics:write:own`, `comics:publish`, `comics:delete:any`)
- Optimised batch endpoint: `GET /comics/{id}/chapters/{n}/pages` returns all page URLs in one round-trip
- Right-to-left reading order flag for manga
