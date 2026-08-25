# Movie module

Owns: films, episodes, cast, ratings, watchlists for the movie vertical.

## Talks to

- `media/api` to resolve asset URLs for playback
- `account/api` for ownership / permission checks at the boundary

## Subscribes to

- `media:asset_ready` — flip movie status to `ready` once HLS variants exist

## Tables (planned)

`movies`, `movie_episodes`, `movie_cast`, `movie_ratings`, `movie_watchlist_entries`.

## Open work

The migration is `0021_movie_core` (not `0006_movie_init`) and CRUD +
permissions are live. Genuinely open:

- **No frontend.** There is no `/movies` route in the Next.js app.
- **Search** — Postgres FTS (D-2). No `tsvector` exists anywhere in the schema
  yet, in any module.
- **No SPEC.** This vertical was built by mirroring comic; nothing specifies it.
