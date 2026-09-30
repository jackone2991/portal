# Test Execution Report — 2026-09-19 — SPEC-12 manual run

**Status:** historical · **Last verified:** 2026-09-19

**Plan:** [SPEC-12 §Testing Decisions](../product/specs/SPEC-12-journal-attachments.md) · **Build:** branch `feat/journal-attachments` at `3ecf06f` (T0–T6), images rebuilt with `docker compose build api worker frontend` · **Env:** DEV-LOCAL — the compose stack over Traefik (`https://portal.localhost`, `https://api.portal.localhost`), host Postgres at schema `0045`, MinIO, the real worker
**Executed by:** agent session (API half scripted, UI half driven in Chrome as `demo@portal.localhost`) · **Closes:** the manual-run half of [backlog](../product/backlog.md) P1 17a

Every step below ran against the live stack; nothing is inferred from tests.

## API half — 28 checks, 28 pass

Five 640×480 PNGs went through the real pipeline (`POST /assets` → `PUT /source` → `POST /complete` → poll to `ready`), then:

| # | Check | Result |
|---|-------|--------|
| 1 | `POST /journal/entries` with five `asset_ids`, a mood and a Location → 201; `asset_ids` back in the order sent; `location` and `mood` on the Entry | pass |
| 2 | `GET /stream`: the journal item carries the same `asset_ids` and `location` | pass |
| 3 | Duplicate id → 422 `journal/invalid-asset`, `detail` names it; off-Earth Location → 422 `journal/invalid-location`; Location alone → 422 `journal/invalid-body`; unknown id → 422 `journal/invalid-asset` | pass |
| 4 | `PATCH` the whole Entry — one photo dropped (order kept), `location: null`, `mood: null` → 200; `GET` agrees; a later `PATCH {location}` sets it back and leaves `asset_ids` alone | pass |
| 5 | `DELETE /assets/{id}` of a photo on two Entries → 204; within 3 s the worker consumer strips it from both, the others survive in order, the stream item shows one fewer, the variant answers 404 | pass |
| 6 | A photo-only Entry whose only photo is deleted still exists with `[]` and `""`; a mood-only PATCH on it is allowed; a body write that leaves it empty → 422 `journal/invalid-body` | pass |

Client note: PowerShell 5.1's `Invoke-WebRequest` drops 4xx bodies and sends non-ASCII JSON in the ANSI code page (the first run showed eight false failures for exactly that); the script switched to `curl.exe` with UTF-8 file bodies. Script: session scratchpad, not committed.

## UI half — composer, card, lightbox, edit in place

| Step | Observed |
|------|----------|
| Camera → Add Photo → five files at once | five tiles, each `Sẵn sàng` once processed; popup closed itself; Post enabled with no text (photo-only draft) |
| Eleven files into the five (six new, five re-picked) | ten tiles; notice `Tối đa 10 ảnh mỗi bài — 6 ảnh bị bỏ qua.`; camera reads `Đã đủ 10 ảnh`. **Harness artifact:** the Chrome extension's file upload stamps `lastModified = now` on every pick (verified: two picks of the same file 13 s apart differ), so the five re-picks were not duplicates to the client and filled the strip — a real file picker keeps the file's mtime, and the rule is under vitest (`composer-photos.test.ts`) |
| Tap camera on a full strip | notice `Đã đủ 10 ảnh mỗi bài — bỏ bớt một ảnh để thêm ảnh khác.`; no dialog |
| Remove one tile, add a file that is not an image | tile shows `Không hoàn tất được tải lên.`; Post disabled with title `Bỏ ảnh lỗi trước khi đăng`; removing the tile re-enables Post |
| Post with text and nine photos | card: hero + four thumbs + `+4`; composer reset |
| Tap the hero | lightbox `Ảnh 1 / 9` at `medium`, previous arrow disabled; `ArrowRight` ×2 → `Ảnh 3 / 9`; `Escape` closes |
| Post options → Edit Post | the card is replaced by the composer: tab `Edit Post`, the text and nine tiles pre-filled, Cancel and Save |
| Remove one tile, set the mood `vui`, Save | card back with `Mở ảnh 1 / 8`, `+3`, `19 Sept · vui`; server: 8 ids, mood `vui` |
| Edit again, remove a tile, Cancel | card unchanged (still 8) |
| Edit → pin → search `Hoan Kiem` → pick → Add Location → Save | chip `Hoàn Kiếm` on the card linking to OpenStreetMap; server `location` `{Hoàn Kiếm, 21.0286, 105.8506}` |
| Delete one attached Asset (API), reload | card `Mở ảnh 1 / 7`, `+2`, chip intact — one photo fewer, no broken frame |

**Defect found and fixed in the same session:** opening the mood chip focused it with `requestAnimationFrame`, which never fired in the automation's backgrounded tab, so the typed mood landed in the textarea. Replaced by an effect keyed on an open-counter (`Composer.tsx`). Not reproducible by a person on a focused tab, but the effect is the right mechanism regardless.

## Not covered here

The `0044`/`0045` backfill loops (proven by hand on a throwaway database on 2026-09-13 and 2026-09-19, still without an automated test — the other half of backlog 17a) and the migration path itself, which the `backend` CI job runs from zero.
