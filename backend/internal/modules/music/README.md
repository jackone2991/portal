# Music module

Owns: tracks, albums, artists, playlists for the music vertical.

## Talks to

- `media/api` to resolve audio asset URLs for playback
- `account/api` for ownership / permission checks

## Subscribes to

- `media:asset_ready` — flip track status to `ready` once audio transcode completes

## Tables (planned)

`tracks`, `albums`, `artists`, `playlists`, `playlist_entries`.

## Open work

None listed here on purpose. Implementation status has one written owner
(`/CLAUDE.md` § Current status) and open work one list
(`docs/product/backlog.md`) — ADR-11. A status claim in a module README was
wrong within weeks every time it was tried (the 2026-08-25 audit found seven
of eight sections stale).

