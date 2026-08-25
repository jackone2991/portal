# Comic module

Owns: comics / manga with chapter → page hierarchy. Each page is an image asset (managed by the media module).

## Talks to

- `media/api` to resolve page-image URLs (often as a batch: one chapter = N pages)
- `account/api` for author identity + permission checks

## Tables (planned)

`comics`, `comic_chapters`, `comic_pages`, `comic_authors`, `comic_reading_progress`.

## Open work

Everything the previous version of this list called "open" has shipped — the
migration is `0015_comic_core` (not `0009_comic_init`), CRUD + permissions are
live, and right-to-left reading order landed in `0024_comic_reading_direction`.
What is actually left:

- **P1.8 Bookmarks** — `comic_bookmarks`, `PUT/DELETE /pages/{id}/bookmark`,
  `GET /comics/{id}/bookmarks`. Specced at SPEC-02:301; no table, no route.
  (Note the P-number collision: the shipped "P1.8" is external-source sync.)
- **`RunImport` has no test.** `import.go:148-529` is the largest and riskiest
  function in the module and is reachable only with an object store, a tenant
  runner, a real zip, the media module and wall-clock sleeps.
- **`SaveProgress` skips the published-or-owner gate** its sibling
  `ReaderPagesVisible` enforces — an existence oracle over other users' drafts.
