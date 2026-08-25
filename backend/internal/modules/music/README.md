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

The migration is `0022_music_core` (not `0007_music_init`) and CRUD +
permissions are live. Genuinely open:

- **No frontend.** There is no `/tracks` route in the Next.js app; the three
  `components/music/*` files have zero importers.
- **Audio transcode profile** — audio uploads are stored and served as-is;
  media's pipeline transcodes video and images only.
- **No SPEC.**
