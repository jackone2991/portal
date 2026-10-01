# Portal — Detailed Feature Specs (PRD level)

**Status:** current · **Last verified:** 2026-10-01

**Language policy:** English only, per owner decision 2026-07-07 (ADR-09; the
former vi mirror was deleted in `f11cf3f` — see
[ADR-11](../../adr/11-docs-canonicalisation.md)).

This folder holds the **detailed, implementation-ready specs**. A spec answers
*exactly what to build, how to know it's done, and what to decide before
starting*. SPEC-03, 04, 07, 09, 10, 11, 12 and 14 began as product briefs (brainstorm-level *what
and why*, 2026-07-07 and 2026-07-10); once every brief had been promoted, the
lines no spec carried yet were folded into its spec and the `briefs/` folder was
deleted (`git show ea100d8:docs/product/briefs/`). SPEC-05, 06, 08 and 13 had
no brief. SPEC-01, 02 and 15–18 are **as-built specs**, written retroactively on
2026-10-01 from the shipped code of the six modules no spec owned (`account`,
`music`, `movie`, `story`, `social`, `layout`); they record decisions taken
elsewhere rather than re-deciding them. Every header names its upstream.

## Documents

The **Status** column is read from the code (the module under
`backend/internal/modules/` and the migration that created its tables), not from
spec headers. Per-requirement coverage lives in
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md).

