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
| [SPEC-01](SPEC-01-account-identity-admin.md) | Account — local auth, approval gate, RBAC, admin console, per-user timezone (as-built, retroactive) | `account` | [ADR-02](SPEC-01-account-identity-admin.md#adr-02), [ADR-06](SPEC-01-account-identity-admin.md#adr-06); SPEC-05 (`notify:dispatch`; reset is SPEC-05 P0.3) | Built (`0002`–`0004`, `0006`, `0010`, `0031`); P0.13 timezone, P0.16 audit retention and the 2026-10-02 targets (§11 rows 25–31) unbuilt; also holds the `tenant` module's P0.17 (tenant links, [ADR-12](SPEC-01-account-identity-admin.md#adr-12); rows 32–33, unbuilt) and P0.18 (tenant groups, B15; row 34, unbuilt) |
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
| [SPEC-12](SPEC-12-finance-ledger.md) | Finance ledger (Money-Lover-class) | `bank` | [ADR-08](SPEC-09-life-stream-home.md#adr-08); SPEC-03 P0 live before the first real ledger entry; SPEC-04 (P1 receipts only) | Built (`0014_bank_core`, `0042`) |
| [SPEC-13](SPEC-13-ledger-expansion.md) | Ledger expansion — debts, goals, recurring, cards, net worth, automation, splits, sharing | `bank` (extends) | SPEC-12; SPEC-05 for reminders; SPEC-04 (receipts only, via SPEC-12 P1.10) | Phase 1 (debts, `0043_bank_debts`) built; later phases open |
| [SPEC-14](SPEC-14-comic-vertical.md) | Comic vertical, end-to-end | `comic` | SPEC-04 | Built (`0015_comic_core`; reader + import + sync `0024`–`0030`) |
| [SPEC-15](SPEC-15-music-vertical.md) | Music vertical — tracks, zip/multi-file import, enrich + MusicBrainz lookup, playlists, player (as-built, retroactive) | `music` | SPEC-04 (ingest, `/original`, covers, `media:asset_deleted`); SPEC-05 (bell consumer) | Built (`0022_music_core`; `0038`, `0039`, `0041`) |
| [SPEC-16](SPEC-16-movie-vertical.md) | Movie vertical — catalogue over media video assets (as-built, retroactive) | `movie` | SPEC-04, SPEC-14 (pattern), SPEC-10 (asset-level resume) | Backend built (`0021_movie_core`); no frontend |
| [SPEC-17](SPEC-17-story-vertical.md) | Story vertical — stories + Markdown chapters (as-built, retroactive) | `story` | SPEC-04 (covers), SPEC-14 (pattern) | Backend built (`0023_story_core`); reader is a placeholder |
| [SPEC-18](SPEC-18-social-connections.md) | Social connections — request / accept / decline / disconnect; the social layer's first slice (as-built, retroactive) | `social` | account (`accountapi`); SPEC-05 (bell consumers); SPEC-11 (`/people/suggestions`) | Built (`0037_social_connections`) |

The positioning decision (life-OS pivot) and the parking lot are **not** specs;
they live in [ADR-08](SPEC-09-life-stream-home.md#adr-08) (a record in SPEC-09, the life stream it
was proved on; with [vision.md](../vision.md), the product yardstick) and [backlog.md § Deferred](../backlog.md).

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
  side read; the owning tenant's owner writes). **`tenant_groups`** and
  **`tenant_group_members`** (planned, SPEC-01 P0.18, Decision 2026-10-02b
  (B15)) are ordinary tenant-scoped tables under `tenant_isolation`; a member
  row also needs the User's `organization_memberships` row in that tenant. **One planned exception to the
  fence, and only one** ([ADR-12](SPEC-01-account-identity-admin.md#adr-12),
  Decision 2026-10-02b (B14)): a `FOR SELECT` policy may admit a row of
  another tenant only through the tenant module's `SECURITY DEFINER` function
  `app_can_read_shared(owner_id, tenant_id)` — owner, group co-member in that
  tenant (B15), or accepted friend in that or an actively linked tenant — and
  only for published content
  (`movies`, `music_tracks`, `stories`, `story_chapters` with `status =
  'published'`) and the `shared` assets that render it; no write policy ever
  admits another tenant, and no other table gets such a policy without a new
  decision record. Each
  spec §6 states per table whether it is
  tenant-scoped; DDL in specs drafted before ADR-07 omits the columns, and this
  bullet governs.
- **Events**: every task/event name lands in
  [../../reference/events.md](../../reference/events.md) as part of definition of
  done; every new domain module emits ≥ 1 bus event from day one ([ADR-08](SPEC-09-life-stream-home.md#adr-08)).
  There is no exemption *(owner decision 2026-10-01b (D3); [ADR-17](SPEC-05-notification-module.md#adr-17))*: `account` and
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
- **Pagination** ([ADR-16](SPEC-03-platform-ops.md#adr-16)): list endpoints use an opaque base64 keyset
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
- **Timezone** *(owner decision 2026-09-30, revised 2026-10-02 (A8); D-17;
  [ADR-15](SPEC-01-account-identity-admin.md#adr-15))*:
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
  same PR ([ADR-10](SPEC-03-platform-ops.md#adr-10), accepted 2026-07-11; the CI `openapi` job
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

[ADR-10](SPEC-03-platform-ops.md#adr-10)'s spec-first CI gate is in force; the `ServerInterface`
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
built — and appended three (SPEC-01 rows 32–33, SPEC-18 row 15). B15 (tenant
groups decide who is family, amending ADR-12) appended one (SPEC-01 row 34)
and extended SPEC-01 row 32 in place. B16 (what a blank chapter body is)
appended one (SPEC-17 row 23).

| Spec | Gap section | Rows | By severity (row numbers) | Most severe |
|------|-------------|-----:|---------------------------|-------------|
| [SPEC-01](SPEC-01-account-identity-admin.md) | [§11](SPEC-01-account-identity-admin.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 34 | Sec 1–5 · AuthZ 6–7 · Data 8 · Integ 9–13 (10 superseded by 26) · Func (timezone) 14 · Contract 15–20 · UX 21–22 · Hyg 23 · P1 24 · Sec 25–28 · Data 29 (extended, 2026-10-02b) · Sec 30 · Data 31 (appended, 2026-10-02) · Sec 32 · AuthZ 33 (appended, 2026-10-02b B14 — the `tenant` module) · Sec 34 (appended, B15 — tenant groups) | re-parenting a role under `superadmin` escalates every holder to `*` (1); refresh rotation is check-then-act, so two concurrent presentations fork the chain (2) |
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
| [SPEC-17](SPEC-17-story-vertical.md) | [§11](SPEC-17-story-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 23 | Data 1 · Integ 2–6 · AuthZ 7–8 · Contract 9–15, 17 · Hyg 16 · P1 18–20 · Integ 21 (appended, 2026-10-02b) · AuthZ 22 (appended, B13; rewritten by B14) · Integ 23 (appended, B16) | the `media:asset_deleted` consumer runs with no tenant scope (1); a chapter created without `sort_order`, or with a duplicate, is a 500 at COMMIT (2) |
| [SPEC-18](SPEC-18-social-connections.md) | [§11](SPEC-18-social-connections.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 15 | Data 1 · Integ 2–3 · Contract 4–6 · UX 7 · Test/docs 8–9 · Contract 10–11 · Hyg 12 · Func 13 · P1 14 (appended, 2026-10-02b) · AuthZ 15 (appended, B14) | events published before COMMIT leave a phantom bell entry (1); a concurrent duplicate request aborts the transaction → 500 (2) |
| **Total** | | **286** | | |

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
  by B14 and amended by B15; [ADR-12](SPEC-01-account-identity-admin.md#adr-12)):
  SPEC-01 §11 row 34 (`000N_tenant_groups`, the groups API and screen —
  B15) migrating first, in one PR with row 32 (`000N_tenant_links`,
  `portal_acl`, `app_tenants_linked` and `app_can_read_shared`, the links API
  and screen, `tenantapi` reach), with
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
  non-owners (SPEC-14 P0.2 (a)) SPEC-14 row 9 · SPEC-17 row 21 (on SPEC-17
  row 23's `story_body_is_blank`); D-34 matcher
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
  above. → [ADR-16](SPEC-03-platform-ops.md#adr-16)
- **Timezone** — per user, from the device location, default
  `Asia/Ho_Chi_Minh`, with a `timezone_manual` override; sweeps evaluate each
  owner's local date. (Revised by Decision 2026-10-02 (A8): the stored zone
  may be NULL, `Asia/Ho_Chi_Minh` is the readers' fallback rather than a
  column default, and the manual override is gone.) Detail: the **Timezone** convention above; the D-17
  update in [feature-inventory.md](../feature-inventory.md); SPEC-12 P0.6,
  SPEC-07 P0.4, SPEC-09 P0.1 / P0.3 / P1.5, SPEC-11 P0.3 / P0.4, SPEC-13 §4a.
  → [ADR-15](SPEC-01-account-identity-admin.md#adr-15)
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
  [feature-inventory.md](../feature-inventory.md). → [ADR-16](SPEC-03-platform-ops.md#adr-16)
- **(f) `timezone_manual` and `account/invalid-timezone` are confirmed** —
  *superseded by Decision 2026-10-02 (A8)* as to `timezone_manual`, which is
  dropped everywhere (no column, no field, no request flag); the
  `account/invalid-timezone` slug stands. Kept here as the record of what was
  decided on 2026-10-01. The current rule is the **Timezone** convention above;
  the code follow-up is SPEC-01 P0.13 (§11 row 14), the first item of the
  **Per-user timezone** cross-cutting gap. → [ADR-15](SPEC-01-account-identity-admin.md#adr-15)
- **(g) `{items}` for non-paginated lists is confirmed** — the **Pagination**
  convention above (with the rationale and what counts as a collection); the
  D-29 update in [feature-inventory.md](../feature-inventory.md). Every spec §7
  complies; the shipped divergences are the Envelopes rows indexed above.
  → [ADR-16](SPEC-03-platform-ops.md#adr-16)

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
  → [ADR-13](SPEC-01-account-identity-admin.md#adr-13)
- **(D2) Movie and story are finished, not reverted** — SPEC-16 P1.1 and
  SPEC-17 P1.1 are committed scope (still unbuilt), to the music standard;
  [backlog.md](../backlog.md) P2 line 28 records the decision. No new gap row:
  the frontends are SPEC-16 §11 row 16 and SPEC-17 §11 row 18.
- **(D3) `account` and `layout` emit bus events; notify tells the
  superadmins** — no exemption from [ADR-08](SPEC-09-life-stream-home.md#adr-08)'s rule (the **Events** convention
  above). SPEC-01 P1.3 and §8 (seven `account:*` events, `accountapi.AdminEvent`,
  `SuperadminIDs`), SPEC-02 P1.4 and §8 (one `layout:changed {part}`), SPEC-05
  P1.5 (eight `notify:on_*` consumers, recipients = superadmins minus the actor,
  registration deduplicated against the approver dispatch, email forced only
  for refresh-token reuse); [events.md](../../reference/events.md) planned rows.
  Gap rows: SPEC-01 §11 row 24, SPEC-02 §11 row 14, SPEC-05 §11 row 23;
  TC-ACC-120…126, TC-LAY-050…052, TC-NOTIFY-140…145. → [ADR-17](SPEC-05-notification-module.md#adr-17)
- **(D4) `user` may author music, movies and stories** — SPEC-15 P1.4,
  SPEC-16 P1.3, SPEC-17 P1.3 (each a module-owned `000N_<module>_user_write_grant`
  granting `:write:own` and `:publish:own` to `user`, the
  `0025_comic_user_write_grant` shape; `:any` and delete-any unchanged; useful
  with F009). Gap rows: SPEC-15 §12 row 26, SPEC-16 §11 row 18, SPEC-17 §11
  row 20; TC-MUS-004, TC-MOV-050, TC-STY-070.

## Decisions recorded 2026-10-02

The owner settled twelve points about the account module in a review of
SPEC-01, among them its four open questions (Q2–Q5). No decision record was
written at the time: each is stated in SPEC-01's requirement text, which is the
detail; this list only routes. The significant ones were later promoted to
records — A5 to [ADR-16](SPEC-03-platform-ops.md#adr-16), A6 to
[ADR-13](SPEC-01-account-identity-admin.md#adr-13), A7 to
[ADR-14](SPEC-01-account-identity-admin.md#adr-14), A8 to [ADR-15](SPEC-01-account-identity-admin.md#adr-15),
A11 to [ADR-17](SPEC-05-notification-module.md#adr-17) — marked "→" below. The vocabulary — User, Session, Approval, Rejected,
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
  row (shipped behaviour). → [ADR-16](SPEC-03-platform-ops.md#adr-16)
- **(A6) Deleting a User purges every module, then the row** — SPEC-01 P0.10:
  an idempotent `PurgeOwnerData` per module, a registry run content → media →
  tenant, no grace period, a `deleted_users` snapshot kept 90 days, the email
  freed (keep someone Rejected to refuse them), a restore can resurrect a
  deleted User ([backup-restore.md](../../operations/backup-restore.md)).
  Extends Decision 2026-10-01b (D1). Gap row: §11 row 29; TC-ACC-146…149.
  → [ADR-13](SPEC-01-account-identity-admin.md#adr-13)
- **(A7) Audit identity data lives 90 days** — SPEC-01 P0.16 (owned jointly
  with `platform/audit`): every `audit_log` row's identifying data encrypted
  with `AUDIT_PII_KEY`, readable only by Superadmins, anonymised after 90
  days by `account:expire_identity_data` ([events.md](../../reference/events.md)).
  Gap row: §11 row 31; TC-ACC-150…153.
  → [ADR-14](SPEC-01-account-identity-admin.md#adr-14)
- **(A8) Timezone: NULL means "not set", no manual flag** — the **Timezone**
  convention above, SPEC-01 P0.13; supersedes Decision 2026-10-01 (f) as to
  `timezone_manual`. Gap row: §11 row 14, rewritten in place (unbuilt);
  TC-ACC-100…104. → [ADR-15](SPEC-01-account-identity-admin.md#adr-15)
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
  row 24, rewritten in place (unbuilt); TC-ACC-127. → [ADR-17](SPEC-05-notification-module.md#adr-17)
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
  → [ADR-14](SPEC-01-account-identity-admin.md#adr-14)
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
  for the owner to confirm are listed in its Consequences. *Amended by B15
  below*: "family" is the owner's group co-members, not every member of the
  tenant.
- **(B15) Family is a group inside the tenant, not the whole tenant** — the
  owner's amendment of B14, recorded in
  [ADR-12](SPEC-01-account-identity-admin.md#adr-12) as an explicit
  "Amended 2026-10-02 (B15)" paragraph after its narrative. A tenant can hold
  several **groups** (one household each); a User's family, for reading
  published music, movies and stories, is the Users who share a group with
  them in the item's tenant. SPEC-01 P0.18 (the `tenant` module:
  `000N_tenant_groups` — `tenant_groups`, `tenant_group_members` with a
  foreign key to `organization_memberships`, tenant-scoped RLS — numbered
  before `000N_tenant_links`; the permission `tenants:groups:write` granted
  to no role; `GET`/`POST /tenants/{id}/groups`, `PATCH`/`DELETE
  /tenants/{id}/groups/{gid}`, `PUT /tenants/{id}/groups/{gid}/members`,
  `DELETE /tenants/{id}/groups/{gid}/members/me`; slugs
  `tenant/group-not-found`, `tenant/not-a-member`, `tenant/group-name-taken`;
  `tenant:group_changed` emit-only; no `tenantapi` addition);
  `app_can_read_shared`'s family clause becomes "shares a group in the item's
  tenant", its friend clause names the same tenant explicitly, and
  `portal_acl` also reads `tenant_group_members` (SPEC-01 P0.17). Discovery
  (SPEC-18) is unchanged — still by tenant and links. SPEC-04 P0.8, SPEC-10
  P0.2, SPEC-15, SPEC-16 and SPEC-18 reworded where they defined the
  household; SPEC-17 was aligned in a follow-up (P0.4, P0.5, §6, §11 row 22)
  and SPEC-14 keeps comics out (§3). Gap row: SPEC-01 §11
  row 34 (new), row 32 extended; TC-TEN-011…017, TC-TEN-005 and TC-TEN-010
  reworded. ADR-12 action item 7 records a precondition on tenant switching
  (no gap row). The drafting choices are listed in ADR-12's Consequences.
- **(B16) A blank chapter body is defined from the stored data** — SPEC-17
  P0.3 and §6. Nothing normalises `body_md` on the way in, so a body is blank
  when it holds only Unicode `White_Space` characters plus U+200B–U+200D,
  U+2060 and U+FEFF, written as explicit code points (never a locale- or
  regex-class-dependent test); markup is content, so `&nbsp;`, `<br>`, a
  lone `#` or `---` is not blank. One `IMMUTABLE` SQL function,
  `story_body_is_blank(text)` from the story-owned `000N_story_blank_body`,
  is the only place the set is written: the publish check, the non-owner
  reader filter (B7, row 21) and `chapter_count` all call it, and the Go
  helper and any P1.1 TypeScript copy use the same code points. A read-only
  verification script in SPEC-17 §6 (old `btrim` vs new classification, with
  the raw bytes) runs against the real database first, and its counts go in
  the PR. Gap row: SPEC-17 §11 row 23 (new; row 21 lands with or after it);
  TC-STY-114…117.

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
under `docs/adr/` until the 2026-10-01 fold; the `ADR-NN` IDs that code
comments and documents cite are unchanged, and each record keeps a fixed
`<a id="adr-nn"></a>` anchor. This section indexes all seventeen; each lives
in the spec that owns its subject, under that spec's `## Decision records`.
(Until 2026-10-02 this README also held six — ADR-01, 08, 10, 15, 16 and 17;
they moved verbatim to SPEC-03, SPEC-09, SPEC-01 and SPEC-05, anchors
unchanged.)

Each record keeps the binding shape — Context → Decision → Options considered →
Trade-offs → Consequences → Action items ([STYLE.md](../../STYLE.md)). Decision,
Options considered and Trade-offs are the narrative layer, kept verbatim (ADR-11
rule 2, [SPEC-06](SPEC-06-docs-canonicalisation.md#adr-11)): they record what
was known at the time and may name things since retired. Context, Consequences
and Action items are the fact layer, corrected in place and true as of the
hosting spec's `Last verified`. Where a convention of this README already
states a fact, the record points to it instead of repeating it. A reversed
decision gets a new record that supersedes the old one. Numbers are never reused; `00` is retired
(that file was a review, deleted after `ea100d8` —
`git show ea100d8:docs/product/analysis/architecture-review-2026-05-24.md`).

**The framing constraint every record inherits:** **1 dev · 2-week bursts ·
≤ $100/mo · single VPS** ([ADR-01](SPEC-03-platform-ops.md#adr-01)).

| ADR | Title | Status | One line | Location |
|---|---|---|---|---|
| ADR-01 | v1 scope cut | accepted, amended by ADR-08, executed | What v1 is — and everything it is not | [SPEC-03 § ADR-01](SPEC-03-platform-ops.md#adr-01) |
| ADR-02 | RBAC model reconciliation | accepted, amended by ADR-06 | Role hierarchy is canonical for v1; policy bundles layer on top later | [SPEC-01 § ADR-02](SPEC-01-account-identity-admin.md#adr-02) |
| ADR-03 | Single-VPS topology | accepted | One VPS, compose profiles as the envelope; observability/live profiles stay off | [SPEC-03 § ADR-03](SPEC-03-platform-ops.md#adr-03) |
| ADR-04 | Storage tier & budget | accepted | R2 for prod, MinIO kept for local dev (presigned uploads need an S3 origin) | [SPEC-04 § ADR-04](SPEC-04-media-image-pipeline.md#adr-04) |
| ADR-05 | Phase 0 wiring order | accepted, executed | The critical path to a running demo — closed; kept for the shape of the work | [SPEC-03 § ADR-05](SPEC-03-platform-ops.md#adr-05) |
| ADR-06 | Local auth model | accepted, executed | Passwords in Portal (Argon2id + JWT); Authentik/OIDC removed | [SPEC-01 § ADR-06](SPEC-01-account-identity-admin.md#adr-06) |
| ADR-07 | Multi-tenancy / RLS model | accepted, executed | Tenant column + RLS policies; enforced only when the app connects as `portal_app` | [SPEC-01 § ADR-07](SPEC-01-account-identity-admin.md#adr-07) |
| ADR-08 | Life-OS pivot + finance ledger scope | accepted, executed | Portal is a life OS; ledger in scope; "real bank" stays deferred | [SPEC-09 § ADR-08](SPEC-09-life-stream-home.md#adr-08) |
| ADR-09 | Documentation architecture | accepted, amended by ADR-11, executed | Diátaxis-informed `docs/` tree; English canonical | [SPEC-06 § ADR-09](SPEC-06-docs-canonicalisation.md#adr-09) |
| ADR-10 | OpenAPI contract direction | accepted | Spec-first, enforced: generate Go stubs + TS client; CI drift gate | [SPEC-03 § ADR-10](SPEC-03-platform-ops.md#adr-10) |
| ADR-11 | Documentation canonicalisation | accepted, executed | One owner per fact; ADRs corrected in place by layer; nothing archived | [SPEC-06 § ADR-11](SPEC-06-docs-canonicalisation.md#adr-11) |
| ADR-12 | Sharing published content with household and friends; tenant links | accepted (Decision 2026-10-02b (B14)), amended by Decision 2026-10-02b (B15), not built | Published music, movies and stories are read by the owner's family — group co-members in the item's tenant (B15) — and friends in the same or mutually linked tenants, through one `SECURITY DEFINER` predicate in the SELECT policies — a read-only exception to ADR-07's fence | [SPEC-01 § ADR-12](SPEC-01-account-identity-admin.md#adr-12) |
| ADR-13 | Deleting a User purges every module, then the row | accepted (Decisions 2026-10-01b (D1), 2026-10-02 (A6)), not built | Each module's `PurgeOwnerData` runs in a fixed order, then the row goes under a lock; 503 `account/delete-incomplete` otherwise; no grace period, a 90-day `deleted_users` snapshot, the email freed | [SPEC-01 § ADR-13](SPEC-01-account-identity-admin.md#adr-13) |
| ADR-14 | Identity data that outlives its User | accepted (Decisions 2026-10-02 (A7), 2026-10-02b (B1)), not built | Audit and snapshot identity data sealed with `AUDIT_PII_KEY`, decryptable only by Superadmins, anonymised after 90 days | [SPEC-01 § ADR-14](SPEC-01-account-identity-admin.md#adr-14) |
| ADR-15 | Per-user timezone | accepted (Decisions 2026-09-30, 2026-10-01 (f), 2026-10-02 (A8)), not built | Day and month boundaries in the User's own zone, NULL = not set, `Asia/Ho_Chi_Minh` fallback; sweeps evaluate each owner's local date | [SPEC-01 § ADR-15](SPEC-01-account-identity-admin.md#adr-15) |
| ADR-16 | List API contract | accepted (Decisions 2026-09-30, 2026-10-01 (e), (g), 2026-10-02 (A5)), partly built | Every collection is `{items[, next_cursor]}`, keyset cursors, a lenient clamped `limit`; admin users keep offset as the one exception | [SPEC-03 § ADR-16](SPEC-03-platform-ops.md#adr-16) |
| ADR-17 | Every module emits bus events; notify tells the Superadmins | accepted (Decisions 2026-10-01b (D3), 2026-10-02 (A11)), not built | No exemption from ADR-08's rule: `account` and `layout` emit, and the Superadmins (effective `*`) get each admin change in the bell | [SPEC-05 § ADR-17](SPEC-05-notification-module.md#adr-17) |

**When to write one.** A choice that (a) is expensive to reverse, (b) crosses
module boundaries, or (c) contradicts a previous record or the scope cut gets a
record, always filed inside a spec — under `## Decision records` in the one
that owns its subject — with the next unused number; this README holds only
the index above. Day-to-day feature
decisions belong in [feature-inventory.md](../feature-inventory.md) as `D-N`
entries; specs cite both kinds by ID.

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
  rewritten in place and three appended (gap total 281 → 284). B15 then
  narrowed the family half to the owner's tenant-group co-members (SPEC-01
  P0.18, an amendment to ADR-12): one row appended (gap total 284 → 285). B16
  defined a blank story chapter body from the stored data (SPEC-17 P0.3): one
  row appended (gap total 285 → 286). The six decision records this README
  still held (ADR-01, 08, 10, 15, 16, 17) moved verbatim into SPEC-03, SPEC-09,
  SPEC-01 and SPEC-05, so every record now lives in a spec and this README
  keeps only the index. No owner question is open.
