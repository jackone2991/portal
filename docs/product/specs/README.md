# Portal — Detailed Feature Specs (PRD level)

**Status:** current · **Last verified:** 2026-10-01

**Language policy:** English only, per owner decision 2026-07-07
([ADR-09](SPEC-06-docs-canonicalisation.md#adr-09); the former vi mirror was
deleted in `f11cf3f` — see [ADR-11](SPEC-06-docs-canonicalisation.md#adr-11)).
The architecture decisions every spec cites by `ADR-NN` are indexed under
[Decision records](#decision-records) below.

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
| [SPEC-01](SPEC-01-account-identity-admin.md) | Account — local auth, approval gate, RBAC, admin console, per-user timezone (as-built, retroactive) | `account` | [ADR-02](SPEC-01-account-identity-admin.md#adr-02), [ADR-06](SPEC-01-account-identity-admin.md#adr-06); SPEC-05 (`notify:dispatch`; reset is SPEC-05 P0.3) | Built (`0002`–`0004`, `0006`, `0010`, `0031`); P0.13 timezone, P0.16 audit retention and the 2026-10-02 targets (§11 rows 25–31) unbuilt; also holds the `tenant` module's P0.17 (tenant links, [ADR-12](SPEC-01-account-identity-admin.md#adr-12); rows 32–33, unbuilt) |
| [SPEC-02](SPEC-02-shell-layout.md) | Shell layout — data-driven navigation menu + registry-backed dashboard widget placement (as-built, retroactive) | `layout` + frontend shell | account (`accountapi.HasPermission`, RBAC); SPEC-09 P0.4 consumes the rails | Built (`0036_layout_core`) |
| [SPEC-03](SPEC-03-platform-ops.md) | Platform ops — backup/restore, queue console, takeout | `ops` | — (land P0 before SPEC-12 data accrues) | P0 built (`0012_ops_backup_runs`); P1.7 takeout unbuilt |
| [SPEC-04](SPEC-04-media-image-pipeline.md) | Media image pipeline + asset management | `media` | — | Built (`0008_media_image_pipeline`) |
| [SPEC-05](SPEC-05-notification-module.md) | Notification module (life-stream backbone) | `notify` | SPEC-04 P0.6 (fan-out); P1.2 not a gate | Built (`0009_notify_notifications`, `0010`) |
| [SPEC-06](SPEC-06-docs-canonicalisation.md) | Docs canonicalisation — 79 stale ADR statements, 95 links, four CI docs checks | none (docs + CI) | [ADR-11](SPEC-06-docs-canonicalisation.md#adr-11) | Executed 2026-09-11 |
| [SPEC-07](SPEC-07-journal.md) | Journal — life-stream write path | `journal` | SPEC-04 P0.6 (fan-out) only; photos via SPEC-08 | Built (`0011_journal_entries`) |
| [SPEC-08](SPEC-08-journal-attachments.md) | Journal attachments — Attachments (≤ 10 image Assets) and Location as first-class Entry properties; the markdown-link workaround backfilled out of every body (issue #8) | `journal` (extends) · frontend | SPEC-04, SPEC-07, SPEC-09 | Executed 2026-09-19 (`0044`, `0045`) |
| [SPEC-09](SPEC-09-life-stream-home.md) | Life-stream home — projection + dashboard | `journal` + home | SPEC-07; producers attach as they land | Built (`0017_journal_stream_items`) |
| [SPEC-10](SPEC-10-continue-rail.md) | Playback resume + continue rail (D-20) | `media` | SPEC-04 P0.6 (P1.5 event); comic leg after SPEC-14 | Built (`0013_media_playback_progress`) |
| [SPEC-11](SPEC-11-people-registry.md) | People registry — contacts + birthdays | `people` | SPEC-04 P0.3 (scheduler) + P0.6 (fan-out); avatars need SPEC-04 | Built (`0016_people_persons`, `0035`) |
| [SPEC-12](SPEC-12-finance-ledger.md) | Finance ledger (Money-Lover-class) | `bank` | [ADR-08](#adr-08); SPEC-03 P0 live before the first real ledger entry; SPEC-04 (P1 receipts only) | Built (`0014_bank_core`, `0042`) |
| [SPEC-13](SPEC-13-ledger-expansion.md) | Ledger expansion — debts, goals, recurring, cards, net worth, automation, splits, sharing | `bank` (extends) | SPEC-12; SPEC-05 for reminders; SPEC-04 (receipts only, via SPEC-12 P1.10) | Phase 1 (debts, `0043_bank_debts`) built; later phases open |
| [SPEC-14](SPEC-14-comic-vertical.md) | Comic vertical, end-to-end | `comic` | SPEC-04 | Built (`0015_comic_core`; reader + import + sync `0024`–`0030`) |
| [SPEC-15](SPEC-15-music-vertical.md) | Music vertical — tracks, zip/multi-file import, enrich + MusicBrainz lookup, playlists, player (as-built, retroactive) | `music` | SPEC-04 (ingest, `/original`, covers, `media:asset_deleted`); SPEC-05 (bell consumer) | Built (`0022_music_core`; `0038`, `0039`, `0041`) |
| [SPEC-16](SPEC-16-movie-vertical.md) | Movie vertical — catalogue over media video assets (as-built, retroactive) | `movie` | SPEC-04, SPEC-14 (pattern), SPEC-10 (asset-level resume) | Backend built (`0021_movie_core`); no frontend |
| [SPEC-17](SPEC-17-story-vertical.md) | Story vertical — stories + Markdown chapters (as-built, retroactive) | `story` | SPEC-04 (covers), SPEC-14 (pattern) | Backend built (`0023_story_core`); reader is a placeholder |
| [SPEC-18](SPEC-18-social-connections.md) | Social connections — request / accept / decline / disconnect; the social layer's first slice (as-built, retroactive) | `social` | account (`accountapi`); SPEC-05 (bell consumers); SPEC-11 (`/people/suggestions`) | Built (`0037_social_connections`) |

The positioning decision (life-OS pivot) and the parking lot are **not** specs;
they live in [ADR-08](#adr-08) below (with [vision.md](../vision.md), the
product yardstick) and [backlog.md § Deferred](../backlog.md).

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
  `tenant_id` ([ADR-07](SPEC-01-account-identity-admin.md#adr-07)).
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
  `refresh_tokens`, `password_reset_tokens`, and the planned
  `email_change_requests` and `deleted_users` — read before any tenant is
  resolved, or after the User's tenant is gone, SPEC-01 §6) and the shell tables `layout_menu_items` /
  `layout_widgets` (one instance-wide shell of configuration rows, readable by
  every signed-in user and filtered per permission in the service; fenced per
  tenant, each personal tenant would get an empty menu — SPEC-02 §6). So are
  system tables (`ops_backup_runs`). `social_connections` and the planned
  `social_declines` have no `tenant_id` but are not exempt: they are fenced by
  **per-user** RLS on `app.current_user` (`0037`, SPEC-18 §6; the cooldown
  table is Decision 2026-10-02b (B8)), because a connection — and a decline —
  spans two personal tenants. **`tenant_links`** (planned, SPEC-01 P0.17) is
  cross-tenant the same way and fenced by its own policies (members of either
  side read; the owning tenant's owner writes). **One planned exception to the
  fence, and only one** ([ADR-12](SPEC-01-account-identity-admin.md#adr-12),
  Decision 2026-10-02b (B14)): a `FOR SELECT` policy may admit a row of
  another tenant only through the tenant module's `SECURITY DEFINER` function
  `app_can_read_shared(owner_id, tenant_id)`, and only for published content
  (`movies`, `music_tracks`, `stories`, `story_chapters` with `status =
  'published'`) and the `shared` assets that render it; no write policy ever
  admits another tenant, and no other table gets such a policy without a new
  decision record. Each
  spec §6 states per table whether it is
  tenant-scoped; DDL in specs drafted before ADR-07 omits the columns, and this
  bullet governs.
- **Events**: every task/event name lands in
  [../../reference/events.md](../../reference/events.md) as part of definition of
  done; every new domain module emits ≥ 1 bus event from day one ([ADR-08](#adr-08)).
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
  Each §7 lists both for every list endpoint. **One named exception**
  *(owner decision 2026-10-02 (A5))*: the admin-only `GET /admin/users`
  (SPEC-01 P0.8) keeps `limit` + `offset` paging with `total` and per-status
  `counts` — an operator approval queue read as "how many are waiting", at
  household scale. It wears the `{items}` envelope like every other list, and
  it is not a precedent: a new list endpoint is keyset-paged.
- **Timezone** *(owner decision 2026-09-30, revised 2026-10-02 (A8); D-17)*:
  every user-facing day or month boundary — "today", "this month", a default
  month, a date-only value turned into an instant, day grouping, on-this-day,
  birthday and due-date countdowns — is computed in **the User's own
  timezone**: `users.timezone`, an IANA name stored per User. The column is
  **NULLable — NULL means "not set"** — and has no default. While it is NULL,
  or when a stored name does not parse (which also logs a warning), backend
  readers use `Asia/Ho_Chi_Minh`; the frontend uses the device zone. There is
  no instance-wide fallback: `APP_TIMEZONE`, "the instance default" and UTC are
  not v1 sources for user-facing boundaries. **Write path**:
  `PATCH /api/v1/auth/me {timezone}` (authenticated; the caller's own row
  only) — `timezone` is required; the response is the updated `/auth/me`
  body. After sign-in the frontend reads the device zone
  (`Intl.DateTimeFormat().resolvedOptions().timeZone`): when the stored zone is
  NULL it saves the device zone automatically; when a zone is set it is applied
  everywhere and the device **never overwrites it** — if the device zone
  differs, the UI offers one prompt to switch and saves only on confirmation.
  Settings offer an IANA picker. *(There is no manual flag: Decision
  2026-10-02 (A8) dropped `timezone_manual`, superseding Decision 2026-10-01
  (f).)* The API validates the name with `time.LoadLocation`; an empty or
  unknown IANA name is 422 `account/invalid-timezone` and nothing is written.
  The slug is registered in `frontend/src/lib/problems.ts` (Errors
  convention). `GET /auth/me` returns `timezone` as the stored name or `null`.
  **Readers**: the frontend reads the zone from `GET /auth/me`; backend modules
  read it through `accountapi` (`UserSummary.Timezone`, which already maps NULL
  to `Asia/Ho_Chi_Minh`), never by querying `users`. **Sweeps**: a periodic
  task never uses one "today" for everyone — it runs often enough (hourly,
  D-17's per-TZ pattern) and evaluates each User's local date in that User's
  zone, relying on its dedup keys for exactly-once. SQL converts at the query
  layer (`occurred_at AT TIME ZONE $tz`), never through the session zone
  (UTC). *(Code follow-up — SPEC-01 P0.13, tracked as SPEC-01 §11 row 14 and
  as the first item of the **Per-user timezone** cross-cutting gap below:
  `0002_account_users` ships `timezone TEXT NOT NULL DEFAULT 'UTC'` — a new
  migration drops the `NOT NULL` and the default and sets the never-chosen
  `'UTC'` rows to NULL; there is no `PATCH /auth/me` and no
  `account/invalid-timezone` in `problems.ts`, `CurrentUser` and
  `accountapi.UserSummary` carry no timezone, and the frontend's `lib/time.ts`
  takes its display zone from `GET /api/v1/time`, i.e. `APP_TIMEZONE` — it
  switches to the User's zone and `/time` keeps only the server clock.)*
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
  same PR ([ADR-10](#adr-10), accepted 2026-07-11; the CI `openapi` job
  diffs them; `backend/MODULES.md` §8). The gate proves the committed codegen
  matches the spec, not that handlers do: handler conformance stays a review
  item until handlers implement `ServerInterface`. This bullet is the one
  statement of the rule; ADR-10's record points here.
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

[ADR-10](#adr-10)'s spec-first CI gate is in force; the `ServerInterface`
retrofit is per-module-as-touched (ADR-10 action items) and gates nothing.

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
**Integ** integrity or a wrong answer · **AuthZ** · **Func** a specced P0
behaviour that is missing · **Contract** API, OpenAPI, Problem types · **UX**
frontend behaviour · **Hyg** schema hygiene · **P1** unbuilt P1 feature.
Rows added by the 2026-10-01 owner decisions, rows found
later the same day while writing SPEC-01…18 (SPEC-14 rows 18–19, SPEC-15 row
25), and rows added by the second round of decisions (SPEC-04 row 19, SPEC-05
row 23, SPEC-01 row 24, SPEC-15 row 26, SPEC-16 row 18, SPEC-17 row 20,
SPEC-02 row 14) are appended at the end of their table instead of renumbering
the rows other documents cite, so a few ranges below are split. So are the
seven SPEC-01 rows the 2026-10-02 decisions added (rows 25–31), and the six
rows Decision 2026-10-02b added (SPEC-15 rows 27–29, SPEC-17 row 21, SPEC-18
rows 13–14); that round also extended SPEC-01 row 29 and narrowed SPEC-02
row 13 in place. Its B13 (tenant asset visibility) appended five more (SPEC-04
rows 20–21, SPEC-10 row 15, SPEC-16 row 19, SPEC-17 row 22) and rewrote
SPEC-15 rows 27 and 29 in place. B14 ([ADR-12](SPEC-01-account-identity-admin.md#adr-12),
shared reads and tenant links) rewrote all seven of those in place — none was
built — and appended three (SPEC-01 rows 32–33, SPEC-18 row 15).

| Spec | Gap section | Rows | By severity (row numbers) | Most severe |
|------|-------------|-----:|---------------------------|-------------|
| [SPEC-01](SPEC-01-account-identity-admin.md) | [§11](SPEC-01-account-identity-admin.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 33 | Sec 1–5 · AuthZ 6–7 · Data 8 · Integ 9–13 (10 superseded by 26) · Func (timezone) 14 · Contract 15–20 · UX 21–22 · Hyg 23 · P1 24 · Sec 25–28 · Data 29 (extended, 2026-10-02b) · Sec 30 · Data 31 (appended, 2026-10-02) · Sec 32 · AuthZ 33 (appended, 2026-10-02b B14 — the `tenant` module) | re-parenting a role under `superadmin` escalates every holder to `*` (1); refresh rotation is check-then-act, so two concurrent presentations fork the chain (2) |
| [SPEC-02](SPEC-02-shell-layout.md) | [§11](SPEC-02-shell-layout.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 14 | Sec 1 · Integ 2–4 · UX 5 · Contract 6–8 · Test 9 · UX 10 · Test 11 · Hyg 12 · P1 13 (P1.2–P1.3; P1.1 dropped, 2026-10-02b) · P1 14 | `href` guard bypassed by `/\` and control characters — an open redirect in the menu shown to every user (1) |
| [SPEC-03](SPEC-03-platform-ops.md) | [§11](SPEC-03-platform-ops.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 11 | Sec 1–4 · Data 5–6 · Func 7–8 · Contract 9 · Hyg 10 · P1 11 | queue console writable by any `queues:read` holder, no CSRF guard (1); the restore drill can only reach the dev MinIO (2) |
| [SPEC-04](SPEC-04-media-image-pipeline.md) | [§11](SPEC-04-media-image-pipeline.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 21 | Data 1–3, 19 · Sec/Integ (`/original`) 4–6 · Integ 7–8 · AuthZ 9 · Contract/UX 10–13, 15–18 · P1 14 · AuthZ 20–21 (appended, 2026-10-02b B13; rewritten by B14) | DELETE's 500 rolls back the `deleting` tombstone after objects are purged (1); `/original` streams abandoned or purged uploads, sized from the client's claim (4–6) |
| [SPEC-05](SPEC-05-notification-module.md) | [§11](SPEC-05-notification-module.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 23 | Sec 1–2 · Data 3–4 · Func/UX 5–12 · Contract 13–16, 22 · Hyg 17–20 · P1 21, 23 | plaintext reset token in the Asynq payload (1); reset token consumed check-then-act, the user's other tokens not revoked (2) |
| [SPEC-07](SPEC-07-journal.md) | [§11](SPEC-07-journal.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 8 | Data 1 · UX 2–5 · Contract 6–8 | `journal:entry_created` published before the request transaction commits (1) |
| [SPEC-08](SPEC-08-journal-attachments.md) | [section](SPEC-08-journal-attachments.md#implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 2 | AuthZ 1 · UX 2 (both owned by other specs) | no `user` grant of `assets:write:own` and no enforcement on upload (1) |
| [SPEC-09](SPEC-09-life-stream-home.md) | [§11](SPEC-09-life-stream-home.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 16 | Data / stale 1–5 · Integ (order, render) 6–10 · UX (home, rail, `/weather` gate) 11–14 · Contract 15 · P1 16 | stream unique key lacks `user_id`, so a second user's playback is swallowed (1); deleted or edited birthdays leave stale cards (2) |
| [SPEC-10](SPEC-10-continue-rail.md) | [§11](SPEC-10-continue-rail.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 15 | Data 1–3 · Integ 4–7 · Contract/UX 8–13 · P1 14 · AuthZ 15 (appended, 2026-10-02b B13; rewritten by B14) | the `pagehide` save is a `sendBeacon` POST → 405, silently lost (1); completion latched even when Publish fails, so the event is lost (2) |
| [SPEC-11](SPEC-11-people-registry.md) | [§11](SPEC-11-people-registry.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 15 | Data (lost / duplicate events) 1–4 · Integ (dates) 5–7 · Contract/UX 8–12, 14–15 · P1 13 | every PATCH carrying a birthday resets notices → duplicate stream items (1); no revoke / delete events (2) |
| [SPEC-12](SPEC-12-finance-ledger.md) | [§12](SPEC-12-finance-ledger.md#12-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 17 | Sec 1 · Data 2–5 · Contract 6–12, 17 · UX 13–16 | derived-balance queries (accounts, dashboard) not filtered by the caller (1); archived accounts still accept writes (2) |
| [SPEC-13](SPEC-13-ledger-expansion.md) | [§8](SPEC-13-ledger-expansion.md#8-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 5 | Data 1 · Scheduling 2 · Contract 3–5 | opening a debt is three writes with no enclosing transaction (1) |
| [SPEC-14](SPEC-14-comic-vertical.md) | [§11](SPEC-14-comic-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 19 | Sec 1–5 · Integ 6–9 · Contract/UX 10–17 · Data 18 · Integ 19 | `/api/v1/internal/*` public at the edge, secret compared with `!=` (1–3); publish never checks `comics:publish:own`, drafts leak as 403 (4–5) |
| [SPEC-15](SPEC-15-music-vertical.md) | [§12](SPEC-15-music-vertical.md#12-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 29 | Data 1–2, 4 · Integ 3, 5–13 · Contract 14–17, 22–23 · UX 18–19, 24 · Hyg 20–21 · AuthZ 25 (appended) · P1 26 · Func 27 · Integ 28 · AuthZ 29 (appended, 2026-10-02b; 27 and 29 rewritten by B13, then B14) | the `media:asset_deleted` consumer runs with no tenant scope, so a deleted audio asset leaves a published track pointing at nothing (1); a second zip upload re-imports every track (2) |
| [SPEC-16](SPEC-16-movie-vertical.md) | [§11](SPEC-16-movie-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 19 | Data 1–3 · Integ 4–5 · AuthZ 6–7 · Contract 8–13 · Hyg 14–15 · P1 16–18 · AuthZ 19 (appended, 2026-10-02b B13; rewritten by B14) | the `media:asset_deleted` consumer runs with no tenant scope (1); clearing the video leaves the movie published (2) |
| [SPEC-17](SPEC-17-story-vertical.md) | [§11](SPEC-17-story-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 22 | Data 1 · Integ 2–6 · AuthZ 7–8 · Contract 9–15, 17 · Hyg 16 · P1 18–20 · Integ 21 (appended, 2026-10-02b) · AuthZ 22 (appended, B13; rewritten by B14) | the `media:asset_deleted` consumer runs with no tenant scope (1); a chapter created without `sort_order`, or with a duplicate, is a 500 at COMMIT (2) |
| [SPEC-18](SPEC-18-social-connections.md) | [§11](SPEC-18-social-connections.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 15 | Data 1 · Integ 2–3 · Contract 4–6 · UX 7 · Test/docs 8–9 · Contract 10–11 · Hyg 12 · Func 13 · P1 14 (appended, 2026-10-02b) · AuthZ 15 (appended, B14) | events published before COMMIT leave a phantom bell entry (1); a concurrent duplicate request aborts the transaction → 500 (2) |
| **Total** | | **284** | | |

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
  2026-10-02 (A8)): the account change comes first; it is SPEC-01 P0.13 and its
  gap row is SPEC-01 §11 row 14, summarised here:
  1. **migration** (account-owned): `users.timezone` loses its `NOT NULL` and
     its default, and the never-chosen `'UTC'` rows become NULL ("not set");
  2. **backend** (`account`): `PATCH /api/v1/auth/me {timezone}` validating
     with `time.LoadLocation` → 422 `account/invalid-timezone`; `GET /auth/me`
     / `CurrentUser` carry `timezone` (nullable); `accountapi.UserSummary.Timezone`
     (plus a batch lookup for sweeps) maps NULL to `Asia/Ho_Chi_Minh`;
  3. **openapi**: both operations and the 422;
  4. **frontend**: `account/invalid-timezone` in `problems.ts`; the
     post-sign-in save of the device zone while the stored one is NULL, the
     one-time switch prompt when a set zone differs from the device, the
     settings picker; `lib/time.ts` takes the zone from `/auth/me`, and
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
- **Deleting a User purges every module, media last** (Decisions 2026-10-01b
  (D1) and 2026-10-02 (A6)): SPEC-04 §11 row 19 (`mediaapi.PurgeOwnerAssets`,
  media's `PurgeOwnerData`) before SPEC-01 §11 row 8 (the delete order that
  calls it); then SPEC-01 §11 row 29 — one `PurgeOwnerData` in each of
  `comic`, `music`, `movie`, `story`, `journal`, `bank`, `people`, `social`,
  `notify` and `tenant`, the registry and the `deleted_users` snapshot,
  sealed through SPEC-01 row 31's `platform/audit` encryption function
  (Decision 2026-10-02b (B1)), so row 29 lands with or after row 31. The
  per-module methods are listed in SPEC-01 row 29 rather than as rows in each
  module's spec; land them module by module, each with a test that its purge
  leaves no row, object or key behind.
- **Shared reads and tenant links** (Decision 2026-10-02b (B13) as revised
  by B14; [ADR-12](SPEC-01-account-identity-admin.md#adr-12)): SPEC-01 §11
  row 32 first (`000N_tenant_links`, `portal_acl`, `app_tenants_linked` and
  `app_can_read_shared`, the links API and screen, `tenantapi` reach), with
  SPEC-18 §11 row 15's `000N_social_acl_grant` in the same PR; then SPEC-04
  §11 row 20 (`000N_media_shared_visibility` and the read rule) together
  with SPEC-01 §11 row 33 (request scopes without the tenant-admin flag), and
  SPEC-04 row 21 (`mediaapi.SetVisibility`); then SPEC-10 §11 row 15 (a
  reader keeps their own progress row; with or after SPEC-10 row 6); then the
  content modules, each adding its `shared_read` policy and raising and
  lowering its own assets — SPEC-15 §12 row 29 (music; with row 27 for
  readers' resume), SPEC-16 §11 row 19 (movie), SPEC-17 §11 row 22 (story);
  and SPEC-18 row 15's discovery narrowing no earlier than the links screen,
  since on an instance of personal organisations it empties the directory
  until owners link their tenants.
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
  row 19 · SPEC-17 row 2; blank chapters of a published work hidden from
  non-owners (SPEC-14 P0.2 (a)) SPEC-14 row 9 · SPEC-17 row 21; D-34 matcher
  (F032) SPEC-09 row 14 (`/weather`) ·
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
  owner's local date. (Revised by Decision 2026-10-02 (A8): the stored zone
  may be NULL, `Asia/Ho_Chi_Minh` is the readers' fallback rather than a
  column default, and the manual override is gone.) Detail: the **Timezone** convention above; the D-17
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
- **(f) `timezone_manual` and `account/invalid-timezone` are confirmed** —
  *superseded by Decision 2026-10-02 (A8)* as to `timezone_manual`, which is
  dropped everywhere (no column, no field, no request flag); the
  `account/invalid-timezone` slug stands. Kept here as the record of what was
  decided on 2026-10-01. The current rule is the **Timezone** convention above;
  the code follow-up is SPEC-01 P0.13 (§11 row 14), the first item of the
  **Per-user timezone** cross-cutting gap.
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
  superadmins** — no exemption from [ADR-08](#adr-08)'s rule (the **Events** convention
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

## Decisions recorded 2026-10-02

The owner settled twelve points about the account module in a review of
SPEC-01, among them its four open questions (Q2–Q5). No decision record was
written: each is stated in SPEC-01's requirement text, which is the detail;
this list only routes. The vocabulary — User, Session, Approval, Rejected,
Disabled, Superadmin, Approver — is [CONTEXT.md](../../../CONTEXT.md)'s. Gap
rows cite them as "Decision 2026-10-02 (A*n*)".

- **(A1) Registration answers uniformly** — SPEC-01 P0.1: always 201
  `{status: "registered"}`, never 409 `account/email-taken` (which survives
  only on admin create/edit); a new email becomes a Pending User and gets a
  "registered, awaiting approval" email; an existing email writes nothing,
  notifies no Approver, and gets one rate-limited email by state; the password
  is hashed on both paths; registration succeeds without SMTP. Closes SPEC-01
  Q3. Gap row: SPEC-01 §11 row 25 (and row 5 becomes load-bearing);
  TC-ACC-130…134.
- **(A2) No first-registrant bootstrap; a pre-created Superadmin** — SPEC-01
  P0.12 (replaces P0.1's founder rule): `cmd/api` creates the User named by
  `BOOTSTRAP_SUPERADMIN_EMAIL` with the new `BOOTSTRAP_SUPERADMIN_PASSWORD`
  (Approved, `superadmin`, enabled, `users.password_must_change`), re-asserts
  without touching the password when it exists, warns when unset; until the
  password is changed every authenticated route but four answers 403
  `account/password-change-required`; new `POST /auth/password`. Gap row:
  SPEC-01 §11 row 26 (row 10 superseded by it); TC-ACC-135…140.
- **(A3) Logout ends one Session** — SPEC-01 P0.6: revokes only the presented
  refresh-token chain, no `token_version` bump, 422 without a refresh token;
  logout-all ends every Session. Closes SPEC-01 Q4. Gap row: §11 row 27;
  TC-ACC-141…143.
- **(A4) Login lockout per (email, client IP)** — SPEC-01 P0.2: the global
  per-IP cap stays; the per-email counter becomes per-(email, IP). Closes
  SPEC-01 Q5; depends on §11 row 4. Gap row: §11 row 28; TC-ACC-144, 145.
- **(A5) Admin list paging stays offset** — the **Pagination** convention
  above (its one named exception) and SPEC-01 P0.8. Closes SPEC-01 Q2. No gap
  row (shipped behaviour).
- **(A6) Deleting a User purges every module, then the row** — SPEC-01 P0.10:
  an idempotent `PurgeOwnerData` per module, a registry run content → media →
  tenant, no grace period, a `deleted_users` snapshot kept 90 days, the email
  freed (keep someone Rejected to refuse them), a restore can resurrect a
  deleted User ([backup-restore.md](../../operations/backup-restore.md)).
  Extends Decision 2026-10-01b (D1). Gap row: §11 row 29; TC-ACC-146…149.
- **(A7) Audit identity data lives 90 days** — SPEC-01 P0.16 (owned jointly
  with `platform/audit`): every `audit_log` row's identifying data encrypted
  with `AUDIT_PII_KEY`, readable only by Superadmins, anonymised after 90
  days by `account:expire_identity_data` ([events.md](../../reference/events.md)).
  Gap row: §11 row 31; TC-ACC-150…153.
- **(A8) Timezone: NULL means "not set", no manual flag** — the **Timezone**
  convention above, SPEC-01 P0.13; supersedes Decision 2026-10-01 (f) as to
  `timezone_manual`. Gap row: §11 row 14, rewritten in place (unbuilt);
  TC-ACC-100…104.
- **(A9) Email change requires verification** — SPEC-01 P0.10 (admin edit,
  P0) and P1.4 (self-service): pending change in `email_change_requests`,
  1-hour confirm link to the new address, cancel link to the old, uniqueness
  re-checked at confirm, 503 `account/mail-unavailable` without SMTP. Gap row:
  §11 row 30; TC-ACC-155…160.
- **(A10) Approval semantics** — SPEC-01 P0.9/P0.10 wording: Disable is a
  temporary suspension of an Approved User; revoke-approval sends a User back
  to Pending; a Rejected User cannot register again. Shipped; no gap row.
- **(A11) Superadmin by permission** — SPEC-01 P1.3 `SuperadminIDs` = Users
  whose effective permissions contain `*`; SPEC-05 P1.5 follows. Gap row: §11
  row 24, rewritten in place (unbuilt); TC-ACC-127.
- **(A12) "A second person on the instance"** — SPEC-01 §4: a second Approved
  User has their own, unshared data; shared household data is not designed.

## Decisions recorded 2026-10-02 (second round)

The owner settled the last twelve open questions the as-built specs raised:
the eleven left from SPEC-02 and SPEC-15–18 and SPEC-01 Q6, which the first
2026-10-02 round itself opened. The detail lives in the places named here; this
list only routes. Gap rows cite them as "Decision 2026-10-02b (B*n*)".

- **(B1) The `deleted_users` snapshot is encrypted** — SPEC-01 P0.10, P0.16
  and §6: the user id, the acting admin and the time stay in clear; email,
  display name and role codes are sealed into `pii` with `AUDIT_PII_KEY` by
  the same `platform/audit` function as audit identity data, readable only by
  Superadmins, dropped when the key is unset. Closes SPEC-01 Q6. Gap row:
  §11 row 29, extended in place (it now lands with or after row 31's
  encryption half); TC-ACC-161, 162, and TC-ACC-148 reworded.
- **(B2) Music resumes through media progress** — SPEC-15 P0.10: the player
  saves and reads the progress row of the track's audio asset (SPEC-10's API)
  for the caller's own tracks; tracks join `/continue` as `media` items; no
  `music.listen_progress` (D-20 superseded for music in
  [feature-inventory.md](../feature-inventory.md)). Gap row: SPEC-15 §12
  row 27, which depends on SPEC-10 §11 rows 4–5; TC-MUS-125, 126.
- **(B3) Music genre stays free text** — SPEC-15 P0.8: one MusicBrainz
  folksonomy tag in `genre text`, a recorded exception to D-22 (the D-22
  update in [feature-inventory.md](../feature-inventory.md)). Shipped; no gap
  row.
- **(B4) A self-publish makes no bell entry** — SPEC-15 P0.3 and SPEC-05 §7:
  `music:track_published` gains `actor_user_id`; `notify:on_track_published`
  writes nothing when it equals `owner_user_id`, so a bulk publish is silent;
  no summary event ([events.md](../../reference/events.md)). Gap row:
  SPEC-15 §12 row 28; TC-MUS-127, 128.
- **(B5) Tenant members play a published track, via SignedURL; `/original`
  stays owner-only** — *mechanism revised by B13 below*: members of the
  owner's tenant play a published track (that part stands), but through the
  asset's `tenant` visibility and `/original`, not a signed URL; there is no
  `GET /tracks/{id}/play-url`. Gap row: SPEC-15 §12 row 29 (rewritten);
  TC-MUS-129, 130 (reworded). *B14 below* renames the value `shared` and
  widens the audience to friends in linked tenants.
- **(B6) "Published" never crosses the tenant fence** — SPEC-16 P0.3 and §10
  (a status flag inside the owner's tenant), SPEC-15 §3 and P0.10 for music,
  SPEC-17 §3 for story. Shipped; no gap row; TC-MOV-027. *Revised by B13
  below*: still never across the fence, but inside it a published movie's
  video and poster are widened to `tenant`, so members can watch it
  (TC-MOV-027 reworded). *Revised again by B14 below*: the fence opens for
  reads by the owner's friends in actively linked tenants.
- **(B7) Blank chapters of a published story are hidden** — SPEC-17 P0.4
  adopts SPEC-14 P0.2 (a): non-owners get no blank chapter in the detail, the
  reader payload or `chapter_count`; nothing auto-unpublishes. Gap row:
  SPEC-17 §11 row 21 (SPEC-14 §11 row 9's pattern); TC-STY-047, 048.
- **(B8) A decline starts a 24-hour re-request cooldown** — SPEC-18 P0.9 (new;
  was P1.2), §6 (`social_declines`, per direction, per-user RLS) and §7 (429
  `social/request-cooldown` with `Retry-After`). Gap row: SPEC-18 §11 row 13;
  TC-SOC-071…074.
- **(B9) `social:connection_removed` is emit-only** — SPEC-18 P1.1 and §8: no
  consumer in v1 ([events.md](../../reference/events.md) planned row). Gap
  row: SPEC-18 §11 row 14; TC-SOC-075.
- **(B10) No opt-out from being asked** — SPEC-18 P0.2 and §3: every approved,
  enabled account is askable. Shipped; no gap row. *Amended by B14 below*:
  every approved, enabled account the caller can reach (same tenant or an
  actively linked one) — SPEC-18 §11 row 15.
- **(B11) One instance-wide layout** — SPEC-02 §3, §6 and P2: every user gets
  the same shell, configured only by `system:settings:write` holders; no
  per-tenant or per-household shells and no `tenant_id`. Shipped; no gap row.
- **(B12) Layout saves stay last-write-wins** — SPEC-02 P1.1 dropped (no
  `version`, no 409 `layout/stale`). Gap row: SPEC-02 §11 row 13 narrowed to
  P1.2–P1.3.
- **(B13) Tenant members play and watch published music and movies, through
  a third asset visibility** — *audience revised by B14 below, before any of
  it was built: the value is `shared`, the readers are household and
  friends in linked tenants, and the tenant-admin read is gone.* A later
  owner decision of the same day,
  revising B5's mechanism and amending B6. SPEC-04 P0.8:
  `assets.visibility` gains `tenant` (migration
  `000N_media_tenant_visibility`, media-owned), readable by every member of
  the owning tenant, writes unchanged; `GET /assets/{id}` and, for
  `video`/`audio` only, `/original` admit the owner, a tenant admin, or a
  member for a `tenant` asset — never across the tenant fence, and an image's
  original (EXIF) never to a non-owner, who gets its variants; content modules
  set it through the new `mediaapi.SetVisibility` (`tenant`/`private` only,
  never touching `public`); `SignedURL` is left with no caller. SPEC-15 P0.2,
  P0.3, P0.10: publishing a track raises its audio and cover to `tenant`;
  unpublish, delete, clearing or replacing lower what no other published track
  of the owner still uses (cross-module sharing and a `:publish:any` publisher
  who is not a tenant admin are recorded known edges); members play from
  `/original`. SPEC-16 P0.3, P0.4, P0.6: the same for a movie's video and
  poster; members watch at `/library/media/{video_asset_id}`. SPEC-10 P0.2: a
  member keeps their own progress row on an asset they may play. SPEC-17 P0.5
  applies it to the story cover (a published story was already
  tenant-readable). Gap rows: SPEC-04 §11 rows 20–21, SPEC-10 §11 row 15,
  SPEC-15 §12 rows 27 and 29 (rewritten), SPEC-16 §11 row 19, SPEC-17 §11
  row 22; TC-MEDIA-115…119, TC-CONT-103, TC-MUS-126, 129, 130 (reworded),
  TC-MOV-027 (reworded), TC-MOV-112, 113, TC-STY-112.
- **(B14) Only family and friends read published content; tenants link to
  each other** — the owner's revision of B13, recorded as
  [ADR-12](SPEC-01-account-identity-admin.md#adr-12) (the first record since
  the fold; it supersedes B13's audience and amends B6 and B10). Four points:
  image originals stay owner-only; a user's published music, movies and
  stories are read only by the user's **family** (members of the tenant the
  content lives in) and **friends** (accepted SPEC-18 connections) — and a
  friend only from the same tenant or a **linked** one; each tenant keeps an
  allow-list of tenants it links to, a link is active only when both list
  each other (mutual), and only reachable users (same tenant or actively
  linked) can find, ask and accept each other; a tenant admin reads nothing
  extra. SPEC-01 P0.17 (the `tenant` module: `000N_tenant_links`, the role
  `portal_acl`, `app_tenants_linked` and `app_can_read_shared`, `GET`/`PUT
  /tenants/{id}/links`, `GET /admin/tenants`, the permission
  `tenants:links:write` granted to no role, `tenant:link_changed`
  emit-only, request scopes without `app.tenant_admin`, `tenantapi`
  reach); SPEC-04 P0.8 (`shared` replaces `tenant`,
  `000N_media_shared_visibility`); SPEC-15 §6, SPEC-16 §6, SPEC-17 §6
  (`shared_read` policies); SPEC-10 P0.2 (the reader's row lives in the
  reader's tenant); SPEC-18 P0.2, P0.3 and §6 (reach; `000N_social_acl_grant`;
  an unlink keeps connections but stops reads, requests and accepts). Comic
  stays private (out of scope). Gap rows: SPEC-01 §11 rows 32–33 and SPEC-18
  §11 row 15 (new); SPEC-04 rows 20–21, SPEC-10 row 15, SPEC-15 rows 27 and
  29, SPEC-16 row 19, SPEC-17 row 22 (rewritten); TC-TEN-001…010,
  TC-MEDIA-120, 121, TC-CONT-104, TC-MUS-131, TC-MOV-114, TC-STY-113,
  TC-SOC-076…079, and the B13 TCs reworded. The drafting choices ADR-12 made
  for the owner to confirm are listed in its Consequences.

## Open owner decisions

None are open. Of the 22 questions the as-built specs (SPEC-01, 02 and 15–18)
raised, seven were closed by Decision 2026-10-01b, four by Decision 2026-10-02
and the remaining eleven — with SPEC-01 Q6, which the 2026-10-02 round opened —
by Decision 2026-10-02b: twelve in all. When a spec raises a new owner
question, state it with its options in that spec's "Open questions" section
and route it here; when it is decided, move it to a "Decisions recorded" list
like the ones above and correct the spec text.

## Decision records

Architecture decisions are recorded as `ADR-NN`. They were standalone files
under `docs/adr/` until the 2026-10-01 fold; each record now lives in the spec
that owns its subject, behind a fixed `<a id="adr-nn"></a>` anchor, and the
`ADR-NN` IDs that code comments and documents cite are unchanged. This section
indexes all twelve and holds the three that belong to no single spec: the v1
scope cut, the life-OS positioning and the API contract direction.

Each record keeps the binding shape — Context → Decision → Options considered →
Trade-offs → Consequences → Action items ([STYLE.md](../../STYLE.md)). Decision,
Options considered and Trade-offs are the narrative layer, kept verbatim (ADR-11
rule 2, [SPEC-06](SPEC-06-docs-canonicalisation.md#adr-11)): they record what
was known at the time and may name things since retired. Context, Consequences
and Action items are the fact layer, corrected in place and true as of this
file's `Last verified`. Where a convention above already states a fact, the
record points to it instead of repeating it. A reversed decision gets a new
record that supersedes the old one. Numbers are never reused; `00` is retired
(that file was a review, deleted after `ea100d8` —
`git show ea100d8:docs/product/analysis/architecture-review-2026-05-24.md`).

**The framing constraint every record inherits:** **1 dev · 2-week bursts ·
≤ $100/mo · single VPS** ([ADR-01](#adr-01)).

| ADR | Title | Status | One line | Location |
|---|---|---|---|---|
| ADR-01 | v1 scope cut | accepted, amended by ADR-08, executed | What v1 is — and everything it is not | [README.md § ADR-01](#adr-01) |
| ADR-02 | RBAC model reconciliation | accepted, amended by ADR-06 | Role hierarchy is canonical for v1; policy bundles layer on top later | [SPEC-01 § ADR-02](SPEC-01-account-identity-admin.md#adr-02) |
| ADR-03 | Single-VPS topology | accepted | One VPS, compose profiles as the envelope; observability/live profiles stay off | [SPEC-03 § ADR-03](SPEC-03-platform-ops.md#adr-03) |
| ADR-04 | Storage tier & budget | accepted | R2 for prod, MinIO kept for local dev (presigned uploads need an S3 origin) | [SPEC-04 § ADR-04](SPEC-04-media-image-pipeline.md#adr-04) |
| ADR-05 | Phase 0 wiring order | accepted, executed | The critical path to a running demo — closed; kept for the shape of the work | [SPEC-03 § ADR-05](SPEC-03-platform-ops.md#adr-05) |
| ADR-06 | Local auth model | accepted, executed | Passwords in Portal (Argon2id + JWT); Authentik/OIDC removed | [SPEC-01 § ADR-06](SPEC-01-account-identity-admin.md#adr-06) |
| ADR-07 | Multi-tenancy / RLS model | accepted, executed | Tenant column + RLS policies; enforced only when the app connects as `portal_app` | [SPEC-01 § ADR-07](SPEC-01-account-identity-admin.md#adr-07) |
| ADR-08 | Life-OS pivot + finance ledger scope | accepted, executed | Portal is a life OS; ledger in scope; "real bank" stays deferred | [README.md § ADR-08](#adr-08) |
| ADR-09 | Documentation architecture | accepted, amended by ADR-11, executed | Diátaxis-informed `docs/` tree; English canonical | [SPEC-06 § ADR-09](SPEC-06-docs-canonicalisation.md#adr-09) |
| ADR-10 | OpenAPI contract direction | accepted | Spec-first, enforced: generate Go stubs + TS client; CI drift gate | [README.md § ADR-10](#adr-10) |
| ADR-11 | Documentation canonicalisation | accepted, executed | One owner per fact; ADRs corrected in place by layer; nothing archived | [SPEC-06 § ADR-11](SPEC-06-docs-canonicalisation.md#adr-11) |
| ADR-12 | Sharing published content with household and friends; tenant links | accepted (Decision 2026-10-02b (B14)), not built | Published music, movies and stories are read by the owner's household and friends in mutually linked tenants, through one `SECURITY DEFINER` predicate in the SELECT policies — a read-only exception to ADR-07's fence | [SPEC-01 § ADR-12](SPEC-01-account-identity-admin.md#adr-12) |

**When to write one.** A choice that (a) is expensive to reverse, (b) crosses
module boundaries, or (c) contradicts a previous record or the scope cut gets a
record, filed under `## Decision records` in the spec that owns its subject (or
here, if no spec does), with the next unused number. Day-to-day feature
decisions belong in [feature-inventory.md](../feature-inventory.md) as `D-N`
entries; specs cite both kinds by ID.

<a id="adr-01"></a>
### ADR-01 — v1 scope cut: what fits in 2 weeks / 1 dev / $100/mo / 1 VPS

**Decided:** 2026-05-24 · **Status:** accepted; amended by [ADR-08](#adr-08) (scope since widened); executed (the demo loop shipped 2026-07-06)

Deciders: kirito.

#### Context

*As found on 2026-05-24. The cut this record made was executed and closed: the
demo loop shipped on 2026-07-06 and is the regression baseline. What has been
built since is not tracked here — [`/CLAUDE.md`](../../../CLAUDE.md) § Current
status owns that.*

[`feature-inventory.md`](../feature-inventory.md) (then `feature.md`) described 12 phases and 40 settled decisions covering identity, multi-tenancy, media, four content verticals, personal finance, notifications, social, search, marketing site, advanced social (reels/live/audio rooms), creator economy, marketplace, and ML safety. The decisions are individually sound; collectively they describe a platform that would take a small team a year or more to ship.

The stated constraint envelope was:

- 1 developer
- 2 weeks to v1
- ≤ $100/month infrastructure budget
- single VPS

A version of Portal that tried to honour every Phase 0 deliverable in 2 weeks would run out of time around Phase 0 step 8 (out of 14) and ship nothing. A version that picked one coherent slice, shipped it, and treated the rest as a backlog could produce a *running* artefact at the end of the sprint.

This record made the cut explicit so it was a decision, not a drift.

<!-- adr-narrative -->
#### Decision

**v1 ships Phase 0 (foundation wiring) plus a vertical slice of Phase 2 (one video upload happy path) and nothing else.** Everything in Phases 1, 3–12 was deferred by this cut. The phase ordering in `feature.md` was unchanged; the scope of what counted as "v1" was the only thing this ADR moved.

Concretely, v1 = the smallest demo that proves the architecture works end-to-end. As built (two steps differ from the 2026-05-24 text: sign-in was to be Authentik/OIDC and the upload was to go straight to R2):

1. A user signs in with a local password ([ADR-06](SPEC-01-account-identity-admin.md#adr-06): `POST /api/v1/auth/login`; Authentik/OIDC never shipped).
2. They land on the Next.js home page authenticated.
3. They upload an mp4 via the UI.
4. The upload is persisted to MinIO in dev and R2 in deployed environments ([ADR-04](SPEC-04-media-image-pipeline.md#adr-04)).
5. The worker picks up the transcode task, produces an HLS ladder, and updates `assets.status = ready`.
6. The user plays the video back in the browser using Vidstack.
7. They sign out; the session is revocable via the existing two-channel mechanism.

That was the entire v1 demo loop. At the time of the cut: no tenants, no movies/music/stories/comics CRUD, no bank, no social, no notifications, no mediamtx, no LiveKit, no observability stack, no file-gated permissions, no policy bundles, no marketplace. Of those, tenants, the four verticals, bank, notifications and a first social slice have since shipped under ADR-08; the rest remain out.

#### Options considered

##### Option A — Honour Phase 0 in full, defer everything else

| Dimension | Assessment |
| --- | --- |
| Complexity | High — 14 deliverables in Phase 0 alone |
| Cost | $30–60/mo |
| Scalability | N/A (foundation only) |
| Team familiarity | Solo dev knows this stack |

**Pros:** Phase 0 is the "spec-correct" sprint; every piece set up here pays dividends in every later phase.
**Cons:** 14 deliverables in 2 weeks for 1 dev is ~2 hours each including testing — unrealistic when several (CI workflows, frontend conventions doc, migration audit, RFC 7807 retrofit, observability stack) are multi-hour items. The most likely outcome is "Phase 0 partially done, no running demo."

##### Option B — Phase 0 minimum + Phase 2 vertical slice  *(chosen)*

| Dimension | Assessment |
| --- | --- |
| Complexity | Medium — cut Phase 0 from 14 items to ~8 |
| Cost | $30–60/mo |
| Scalability | Single-user demo; multi-tenant deferred |
| Team familiarity | Solo dev knows this stack |

**Pros:** Produces a running demo end-of-sprint. Forces the wiring gap to close on Day 3. Surfaces the integration bugs (cookie flags, CORS, oapi-codegen handler shape, sqlc adapter signatures) that are the actual risk.
**Cons:** Skips the migration audit ([D-18]), the observability stack ([D-8]), the frontend conventions doc ([D-32]/[D-33]), CI workflows ([D-9]), the OpenAPI cross-module schema retrofit ([D-29]). All of these have to land later; some will hurt to retrofit.

##### Option C — Skip Phase 0, hand-write a thin auth layer + media demo

| Dimension | Assessment |
| --- | --- |
| Complexity | Low for v1 |
| Cost | $30/mo |
| Scalability | Throwaway — would need full rewrite |
| Team familiarity | Solo dev knows this stack |

**Pros:** Fastest path to a running demo.
**Cons:** Throws away the existing account module (which is already written), the OpenAPI spec, the module boundary discipline, and the modular monolith layout. Builds technical debt the rest of the year is paying off. Only correct if v1 is a *throwaway prototype*; if it's the seed of the real product, this is wrong.

#### Trade-offs

Option A's failure mode is "no demo at end of sprint, lots of half-finished plumbing." Option C's failure mode is "demo works, can't extend it." Option B's failure mode is "demo works, missing some Phase 0 niceties that need a Phase 0.5 sprint." Of the three, Option B's failure mode is the cheapest to recover from: the missing pieces (CI workflows, frontend conventions doc, observability profile) can each be added in a half-day sprint without touching application code.

The cut to ~8 Phase 0 deliverables is:

| Phase 0 deliverable (from feature.md) | v1? | Reason |
| --- | --- | --- |
| Wire `cmd/api/main.go` | **Yes** | The actual blocker. |
| `make sqlc` for account block + commit decision | **Yes** | Required before adapters compile. |
| Repository adapters for account interfaces | **Yes** | Required to construct the module. |
| Migration `0001` audit (split into 0001/0002/0003/0005) | **Yes** | Cheap to do *now* before data exists; impossible later. [D-18] |
| `users.locale` + `users.timezone` columns | **Yes** | One-line addition during the audit; needed by frontend on day one. |
| Move `audit/` → `platform/audit/` + rename event | **Yes** | Cheap during the migration audit; expensive after audit_log has rows. [D-25] |
| Surface `amr`/`acr`/`auth_time` claims into context | **Yes** | One file change; lets [D-27]/[D-28] land later without rewriting middleware. |
| `user_oidc_roles` table | **Yes** | Lands in `0003_account_rbac`; OIDC group sync writes to it on first login. [D-26] |
| RFC 7807 `Problem` adoption in OpenAPI | **Partial** | Add the schema; retrofit handlers as they're written, not in a sweep. |
| Reserve `notify:*` Asynq prefix | **Yes** | Documentation-only; one line in MODULES.md §5.2. |
| OpenAPI cross-module schemas (Money, PaginatedResult, TenantContext, ContinuingItem) | **No** | Money/Continue/TenantContext aren't needed until bank/Phase 4/tenant ships. Add when first needed. |
| URL versioning + RFC 9745 doc | **No** | `/api/v1/` is already in place; the doc is paperwork that can land in week 3. |
| Frontend server-only API client + refresh-and-return route | **Yes** | Without this, RSC pages can't authenticate against the API. [D-34] |
| Frontend conventions doc (Zustand/TanStack/RHF boundary) | **No** | Solo dev; a doc for an audience of one is paperwork. Add when a second contributor appears. [D-32]/[D-33] |
| CI workflows (lint + test + drift + roundtrip + build + security) | **Partial** | Ship the drift check (`sqlc-drift`, `openapi-drift`) only. Skip multi-arch builds, security scan, integration matrix until week 3. [D-9] |

For Phase 2 the v1 cut is: one queue priority, libx264 only, no hardware encoder paths, no per-user/per-tenant quotas, no backpressure, no dead-letter queue UI (failed transcodes get logged loudly and the operator fishes them out by hand). The single happy-path flow proves the architecture; the polish lands in Phase 2.5.
<!-- /adr-narrative -->

#### Consequences

**What became easier:**

- The 2-week sprint had a single, demonstrable success criterion: the 7 steps above. It ran, and it still runs — it is the regression baseline every later change is checked against.
- The wiring gap (the actual blocker) closed first because everything else depended on it.
- Solo-dev cognitive load dropped — only the modules touched by the demo loop needed to be understood deeply in week 1.

**What became harder, and how it resolved:**

- The migration audit was done under v1 rather than punted; it cost a day and was worth it.
- The frontend conventions doc was skipped for v1 and later written as [`frontend/CLAUDE.md`](../../../frontend/CLAUDE.md) ([D-32]/[D-33]/[D-34]).
- The observability stack was skipped and is still absent ([ADR-03](SPEC-03-platform-ops.md#adr-03)).
- The `/t/{tenant}/...` URL prefix was never adopted. Tenancy landed ([ADR-07](SPEC-01-account-identity-admin.md#adr-07)) with routes staying under plain `/api/v1`; the tenant is resolved by middleware (`RequireTenant` in `cmd/api/main.go`), not by the path.
- The [D-34] refresh-and-return route was replaced by the `portal_session` middleware gate plus `SessionKeeper` client-side silent refresh.

**What we said we'd revisit:**

- A Phase 0.5 sprint for the skipped items never ran as such; CI landed in Phase 6 (the `backend`, `lint`, `openapi`, `frontend` and `link-check` jobs in `.github/workflows/ci.yml` — there is no `sqlc-drift` job and never was, sqlc output is not committed), the conventions doc landed with the frontend work, observability has not.
- Phase 1 (tenancy + RLS) landed per ADR-07; the `me` synthetic tenant was not carried forward — personal organisations are real rows.
- The RBAC schism was resolved by [ADR-02](SPEC-01-account-identity-admin.md#adr-02) before tenancy, as intended.

#### Action items

1. [x] Pin this record (`Accepted`) before writing any code for the 2-week sprint (2026-07-06 status flip; v1 was built under this cut).
2. [x] Acceptance criterion tracked and met — the loop shipped 2026-07-06. (It was tracked in `MILESTONE_CHECKS.md`, deleted in `f11cf3f`; status now lives in code, see `/CLAUDE.md`.)
3. [ ] A `v1-out-of-scope` label in the issue tracker — GitHub Issues on `jackone2991/portal` exist now, but no such label does; the deferred list lives in [backlog.md § Deferred](../backlog.md) (and `/CLAUDE.md` § "Still deferred").
4. [x] Scope comment at the top of `backend/cmd/api/main.go`. It still cites the file this record was folded from (`docs/adr/01-v1-scope-cut.md`, retired by the 2026-10-01 fold); it is corrected to `docs/product/specs/README.md#adr-01` when that file is next touched.
5. [ ] Sprint-end retrospective — never written. The nearest thing was the 2026-07-11 spec-gap worklog (deleted — `git show ea100d8:docs/product/analysis/spec-gap-fix-worklog-2026-07-11.md`).

<a id="adr-08"></a>
### ADR-08 — Life-OS Positioning + Finance Ledger Scope

**Decided:** 2026-07-07 · **Status:** accepted; executed from 2026-07-12 (`6160f8e` media image pipeline, `66c036f` ledger); the status field was flipped only on 2026-09-11

Amends [ADR-01](#adr-01) · relates to D-27/D-28 (step-up/MFA) and
[ADR-06](SPEC-01-account-identity-admin.md#adr-06) · yardstick:
[vision.md](../vision.md), which keeps the product-yardstick role and is not
restated here.

#### Context

*As found on 2026-07-07. This is the decision the whole spec line
descends from; everything below the Decision holds as written.*

Portal's post-v1 gap analyses (`product/backlog.md` as it then was,
`product/analysis/facebook-comparison.md`, since deleted) measured the product against Facebook.
That yardstick made sense while porting the Olympus UI, but it embeds an
assumption Portal does not satisfy: Facebook's features derive value from network
effects, while Portal is self-hosted, single-VPS, and starts from **one user**.
Following the parity-driven backlog order (friends → messenger → people search)
would spend the scarce solo-dev budget on features that are near-worthless at n=1.

The owner's stated intent (2026-07-07): Portal should be tools supporting the
user's daily life and work — "like a human individual with their surrounding
facets: money, time, learning, social…". Several previously "orphan" spec items
(bank §8, calendar/birthdays, library verticals) are coherent under this framing
and incoherent under Facebook parity.

Two existing architectural assets make an integrated life platform more than a
bundle of clones: the **event bus** (hard rule: modules couple only via Asynq
`<module>:<event>`) and **one identity + RBAC** across all domains.

The immediate scope tension: the owner wanted **money** first, but ADR-01 deferred
"bank" wholesale, and D-27/D-28 gated bank behind MFA/step-up.

<!-- adr-narrative -->
#### Decision

1. **Portal is a self-hosted life OS**: one digital identity with facets — money,
   time, learning, social, entertainment. Facebook parity is retired as the
   backlog-ordering principle; `facebook-comparison.md` is reclassified as a
   historical analysis.
2. The existing newsfeed surface is re-purposed (long-term) as the user's **life
   stream**, fed by domain events. Every new domain module must emit at least one
   bus event from its first release.
3. **"Bank" is split.** A **finance ledger** (manual multi-account bookkeeping:
   accounts, transactions, categories, budgets, transfers — `product/specs/SPEC-12`)
   enters v1 scope. **Real bank integration** (credentials, API sync, money
   movement) remains deferred exactly as ADR-01 had it.
4. **MFA/TOTP gating is re-anchored**: D-27/D-28's "MFA before bank" applies to
   *credential-holding / money-moving* features, not to the manual ledger, which
   stores no bank credentials. TOTP becomes the named unlock task for real bank
   integration.
5. First build order under the new positioning: media image pipeline → comic
   vertical → finance ledger (`product/specs/`), with the notification module
   immediately after as the life-stream backbone.

#### Options considered

- **A. Continue the parity-driven order** (notifications → posts → friends →
  search). Rejected: optimizes believability of a Facebook clone, not value to the
  actual single user; friend graph and messenger are dead weight at n=1.
- **B. Life OS with finance ledger in scope** *(chosen)*: aligns effort with the
  owner's daily use; reuses the event bus as the differentiator; keeps risky bank
  features deferred.
- **C. Entertainment verticals only, defer all finance**: safest read of ADR-01,
  but leaves the owner's top-priority facet (money) unbuilt on a doctrinal
  technicality; the ledger's actual risk profile (no credentials) doesn't warrant it.
- **D. Full §8 bank module including debts/loans/investments now**: rejected;
  violates the v1 envelope and front-loads models (amortization, holdings) with no
  dogfooding behind them.

#### Trade-offs

- The Olympus social shell stays partially decorative for longer (friends panel,
  chat bar). Accepted: the shell is kept, only priorities move.
- Two positioning documents coexist during transition (old comparison, new vision);
  mitigated by reclassifying the comparison as historical.
- The ledger without statement import means manual entry only; accepted explicitly
  (owner's bank exports PDF → import needs OCR; schema is import-ready from
  migration #1 so the deferral costs nothing structurally).
- Finance data becomes the most sensitive data in the system while auth is
  password-only (no MFA). Accepted for a self-hosted single-user deployment;
  consequence noted below.
<!-- /adr-narrative -->

#### Consequences

What followed (checked 2026-10-01):

- **The build order ran as decided and kept going:** SPEC-04 (media image
  pipeline) → SPEC-14 (comic) → SPEC-12 (ledger) landed 2026-07-12, followed by
  SPEC-05 (notify), SPEC-07 and SPEC-09 (journal + life stream), SPEC-10
  (continue rail), SPEC-11 (people/birthdays), SPEC-03 (ops) and SPEC-13
  (ledger expansion: debts first) — the order and migrations are under
  [Build state and what remains](#build-state-and-what-remains). The `bank`
  module is the finance ledger; real bank integration (credentials, API sync,
  money movement) is still deferred, exactly as item 3 said.
- **The life stream exists, and is narrower than first built:** `journal`
  projects bank transactions (`bank:transaction_created` / `_updated` /
  `_deleted`), upcoming birthdays (`people:birthday_upcoming`) and finished
  playback (`media:playback_completed`), and cleans up on `media:asset_deleted`
  (`grep -n 'journalapi.TaskStream' backend/cmd/*/main.go`). Uploads
  (`media:asset_ready`, since `0033`) and comic publishing (since `0034`) are
  no longer projected; they reach the bell through notify instead (SPEC-09).
  Every event lives in [`reference/events.md`](../../reference/events.md);
  `notify:*` stayed reserved for the notification module, which shipped. The
  "every new module emits" rule is the **Events** convention above.
- **Backlog re-rank:** happened in the July backlog. That file was
  archived-in-place on 2026-08-25 and replaced under
  [ADR-11](SPEC-06-docs-canonicalisation.md#adr-11) by a live one,
  [backlog.md](../backlog.md). Of the demoted items, email password-reset came
  back and shipped (`0010`, SPEC-05); friend graph shipped a first slice as the
  `social` module (`0037`, SPEC-18); messaging and people search did not.
- **The admin wildcard does not reach another user's finance data by design:**
  `*` passes the bank route gate, but every bank query filters
  `user_id = caller` (SPEC-12 P0.8 — "no cross-user read or write at any level,
  including `*`"). The shipped exception is a code defect, not a grant: the
  derived-balance queries are not filtered by the caller (SPEC-12 §12 row 1).
- **MFA/TOTP is still the named unlock for real bank integration** and is not
  built ([ADR-06](SPEC-01-account-identity-admin.md#adr-06)). The ledger —
  including debts and interest accrual (SPEC-13) — runs under password-only
  auth, as the trade-off accepted.
- "Posts" changed meaning as predicted: the journal entry is the first real
  post type.
- ADR-01 remains in force for what it still defers: marketplace, creator
  economy, observability, LiveKit/mediamtx ([backlog.md § Deferred](../backlog.md)).
  Multi-tenancy/RLS is no longer on that list — ADR-07 executed.

#### Action items

- [x] Accept this record (executed from 2026-07-12; status field corrected 2026-09-11).
- [x] Backlog ordering note points at the specs — carried by today's [backlog.md](../backlog.md); `product/briefs/` was folded into the specs and deleted.
- [x] Historical label on `product/analysis/facebook-comparison.md`; the file was later deleted (`git show ea100d8:docs/product/analysis/facebook-comparison.md`).
- [x] Build order SPEC-04 → SPEC-14 → SPEC-12, notification module next — all four shipped.
- [ ] Revisit TOTP as a named prerequisite when any credential-holding bank feature is proposed. None has been; SPEC-13's items are all manual-entry.

<a id="adr-10"></a>
### ADR-10 — OpenAPI Contract Direction (spec-first, enforced)

**Decided:** 2026-07-11 (drafted 2026-07-08, from the 2026-07-08 gap audit) · **Status:** accepted

Relates to backlog §9 as it stood in 2026-07 (that file was replaced on
2026-09-11; `git log --follow -- docs/product/backlog.md`) · the **API
contract** convention above, which is the one statement of the rule ·
[ADR-09](SPEC-06-docs-canonicalisation.md#adr-09)'s canonical-source rule ·
[`backend/MODULES.md`](../../../backend/MODULES.md) § 8 (states the rule since
2026-09-11).

#### Context

*The state this decision was made against, as the 2026-07-08 audit found it.
Where it stands now is under Consequences and Action items.*

`shared/openapi.yaml` (OpenAPI 3.1, then ~730 lines, 12 paths) declared itself
the source of truth — its `info` block said *"Server stubs (Go) and the
TypeScript client are both generated from it,"* and CLAUDE.md repeated the rule.
On disk that claim was aspirational, not real:

- **No generated code existed.** `backend/internal/handler/api.gen.go` and
  `frontend/src/lib/types.gen.ts` were `.gitignore`d and had never been
  generated or committed — `internal/handler/` did not exist. `make openapi`
  was fully wired (oapi-codegen `chi-server` + `openapi-typescript`, config in
  `backend/oapi-codegen.yaml`) but had never been run in anger.
- **All 12 handlers were hand-written plain-chi** (6 account, 6 media) with zero
  `ServerInterface` — `grep -rn ServerInterface backend/` returned nothing. The
  hand-written `frontend/src/lib/api-client.ts` carried a TODO to switch to the
  generated types.
- **The contract already lied.** Handlers emitted the legacy `{code, message}`
  error body via a local `writeErr`, but the spec mandated RFC 7807 `Problem`.
  The auth-path drift (`/auth/register`, retired `/auth/callback`) had been fixed
  by hand; the error-shape drift persisted.
- **CI did not gate drift — of anything.** The `openapi` job only asserted the
  YAML parsed and had `openapi`/`info`/`paths` keys (a ~10-line Python check): no
  lint, no codegen, no spec-vs-handler comparison. (The audit also credited the
  `backend` job with an sqlc drift gate to mirror. It had none: sqlc output is
  `.gitignore`d and regenerated on every build, so there was nothing to diff.
  The *pattern* — regenerate, then `git diff --exit-code` — was still the right
  one; it just had no precedent in this repo.)

The forcing function: **SPEC-04/14/12 (and SPEC-05) were about to add ~30
endpoints** across media/comic/bank/notify onto a contract nothing
machine-checked. Each spec's Definition of Done said *"fix the drift in the same
or an earlier PR"* — but there was no mechanism to enforce that, so it would
rot. Two facts made timing decisive: the generated side was **greenfield**
(nothing committed to reconcile or delete), and the handler count was at its
**all-time low** (12, two wired modules). This was the cheapest the decision
would ever be; every week of deferral raised the price.

The underlying decision (backlog §9) had never actually been made: *adopt
oapi-codegen/openapi-typescript, or drop the spec as source of truth.* It is
expensive to reverse and touches every module — hence an ADR.

<!-- adr-narrative -->
#### Decision

**Keep `shared/openapi.yaml` as the single source of truth and make that real and
enforced (spec→code).** Concretely:

1. **Go: adopt oapi-codegen `chi-server` + `models`.** Handlers implement the
   generated `ServerInterface`; **bodies stay hand-written** — cookie/throttle/
   audit logic in the account handlers is unchanged, only signatures and
   request/response types come from generation. Routes register via the generated
   mux instead of ad-hoc `r.Post(...)`.
2. **TS: adopt `openapi-typescript`.** `api-client.ts` consumes `types.gen.ts`;
   the frontend gets end-to-end typed API access.
3. **Commit the generated artifacts** (remove `api.gen.go` + `types.gen.ts` from
   `.gitignore`). CI drift becomes the sqlc pattern: `make openapi` then
   `git diff --exit-code`. Committing means PRs show the contract surface changing —
   that diff *is* the contract review.
4. **Pin the toolchain.** Add oapi-codegen via a `tools.go` / `go.mod` tool
   directive (it is currently unpinned — no `tools.go`, absent from `go.mod`, so
   the drift gate would be non-reproducible without this).
5. **Replace the CI parse-check** with (a) a real lint (`redocly lint` or
   `vacuum`) and (b) the regenerate-and-diff drift gate for **both** Go and TS.
6. **Resolve the RFC 7807 drift as part of the cutover** — a module-wide
   `writeErr` → `Problem` helper, so the error contract stops lying.

Scope: do the cutover **now** on the two wired modules (account, media), before
SPEC-04. Thereafter each spec adds its paths spec-first, and an endpoint missing
from the spec fails CI — the specs' DoD becomes mechanical, not aspirational.

#### Options considered

- **A. Spec→code, enforced** *(chosen)*: makes the declared intent real, reuses
  the working sqlc drift-gate pattern, gives the frontend typed access for free,
  and closes the door on ~30 endpoints of future drift at the point of minimum cost.
- **B. Code→spec (swaggo annotations; generate the spec from handlers).** Rejected:
  inverts the source of truth the project has declared three times; imposes a
  per-handler annotation burden forever; swaggo's OpenAPI 3.1 support lags; the
  annotations drift from behavior just as easily as a hand-kept spec.
- **C. Drop the spec as source of truth (code-first, hand-written TS client).**
  Rejected: abandons a principle stated in the spec, CLAUDE.md, and MODULES.md;
  discards a ~730-line asset; lets frontend types drift silently into runtime bugs.
  Saves work now; the frontend repays it with interest. Cheapest today, most
  expensive across the four verticals.
- **D. Spec-first-lite: generate only the TS client, keep Go hand-written, add a
  custom route-set diff in CI.** Rejected: the path-presence check is bespoke
  tooling that catches *missing endpoints* but not *schema drift* (request/response
  body shape) — and body-shape is exactly where ledger/finance correctness bugs
  hide. oapi-codegen gives full-shape conformance for less long-run maintenance.

#### Trade-offs

- **Rewriting 12 handlers to the generated interface + fixing the error shape is
  real work now (~1 dev-day).** Accepted: one-time, at the all-time-low handler
  count, and it forecloses drift across the four incoming specs.
- **oapi-codegen `chi-server`'s interface is somewhat rigid.** The complex auth
  handlers keep hand-written bodies but must match generated signatures. Mitigated:
  chi-server generates routing + types, not logic — the throttle/cookie/audit code
  is untouched.
- **Committing generated code adds diff noise on contract changes.** Accepted —
  that noise is the point; it surfaces contract changes in review.
- **One more pinned tool in the build.** Accepted: mirrors sqlc; reproducibility
  is the whole reason for the gate.
- **The spec still covers only ~2 of ~7 modules.** Accepted: the specs backfill
  their own paths as they land; enforcement from now prevents the gap widening.
<!-- /adr-narrative -->

#### Consequences

What followed, checked against the tree on 2026-10-01. The working rule itself
is the **API contract** convention above; this list records how it came to be
true and where it still is not.

- **The drift gate runs, and is required.** `api.gen.go` and `types.gen.ts`
  (`wc -l backend/internal/handler/api.gen.go frontend/src/lib/types.gen.ts`)
  have been committed since `6160f8e` (2026-07-12); `.gitignore` excludes sqlc
  output only; the `openapi` job runs `make openapi` then
  `git diff --exit-code`. From `edadf28` (2026-07-08) to `a30b887` (2026-09-11)
  the job never reached that step — `frontend/pnpm-lock.yaml` had been deleted
  while `setup-node` still cached on it — so for two months the gate was true on
  paper only. `a30b887` committed the lockfile; since then the job runs to its
  diff and is green on `main` (`gh run list --workflow ci.yml --branch main`),
  and branch protection on `main` requires it ("OpenAPI contract (parse +
  codegen drift gate)" — `gh api repos/jackone2991/portal/branches/main/protection`).
- **Spec-first held for every module that came after.** The spec is
  `wc -l shared/openapi.yaml` lines (5,187 at last check) and
  `python3 -c "import yaml;print(len(yaml.safe_load(open('shared/openapi.yaml'))['paths']))"`
  paths (111), tagged for every wired module — account/admin, media, movies,
  music, stories, comic, bank, journal, notifications, layout, ops, people,
  social, tenant, platform. The "~2 of ~7 modules" gap closed.
- **The error contract stopped lying — but by a different route.**
  `internal/platform/server.Problem` became the single RFC 7807 writer on
  2026-08-25; the four surviving `writeErr`/`writeError` shims (account, media,
  notify, tenant) delegate to it, and `schemas/Error` is deprecated with no
  referent (the **Errors** convention above). This landed without the
  `ServerInterface` cutover it was scoped under.
- **No handler implements the generated `ServerInterface`.** `api.gen.go` is
  its only referent in the tree (`grep -rln ServerInterface backend`). Every
  handler is still hand-written plain-chi, so the gate proves the generated
  code matches the spec, **not** that handler behaviour does (the **API
  contract** convention above).
- **The frontend is not typed end-to-end.** `types.gen.ts` is committed, but
  `grep -rln 'from "./types.gen"' frontend/src` finds one importer
  (`lib/comic-sync.ts`); `api-client.ts` still opens with "Once `make openapi`
  runs …" and the other `lib/*.ts` modules hand-declare their types. The typed
  client the decision promised is available and unused.
- **backlog §9 closed** (the either/or is resolved). The specs' DoD is
  enforceable for *path presence and schema shape*; handler conformance stays
  a review question.
- The spec's own `info`-block claim ("stubs and client are generated from it")
  is true. CLAUDE.md's "don't hand-edit generated files" protects real files.
- **`backend/MODULES.md` §8 gained its step on 2026-09-11** — two months after
  this record said it would; until then a new module that followed the
  checklist failed the `openapi` job. ADR-09's canonical-source rule holds: the
  contract stays at `shared/openapi.yaml`; `docs/reference/` points at it.

#### Action items

- [x] Accepted 2026-07-11; generated files un-ignored and committed (`6160f8e`); CI regenerate-and-diff for Go + TS (`openapi` job, `6160f8e`; reachable since `a30b887`); `Problem` error helper (`internal/platform/server`, 2026-08-25); every spec since SPEC-04 added its paths spec-first; `backend/MODULES.md` §8 and CLAUDE.md state the rule (2026-09-11).
- [ ] Pin oapi-codegen for **local** runs. CI pins `oapi-codegen@v2.7.2`
      (`.github/workflows/ci.yml`) and `openapi-typescript ^7.4.0`
      (`frontend/package.json`), but `backend/go.mod` has no `tool` directive
      and there is no `tools.go`, so `make openapi` on a developer machine uses
      whatever version is on `PATH` — and a version skew produces a diff the
      gate rejects.
- [ ] Refactor handlers onto the generated `ServerInterface` — **none done**,
      account and media included. Retrofit per module as each is touched. (The
      "cutover PR before SPEC-04" as scoped never landed; only the gate did.)
- [ ] Replace the parse-check with a real lint (`redocly lint` / `vacuum`) —
      the job still only checks that the YAML parses.
- [ ] Make the frontend consume `types.gen.ts` beyond `comic-sync.ts`, or
      strike the "typed client" claim from the spec's `info` block.

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
- **2026-10-02** — a review of SPEC-01 produced twelve owner decisions about
  the account module (A1–A12, "Decisions recorded 2026-10-02"), closing its
  Q2–Q5; SPEC-01 rev 4 states them and appends seven gap rows (gap total
  263 → 270). The same day a second round (B1–B12, "Decisions recorded
  2026-10-02 (second round)") closed the last twelve open owner questions —
  SPEC-01 Q6 and the eleven left in SPEC-02 and SPEC-15–18 — appended six gap
  rows, extended SPEC-01 row 29 and narrowed SPEC-02 row 13 (gap total
  270 → 276). A later decision that day, B13 (tenant asset visibility),
  replaced B5's signed-URL mechanism and amended B6: SPEC-04 rev 7 gains P0.8,
  five gap rows are appended and SPEC-15 rows 27 and 29 are rewritten (gap
  total 276 → 281). The owner then revised B13 as B14, recorded as
  [ADR-12](SPEC-01-account-identity-admin.md#adr-12) — the audience becomes
  household plus friends in mutually linked tenants, the value `shared`, and
  the `tenant` module gains links (SPEC-01 P0.17): the seven B13 rows are
  rewritten in place and three appended (gap total 281 → 284). No owner
  question is open.
