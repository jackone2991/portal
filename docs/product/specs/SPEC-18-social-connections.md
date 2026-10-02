# SPEC-18 — Social Connections (mutual links between accounts)

**Status:** current, rev 2 · **Drafted:** 2026-10-01 · **Last verified:** 2026-10-01
**Module:** `social` (`backend/internal/modules/social/`) · **Depends on:** `account` through `accountapi` (`GetUserNames` for display names; the directory behind discovery is `accountapi.ListDirectory`, reached by `people`); `tenant` through `tenantapi` (`ReachableUserIDs`, `CanReach` — who may find and ask whom, SPEC-01 P0.17, Decision 2026-10-02b (B14), [ADR-12](SPEC-01-account-identity-admin.md#adr-12); unbuilt); SPEC-04 P0.6 (`platform/events` fan-out); SPEC-05 (the two `notify:on_connection_*` consumers turn the events into bell entries); the per-user RLS GUC `app.current_user` set by `platform/db.BeginScope` (ADR-07 request scope)
**Upstream:** as-built spec, written retroactively on 2026-10-01 from the shipped code (`0037_social_connections`, `internal/modules/social`, `frontend/src/lib/social.ts`) and the decisions it implements — [ADR-08](README.md#adr-08) (life OS; "friend graph shipped a first slice as the `social` module"), [ADR-01](README.md#adr-01) (social deferred), [ADR-07](SPEC-01-account-identity-admin.md#adr-07) (RLS), feature-inventory §9.3 "Friend graph" (requests only) · **Refs:** `D-1` (notifications are a standalone module; emitters publish events), `D-7` (RFC 7807 + i18n keys), `D-19` (rich profile lives in a future `social.profiles`, not here), `D-29` (envelopes), `D-32`/`D-33`/`D-34` (frontend), [SPEC-11](SPEC-11-people-registry.md) (`people_persons.linked_user_id`, `GET /people/suggestions`)
**Downstream consumers:** SPEC-05 notify (`notify:on_connection_requested`, `notify:on_connection_accepted`); `people` (`socialapi.API.CounterpartIDs` subtracts connected accounts from `GET /people/suggestions`); the frontend header menu, right rail and `/people?circle=requests` tab; SPEC-03 P1.7 takeout (future)

---

## 1. Problem statement

Portal has had a Facebook-shaped shell since the Olympus port — a friend-request
menu in the header, a "friends" right rail, "Kết bạn" buttons — with nothing
behind it: the header menu held four invented people and a badge that counted
them. [ADR-08](README.md#adr-08) demoted the friend graph ("dead
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

1. An account can ask another account it can reach — in the same tenant or an
   actively linked one (Decision 2026-10-02b (B14)) — to connect, and the
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
- **No block / mute / report.** A decline holds the same requester off for 24
  hours (P0.9, Decision 2026-10-02b (B8)) and then lapses; nothing refuses an
  account for good. Block stays P2.
- **No opt-out of being askable** (Decision 2026-10-02b (B10), amended by
  B14). Every approved, enabled account **the caller can reach** — one that
  shares a tenant with the caller or belongs to a tenant actively linked to
  one of the caller's (SPEC-01 P0.17) — can be asked and is offered by
  `GET /people/suggestions`; there is no per-person "hide me" or "nobody may
  ask me" setting. The only switch is the tenant's: an unlinked tenant is
  unreachable as a whole (P0.2). On a household instance the directory *is*
  the people you live with, plus the linked households.
- **No friend groups / circles on the connection.** Grouping lives in the
  private registry (`people_persons.circle`, SPEC-11 `0035`); a connection
  carries no label.
- **No connection-scoped permissions — except one read.** Being connected
  grants no permission code and no write. Since Decision 2026-10-02b (B14)
  ([ADR-12](SPEC-01-account-identity-admin.md#adr-12)) it grants exactly one
  read: an accepted connection whose tenant is actively linked to the owner's
  reads the owner's **published** music, movies and stories, decided in
  Postgres by the tenant module's `app_can_read_shared`, which reads this
  table (§6). Nothing in Go reads `social` to authorise anything.
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
  us, and Lan cannot ask me again until the same time tomorrow — Portal keeps
  that much of the refusal and no more (P0.9).
- As the asker, I press **Thu hồi** on a request nobody has answered.
- Edge: two people press **Kết bạn** on each other at the same time — the second
  press accepts the first request instead of failing.
- Edge: Lan, declined an hour ago, presses **Kết bạn** again — she is told she
  can ask again later (429 `social/request-cooldown`) and I get no new bell
  entry. If I press **Kết bạn** on Lan in that hour, it works: the cooldown
  binds only the direction that was refused.
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
  'approved' AND disabled_at IS NULL`) — **and reachable from the caller**
  *(Decision 2026-10-02b (B14), amending B10; unbuilt — §11 row 15)*: it shares
  a tenant with the caller or belongs to a tenant actively linked to one of
  the caller's (`tenantapi.CanReach`, SPEC-01 P0.17). Every such account is
  askable, with no per-person opt-out (B10). Otherwise **422
  `social/validation`**, with one body for "no such account", "account not
  eligible" and "account not reachable", so the endpoint confirms neither
  which accounts exist nor which tenants are linked. The checks go through
  `accountapi` and `tenantapi`, never `users` or `tenant_links`. *(Code follow-up: `HEAD` checks nothing; an
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
- **No row, but the target declined the caller within the last 24 hours** →
  **429 `social/request-cooldown`** (P0.9), nothing written, no event. The
  order of checks is self → eligibility → existing row → cooldown → insert, so
  an ineligible target is still the uniform 422 and a pending request *from*
  the target is still accepted (agreeing is not re-asking).
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
- Given B declined A within 24 hours, when A POSTs `{user_id: B}`, then 429
  `social/request-cooldown` (P0.9; TC-SOC-071).
- Given A and B in two personal tenants that are not actively linked (no row,
  or one side only), when A POSTs `{user_id: B}`, then 422 `social/validation`
  with the body an unknown id gets, nothing written, no event; once both sides
  list each other, then 201 (TC-SOC-076).
- Given A's tenant linked to B's and to nobody else, then A's
  `GET /people/suggestions` lists B and the other reachable accounts and no
  account of an unlinked tenant (TC-SOC-077).

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

**Accepting needs reach too** *(Decision 2026-10-02b (B14); unbuilt — §11
row 15)*. A pending request whose two parties are no longer reachable from
each other (their tenants were unlinked after it was sent) stays listed and
can be declined or withdrawn (P0.4), but accepting it — here or through
P0.2's reverse request — answers **422 `social/validation`** ("this account
is not reachable") and changes nothing: the caller is a party, so there is no
existence to hide, and an accept would make a new friendship across a fence
the tenants closed. **An accepted connection is never removed by an unlink**:
it stays in both lists and can be disconnected as usual, but grants no
content read while the link is down (ADR-12), and comes back to life if the
tenants link again.

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
- Given A asked B while their tenants were linked, when the link drops and B
  accepts, then 422 `social/validation` and the row stays pending; B can still
  decline it; given an accepted A–B connection, when the link drops, then it
  is still listed for both and still removable (TC-SOC-078).

### P0.4 — Remove (withdraw, decline, disconnect)

`DELETE /api/v1/connections/{id}` — permission `social:write:own` (answering or
withdrawing a request is the same capability as sending one). Either party may
delete the row at any stage; it is one statement for all three meanings because
the row means the same thing in each: this link no longer exists. **204** on
success. A row the caller is not party to, a missing id, a malformed id and a
second DELETE all answer **404 `social/connection-not-found`**, byte-identical.

The row is **deleted in all three cases**: there is no `declined` status, so no
connection row records a refusal. A **decline** — the addressee deleting a
pending row — additionally writes a 24-hour cooldown record for that direction
in the same transaction (P0.9, Decision 2026-10-02b (B8)); a withdraw (the
requester deleting a pending row) and a disconnect (either party deleting an
accepted row) write none. Removal emits `social:connection_removed` after commit
(P1.1, emit-only — Decision 2026-10-02b (B9)); `HEAD` emits nothing (§8).

**Acceptance criteria.**
- Given a pending row, when the requester deletes it, then 204 and the
  addressee's incoming list no longer shows it (TC-SOC-020).
- Given a pending row, when the addressee deletes it, then 204; the addressee
  can then ask the requester at once, and the requester can ask again after
  24 hours — each new request gets a new `id` (TC-SOC-021; the cooldown itself
  is P0.9).
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
- Given `000N_social_acl_grant` applied, then `app_can_read_shared` (running as
  `portal_acl`) sees an accepted A–B row from a scope belonging to neither, while
  `portal_app` in that scope still reads no row of it; `portal_acl` has no
  grant on `social_declines` (TC-SOC-079; Decision 2026-10-02b (B14), §6).

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

### P0.9 — Re-request cooldown after a decline

*(Decision 2026-10-02b (B8); promoted from P1.2; unbuilt — §11 row 13.)*
After the addressee **declines** a request (P0.4: the addressee deletes a
pending row), the same requester cannot ask the same addressee again for **24
hours**. The period is a constant (`RequestCooldown = 24 * time.Hour` in
`service.go`), not configuration: the owner chose one day, and a knob nobody
turns is one more thing to document.

- **How the decline is remembered.** The connection row is still deleted
  (P0.4); the service, seeing that the row it just deleted was pending and the
  caller its addressee, upserts `social_declines (requester_id, addressee_id,
  declined_at = now())` in the same transaction (§6). A repeat decline after the
  window refreshes `declined_at` rather than adding a row, so the table holds at
  most one row per ordered pair and needs no janitor; an expired row is inert.
- **Per direction.** The record binds `requester_id → addressee_id` only. The
  decliner may ask the declined account at any time, and if the decliner has a
  pending request to the declined account, the declined account's POST still
  accepts it (P0.2's reverse-accept runs before the cooldown check).
- **Only a decline starts it.** A withdraw (the requester deleting their own
  pending row) and a disconnect write nothing. A later connection between the
  pair does not clear an earlier record; it lapses on its own.
- **What the API answers.** `POST /connections` for a target that declined the
  caller less than 24 hours ago → **429 `social/request-cooldown`** with a
  `Retry-After` header (whole seconds until `declined_at + 24 h`, rounded up),
  detail "you cannot ask this account again yet"; nothing is written and no
  event fires, so the addressee gets no new bell entry. The check runs after
  eligibility (P0.2), so a target that is no longer eligible answers the
  uniform 422 rather than revealing a decline.
- **What it reveals.** Within the window, the 429 tells the requester that
  their request was declined rather than withdrawn — the price of the owner's
  choice, and little more than the request's vanishing from "You asked"
  already told them. After 24 hours the record answers nothing.

**Acceptance criteria.**
- Given B declined A's request, when A POSTs `{user_id: B}` within 24 hours,
  then 429 `social/request-cooldown` with `Retry-After` > 0, no row, no event
  (TC-SOC-071).
- Given B declined A's request, when B POSTs `{user_id: A}`, then 201 pending;
  and given that pending B→A row, when A POSTs `{user_id: B}`, then 201 with the
  row accepted (TC-SOC-072).
- Given A withdrew its own request, or either party disconnected, when A POSTs
  again, then 201 — no cooldown; and given B declined A more than 24 hours ago,
  when A POSTs, then 201 (TC-SOC-073; a service test with an injected clock).
- Given a decline record for A→B, then C sees no row of `social_declines`, A and
  B each see it, and only B (the addressee) can insert or refresh it
  (TC-SOC-074; RLS suite).

### P1 — nice to have

- **P1.1 `social:connection_removed`** *(Decision 2026-10-02b (B9): emit-only;
  unbuilt — §11 row 14)* `{connection_id, actor_id, other_id, was:
  'pending'|'accepted'}`, published after commit on every successful DELETE
  (withdraw, decline, disconnect — `actor_id` and `was` tell them apart). **No
  consumer in v1:** the addressee's bell entry for a withdrawn request is not
  retracted, and the other party is not told about a disconnect or a decline.
  The event exists so a later consumer (bell retraction, a cache) needs no
  emitter change; a consumer needs its own spec line first. *AC:* one event per
  successful DELETE with the caller as `actor_id`, none on a 404, none when the
  commit fails; `cmd/api` subscribes nothing to it (TC-SOC-075).
- **P1.2** *Promoted to P0.9 by Decision 2026-10-02b (B8) (24 hours, not the
  7-day configurable period sketched here); the number is kept so citations
  hold.*
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
- **Connection-scoped sharing** beyond ADR-12's published-content read (a
  household member sees selected ledger categories) arrives with tenant
  `kind: household` (`D-24`), not through this table.

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
- **No `declined` status** (P0.4). The CHECK admits two values; a decline is
  remembered in `social_declines` below, not on the connection.
- **Account deletion** cascades both FKs, so deleting an account silently
  removes its connections; no social event fires (on `HEAD` the account module
  emits none, and SPEC-01 P1.3's planned `account:user_deleted` has notify as
  its only consumer).
- **`app_current_user()`** is created here and dropped by the down migration.
  No later migration uses it on `HEAD` (`grep -ln app_current_user
  backend/db/migrations/`). Its one planned user is social's own
  `000N_social_request_cooldown` below, whose down runs before `0037`'s; a table
  in any other module must move the function's ownership to a platform
  migration first, or `0037`'s down breaks it.
- **Read by the tenant module's shared-read function** *(Decision 2026-10-02b
  (B14), [ADR-12](SPEC-01-account-identity-admin.md#adr-12); unbuilt — §11
  row 15)*. `app_can_read_shared` (SPEC-01 P0.17) runs as the `NOLOGIN
  BYPASSRLS` role `portal_acl` and reads this table to ask "are these two
  accounts accepted connections?". Social consents in its own migration,
  `000N_social_acl_grant` — `GRANT SELECT ON social_connections TO
  portal_acl;` (down: `REVOKE`), numbered right after `000N_tenant_links`,
  which creates the role. That makes three things a **contract** this module
  must keep or change only together with SPEC-01 P0.17: the columns
  `requester_id` and `addressee_id`, the value `status = 'accepted'` meaning
  "friends", and one row per pair. The function inlines its own read of
  `app.current_user` instead of calling `app_current_user()`, so the note
  above about `0037`'s down migration still holds. `social_declines` is not
  granted and not read.
- **Takeout** (README Takeout): connections are user-history data. Intended
  format: JSON array of the caller's rows rendered from their side — `{other_user_id,
  other_display_name, status, outgoing, created_at, responded_at}` — via
  `socialapi` implementing `opsapi.ExportProvider` when SPEC-03 P1.7 lands.
  Decline records are not exported: a cooldown is a throttle that means
  nothing after 24 hours, not history.

**Cooldown record — migration `000N_social_request_cooldown`** *(Decision
2026-10-02b (B8); unbuilt — §11 row 13; `ls backend/db/migrations | tail -2` for
the number)*. Global and per-user, like `social_connections`, and for the same
reason:

```sql
CREATE TABLE social_declines (
    requester_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- who was declined
    addressee_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,  -- who declined
    declined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (requester_id, addressee_id),        -- per direction, one row
    CONSTRAINT social_declines_not_self CHECK (requester_id <> addressee_id)
);

ALTER TABLE social_declines ENABLE ROW LEVEL SECURITY;
ALTER TABLE social_declines FORCE ROW LEVEL SECURITY;
CREATE POLICY decline_select ON social_declines FOR SELECT
    USING (app_current_user() IN (requester_id, addressee_id));
CREATE POLICY decline_insert ON social_declines FOR INSERT
    WITH CHECK (addressee_id = app_current_user());
CREATE POLICY decline_update ON social_declines FOR UPDATE
    USING (addressee_id = app_current_user()) WITH CHECK (addressee_id = app_current_user());
```

- The primary key is ordered, not `least/greatest`: the cooldown is per
  direction (P0.9), unlike the connection's pair index.
- Only the decliner writes (INSERT, and UPDATE for the `ON CONFLICT DO UPDATE
  SET declined_at = now()` refresh). The requester must be able to read the row,
  because P0.2's cooldown check runs in the requester's scope. There is no
  DELETE policy: rows leave only by the FK cascade when either account is
  deleted, and an expired row is simply ignored (`declined_at > now() -
  interval '24 hours'` is the check).
- No `updated_at`: `declined_at` is the only mutable column and is the
  timestamp.
- The down migration drops the table and its policies; it runs before `0037`'s
  down, which the numeric order guarantees.

Queries: `internal/modules/social/query/connections.sql` (DML only; P0.9 adds
`RecordDecline` — the upsert — and `GetDecline` for one ordered pair); they return
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
| POST | `/connections` | `social:write:own` | `{user_id: uuid}` | 201 `Connection` (pending, or accepted when it answered the other side's request) | 400 `about:blank` (non-JSON body); 409 `social/connection-exists`; 422 `social/cannot-connect-to-self`; 422 `social/validation` (malformed, unknown, ineligible or unreachable `user_id` — B14); 429 `social/request-cooldown` + `Retry-After` (the target declined the caller < 24 h ago — P0.9) |
| GET | `/connections?status=` | `social:read:own` | `status` ∈ `accepted` (default) \| `incoming` \| `outgoing` | 200 `{items: Connection[]}` — non-paginated, ordered `created_at DESC, id` | 422 `social/validation` (unknown `status`) |
| GET | `/connections/summary` | `social:read:own` | — | 200 `{incoming: integer}` | — |
| POST | `/connections/{id}/accept` | `social:write:own` | — | 200 `Connection` (accepted) | 404 `social/connection-not-found`; 422 `social/validation` (the other party is no longer reachable — P0.3, B14) |
| DELETE | `/connections/{id}` | `social:write:own` | — | 204 (a decline also starts P0.9's cooldown) | 404 `social/connection-not-found` |

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
body/param shape and ineligible targets; declared by this spec), and P0.9's
`social/request-cooldown` (429, with `Retry-After`; Decision 2026-10-02b (B8)).
Each is registered in
`frontend/src/lib/problems.ts` per the README Errors convention. A malformed
path `id` answers `social/connection-not-found`, not `social/validation`: a
string that is not a uuid names no connection, and the same body as a stranger's
id is the point of P0.3/P0.4.

## 8. Events

| Name | Payload | Status | Consumers |
|---|---|---|---|
| `social:connection_requested` | `{connection_id, requester_id, addressee_id, requester_name, addressee_name}` | live (`0037`), emitted by `cmd/api` | notify — `notify:on_connection_requested` (bell entry for the addressee, `dedup_key = connection_id:requested`) |
| `social:connection_accepted` | same | live (`0037`), emitted by `cmd/api` | notify — `notify:on_connection_accepted` (bell entry for the requester, `dedup_key = connection_id:accepted`) |
| `social:connection_removed` | `{connection_id, actor_id, other_id, was}` | planned (P1.1, emit-only — Decision 2026-10-02b (B9); §11 row 14), emitted by `cmd/api` after commit | **none in v1** (B9): no bell retraction, no notice to the other party; `cmd/api` subscribes nothing |

Wiring: `grep -n 'socialapi.Event' backend/cmd/*/main.go` — both `Subscribe`
calls are in `cmd/api/main.go` only, correctly (only the API binary emits).
`social:connection_removed` will have no `Subscribe` edge in v1, so a publish
enqueues nothing until a consumer is specced; its events.md row records it as
planned and consumer-less. The cooldown (P0.9) emits no event of its own: a
refused POST writes nothing.
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

Q1–Q3 were decided on 2026-10-02 — Decision 2026-10-02b:

- **Q1 (re-request after decline) — B8.** Build the cooldown: after a decline
  the same requester cannot ask the same addressee for 24 hours, per direction,
  remembered in `social_declines`; 429 `social/request-cooldown`. Now P0.9 (was
  P1.2), §6 and §7; §11 row 13. Block stays P2.
- **Q2 (removal as an event) — B9.** Emit-only: P1.1 adds
  `social:connection_removed` with no consumer in v1 — no bell retraction, no
  message to the other party. Now P1.1 and §8; §11 row 14.
- **Q3 (who may be asked) — B10.** No opt-out: every approved, enabled account
  on the instance is askable, and P0.2's directory population is the default for
  a household deployment. Now P0.2 and §3; shipped code adds no opt-out, so §11
  gains no row (target eligibility itself is row 3). **Amended by B14** the
  same day ([ADR-12](SPEC-01-account-identity-admin.md#adr-12)): "on the
  instance" becomes "reachable from the caller" — the same tenant or an
  actively linked one; the opt-out is the tenant's link list, not a person's
  setting (P0.2, P0.3, §3; §11 row 15).

No open questions remain.

## 11. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (`git log 99b5a0b..HEAD -- backend frontend
shared` is empty: the docs commits on top of it change no code). The spec text above is the target; this
section lists every place the shipped code diverges from it. Rows are ordered by
severity: lost or phantom data first, then integrity, then authorization and
contract, then UX and hygiene; rows 13–14 (added by Decision 2026-10-02b (B8,
B9)) are appended after row 12, and row 15 (added by B14) after row 14. A row closes when the code matches the
requirement it cites and a TRACEABILITY-MATRIX row for SPEC-18 is graded on a
named test. The module lives in `backend/internal/modules/social/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | P0.6 publish after commit | `social:connection_requested` / `_accepted` are published only after the request transaction commits; a failed commit publishes nothing. | `service.go` `Request` and `Accept` call `emit` → `EventPublisher.Publish` inside the handler, while `RequireTenant`'s transaction (`tenant/middleware/require_tenant.go` `serveBuffered`) is still open; on a commit failure the client gets 500 but the notify task is already enqueued — a bell entry for a connection that does not exist. | **backend:** defer the publish to after commit (an after-commit hook on the request context, or an outbox row in the same transaction drained after commit — the same fix SPEC-07 §11 row 1 needs; land one mechanism for both). **test:** TC-SOC-043 — a fake scoper whose commit fails → zero publishes. | Found while writing SPEC-18, 2026-10-01 |
| 2 | P0.2 concurrent requests | A concurrent duplicate answers 409 (or the reverse-accept), never 500. | `repository/adapter.go` `CreateRequest` lets the pair-index violation raise and maps SQLSTATE `23505` to `ErrExists`; the statement has already aborted the request's tenant transaction, so `serveBuffered`'s `tx.Commit` fails and the handler's 409 is replaced by **500** "the change could not be committed". Reached only by a race (the service's `FindBetween` pre-check catches the sequential case). The same trap SPEC-11 P0.2 documents for `people/already-in-registry`. | **backend:** `CreateRequest` as `INSERT … ON CONFLICT DO NOTHING RETURNING *` (conflict target: the pair expression index) and map `pgx.ErrNoRows` → `ErrExists`; on `ErrExists` re-run `FindBetween` so a reverse pending row is accepted (P0.2). **test:** TC-SOC-006 — an integration test (RLS harness, two scoped transactions) or an adapter test asserting the conflict leaves the transaction usable. | Found while writing SPEC-18, 2026-10-01 |
| 3 | P0.2 target eligibility | `user_id` must name an approved, non-disabled account, checked through `accountapi`; otherwise 422 `social/validation`, one body for unknown and ineligible. | `service.go` `Request` checks only self/nil. An unknown uuid reaches the `REFERENCES users(id)` FK → SQLSTATE `23503` → `writeSocialErr` default → **500** (and an aborted transaction). A pending, rejected or disabled account can be asked: the row is created and `notify:on_connection_requested` writes a bell entry for an account that cannot sign in. | **backend:** add an eligibility lookup to `Deps` (a closure over a new `accountapi` method, e.g. `IsDirectoryMember(ctx, id) (bool, error)`, mirroring `ListUserDirectory`'s filter) and return `ErrValidation` before the insert; map it to 422 `social/validation`. **openapi:** add the 422 description. **test:** TC-SOC-005 (unknown, pending, rejected, disabled → identical 422). | Found while writing SPEC-18, 2026-10-01 |
| 4 | §7 list envelope | `GET /connections` returns `{items: Connection[]}`; handler, OpenAPI and readers change in one PR. | `handler.go` `List` writes `{"connections": items}`; `shared/openapi.yaml` `listConnections` requires `[connections]`; `frontend/src/lib/social.ts` `listConnections` reads `r.connections`. | **backend:** rename the key. **openapi:** `required: [items]`, property `items`. **frontend:** `listConnections` reads `r.items` (its three callers use the function, not the key). **test:** TC-SOC-035 over HTTP. | Decision 2026-09-30 (Envelopes); specs README "Code follow-up for lists no spec owns" (social `{connections}`) |
| 5 | P0.2, P0.5 validation status | Malformed `user_id` and unknown `status` are 422 `social/validation`. | `handler.go` `Request`: `uuid.Parse` failure → `server.BadRequest("invalid user_id")` (400 `about:blank`); `List`: unknown `status` → `server.BadRequest` (400 `about:blank`). | **backend:** `server.Problem(w, 422, "social/validation", …)` in both; keep `server.Decode`'s 400 for non-JSON bodies. **openapi:** add 422 `social/validation` to `requestConnection` and `listConnections` (neither declares a 400 today either). **test:** TC-SOC-005, TC-SOC-033. | Specs README Pagination/Errors convention (`<module>/validation` is 422) |
| 6 | §7 problem types | Every `social/*` slug the API emits is registered in `problems.ts`. | `frontend/src/lib/problems.ts` has no `social/` entry; `social/connection-not-found`, `social/connection-exists` and `social/cannot-connect-to-self` are emitted by `handler.go` (`probNotFound`, `probExists`, `probSelf`), and `social/validation` arrives with rows 3/5. `PeopleIndexView.tsx` renders them through `problemDisplayMessage`, which falls back to the server `detail`. | **frontend:** add the four slugs to `ProblemType` and `PROBLEM_MESSAGES` (and `social/request-cooldown` with P0.9 — row 13). **test:** TC-SOC-062. | Specs README Errors convention (DoD) |
| 7 | P0.8 disconnect | An accepted connection can be removed from the UI. | `templates/v1/views/people/PeopleIndexView.tsx` calls `removeConnection` only from `RequestLists` (pending rows); `templates/v1/components/menu/SidebarRight.tsx` reads accepted connections but has no remove action; `NotifMenus.tsx` declines pending rows only. `DELETE /connections/{id}` on an accepted row works over the API but nothing in the UI sends it. | **frontend:** a "Huỷ kết bạn" action (with confirm) on the rail entry for `state = connected`, calling `removeConnection(c.id)` and invalidating `["connections"]`/`["people"]`. **test:** TC-SOC-063. | Found while writing SPEC-18, 2026-10-01 |
| 8 | P0.2–P0.6 HTTP and event tests | Status codes, slugs and event emission are asserted over the real router. | `http_test.go` covers only the third-party DELETE and DELETE-twice; `social_test.go` covers the service rules. Nothing asserts POST 201/409/422, accept 200/404, list/summary shapes, a 403 without the permission, or that `Request`/`Accept` publish the right event with the right ids (the fake service is built without `Events`). | **test:** extend `http_test.go` (TC-SOC-001…005, 010…013, 030…035) and add a recording `EventPublisher` to `social_test.go` (TC-SOC-040…042). | Found while writing SPEC-18, 2026-10-01 |
| 9 | §8 events.md payload | events.md lists the shipped payload. | [events.md](../../reference/events.md) rows `social:connection_requested / _accepted` and `notify:on_connection_*` give `{connection_id, requester_id, addressee_id}`; `api/api.go` `ConnectionEvent` also carries `requester_name`, `addressee_name`, which `notify/service.go` `connectionNotice` uses for the title. | **docs:** add the two name fields to both rows (docs-only; owned by events.md). | Found while writing SPEC-18, 2026-10-01 |
| 10 | §7 OpenAPI 403 | Each operation documents 403 (missing permission). | `shared/openapi.yaml` declares 401 on all five social operations and 403 on none, although every route is behind `RequirePermission`. `x-required-permission` is absent too (cross-cutting README gap). | **openapi:** add `"403": { $ref: "#/components/responses/Forbidden" }` and `x-required-permission` to each (the annotation with the README cross-cutting retrofit). **test:** TC-SOC-070 (403 for a role without `social:*`). | Specs README AuthZ convention (OpenAPI encoding) |
| 11 | §7 list `responded_at` | List items carry `responded_at` when set (the `Connection` schema property). | `handler.go` `List` builds items from `Party`, which has no `RespondedAt`, so list items never carry `responded_at`; only the write responses (`connectionJSON`) do. | **backend:** add `RespondedAt` to `Party` and emit it in `List`. **test:** TC-SOC-030 asserts it on an accepted row. | Found while writing SPEC-18, 2026-10-01 |
| 12 | P0.8 badge source | The badge count is the incoming list (no extra request) — `GET /connections/summary` is the documented badge endpoint. | `frontend/src/lib/social.ts` `connectionSummary` has no caller (`grep -rn connectionSummary frontend/src`); `FriendRequestsMenu` counts `listConnections("incoming")` instead, polling the full list every 60 s. | **decide in the PR:** either switch the badge to `connectionSummary` (cheaper poll; the list loads on open) or delete `connectionSummary` and keep the endpoint for API clients. No behaviour bug. **test:** none. | Found while writing SPEC-18, 2026-10-01 |
| 13 | P0.9 re-request cooldown; §6 `social_declines`; §7 429 | A decline (addressee deletes a pending row) upserts `social_declines (requester_id, addressee_id, declined_at)` in the same transaction; `POST /connections` within 24 h of a decline in that direction answers 429 `social/request-cooldown` with `Retry-After`, writes nothing and emits nothing; per direction; withdraw and disconnect start nothing. | Not built. No `social_declines` table or migration (latest is `0045_journal_location_in_columns`); `query/connections.sql` has no decline query; `service.go` `Remove` discards the row `repo.Delete` returns, and `Request` checks only self and the existing row before inserting; `handler.go` declares no `social/request-cooldown` and `writeSocialErr` has no 429 branch; `shared/openapi.yaml` `requestConnection` declares no 429; `frontend/src/lib/problems.ts` has no `social/` slug. The comments that say a declined request leaves the pair free to try again — `0037_social_connections.up.sql` (status comment) and `types.go` (statuses) — go stale. | **migration:** `000N_social_request_cooldown` (§6). **backend:** `RecordDecline` (upsert) and `GetDecline` queries + `make sqlc`; repository methods; `Remove` records a decline when the deleted row was pending and the caller its addressee; `Request` checks `GetDecline(me, target)` after the existing-row check and returns a new `ErrCooldown{Until}`; `writeSocialErr` maps it to 429 + `Retry-After`; `RequestCooldown` constant; reword the two comments. **openapi:** 429 on `requestConnection` with the `Retry-After` header. **frontend:** `social/request-cooldown` in `problems.ts` (with row 6). **test:** TC-SOC-071…073 (`social_test.go`, an injected clock), TC-SOC-074 (`platform/db/rls_social_test.go`). | Decision 2026-10-02b (B8) |
| 14 | P1.1 `social:connection_removed` | Every successful DELETE publishes `social:connection_removed {connection_id, actor_id, other_id, was}` after commit; no subscriber in v1. | Not built: `api/api.go` declares only `EventConnectionRequested` and `EventConnectionAccepted`; `service.go` `Remove` calls `repo.Delete` and emits nothing; nothing in `backend/` names `connection_removed`. | **backend:** `EventConnectionRemoved` and a `RemovedEvent` struct in `socialapi`; `Remove` publishes after commit through the same mechanism as row 1 (land with or after it); no `Subscribe` call in `cmd/api`. **docs:** events.md row planned → live in the same PR, consumer "none (Decision 2026-10-02b (B9))". **test:** TC-SOC-075 (a recording `EventPublisher`, row 8's harness). | Decision 2026-10-02b (B9) |
| 15 | P0.2 / P0.3 discovery, requests and accepts limited to reach; §6 the shared-read grant | The directory (`GET /people/suggestions`) offers only approved, enabled accounts the caller can reach — same tenant or an actively linked one; a request to any other account is the uniform 422 `social/validation`; accepting a pending row whose parties are no longer reachable is 422 and writes nothing; accepted connections survive an unlink; `000N_social_acl_grant` grants `portal_acl` `SELECT` on `social_connections`. | `backend/cmd/api/main.go` builds `people.Deps.Directory` straight over `accountMod.API().ListDirectory`, which returns every approved, enabled User but the caller — no tenant filter; `service.go` `Request` checks only self/nil (row 3) and `Accept` only the addressee and status; nothing in `backend/` reads tenant links (`tenant_links` does not exist — SPEC-01 §11 row 32); no grant to `portal_acl` (the role does not exist). | **migration:** `000N_social_acl_grant` (§6), right after `000N_tenant_links`. **backend:** a `Reach` dependency in `social.Deps` (a closure over `tenantapi.CanReach`, bound in `cmd/api`) checked in `Request` with row 3's eligibility (same uniform `ErrValidation`) and in `Accept` and the reverse-accept (`ErrValidation` → 422); `cmd/api`'s `people.Deps.Directory` closure intersects the roster with `tenantapi.ReachableUserIDs` — through a new `accountapi.ListDirectoryAmong(ctx, among, exclude, limit)` so `limit` still applies after the filter (SPEC-01 P0.14 gains it). **openapi:** the 422 on `acceptConnection`; the `requestConnection` 422 description. **frontend:** none required — the suggestions list just shrinks; the links screen is SPEC-01 P0.17's. **test:** TC-SOC-076…078 (`social_test.go` with a fake reach), TC-SOC-079 (the grant: `portal_acl` reads the table, `portal_app` still sees only its own rows — RLS suite). **Lands with or after SPEC-01 §11 row 32 and never before the links screen**: on an instance of personal organisations it empties the directory until owners link their tenants (ADR-12 Consequences). | Decision 2026-10-02b (B14), amending B10; ADR-12 |

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
- Nothing to test yet for the cooldown or the removal event (TC-SOC-071…075 —
  rows 13, 14).

## 12. Out of scope

Posts, feed, reactions, comments, messaging, groups/communities, events/RSVP,
follow graph, profiles (`D-19`), block/mute/report, a per-person opt-out from
being asked or suggested (Decision 2026-10-02b (B10)), mentions, user search,
"friends in common", connection-based authorization beyond ADR-12's
published-content read, stream projection of connections, per-tenant or
household social graphs (a tenant link scopes who may connect; the graph
stays one table), and federation with other Portal instances. Each needs its own spec and an ADR-01 envelope argument;
[backlog.md § Deferred](../backlog.md) holds them.
