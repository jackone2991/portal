# SPEC-18 — Social Connections (mutual links between accounts)

**Status:** current, rev 1 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `social` (`backend/internal/modules/social/`) · **Depends on:** `account` through `accountapi` (`GetUserNames` for display names; the directory behind discovery is `accountapi.ListDirectory`, reached by `people`); SPEC-04 P0.6 (`platform/events` fan-out); SPEC-05 (the two `notify:on_connection_*` consumers turn the events into bell entries); the per-user RLS GUC `app.current_user` set by `platform/db.BeginScope` (ADR-07 request scope)
**Upstream:** as-built spec, written retroactively on 2026-10-01 from the shipped code (`0037_social_connections`, `internal/modules/social`, `frontend/src/lib/social.ts`) and the decisions it implements — [ADR-08](../../adr/08-life-os-pivot.md) (life OS; "friend graph shipped a first slice as the `social` module"), [ADR-01](../../adr/01-v1-scope-cut.md) (social deferred), [ADR-07](../../adr/07-tenancy-rls-model.md) (RLS), feature-inventory §9.3 "Friend graph" (requests only) · **Refs:** `D-1` (notifications are a standalone module; emitters publish events), `D-7` (RFC 7807 + i18n keys), `D-19` (rich profile lives in a future `social.profiles`, not here), `D-29` (envelopes), `D-32`/`D-33`/`D-34` (frontend), [SPEC-11](SPEC-11-people-registry.md) (`people_persons.linked_user_id`, `GET /people/suggestions`)
**Downstream consumers:** SPEC-05 notify (`notify:on_connection_requested`, `notify:on_connection_accepted`); `people` (`socialapi.API.CounterpartIDs` subtracts connected accounts from `GET /people/suggestions`); the frontend header menu, right rail and `/people?circle=requests` tab; SPEC-03 P1.7 takeout (future)

---

## 1. Problem statement

Portal has had a Facebook-shaped shell since the Olympus port — a friend-request
menu in the header, a "friends" right rail, "Kết bạn" buttons — with nothing
behind it: the header menu held four invented people and a badge that counted
them. [ADR-08](../../adr/08-life-os-pivot.md) demoted the friend graph ("dead
weight at n=1") and [SPEC-11](SPEC-11-people-registry.md) deliberately made people
*data*, not accounts. But a household instance does have more than one account
(registration with approval, migration `0031`), and two people on the same
Portal need one fact the registry cannot express: **we both agreed that we know
each other.** A registry row is one-sided and private; a connection is mutual
and both parties see it.

`social` is the smallest object that makes the shell honest: one table, one
relationship per pair of accounts, request → accept / decline / withdraw /
disconnect, and two events so the bell tells the other person. Everything the
feature inventory lists under the social layer — posts, reactions, groups,
messaging, follow graph, block/mute — builds on knowing who is connected to
whom, which is why this slice came first and why it stops here.

## 2. Goals

1. An account can ask another account on the same instance to connect, and the
   other account can accept or decline; either party can later disconnect.
2. One relationship per pair of accounts, whichever side asked — "are we
   connected?" has exactly one answer.
3. Isolation is enforced by the database: a connection is visible to, and
   writable by, its two parties only, even if a query forgets its predicate.
4. The person being asked learns about it in the bell, and the asker learns when
   it is accepted (via SPEC-05), without `social` depending on `notify`.
5. The header badge, the requests tab and the right rail render real rows, not
   fixtures.

## 3. Non-goals

- **Not the social layer.** No posts, reactions, comments, shares, feed,
  messaging, groups/communities, events/RSVP, follow graph (feature-inventory
  §9.12), profiles (`D-19`), mentions or search. ADR-01's deferral of the social
  layer and [backlog.md § Deferred](../backlog.md) ("Social layer beyond `social`
  connections") still hold for all of them. **Reconciling the tension:** ADR-01
  deferred "social" wholesale and ADR-08 called the friend graph dead weight at
  n=1; this module exists because (a) the instance is n>1 once registration with
  approval exists, (b) the slice is one table with no new infrastructure (no
  realtime, no media, no fan-out beyond the existing bell) and so fits the ADR-01
  envelope, and (c) ADR-08's own Consequences record it ("friend graph shipped a
  first slice as the `social` module (`0037`)"). It is a re-entry of *one*
  deferred item under ADR-08, not a reopening of the social roadmap; the next
  social item needs its own spec and its own envelope argument.
- **No block / mute / report.** A declined request is deleted, so the same
  account can ask again (§10 Q1).
- **No friend groups / circles on the connection.** Grouping lives in the
  private registry (`people_persons.circle`, SPEC-11 `0035`); a connection
  carries no label.
- **No connection-scoped permissions.** Being connected grants nothing: no
  module reads `social` to authorize access to content today.
- **No user search.** Discovery is the people module's directory list
  (`GET /people/suggestions`, approved and enabled accounts only); `social`
  takes an account id and never lists accounts.
- **No life-stream projection.** A connection is an event between two people,
  not a card in either one's day (`cmd/api/main.go`, comment above the two
  `Subscribe` calls).
- **Not tenant-scoped.** A connection is between two accounts that each own a
  personal tenant; see §6.

## 4. User stories

- As a member of a household instance, I see "Có thể bạn biết" (people you may
  know) on `/people`, press **Kết bạn** next to my sister's account, and she gets
  a bell entry "Lan wants to connect with you". *(primary)*
- As the person asked, I open the header's friend-request menu (badge = pending
  count) or `/people?circle=requests`, and press **Chấp nhận**; the asker gets
  "Mai accepted your connection request".
