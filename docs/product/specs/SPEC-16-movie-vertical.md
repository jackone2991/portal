# SPEC-16 — Movie Vertical (catalogue over media video assets)

**Status:** current, rev 2 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `movie` · **Depends on:** SPEC-04 (asset lifecycle, `media:asset_deleted`, the `poster` variant, P0.8 `shared` visibility and `mediaapi.SetVisibility` — Decision 2026-10-02b (B13, B14)), SPEC-01 P0.17 (`app_can_read_shared`, called by the `shared_read` policy — [ADR-12](SPEC-01-account-identity-admin.md#adr-12)), the video pipeline from ADR-01's demo loop (upload → `media:transcode` on the `heavy` server → HLS → `assets.status = ready`), SPEC-10 (asset-level resume); pattern copied from SPEC-14
**Upstream:** as-built spec, written retroactively on 2026-10-01 from the shipped code (`backend/internal/modules/movie/`, migration `0021_movie_core`) and the decisions it rests on — there was never a brief · **Refs:** [ADR-01](SPEC-03-platform-ops.md#adr-01) (the video loop this rides on), [ADR-07](SPEC-01-account-identity-admin.md#adr-07) (tenancy), [ADR-08](SPEC-09-life-stream-home.md#adr-08), [SPEC-14](SPEC-14-comic-vertical.md) (the reference vertical), feature-inventory `D-7` (RFC 7807), `D-20` (per-domain progress + continue aggregator), `D-29` (spec-first OpenAPI, `{items}`), `D-32`/`D-33` (frontend state and rendering), [backlog.md](../backlog.md) P2 lines 28–29
**Downstream consumers:** SPEC-05 (`notify:on_movie_published` bell), SPEC-10 (a future `movie` leg of `/continue`), SPEC-03 P1.7 (takeout)

---

## 1. Problem statement

A video asset is a file: it has a filename, a duration and an HLS rendition, but
no title the owner chose, no year, no poster that is not a frame grab, and no
notion of "this is finished and belongs in the library". The ADR-01 loop proved
that Portal can upload, transcode and play a video; it did not give the owner a
**catalogue** of films.

The `movie` module is that catalogue. A movie is one row that names a single
ready video asset and an optional poster image, carries editorial metadata, and
moves between `draft` and `published`. It was built on 2026-07-19 (`f11cf3f`,
migration `0021_movie_core`) as the first copy of the comic pattern (SPEC-14),
deliberately slimmer: a movie *is* one video — no chapters, pages or
module-owned progress. No spec was written at the time; this one records what
shipped, states the contract the code should meet, and lists every place it
does not (§11).

## 2. Goals

1. The owner turns a ready video asset into a titled, dated, postered movie and
   publishes it.
2. A published movie is readable by its ADR-12 audience — the Users who
   share a group with the owner in the movie's tenant (SPEC-01 P0.18,
   Decision 2026-10-02b (B15)) and the owner's friends in that tenant or an
   actively linked one; a draft is invisible to everyone but its owner (404, never 403).
3. Deleting the underlying video or poster never leaves a movie pointing at a
   missing asset, and never leaves a published movie with nothing to play.
4. Playback and resume are the media pipeline's, unchanged: the movie module
   adds no second player and no second progress table.

## 3. Non-goals

- **Series, seasons, episodes, cast, genres, ratings, watchlists** — the
  feature-inventory §4 wish list and the module README's "planned tables". None
  is built, none is scheduled; ratings would follow `D-21`, genres `D-22`.
- **Transcoding, variants, playback URLs** — owned by `media` (SPEC-04, ADR-01).
  The movie module never touches storage keys or HLS manifests.
- **A `processing` status.** A video can only be attached once it is `ready`
  (P0.2); the module never waits on `media:asset_ready`, unlike the README's
  original plan.
- **Visibility beyond family and friends.** "Published" means visible and
  watchable by the ADR-12 audience — the owner's group co-members and the
  owner's friends in the same or an actively linked tenant (P0.3) — and never
  more: Decision 2026-10-02b (B6), amended by B13, B14 and B15, §10.
- **Search** — Postgres FTS (`D-2`) is deferred while the corpus is n=1
  ([backlog.md](../backlog.md) P2 line 29).

## 4. User stories

- As the owner, after a video finishes transcoding, I create a movie for it with
  a proper title, a release year and a poster I uploaded, and publish it.
- As the owner, I keep a movie in draft while I fix its metadata; nobody else in
  my tenant sees it in lists or by id.
- As the owner, I swap the video for a better encode without recreating the
  movie.
- As the owner, I delete a video from my media library and the movie that used
  it quietly drops back to draft instead of offering a broken player.
- As the owner, I stop a film at minute 43 and resume there later — on any
  device — because the progress lives on the video asset (SPEC-10).
- Edge: as an editor (`movies:write:any`), I fix a typo in someone else's
  published movie; I cannot attach my own asset to it.

## 5. Requirements

### P0.1 — Module scaffold and wiring

`backend/internal/modules/movie/` follows MODULES.md §8: `module.go` (`New`,
`MountHTTP`, `RegisterTasks`, `OwnerByMovie`), `api/` (task and event names,
payload structs — the only importable package), `query/movie.sql` (sqlc input),
`repository/` (sqlc output plus `adapter.go`), handler, service and types at the
package root. `cmd/api/main.go` constructs it with the media API, the shared
publisher and three owner-or-elevated guards, and mounts it; `cmd/worker/main.go`
constructs it with the repository only and registers `movie:on_asset_deleted` on
the **light** server's mux. `backend/.golangci.yml` carries the
`module-movie-isolation` depguard block.

**Acceptance criteria.**
- `grep -n 'movieMod' backend/cmd/api/main.go backend/cmd/worker/main.go` shows
  `movie.New(` and `MountHTTP` in the API and `RegisterTasks(lightMux)` in the
  worker.
- Importing `modules/movie` (not `movie/api`) from another module fails lint.

### P0.2 — Entity, create and update

A movie is `{title, description?, video_asset_id?, poster_asset_id?,
release_year?, status}` owned by `owner_user_id` (§6).

- **Title**: trimmed, 1–200 characters (runes); otherwise 422 `movie/validation`.
- **Description**: free text, optional, no length rule.
- **Release year**: optional integer in 1880–2200 (the §6 CHECK); outside →
  422 `movie/validation`. *(Code follow-up: HEAD has no app-layer check, so the
  CHECK raises and the request answers 500 — §11 row 4.)*
- **Video**: optional at create. When present it must name an asset that
  `mediaapi.GetAsset` returns with `kind = video`, `status = ready` and
  `owner_id = movies.owner_user_id` — the movie's owner, whoever the caller is —
  else 422 `movie/invalid-video-asset` and nothing is written.
- **Poster**: optional; the same rule with `kind = image` → else 422
  `movie/invalid-poster-asset`.
- No cross-module FK on either id (SPEC-14 P0.1 pattern): validation happens at
  write time, reaping happens through P0.5.

`POST /movies` creates a `draft` (201, the Movie). `PATCH /movies/{id}` is
partial: an absent key leaves the field unchanged. For `video_asset_id` and
`poster_asset_id` an explicit `null` clears the reference; for `description`
and `release_year` an explicit `null` clears the field too, as the OpenAPI
`MoviePatch` description promises. *(Code follow-up: `query/movie.sql`
`UpdateMovie` uses `COALESCE` for both, so `null` is ignored — §11 row 12.)*
`status` is never changed by PATCH. **Clearing the video of a published movie
returns it to `draft` in the same UPDATE** — the rule P0.5 already applies when
the asset is deleted; a published movie with nothing to play is the state P0.4
refuses to create. *(Code follow-up: HEAD leaves it published — §11 row 2.)*
Every update sets `updated_at = now()`.

A lookup failure inside `mediaapi.GetAsset` (database error, as opposed to "no
such asset") is a 500, not a 422. *(Code follow-up: HEAD maps every error to
the 422 — §11 row 5.)*

**Acceptance criteria.**
- Given a create with a video asset owned by another user, an image as the
  video, a video as the poster, or a `processing` video, then 422 with the
  matching slug and no row. *(TC-MOV-003…006)*
- Given a title of spaces, or 201 characters, then 422 `movie/validation`;
  given `"  Ran  "`, then the stored title is `Ran`. *(TC-MOV-001, 002)*
- Given `release_year: 1700`, then 422 `movie/validation`. *(TC-MOV-007)*
- Given editor E (`movies:write:any`) PATCHing creator C's movie with a video E
  owns, then 422 `movie/invalid-video-asset`. *(TC-MOV-008)*
- Given `PATCH {description: null}`, then `description` is null afterwards;
  given `PATCH {}`, then nothing but `updated_at` changes. *(TC-MOV-010, 011)*
- Given a published movie and `PATCH {video_asset_id: null}`, then the response
  shows `video_asset_id: null` and `status: draft`. *(TC-MOV-012)*
- Given a malformed JSON body, or `video_asset_id: "not-a-uuid"`, then 422
  `movie/validation`. *(TC-MOV-009)*

### P0.3 — Visibility and lists

- **Detail** `GET /movies/{id}`: a published movie is returned to any caller
  holding `movies:read`; a draft only to its owner. A draft that is not the
  caller's answers exactly what a missing id answers — 404 `movie/not-found`,
  byte-identical body.
- **Tenancy fences visibility first.** `movies` is tenant-scoped under FORCE RLS
  (§6), and every request runs inside the caller's personal-org scope
  (`RequireTenant`). "Published" therefore means *visible to the members of the
  owner's tenant*; with personal orgs that is the owner alone. Comic's "published
  = all authenticated users" (SPEC-14 §3) does not hold here, by decision:
  `published` was a status flag inside the owner's tenant, never a sharing
  mechanism across it (Decision 2026-10-02b (B6), §10). **Inside** the tenant
  a published movie is watchable by the owner's group co-members (B15 — not
  by every member) and the owner's friends there, and **across** it by the
  owner's accepted friends whose tenant is actively linked to the movie's
  *(Decision 2026-10-02b (B13), amending B6, audience revised by B14 and B15 —
  [ADR-12](SPEC-01-account-identity-admin.md#adr-12); unbuilt — §11 row 19)*:
  a `shared_read` policy on `movies` (§6) admits those readers to the row,
  and publishing raises the video and the poster to SPEC-04 P0.8's `shared`
  visibility, unpublishing lowers them again (P0.4). Nobody else reads it, and
  publishing never sets `public`. `GET /movies` therefore lists every
  published movie the caller may read, in any tenant.
- **`GET /movies`** lists published movies; **`GET /movies/mine`** lists the
  caller's own, drafts included. Both are keyset-paginated on
  (`updated_at DESC`, `id DESC`); the opaque cursor encodes both keys
  (`platform/server.EncodeCursor`). `limit` default 30, max 50, lenient per the
  specs README rule: missing, non-integer or < 1 → 30; above 50 → clamped to 50;
  never a Problem. *(Code follow-up: HEAD resets values above 50 to 30 — §11
  row 9.)* A malformed cursor is 400 `movie/invalid-cursor`.
- Both lists answer `{items: Movie[], next_cursor?: string}` (specs README
  Pagination; pre-rule endpoints are retrofitted, owner decision 2026-09-30).
  *(Code follow-up: HEAD answers `{movies, next_cursor?}` — §11 row 8.)*

**Acceptance criteria.**
- Given C's draft, when stranger S in the same tenant GETs it, then 404
  `movie/not-found` with the same body as a random id; C gets 200.
  *(TC-MOV-020)*
- Given C's draft, then it is absent from `GET /movies` and present in C's
  `GET /movies/mine`. *(TC-MOV-021, 022)*
- Given 75 published movies, then three pages of 30/30/15 with no duplicates or
  gaps, the last without `next_cursor`. *(TC-MOV-023)*
- Given `?limit=500`, then 50 items; given `?limit=abc`, then 30.
  *(TC-MOV-024)*
- Given `?cursor=garbage`, then 400 `movie/invalid-cursor`. *(TC-MOV-025)*
- Given C's published movie, when a caller in another tenant who is not C's
  friend, or whose tenant is not actively linked to C's, GETs it or lists
  `GET /movies`, then 404 `movie/not-found` and absent, and that caller cannot
  read its video or poster asset either: publishing made them `shared`, never
  `public`. *(TC-MOV-027, RLS suite)*
- Given C's published movie and C's friend F in an actively linked tenant,
  then F GETs and lists it, F's `PATCH`, `publish` and `DELETE` on it change
  nothing whatever F's codes, and C's drafts never appear; when they
  disconnect or a side drops the link, then F's next GET is 404.
  *(TC-MOV-114, RLS suite)*
- Given any list response, then the array is under `items` and no `movies` key
  exists. *(TC-MOV-026)*

### P0.4 — Publish, unpublish, delete and RBAC

`POST /movies/{id}/publish` requires a video: the movie's `video_asset_id` must
be set and pass the P0.2 video check *at publish time* (still ready, still the
owner's). Otherwise 422 `movie/not-publishable` and the status is unchanged.
On success the status becomes `published` (200, the Movie) and the service
emits **`movie:published`** `{movie_id, owner_user_id, title}` **after the
request transaction commits**; a publish that does not commit emits nothing.
*(Code follow-up: HEAD publishes inside the still-open request transaction —
§11 row 3.)* Publishing an already-published movie is allowed and emits again;
SPEC-05's consumer dedups on the movie id, so the bell stays silent.
`POST /movies/{id}/unpublish` sets `draft` (200) and emits nothing; it is
idempotent. `DELETE /movies/{id}` is 204, then 404 `movie/not-found` on a
repeat. Event emission is best-effort: a nil or failing publisher is logged and
never fails a committed publish.

**Asset visibility follows the movie** *(Decision 2026-10-02b (B13); unbuilt —
§11 row 19)*. Each write below calls `mediaapi.SetVisibility` (SPEC-04 P0.8)
with the **movie owner** as `ownerID`, in the same transaction as the movie
write, so a rolled-back publish changes no asset:

- publish raises the video and the poster to `shared`;
- unpublish, `DELETE` (before the row goes) and a `PATCH` that clears the video
  (which returns the movie to draft once §11 row 2 lands) lower them to
  `private`;
- a `PATCH` that sets a new video or poster on a **published** movie raises the
  new asset and lowers the one it replaced.

An asset is lowered only when no other published movie of the same owner still
uses it as video or poster. Movie cannot see other modules' references: a
poster that is also a published track's cover is lowered by the movie's
unpublish, and publishing the track again restores it — a known edge,
accepted. `SetVisibility` never touches a `public` asset. A
`movies:publish:any` holder who is not the owner passes the route guard, but
`asset_update` (`0032`) lets the visibility write change no row (a request
scope carries no tenant-admin flag, SPEC-01 P0.17): the movie is published and
its files stay private — a known edge, latent while every user has a personal organisation, to be
settled with shared tenants (`D-23`).

**Permissions** (canonical scheme, specs README AuthZ):

| Action | Permission |
|---|---|
| list published, read a published movie | `movies:read` (`RequirePermission`) |
| read own draft | owner check in `Service.GetMovie` |
| create (`POST /movies`), list own (`GET /movies/mine`) | `movies:write:own` (`RequirePermission`) — held by `user` and up once P1.3 lands; `creator` and up on `HEAD` |
| update (`PATCH /movies/{id}`) | owner, or `movies:write:any` — `RequireOwnerOrPermission(engine, "movies:write:any", byMovie)` |
| delete | owner, or `movies:delete:any` |
| publish / unpublish | `RequirePermission("movies:publish:own")` chained before `RequireOwnerOrPermission(engine, "movies:publish:any", byMovie)` — the SPEC-14 P0.2 rule. *(Code follow-up: HEAD wires only the second half, so `movies:publish:own` is seeded but never checked — §11 row 6.)* |

An owner-or-elevated guard answers 404 for a draft when the caller is neither
the owner nor a holder of the endpoint's `:any` code (SPEC-14 P0.2 AC). *(Code
follow-up: `cmd/api` `ownerExtractor` resolves the owner whatever the status;
inside one shared tenant a non-owner gets 403 and learns the draft exists —
§11 row 7. Across personal orgs RLS already turns it into 404.)*

**Seeding** (`0021_movie_core`, the 0003 `WITH grants(...)` pattern):
`movies:read` → `user`; `movies:write:own` and `movies:publish:own` →
`creator`; `movies:write:any` and `movies:publish:any` → `editor`;
`movies:delete:any` → `admin`. 0003 had already seeded `movies:read` to `guest`
and the two-segment `movies:publish` to `editor` (which satisfies
`movies:publish:any` under the grammar). On `HEAD` a plain `user` therefore
reads but cannot create; P1.3 widens the two `:own` codes to `user`, as comic's
`0025_comic_user_write_grant` did.

**Acceptance criteria.**
- Given a movie with no video, or whose video was replaced by a `processing`
  asset, or whose video's owner is not the movie's owner, then publish is 422
  `movie/not-publishable` and status stays `draft`. *(TC-MOV-040, 041)*
- Given a ready owned video, then publish is 200 `published` and exactly one
  `movie:published` with the movie's id, owner and title is enqueued after
  commit; given a COMMIT that fails, then none is. *(TC-MOV-042, 043)*
- Given a failing publisher, then publish still answers 200. *(TC-MOV-044)*
- Given a user holding only `movies:read`, then `POST /movies` and
  `GET /movies/mine` are 403. *(TC-MOV-045)*
- Given an owner lacking `movies:publish:own`, then publish is 403 and the
  status is unchanged; given an editor with `movies:publish:any`, then
  unpublishing C's movie is 200. *(TC-MOV-046, 047)*
- Given DELETE twice, then 204 then 404 `movie/not-found`. *(TC-MOV-048)*
- Given creator D (no `:any` codes) in C's tenant, when D PATCHes or publishes
  C's draft, then 404 and no change; on C's published movie, 403.
  *(TC-MOV-049)*
- Given a draft movie with a `private` video and poster, when it is published,
  then both are `shared`; when it is unpublished or deleted, then both are
  `private` again unless another published movie of the owner uses them; given
  a video the owner had made `public`, then publish and unpublish leave it
  `public`; given a publish whose transaction rolls back, then nothing changed
  visibility. *(TC-MOV-112)*

### P0.5 — Asset lifecycle coupling (`media:asset_deleted`)

The module consumes `media:asset_deleted` `{asset_id, owner_user_id}` through
the `platform/events` fan-out as task **`movie:on_asset_deleted`**. For the
deleted id it (1) clears `video_asset_id` on every movie that used it **and
returns those movies to `draft`**, and (2) clears `poster_asset_id`. Both
UPDATEs set `updated_at`. The handler is idempotent (a redelivery changes
nothing) and skips — without retrying — a payload that does not decode or
whose `asset_id` is not a uuid.

The worker runs it **inside the payload owner's tenant scope** (the specs
README Tenancy rule: worker handlers open the payload user's scope before any
query) — the deleted asset's owner is the movie's owner, because P0.2 only
accepts the owner's own assets. *(Code follow-up: `cmd/worker` builds the
module without a tenant runner and `handleAssetDeleted` queries the pool
unscoped, which the FORCE RLS policy refuses under `portal_app` — §11 row 1.)*

**Acceptance criteria.**
- Given a published movie whose video asset is deleted, then after the task the
  movie has `video_asset_id = null` and `status = draft`; its poster is
  untouched. *(TC-MOV-060)*
- Given a poster asset deleted, then only `poster_asset_id` is cleared and the
  status is unchanged. *(TC-MOV-061)*
- Given the same event delivered three times, then the end state is the same
  and every run succeeds. *(TC-MOV-062)*
- Given the worker connected as `portal_app`, then the task commits inside the
  owner's tenant scope (an RLS integration test, not a fake). *(TC-MOV-063)*

### P0.6 — Playback and resume ride on the video asset

The movie module stores no playback state. Upload is SPEC-04's `/assets` flow
(MinIO in dev, R2 in prod — ADR-04); the transcode is `media:transcode` on the
`heavy` server (concurrency-capped, the SPEC-04 P0.1 OOM guard), which produces
the HLS rendition and sets the asset `ready`; the `poster` variant is
`media:thumbnail` on the light server. A movie's video plays through the media
player (`frontend/src/templates/v1/views/library/media/MediaDetailView.tsx`,
Vidstack over `hls_url`) at `/library/media/{video_asset_id}`, and resume is
SPEC-10's `media_playback_progress`, keyed by **asset id**. The `/continue` rail
therefore shows a movie as a `module: "media"` item titled with the asset's
title, not the movie's (SPEC-10 P0.3).

**Family and friends watch a published movie** *(Decision 2026-10-02b (B13),
audience revised by B14; unbuilt — §11 row 19, on SPEC-01 §11 row 32, SPEC-04
§11 rows 20–21 and SPEC-10 §11 row 15)*. While the movie is published its video
is `shared`, so a reader in its ADR-12 audience — a User who shares a group
with the owner in the movie's tenant (B15), or the owner's friend in the same
or an actively linked tenant — opens the same
page: `GET /assets/{video_asset_id}` answers them with `hls_url`, the HLS
and `poster` routes serve them under RLS, and they resume from their **own**
progress row — SPEC-10's rows are keyed by `(user_id, asset_id)`, and SPEC-10
P0.2 admits anyone SPEC-04 P0.8 lets play the asset. The reader's row lives in the reader's own tenant
(SPEC-10 P0.2) and the owner's row is never touched by anyone else. Once the
movie is unpublished, or the reader leaves the audience (unfriend, unlink),
these routes answer 404 to them; their row stays and is reachable again when
access returns.

This deliberately diverges from `D-20`'s per-domain `movie.watch_progress`
table: one video is one asset, so asset-level progress is already
movie-level progress, and a second table would only duplicate it. A `movie`
leg of `/continue` (P1.2) changes the *presentation*, not the storage.

**Acceptance criteria.**
- Given a movie whose video was watched to 43:00 on one device, when the asset
  is opened on another, then playback resumes within ±10 s (SPEC-10 P0.2 AC,
  inherited). *(TC-MOV-070)*
- Given a movie deleted, then the video asset and its progress row survive
  (assets have their own lifecycle). *(TC-MOV-071)*
- Given C's published movie and a reader R in its audience — M who shares a
  group with C in C's tenant (B15), or C's friend F in an actively linked
  tenant — then R's
  `GET /assets/{video_asset_id}` carries `hls_url`, the manifest, segments and
  `poster` variant load, and after watching to 20:00 R resumes at ~20:00 while
  C's own position is unchanged; given the movie unpublished, or F unfriended
  or unlinked, then R's asset and progress requests answer 404.
  *(TC-MOV-113)*

### P1 — next

- **P1.1 Frontend** *(committed scope — Decision 2026-10-01b (D2): movie is
  finished to the music standard, not reverted; unbuilt)*. A
  `/library/movies` list (published + mine), a movie manager (metadata, poster upload through `lib/media-upload.ts`, video picker
  over the owner's ready video assets, publish/unpublish, delete) and a movie
  page that embeds the media player for `video_asset_id` — RSC shell plus
  TanStack client islands (D-32, D-33, specs README Frontend), views declared in
  `TemplateManifest.views` and resolved through `activeTemplate()`. The
  `/library/:path*` matcher already covers it. Today nothing exists: no route,
  no view, no `lib/movie.ts` ([backlog.md](../backlog.md) P2 line 28). The
  bell link from `notify:on_movie_published` then moves from `/library/media` to the movie
  page. *AC:* a signed-in owner creates, publishes and plays a movie without
  leaving `/library/movies`; a draft of anyone else is never listed, and a friend's published movie from a
  linked tenant is.
- **P1.2 `movie` leg of `/continue`.** `movieapi.Continue(ctx, user, limit)`
  maps the owner's in-progress video assets to their movies and returns SPEC-10
  items with `module: "movie"`, the movie title, the poster's `thumb` variant
  and the movie page as `href`; `handleContinue` merges it and drops the
  matching `media` items. *AC:* a movie in progress appears once, as a movie.
- **P1.3 `user` may author movies** *(Decision 2026-10-01b (D4); unbuilt — §11
  row 18)*. A movie-owned migration `000N_movie_user_write_grant` (the
  `0025_comic_user_write_grant` shape: `WITH grants(...)`, `ON CONFLICT DO
  NOTHING`, idempotent) grants `movies:write:own` and `movies:publish:own` to
  `user`; the role hierarchy carries them to every role above. `:any` codes and
  `movies:delete:any` stay with `editor` / `admin`. It is useful together with
  F009 — `user` holding `assets:write:own` (SPEC-04 §11 row 9), without which a
  `user` cannot upload the video or the poster once uploads enforce that code.
  Its down migration deletes only those two `role_permissions` rows (not the
  codes, which `0003`/`0021` own). *AC:* given an account holding only `user`,
  then `POST /movies` is 201, `GET /movies/mine` 200, and publishing its own
  movie 200; PATCH or publish on another member's movie is still 403
  *(TC-MOV-050)*.

### P2 — future considerations (design for, don't build)

- Series → seasons → episodes (each episode one video asset, the same P0.2
  rule), cast, genres per `D-22`, per-movie ratings per `D-21`.
- FTS over title and description (`D-2`) once there is a corpus.
- Watch history (the SPEC-10 table already records first completion).

## 6. Data model — migration `0021_movie_core`

**Tenancy** (specs README convention, ADR-07): `movies` is tenant-scoped from
birth — `tenant_id` with the `current_setting('app.current_tenant')` DEFAULT,
`movies_tenant_idx`, `ENABLE` + `FORCE ROW LEVEL SECURITY` and policy
`tenant_isolation` (USING and WITH CHECK `tenant_id = current_setting(...)`).
The repository never writes `tenant_id`; the DEFAULT fills it inside the
request's tenant scope.

```sql
CREATE TABLE movies (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    owner_user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- identity anchor
    tenant_id       UUID NOT NULL DEFAULT current_setting('app.current_tenant')::uuid
                        REFERENCES organizations(id),
    title           TEXT NOT NULL CHECK (char_length(title) BETWEEN 1 AND 200),
    description     TEXT,
    video_asset_id  UUID,          -- media video asset; validated via mediaapi, no FK
    poster_asset_id UUID,          -- media image asset; validated via mediaapi, no FK
    release_year    INT CHECK (release_year IS NULL OR release_year BETWEEN 1880 AND 2200),
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX movies_status_updated_idx ON movies (status, updated_at DESC, id DESC);  -- GET /movies
CREATE INDEX movies_owner_idx  ON movies (owner_user_id);
CREATE INDEX movies_tenant_idx ON movies (tenant_id);
```

Queries live in `backend/internal/modules/movie/query/movie.sql`; regenerate with
`make sqlc`. Every UPDATE there sets `updated_at = now()` (specs README).
`DeleteMovie` is `:execrows` so the adapter can answer `ErrNotFound` for a
repeat. **Target indexes** (SPEC-14 §11 row 13 pattern, not shipped):
`movies_owner_cursor_idx (owner_user_id, updated_at DESC, id DESC)` for
`/movies/mine`, and partial indexes on `video_asset_id` / `poster_asset_id`
`WHERE … IS NOT NULL` for the P0.5 UPDATEs, in a new `000N_movie_indexes`
(`ls backend/db/migrations | tail -2` for the number).

**Shared read** *(Decision 2026-10-02b (B14), [ADR-12](SPEC-01-account-identity-admin.md#adr-12);
unbuilt — §11 row 19)*: a movie-owned `000N_movie_shared_read`, after
`000N_tenant_links`, adds `CREATE POLICY shared_read ON movies FOR SELECT
USING (status = 'published' AND app_can_read_shared(owner_user_id,
tenant_id))`. It is permissive, so it widens reads only; INSERT, UPDATE and
DELETE stay `tenant_isolation`, and a movie is written only from its own
tenant. `ListPublishedMovies` is unchanged — its status predicate now returns
the wider set. (Before a request can run in a shared tenant,
`tenant_isolation`'s read half narrows to the owner's rows, so a member in no
group with the owner does not read the movie in-tenant — SPEC-01 P0.18,
B15.) The down migration drops the policy.

**Takeout** (specs README): movies are user-authored, so `movie/api` implements
`opsapi.ExportProvider` when SPEC-03 P1.7 lands — one JSON array of Movie
objects (the §7 shape), asset ids as references; the assets themselves export
through media.

## 7. API summary (`shared/openapi.yaml`, tag `movies`)

All routes sit under `/api/v1`, behind `authTenant` (authentication, then the
caller's tenant scope). Unauthenticated → 401 `about:blank`; a guard denial →
403 `about:blank`; a guard that cannot resolve the id (missing, unparseable, or
hidden by RLS) → 404 `about:blank`.

| Method | Path | Permission | Request | 2xx response | Errors |
|---|---|---|---|---|---|
| GET | `/movies?cursor=&limit=` | `movies:read` | — | 200 `{items: Movie[], next_cursor?}` | 400 `movie/invalid-cursor` |
| GET | `/movies/mine?cursor=&limit=` | `movies:write:own` (held by `user` after P1.3) | — | 200 `{items: Movie[], next_cursor?}` | 400 `movie/invalid-cursor`, 403 |
| POST | `/movies` | `movies:write:own` (held by `user` after P1.3) | `MovieCreate {title, description?, video_asset_id?, poster_asset_id?, release_year?}` | 201 `Movie` | 422 `movie/validation`, `movie/invalid-video-asset`, `movie/invalid-poster-asset` |
| GET | `/movies/{id}` | `movies:read` (draft: owner only) | — | 200 `Movie` | 404 `movie/not-found` |
| PATCH | `/movies/{id}` | owner, or `movies:write:any` | `MoviePatch` (absent = unchanged, `null` clears) | 200 `Movie` | 404, 422 as POST |
| DELETE | `/movies/{id}` | owner, or `movies:delete:any` | — | 204 | 404 `movie/not-found` |
| POST | `/movies/{id}/publish` | `movies:publish:own` (held by `user` after P1.3), then owner or `movies:publish:any` | — | 200 `Movie` | 422 `movie/not-publishable`, 404 |
| POST | `/movies/{id}/unpublish` | `movies:publish:own` (held by `user` after P1.3), then owner or `movies:publish:any` | — | 200 `Movie` | 404 |

**Movie**: `{id, owner_id, title, description|null, video_asset_id|null,
poster_asset_id|null, release_year|null, status: draft|published, created_at,
updated_at}` (RFC 3339 timestamps).

**Pagination**: both lists — default 30, max 50, clamped (never an error);
order (`updated_at DESC`, `id DESC`); malformed cursor 400
`movie/invalid-cursor`; a body or parameter shape failure without a named type
(malformed JSON, a non-uuid asset id) is 422 `movie/validation`. *(Code
follow-up: HEAD answers those with 400 `about:blank` — §11 row 11.)*

**Problem types**: `movie/not-found` (404), `movie/not-publishable` (422),
`movie/invalid-video-asset` (422), `movie/invalid-poster-asset` (422),
`movie/validation` (422), `movie/invalid-cursor` (400). Each is registered in
`frontend/src/lib/problems.ts` (specs README Errors). *(Code follow-up: none is
— §11 row 10.)* Annotate each operation with `x-required-permission` per the
specs README AuthZ **OpenAPI encoding** (the publish rows carry the chained
`movies:publish:own` plus `{owner_or: movies:publish:any}`).

## 8. Events and tasks

| Name | Kind | Payload | Emitted / handled | Wiring |
|---|---|---|---|---|
| `movie:published` | event | `{movie_id, owner_user_id, title}` (`movieapi.MoviePublishedEvent`) | `Service.Publish` → `emitPublished`, in `cmd/api` | `Subscribe(movieapi.EventMoviePublished, notifyapi.TaskOnMoviePublished)` in both `cmd/api/main.go` and `cmd/worker/main.go`; one consumer, `notify:on_movie_published` (bell, `dedup_key` = movie id, link `/library/media`) |
| `movie:on_asset_deleted` | consumer task | `{asset_id, owner_user_id}` (`movieapi.AssetDeletedPayload`) | `Module.handleAssetDeleted` on the light server, `default` queue | `Subscribe(media.EventAssetDeleted, movieapi.TaskOnAssetDeleted)` in both binaries |

Not projected into the life stream: `0040_journal_drop_catalogue_publish_stream`
removed the stream consumer — a publish is a library event, not a moment in the
day (SPEC-09 P0.1). The module emits no event on unpublish, edit or delete.

**Drift against [events.md](../../reference/events.md).** The two rows match the
names, payloads and consumers. Three facts are missing there: the consumer also
returns a movie to `draft` when its video is reaped (P0.5); the consumer runs
unscoped today (§11 row 1); and `movie:published` is published before the
request commits (§11 row 3).

## 9. Success metrics (n=1, honest)

- Leading: the owner catalogues ≥ 5 real films (title, year, poster) within a
  week of P1.1 shipping. Before P1.1 the honest number is zero: the module has
  no UI, so nothing but `curl` has ever created a movie.
- Leading: zero movies with `status = 'published' AND video_asset_id IS NULL`
  (`SELECT count(*) …` as the superuser) at any time after §11 rows 1–2 close.
- Lagging: a movie opened from the bell or `/continue` lands on a playable page,
  never on `/library/media` (P1.1 + P1.2).

## 10. Open questions

Two questions were decided on 2026-10-01 (Decision 2026-10-01b): movie is
finished, not reverted (D2 — P1.1 is committed scope), and `user` may author
(D4 — P1.3). One more was decided on 2026-10-02 (Decision 2026-10-02b):
"published" never crosses the tenant fence (B6 — P0.3); shipped code already
behaves this way, so it gained no row. The same day B13 amended B6: inside the
tenant a published movie is watchable by every member — publishing raises its
video and poster to SPEC-04 P0.8's asset visibility, unpublishing lowers them,
and members resume from their own progress row (P0.3, P0.4, P0.6; §11 row 19).
B14 then revised the audience ([ADR-12](SPEC-01-account-identity-admin.md#adr-12)):
the value is `shared`, and the owner's friends in actively linked tenants
watch too, through a `shared_read` policy on `movies`; a tenant admin reads
nothing extra. B15 narrowed the in-tenant half the same day: not every
member, only the owner's group co-members (SPEC-01 P0.18). The same rule
holds for music (SPEC-15) and story (SPEC-17).

No open questions remain.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

Baseline: `main` @ `99b5a0b` (the docs commits on top change no code). The spec
text above is the target; this section lists every place the shipped code still
diverges from it. Rows are ordered by severity: lost or wrong data first, then
integrity, authorization, contract, hygiene and unbuilt work; row 18 (P1, added
by Decision 2026-10-01b) is appended after row 17, and row 19 (AuthZ, added by
Decision 2026-10-02b (B13), rewritten in place by B14) after it. A row closes when
the code matches the requirement it cites and the SPEC-16 rows of
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) are regraded on
a named test. Paths are relative to `backend/internal/modules/movie/` unless
stated otherwise.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.5 tenant scope | The `movie:on_asset_deleted` handler runs inside the payload owner's tenant scope. | `cmd/worker/main.go` builds `movie.New(movie.Deps{Repo: movierepo.NewAdapter(conn)})` with no tenant runner; `module.go` `handleAssetDeleted` → `service.go` `HandleAssetDeleted` → `NullVideoByAsset`/`NullPosterByAsset` run on the bare pool (`platform/db.Conn` with no tx in the context). Under `portal_app` the FORCE RLS policy evaluates `current_setting('app.current_tenant')` with no scope and the UPDATE errors, so the task retries and dies: deleted videos stay referenced and published movies keep a dangling video. No test runs the consumer against Postgres. | **backend:** add `RunInTenant func(ctx, userID, fn) error` to `Deps`; `cmd/worker` passes `runInUserTenant`; `handleAssetDeleted` parses `owner_user_id` and runs `HandleAssetDeleted` inside it (skip, without retry, when it is missing). **test:** TC-MOV-063 (RLS suite, `RLS_TEST_APP_URL`) plus a unit test that the handler calls the runner with the payload owner. | Found while writing SPEC-16, 2026-10-01 (comic and story share it — see SPEC-17 §11 row 1) |
| 2 | P0.2 clearing the video | `PATCH {video_asset_id: null}` on a published movie also sets `status = 'draft'`. | `query/movie.sql` `UpdateMovie` sets the column only; `service.go` `UpdateMovie` never looks at `status`. The movie stays published with nothing to play — the state `NullVideoByAsset` exists to prevent. | **backend:** in `UpdateMovie`, `status = CASE WHEN @set_video AND sqlc.narg('video_asset_id') IS NULL THEN 'draft' ELSE status END`; `make sqlc`. **test:** TC-MOV-012. | Found while writing SPEC-16, 2026-10-01 |
| 3 | P0.4 event after commit | `movie:published` is enqueued only after the request transaction commits. | `service.go` `Publish` → `emitPublished` calls `events.Publish` right after `repo.SetStatus`, inside the transaction `tenant/middleware/require_tenant.go` commits only after the handler returns. A COMMIT that fails (the middleware then turns the buffered 200 into a 500) still leaves a bell notification for a publish that never happened. | **backend:** register `emitPublished` through the after-commit hook SPEC-07 §11 row 1 introduces (`db.AfterCommit`). **test:** TC-MOV-043 (a failing outer COMMIT publishes nothing). | Found while writing SPEC-16, 2026-10-01 (SPEC-07 row 1 pattern) |
| 4 | P0.2 release year | Outside 1880–2200 → 422 `movie/validation`. | `service.go` `CreateMovie`/`UpdateMovie` never check `ReleaseYear`; the `0021` CHECK raises, `writeMovieErr` falls to its default and the request is a 500 `about:blank` (and the tenant transaction rolls back). | **backend:** validate in both service methods → `ErrValidation`. **test:** TC-MOV-007. | Found while writing SPEC-16, 2026-10-01 |
| 5 | P0.2 / P0.4 lookup failures | A `mediaapi.GetAsset` infrastructure error is a 500; only "no such / wrong kind / not ready / not owned" is a 422. | `service.go` `validateVideoAsset`/`validateImageAsset` return the lookup error, and `CreateMovie`, `UpdateMovie` and `Publish` replace every non-nil result with `ErrInvalidVideoAsset` / `ErrInvalidPosterAsset` / `ErrNotPublishable`. A database blip reads as "your asset is invalid". | **backend:** return the sentinel only for `ErrValidation` or the media not-found error; pass anything else through. **test:** a fake `GetAsset` returning a plain error → 500. | Found while writing SPEC-16, 2026-10-01 |
| 6 | P0.4 publish RBAC | `RequirePermission("movies:publish:own")` chained before the owner-or-`movies:publish:any` guard on `/publish` and `/unpublish`. | `cmd/api/main.go` builds `PublishMW` as `RequireOwnerOrPermission(engine, "movies:publish:any", byMovie)` only; `module.go` `MountHTTP` mounts both routes with it. `movies:publish:own` (seeded to `creator` in `0021`) is never checked. | **backend:** mount `m.perm("movies:publish:own")` before `PublishMW` on both routes. **test:** TC-MOV-046, TC-MOV-047. | SPEC-14 §11 row 4 pattern (F051) |
| 7 | P0.4 draft is 404 on guarded routes | A non-owner without the endpoint's `:any` code gets 404 for a draft on PATCH / DELETE / publish / unpublish. | `cmd/api/main.go` `ownerExtractor` over `OwnerByMovie` (`query/movie.sql` `GetMovieOwner`) resolves the owner whatever the status; `accountmw.RequireOwnerOrPermission` then answers 403. Reachable only inside a shared tenant (RLS hides other tenants' rows → 404). | **backend:** `GetMovieOwner` also returns `status`; the extractor returns `ErrOwnerNotFound` for a draft unless the caller is the owner or holds the `:any` code (same change as SPEC-14 §11 row 5). **test:** TC-MOV-049. | SPEC-14 §11 row 5 pattern (F119) |
| 8 | P0.3 / §7 list envelope | Both lists answer `{items, next_cursor?}`. | `handler.go` `writeMovieList` writes `{"movies": …}`; `shared/openapi.yaml` `MovieList` is `required: [movies]`. No frontend reader exists. | **backend:** key `items`. **openapi:** `MovieList` → `required: [items]`. **test:** TC-MOV-026. | Decision 2026-09-30 (Envelopes); specs README unowned-list note |
| 9 | P0.3 `limit` | Missing / non-integer / < 1 → 30; > 50 → clamped to 50. | `service.go` `list`: `if limit <= 0 \|\| limit > maxLimit { limit = defaultLimit }` — `?limit=500` returns 30. | **backend:** `server.Limit(r, 30, 50)` in `handler.go` (or clamp in `list`). **openapi:** describe `limit` as defaulted and clamped. **test:** TC-MOV-024. | Decision 2026-10-01 (limit) |
| 10 | §7 problem types | Every `movie/*` slug the API emits is in `ProblemType` and `PROBLEM_MESSAGES`. | `grep -n 'movie/' frontend/src/lib/problems.ts` finds nothing; `handler.go` `writeMovieErr` emits six slugs. | **frontend:** add the six. **test:** the CC-1 catalogue check (TC-MOV-110). | Errors convention (D-7) |
| 11 | §7 shape failures | Malformed JSON, or a non-uuid `video_asset_id`/`poster_asset_id`, is 422 `movie/validation`. | `handler.go` `CreateMovie`/`UpdateMovie` answer `server.Decode`'s 400 `about:blank` ("invalid JSON body") and `server.BadRequest("invalid video_asset_id")` (400 `about:blank`). | **backend:** map both to `ErrValidation`. **openapi:** drop the 400 from `createMovie`/`updateMovie`. **test:** TC-MOV-009. | Pagination/Errors convention (README) |
| 12 | P0.2 `null` clears | `PATCH {description: null}` / `{release_year: null}` clears the field, as `MoviePatch` documents. | `query/movie.sql` `UpdateMovie` uses `COALESCE(sqlc.narg('description'), description)` and the same for `release_year`; `handler.go` decodes both as plain pointers, so `null` and absent are indistinguishable. | **backend:** three-state decoding (`json.RawMessage`, as the asset ids already do) plus `set_description` / `set_release_year` flags in the query; `make sqlc`. **test:** TC-MOV-010. | Found while writing SPEC-16, 2026-10-01 (openapi vs handler) |
| 13 | §7 OpenAPI annotations | Every operation carries `x-required-permission`; the path parameter is named for a movie. | No movie operation is annotated; all `/movies/{id}*` paths reuse `#/components/parameters/AssetID`. | **openapi:** annotate the eight operations (publish rows chained); a `MovieID` parameter. **test:** the drift check the AuthZ convention asks for. | AuthZ convention (OpenAPI encoding) |
| 14 | §6 indexes | `movies_owner_cursor_idx (owner_user_id, updated_at DESC, id DESC)`; partial indexes on the two asset columns. | `0021` ships `movies_owner_idx (owner_user_id)` only; `ListOwnMovies` and the two P0.5 UPDATEs run without a matching index. | **migration:** `000N_movie_indexes` creates them and drops `movies_owner_idx`. **test:** migration up/down (TC-MOV-111). | SPEC-14 §11 row 13 pattern |
| 15 | §6 down migration | `0021`'s down removes only what `0021` added. | `0021_movie_core.down.sql` deletes `permissions WHERE code LIKE 'movies:%'`, which also removes the 0003-seeded `movies:read`, `movies:write:own`, `movies:write:any`, `movies:publish` and `movies:delete:any` with their `guest`/`creator`/`editor`/`admin` grants. Forward-only production (`D-12`) keeps this off the live path. | **migration:** a corrective down is not possible for an applied file; record it and make future vertical downs delete only their own codes. **test:** none. | Found while writing SPEC-16, 2026-10-01 |
| 16 | P1.1 frontend | `/library/movies` list, manager and movie page. | None: no route under `frontend/src/app/(app)/library/`, no view in `templates/v1`, no `lib/movie.ts`; `notify/service.go` `workKinds` links a published movie to `/library/media`. | **frontend:** P1.1. **backend:** the notify `href` to the movie page. **test:** TC-MOV-080…083. | [backlog.md](../backlog.md) P2 line 28 |
| 17 | P1.2 continue leg | `/continue` shows a movie as `module: "movie"`. | `cmd/api/main.go` `handleContinue` calls only `mediaMod.API().Continue`; `movie/api` has no `Continue`. | **backend:** P1.2. **test:** TC-MOV-090. | SPEC-10 §5 (D-20 fan-out); backlog P2 line 25 pattern |
| 18 | P1.3 `user` authoring grant | `user` holds `movies:write:own` and `movies:publish:own` (a movie-owned grant migration); `:any` and delete-any unchanged. | `backend/db/migrations/0021_movie_core.up.sql` grants both codes to `creator` only and no later migration widens them, so a `user` gets 403 from `POST /movies` and `GET /movies/mine` (`module.go` `m.perm("movies:write:own")`). | **migration:** `000N_movie_user_write_grant` (`ls backend/db/migrations \| tail -2` for the number). **frontend:** none beyond P1.1 (`lib/session.ts` `can()` reads the new codes from `/auth/me`). **test:** TC-MOV-050 (RLS/migration suite: a `user` creates and publishes). Lands with or after F009 (SPEC-04 §11 row 9). | Decision 2026-10-01b (D4) |
| 19 | P0.3 / P0.4 / P0.6 family and friends watch a published movie; §6 shared read | Publish raises the video and poster to `shared` through `mediaapi.SetVisibility` with the movie owner; unpublish, delete, clearing the video and replacing an asset lower what no other published movie of the owner still uses; every write in the same transaction; `movies` gains the `shared_read` policy, so the ADR-12 audience reads published movies in any tenant and writes none outside its own. | `service.go` `Publish`, `Unpublish`, `DeleteMovie` and `UpdateMovie` write `movies` only; `types.go` `MediaAPI` has `GetAsset` alone. The video and poster stay `private`, so `0032`'s `asset_select` / `variant_select` hide them from everyone but the owner: `GET /assets/{id}` (`backend/internal/modules/media/service.go` `Get` → `owned()`), the HLS proxy and the `poster` variant answer 404 to them, although `GET /movies` lists the movie to every member. `0021` gives `movies` only `tenant_isolation`, so a friend in another tenant sees no movie at all. Latent while each user has a personal organisation and no tenants are linked. | **migration:** `000N_movie_shared_read` (§6), after `000N_tenant_links`. **backend:** `MediaAPI` gains `SetVisibility`; a movie query listing which of a set of asset ids another published movie of the owner still uses (owner-predicated, `status = 'published'`); `Publish`, `Unpublish`, `DeleteMovie` (before the row goes) and `UpdateMovie` raise or lower per P0.4, always passing the movie's `owner_user_id`. **openapi:** the `listMovies` description says "published movies you may read". **frontend:** none beyond P1.1 (the media detail page already plays from `hls_url`). **test:** TC-MOV-027, TC-MOV-112, TC-MOV-113, TC-MOV-114. Needs SPEC-01 §11 row 32 and SPEC-04 §11 rows 20–21; readers' resume needs SPEC-10 §11 row 15. | Decision 2026-10-02b (B13), amending B6; audience revised by B14 (ADR-12) |

**Already matching on HEAD.**
- `0021_movie_core`: the table, the title and year CHECKs, the status CHECK,
  the published-list keyset index, `tenant_id` + FORCE RLS + `tenant_isolation`
  from birth, and the six-code seed with its grants.
- Create validates the title (trimmed, 1–200 runes) and both asset references
  against `mediaapi` (kind, `ready`, owned by the movie's owner, not the
  caller); PATCH validates against the stored owner and distinguishes absent
  from `null` for both asset ids.
- Detail is published-or-owner; a stranger's draft and a missing id answer the
  same 404 `movie/not-found` body (`http_test.go`
  `TestHTTPDraftIsNotFoundToAStranger`); a repeat DELETE is 404
  (`TestHTTPDeleteTwiceIs404`, `DeleteMovie :execrows`).
- Both lists keyset on (`updated_at DESC`, `id DESC`) with `limit + 1` probing;
  a malformed cursor is 400 `movie/invalid-cursor`.
- Publish refuses a missing, unready or foreign video with 422
  `movie/not-publishable`, sets `published`, and emits `movie:published` with
  the documented payload; a nil or failing publisher never fails the publish.
- `media:asset_deleted` → `movie:on_asset_deleted` is subscribed in both
  binaries, the handler skips malformed payloads without retrying, and
  `NullVideoByAsset` also unpublishes.
- Create / list-own use `RequirePermission("movies:write:own")`; PATCH and
  DELETE use the owner-or-`:any` guards.

**Test evidence to add or fix.**
- `movie_test.go` has no test for `UpdateMovie` at all (three-state asset ids,
  title trimming on PATCH, the editor/owner asset rule — TC-MOV-008, 010–012),
  for either list (cursor, `limit`, envelope — TC-MOV-023…026) or for
  `release_year` (TC-MOV-007).
- `TestAssetDeletedClearsBothReferences` proves the calls on a fake; nothing
  proves the UPDATEs under RLS (TC-MOV-063) or that the video reaper
  unpublishes (TC-MOV-060).
- `TestPublishSucceedsAndEmits` asserts the emit, not its timing (TC-MOV-043).
- No HTTP test covers 403 for `movies:read`-only callers or the chained publish
  permission (TC-MOV-045…047) — the guards are mounted as `nil` in
  `http_test.go`, so these need a router built with a real `rbac.Engine` fake.

## 12. Out of scope

- Everything in §3: series/episodes, cast, genres, ratings, watchlists, FTS,
  publishing beyond the ADR-12 audience (a friend in an actively linked tenant
  is the only cross-tenant reader).
- The media pipeline's own gaps (HLS ladder per tier, multipart upload, audio
  transcode — backlog P2 line 29; SPEC-04 §11) and SPEC-10's resume gaps
  (SPEC-10 §11): a movie inherits them, it does not own them.
- The account-level `assets:write:own` grant for uploads (SPEC-04 §11 row 9).
- Story (SPEC-17) and music (SPEC-15).
