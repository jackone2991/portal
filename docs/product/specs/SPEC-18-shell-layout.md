# SPEC-18 — Shell Layout (data-driven navigation menu + dashboard widget placement)

**Status:** current, rev 2 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `layout` (`backend/internal/modules/layout/`) + frontend shell (`SidebarLeft`, `HomeView` rails, `/admin/layout` editor, `widget/registry.ts`) · **Depends on:** `account` — `accountapi.HasPermission` (per-caller filtering; implemented for this module, it had been a stub returning `false`), `RequireAuth`, and `RequirePermission` built by `cmd/api` from the RBAC engine; `platform/audit` (`D-25`); `platform/events` and SPEC-04 P1.5 (the `layout:changed` consumer, P1.4); the frontend widget registry
**Upstream:** as-built spec, written retroactively on 2026-10-01 from the shipped code (`0036_layout_core`, `internal/modules/layout`, `frontend/src/lib/layout.ts`, `AdminLayoutView.tsx`) and the decisions it implements — the root `CLAUDE.md` "Layout module — the shell is data now (migration 0036)", feature-inventory §9.11 Widgets (`Manage Widgets.html`, customisable rails), [ADR-08](../../adr/08-life-os-pivot.md) (the home is the life-stream surface with a facet widget rail) · **Refs:** [ADR-02](../../adr/02-rbac-model-reconciliation.md) (role hierarchy is canonical; the editor's `permission` field uses its grammar), `D-7` (RFC 7807 + i18n keys), `D-25` (audit), `D-29` (envelopes; composite reads keep named arrays), `D-32`/`D-33`/`D-34` (frontend), [ADR-07](../../adr/07-tenancy-rls-model.md) (why these tables are global)
**Downstream consumers:** every authenticated page (the left menu); the home dashboard rails ([SPEC-06](SPEC-06-life-stream-home.md) P0.4 — each widget it specifies is a row here plus a registry entry); any spec that adds a widget (its migration seeds a `layout_widgets` row); notify (`notify:on_layout_changed`, SPEC-04 P1.5 — planned)

---

## 1. Problem statement

The Olympus port shipped the app shell as code: `SidebarLeft` held an `ITEMS`
array and `HomeView` composed its two widget columns in JSX. Reordering a menu
entry, hiding a decorative template row ("Friend Groups", "Community Badges",
"Account Stats" link nowhere), or moving the birthdays card to the other rail
meant a rebuild and a redeploy. Worse, an admin-only entry ("Người dùng",
"Vai trò & quyền") could only be hidden *in the bundle*: every user downloads
the code that knows which admin screens exist, and a client-side check is the
only thing deciding who sees them.

`layout` moves the decision out of the bundle. The menu and the widget
placement become two small tables; the API serves each caller only the rows
they may see; an admin edits both from inside the shell. Applying the migration
changed nothing visible — the seed reproduces the hardcoded shell row for row.

## 2. Goals

1. The navigation menu and the dashboard widget placement are data, editable
   from the shell by an admin without a deploy.
2. Each caller receives only the visible rows whose permission they hold,
   filtered on the server, so an admin-only entry never reaches a non-admin.
3. A save is whole-set and atomic: the shell is never observed half-saved, and
   array order is the order shown.
4. The shell cannot be broken from inside the editor: the menu cannot be
   emptied, the editor's own link cannot be deleted, a link cannot leave the
   app, and a widget the bundle cannot render cannot be placed.
5. An API failure degrades the menu to a static copy of the unrestricted seed,
   never to an empty sidebar.

## 3. Non-goals

- **No per-user or per-tenant personalisation.** One layout per instance; a user
  cannot hide a widget for themselves (P2).
- **No widget catalogue in the database.** A widget is a React component; the
  table stores where it goes and whether it shows, never what it is. There is
  no "add widget" button and no API that creates or deletes a widget row (P0.5).
- **No page builder / CMS.** Menu rows are links (in-app paths) with a label and
  an icon; no nested menus, no external links, no arbitrary HTML.
- **No settings for the widgets themselves** (e.g. the weather location) — each
  widget owns its own configuration (SPEC-06 P0.4).
- **No life-stream events.** Reordering a menu is not a moment in anyone's day:
  `layout:changed` (P1.4) is an admin notice for the bell, not projected into
  the stream.

## 4. User stories

- As the operator, I open **Menu & Widget** (`/admin/layout`), delete "Community
  Badges" and "Account Stats", drag "Ledger" to the top, press **Lưu**, and every
  signed-in user's sidebar changes on their next layout fetch. *(primary)*
- As the operator, I add a row "Nhạc" → `/library/music` visible only to holders
  of `music:read`; users without it never receive the row.
- As the operator, I move the birthdays card from the right rail to the left and
  hide the "Trang" card.
- As a regular user, I never see "Người dùng" or "Vai trò & quyền" in my
  sidebar, and they are not in my `GET /layout` response.
- Edge: the API is down — my sidebar still shows the unrestricted seed menu.
- Edge: a stale editor tab saves a menu without the editor's own row — the row
  survives.

## 5. Requirements

Status codes, slugs and permission codes below are the contract. Where `HEAD`
diverges, the requirement carries *(code follow-up)* and §11 has the row.

### P0.1 — Module scaffold and placement

`internal/modules/layout/` with `module.go` (`New(Deps)`, `MountHTTP`),
`handler.go`, `service.go`, `types.go`, `query/layout.sql`, `repository/`
(sqlc + `adapter.go`); a `sqlc.yaml` block; migration `0036_layout_core`; a
`module-layout-isolation` depguard block; constructed and mounted in
`cmd/api/main.go` (`layout.New` + `layoutMod.MountHTTP(r)`). It owns no Asynq
task and is absent from `cmd/worker/main.go`. On `HEAD` it has **no `api/`
package** — nothing outside the module reads the layout; P1.4 adds one
(MODULES.md §8) holding only its event name and payload.

The tables are global (§6), so the routes take plain `RequireAuth`, not
`authTenant`: there is no tenant row to fence and no tenant transaction is
opened. The adapter receives the **pool**, because each whole-set save runs in
its own transaction. `Deps.Perms` is `accountMod.API()` — layout never imports
`account/rbac` (depguard).

### P0.2 — The caller's layout

`GET /api/v1/layout` — **authenticated, not permission-gated**: the shell cannot
render without it, and a gate would only mean nobody has a sidebar. Response
`LayoutConfig {menu: MenuItem[], widgets: Widget[]}` (§7), filtered per caller
on the server (`Service.ForCaller`):

- a row is returned iff `visible = true` **and** (`permission` is NULL **or**
  `accountapi.HasPermission(ctx, permission)` is true);
- `HasPermission` resolves the principal from the request context and asks the
  RBAC engine (`Authorize`, wildcard grammar, role hierarchy, cache keyed by
  `token_version`); it is **fail-closed** on every failure path — no identity, a
  code `rbac.Parse` rejects, a loader error;
- with no checker wired (`Deps.Perms == nil`), gated rows are hidden — never
  shown;
- menu rows in `position, id` order; widgets in `slot, position, id` order.

This is a composite read (one object holding two named arrays), not a
collection, so it keeps `menu`/`widgets` rather than `{items}` (specs README
Pagination: "a composite read … keeps its arrays as named fields").

**Acceptance criteria.**
- Given the seed, when a `user` fetches `/layout`, then the menu has the 12
  ungated rows and none of `admin-users`, `admin-roles`, `admin-layout`
  (TC-LAY-001).
- Given the seed, when an `admin` fetches it, then all 15 rows are present
  (`admin` holds `users:read:any`, `rbac:role:read`, `system:settings:write` in
  0003) (TC-LAY-002).
- Given a row with `visible = false`, then nobody receives it, including a `*`
  holder (TC-LAY-003; `TestForCallerDropsRowsTheCallerCannotSee`).
- Given no checker, then gated rows are hidden and ungated rows shown
  (TC-LAY-004; `TestForCallerWithNoPermissionCheckerHidesGatedRows`).
- Given no session, then 401 `about:blank` (TC-LAY-005).
- Given a user lacking `bank-accounts:read:own`, then the `finance` widget is
  absent (TC-LAY-006).

### P0.3 — The whole layout (admin)

`GET /api/v1/admin/layout` — permission `system:settings:write` (a grandfathered
admin-plane literal code from 0003, held by `admin` and, through `*`, by
`superadmin`). Returns every menu row and widget, hidden and gated rows
included, same shape and order as P0.2. 403 `about:blank` without the
permission.

**Acceptance criteria.**
- Given a hidden row and a gated row, then both are returned (TC-LAY-010;
  `TestFullKeepsHiddenAndGatedRows`).
- Given a `user`, then 403 (TC-LAY-011).

### P0.4 — Save the menu (whole set)

`PUT /api/v1/admin/layout/menu {items: [{key, label, icon, href?, permission?, visible}]}`
— permission `system:settings:write`. The array **is** the menu, in render
order. Validation (all in `Service.SaveMenu`, first failure wins, `detail` names
the offending row's key so an admin editing fifteen rows knows which one):

| Rule | Failure |
|---|---|
| 1 ≤ items ≤ 80 ("the menu cannot be empty") | 422 `layout/validation` |
| `key` (trimmed): 1–40 chars of `[a-z0-9_-]`, unique in the payload | 422 `layout/validation` |
| `label` (trimmed): 1–60 **characters** | 422 `layout/validation` |
| `icon` (trimmed): non-empty (a sprite name; not checked against the sprite) | 422 `layout/validation` |
| `href` (trimmed): empty (= a row that navigates nowhere) **or** an in-app path: starts with exactly one `/`, contains no `\`, no ASCII control character or whitespace, and at most 300 bytes | 422 `layout/validation` |
| `permission` (trimmed): empty (= every signed-in user) **or** a code `rbac.Parse` accepts, checked through `accountapi` | 422 `layout/validation` |
| `visible`: a boolean, required | 422 `layout/validation` |

*(Code follow-up: `HEAD` answers every validation failure with **400**, not
422 — §11 row 7; counts label length in **bytes** — §11 row 8; accepts `\` and
embedded control characters in `href` — §11 row 1; never checks `permission` —
§11 row 2; and treats an omitted `visible` as `false` — §11 row 7.)*

**Why the `href` rule is strict.** Every user is shown the menu, so an absolute
or protocol-relative URL would turn the app's own navigation into an
open-redirect surface. Browsers normalise `\` to `/` and strip tab/newline
inside URLs, so `/\evil.example` and `/<TAB>/evil.example` resolve to
`//evil.example` — a leading-`/` check alone is not enough.

**Apply** (`repository/adapter.go` `SaveMenu`, one transaction): `position` is
renumbered from array order (`(i + 1) * 10`) — a client-sent position is never
read, so two rows can never claim one slot; each row is upserted on `key`
(insert = add, conflict = edit, `updated_at = now()`); then every row whose key
is absent **and** `is_system = false` is deleted. An omitted `is_system` row
survives and is placed **after** the submitted rows (renumbered to follow the
last one), so a stale tab can neither delete the editor's own link nor leave it
tied with a submitted row. *(Code follow-up: `HEAD` keeps the omitted system
row's stored position, which can equal a renumbered one — §11 row 3.)* The
response is **200** with the full layout re-read (P0.3 shape).

Exactly one seeded row is `is_system`: `admin-layout`, the link to this editor.
It can be renamed, reordered, hidden and re-gated, but not deleted ("delete the
door you are standing in" is refused rather than explained afterwards). Every
other row, seeded or added, is deletable — removing the template's decorative
rows is the most likely reason anyone opens the editor. The guard against
wiping the menu is the non-empty rule, not a flag on every row.

**Acceptance criteria.**
- Given rows submitted as `[c, a, b]`, then the stored positions are 10, 20, 30
  in that order, whatever positions the client sent (TC-LAY-020;
  `TestSaveMenuRenumbersFromArrayOrder`).
- Given `href` ∈ {`https://evil.test/x`, `//evil.test`, `javascript:alert(1)`,
  `relative/path`, `/\evil.test`, `/` + TAB + `/evil.test`}, then 422 and nothing
  is written (TC-LAY-021; partly `TestSaveMenuRejectsOffSiteLinks`).
- Given `href = /library/music`, then it is stored unchanged (TC-LAY-022;
  `TestSaveMenuAcceptsInAppPaths`).
- Given an empty array, or 81 rows, then 422 (TC-LAY-023;
  `TestSaveMenuRejectsAnEmptyMenu`).
- Given a duplicate key or a key with an uppercase letter, then 422 naming it
  (TC-LAY-024; `TestSaveMenuRejectsDuplicateKeys`, `TestSaveMenuRejectsMalformedKeys`,
  `TestSaveMenuErrorNamesTheOffendingRow`).
- Given a 45-character Vietnamese label, then it is accepted; given 61
  characters, then 422 (TC-LAY-025).
- Given `permission = "users:read:any:extra"` (4 segments) or `"foo"`, then 422
  (TC-LAY-026).
- Given a save omitting `admin-layout` and `badges`, then `badges` is deleted,
  `admin-layout` survives and sorts after every submitted row (TC-LAY-027).
- Given a failure while upserting the third row, then no row of the save is
  applied (TC-LAY-028).

### P0.5 — Save widget placement (registry-backed)

`PUT /api/v1/admin/layout/widgets {widgets: [{key, label, slot, permission?, visible}]}`
— permission `system:settings:write`. Sets slot, order, label, permission and
visibility of the widgets that exist; **never inserts or deletes** a widget
(`UpdateWidget` is a keyed `UPDATE`). Rules (`Service.SaveWidgets`):

| Rule | Failure |
|---|---|
| every `key` is an existing `layout_widgets.key` (trimmed) | 422 `layout/unknown-widget` |
| each key at most once | 422 `layout/validation` |
| the payload names **every** existing widget exactly once (it is the whole placement) | 422 `layout/validation` |
| `slot` ∈ `left` \| `right` | 422 `layout/validation` |
| `label` 1–60 characters; `permission` and `visible` as in P0.4 | 422 `layout/validation` |

`position` restarts per slot from array order (10, 20, … within each slot), so a
card moved between rails does not carry its old number. Applied in one
transaction; **200** with the full layout re-read. *(Code follow-up: `HEAD`
answers 400 for both slugs — §11 row 7 — and accepts a partial payload, leaving
omitted widgets at stale positions that can tie with renumbered ones — §11
row 4.)*

**The registry contract.** A widget row's `key` must match a key in
`frontend/src/templates/v1/components/widget/registry.ts` (`WIDGET_REGISTRY`);
the database cannot conjure a component. The API enforces this indirectly — it
refuses any key not already seeded — so the invariant that makes it true is
**seed ⇔ registry parity**: the set of keys seeded by `*_layout_*` migrations
equals the registry's key set. Adding a widget is therefore one PR that adds the
component, its registry entry and a layout-owned migration
`000N_layout_<widget>` inserting its row (SPEC-06 §11 row 13's `personal-info`
is the next instance). Parity is checked mechanically. *(Code follow-up: no
check exists — §11 row 9.)*

**Acceptance criteria.**
- Given an unknown key `"nope"`, then 422 `layout/unknown-widget` naming it and
  nothing written (TC-LAY-030; `TestSaveWidgetsRejectsAnUnknownKey`).
- Given widgets `[a(left), b(right), c(left)]`, then positions are a=10, c=20
  (left) and b=10 (right) (TC-LAY-031; `TestSaveWidgetsNumbersEachSlotIndependently`).
- Given `slot = "top"`, then 422 `layout/validation` (TC-LAY-032;
  `TestSaveWidgetsRejectsAnUnknownSlot`).
- Given a payload missing one seeded widget, then 422 `layout/validation`
  (TC-LAY-033).
- Given the registry and the seeded keys, then the two sets are equal
  (TC-LAY-034).

### P0.6 — Audit

A successful save writes `layout.menu.saved` (metadata `{items: n}`) or
`layout.widgets.saved` (`{widgets: n}`) through `platform/audit`, target kind
`layout`, after the transaction commits. Best-effort per the audit rule: a
failed audit write never fails the save. P1.4's `layout:changed` is published
at the same point, beside the audit row, not instead of it.

### P0.7 — Frontend

- **Data** (`frontend/src/lib/layout.ts`): `useLayout()` — TanStack query
  `["layout"]`, `staleTime` 5 min, `retry: false`; `getAdminLayout`, `saveMenu`,
  `saveWidgets`; types mirror the handler JSON.
- **Left menu** (`templates/v1/components/menu/SidebarLeft.tsx`): renders
  `layout.menu`; a row with an `href` is a `next/link`, a row without one is an
  inert button. While the query is pending, or if it fails, it renders
  `FALLBACK` — a static copy of the **unrestricted** seed rows only (no gated
  row, so it never flashes admin links). A successful response with an empty
  `menu` (everything hidden from this caller) renders an empty menu, not the
  fallback. *(Code follow-up: `HEAD` falls back whenever `menu` is empty — §11
  row 5.)*
- **Home rails** (`templates/v1/views/home/HomeView.tsx` `WidgetRail`): each
  rail renders the caller's widgets for its slot through `widgetComponent(key)`;
  an unknown key is skipped (an old bundle meeting a newly seeded widget shows one
  card less, not a crash); nothing renders until the layout resolves (a blank
  rail for one beat beats a reshuffle). Widgets have no static fallback.
- **Editor** (`/admin/layout`, `templates/v1/views/admin/AdminLayoutView.tsx`,
  resolved through `activeTemplate().views.adminLayout`): two tabs, Menu and
  Widget; edits are staged and saved as a whole set ("Lưu"), with a dirty marker
  and a reset; rows reorder by move-up/move-down; "Thêm mục" adds a menu row;
  the delete control is absent on `is_system` rows; there is **no "add widget"**
  control; after a save the response replaces the admin query and `["layout"]`
  is invalidated so the editor's own sidebar updates at once. Errors render
  `problemDisplayMessage`, which prefers the server `detail` (it names the
  row); `layout/validation` and `layout/unknown-widget` are registered in
  `problems.ts`.
- **Route gate**: `/admin/:path*` is in `config.matcher` of
  `frontend/src/middleware.ts` (D-34 edge gate) so a signed-out visitor is
  redirected to `/login`. *(Code follow-up: absent — §11 row 10, shared with
  backlog item 15 / F032.)*

**Acceptance criteria.**
- Given the API is down, then the sidebar shows the 12 unrestricted seed rows
  and no admin row (TC-LAY-040).
- Given an admin hides every row, then users see an empty menu, not the
  fallback's decorative rows (TC-LAY-041).
- Given a widget key absent from the registry in the response, then the rail
  renders the other cards (TC-LAY-042).
- Given the editor, then `admin-layout` has no delete control and the Widget tab
  has no add control (TC-LAY-043).
- Given a save, then the editor's own sidebar reflects it without a reload
  (TC-LAY-044).
- Given a signed-out visitor on `/admin/layout`, then a redirect to `/login`
  (TC-LAY-045).

### P0.8 — Seed equals the former hardcoded shell

`0036_layout_core` seeds 15 menu rows (12 ungated, three admin rows gated by
`users:read:any`, `rbac:role:read`, `system:settings:write`; `admin-layout`
`is_system`) and 9 widgets (left: `finance` `bank-accounts:read:own`,
`continue`, `music` `music:read`, `weather`, `calendar`, `pages`; right:
`birthdays` `people:read:own`, `friend-suggestions`, `activity-feed`), the
widget permissions mirroring each widget's own query so a user who cannot read
the ledger is not served an empty finance card. Applying it changed nothing
visible; `SidebarLeft`'s `FALLBACK` equals the 12 ungated rows.

### P1 — nice to have

- **P1.1 Optimistic concurrency.** Both saves carry the `version` (max
  `updated_at`, or a counter) the editor loaded; a mismatch is **409
  `layout/stale`** and the editor reloads. Today two admins (or two tabs) save
  last-write-wins, and the later whole-set save silently deletes rows the other
  added. Gated on §10 Q3.
- **P1.2 Audit diff.** Audit metadata records the keys added, removed and
  reordered, not only the count.
- **P1.3 Icon picker.** The editor offers the sprite's icon names instead of a
  free-text field; the API accepts only known names.
- **P1.4 `layout:changed` event** *(Decision 2026-10-01b (D3); unbuilt — §11
  row 14)*. Layout is not exempt from ADR-08's "≥ 1 bus event": each successful
  save publishes **`layout:changed`** `{event_id, occurred_at, part: "menu" |
  "widgets", actor_id, actor_name, count}` through `platform/events` after its
  transaction commits (the point P0.6 audits at); a failed save publishes
  nothing, and a publish error is logged and never fails the save. `count` is
  the number of rows saved (the audit metadata's `items` / `widgets`);
  `actor_name` comes from the caller's token. The name and the payload struct
  live in a new `layout/api` package; `Deps.Events` is the publisher, and
  `cmd/api` registers `Subscribe("layout:changed",
  "notify:on_layout_changed")`. Notify tells every superadmin except the actor
  (SPEC-04 P1.5, type `layout.changed`, link `/admin/layout`).
  **One event, not `layout:menu_saved` + `layout:widgets_saved`:** the two
  saves differ only in which half of one configuration changed, they share
  the permission, the editor and the audience, and the only consumer renders
  both the same way; one name means one consumer task and one notify type,
  and a future consumer that cares about one half filters on `part`. *AC:*
  given a menu save, then exactly one `layout:changed` with `part: "menu"`,
  the saver as `actor_id` and `count` = the submitted rows is enqueued after
  commit *(TC-LAY-050)*; given a 422 or a rolled-back save, then none
  *(TC-LAY-051)*; given a widget save, then `part: "widgets"`
  *(TC-LAY-052)*.

### P2 — future considerations (design for, don't build)

- **Per-tenant layouts** when tenant `kind: household` ships (`D-24`): a
  nullable `tenant_id` with instance rows as `tenant_id IS NULL` defaults (the
  `bank_categories` shared-seed precedent), resolved most-specific-first.
- **Per-user hide/collapse** of widgets, as a user preference layered on top of
  the instance placement (not a copy of it).
- **A read API in `layout/api`** (the package P1.4 creates for its event) once
  another module (e.g. a future onboarding checklist) needs to read the menu.

## 6. Data model — migration `0036_layout_core`

**Tenancy.** Both tables are **global and carry no `tenant_id` and no RLS**:
there is one shell per instance, the rows are configuration rather than
user-authored data, every signed-in user may read them (P0.2 filters per
permission in the service), and writes are gated by `system:settings:write`.
Fencing them per tenant would give each personal tenant its own empty menu.
The specs README Tenancy bullet lists the exempt global tables and should name
these two (docs follow-up). `portal_app` reaches them through `0019`'s
`ALTER DEFAULT PRIVILEGES`. **Takeout:** excluded — no user-authored or
user-history data.

```sql
CREATE TABLE layout_menu_items (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key         TEXT UNIQUE NOT NULL,   -- stable handle; survives a rename
    label       TEXT NOT NULL,
    icon        TEXT NOT NULL,          -- sprite name (components/ui/Icon)
    href        TEXT,                   -- NULL = renders, navigates nowhere
    permission  TEXT,                   -- code required to SEE the row; NULL = everyone signed in
    position    INTEGER NOT NULL,
    visible     BOOLEAN NOT NULL DEFAULT true,
    is_system   BOOLEAN NOT NULL DEFAULT false,  -- undeletable; only 'admin-layout'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX layout_menu_items_position_idx ON layout_menu_items (position, id);

CREATE TABLE layout_widgets (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key         TEXT UNIQUE NOT NULL,   -- MUST match WIDGET_REGISTRY
    label       TEXT NOT NULL,          -- the editor's name for it
    slot        TEXT NOT NULL,
    permission  TEXT,
    position    INTEGER NOT NULL,
    visible     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT layout_widgets_slot_check CHECK (slot IN ('left', 'right'))
);
CREATE INDEX layout_widgets_slot_position_idx ON layout_widgets (slot, position, id);
```

Notes:
- Widgets have no `is_system` column: every widget is effectively a system row
  (P0.5 never deletes one).
- The `href` and `permission` rules of P0.4 are app-layer; the columns carry no
  CHECK. A defence-in-depth CHECK (`href IS NULL OR (href ~ '^/[^/\\]' AND href
  !~ '[[:cntrl:][:space:]\\]')` or equivalent) may land with §11 row 1, `NOT
  VALID` then `VALIDATE`.
- No permissions are seeded: the module reuses `system:settings:write` (0003)
  and the domain codes its rows reference (0003, 0014, 0016, 0022).
- `updated_at` is set explicitly by `UpsertMenuItem` and `UpdateWidget`.
- The down migration drops both tables; the frontend's `FALLBACK` keeps the
  menu usable (widgets disappear).

## 7. API summary

All under `/api/v1`. Authenticated through `RequireAuth` (401 `about:blank`
without a session); admin routes additionally `RequirePermission`
(403 `about:blank`). Each authenticated operation declares
`security: [{bearerAuth: []}]` and the admin ones `x-required-permission:
system:settings:write` per the specs README AuthZ **OpenAPI encoding**.
*(Code follow-up: the four layout operations declare no `security` at all —
§11 row 6.)*

| Method | Path | Permission | Request | Response | Errors |
|---|---|---|---|---|---|
| GET | `/layout` | authenticated | — | 200 `LayoutConfig` filtered per caller (P0.2) | 401 |
| GET | `/admin/layout` | `system:settings:write` | — | 200 `LayoutConfig`, everything | 401, 403 |
| PUT | `/admin/layout/menu` | `system:settings:write` | `{items: MenuSaveItem[]}` (1–80) | 200 `LayoutConfig` (re-read, everything) | 400 `about:blank` (non-JSON); 422 `layout/validation`; 401, 403 |
| PUT | `/admin/layout/widgets` | `system:settings:write` | `{widgets: WidgetSaveItem[]}` (every widget once) | 200 `LayoutConfig` (re-read, everything) | 400 `about:blank` (non-JSON); 422 `layout/unknown-widget`; 422 `layout/validation`; 401, 403 |

**Shapes.**
`LayoutConfig = {menu: MenuItem[], widgets: Widget[]}` — a composite read, so
named arrays, not `{items}`.
`MenuItem = {id, key, label, icon, href|null, permission|null, position, visible, is_system}`.
`Widget = {id, key, label, slot: 'left'|'right', permission|null, position, visible}`.
`MenuSaveItem = {key, label, icon, href?, permission?, visible}` (empty string =
none); `WidgetSaveItem = {key, label, slot, permission?, visible}`. The two
request bodies name their arrays `items` and `widgets` respectively; request
bodies are not collection responses and keep their shipped keys.

**Pagination.** None: no list endpoint (the menu is capped at 80 rows by P0.4,
the widget set by the registry).

**Problem types:** `layout/validation` (422) and `layout/unknown-widget` (422),
both registered in `frontend/src/lib/problems.ts`; P1.1 adds `layout/stale`
(409). The handler builds them with `server.ProblemType("layout",
"validation" | "unknown_widget")`, which normalises `_` to `-`.

## 8. Events

| Name | Kind | Payload | Emitted by | Consumer |
|---|---|---|---|---|
| `layout:changed` | event — **planned** (P1.4) | `{event_id, occurred_at, part, actor_id, actor_name, count}` (`layoutapi.ChangedEvent`) | `Handler.SaveMenu` / `SaveWidgets`, after commit, in `cmd/api` | `notify:on_layout_changed` (SPEC-04 P1.5): a bell entry for every superadmin except the saver |

Decision 2026-10-01b (D3) settled ADR-08's "every new domain module must emit
at least one bus event" for this module: no exemption. `HEAD` publishes nothing
and consumes nothing (§11 row 14); [events.md](../../reference/events.md)
carries the event as planned. The module consumes no event, and the stream does
not project this one.

## 9. Success metrics (n=1 honest)

- Leading: the operator removes or relabels the template's decorative rows
  ("Friend Groups", "Community Badges", "Account Stats", the "Commic" typo)
  within the first week of the editor existing — the reason the module was built.
- Leading: zero support-style surprises — no non-admin ever receives a gated
  row (auditable: `GET /layout` as a `user` vs the seed; TC-LAY-001).
- Lagging: no shell change since `0036` needed a frontend deploy, except adding a
  new widget component (which by design needs one).
- Honest caveat: at n=1 the "admin" and the "user" are the same person; the
  per-caller filter earns its keep only once a second account exists.

## 10. Open questions

Q1 (the ADR-08 event rule) was decided on 2026-10-01 — Decision 2026-10-01b
(D3), now P1.4 and §8; the remaining questions keep their numbers so citations
hold.

- **Q2 (owner, non-blocking) — Per-tenant shells.** When households arrive, does
  a household get its own menu (P2), or does the instance layout stay global?
  Decides whether `tenant_id` lands now as nullable (cheap) or later.
- **Q3 (owner, non-blocking) — Concurrent editors.** Is P1.1's `layout/stale`
  worth building for a one-operator instance, or is last-write-wins accepted?

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (`git log 99b5a0b..HEAD -- backend frontend
shared` is empty: the docs commits on top of it change no code). The spec text
above is the target; this section lists every place the shipped code diverges
from it. Rows are ordered by severity: security first, then integrity (wrong
rows shown or lost), then UX, contract, tests and hygiene, then unbuilt P1;
row 14 (P1, added by Decision 2026-10-01b) is appended after row 13. A
row closes when the code matches the requirement it cites and a
TRACEABILITY-MATRIX row for SPEC-18 is graded on a named test. The module lives
in `backend/internal/modules/layout/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.4 `href` in-app only | Reject `\`, ASCII control characters and whitespace anywhere in `href`, in addition to the leading-single-`/` rule. | `service.go` `SaveMenu` checks only `HasPrefix(href, "/") && !HasPrefix(href, "//")` after `TrimSpace`. `/\evil.example` and `/` + TAB + `/evil.example` pass, are stored, and `SidebarLeft.tsx` `Row` renders them as `next/link` hrefs that browsers resolve to `//evil.example` — the open redirect the rule exists to prevent, shown to every user. Requires `system:settings:write` to plant, so it is defence in depth, but the guard is the module's stated purpose. | **backend:** reject any `\`, any rune `< 0x20` or `0x7f`, and any whitespace; then require `href[1] != '/'`. **migration (optional):** a CHECK as in §6. **test:** extend `TestSaveMenuRejectsOffSiteLinks` with the two bypasses (TC-LAY-021). | Found while writing SPEC-18, 2026-10-01 |
| 2 | P0.4/P0.5 `permission` validated | A non-empty `permission` must be a code `rbac.Parse` accepts (checked through `accountapi`); otherwise 422. | `service.go` `SaveMenu`/`SaveWidgets` only trim `permission`. A malformed code (`foo`, a 4-segment code) is stored; `accountapi.HasPermission` is fail-closed on a parse error **even for a `*` holder**, so the row disappears from every caller's `GET /layout`, including the admin who saved it, while still showing in the editor. A well-formed code nobody holds hides the row from everyone but `*`. | **backend:** add a validator to `accountapi` (e.g. `ValidPermissionCode(code) bool`, wrapping `rbac.Parse`) and reject in both saves; optionally also warn when the code is not in the `permissions` catalogue. **test:** TC-LAY-026. | Found while writing SPEC-18, 2026-10-01 |
| 3 | P0.4 omitted system row placement | An omitted `is_system` row survives and is renumbered after the submitted rows. | `query/layout.sql` `DeleteMenuItemsExcept` keeps it (correct) but nothing renumbers it, so it keeps its stored position — e.g. `admin-layout` at 150 survives a 15-row save that renumbers another row to 150, and the order between them falls to `id`. | **backend:** in `adapter.go` `SaveMenu`, after the delete, `UPDATE layout_menu_items SET position = <next>, updated_at = now() WHERE is_system AND NOT (key = ANY($keys))` (new query). **test:** adapter/integration test TC-LAY-027. | Found while writing SPEC-18, 2026-10-01 |
| 4 | P0.5 whole placement | The widget payload names every existing widget exactly once; otherwise 422 `layout/validation`. | `service.go` `SaveWidgets` accepts any subset; `adapter.go` `SaveWidgets` updates only the submitted keys, so omitted widgets keep their old slot and position and can tie with the renumbered ones (order then by `id`). The OpenAPI description already says "Every widget". | **backend:** after the per-key checks, require `len(seen) == len(known)`. **test:** TC-LAY-033. | Found while writing SPEC-18, 2026-10-01 |
| 5 | P0.7 fallback only on no data | `FALLBACK` renders while pending or on error; a successful empty `menu` renders empty. | `SidebarLeft.tsx` `toEntries` returns `FALLBACK` whenever `items?.length` is 0, so if an admin hides every row (allowed: P0.4 counts submitted rows, not visible ones) every user sees the 12 seed rows — including the decorative ones the admin hid. | **frontend:** pass `layout === undefined` (or `isPending || isError`) to choose the fallback, not the array length. **test:** TC-LAY-041 (vitest on `toEntries` once extracted to `lib/layout.ts`). | Found while writing SPEC-18, 2026-10-01 |
| 6 | §7 OpenAPI security | Each of the four operations declares `security: [{bearerAuth: []}]`, the admin ones `x-required-permission: system:settings:write`. | `shared/openapi.yaml` `getMyLayout`, `adminGetLayout`, `adminSaveLayoutMenu`, `adminSaveLayoutWidgets` declare no `security` key (the file has no global `security`), so the contract reads them as public. | **openapi:** add both annotations to each operation; `make openapi`; commit the regenerated files. **test:** none beyond the CI `openapi` drift gate (the annotation drift check is the README cross-cutting item). | Specs README AuthZ convention (OpenAPI encoding) |
| 7 | P0.4/P0.5 status and required fields | Validation failures are 422 (`layout/validation`, `layout/unknown-widget`); an omitted `visible` is a validation failure, not `false`. | `handler.go` `writeSaveError` answers **400** for both slugs; `shared/openapi.yaml` declares them under `"400"`. The handler's anonymous body structs decode an omitted `visible` as `false`, silently hiding the row, although OpenAPI marks `visible` required. | **backend:** 422 in `writeSaveError`; decode `visible` as `*bool` and refuse nil. **openapi:** move both slugs to `"422"`. **frontend:** none (`problemDisplayMessage` keys on the slug). **test:** an HTTP test asserting 422 + slug (TC-LAY-023, TC-LAY-030). | Specs README Pagination/Errors convention (`<module>/validation` is 422) |
| 8 | P0.4/P0.5 label length | Labels are 1–60 **characters** (OpenAPI `maxLength: 60`). | `service.go` compares `len(it.Label)` / `len(w.Label)` — bytes. Vietnamese letters with diacritics take 2–3 bytes each, so a Vietnamese label well under 60 characters can exceed 60 bytes and be refused as "needs a label of 1-60 characters". | **backend:** `utf8.RuneCountInString`. **test:** TC-LAY-025. | Found while writing SPEC-18, 2026-10-01 |
| 9 | P0.5 seed ⇔ registry parity | A mechanical check that the keys seeded by `*_layout_*` migrations equal `WIDGET_REGISTRY`'s keys. | Nothing checks it. `registry.ts` and the `0036` seed agree today (9 keys each), but a widget added to one side only either never renders (seeded, not registered — skipped silently) or can never be placed (registered, not seeded). | **test:** a check in CI (e.g. a script in the `link-check` job, or a vitest that reads the migrations) comparing the two key sets (TC-LAY-034). | Found while writing SPEC-18, 2026-10-01 |
| 10 | P0.7 route gate | `/admin/:path*` is in `config.matcher`. | `frontend/src/middleware.ts` `config.matcher` = `["/", "/login", "/register", "/upload", "/library/:path*", "/bank/:path*", "/people/:path*"]`; `app/(app)/admin/layout/page.tsx` (and `/admin/users`, `/admin/roles`) render without the D-34 edge gate — the API still refuses, so this is a UX gap (a signed-out visitor gets the editor's error state, not `/login`). | **frontend:** add `'/admin/:path*'` together with the other F032 routes (`/weather`, `/calendar`). **test:** TC-LAY-045. | F032; backlog item 15 (extends it to `/admin`) |
| 11 | P0.2–P0.5 HTTP and adapter tests | Status codes over the real router; the transactional save against a database. | `service_test.go` tests the service with a fake repository only. Nothing asserts 401 on `/layout`, 403 on `/admin/*`, the response shape, that `DeleteMenuItemsExcept` spares `is_system` rows, or that a failing upsert rolls back the whole save. | **test:** `http_test.go` over `MountHTTP` with `servertest` (TC-LAY-005, 011, 023, 030); an integration test on the RLS harness for the adapter (TC-LAY-027, 028). | Found while writing SPEC-18, 2026-10-01 |
| 12 | Hygiene — stale statements in code and contract | Comments and descriptions match the code: migration `0036`; one `is_system` row; a `layout/api` package only once P1.4 adds it. | `backend/.golangci.yml` (layout block) and `backend/sqlc.yaml` (layout block) say "(0035)"; `HomeView.tsx` `WidgetRail` doc says "migration 0035"; `types.go` package doc says "Other modules import only layout/api" (there is none, `module.go` says so); `shared/openapi.yaml` `LayoutMenuItem.is_system` and `lib/layout.ts` `MenuItem.is_system` say "Seeded rows … never deleted/deletable", but only `admin-layout` is system; the seeded label and `FALLBACK` read "Commic". | **backend:** fix the four comments. **openapi:** "The editor's own link (`admin-layout`); renameable, reorderable, hideable, never deleted." **frontend:** same for the TS doc; "Comic" in `FALLBACK`. **migration:** none (the label is data; the operator renames it, or a `000N_layout_*` UPDATE). | Found while writing SPEC-18, 2026-10-01 |
| 13 | P1.1–P1.3 | Optimistic concurrency (`layout/stale`), audit diff, icon picker. | Not built: both saves are last-write-wins; audit metadata is a count; `icon` is free text. | **backend · openapi · frontend · test:** as P1.1–P1.3. | This spec (P1) |
| 14 | P1.4 `layout:changed` | Each successful save publishes `layout:changed {event_id, occurred_at, part, actor_id, actor_name, count}` after commit; `layout/api` holds the name and payload; `cmd/api` subscribes `notify:on_layout_changed`. | Not built: `module.go` `Deps` has no publisher and says there is no `api/` package; `handler.go` `SaveMenu` / `SaveWidgets` write the audit row only; `backend/cmd/api/main.go` subscribes no `layout:*` name. | **backend:** `layout/api` (constant + `ChangedEvent`); `Deps.Events`; publish beside the audit write in both handlers; the `Subscribe` edge in `cmd/api` (the consumer is SPEC-04 §11 row 23); a depguard allowance for `layout/api` if the isolation block needs one. **docs:** events.md row planned → live in the same PR. **test:** TC-LAY-050…052. | Decision 2026-10-01b (D3) |

**Already matching on HEAD.**
- `0036_layout_core` ships both tables, the slot CHECK, both order indexes, and
  the 15 + 9 seed rows of P0.8 with `admin-layout` as the only `is_system` row;
  the seed equals `SidebarLeft`'s `FALLBACK` for the 12 ungated rows.
- `GET /layout` is behind `RequireAuth` only and filters server-side on
  `visible` and `accountapi.HasPermission`, which is implemented (engine-backed,
  fail-closed) — the stub that returned `false` is gone; with no checker, gated
  rows are hidden (`TestForCallerDropsRowsTheCallerCannotSee`,
  `TestForCallerWithNoPermissionCheckerHidesGatedRows`).
- `/admin/layout*` are gated by `system:settings:write`; `GET /admin/layout`
  keeps hidden and gated rows (`TestFullKeepsHiddenAndGatedRows`).
- Menu save: non-empty, ≤ 80, key grammar and uniqueness, label/icon presence,
  absolute and protocol-relative URLs and `javascript:` refused, positions from
  array order, the error naming the row (`TestSaveMenuRenumbersFromArrayOrder`,
  `TestSaveMenuRejectsOffSiteLinks`, `TestSaveMenuAcceptsInAppPaths`,
  `TestSaveMenuRejectsAnEmptyMenu`, `TestSaveMenuRejectsDuplicateKeys`,
  `TestSaveMenuRejectsMalformedKeys`, `TestSaveMenuErrorNamesTheOffendingRow`);
  upsert-by-key then delete non-system absentees in one transaction.
- Widget save: unknown key refused with `layout/unknown-widget`, unknown slot
  refused, per-slot renumbering, update-only (no insert/delete)
  (`TestSaveWidgetsRejectsAnUnknownKey`, `TestSaveWidgetsNumbersEachSlotIndependently`,
  `TestSaveWidgetsRejectsAnUnknownSlot`).
- Both saves write their audit action and re-read the full layout.
- Frontend: `useLayout` (`["layout"]`, 5-minute `staleTime`, no retry); the
  sidebar's unrestricted fallback; rails composed from the layout with unknown
  keys skipped; the editor's staged whole-set saves, dirty marker, no delete on
  `is_system`, no "add widget", invalidation of `["layout"]` after a save; both
  slugs registered in `problems.ts`; `views.adminLayout` resolved through the
  template registry.

**Test evidence to add or fix.**
- The two `href` bypasses (TC-LAY-021), permission validation (TC-LAY-026),
  rune-counted labels (TC-LAY-025), the complete-placement rule (TC-LAY-033).
- HTTP status tests and the adapter's transactional behaviour (row 11).
- The parity check (row 9) and the sidebar fallback rule (row 5).

## 12. Out of scope

Per-user or per-tenant layouts (P2), nested or external menu links, a widget
catalogue in the database, widget-level settings, page building, a public (signed-out)
shell, stream projection of `layout:changed`, and theming (the template version
switch, `NEXT_PUBLIC_TEMPLATE_VERSION`, is a build-time choice owned by
`frontend/src/templates/README.md`, not data).