- As the person asked, I press **Từ chối**; the request disappears for both of
  us and nothing records that I refused.
- As the asker, I press **Thu hồi** on a request nobody has answered.
- Edge: two people press **Kết bạn** on each other at the same time — the second
  press accepts the first request instead of failing.
- Edge: a third account guessing a connection id learns nothing — the answer is
  the same 404 as a random id.

## 5. Requirements

Status codes, slugs and permission codes below are the contract. Where `HEAD`
diverges, the requirement carries *(code follow-up)* and §11 has the row.

### P0.1 — Module scaffold

`internal/modules/social/` with `module.go` (`New(Deps)`, `MountHTTP`, `API()`),
`api/` (`socialapi`), `handler.go`, `service.go`, `types.go`,
`query/connections.sql`, `repository/` (sqlc output + `adapter.go`); a `sqlc.yaml`
block; migration `0037_social_connections`; a `module-social-isolation` depguard
block in `backend/.golangci.yml`; constructed and mounted in `cmd/api/main.go`
(`social.New` + `socialMod.MountHTTP(r)`). The module owns no Asynq task, so
`cmd/worker/main.go` does not construct it. It never reads `users`: display
names come through `Deps.Names`, a closure over `accountapi.GetUserNames`.

### P0.2 — Request a connection

`POST /api/v1/connections {user_id}` — permission `social:write:own`. The caller
asks the account `user_id` to connect.

- `user_id` missing, not a uuid, or the nil uuid → **422 `social/validation`**,
  except that the caller's own id or the nil uuid is **422
  `social/cannot-connect-to-self`** (`ErrSelf`). *(Code follow-up: `HEAD`
  answers a malformed id with 400 `about:blank` — §11 row 5.)* A body that is
  not JSON is 400 `about:blank` (generic transport, `server.Decode`).
- `user_id` must name an account that is **approved and not disabled** — the
  same population the directory offers (`ListUserDirectory`: `approval_status =
  'approved' AND disabled_at IS NULL`). Otherwise **422 `social/validation`**,
  with one body for "no such account" and "account not eligible", so the
  endpoint does not confirm which accounts exist. The check goes through
  `accountapi`, never `users`. *(Code follow-up: `HEAD` checks nothing; an
  unknown id hits the `users(id)` FK → 500, and a pending, rejected or disabled
  account can be asked and is notified — §11 row 3.)*
- **No row for the pair** → insert `pending` (`requester_id` = caller), respond
  **201** with the Connection (§7), emit `social:connection_requested` (P0.6).
- **The pair already has a row**, in either direction:
  - pending **and the caller is its addressee** (they asked you first) → this
    request *is* agreement: accept that row exactly as P0.3 does, respond
    **201** with the now-`accepted` Connection (same `id`), emit
    `social:connection_accepted` — never a second row;
  - otherwise (already connected, or the caller already asked) → **409
    `social/connection-exists`**, one body for both cases.
