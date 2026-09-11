# Comic module

Owns: comics / manga with chapter → page hierarchy. Each page is an image asset (managed by the media module).

## Talks to

- `media/api` to resolve page-image URLs (often as a batch: one chapter = N pages)
- `account/api` for author identity + permission checks

## Tables (planned)

`comics`, `comic_chapters`, `comic_pages`, `comic_authors`, `comic_reading_progress`.

## Open work

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

