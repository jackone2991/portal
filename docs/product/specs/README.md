# Portal — Detailed Feature Specs (PRD level)

**Status:** current · **Last verified:** 2026-10-01

**Language policy:** English only, per owner decision 2026-07-07 (ADR-09; the
former vi mirror was deleted in `f11cf3f` — see
[ADR-11](../../adr/11-docs-canonicalisation.md)).

This folder holds the **detailed, implementation-ready specs**. A spec answers
*exactly what to build, how to know it's done, and what to decide before
starting*. SPEC-01…03 and 05…09 began as product briefs (brainstorm-level *what
and why*, 2026-07-07 and 2026-07-10); once every brief had been promoted, the
lines no spec carried yet were folded into its spec and the `briefs/` folder was
deleted (`git show 2cdda7e:docs/product/briefs/`). SPEC-04, 10, 11 and 12 had
no brief. Every header names its upstream.

## Documents

The **Status** column is read from the code (the module under
`backend/internal/modules/` and the migration that created its tables), not from
spec headers. Per-requirement coverage lives in
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md).

| Spec | Feature | Module | Depends on | Status |
|------|---------|--------|------------|--------|
| [SPEC-01](SPEC-01-media-image-pipeline.md) | Media image pipeline + asset management | `media` | — | Built (`0008_media_image_pipeline`) |
| [SPEC-02](SPEC-02-comic-vertical.md) | Comic vertical, end-to-end | `comic` | SPEC-01 | Built (`0015_comic_core`; reader + import + sync `0024`–`0030`) |
| [SPEC-03](SPEC-03-finance-ledger.md) | Finance ledger (Money-Lover-class) | `bank` | ADR-08; SPEC-09 P0 live before the first real ledger entry; SPEC-01 (P1 receipts only) | Built (`0014_bank_core`, `0042`) |
| [SPEC-04](SPEC-04-notification-module.md) | Notification module (life-stream backbone) | `notify` | SPEC-01 P0.6 (fan-out); P1.2 not a gate | Built (`0009_notify_notifications`, `0010`) |
| [SPEC-05](SPEC-05-journal.md) | Journal — life-stream write path | `journal` | SPEC-01 P0.6 (fan-out) only; photos via SPEC-12 | Built (`0011_journal_entries`) |
| [SPEC-06](SPEC-06-life-stream-home.md) | Life-stream home — projection + dashboard | `journal` + home | SPEC-05; producers attach as they land | Built (`0017_journal_stream_items`) |
| [SPEC-07](SPEC-07-continue-rail.md) | Playback resume + continue rail (D-20) | `media` | SPEC-01 P0.6 (P1.5 event); comic leg after SPEC-02 | Built (`0013_media_playback_progress`) |
| [SPEC-08](SPEC-08-people-registry.md) | People registry — contacts + birthdays | `people` | SPEC-01 P0.3 (scheduler) + P0.6 (fan-out); avatars need SPEC-01 | Built (`0016_people_persons`, `0035`) |
| [SPEC-09](SPEC-09-platform-ops.md) | Platform ops — backup/restore, queue console, takeout | `ops` | — (land P0 before SPEC-03 data accrues) | P0 built (`0012_ops_backup_runs`); P1.7 takeout unbuilt |
| [SPEC-10](SPEC-10-ledger-expansion.md) | Ledger expansion — debts, goals, recurring, cards, net worth, automation, splits, sharing | `bank` (extends) | SPEC-03; SPEC-04 for reminders; SPEC-01 (receipts only, via SPEC-03 P1.10) | Phase 1 (debts, `0043_bank_debts`) built; later phases open |
| [SPEC-11](SPEC-11-docs-canonicalisation.md) | Docs canonicalisation — 79 stale ADR statements, 95 links, four CI docs checks | none (docs + CI) | ADR-11 | Executed 2026-09-11 |
| [SPEC-12](SPEC-12-journal-attachments.md) | Journal attachments — Attachments (≤ 10 image Assets) and Location as first-class Entry properties; the markdown-link workaround backfilled out of every body (issue #8) | `journal` (extends) · frontend | SPEC-01, SPEC-05, SPEC-06 | Executed 2026-09-19 (`0044`, `0045`) |

The positioning decision (life-OS pivot) and the parking lot are **not** specs;
they live in [ADR-08](../../adr/08-life-os-pivot.md) (with
[vision.md](../vision.md)) and [backlog.md § Deferred](../backlog.md).

## Conventions binding on all specs

- **Spec header**: `Status:` uses [STYLE.md](../../STYLE.md)'s vocabulary
  (`draft` before build; `current` once the spec describes shipped behaviour;
  `historical` for executed one-shot specs such as SPEC-11). There is exactly
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
  FKs — with two sanctioned **identity-anchor FKs**: `users(id)` (SPEC-04 §6,
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
  RLS it would see no tenant's rows. Global tables (`users`, `roles`,
  `permissions`, `role_permissions`, `user_roles`) and system tables
  (`ops_backup_runs`) are exempt. Each spec §6 states per table whether it is
  tenant-scoped; DDL in specs drafted before ADR-07 omits the columns, and this
  bullet governs.
- **Events**: every task/event name lands in
  [../../reference/events.md](../../reference/events.md) as part of definition of
  done; every new domain module emits ≥ 1 bus event from day one (ADR-08). The
  `platform/events` fan-out helper (`Publish(ctx, name, payload)` + the
  event-name→consumer-task subscription table) was built by SPEC-01 P0.6 — a
  prerequisite of the first spec to land (SPEC-01 P0.6 in the build order;
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
  handler, `shared/openapi.yaml` and the frontend readers. *(Code follow-up
  for lists no spec owns: music `{imports}` and `{playlists}`, social
  `{connections}`, story `{chapters}` and people `{suggestions}` retrofit to
  `{items}` the same way.)* The ordering key
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
  the session zone (UTC). *(Code follow-up — no spec owns the account module,
  so it is tracked as the first item of the **Per-user timezone** cross-cutting
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
  still govern multi-currency/creator-economy money. Rationale in SPEC-03 §7.)*
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
  implements the `opsapi.ExportProvider` interface (SPEC-09 P1.7) from its own
  `api/` package, in the PR that creates the table — or its spec states the
  exclusion and the reason. The spec's §6 names the export format per table.
  (The interface itself is unbuilt until SPEC-09 P1.7 lands; until then, the
  spec records the intended format.)
- **Definition of done (every spec PR)** also includes a
  `docs/testing/TEST-CASES-SPEC-NN-<module>.md` with rows citing the real
  `_test.go` evidence, and the spec's rows in
  [TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md).

## Build state and what remains

ADR-10's spec-first CI gate is in force; the `ServerInterface` retrofit is
per-module-as-touched (ADR-10 action items) and gates nothing.

**As built.** Every spec from SPEC-01 to SPEC-09 landed on 2026-07-12, in
migration order `0008` (media) → `0009`/`0010` (notify) → `0011` (journal) →
`0012` (ops backups) → `0013` (playback progress) → `0014` (bank) → `0015`
(comic) → `0016` (people) → `0017` (stream projection)
(`git log --diff-filter=A --format='%h %ad' -- backend/db/migrations/`). SPEC-09
P0 (`0012`, `6160f8e`) therefore preceded the SPEC-03 ledger (`0014`,
`66c036f`). The stream projection (`0017`) landed *after* its producers,
contrary to SPEC-06 §1; any event published before it was never projected
(SPEC-06's P1.6 backfill, which only ever covered media assets, is retired).

**Sequencing rules that still bind** (for rebuilds and new producers):

1. **SPEC-01** (shared bottleneck; P0.6 fan-out is everyone's prerequisite).
2. **SPEC-04** (consumer for `media:asset_ready`; unblocks password reset).
3. **SPEC-05** + **SPEC-06 P0.1** (journal module + the `stream_items`
   projection and its `journal:stream_*` consumers; must precede every producer
   below, SPEC-06 §1).
4. **SPEC-02** and **SPEC-09 P0** (backups — hard gate immediately before
   SPEC-03).
5. **SPEC-03** (after SPEC-09 P0 is live; in parallel with SPEC-07, the
   burst-filler); **SPEC-09 P1** here or later.
6. **SPEC-08**, then **SPEC-06 P0.2–P0.4 + P1** (read API, home, rail).

**What remains open** (verify against the matrix before starting):

- SPEC-09 P1.7 owner takeout — unbuilt (no `ExportProvider` in code).
- SPEC-10 phases after phase 1 (debts) — build on SPEC-03 + SPEC-04.
- Every built spec still diverges from its shipped code in places: see the
  [Implementation gaps index](#implementation-gaps-index-2026-10-01) below.

## Implementation gaps index (2026-10-01)

Every built spec ends with a section **"Implementation gaps vs shipped code (as
of 2026-10-01)"**: one row per place where `main` @ `99b5a0b` still diverges
from the spec text, naming the shipped file and function, the change needed
(migration · backend · openapi · frontend · test) and the finding id. That
section is the **live list**: a row closes, and is deleted, in the PR that makes
the code match and regrades the spec's
[TRACEABILITY-MATRIX.md](../../reference/TRACEABILITY-MATRIX.md) rows. This
index only counts and routes; the detail lives in the spec. SPEC-11 is a
historical docs-only spec and has no gap section.

Row ranges use each section's own severity order (its intro paragraph names
it): **Sec** security · **Data** data loss or silently wrong / lost data ·
**Integ** integrity or a wrong answer · **AuthZ** · **Contract** API, OpenAPI,
Problem types · **UX** frontend behaviour · **Hyg** schema hygiene · **P1**
unbuilt P1 feature. Rows added by the 2026-10-01 owner decisions are appended
at the end of their table instead of renumbering the rows other documents
cite, so a few ranges below are split.

| Spec | Gap section | Rows | By severity (row numbers) | Most severe |
|------|-------------|-----:|---------------------------|-------------|
| [SPEC-01](SPEC-01-media-image-pipeline.md) | [§11](SPEC-01-media-image-pipeline.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 18 | Data 1–3 · Sec/Integ (`/original`) 4–6 · Integ 7–8 · AuthZ 9 · Contract/UX 10–13, 15–18 · P1 14 | DELETE's 500 rolls back the `deleting` tombstone after objects are purged (1); `/original` streams abandoned or purged uploads, sized from the client's claim (4–6) |
| [SPEC-02](SPEC-02-comic-vertical.md) | [§11](SPEC-02-comic-vertical.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 17 | Sec 1–5 · Integ 6–9 · Contract/UX 10–17 | `/api/v1/internal/*` public at the edge, secret compared with `!=` (1–3); publish never checks `comics:publish:own`, drafts leak as 403 (4–5) |
| [SPEC-03](SPEC-03-finance-ledger.md) | [§12](SPEC-03-finance-ledger.md#12-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 17 | Sec 1 · Data 2–5 · Contract 6–12, 17 · UX 13–16 | derived-balance queries (accounts, dashboard) not filtered by the caller (1); archived accounts still accept writes (2) |
| [SPEC-04](SPEC-04-notification-module.md) | [§11](SPEC-04-notification-module.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 22 | Sec 1–2 · Data 3–4 · Func/UX 5–12 · Contract 13–16, 22 · Hyg 17–20 · P1 21 | plaintext reset token in the Asynq payload (1); reset token consumed check-then-act, the user's other tokens not revoked (2) |
| [SPEC-05](SPEC-05-journal.md) | [§11](SPEC-05-journal.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 8 | Data 1 · UX 2–5 · Contract 6–8 | `journal:entry_created` published before the request transaction commits (1) |
| [SPEC-06](SPEC-06-life-stream-home.md) | [§11](SPEC-06-life-stream-home.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 16 | Data / stale 1–5 · Integ (order, render) 6–10 · UX (home, rail, `/weather` gate) 11–14 · Contract 15 · P1 16 | stream unique key lacks `user_id`, so a second user's playback is swallowed (1); deleted or edited birthdays leave stale cards (2) |
| [SPEC-07](SPEC-07-continue-rail.md) | [§11](SPEC-07-continue-rail.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 14 | Data 1–3 · Integ 4–7 · Contract/UX 8–13 · P1 14 | the `pagehide` save is a `sendBeacon` POST → 405, silently lost (1); completion latched even when Publish fails, so the event is lost (2) |
| [SPEC-08](SPEC-08-people-registry.md) | [§11](SPEC-08-people-registry.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 15 | Data (lost / duplicate events) 1–4 · Integ (dates) 5–7 · Contract/UX 8–12, 14–15 · P1 13 | every PATCH carrying a birthday resets notices → duplicate stream items (1); no revoke / delete events (2) |
| [SPEC-09](SPEC-09-platform-ops.md) | [§11](SPEC-09-platform-ops.md#11-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 11 | Sec 1–4 · Data 5–6 · Func 7–8 · Contract 9 · Hyg 10 · P1 11 | queue console writable by any `queues:read` holder, no CSRF guard (1); the restore drill can only reach the dev MinIO (2) |
| [SPEC-10](SPEC-10-ledger-expansion.md) | [§8](SPEC-10-ledger-expansion.md#8-implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 5 | Data 1 · Scheduling 2 · Contract 3–5 | opening a debt is three writes with no enclosing transaction (1) |
| [SPEC-12](SPEC-12-journal-attachments.md) | [section](SPEC-12-journal-attachments.md#implementation-gaps-vs-shipped-code-as-of-2026-10-01) | 2 | AuthZ 1 · UX 2 (both owned by other specs) | no `user` grant of `assets:write:own` and no enforcement on upload (1) |
| **Total** | | **145** | | |

**Cross-cutting gaps** — one change closes rows in several specs; land it as
one change (or one PR per module in a fixed order) and close every row it
names:

- **Envelopes retrofit to `{items}`** (Pagination convention): SPEC-01 §11
  row 12 · SPEC-02 §11 row 12 · SPEC-03 §12 row 9 · SPEC-08 §11 rows 9, 11 ·
  SPEC-10 §8 row 3; plus the unowned lists (music, social, story, people
  suggestions) the Pagination convention names. Each retrofit moves handler,
  `shared/openapi.yaml` and the frontend readers in one PR.
- **Per-user timezone** (Timezone convention, Decisions 2026-09-30 and
  2026-10-01 (f)): the account change comes first and has no spec row (no spec
  owns the account module), so it is listed here in full:
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

  Then its readers: SPEC-03 §12 rows 8, 16 · SPEC-05 §11 row 2 · SPEC-06 §11
  rows 3, 9, 16 · SPEC-08 §11 rows 5–6 · SPEC-10 §8 row 2.
- **`problems.ts` slugs** (Errors convention): SPEC-01 §11 row 12 · SPEC-02
  §11 row 10 · SPEC-03 §12 rows 2, 8 · SPEC-04 §11 row 14 · SPEC-05 §11
  row 6 · SPEC-06 §11 row 15 · SPEC-07 §11 row 13 · SPEC-08 §11 row 12 ·
  SPEC-10 §8 row 5; plus `account/invalid-timezone`.
- **OpenAPI `x-required-permission`** (AuthZ convention, OpenAPI encoding):
  SPEC-01 §11 row 17 · SPEC-02 §11 row 16 · SPEC-03 §12 row 12 · SPEC-04 §11
  row 16 · SPEC-05 §11 row 7 · SPEC-06 §11 row 15 · SPEC-09 §11 row 9 ·
  SPEC-10 §8 row 5. Best done as one retrofit together with the drift check
  the convention asks for.
- **`limit` clamps instead of resetting** (Pagination convention, Decision
  2026-10-01 (e)): SPEC-01 §11 row 18 · SPEC-02 §11 row 17 · SPEC-03 §12
  row 17 · SPEC-04 §11 row 22 · SPEC-05 §11 row 8 · SPEC-06 §11 row 8 ·
  SPEC-08 §11 row 15; plus the unowned movie, music (catalogue and imports),
  story and people-suggestions lists, which reset the same way. Each is one
  line (or a call to `platform/server.Limit`, which already clamps) plus a
  test; `/continue` already clamps.
- **F009 — `user` upload grant**: SPEC-01 §11 row 9 (owner) · SPEC-12 row 1.
  Seed the grant in the same migration that adds `RequirePermission` to the
  upload routes, or every `user` loses photo upload.
- Smaller shared items: `origin='import'` (F038/F012) SPEC-01 row 10 · SPEC-02
  row 7 · SPEC-04 row 8 · SPEC-12 row 2; media deep link (F018) SPEC-01 row 15
  · SPEC-04 row 9 · SPEC-06 row 7 · SPEC-07 row 9; Audio SPEC-01 row 16 ·
  SPEC-07 rows 4, 5, 8; keepalive `PUT` instead of `sendBeacon` (F001) SPEC-02
  row 11 · SPEC-07 row 1; `bulk` queue (F095) SPEC-02 row 8 · SPEC-04 row 19 ·
  SPEC-09 row 11; people retraction events (F017) SPEC-08 row 2 · SPEC-06
  row 2; optimistic placement (F015) SPEC-05 row 4 · SPEC-06 row 10; D-34
  matcher (F032) SPEC-06 row 14 (`/weather`; `/admin` and `/calendar` are
  unowned).

**Suggested build order for closing gaps** (a suggestion, not a gate):

1. **Security.** SPEC-02 §11 rows 1–3 (internal endpoints off the public edge,
   constant-time secret), SPEC-04 §11 rows 1–2 (no plaintext reset token;
   atomic consume + revoke-all), SPEC-09 §11 row 1 (queue console read/write
   split + CSRF), SPEC-01 §11 rows 4–6 and 1 (`/original` states and size; the
   delete commit), SPEC-03 §12 row 1 (caller filter on balance queries). Then
   the remaining Sec / AuthZ rows: SPEC-02 rows 4–5, SPEC-09 rows 2–4, F009.
2. **Data loss.** SPEC-07 rows 1–3, SPEC-05 row 1, SPEC-06 rows 1–2 together
   with SPEC-08 rows 1–4 (the retraction events and their stream consumers),
   SPEC-01 rows 2–3, SPEC-04 rows 3–4, SPEC-09 rows 5–6, SPEC-10 row 1,
   SPEC-03 rows 2–5, SPEC-02 row 6.
3. **Cross-cutting foundations.** Timezone (account change, then its readers),
   Envelopes (per module, with OpenAPI and `problems.ts` in the same PR), Audio
   (SPEC-01 row 16 before SPEC-07 rows 4, 5, 8).
4. **Remaining integrity, contract and UX rows**, per spec, in each section's
   order; the `x-required-permission` retrofit with its drift check.
5. **Unbuilt P1**: SPEC-01 row 14, SPEC-04 row 21, SPEC-06 row 16, SPEC-07
   row 14, SPEC-08 row 13, SPEC-09 row 11 (owner takeout, above).

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
  update in [feature-inventory.md](../feature-inventory.md); SPEC-03 P0.6,
  SPEC-05 P0.4, SPEC-06 P0.1 / P0.3 / P1.5, SPEC-08 P0.3 / P0.4, SPEC-10 §4a.
- **Audio opens the player** — audio is a playable kind: player page, plays
  from `/original`, saves progress, joins `/continue`. Detail: SPEC-07 P0.1,
  P0.2, P0.4; SPEC-01 §11 row 16; the D-20 update in
  [feature-inventory.md](../feature-inventory.md).

## Decisions recorded 2026-10-01

The owner settled the seven questions the 2026-10-01 gap verification left
open. The detail lives in the places named here; the gap rows cite them as
"Decision 2026-10-01".

- **(a) `GET /bank/report` is documented, not retired** — SPEC-03 P1.11
  (contract and ACs) and its §7 row; TC-BANK-185…190; the SPEC-03 P1.11 row
  of the matrix. No new gap row: its month handling is SPEC-03 §12 row 8, its
  OpenAPI annotation row 12.
- **(b) Debt movements use `wallet_id`** — defined in SPEC-10 §6 (the user's
  own non-debt account the money moves through). The old SPEC-10 §8 row 4
  (rename to `account_id`) is gone; row 4 is now the non-debt check `HEAD`
  lacks.
- **(c) `people/already-in-registry` is declared** — SPEC-08 P0.2 (409 on a
  second link of one account) and §7 Problem types; §11 row 12 now covers only
  its missing `problems.ts` entry, row 14 the `linked_user_id` shape rule;
  TC-PPL-021, TC-PPL-022.
- **(d) The weather widget is in scope** — SPEC-06 §3 (no longer a non-goal)
  and P0.4 (requirement and ACs); §11 row 14 is now the `/weather` matcher gap;
  TC-STREAM-073…076; brief 06 annotated (since folded into SPEC-06 and deleted).
- **(e) `limit` is lenient and clamps** — the **Pagination** convention above;
  every §7 that declares a limit; one gap row per service that resets instead
  (the `limit` cross-cutting gap below); the `limit` TCs; the D-29 update in
  [feature-inventory.md](../feature-inventory.md).
- **(f) `timezone_manual` and `account/invalid-timezone` are confirmed** — the
  **Timezone** convention above; the D-17 update in
  [feature-inventory.md](../feature-inventory.md);
  [frontend.md](../../architecture/frontend.md) §5.4;
  [security.md](../../architecture/security.md)'s route list. The code
  follow-up is the first item of the **Per-user timezone** cross-cutting gap
  below (no spec owns the account module).
- **(g) `{items}` for non-paginated lists is confirmed** — the **Pagination**
  convention above (with the rationale and what counts as a collection); the
  D-29 update in [feature-inventory.md](../feature-inventory.md). Every spec §7
  complies; the shipped divergences are the Envelopes rows indexed above.

## Review history

- **2026-07-11** — first spec-gap review and fix pass:
  `spec-gap-fix-worklog-2026-07-11.md` (deleted once the 2026-09-30 review
  superseded it — `git show 2cdda7e:docs/product/analysis/spec-gap-fix-worklog-2026-07-11.md`).
- **2026-09-30** — second review (184 confirmed findings), fixed in the specs
  by `1db1e32` (PR #17); the owner decisions on envelopes, timezone and audio
  applied and the matrix regraded by `f54389b`:
  [spec-gap-fix-worklog-2026-09-30.md](../analysis/spec-gap-fix-worklog-2026-09-30.md)
  (its last section before "Refuted" records the corrections found on
  2026-10-01).
- **2026-10-01** — every `[c]` finding re-verified against `99b5a0b` and
  folded into each spec's "Implementation gaps vs shipped code" section.
  **From now on that section, not a worklog, is the live list** of where the
  code diverges; the worklogs are the dated record. The same day the owner
  decided the seven open questions; they are applied and listed under
  "Decisions recorded 2026-10-01" (gap total 138 → 145).
