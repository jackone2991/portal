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

- Migration `0006_movie_init.up.sql`
- CRUD endpoints + permissions wired (`movies:read`, `movies:write:own`, `movies:publish`, `movies:delete:any`)
- Search hookup (Postgres FTS first)