- **Concurrent requests** for one pair (A→B and B→A, or a double submit) must
  still answer 409 or the reverse-accept above, never 500. The insert must not
  raise inside the request's tenant transaction: run it `ON CONFLICT DO NOTHING`
  against the pair index and map "no row" to `ErrExists` — a raised unique
  violation aborts the transaction and `RequireTenant` turns the handler's 409
  into a 500 at commit (the same trap as SPEC-11's `people/already-in-registry`).
  *(Code follow-up: `repository/adapter.go` `CreateRequest` maps SQLSTATE
  `23505` to `ErrExists` after the statement has already aborted the
  transaction — §11 row 2.)*

**Acceptance criteria.**
- Given A has never asked B, when A POSTs `{user_id: B}`, then 201, `status =
  pending`, `outgoing = true`, `user_id = B`, and one `social:connection_requested`
  event (TC-SOC-001).
- Given A asked B, when B POSTs `{user_id: A}`, then 201 with the same `id`,
  `status = accepted`, `outgoing = false`, and the table still holds one row for
  the pair (TC-SOC-002; `TestRequestAcceptsAReverseRequest`).
- Given A asked B, when A POSTs again, or given A and B are connected, when
  either POSTs, then 409 `social/connection-exists` (TC-SOC-003).
- Given A POSTs its own id or the nil uuid, then 422
  `social/cannot-connect-to-self` (TC-SOC-004).
- Given `user_id` is `"abc"`, a random uuid, or the id of a pending, rejected or
  disabled account, then 422 `social/validation`, identical bodies for the last
  three, and nothing written (TC-SOC-005).
- Given two concurrent first requests for one pair, then exactly one row exists
  afterwards and neither response is a 5xx (TC-SOC-006).

### P0.3 — Accept

`POST /api/v1/connections/{id}/accept` — permission `social:write:own`. Only the
**addressee** of a **pending** row may accept: the statement is `UPDATE … SET
status = 'accepted', responded_at = now() WHERE id = $1 AND addressee_id = $2
AND status = 'pending'`, and the `connection_update` policy repeats the
addressee rule at the database. Respond **200** with the Connection; emit
`social:connection_accepted`.

Every other case is **404 `social/connection-not-found`** with one body: an id
that is not a uuid, a missing id, a row the caller is not party to, the
requester accepting their own request, and an already-accepted row (a
double-click answers 404 rather than rewriting `responded_at`).

**Acceptance criteria.**
- Given A asked B, when B accepts, then 200, `status = accepted`,
  `responded_at` set, one `social:connection_accepted` event (TC-SOC-010).
- Given A asked B, when A accepts its own request, then 404 and the row stays
  pending (TC-SOC-011; `rls_social_test.go:
  TestRLSRequesterCannotAcceptTheirOwnRequest`).
- Given an accepted row, when B accepts again, then 404 and `responded_at` is
  unchanged (TC-SOC-012).
- Given a third account C, when C accepts, then 404 with the body a random id
  gets (TC-SOC-013).

### P0.4 — Remove (withdraw, decline, disconnect)

`DELETE /api/v1/connections/{id}` — permission `social:write:own` (answering or
withdrawing a request is the same capability as sending one). Either party may
delete the row at any stage; it is one statement for all three meanings because
the row means the same thing in each: this link no longer exists. **204** on
success. A row the caller is not party to, a missing id, a malformed id and a
second DELETE all answer **404 `social/connection-not-found`**, byte-identical.

A declined request is **deleted, not stored**: there is no `declined` status, so
nobody's refusal becomes a permanent record and the pair may try again later
(§10 Q1 covers the resulting re-request loop). Removal emits no event on `HEAD`
(§8, §10 Q2).

**Acceptance criteria.**
- Given a pending row, when the requester deletes it, then 204 and the
  addressee's incoming list no longer shows it (TC-SOC-020).
- Given a pending row, when the addressee deletes it, then 204; the same pair
  can then send a new request, which gets a new `id` (TC-SOC-021).
- Given an accepted row, when either party deletes it, then 204 and it leaves
  both accepted lists (TC-SOC-022).
- Given a third account, when it deletes the row, then 404 and the row survives
  (TC-SOC-023; `TestHTTPConnectionIsNotFoundToAThirdParty`).
- Given a deleted row, when the same party deletes it again, then 404
  `social/connection-not-found` (TC-SOC-024; `TestHTTPDeleteTwiceIs404`).

### P0.5 — Lists and badge

`GET /api/v1/connections?status=accepted|incoming|outgoing` — permission
`social:read:own`. `accepted` (the default when `status` is absent or empty)
lists rows with `status = accepted` the caller is party to; `incoming` lists
pending rows addressed to the caller; `outgoing` lists pending rows the caller
sent. Ordered `created_at DESC, id`. Any other `status` value → **422
`social/validation`**. *(Code follow-up: `HEAD` answers 400 `about:blank` — §11
row 5.)*

Every item is rendered **from the caller's side**: `user_id` and `display_name`
are the *other* account, `outgoing` is true when the caller asked. Names are
resolved in one batch through `accountapi.GetUserNames`; a name that does not
resolve renders as `"Unknown"` and the row is kept (an unanswerable request must
not become invisible). Response `{items: Connection[]}` — non-paginated (§7).
*(Code follow-up: `HEAD` answers `{connections: [...]}` — §11 row 4.)*

`GET /api/v1/connections/summary` — permission `social:read:own` — answers
`{incoming: <count of pending rows addressed to the caller>}`. It is a single
resource, not a list, so it keeps its named field.

**Acceptance criteria.**
- Given A asked B, then B's `incoming` and A's `outgoing` each hold one row whose
  `user_id` is the other party, with `outgoing` false for B and true for A
  (TC-SOC-030; `TestListIsRenderedFromTheCallersSide`).
- Given a counterpart whose name does not resolve, then the row is returned with
  `display_name = "Unknown"` (TC-SOC-031; `TestListKeepsRowsWithUnresolvableNames`).
- Given no `status`, then the accepted list is returned (TC-SOC-032).
- Given `status=blocked`, then 422 `social/validation` (TC-SOC-033).
- Given three requests addressed to B, then `summary` answers `{incoming: 3}`
  (TC-SOC-034).
- Given a caller with no connections, then `{items: []}`, not `null`
  (TC-SOC-035).

### P0.6 — Events

After a request and after an accept (including the reverse-request accept of
P0.2), the service publishes through `platform/events`:

| Event | When | Payload |
|---|---|---|
| `social:connection_requested` | a new pending row | `{connection_id, requester_id, addressee_id, requester_name, addressee_name}` |
| `social:connection_accepted` | pending → accepted | same shape; `requester_id`/`addressee_id` keep their meaning (who asked, who was asked) |

Names are resolved best-effort at publish time (empty string when the lookup
fails; the consumer then writes "Someone"). Publishing is best-effort: a failed
publish is logged (`social: publish failed`) and never fails the request.
`cmd/api/main.go` subscribes each event to its notify task on the `default`
queue (`notify:on_connection_requested`, `notify:on_connection_accepted`); the
worker binary emits neither, so it registers neither (specs README Events
convention). The consumer dedups on `connection_id:requested|accepted`, so a
redelivery is silent while request-then-accept produces two entries, and links
to `/people?circle=requests`.

**Events are published after the request transaction commits.** The request
runs inside `RequireTenant`'s transaction, which is committed after the handler
returns; publishing from the service enqueues the notify task before that, so a
commit that fails (P0.2's race, a dropped connection) leaves a bell entry for a
connection that never existed. *(Code follow-up: `service.go` `emit` runs
inside the handler — §11 row 1.)*

**Acceptance criteria.**
- Given A asks B, then exactly one `social:connection_requested` with
  `requester_id = A`, `addressee_id = B` and both names (TC-SOC-040).
- Given B accepts, then exactly one `social:connection_accepted` with the same
  ids (TC-SOC-041).
- Given the publisher returns an error, then the request still answers 201/200
  (TC-SOC-042).
- Given the request transaction fails to commit, then no event is published
  (TC-SOC-043).

### P0.7 — Isolation at the database

`social_connections` is protected by per-user row-level security keyed on
`app.current_user` (§6), so isolation does not depend on the queries'
predicates: SELECT and DELETE for either party; INSERT only with `requester_id =
current user`; UPDATE only by the addressee, who cannot rewrite the pair. An
unset or empty GUC makes every policy false (an unscoped connection sees
nothing). The routes run behind `authTenant` (auth → `RequireTenant`) because
that is what opens the scoped transaction carrying `app.current_user`, not
because the rows are tenant-scoped.

**Acceptance criteria** (env-gated on `RLS_TEST_ADMIN_URL`/`RLS_TEST_APP_URL`;
CI job `backend` sets both).
- Given a connection between A and B, then C sees no row (TC-SOC-050;
  `TestRLSConnectionVisibleOnlyToItsTwoParties`).
- Given a pending request, then the addressee sees it (TC-SOC-051;
  `TestRLSPendingRequestIsVisibleToTheAddressee`).
- Given C inserts a row with `requester_id = A`, then the policy refuses it
  (TC-SOC-052; `TestRLSCannotForgeARequestFromAnotherUser`).
- Given no user scope, then the table reads empty (TC-SOC-053;
  `TestRLSConnectionsInvisibleWithoutAUserScope`).

### P0.8 — Frontend

Data layer `frontend/src/lib/social.ts` (TanStack, `D-32`; query keys
`["connections", "accepted" | "incoming" | "outgoing"]`, shared so one fetch
serves every surface):

- **Header friend-request menu** (`templates/v1/components/headers/NotifMenus.tsx`
  `FriendRequestsMenu`): badge = number of incoming requests, polled every 60 s;
  rows with **Accept** / **Decline**; no "friends in common" line (there is no
  honest number for it).
- **`/people?circle=requests`** (`templates/v1/views/people/PeopleIndexView.tsx`,
  tab "Lời mời"): "Waiting on you" (Chấp nhận / Từ chối) and "You asked" (Đang
  chờ / Thu hồi); the bell entries link here. The tab "Có thể bạn biết" lists
  `GET /people/suggestions` with **Kết bạn** (request) beside **Thêm vào sổ**
  (add to the private registry with `linked_user_id`).
- **Right rail** (`templates/v1/components/menu/SidebarRight.tsx`): accounts are
  merged from accepted, incoming and outgoing connections and suggestions, with a
  link-state dot (`connected` / `asking` / `asked`); people filed into the
  `close_friend` / `family` circles by `linked_user_id` appear in those sections
  instead.
- **Disconnect** is reachable from the UI for an accepted connection (a
  "Huỷ kết bạn" action on the rail entry or the person's detail). *(Code
  follow-up: `HEAD` calls `DELETE /connections/{id}` only for pending rows; an
  accepted connection cannot be removed from the UI — §11 row 7.)*
- Every mutation invalidates `["connections"]` and `["people"]` (suggestions
  change when a request is sent). Errors render `problemDisplayMessage`, so every
  slug of §7 is registered in `frontend/src/lib/problems.ts`. *(Code follow-up:
  none of the four `social/*` slugs is registered — §11 row 6.)*

**Acceptance criteria.**
- Given two pending requests addressed to me, then the header badge reads 2
  without opening the menu (TC-SOC-060).
- Given I accept from the header menu, then the badge, the requests tab and the
  rail all update without a reload (TC-SOC-061).
- Given a 409 `social/connection-exists`, then the catalog message renders, not
  the raw slug (TC-SOC-062).
- Given an accepted connection, then a disconnect action exists and removes it
  for both parties (TC-SOC-063).

### P1 — nice to have

- **P1.1 `social:connection_removed`** `{connection_id, actor_id, other_id,
  was: 'pending'|'accepted'}` after commit on DELETE, so a future consumer (the
  other party's bell for a withdrawn request, a cache) can react. Emit-only until
  a consumer is specced. Gated on §10 Q2.
- **P1.2 Re-request cooldown** — after a decline, the same requester cannot ask
  the same addressee again for a configurable period (default 7 days); 429
  `social/request-cooldown`. Needs a small `social_declines` table (pair, until).
  Gated on §10 Q1.
- **P1.3 `GET /connections/status?user_id=`** — the caller's relationship with
  one account (`none | asked | asking | connected` + connection id), for a future
  person/profile page that should not fetch three lists.

### P2 — future considerations (design for, don't build)

- **Block / mute** (feature-inventory §9.3) — would add a `blocked` state that
  survives deletion; keep the pair index as the single source of the
  relationship.
- **Follow graph** (§9.12) — a separate asymmetric table, not a third status
  here.
- **Pagination** of the accepted list once one account has hundreds of
  connections (keyset on `created_at, id`, default 30, max 50 per the README
  rule); until then the lists are bounded by the instance's account count.
- **Connection-scoped sharing** (a household member sees selected ledger
  categories) arrives with tenant `kind: household` (`D-24`), not through this
  table.

## 6. Data model — migration `0037_social_connections`

**Tenancy.** `social_connections` is **global, not tenant-scoped**, and is
isolated **per user** rather than per tenant: a connection is by definition
between two accounts that each own a personal tenant, so a `tenant_id` would have
to pick a side and the other party could never read the row. It sits at the
level of `users`. The specs README Tenancy bullet lists the global exemptions
(`users`, `roles`, …) without this table; it is a sanctioned exception with its
own policies, and the README bullet should name it (docs follow-up). Request paths still
run inside `RequireTenant`'s scope, which is what sets `app.current_user`.

```sql
CREATE TABLE social_connections (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    requester_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- identity anchor
    addressee_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- identity anchor
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    responded_at  TIMESTAMPTZ,                     -- set when accepted
    CONSTRAINT social_connections_not_self CHECK (requester_id <> addressee_id)
);
-- one relationship per pair, whichever way round
CREATE UNIQUE INDEX social_connections_pair_idx ON social_connections
    (least(requester_id, addressee_id), greatest(requester_id, addressee_id));
CREATE INDEX social_connections_addressee_idx ON social_connections (addressee_id, status);
CREATE INDEX social_connections_requester_idx ON social_connections (requester_id, status);

CREATE FUNCTION app_current_user() RETURNS uuid LANGUAGE sql STABLE AS
$$ SELECT NULLIF(current_setting('app.current_user', true), '')::uuid $$;

ALTER TABLE social_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE social_connections FORCE ROW LEVEL SECURITY;
CREATE POLICY connection_select ON social_connections FOR SELECT
    USING (app_current_user() IN (requester_id, addressee_id));
CREATE POLICY connection_insert ON social_connections FOR INSERT
    WITH CHECK (requester_id = app_current_user());
CREATE POLICY connection_update ON social_connections FOR UPDATE
    USING (addressee_id = app_current_user()) WITH CHECK (addressee_id = app_current_user());
CREATE POLICY connection_delete ON social_connections FOR DELETE
    USING (app_current_user() IN (requester_id, addressee_id));
```

The same migration seeds `social:read:own` and `social:write:own` and grants
both to the base `user` role (0003's `WITH grants(...)` pattern), so every
approved account can connect. Notes:

- **No `updated_at`.** The only mutation is pending → accepted, recorded in
  `responded_at`; the README `updated_at` rule has nothing to apply to.
- **No `declined` status** (P0.4). The CHECK admits two values.
- **Account deletion** cascades both FKs, so deleting an account silently
  removes its connections; no social event fires (on `HEAD` the account module
  emits none, and SPEC-01 P1.3's planned `account:user_deleted` has notify as
  its only consumer).
- **`app_current_user()`** is created here and dropped by the down migration.
  No later migration uses it (`grep -ln app_current_user backend/db/migrations/`);
  a future table that does must move the function's ownership to a platform
  migration first, or `0037`'s down breaks it.
- **Takeout** (README Takeout): connections are user-history data. Intended
  format: JSON array of the caller's rows rendered from their side — `{other_user_id,
  other_display_name, status, outgoing, created_at, responded_at}` — via
  `socialapi` implementing `opsapi.ExportProvider` when SPEC-03 P1.7 lands.

Queries: `internal/modules/social/query/connections.sql` (DML only); they return
ids, never names, and their predicates express intent while the policies
enforce isolation.

## 7. API summary

All routes are under `/api/v1`, behind `authTenant` (authenticated; 401
`about:blank` without a session), and gated per row below with
`RequirePermission`. Annotate each operation per the specs README AuthZ
**OpenAPI encoding** (`security: [{bearerAuth: []}]` is present on all five;
`x-required-permission` is not — cross-cutting gap).

| Method | Path | Permission | Request | Response | Errors |
|---|---|---|---|---|---|
| POST | `/connections` | `social:write:own` | `{user_id: uuid}` | 201 `Connection` (pending, or accepted when it answered the other side's request) | 400 `about:blank` (non-JSON body); 409 `social/connection-exists`; 422 `social/cannot-connect-to-self`; 422 `social/validation` (malformed, unknown or ineligible `user_id`) |
| GET | `/connections?status=` | `social:read:own` | `status` ∈ `accepted` (default) \| `incoming` \| `outgoing` | 200 `{items: Connection[]}` — non-paginated, ordered `created_at DESC, id` | 422 `social/validation` (unknown `status`) |
| GET | `/connections/summary` | `social:read:own` | — | 200 `{incoming: integer}` | — |
| POST | `/connections/{id}/accept` | `social:write:own` | — | 200 `Connection` (accepted) | 404 `social/connection-not-found` |
| DELETE | `/connections/{id}` | `social:write:own` | — | 204 | 404 `social/connection-not-found` |

Every route also answers 403 `about:blank` when the caller lacks the permission
(below `user` only, since `0037` grants both codes to `user`).

**Connection** (every connection-returning operation):
`{id, user_id, display_name?, status: 'pending'|'accepted', outgoing: boolean, created_at, responded_at?}` —
`id` is the connection's id (what accept/remove take), `user_id` the other
account. List items carry `display_name`; the write responses carry
`responded_at` when set and no `display_name` (they return the row, not a
rendered list) — the OpenAPI schema says so. List items omit `responded_at`.

**Pagination.** `GET /connections` is **not paginated** and takes no `limit` or
`cursor`: its size is bounded by the number of accounts on the instance (a
household), and the three surfaces need the whole set to compute link state. It
answers `{items}` per the specs README Pagination convention (non-paginated
lists included, owner decision 2026-09-30). Pagination is P2.
*(Code follow-up: `HEAD` answers `{connections}`; the README's "Code follow-up
for lists no spec owns" names social `{connections}` — this spec now owns it,
§11 row 4.)*

**Problem types**: `social/connection-not-found` (404), `social/connection-exists`
(409), `social/cannot-connect-to-self` (422), `social/validation` (422,
body/param shape and ineligible targets; declared by this spec), and P1.2's
`social/request-cooldown` (429). Each is registered in
`frontend/src/lib/problems.ts` per the README Errors convention. A malformed
path `id` answers `social/connection-not-found`, not `social/validation`: a
string that is not a uuid names no connection, and the same body as a stranger's
id is the point of P0.3/P0.4.

## 8. Events

| Name | Payload | Status | Consumers |
|---|---|---|---|
| `social:connection_requested` | `{connection_id, requester_id, addressee_id, requester_name, addressee_name}` | live (`0037`), emitted by `cmd/api` | notify — `notify:on_connection_requested` (bell entry for the addressee, `dedup_key = connection_id:requested`) |
| `social:connection_accepted` | same | live (`0037`), emitted by `cmd/api` | notify — `notify:on_connection_accepted` (bell entry for the requester, `dedup_key = connection_id:accepted`) |
| `social:connection_removed` | `{connection_id, actor_id, other_id, was}` | planned (P1.1; §10 Q2) | none specced |

Wiring: `grep -n 'socialapi.Event' backend/cmd/*/main.go` — both `Subscribe`
calls are in `cmd/api/main.go` only, correctly (only the API binary emits).
The module satisfies ADR-08's "≥ 1 bus event from day one".

**Drift with [events.md](../../reference/events.md):** its `social:connection_*`
row and the `notify:on_connection_*` row give the payload as `{connection_id,
requester_id, addressee_id}`; the shipped `socialapi.ConnectionEvent` also
carries `requester_name` and `addressee_name`, which the notify handler reads
for the title. The row should list all five fields (§11 row 9). Both rows
otherwise match the code.

## 9. Success metrics (n=1 honest)

- Leading: on a household instance, every member who has an account is
  connected to at least one other member within the first week, with zero
  requests left unanswered for more than 7 days (auditable: `SELECT count(*)
  FROM social_connections WHERE status = 'pending' AND created_at < now() -
  interval '7 days'`, run as the owner role).
- Leading: zero 5xx on `/api/v1/connections*` in the API logs (the P0.2 race and
  FK paths are the known sources).
- Lagging: the header badge, the requests tab and the rail never show a fixture
  row (grep: no hard-coded people in `NotifMenus.tsx` / `SidebarRight.tsx`), and
  every bell entry for a connection links to a request that exists.
- Honest caveat: at n=1 this module has no user; its value is zero until a
  second approved account exists, which is why it is the whole social slice and
  not the start of a roadmap.

## 10. Open questions

- **Q1 (owner, non-blocking) — Re-request after decline.** A decline deletes the
  row, so the requester can immediately ask again and the addressee gets a new
  bell entry each time (new `connection_id`, new dedup key). Accept this at
  household scale, or build P1.2's cooldown (and later block, §5 P2)?
- **Q2 (owner, non-blocking) — Should removal be an event?** Today a withdrawn
  request leaves the addressee's bell entry pointing at a request that no longer
  exists, and a disconnect is silent to the other party. P1.1 adds
  `social:connection_removed` emit-only; does any consumer (retract the bell
  entry, tell the other party) belong in v1?
- **Q3 (owner, non-blocking) — Who may be asked.** P0.2 restricts targets to
  approved, enabled accounts (the directory population). Should accounts be able
  to opt out of being discoverable/askable, or is "everyone on my instance" the
  right default for a household deployment?

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (`git log 99b5a0b..HEAD -- backend frontend
shared` is empty: the docs commits on top of it change no code). The spec text above is the target; this
section lists every place the shipped code diverges from it. Rows are ordered by
severity: lost or phantom data first, then integrity, then authorization and
contract, then UX and hygiene. A row closes when the code matches the
requirement it cites and a TRACEABILITY-MATRIX row for SPEC-18 is graded on a
named test. The module lives in `backend/internal/modules/social/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.6 publish after commit | `social:connection_requested` / `_accepted` are published only after the request transaction commits; a failed commit publishes nothing. | `service.go` `Request` and `Accept` call `emit` → `EventPublisher.Publish` inside the handler, while `RequireTenant`'s transaction (`tenant/middleware/require_tenant.go` `serveBuffered`) is still open; on a commit failure the client gets 500 but the notify task is already enqueued — a bell entry for a connection that does not exist. | **backend:** defer the publish to after commit (an after-commit hook on the request context, or an outbox row in the same transaction drained after commit — the same fix SPEC-07 §11 row 1 needs; land one mechanism for both). **test:** TC-SOC-043 — a fake scoper whose commit fails → zero publishes. | Found while writing SPEC-18, 2026-10-01 |
| 2 | P0.2 concurrent requests | A concurrent duplicate answers 409 (or the reverse-accept), never 500. | `repository/adapter.go` `CreateRequest` lets the pair-index violation raise and maps SQLSTATE `23505` to `ErrExists`; the statement has already aborted the request's tenant transaction, so `serveBuffered`'s `tx.Commit` fails and the handler's 409 is replaced by **500** "the change could not be committed". Reached only by a race (the service's `FindBetween` pre-check catches the sequential case). The same trap SPEC-11 P0.2 documents for `people/already-in-registry`. | **backend:** `CreateRequest` as `INSERT … ON CONFLICT DO NOTHING RETURNING *` (conflict target: the pair expression index) and map `pgx.ErrNoRows` → `ErrExists`; on `ErrExists` re-run `FindBetween` so a reverse pending row is accepted (P0.2). **test:** TC-SOC-006 — an integration test (RLS harness, two scoped transactions) or an adapter test asserting the conflict leaves the transaction usable. | Found while writing SPEC-18, 2026-10-01 |
| 3 | P0.2 target eligibility | `user_id` must name an approved, non-disabled account, checked through `accountapi`; otherwise 422 `social/validation`, one body for unknown and ineligible. | `service.go` `Request` checks only self/nil. An unknown uuid reaches the `REFERENCES users(id)` FK → SQLSTATE `23503` → `writeSocialErr` default → **500** (and an aborted transaction). A pending, rejected or disabled account can be asked: the row is created and `notify:on_connection_requested` writes a bell entry for an account that cannot sign in. | **backend:** add an eligibility lookup to `Deps` (a closure over a new `accountapi` method, e.g. `IsDirectoryMember(ctx, id) (bool, error)`, mirroring `ListUserDirectory`'s filter) and return `ErrValidation` before the insert; map it to 422 `social/validation`. **openapi:** add the 422 description. **test:** TC-SOC-005 (unknown, pending, rejected, disabled → identical 422). | Found while writing SPEC-18, 2026-10-01 |
| 4 | §7 list envelope | `GET /connections` returns `{items: Connection[]}`; handler, OpenAPI and readers change in one PR. | `handler.go` `List` writes `{"connections": items}`; `shared/openapi.yaml` `listConnections` requires `[connections]`; `frontend/src/lib/social.ts` `listConnections` reads `r.connections`. | **backend:** rename the key. **openapi:** `required: [items]`, property `items`. **frontend:** `listConnections` reads `r.items` (its three callers use the function, not the key). **test:** TC-SOC-035 over HTTP. | Decision 2026-09-30 (Envelopes); specs README "Code follow-up for lists no spec owns" (social `{connections}`) |
| 5 | P0.2, P0.5 validation status | Malformed `user_id` and unknown `status` are 422 `social/validation`. | `handler.go` `Request`: `uuid.Parse` failure → `server.BadRequest("invalid user_id")` (400 `about:blank`); `List`: unknown `status` → `server.BadRequest` (400 `about:blank`). | **backend:** `server.Problem(w, 422, "social/validation", …)` in both; keep `server.Decode`'s 400 for non-JSON bodies. **openapi:** add 422 `social/validation` to `requestConnection` and `listConnections` (neither declares a 400 today either). **test:** TC-SOC-005, TC-SOC-033. | Specs README Pagination/Errors convention (`<module>/validation` is 422) |
| 6 | §7 problem types | Every `social/*` slug the API emits is registered in `problems.ts`. | `frontend/src/lib/problems.ts` has no `social/` entry; `social/connection-not-found`, `social/connection-exists` and `social/cannot-connect-to-self` are emitted by `handler.go` (`probNotFound`, `probExists`, `probSelf`), and `social/validation` arrives with rows 3/5. `PeopleIndexView.tsx` renders them through `problemDisplayMessage`, which falls back to the server `detail`. | **frontend:** add the four slugs to `ProblemType` and `PROBLEM_MESSAGES` (and `social/request-cooldown` with P1.2). **test:** TC-SOC-062. | Specs README Errors convention (DoD) |
| 7 | P0.8 disconnect | An accepted connection can be removed from the UI. | `templates/v1/views/people/PeopleIndexView.tsx` calls `removeConnection` only from `RequestLists` (pending rows); `templates/v1/components/menu/SidebarRight.tsx` reads accepted connections but has no remove action; `NotifMenus.tsx` declines pending rows only. `DELETE /connections/{id}` on an accepted row works over the API but nothing in the UI sends it. | **frontend:** a "Huỷ kết bạn" action (with confirm) on the rail entry for `state = connected`, calling `removeConnection(c.id)` and invalidating `["connections"]`/`["people"]`. **test:** TC-SOC-063. | Found while writing SPEC-18, 2026-10-01 |
| 8 | P0.2–P0.6 HTTP and event tests | Status codes, slugs and event emission are asserted over the real router. | `http_test.go` covers only the third-party DELETE and DELETE-twice; `social_test.go` covers the service rules. Nothing asserts POST 201/409/422, accept 200/404, list/summary shapes, a 403 without the permission, or that `Request`/`Accept` publish the right event with the right ids (the fake service is built without `Events`). | **test:** extend `http_test.go` (TC-SOC-001…005, 010…013, 030…035) and add a recording `EventPublisher` to `social_test.go` (TC-SOC-040…042). | Found while writing SPEC-18, 2026-10-01 |
| 9 | §8 events.md payload | events.md lists the shipped payload. | [events.md](../../reference/events.md) rows `social:connection_requested / _accepted` and `notify:on_connection_*` give `{connection_id, requester_id, addressee_id}`; `api/api.go` `ConnectionEvent` also carries `requester_name`, `addressee_name`, which `notify/service.go` `connectionNotice` uses for the title. | **docs:** add the two name fields to both rows (docs-only; owned by events.md). | Found while writing SPEC-18, 2026-10-01 |
| 10 | §7 OpenAPI 403 | Each operation documents 403 (missing permission). | `shared/openapi.yaml` declares 401 on all five social operations and 403 on none, although every route is behind `RequirePermission`. `x-required-permission` is absent too (cross-cutting README gap). | **openapi:** add `"403": { $ref: "#/components/responses/Forbidden" }` and `x-required-permission` to each (the annotation with the README cross-cutting retrofit). **test:** TC-SOC-070 (403 for a role without `social:*`). | Specs README AuthZ convention (OpenAPI encoding) |
| 11 | §7 list `responded_at` | List items carry `responded_at` when set (the `Connection` schema property). | `handler.go` `List` builds items from `Party`, which has no `RespondedAt`, so list items never carry `responded_at`; only the write responses (`connectionJSON`) do. | **backend:** add `RespondedAt` to `Party` and emit it in `List`. **test:** TC-SOC-030 asserts it on an accepted row. | Found while writing SPEC-18, 2026-10-01 |
| 12 | P0.8 badge source | The badge count is the incoming list (no extra request) — `GET /connections/summary` is the documented badge endpoint. | `frontend/src/lib/social.ts` `connectionSummary` has no caller (`grep -rn connectionSummary frontend/src`); `FriendRequestsMenu` counts `listConnections("incoming")` instead, polling the full list every 60 s. | **decide in the PR:** either switch the badge to `connectionSummary` (cheaper poll; the list loads on open) or delete `connectionSummary` and keep the endpoint for API clients. No behaviour bug. **test:** none. | Found while writing SPEC-18, 2026-10-01 |

**Already matching on HEAD.**
- `0037_social_connections` ships the table, the not-self CHECK, the
  either-direction pair unique index, both lookup indexes, `app_current_user()`,
  ENABLE + FORCE RLS with the four per-party policies, and seeds
  `social:read:own` / `social:write:own` to `user`; the down migration drops all
  of it.
- Routes and permissions match §7 (`module.go` `MountHTTP`), behind `authTenant`
  so the scoped transaction carries `app.current_user`.
- Reverse request = accept of the existing row, same id, one row
  (`service.go` `Request`; `TestRequestAcceptsAReverseRequest`); a repeat request
  in the same direction is 409 (`TestRequestTwiceIsAConflict`); self and nil
  targets are `ErrSelf` → 422 `social/cannot-connect-to-self`
  (`TestRequestSelfRejected`).
- Accept only by the addressee of a pending row; every other case 404 with one
  body (`AcceptRequest` predicate + `connection_update` policy;
  `TestRLSRequesterCannotAcceptTheirOwnRequest`).
- DELETE by either party, 204 then 404, a stranger's id indistinguishable from a
  missing one (`TestHTTPConnectionIsNotFoundToAThirdParty`,
  `TestHTTPDeleteTwiceIs404`, `TestRemoveOnlyByAParty`); a malformed path id is
  404 `social/connection-not-found`.
- Lists rendered from the caller's side, ordered `created_at DESC, id`, names in
  one batch, unresolved names kept as "Unknown"
  (`TestListIsRenderedFromTheCallersSide`, `TestListKeepsRowsWithUnresolvableNames`);
  `status` defaults to `accepted`; `summary` answers `{incoming}`.
- Events: both names, the five-field payload, best-effort publish, both
  `Subscribe` calls in `cmd/api/main.go` only, and the notify consumers'
  `connection_id:phase` dedup keys and `/people?circle=requests` link.
- RLS isolation proven at the database (`platform/db/rls_social_test.go`, all
  five tests, run in CI's `backend` job).
- `people` reads the module only through `socialapi.API.CounterpartIDs`
  (`cmd/api/main.go` `people.Deps.Connected`); `social` reads names only through
  `accountapi.GetUserNames`; depguard's `module-social-isolation` block enforces
  both directions.
- Frontend: the header menu, the requests tab and the rail are on real data with
  shared query keys and no fixture rows; `/people/:path*` is in the
  `middleware.ts` matcher.

**Test evidence to add or fix.**
- No HTTP test for POST, accept, list or summary, and none for 403
  (TC-SOC-001…005, 010…013, 030…035, 070 — row 8).
- No test that the service publishes, or does not publish on failure
  (TC-SOC-040…043 — rows 1, 8).
- No test for the concurrent-request path or the FK/eligibility path
  (TC-SOC-005/006 — rows 2, 3).
- No frontend test (TC-SOC-060…063).

## 12. Out of scope

Posts, feed, reactions, comments, messaging, groups/communities, events/RSVP,
follow graph, profiles (`D-19`), block/mute/report, mentions, user search,
"friends in common", connection-based authorization, stream projection of
connections, per-tenant or household social graphs, and federation with other
Portal instances. Each needs its own spec and an ADR-01 envelope argument;
[backlog.md § Deferred](../backlog.md) holds them.
