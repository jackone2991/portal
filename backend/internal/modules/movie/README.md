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

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