| Spec | Feature | Module | Depends on | Status |
|------|---------|--------|------------|--------|
| [SPEC-01](SPEC-01-account-identity-admin.md) | Account — local auth, approval gate, RBAC, admin console, per-user timezone (as-built, retroactive) | `account` | ADR-02, ADR-06; SPEC-05 (`notify:dispatch`; reset is SPEC-05 P0.3) | Built (`0002`–`0004`, `0006`, `0010`, `0031`); P0.13 timezone unbuilt |
| [SPEC-02](SPEC-02-shell-layout.md) | Shell layout — data-driven navigation menu + registry-backed dashboard widget placement (as-built, retroactive) | `layout` + frontend shell | account (`accountapi.HasPermission`, RBAC); SPEC-09 P0.4 consumes the rails | Built (`0036_layout_core`) |
| [SPEC-03](SPEC-03-platform-ops.md) | Platform ops — backup/restore, queue console, takeout | `ops` | — (land P0 before SPEC-12 data accrues) | P0 built (`0012_ops_backup_runs`); P1.7 takeout unbuilt |
| [SPEC-04](SPEC-04-media-image-pipeline.md) | Media image pipeline + asset management | `media` | — | Built (`0008_media_image_pipeline`) |
| [SPEC-05](SPEC-05-notification-module.md) | Notification module (life-stream backbone) | `notify` | SPEC-04 P0.6 (fan-out); P1.2 not a gate | Built (`0009_notify_notifications`, `0010`) |
| [SPEC-06](SPEC-06-docs-canonicalisation.md) | Docs canonicalisation — 79 stale ADR statements, 95 links, four CI docs checks | none (docs + CI) | ADR-11 | Executed 2026-09-11 |
| [SPEC-07](SPEC-07-journal.md) | Journal — life-stream write path | `journal` | SPEC-04 P0.6 (fan-out) only; photos via SPEC-08 | Built (`0011_journal_entries`) |
| [SPEC-08](SPEC-08-journal-attachments.md) | Journal attachments — Attachments (≤ 10 image Assets) and Location as first-class Entry properties; the markdown-link workaround backfilled out of every body (issue #8) | `journal` (extends) · frontend | SPEC-04, SPEC-07, SPEC-09 | Executed 2026-09-19 (`0044`, `0045`) |
| [SPEC-09](SPEC-09-life-stream-home.md) | Life-stream home — projection + dashboard | `journal` + home | SPEC-07; producers attach as they land | Built (`0017_journal_stream_items`) |
| [SPEC-10](SPEC-10-continue-rail.md) | Playback resume + continue rail (D-20) | `media` | SPEC-04 P0.6 (P1.5 event); comic leg after SPEC-14 | Built (`0013_media_playback_progress`) |
| [SPEC-11](SPEC-11-people-registry.md) | People registry — contacts + birthdays | `people` | SPEC-04 P0.3 (scheduler) + P0.6 (fan-out); avatars need SPEC-04 | Built (`0016_people_persons`, `0035`) |
| [SPEC-12](SPEC-12-finance-ledger.md) | Finance ledger (Money-Lover-class) | `bank` | ADR-08; SPEC-03 P0 live before the first real ledger entry; SPEC-04 (P1 receipts only) | Built (`0014_bank_core`, `0042`) |
| [SPEC-13](SPEC-13-ledger-expansion.md) | Ledger expansion — debts, goals, recurring, cards, net worth, automation, splits, sharing | `bank` (extends) | SPEC-12; SPEC-05 for reminders; SPEC-04 (receipts only, via SPEC-12 P1.10) | Phase 1 (debts, `0043_bank_debts`) built; later phases open |
| [SPEC-14](SPEC-14-comic-vertical.md) | Comic vertical, end-to-end | `comic` | SPEC-04 | Built (`0015_comic_core`; reader + import + sync `0024`–`0030`) |
| [SPEC-15](SPEC-15-music-vertical.md) | Music vertical — tracks, zip/multi-file import, enrich + MusicBrainz lookup, playlists, player (as-built, retroactive) | `music` | SPEC-04 (ingest, `/original`, covers, `media:asset_deleted`); SPEC-05 (bell consumer) | Built (`0022_music_core`; `0038`, `0039`, `0041`) |
| [SPEC-16](SPEC-16-movie-vertical.md) | Movie vertical — catalogue over media video assets (as-built, retroactive) | `movie` | SPEC-04, SPEC-14 (pattern), SPEC-10 (asset-level resume) | Backend built (`0021_movie_core`); no frontend |
| [SPEC-17](SPEC-17-story-vertical.md) | Story vertical — stories + Markdown chapters (as-built, retroactive) | `story` | SPEC-04 (covers), SPEC-14 (pattern) | Backend built (`0023_story_core`); reader is a placeholder |
| [SPEC-18](SPEC-18-social-connections.md) | Social connections — request / accept / decline / disconnect; the social layer's first slice (as-built, retroactive) | `social` | account (`accountapi`); SPEC-05 (bell consumers); SPEC-11 (`/people/suggestions`) | Built (`0037_social_connections`) |

The positioning decision (life-OS pivot) and the parking lot are **not** specs;
they live in [ADR-08](../../adr/08-life-os-pivot.md) (with
[vision.md](../vision.md)) and [backlog.md § Deferred](../backlog.md).

## Renumbering (2026-10-01)

On 2026-10-01 the specs were renumbered by **build priority**: the platform and
configuration specs every feature stands on come first, then the features in
dependency order. Files, links and every `SPEC-NN` in `docs/` and `/CLAUDE.md`
were moved to the new numbers in one commit.

**Code was not touched**: comments, test names, `shared/openapi.yaml` and
`.github/workflows/ci.yml` still cite the **old** numbers, and so do commit
messages, GitHub issues and the dated audit
[spec-gap-fix-worklog-2026-09-30.md](../analysis/spec-gap-fix-worklog-2026-09-30.md)
(whose text is never edited). Translate with this table. A code comment is
corrected to the new number when its file is next touched.

| New | Old | Spec | Group |
|-----|-----|------|-------|
| SPEC-01 | SPEC-13 | Account — identity, approval, RBAC, admin console | Platform |
| SPEC-02 | SPEC-18 | Shell layout — menu + widgets | Platform |
| SPEC-03 | SPEC-09 | Platform ops — backup/restore, queue console, takeout | Platform |
| SPEC-04 | SPEC-01 | Media image pipeline + asset management | Platform |
| SPEC-05 | SPEC-04 | Notification module | Platform |
| SPEC-06 | SPEC-11 | Docs canonicalisation | Platform (docs + CI) |
| SPEC-07 | SPEC-05 | Journal | Feature |
| SPEC-08 | SPEC-12 | Journal attachments | Feature |
| SPEC-09 | SPEC-06 | Life-stream home | Feature |
| SPEC-10 | SPEC-07 | Continue rail | Feature |
| SPEC-11 | SPEC-08 | People registry | Feature |
| SPEC-12 | SPEC-03 | Finance ledger | Feature |
| SPEC-13 | SPEC-10 | Ledger expansion | Feature |
| SPEC-14 | SPEC-02 | Comic vertical | Feature |
| SPEC-15 | SPEC-14 | Music vertical | Feature |
| SPEC-16 | SPEC-15 | Movie vertical | Feature |
| SPEC-17 | SPEC-16 | Story vertical | Feature |
| SPEC-18 | SPEC-17 | Social connections | Feature |

Old → new, for reading code: 01→04 · 02→14 · 03→12 · 04→05 · 05→07 · 06→09 ·
07→10 · 08→11 · 09→03 · 10→13 · 11→06 · 12→08 · 13→01 · 14→15 · 15→16 ·
16→17 · 17→18 · 18→02.

## Conventions binding on all specs

- **Spec header**: `Status:` uses [STYLE.md](../../STYLE.md)'s vocabulary
  (`draft` before build; `current` once the spec describes shipped behaviour;
  `historical` for executed one-shot specs such as SPEC-06). There is exactly
  one `Last verified:` field. Implementation state is not restated in headers:
  it lives in `/CLAUDE.md` § Current status and
  [TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md). The PR that
  ships a phase corrects the spec text and bumps `Last verified`.
- **Migrations**: next free numeric sequence, `000N_<owning-module>_<desc>.up/down.sql`.
  Specs are written as `000N_*` placeholders; **verify against the repo**
  (`ls backend/db/migrations | tail -2`) before assigning a number — several
  specs can claim numbers concurrently.
- **Module boundaries**: cross-module access only via the other module's `api/`
  package; coupling via Asynq events `<module>:<event>`; no cross-module JOINs or
  FKs — with two sanctioned **identity-anchor FKs**: `users(id)` (SPEC-05 §6,
  matching the `0007_media_assets` precedent) and `organizations(id)` via
  `tenant_id` ([ADR-07](../../adr/07-tenancy-rls-model.md)).
- **Tenancy (ADR-07)**: every user/org-data table carries
  `tenant_id uuid NOT NULL DEFAULT current_setting('app.current_tenant')::uuid REFERENCES organizations(id)`
  plus `CREATE INDEX <t>_tenant_idx ON <t>(tenant_id)`. The migration that
  creates the table runs `ENABLE` + `FORCE ROW LEVEL SECURITY` and
  `CREATE POLICY tenant_isolation … USING (tenant_id = current_setting('app.current_tenant')::uuid) WITH CHECK (same)`,
  per `0020_platform_rls_enable` and `0043_bank_debts`. Shared seed rows visible
  to every tenant use a nullable `tenant_id` and `USING (… OR tenant_id IS NULL)`
  (the `bank_categories` precedent), and migrations insert them with explicit
  `tenant_id = NULL`. Request paths run inside `platform/db.BeginTenantScope`
  (`RequireTenant`). Worker handlers open the payload user's tenant scope before
  any query. Periodic sweeps iterate tenants through `forEachTenant` in
  `cmd/worker` (one committed scope per tenant). `pg_dump` connects as the
  superuser/owner role via `BACKUP_DATABASE_URL` — as `portal_app` under FORCE
  RLS it would see no tenant's rows. Global tables are exempt: the account
  tables (`users`, `roles`, `permissions`, `role_permissions`, `user_roles`,
  `refresh_tokens`, `password_reset_tokens` — read before any tenant is
  resolved, SPEC-01 §6) and the shell tables `layout_menu_items` /
  `layout_widgets` (one instance-wide shell of configuration rows, readable by
  every signed-in user and filtered per permission in the service; fenced per
  tenant, each personal tenant would get an empty menu — SPEC-02 §6). So are
  system tables (`ops_backup_runs`). `social_connections` has no `tenant_id`
  but is not exempt: it is fenced by **per-user** RLS on `app.current_user`
  (`0037`, SPEC-18 §6), because a connection spans two personal tenants. Each
  spec §6 states per table whether it is
  tenant-scoped; DDL in specs drafted before ADR-07 omits the columns, and this
  bullet governs.
- **Events**: every task/event name lands in
  [../../reference/events.md](../../reference/events.md) as part of definition of
  done; every new domain module emits ≥ 1 bus event from day one (ADR-08).
  There is no exemption *(owner decision 2026-10-01b (D3))*: `account` and
  `layout`, the two shipped modules that emit none on `HEAD`, emit — seven
  `account:*` admin-change events and `layout:changed`, which notify delivers
  to superadmins (SPEC-01 §8 and P1.3, SPEC-02 §8 and P1.4, SPEC-05 P1.5;
  code follow-up SPEC-01 §11 row 24, SPEC-02 §11 row 14). The
  `platform/events` fan-out helper (`Publish(ctx, name, payload)` + the
  event-name→consumer-task subscription table) was built by SPEC-04 P0.6 — a
  prerequisite of the first spec to land (SPEC-04 P0.6 in the build order;
  otherwise whichever lands first builds it). The table is **per-binary**: every
  binary that **emits** an event registers that event's consumer edges on its
  own publisher (`cmd/api/main.go` for HTTP-emitted events, `cmd/worker/main.go`
  for worker-emitted ones; also wiring the other binary is harmless because
  `Subscribe` is idempotent). `Publish` on a name the emitting binary has not
  registered is a silent no-op (events.md "What is NOT enforced"). **DoD**: the
  events.md row and the `Subscribe(` call in each emitting binary land in the
  same PR.
- **Errors**: RFC 7807 `Problem` on every non-2xx (D-7). Every Problem `type` a
  spec introduces is a relative slug `<module>/<kebab-reason>` and doubles as the
  i18n key. **DoD**: the same PR adds it to `ProblemType` and `PROBLEM_MESSAGES`
  in `frontend/src/lib/problems.ts`, the live catalog (the next-intl
  `errors.json` in `frontend.md` §5.2 is the future target). Generic transport
  failures use `about:blank` and need no key. A backend-emitted type absent from
  `problems.ts` is a DoD failure.
- **Pagination**: list endpoints use an opaque base64 keyset
  `?cursor=&limit=`. Each spec §7 states the default and max limit; a new
  endpoint that states none uses default 30, max 50 (existing endpoints keep
  their declared defaults and maxima). *(Owner decision 2026-10-01:)* **`limit`
  is lenient and never an error: missing, non-integer or < 1 → the endpoint's
  declared default; above the endpoint's max → clamped to the max.** No
  Problem type is ever emitted for `limit`, and OpenAPI describes it that way
  (its `minimum`/`maximum` document the range, not a 4xx). The shared
  `platform/server.Limit(r, def, max)` helper already implements the rule.
  **Every collection response is `{items}`** *(owner decision 2026-09-30,
  confirmed 2026-10-01)*: a cursor-paginated list responds
  `{items: [...], next_cursor: string}` with `next_cursor` absent (or null) on
  the last page, and a non-paginated list responds `{items: [...]}` too — so a
  list that later gains pagination or metadata changes no client. Extra
  top-level fields that are not the list itself are allowed alongside `items`
  (e.g. notifications' `unread_count`, budgets' `month`); a resource-named
  list key (`{assets}`, `{comics}`, `{people}`, …) is not. A collection
  response is any response whose payload is a list of resources (every list
  endpoint, paginated or not); a single resource or a composite read — a
  dashboard, a report, a detail with embedded children — keeps its arrays as
  named fields of that object (`ComicDetail.chapters`, the bank report's
  `expenses`/`incomes`/`trend`). *(Owner decision 2026-09-30:)* endpoints that
  shipped before this rule are
  **retrofitted, not grandfathered** — only their limits are kept; each such
  spec §7 states the `{items}` shape and carries a code follow-up for the
  handler, `shared/openapi.yaml` and the frontend readers. *(The lists this
  note once called unowned now have specs: music `{tracks}`, `{imports}` and
  `{playlists}` — SPEC-15 §12 row 15; social `{connections}` — SPEC-18 §11
  row 4; story `{stories}` and `{chapters}` — SPEC-17 §11 row 9. Code
  follow-up for the one list no spec owns: people `{suggestions}` — the route
  is the `people` module's, but SPEC-11 P0.2 leaves its contract unowned and
  SPEC-18 only consumes it — retrofits to `{items}` the same way.)* The
  ordering key
  ends in `id`. A malformed cursor is 400 `<module>/invalid-cursor`, and a
  body/param-shape failure without a named type is 422 `<module>/validation`.
  Each §7 lists both for every list endpoint.
- **Timezone** *(owner decision 2026-09-30; D-17)*: every user-facing day or
  month boundary — "today", "this month", a default month, a date-only value
  turned into an instant, day grouping, on-this-day, birthday and due-date
  countdowns — is computed in **the user's own timezone**: `users.timezone`, an
  IANA name taken from the user's location, stored per user. Unknown or
  unparseable → `Asia/Ho_Chi_Minh` (the column default; an unparseable stored
  name also logs a warning). There is no instance-wide fallback: `APP_TIMEZONE`,
  "the instance default" and UTC are not v1 sources for user-facing
  boundaries. **Manual flag** *(owner decision 2026-10-01)*:
  `users.timezone_manual boolean NOT NULL DEFAULT false`. While it is true
  the device-detected zone never overwrites the stored one. **Write path**:
  `PATCH /api/v1/auth/me {timezone, timezone_manual?}` (authenticated; the
  caller's own row only) — `timezone` is required, `timezone_manual` omitted
  leaves the flag unchanged; the response is the updated `/auth/me` body. The
  frontend detects the device zone
  (`Intl.DateTimeFormat().resolvedOptions().timeZone`, i.e. the user's current
  location) after sign-in and, when `timezone_manual` is false and the zone
  differs from the stored value, saves `{timezone}`; settings offer a manual
  override (an IANA picker, saved as `{timezone, timezone_manual: true}`) and
  "use my location", which saves `{timezone: <device zone>, timezone_manual:
  false}`. The API validates the name with `time.LoadLocation`; an empty or
  unknown IANA name is 422 `account/invalid-timezone` and nothing is written.
  The slug is registered in `frontend/src/lib/problems.ts` (Errors
  convention). `GET /auth/me` returns both `timezone` and `timezone_manual`.
  **Readers**:
  the frontend reads the zone from `GET /auth/me`; backend modules read it
  through `accountapi` (`UserSummary.Timezone`), never by querying `users`.
  **Sweeps**: a periodic task never uses one "today" for everyone — it runs
  often enough (hourly, D-17's per-TZ pattern) and evaluates each user's local
  date in that user's zone, relying on its dedup keys for exactly-once. SQL
  converts at the query layer (`occurred_at AT TIME ZONE $tz`), never through
  the session zone (UTC). *(Code follow-up — SPEC-01 P0.13, tracked as SPEC-01
  §11 row 14 and as the first item of the **Per-user timezone** cross-cutting
  gap below: `0002_account_users` ships `timezone DEFAULT 'UTC'` — a new
  migration changes the default to `'Asia/Ho_Chi_Minh'`, rewrites the
  untouched `'UTC'` rows and adds `timezone_manual boolean NOT NULL DEFAULT
  false`; there is no `PATCH /auth/me` and no `account/invalid-timezone` in
  `problems.ts`, `CurrentUser` and `accountapi.UserSummary` carry no
  timezone, and the frontend's `lib/time.ts` takes its display zone from
  `GET /api/v1/time`, i.e. `APP_TIMEZONE` — it switches to the user's zone and
  `/time` keeps only the server clock.)*
- **updated_at**: there is no trigger; every UPDATE and every
  `ON CONFLICT … DO UPDATE` in `query/*.sql` sets `updated_at = now()`
  explicitly.
- **AuthZ**: every endpoint lists its required permission (or an explicit
  "authenticated"), enforced via `RequirePermission`; the grammar is **strictly
  2–3 segments** `<resource>:<action>[:<scope>]` — `rbac.Parse` rejects
  4-segment codes and `AllowsCode` is fail-closed. §7 tables give exactly one
  literal, parseable code per method (split `GET/POST` rows); shorthand like
  `read/write` or `read|write` is not a code, and copying it into
  `RequirePermission` panics at start. **Canonical naming scheme**
  (reconciles the drafts; matches `rbac/permission.go`'s examples and the
  0003 catalog's domain rows (`assets:*`, `movies:*`, ...) — 0003's
  admin-plane codes (`rbac:role:*`, `system:settings:write`, `moderation:*`)
  predate this scheme and are grandfathered literal codes; they must not be
  used as templates for new modules): resource = plain noun, kebab-compound
  where needed (`assets`, `comics`, `bank-accounts`, `notification-prefs`);
  action ∈ `read | write | delete` plus sparing domain verbs (`publish`);
  scope ∈ `own | any` **only** — the matcher special-cases exactly those (a
  literal scope like `published` would only ever match its own literal grant).
  **Seeding rule**: a spec that introduces a permission also names the
  receiving role and ships the `permissions` + `role_permissions` seed rows in
  its own migration (0003's `WITH grants(...)` pattern) — an unseeded code
  403s everyone below superadmin. Grants to `user` are the floor: any
  capability a `user`-granted feature depends on (including `assets:write:own`)
  must itself be granted to `user`; `creator`/`editor` hold only
  catalogue/moderation tiers. **OpenAPI encoding**: public operations declare
  `security: []`; authenticated operations declare
  `security: [{bearerAuth: []}]`. An operation behind `RequirePermission`
  additionally carries `x-required-permission: <code>`; one behind
  `RequireOwnerOrPermission` carries
  `x-required-permission: {owner_or: <elevated code>, also: <code>?}`.
  Combined-method §7 rows are split per operation. No operation carries the
  annotation yet ([security.md](../../architecture/security.md) calls it the
  target convention) and no CI check compares it with the route middleware;
  until a retrofit annotates every operation and a drift check exists, it is a
  review item.
- **API contract**: declare every endpoint in `shared/openapi.yaml` first, run
  `make openapi`, and commit the regenerated `api.gen.go` + `types.gen.ts` in the
  same PR ([ADR-10](../../adr/10-openapi-contract-direction.md), accepted
  2026-07-11; the CI `openapi` job diffs them; `backend/MODULES.md` §8). The gate
  proves presence only; handler conformance stays a review item until handlers
  implement `ServerInterface`.
- **Money**: integer minor units, never floats. VND exponent = 0. *(Ratified
  divergence: D-41 — integer minor units for the v1 ledger only; D-14/D-15
  still govern multi-currency/creator-economy money. Rationale in SPEC-12 §7.)*
- **Frontend**: RSC-first (D-33), TanStack owns server state (D-32), performance
  budgets from `frontend.md` §8 apply to all new pages. **"RSC shell"** (binding
  until `src/lib/api-server.ts` exists): the route `page.tsx` is a server
  component that exports `metadata` and renders the template view; all
  authenticated data is fetched in client islands via TanStack (D-32). A spec
  that wants server-side fetching must own building `api-server.ts`
  (`server-only` + `cookies()` forwarding) and the 401 path when `portal_access`
  has lapsed. Every new route group under `app/(app)/` extends `config.matcher`
  in `frontend/src/middleware.ts` (D-34 edge gate) in the same PR; a route group
  absent from the matcher is a DoD failure.
- **Takeout**: every module that stores user-authored or user-history data
  implements the `opsapi.ExportProvider` interface (SPEC-03 P1.7) from its own
  `api/` package, in the PR that creates the table — or its spec states the
  exclusion and the reason. The spec's §6 names the export format per table.
  (The interface itself is unbuilt until SPEC-03 P1.7 lands; until then, the
  spec records the intended format.)
- **Definition of done (every spec PR)** also includes a
  `docs/testing/TEST-CASES-SPEC-NN-<module>.md` with rows citing the real
  `_test.go` evidence, and the spec's rows in
  [TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md).

## Build state and what remains

ADR-10's spec-first CI gate is in force; the `ServerInterface` retrofit is
per-module-as-touched (ADR-10 action items) and gates nothing.

**As built.** Every spec that existed then (today's SPEC-03, 04, 05, 07, 09, 10, 11, 12 and 14) landed on 2026-07-12, in
migration order `0008` (media) → `0009`/`0010` (notify) → `0011` (journal) →
`0012` (ops backups) → `0013` (playback progress) → `0014` (bank) → `0015`
(comic) → `0016` (people) → `0017` (stream projection)
(`git log --diff-filter=A --format='%h %ad' -- backend/db/migrations/`). SPEC-03
P0 (`0012`, `6160f8e`) therefore preceded the SPEC-12 ledger (`0014`,
`66c036f`). The stream projection (`0017`) landed *after* its producers,
contrary to SPEC-09 §1; any event published before it was never projected
(SPEC-09's P1.6 backfill, which only ever covered media assets, is retired).

**Sequencing rules that still bind** (for rebuilds and new producers):

1. **SPEC-04** (shared bottleneck; P0.6 fan-out is everyone's prerequisite).
2. **SPEC-05** (consumer for `media:asset_ready`; unblocks password reset).
3. **SPEC-07** + **SPEC-09 P0.1** (journal module + the `stream_items`
   projection and its `journal:stream_*` consumers; must precede every producer
   below, SPEC-09 §1).
4. **SPEC-14** and **SPEC-03 P0** (backups — hard gate immediately before
   SPEC-12).
5. **SPEC-12** (after SPEC-03 P0 is live; in parallel with SPEC-10, the
   burst-filler); **SPEC-03 P1** here or later.
6. **SPEC-11**, then **SPEC-09 P0.2–P0.4 + P1** (read API, home, rail).

**What remains open** (verify against the matrix before starting):

- SPEC-03 P1.7 owner takeout — unbuilt (no `ExportProvider` in code).
- SPEC-13 phases after phase 1 (debts) — build on SPEC-12 + SPEC-05.
- Every built spec still diverges from its shipped code in places: see the
  [Implementation gaps index](#implementation-gaps-index-2026-10-01) below.

## Implementation gaps index (2026-10-01)

Every built spec ends with a section **"Implementation gaps vs shipped code (as
of 2026-10-01)"**: one row per place where `main` @ `99b5a0b` still diverges
from the spec text, naming the shipped file and function, the change needed
(migration · backend · openapi · frontend · test) and the finding id. That
section is the **live record of the divergence** (what is wrong and what
closes it); *when* it is worked is ranked only in [backlog.md](../backlog.md). A
row closes, and is deleted, in the PR that makes
the code match and regrades the spec's
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) rows. This
index only counts and routes; the detail lives in the spec. SPEC-06 is a
historical docs-only spec and has no gap section.

Row ranges use each section's own severity order (its intro paragraph names
it): **Sec** security · **Data** data loss or silently wrong / lost data ·
**Integ** integrity or a wrong answer · **AuthZ** · **Contract** API, OpenAPI,
Problem types · **UX** frontend behaviour · **Hyg** schema hygiene · **P1**
unbuilt P1 feature. Rows added by the 2026-10-01 owner decisions, rows found
later the same day while writing SPEC-01…18 (SPEC-14 rows 18–19, SPEC-15 row
25), and rows added by the second round of decisions (SPEC-04 row 19, SPEC-05
row 23, SPEC-01 row 24, SPEC-15 row 26, SPEC-16 row 18, SPEC-17 row 20,
SPEC-02 row 14) are appended at the end of their table instead of renumbering
the rows other documents cite, so a few ranges below are split.

| Spec | Gap section | Rows | By severity (row numbers) | Most severe |
|------|-------------|-----:|---------------------------|-------------|
| [SPEC-01](SPEC-01-account-identity-admin.md) | [§11](SPEC-01-account-identity-admin.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 24 | Sec 1–5 · AuthZ 6–7 · Data 8 · Integ 9–13 · Func (timezone) 14 · Contract 15–20 · UX 21–22 · Hyg 23 · P1 24 | re-parenting a role under `superadmin` escalates every holder to `*` (1); refresh rotation is check-then-act, so two concurrent presentations fork the chain (2) |
| [SPEC-02](SPEC-02-shell-layout.md) | [§11](SPEC-02-shell-layout.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 14 | Sec 1 · Integ 2–4 · UX 5 · Contract 6–8 · Test 9 · UX 10 · Test 11 · Hyg 12 · P1 13–14 | `href` guard bypassed by `/\` and control characters — an open redirect in the menu shown to every user (1) |
| [SPEC-03](SPEC-03-platform-ops.md) | [§11](SPEC-03-platform-ops.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 11 | Sec 1–4 · Data 5–6 · Func 7–8 · Contract 9 · Hyg 10 · P1 11 | queue console writable by any `queues:read` holder, no CSRF guard (1); the restore drill can only reach the dev MinIO (2) |
| [SPEC-04](SPEC-04-media-image-pipeline.md) | [§11](SPEC-04-media-image-pipeline.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 19 | Data 1–3, 19 · Sec/Integ (`/original`) 4–6 · Integ 7–8 · AuthZ 9 · Contract/UX 10–13, 15–18 · P1 14 | DELETE's 500 rolls back the `deleting` tombstone after objects are purged (1); `/original` streams abandoned or purged uploads, sized from the client's claim (4–6) |
| [SPEC-05](SPEC-05-notification-module.md) | [§11](SPEC-05-notification-module.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 23 | Sec 1–2 · Data 3–4 · Func/UX 5–12 · Contract 13–16, 22 · Hyg 17–20 · P1 21, 23 | plaintext reset token in the Asynq payload (1); reset token consumed check-then-act, the user's other tokens not revoked (2) |
| [SPEC-07](SPEC-07-journal.md) | [§11](SPEC-07-journal.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 8 | Data 1 · UX 2–5 · Contract 6–8 | `journal:entry_created` published before the request transaction commits (1) |
| [SPEC-08](SPEC-08-journal-attachments.md) | [section](SPEC-08-journal-attachments.md#implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 2 | AuthZ 1 · UX 2 (both owned by other specs) | no `user` grant of `assets:write:own` and no enforcement on upload (1) |
| [SPEC-09](SPEC-09-life-stream-home.md) | [§11](SPEC-09-life-stream-home.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 16 | Data / stale 1–5 · Integ (order, render) 6–10 · UX (home, rail, `/weather` gate) 11–14 · Contract 15 · P1 16 | stream unique key lacks `user_id`, so a second user's playback is swallowed (1); deleted or edited birthdays leave stale cards (2) |
| [SPEC-10](SPEC-10-continue-rail.md) | [§11](SPEC-10-continue-rail.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 14 | Data 1–3 · Integ 4–7 · Contract/UX 8–13 · P1 14 | the `pagehide` save is a `sendBeacon` POST → 405, silently lost (1); completion latched even when Publish fails, so the event is lost (2) |
| [SPEC-11](SPEC-11-people-registry.md) | [§11](SPEC-11-people-registry.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 15 | Data (lost / duplicate events) 1–4 · Integ (dates) 5–7 · Contract/UX 8–12, 14–15 · P1 13 | every PATCH carrying a birthday resets notices → duplicate stream items (1); no revoke / delete events (2) |
| [SPEC-12](SPEC-12-finance-ledger.md) | [§12](SPEC-12-finance-ledger.md#12-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 17 | Sec 1 · Data 2–5 · Contract 6–12, 17 · UX 13–16 | derived-balance queries (accounts, dashboard) not filtered by the caller (1); archived accounts still accept writes (2) |
| [SPEC-13](SPEC-13-ledger-expansion.md) | [§8](SPEC-13-ledger-expansion.md#8-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 5 | Data 1 · Scheduling 2 · Contract 3–5 | opening a debt is three writes with no enclosing transaction (1) |
| [SPEC-14](SPEC-14-comic-vertical.md) | [§11](SPEC-14-comic-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 19 | Sec 1–5 · Integ 6–9 · Contract/UX 10–17 · Data 18 · Integ 19 | `/api/v1/internal/*` public at the edge, secret compared with `!=` (1–3); publish never checks `comics:publish:own`, drafts leak as 403 (4–5) |
| [SPEC-15](SPEC-15-music-vertical.md) | [§12](SPEC-15-music-vertical.md#12-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 26 | Data 1–2, 4 · Integ 3, 5–13 · Contract 14–17, 22–23 · UX 18–19, 24 · Hyg 20–21 · AuthZ 25 (appended) · P1 26 | the `media:asset_deleted` consumer runs with no tenant scope, so a deleted audio asset leaves a published track pointing at nothing (1); a second zip upload re-imports every track (2) |
| [SPEC-16](SPEC-16-movie-vertical.md) | [§11](SPEC-16-movie-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 18 | Data 1–3 · Integ 4–5 · AuthZ 6–7 · Contract 8–13 · Hyg 14–15 · P1 16–18 | the `media:asset_deleted` consumer runs with no tenant scope (1); clearing the video leaves the movie published (2) |
| [SPEC-17](SPEC-17-story-vertical.md) | [§11](SPEC-17-story-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 20 | Data 1 · Integ 2–6 · AuthZ 7–8 · Contract 9–15, 17 · Hyg 16 · P1 18–20 | the `media:asset_deleted` consumer runs with no tenant scope (1); a chapter created without `sort_order`, or with a duplicate, is a 500 at COMMIT (2) |
| [SPEC-18](SPEC-18-social-connections.md) | [§11](SPEC-18-social-connections.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 12 | Data 1 · Integ 2–3 · Contract 4–6 · UX 7 · Test/docs 8–9 · Contract 10–11 · Hyg 12 | events published before COMMIT leave a phantom bell entry (1); a concurrent duplicate request aborts the transaction → 500 (2) |
| **Total** | | **263** | | |

**Cross-cutting gaps** — one change closes rows in several specs; land it as
one change (or one PR per module in a fixed order) and close every row it
names:

- **Envelopes retrofit to `{items}`** (Pagination convention): SPEC-04 §11
  row 12 · SPEC-14 §11 row 12 · SPEC-12 §12 row 9 · SPEC-11 §11 rows 9, 11 ·
  SPEC-13 §8 row 3 · SPEC-01 §11 row 18 · SPEC-15 §12 row 15 · SPEC-16 §11
  row 8 · SPEC-17 §11 row 9 · SPEC-18 §11 row 4; plus the one unowned list
  (people `{suggestions}`) the Pagination convention names. Each retrofit
  moves handler, `shared/openapi.yaml` and the frontend readers in one PR.
- **Per-user timezone** (Timezone convention, Decisions 2026-09-30 and
  2026-10-01 (f)): the account change comes first; it is SPEC-01 P0.13 and its
  gap row is SPEC-01 §11 row 14, summarised here:
  1. **migration** (account-owned): `users.timezone` default
     `'Asia/Ho_Chi_Minh'`, untouched `'UTC'` rows rewritten, and
     `timezone_manual boolean NOT NULL DEFAULT false` added;
  2. **backend** (`account`): `PATCH /api/v1/auth/me {timezone,
     timezone_manual?}` validating with `time.LoadLocation` → 422
     `account/invalid-timezone`; `GET /auth/me` / `CurrentUser` carry
     `timezone` and `timezone_manual`; `accountapi.UserSummary.Timezone` (plus
     a batch lookup for sweeps);
  3. **openapi**: both operations and the 422;
  4. **frontend**: `account/invalid-timezone` in `problems.ts`; the post-sign-in
     device-zone save (skipped while `timezone_manual`), the settings picker and
     "use my location"; `lib/time.ts` takes the zone from `/auth/me`, and
     `GET /api/v1/time` keeps only the server clock.

  Then its readers: SPEC-12 §12 rows 8, 16 · SPEC-07 §11 row 2 · SPEC-09 §11
  rows 3, 9, 16 · SPEC-11 §11 rows 5–6 · SPEC-13 §8 row 2.
- **`problems.ts` slugs** (Errors convention): SPEC-04 §11 row 12 · SPEC-14
  §11 row 10 · SPEC-12 §12 rows 2, 8 · SPEC-05 §11 row 14 · SPEC-07 §11
  row 6 · SPEC-09 §11 row 15 · SPEC-10 §11 row 13 · SPEC-11 §11 row 12 ·
  SPEC-13 §8 row 5 · SPEC-01 §11 row 16 (`account/invalid-timezone` among
  them) · SPEC-15 §12 row 23 · SPEC-16 §11 row 10 · SPEC-17 §11 row 11 ·
  SPEC-18 §11 row 6.
- **OpenAPI `x-required-permission`** (AuthZ convention, OpenAPI encoding):
  SPEC-04 §11 row 17 · SPEC-14 §11 row 16 · SPEC-12 §12 row 12 · SPEC-05 §11
  row 16 · SPEC-07 §11 row 7 · SPEC-09 §11 row 15 · SPEC-03 §11 row 9 ·
  SPEC-13 §8 row 5 · SPEC-01 §11 row 20 · SPEC-15 §12 row 22 · SPEC-16 §11
  row 13 · SPEC-17 §11 row 17 · SPEC-18 §11 row 10 · SPEC-02 §11 row 6. Best
  done as one retrofit together with the drift check the convention asks for.
- **`limit` clamps instead of resetting** (Pagination convention, Decision
  2026-10-01 (e)): SPEC-04 §11 row 18 · SPEC-14 §11 row 17 · SPEC-12 §12
  row 17 · SPEC-05 §11 row 22 · SPEC-07 §11 row 8 · SPEC-09 §11 row 8 ·
  SPEC-11 §11 row 15 · SPEC-01 §11 row 19 (`accountapi.ListDirectory`) ·
  SPEC-15 §12 row 16 (catalogue and imports) · SPEC-16 §11 row 9 · SPEC-17
  §11 row 10; plus the unowned people-suggestions list, which resets the same
  way. Each is one line (or a call to `platform/server.Limit`, which already
  clamps) plus a test; `/continue` already clamps.
- **Publish after commit** (CC-5; the mechanism SPEC-07 §11 row 1
  introduces): SPEC-07 §11 row 1 · SPEC-15 §12 row 4 · SPEC-16 §11 row 3 ·
  SPEC-17 §11 row 4 · SPEC-18 §11 row 1. Each emits its bus event inside the
  open `RequireTenant` transaction, so a failed COMMIT still leaves a bell
  entry; land one after-commit hook (or outbox) and move every emitter onto it.
- **Unscoped `media:asset_deleted` consumers** (Tenancy convention: worker
  handlers open the payload user's tenant scope before any query): SPEC-14
  §11 row 18 · SPEC-15 §12 row 1 · SPEC-16 §11 row 1 · SPEC-17 §11 row 1. The
  comic, movie, music and story consumers run their UPDATE/DELETE on the bare
  pool; under `portal_app` the FORCE RLS policy cannot evaluate
  `app.current_tenant`, so the task fails every retry and dangling references
  (published items with no media) survive. The fix is the same in each: parse
  `owner_user_id` and run the handler through `cmd/worker`'s
  `runInUserTenant` (movie and story need a `RunInTenant` dep first; comic and
  music already receive one but their consumers ignore it), plus one RLS-suite
  test that runs a consumer as `portal_app`. A payload owner who no longer
  has a personal organisation (the account was deleted, SPEC-01 P0.10) has
  nothing left to repair: the consumer drops the task instead of retrying.
- **F009 — `user` upload grant**: SPEC-04 §11 row 9 (owner) · SPEC-08 row 1.
  Seed the grant in the same migration that adds `RequirePermission` to the
  upload routes, or every `user` loses photo upload.
- **`user` may author music, movies and stories** (Decision 2026-10-01b (D4)):
  SPEC-15 §12 row 26 · SPEC-16 §11 row 18 · SPEC-17 §11 row 20 — one grant
  migration per owning module (`000N_<module>_user_write_grant`, the `0025`
  shape), each useful only with F009 above; SPEC-15 row 20 then requires
  `music:publish:own`.
- **Admin-change events → superadmin notices** (Decision 2026-10-01b (D3)):
  SPEC-01 §11 row 24 (`account:*` emits, `SuperadminIDs`) · SPEC-02 §11 row 14
  (`layout:changed`) · SPEC-05 §11 row 23 (the eight `notify:on_*` consumers).
  Land the notify consumers with or before the first emitter; a `Publish`
  whose consumer edge is not registered is a silent no-op.
- **Deleting a user purges their media first** (Decision 2026-10-01b (D1)):
  SPEC-04 §11 row 19 (`mediaapi.PurgeOwnerAssets`) before SPEC-01 §11 row 8
  (the delete order that calls it).
- Smaller shared items: `origin='import'` (F038/F012) SPEC-04 row 10 · SPEC-14
  row 7 · SPEC-05 row 8 · SPEC-08 row 2 · SPEC-15 row 17; media deep link
  (F018) SPEC-04 row 15 · SPEC-05 row 9 · SPEC-09 row 7 · SPEC-10 row 9; Audio
  SPEC-04 row 16 · SPEC-10 rows 4, 5, 8; keepalive `PUT` instead of
  `sendBeacon` (F001) SPEC-14 row 11 · SPEC-10 row 1; `bulk` queue (F095)
  SPEC-14 row 8 · SPEC-05 row 19 · SPEC-03 row 11 · SPEC-15 row 11; people
  retraction events (F017) SPEC-11 row 2 · SPEC-09 row 2; optimistic placement
  (F015) SPEC-07 row 4 · SPEC-09 row 10; `:publish:own` never checked (F051)
  SPEC-14 row 4 · SPEC-16 row 6 · SPEC-17 row 7; a stranger's draft answers
  403 on guarded writes (F119) SPEC-14 row 5 · SPEC-15 row 25 · SPEC-16 row 7
  · SPEC-17 row 8; chapter `sort_order` on create → 500 at COMMIT SPEC-14
  row 19 · SPEC-17 row 2; D-34 matcher (F032) SPEC-09 row 14 (`/weather`) ·
  SPEC-01 row 21 and SPEC-02 row 10 (both `/admin` — one change closes both);
  `/calendar` is unowned.

**Order of work.** This index says *what* diverges; the order in which to
close it is [backlog.md](../backlog.md) § "P0 — order for closing spec gaps",
the single priority list for all open work.

## Decisions recorded 2026-09-30

The owner settled the three decisions the 2026-09-30 review left open (marked
decided in the
[2026-09-30 worklog](../analysis/spec-gap-fix-worklog-2026-09-30.md), "Open
decisions left by the post-fix review"). The detail lives in the places named
here; this list does not restate it. The gap rows cite them as
"Decision 2026-09-30".

- **Envelopes** — every list answers `{items}` (paginated lists add
  `next_cursor`), non-paginated lists included; shipped endpoints are
  retrofitted, not grandfathered. Detail: the **Pagination** convention above;
  each spec §7; the D-29 update in
  [feature-inventory.md](../feature-inventory.md); the Envelopes rows indexed
  above.
- **Timezone** — per user, from the device location, default
  `Asia/Ho_Chi_Minh`, with a `timezone_manual` override; sweeps evaluate each
  owner's local date. Detail: the **Timezone** convention above; the D-17
  update in [feature-inventory.md](../feature-inventory.md); SPEC-12 P0.6,
  SPEC-07 P0.4, SPEC-09 P0.1 / P0.3 / P1.5, SPEC-11 P0.3 / P0.4, SPEC-13 §4a.
- **Audio opens the player** — audio is a playable kind: player page, plays
  from `/original`, saves progress, joins `/continue`. Detail: SPEC-10 P0.1,
  P0.2, P0.4; SPEC-04 §11 row 16; the D-20 update in
  [feature-inventory.md](../feature-inventory.md).

## Decisions recorded 2026-10-01

The owner settled the seven questions the 2026-10-01 gap verification left
open. The detail lives in the places named here; the gap rows cite them as
"Decision 2026-10-01".

- **(a) `GET /bank/report` is documented, not retired** — SPEC-12 P1.11
  (contract and ACs) and its §7 row; TC-BANK-185…190; the SPEC-12 P1.11 row
  of the matrix. No new gap row: its month handling is SPEC-12 §12 row 8, its
  OpenAPI annotation row 12.
- **(b) Debt movements use `wallet_id`** — defined in SPEC-13 §6 (the user's
  own non-debt account the money moves through). The old SPEC-13 §8 row 4
  (rename to `account_id`) is gone; row 4 is now the non-debt check `HEAD`
  lacks.
- **(c) `people/already-in-registry` is declared** — SPEC-11 P0.2 (409 on a
  second link of one account) and §7 Problem types; §11 row 12 now covers only
  its missing `problems.ts` entry, row 14 the `linked_user_id` shape rule;
  TC-PPL-021, TC-PPL-022.
- **(d) The weather widget is in scope** — SPEC-09 §3 (no longer a non-goal)
  and P0.4 (requirement and ACs); §11 row 14 is now the `/weather` matcher gap;
  TC-STREAM-073…076; brief 06 annotated (since folded into SPEC-09 and deleted).
- **(e) `limit` is lenient and clamps** — the **Pagination** convention above;
  every §7 that declares a limit; one gap row per service that resets instead
  (the `limit` cross-cutting gap below); the `limit` TCs; the D-29 update in
  [feature-inventory.md](../feature-inventory.md).
- **(f) `timezone_manual` and `account/invalid-timezone` are confirmed** — the
  **Timezone** convention above; the D-17 update in
  [feature-inventory.md](../feature-inventory.md);
  [frontend.md](../../architecture/frontend.md) §5.4;
  [security.md](../../architecture/security.md)'s route list. The code
  follow-up is SPEC-01 P0.13 (§11 row 14), the first item of the **Per-user
  timezone** cross-cutting gap below.
- **(g) `{items}` for non-paginated lists is confirmed** — the **Pagination**
  convention above (with the rationale and what counts as a collection); the
  D-29 update in [feature-inventory.md](../feature-inventory.md). Every spec §7
  complies; the shipped divergences are the Envelopes rows indexed above.

## Decisions recorded 2026-10-01 (second round)

The owner settled four of the questions the as-built specs (SPEC-01, 02 and 15–18) raised (seven spec
questions, since one decision covered three specs and another two). The detail
lives in the places named here; the gap rows cite them as "Decision
2026-10-01b".

- **(D1) Deleting a user purges their media first** — SPEC-01 P0.10 (the
  delete order: disable → `mediaapi.PurgeOwnerAssets` → a final pass under
  `FOR UPDATE` on the user row that must leave zero rows → `DeleteUser`;
  otherwise 503 `account/delete-incomplete` and the user is kept, with the
  reasoning why tombstoning alone would not survive the cascade) and SPEC-04
  P0.7 (the media method). Gap rows: SPEC-01 §11 row 8 (rewritten),
  SPEC-04 §11 row 19 (new); TC-ACC-064…066, TC-MEDIA-052…054.
- **(D2) Movie and story are finished, not reverted** — SPEC-16 P1.1 and
  SPEC-17 P1.1 are committed scope (still unbuilt), to the music standard;
  [backlog.md](../backlog.md) P2 line 28 records the decision. No new gap row:
  the frontends are SPEC-16 §11 row 16 and SPEC-17 §11 row 18.
- **(D3) `account` and `layout` emit bus events; notify tells the
  superadmins** — no exemption from ADR-08's rule (the **Events** convention
  above). SPEC-01 P1.3 and §8 (seven `account:*` events, `accountapi.AdminEvent`,
  `SuperadminIDs`), SPEC-02 P1.4 and §8 (one `layout:changed {part}`), SPEC-05
  P1.5 (eight `notify:on_*` consumers, recipients = superadmins minus the actor,
  registration deduplicated against the approver dispatch, email forced only
  for refresh-token reuse); [events.md](../../reference/events.md) planned rows.
  Gap rows: SPEC-01 §11 row 24, SPEC-02 §11 row 14, SPEC-05 §11 row 23;
  TC-ACC-120…126, TC-LAY-050…052, TC-NOTIFY-140…145.
- **(D4) `user` may author music, movies and stories** — SPEC-15 P1.4,
  SPEC-16 P1.3, SPEC-17 P1.3 (each a module-owned `000N_<module>_user_write_grant`
  granting `:write:own` and `:publish:own` to `user`, the
  `0025_comic_user_write_grant` shape; `:any` and delete-any unchanged; useful
  with F009). Gap rows: SPEC-15 §12 row 26, SPEC-16 §11 row 18, SPEC-17 §11
  row 20; TC-MUS-004, TC-MOV-050, TC-STY-070.

## Open owner decisions (2026-10-01)

The 2026-09-30 round and both 2026-10-01 rounds are closed (above). Of the 22
questions the as-built specs (SPEC-01, 02 and 15–18) raised, seven were decided in the
second round; the **15** below remain, and none of them blocks a gap row or a
P1. Each is stated, with its options, in the spec's "Open questions" section;
this list only routes. When one is decided, move it to a "Decisions recorded"
list like the ones above and correct the spec text.

- Account — admin list paging stays offset (SPEC-01 Q2); register's 409
  reveals emails (Q3); single-device logout bumps `token_version` (Q4); the
  per-account lockout is triggerable by anyone (Q5).
- Music — resume through media progress vs `music.listen_progress` (SPEC-15
  b); genre as free text vs D-22 (c); bulk publish floods the bell (d); who can
  play a published track (e).
- Publishing across the tenant fence — SPEC-16 §10 (movie), SPEC-15 e (music).
- Story — keep the publish-time-only chapter check, or adopt SPEC-14 P0.2 (a)
  (SPEC-17 §10).
- Social — re-request after decline (SPEC-18 Q1); removal as an event (Q2);
  opting out of being askable (Q3).
- Layout — per-tenant shells (SPEC-02 Q2); concurrent editors (Q3).

## Review history

- **2026-07-11** — first spec-gap review and fix pass:
  `spec-gap-fix-worklog-2026-07-11.md` (deleted once the 2026-09-30 review
  superseded it — `git show ea100d8:docs/product/analysis/spec-gap-fix-worklog-2026-07-11.md`).
- **2026-09-30** — second review (184 confirmed findings), fixed in the specs
  by `1db1e32` (PR #17); the owner decisions on envelopes, timezone and audio
  applied and the matrix regraded by `f54389b`:
  [spec-gap-fix-worklog-2026-09-30.md](../analysis/spec-gap-fix-worklog-2026-09-30.md)
  (its last section before "Refuted" records the corrections found on
  2026-10-01).
- **2026-10-01** — every `[c]` finding re-verified against `99b5a0b` and
  folded into each spec's "Implementation gaps vs shipped code" section.
  **From now on that section, not a worklog, is the live record** of where the
  code diverges; the worklogs are the dated record. The same day the owner
  decided the seven open questions; they are applied and listed under
  "Decisions recorded 2026-10-01" (gap total 138 → 145). Later that day the
  six modules no spec owned got as-built specs (SPEC-01, 02 and 15–18, `8308fdf`), each
  with its own gap section, and SPEC-14 gained two rows found while writing
  them (gap total 145 → 256). Still the same day, a second round of owner
  decisions (D1–D4, "Decisions recorded 2026-10-01 (second round)") closed
  seven of the 22 questions those specs raised and added seven gap rows (gap
  total 256 → 263).
