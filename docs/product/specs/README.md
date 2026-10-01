# Portal — Detailed Feature Specs (PRD level)

**Status:** current · **Last verified:** never

**Language policy:** English only, per owner decision 2026-07-07 (ADR-09; the
former vi mirror was deleted in `f11cf3f` — see
[ADR-11](../../adr/11-docs-canonicalisation.md)).

This folder holds the **detailed, implementation-ready specs** for the features
captured in [../briefs/](../briefs/). A brief answers *what and why in brief*;
a spec answers *exactly what to build, how to know it's done, and what to decide
before starting*. Most specs are promoted from a brief when it is queued for
build (the briefs README tracks the mapping); SPEC-04, 10, 11 and 12 have none,
and each header names its upstream.

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
they remain in [../briefs/00-life-os-pivot.md](../briefs/00-life-os-pivot.md)
(→ [ADR-08](../../adr/08-life-os-pivot.md)) and
[../briefs/04-deferred.md](../briefs/04-deferred.md).

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
  their declared defaults and maxima). **Every** cursor-paginated list
  endpoint responds `{items: [...], next_cursor: string}` with `next_cursor`
  absent (or null) on the last page, and every non-paginated list responds
  `{items: [...]}`. Extra top-level fields that are not the list itself are
  allowed alongside `items` (e.g. notifications' `unread_count`); a resource
  named list key (`{assets}`, `{comics}`, `{people}`, …) is not. *(Owner
  decision 2026-09-30:)* endpoints that shipped before this rule are
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
  boundaries. **Write path**: the frontend detects the device zone
  (`Intl.DateTimeFormat().resolvedOptions().timeZone`, i.e. the user's current
  location) after sign-in and saves it through the account API
  (`PATCH /api/v1/auth/me {timezone}`) when it differs from the stored value;
  settings offer a manual override (an IANA picker, saved as
  `PATCH /auth/me {timezone, timezone_manual: true}`); while
  `users.timezone_manual` is true the automatic save is skipped, and choosing
  "use my location" clears it. The API validates the name with
  `time.LoadLocation` (422 `account/invalid-timezone` otherwise). **Readers**:
  the frontend reads the zone from `GET /auth/me`; backend modules read it
  through `accountapi` (`UserSummary.Timezone`), never by querying `users`.
  **Sweeps**: a periodic task never uses one "today" for everyone — it runs
  often enough (hourly, D-17's per-TZ pattern) and evaluates each user's local
  date in that user's zone, relying on its dedup keys for exactly-once. SQL
  converts at the query layer (`occurred_at AT TIME ZONE $tz`), never through
  the session zone (UTC). *(Code follow-up: `0002_account_users` ships
  `timezone DEFAULT 'UTC'` — a new migration changes the default to
  `'Asia/Ho_Chi_Minh'`, rewrites the untouched `'UTC'` rows and adds
  `timezone_manual boolean NOT NULL DEFAULT false`; there is no
  `PATCH /auth/me` and no `account/invalid-timezone` in `problems.ts`, `CurrentUser` and `accountapi.UserSummary` carry no
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
