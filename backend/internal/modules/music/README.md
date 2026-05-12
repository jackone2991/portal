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

- Migration `0007_music_init.up.sql`
- CRUD + permissions (`music:read`, `music:write:own`, `music:publish`, `music:delete:any`)
- Audio transcode profile (lossless original → AAC + Opus variants)
