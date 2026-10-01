# SPEC-03 — Finance Ledger (module `bank`, ledger scope)

**Status:** current, rev 1 · **Drafted:** 2026-07-10 · **Last verified:** 2026-10-01
**Module:** `bank` · **Depends on:** ADR-08 (scope amendment); **SPEC-09 P0 (nightly backup + exercised restore drill) live before the first real ledger entry**; SPEC-01 only for P1 receipts
**Upstream:** [briefs/03-finance-ledger.md](../briefs/03-finance-ledger.md) · **Refs:** feature-inventory.md §8 (implements a subset of §8.1–8.2 plus monthly budgets from §8.7), frontend.md Phase 5
**Downstream consumers:** SPEC-06 (stream + dashboard widget), SPEC-09 P1.7 (bank ExportProvider), SPEC-10

---

## 1. Problem statement

The money facet of the life OS. Agreed scope: a **Money-Lover-class personal
ledger** — multiple accounts, manual transactions, hierarchical categories, monthly
budgets, and inter-account transfers — with a schema that is **import-ready from
day one**, even though statement import itself is deferred (owner's bank, TCB,
exports PDF → import means OCR; see [briefs/04-deferred.md](../briefs/04-deferred.md)).

Scope insight that unblocks this now: a self-hosted **manual** ledger holds no bank
credentials, so the "MFA before bank" gate (D-27/D-28) does not apply. TOTP becomes
the named unlock for *real bank integration* later. ADR-01 deferred "bank"
wholesale — **[ADR-08](../../adr/08-life-os-pivot.md) landed 2026-07 and amends
that deferral, making this scope legitimate.** *(2026-07-10 note: ADR-08 landed
without ratifying this spec's money-representation divergence — see §7 for the
replacement vehicle.)*

## 2. Goals

1. The dev logs daily spending across ≥3 real accounts (TCB, cash, Momo) for a full
   month without friction walls — entry of a routine expense takes < 10 s.
2. Transfers between own accounts never distort income/expense reporting (the #1
   correctness trap identified in the brainstorm).
3. Every money mutation emits a bus event — the money facet feeds the life
   stream from day one. (One carve-out: bulk category reassignment, P0.7.)
4. A future statement-import feature lands **without a breaking migration**
   (columns, dedup key, and batch table exist now).
5. Month-end reconciliation against real accounts closes within rounding — the
   honest success test of the data model.

## 3. Non-goals (each with rationale)

- **Statement import (any format)** — deferred (TCB=PDF→OCR). Schema readiness only
  (§6). Pre-agreed future design: generic CSV/xlsx + column-mapping templates stored
  as *data not code*, per-bank presets, `dedup_hash` dedup, per-batch rollback.
- **Debts, loans, investments, savings goals** (feature-inventory.md §8.3–8.6) —
  separate iterations with their own models; ledger core first. Re-entry: ledger
  reconciles cleanly ≥1 month.
- **Splits, recurring transactions, tags** — P2 architectural insurance only.
- **Multi-currency FX reporting** — currency is per-account; cross-currency totals
  are excluded (no FX infrastructure at v1). The dashboard reports per currency
  group; VND-only users see one group.
- **Real bank connections, money movement, TOTP/step-up** — explicitly the next
  unlock, not this spec.
- **Shared/household ledgers** — single-owner data; multi-user arrives with the
  household/tenant story, not here.

## 4. User stories

- As the user, I create accounts of different types with opening balances so the
  ledger mirrors reality from day one, without backfilling history.
- As the user, I record an expense in seconds — amount, account, category, note —
  because friction kills daily logging. (Emotional job: staying honest with myself
  about money without it feeling like accounting homework.)
- As the user, I record one transfer TCB → Momo and both balances change while
  monthly income/expense move by exactly zero.
- As the user, I set a monthly budget per category and see at a glance how much
  runway is left this month.
- As the user, I archive a closed account; it leaves my pickers but its history
  stays in every past report.
- As the user, I correct a mistyped transaction (wrong amount/category/account) and
  every derived number updates.
- Edge: as the user, I try to delete a category that has transactions and the system
  stops me with a clear path (reassign) instead of corrupting history.

## 5. Requirements

### P0.1 — Accounts

Fields: name, type `cash|checking|savings|credit_card|ewallet|other`, currency
(ISO-4217 `char(3)`, default `VND`), `opening_balance` bigint minor units,
`archived` bool. **Balance is always derived**: `opening_balance + Σ(credits) −
Σ(debits)` — never a stored mutable column. Opening balance is *timeless* in this
math; anything that draws a balance **series** over time must anchor per the §11
opening-balance note. Credit cards at v1 are ordinary
accounts that may go negative (statement cycles are future). Currency is immutable
once the account has transactions. Accounts with transactions cannot be deleted —
only archived (delete allowed while empty).

Archiving is enforced by the API, not only by the pickers: an archived account
rejects new transactions and new transfer legs (409 `bank/account-archived`,
including a transaction PATCH or transfer PATCH that moves a row onto it), while
edits and deletes of its existing rows stay allowed. Unarchive via PATCH
`{archived: false}`. GET `/bank/accounts` returns every account with its
`archived` flag; filtering is the client's job. *(Code follow-up: the shipped
service does not check `archived` on write.)*

**Acceptance criteria.**
- Given an archived account, then it is absent from transaction-entry pickers but
  present in historical reports and the dashboard's archived section.
- Given an account with 1 transaction, when currency change or DELETE is attempted,
  then 409 Problem `bank/account-not-mutable` / `bank/account-not-empty`.
- Given an archived account, when a transaction or transfer naming it is created,
  then 409 `bank/account-archived` and no row is written; editing or deleting one
  of its existing transactions still succeeds.

### P0.2 — Transactions (manual)

Fields: `amount` bigint minor units, strictly positive (VND exponent 0 — 1 unit =
1 đồng; exponent map lives in the shared Money helper, never floats anywhere);
`direction` `debit|credit`; account; category (required unless transfer leg);
`occurred_at` **date** (time-of-day out of scope); `note`. Import-ready columns
present from migration #1 and unused by the manual path: `description_raw`,
`import_batch_id`, `dedup_hash` (all nullable).

Category attachment rules: `category_id` must resolve to the caller's own category
or a seed (`user_id NULL`) — another user's category id is 404, as if it didn't
exist. Direction must match the category's `kind` (`debit` ↔ `expense`, `credit` ↔
`income`), else 422 `bank/direction-kind-mismatch` — otherwise a credit filed under
an expense category silently vanishes from P0.5's "Σ expense debits" math. Refunds
at v1 are therefore logged as income (seed *Hoàn tiền*); netting refunds against
category spend is a future refinement.

Account attachment rules: `account_id` on POST/PATCH `/bank/transactions`, and
`from_account`/`to_account` on POST/PATCH `/bank/transfers`, must resolve to an
account the caller owns (owner-scoped lookup, `user_id = caller`). A foreign or
nonexistent id is 404 `bank/not-found`, as if it didn't exist — never a 422 that
confirms existence. Derived-balance, dashboard and report queries filter on
`user_id = caller` as well as `account_id`. A PATCH that changes `account_id` to
an account with a different `currency` is 422 `bank/currency-mismatch` (the
integer amount would otherwise be silently reinterpreted in the new currency);
a same-currency move re-derives both balances. *(Code follow-up: the shipped
transaction PATCH checks ownership of the new account but not its currency.)*

`transfer_id` is **server-assigned only**. It is not part of the POST/PATCH
`/bank/transactions` request schema and is never read from a request body; only
`/bank/transfers` mints it, so a client cannot create a one-sided leg. On the
manual path `category_id` is required: omitted or null is 422 `bank/validation`,
checked in the service before the insert — never left to the §6 CHECK, which
would surface as a 500.

Entry UX: quick-add dialog reachable from every `/bank/*` page; ≤4
required fields (amount, direction defaulted to expense, account defaulted to last
used, category defaulted to most-recently-used); date defaults to today in the
user's timezone (specs README Timezone);
`MoneyInput` renders VND thousands separators (`1.500.000`).

**Acceptance criteria.**
- Given create/edit/delete of a transaction, then the account's derived balance and
  the month's totals are correct (property test: random sequences reconcile).
- Given amount ≤ 0 or a fractional VND amount, then 422 `bank/invalid-amount`.
- Given a debit attached to an income-kind category (or a credit to an
  expense-kind one), then 422 `bank/direction-kind-mismatch`.
- Given POST `/bank/transactions` without `category_id`, then 422
  `bank/validation` and no row is written.
- Given a PATCH moving a VND transaction onto a USD account, then 422
  `bank/currency-mismatch` and the row is unchanged.
- Given `occurred_at` in the future, then it is accepted and included in that future
  month's reporting (simple rule; no special-casing).
- Given a routine entry by a practiced user, then ≤ 10 s from dialog open to saved.

### P0.3 — Transfers

Modeled as a **paired debit+credit sharing `transfer_id`** (uuid), `category_id
NULL` on both legs. Created atomically via `POST /bank/transfers {from_account,
to_account, amount, occurred_at, note}`. Legs are **not** independently editable:
`PATCH/DELETE /bank/transfers/{transfer_id}` mutates/removes both legs in one DB
transaction; a `PATCH/DELETE` on a leg's transaction id returns 409
`bank/is-transfer-leg` pointing at the transfer endpoint. Same-account transfers
are rejected (422 `bank/same-account-transfer`). Cross-currency transfers are rejected at v1 (422
`bank/currency-mismatch`) — revisit with FX. *(Refines the brief, which has
editing one leg edit the pair: pair edits go only through
`/bank/transfers/{transfer_id}` so a leg is never half-edited; the leg endpoint
returns 409 and names `/bank/transfers/{transfer_id}` in the Problem `detail`.)*

`PATCH /bank/transfers/{transfer_id}` accepts any subset of `{from_account,
to_account, amount, occurred_at, note}` and rewrites both legs atomically. The
POST validations re-run on the resulting pair: own accounts (404
`bank/not-found`, P0.2), `bank/same-account-transfer`, `bank/currency-mismatch`,
`bank/invalid-amount`. Each leg emits one `bank:transaction_updated` (P0.7).

**Transfer legs, precisely** (this predicate is load-bearing for every report): a
*transfer leg* is a row with `transfer_id IS NOT NULL AND category_id IS NULL`.
Reporting exclusions (P0.5 budgets, P0.6 totals) key on **leg-ness**, not on the
mere presence of `transfer_id` — written this way, P1.13's fee row (which carries
both) counts as a normal expense with zero query rewrites.

**Transfer fees (v1 convention).** The API stays a pure equal-amount pair — no fee
field at v1. When a real transfer costs a fee (ewallet withdrawal, interbank
charge), the user logs the fee as an ordinary **expense on whichever account the
institution charged** (usually the source), seed category *Phí & Lệ phí*, alongside
the transfer. The transfer itself still moves income/expense by exactly zero; the
fee is honestly an expense — and without logging it, the charged account drifts by
the fee at month-end, failing §9's reconciliation metric. Structured fee support on
the transfer API is P1.13.

**Acceptance criteria.**
- Given a 5,000,000 VND transfer TCB→Momo, then TCB −5M, Momo +5M, and the month's
  income and expense totals each move by exactly 0.
- Given that transfer PATCHed to 4,000,000, then TCB and Momo each move back by
  1,000,000 and the month's income and expense move by 0.
- Given deletion of a transfer, then both legs disappear atomically (no orphan leg
  under any failure — covered by a transaction-rollback test).
- Given a filtered transaction list, then transfer legs render with a distinct
  transfer badge and the counterparty account name.
- Given a 5,000,000 VND transfer TCB→Momo plus a 2,200 VND fee logged per the
  convention, then TCB −5,002,200, Momo +5,000,000, month expense +2,200, income
  +0, and month-end reconciliation closes.

### P0.4 — Categories

Hierarchical, 2 levels max, `kind` `income|expense`. Seeded Vietnamese-context
defaults (owner's locale), user-extensible:
*expense:* Ăn uống (Đi chợ, Ăn ngoài, Cà phê), Di chuyển (Xăng xe, Grab/Taxi,
Gửi xe), Hóa đơn (Điện, Nước, Internet, Điện thoại), Nhà cửa, Mua sắm, Sức khỏe,
Giải trí, Giáo dục, Phí & Lệ phí, Khác. *income:* Lương, Thưởng, Quà tặng, Lãi,
Hoàn tiền, Thu nhập khác.
Seeds have `user_id NULL`; users may add their own (and rename/hide seeds is P2 —
at v1 seeds are fixed, users add alongside).

Hierarchy invariants (app-enforced): `parent_id` must reference a **top-level**
category (`parent_id IS NULL` — this is what enforces "2 levels max") of the same
`kind` — a child's `kind` must equal its parent's, otherwise P0.5's "including its
children" roll-up would mix kinds; these violations are 422. Ownership follows the
P0.2 convention: a `parent_id` outside the caller's visible set (own or seed) is
**404, as if it didn't exist** — never a 422 that would leak another user's
category ids. `kind` is immutable after creation: a PATCH that changes it is
422 `bank/category-immutable` (the request is well-formed but semantically
invalid; unlike P0.1's currency, `kind` is unconditionally immutable, so it is a
validation failure, not a state conflict). *(Code follow-up: the shipped PATCH
body has no `kind` field, so a `kind` in the body is silently ignored instead of
rejected.)* `parent_id` is mutable via PATCH, but a category that has children
cannot be assigned a parent, and neither can a category be its own parent
(`parent_id = id`) (both 422 `bank/invalid-category-parent`) — together with
the top-level rule this enforces 2 levels max; re-parenting re-runs the same-kind
and own-or-seed checks. "Children" always means direct children.

Deletion rules:
- Seeds are not deletable: owner-scoped mutations (P0.8) match zero `user_id NULL`
  rows → 404. Load-bearing, not just tidy — a seed delete would cascade every
  user's budgets on it.
- A category with transactions cannot be deleted outright: `DELETE ?reassign_to=`
  moves **its own** transactions to the target in the same DB transaction; without
  it, 409 `bank/category-in-use`. `reassign_to` must resolve within the caller's
  visible set (own or seed), be of the same `kind`, and not be the category being
  deleted — kind mismatch is 422 `bank/category-kind-mismatch`; a nonexistent or
  foreign id is 404. `bank_transactions.category_id` keeps its plain FK (NO
  ACTION) as the backstop: an unreassigned delete fails at the DB, never silently.
- Deleting a **parent** promotes its children to top level — `parent_id` is
  declared `ON DELETE SET NULL`, so the DB performs the promotion atomically with
  the delete. Children keep their own transactions and budgets untouched;
  promotion can never deepen the tree.
- Budgets on the deleted category — **all months, including past ones** — are
  dropped with it (`ON DELETE CASCADE`): a budget without its category is
  meaningless, but note this deliberately erases those bars from historical
  dashboards.

**Acceptance criteria.**
- Given a parent with children and its own transactions, when DELETE with a valid
  `reassign_to`, then the transactions move, the children become top-level with
  history and budgets unchanged, and the parent's budgets are gone.
- Given `reassign_to` pointing at an income category while deleting an expense
  one, then 422 `bank/category-kind-mismatch` and nothing changes.
- Given a create with `parent_id` referencing another user's category, then 404
  (existence never leaks); referencing an own-or-seed but non-top-level category,
  then 422.
- Given a DELETE on a seed category, then 404 and the seed survives.
- Given a PATCH changing an own category's `kind`, then 422
  `bank/category-immutable` and the row is unchanged.

### P0.5 — Monthly budgets

One amount per (category, month); `month` stored as first-of-month date. Spent =
Σ expense debits in the month for the category **including its children**,
excluding transfer legs (leg predicate defined in P0.3). Dashboard shows
spent/budget bars with >100% highlighted. The percent shown is
`round(spent × 100 / amount)`, rounded half-up; the >100% highlight uses the exact
comparison `spent > amount`, not the rounded percent. Budgets do not roll over
(rollover is future).

**Write semantics (2026-07-10 — previously undefined):** the §7 `PUT
/bank/budgets` upserts one `(category_id, month, amount)` with `amount > 0`
(the §6 CHECK); sending `amount: 0` or `null` **deletes** that budget row —
the only removal path, keeping the surface at GET/PUT. Category must resolve
within the caller's visible set (own or seed) per the P0.2/P0.4 convention. The
category must be expense-kind; a PUT naming an income-kind category is 422
`bank/category-kind-mismatch` (spent is defined only over expense debits).

Because a parent's spent **contains** its children's, parent and child bars
rendered as flat siblings read as double-counting (1,000,000 logged in *Ăn ngoài*
appears in both bars; a flat list implies 2,000,000 gone). `/bank/budgets` must
render the hierarchy as an indented tree — child bars nested under their parent; an
unbudgeted parent of a budgeted child appears as a non-bar group header. The
dashboard's compact list (P0.6) shows one bar per budgeted category **with no
budgeted ancestor** (so a child-only budget still surfaces), filtered server-side.
Budget-list responses (GET `/bank/budgets` and the dashboard's budget block) carry
`category_id` + `parent_id` + the category **name**, and include synthesized
non-bar header entries for unbudgeted parents of budgeted children — so the tree
renders without a client-side join (ids alone couldn't produce a header for a
parent that has no budget row).

**Acceptance criteria.**
- Given a budget of 3,000,000 on Ăn uống and 3,200,000 spent across its children,
  then the bar shows 107% and is highlighted.
- Given budgets of 3,000,000 on Ăn uống and 1,000,000 on Ăn ngoài and a single
  1,000,000 expense in Ăn ngoài, then Ăn ngoài shows 100% nested (indented) under
  Ăn uống at 33% — one tree, not two flat siblings — and the dashboard's compact
  list shows only the Ăn uống bar.
- Given no budget on a category, then it gets no progress bar (an unbudgeted
  parent may still appear as a group header above a budgeted child; no
  zero-division states).

### P0.6 — Dashboard

`GET /bank/dashboard?month=` returns: per-account derived
balances (active; archived collapsed) — always **current** balances; `month` scopes
only the flow numbers and budget progress, never back-projects balances (see the
§11 opening-balance note) — month income/expense totals (**transfer legs**
excluded — the P0.3 predicate, *not* `WHERE transfer_id IS NULL`, which would
wrongly drop P1.13 fee rows), budget progress list (the P0.5
**no-budgeted-ancestor** roll-up — *not* a `parent_id IS NULL` filter, which would
drop child-only budgets; the full nested tree lives on `/bank/budgets`), and the
10 most recently **entered** transactions (`created_at DESC` — keying recency on
`occurred_at` would let one future-dated entry pin the list; resolved
2026-07-10). Frontend `/bank` (`app/(app)/bank/page.tsx`) renders it as the
bank landing page.

**Contract.** `month` is `YYYY-MM` (the §7 month rule): when omitted it defaults
to the current month in the caller's `users.timezone` (specs README Timezone,
D-17); a malformed value is 400
`bank/invalid-month`. Response:

```
{month, accounts: [{id, name, type, currency, opening_balance, balance, archived}],
 income, expense,
 budgets: [{category_id, parent_id|null, name, parent_name|null, amount|null, spent}],
 recent: [Transaction]}
```

All money is integer minor units; `amount: null` marks a synthesized header
entry (P0.5). `income`/`expense` are single-currency sums. Per-currency grouping
is realised client-side by `accounts[].currency` while every account shares one
currency; the server must split the totals by currency before a second currency
is allowed (tracked with the §11 budgets-vs-currency question). SPEC-06's finance
widget calls this endpoint without `month`. *(Code follow-up: the shipped
`monthParam` defaults to the UTC month and answers a malformed month with 400
`about:blank`.)*

**Acceptance criteria.**
- Given VND-only accounts, then every `accounts[].currency` is VND and the page
  renders one currency group; given
  `?month=2026-06`, then only flow totals and budget bars change — balances stay
  current (the §11 opening-balance rule).
- Given an archived account with history, then it appears in the archived
  section at its current balance and its transactions still count in past
  months' flows.

### P0.7 — Events

Emit on the bus: `bank:transaction_created`, `bank:transaction_updated`,
`bank:transaction_deleted` — payload `{transaction_id, user_id, account_id,
amount, currency, direction, category_id, occurred_at, is_transfer, transfer_id,
counterparty_account_id}`. `currency` is the account's ISO code, so a consumer
can format `amount` with the right exponent (VND = 0) in a multi-currency
ledger; SPEC-06 resolves account and category names at read time. *(Code
follow-up: the shipped payload has no `currency`.)*
`transfer_id` is nullable and lets a consumer group one transfer's rows into a
single story item ("moved 5M TCB→Momo", not two confusing entries); `is_transfer`
is the P0.3 leg predicate, so a P1.13 fee row emits `is_transfer=false` with its
`transfer_id` set. Emitted for transfer legs too (with `is_transfer=true`).
`counterparty_account_id` is nullable — on a transfer leg it is the OTHER leg's
`account_id`, so either leg's payload alone renders the identical "moved 5M
TCB→Momo" card (SPEC-06 owns the direction-normalizing render rule); it is NULL
on non-transfer rows.
Category-delete reassignment (P0.4) emits **no** per-row `transaction_updated`
flood — a bulk reassignment is one user action, not N money mutations (a
single-row PATCH that recategorizes still emits normally); v1 emits nothing for
the bulk path (Goal 3's one carve-out). No consumer required to ship, but these
events already have a registered first consumer (SPEC-06's stream) — publish via
the `platform/events` helper (events.md "Delivery mechanics") so a second
consumer later is a wiring change.

**Acceptance criteria.**
- Given a transaction create/update/delete, then exactly one matching event is
  emitted, after commit (a rolled-back write emits nothing).
- Given POST/PATCH/DELETE `/bank/transfers`, then exactly two events are emitted
  after commit: one per leg, each `is_transfer=true` with the same `transfer_id`
  and `counterparty_account_id` set to the other leg's account.
- Given a category `DELETE ?reassign_to=` moving 500 transactions, then zero
  `bank:transaction_updated` events are emitted (the documented carve-out).

### P0.8 — RBAC

All rows carry `user_id`; every query is owner-scoped — with one deliberate
carve-out: category **reads** span own + seed rows (`user_id IS NULL`), since
P0.2/P0.4 require attaching to seeds, while category **mutations** stay strictly
owner-scoped (which is exactly what makes seeds immutable and undeletable, P0.4).
Permissions *(2026-07-10 reconciliation — the earlier `bank:account:read:own`
family was 4-segment, which is rejected by `rbac.Parse`: wired through
`RequirePermission` it panics at server start (`MustParse` on the required code),
and any dynamic `AllowsCode` check fails closed — returning false even for a `*`
superadmin grant. Kebab-compound resources follow SPEC-04's `notification-prefs`
precedent; actions follow the 0003 catalog's `read|write|delete`)*:

`bank-accounts:read:own`, `bank-accounts:write:own`, `bank-accounts:delete:own`,
`bank-transactions:read:own`, `bank-transactions:write:own`,
`bank-transactions:delete:own`, `bank-categories:read:own`,
`bank-categories:write:own`, `bank-categories:delete:own`,
`bank-budgets:read:own`, `bank-budgets:write:own` (eleven literal codes) — all
granted to the base `user` role; the bank migration seeds the permission rows
and grants (0003 pattern). `write` covers create + update (+ archive for
accounts; transfers are transaction writes). **No cross-user read or write at
any level, including `*`.** The wildcard only passes the route gate; every query
filters `user_id = caller`. Any future admin or household access is a newly
specced surface (new `:any` code, audited endpoint, ADR), never a side effect of
the wildcard — finance data is the most sensitive in the system.

**Acceptance criteria.**
- Given user B's accounts/transactions/categories/budgets, then user A's list
  endpoints return none of them and direct fetches are 404.
- Given user B's account id, when user A POSTs a transaction or a transfer naming
  it (or PATCHes a transaction onto it), then 404 `bank/not-found`, no row is
  written, and B's derived balance is unchanged.
- Given a seed category (`user_id NULL`), then reads include it for every user,
  and any mutation via the shipped endpoints matches zero rows → 404 (what makes
  seeds immutable, P0.4 — no `:any` mutation surface exists at v1, so this holds
  for superadmin too).

### P0.9 — Import scaffolding (schema only)

`bank_import_batches` table exists (empty until the feature lands);
`import_batch_id` references it; partial unique index on `(account_id, dedup_hash)
WHERE dedup_hash IS NOT NULL` guards future re-imports. Cheap now, painful to
retrofit — this is the whole point of doing it in migration #1.

**Acceptance criteria.**
- Given two inserts sharing `(account_id, dedup_hash)`, then the second fails
  on the partial unique index (the future import path's dedup guarantee,
  testable today).

### P1 — nice to have

- **P1.10 Receipt attachments**: `receipt_asset_id uuid NULL` on a transaction,
  with **no FK** (module boundary, specs README). On write, validate through
  `mediaapi` (SPEC-01) that the asset exists, is owned by the caller and is an
  image; otherwise 404 `media/asset-not-found`, or 422 for a non-image. A
  `bank:on_asset_deleted` consumer of `media:asset_deleted` NULLs matching ids,
  and is registered in events.md. Thumbnail in the transaction row; lightbox on
  click.
- **P1.11 Monthly report** — shipped: `GET /bank/report` and the
  `/bank/reports` page (per-category donut and breakdown + a six-month trend).
  Specified in its own section below.
- **P1.12 `bank:budget_exceeded`** event, emitted once per (category, month) on
  first crossing 100% (dedup via a small state table or cache key).
- **P1.13 Structured transfer fees**: `POST /bank/transfers` gains optional
  `{fee_amount, fee_category_id}`; the service writes a **third row** — an
  ordinary expense debit on the source account: `{account_id = from_account,
  direction = 'debit', amount = fee_amount, category_id = fee_category_id
  (expense-kind, own-or-seed per P0.2; default seed *Phí & Lệ phí*), occurred_at =
  the transfer's, transfer_id = the pair's}`. **No new columns** — the §6 CHECK
  already admits a row with both `category_id` and `transfer_id` set. Two
  predicates, kept distinct: **reporting** keys on leg-ness (P0.3), so the fee row
  counts in expense totals and budgets while the pure pair still moves totals by
  zero; the **mutation guard** keys on membership — any row with `transfer_id` set
  409s `bank/is-transfer-leg` on individual PATCH/DELETE (a slight misnomer for
  the fee row; acceptable while unshipped). `PATCH/DELETE
  /bank/transfers/{transfer_id}` manages all rows sharing the id atomically:
  `fee_amount: null|0` on PATCH deletes the fee row; supplying a fee on a pure
  pair creates it (the `amount > 0` CHECK forbids zero-fee rows at rest). In lists
  the fee row renders as an ordinary expense with a badge linking to its transfer.
  Destination-charged fees stay on the P0.3 manual convention (this row is
  source-hardwired). Until P1.13 lands, the P0.3 convention applies.

### P1.11 — Monthly report (shipped)

`GET /api/v1/bank/report?month=` — permission `bank-transactions:read:own`, as
`module.go` gates it (the report reads transactions and their categories; all bank
read codes are granted to `user` together, P0.8). Owner-scoped like every bank
read: each query filters `user_id = caller`, so no other user's rows can enter a
slice, a total or a trend bar. It backs the `/bank/reports` page (the
`bankReports` view, §8); SPEC-10 §7 measures its slices against the month
totals.

**Contract.** `month` follows the §7 month rule — `YYYY-MM`; omitted → the
current month in the caller's `users.timezone` (specs README Timezone, D-17);
malformed → 400 `bank/invalid-month`. Response `200`:

```
{month: "YYYY-MM", income, expense,
 expenses: [CategoryTotal], incomes: [CategoryTotal],
 trend: [{month: "YYYY-MM", income, expense}]}
CategoryTotal = {category_id, name, kind, icon|null, color|null, total, tx_count}
```

- `income` / `expense` are the month's totals with **pure transfer legs**
  excluded by the P0.3 predicate (a P1.13 or manual fee row counts as expense) —
  the same numbers the dashboard shows for that month (P0.6).
- `expenses` / `incomes` hold one slice per **top-level** category: a child's
  transactions roll up into its parent, and `category_id`, `name`, `icon` and
  `color` are the parent's (the per-child detail stays reachable through the
  transactions list's `category` filter). `total` sums the slice's amounts,
  `tx_count` counts its rows; slices are split by the category's `kind` and
  sorted by `total` descending; own and seed categories alike. Pure legs carry
  no category and never appear. Because every non-leg row has a category whose
  kind matches its direction (P0.2), `Σ expenses[].total = expense` and
  `Σ incomes[].total = income`.
- `trend` is exactly six months (`trendMonths`) ending at `month`, oldest first;
  a month with no activity is present with zeroes, never omitted.
- Money is integer minor units. The sums are single-currency, under the P0.6
  rule: per-currency grouping is client-side while every account shares one
  currency, and the server splits the report by currency before a second
  currency is allowed (§11 budgets-vs-currency question).
- The report is one composite object, not a collection (specs README
  Pagination): its arrays are named fields, there is no `{items}` envelope and no
  `limit`. An empty month answers zeroes and empty arrays, never 404.

Problems: 400 `bank/invalid-month`; 401 unauthenticated; 403 without the
permission. *(Code follow-up: `handler.go` `Report` shares `monthParam`, so an
omitted month is the UTC month and a malformed one 400 `about:blank`, and
`parseMonth` also accepts `YYYY-MM-DD` — §12 row 8; `shared/openapi.yaml`
`getBankReport` declares only the 401 and no `x-required-permission` — §12
rows 8 and 12.)*

**Acceptance criteria.**
- Given 50,000 in *Cà phê* and 250,000 in *Ăn ngoài* (both children of *Ăn
  uống*), a 20,000,000 *Lương* credit and a 1,000,000 transfer between two own
  wallets in August 2026, when `GET /bank/report?month=2026-08`, then
  `expenses` is one *Ăn uống* slice of 300,000 over 2 transactions, `incomes`
  one *Lương* slice of 20,000,000, `expense` = 300,000 and `income` =
  20,000,000 — the transfer counts nowhere.
- Given a transfer with a fee row in that month, then the fee appears in its
  category's slice and in `expense`, and the slices still sum to the totals.
- Given `?month=2026-08`, then `trend` holds 2026-03 … 2026-08 in that order
  and a month without activity reads `{income: 0, expense: 0}`.
- Given no `month` at 2026-06-30 18:00 UTC for an owner in
  `Asia/Ho_Chi_Minh`, then `month` = `2026-07`; given `?month=2026-13` or
  `?month=abc`, then 400 `bank/invalid-month`.
- Given user B's transactions in the same month, then user A's report never
  includes them.

### P2 — future considerations (architectural insurance)

- **Splits**: report queries must aggregate via a join on a (transaction→category)
  relation conceptually — don't bake "exactly one category column" into report SQL
  shapes more than necessary.
- **Recurring transactions**: generated as drafts the user confirms
  (feature-inventory.md §8.2); implies a `status` concept — don't preclude
  adding a status column.
- **Tags**; **statement import** (design pre-agreed, see Non-goals).
- **Rename/hide seed categories** (deferred from P0.4 — at v1 seeds are fixed
  and users add alongside; previously the deferral dangled with no P2 entry).

## 6. Data model — migration `000N_bank_core`

**Tenancy** (specs README convention, ADR-07). Tenant-scoped: `bank_accounts`, `bank_import_batches`, `bank_transactions`, `bank_budgets` (`0020_platform_rls_enable`). `bank_categories` is the shared-seed precedent: nullable `tenant_id`, policy reads own-tenant `OR tenant_id IS NULL`, writes own-tenant only. The DDL below predates ADR-07 and omits the columns.

```sql
-- user_id columns carry the sanctioned identity-anchor FK (SPEC-04 §6 /
-- 0007 precedent) — added 2026-07-10; without it a deleted user orphans
-- finance rows forever.
CREATE TABLE bank_accounts (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name            text NOT NULL,
  type            text NOT NULL CHECK (type IN
                    ('cash','checking','savings','credit_card','ewallet','other')),
  currency        char(3) NOT NULL DEFAULT 'VND',
  opening_balance bigint NOT NULL DEFAULT 0,
  archived        boolean NOT NULL DEFAULT false,
  created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ON bank_accounts (user_id, archived);

CREATE TABLE bank_categories (
  id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id   uuid REFERENCES users(id) ON DELETE CASCADE,  -- NULL = seeded default
  parent_id uuid REFERENCES bank_categories(id) ON DELETE SET NULL,
                                           -- parent delete promotes children (P0.4)
  name      text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
  kind      text NOT NULL CHECK (kind IN ('income','expense'))
);
CREATE INDEX ON bank_categories (user_id);
CREATE INDEX ON bank_categories (parent_id) WHERE parent_id IS NOT NULL;

CREATE TABLE bank_import_batches (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  source     text NOT NULL,
  file_name  text,
  status     text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE bank_transactions (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  account_id      uuid NOT NULL REFERENCES bank_accounts(id),
  category_id     uuid REFERENCES bank_categories(id),   -- NULL only for transfer legs
  amount          bigint NOT NULL CHECK (amount > 0),
  direction       text NOT NULL CHECK (direction IN ('debit','credit')),
  transfer_id     uuid,
  occurred_at     date NOT NULL,
  note            text,
  description_raw text,                                   -- import-ready
  import_batch_id uuid REFERENCES bank_import_batches(id),-- import-ready
  dedup_hash      text,                                   -- import-ready
  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),
  CHECK (category_id IS NOT NULL OR transfer_id IS NOT NULL)
);
CREATE INDEX ON bank_transactions (user_id, occurred_at DESC, id DESC);
CREATE INDEX ON bank_transactions (account_id, occurred_at DESC);
CREATE INDEX ON bank_transactions (category_id) WHERE category_id IS NOT NULL;
CREATE INDEX ON bank_transactions (transfer_id) WHERE transfer_id IS NOT NULL;
CREATE UNIQUE INDEX ON bank_transactions (account_id, dedup_hash)
  WHERE dedup_hash IS NOT NULL;

-- month = first-of-month date
CREATE TABLE bank_budgets (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  category_id uuid NOT NULL REFERENCES bank_categories(id) ON DELETE CASCADE,
                                           -- budgets die with their category (P0.4)
  month       date NOT NULL CHECK (EXTRACT(day FROM month) = 1),
  amount      bigint NOT NULL CHECK (amount > 0),
  UNIQUE (user_id, category_id, month)
);
```

The indexes above mirror the shipped `0014_bank_core`. **Follow-up migration
`000N_bank_integrity`** (not yet shipped):

```sql
CREATE INDEX ON bank_transactions (user_id, created_at DESC);  -- dashboard recency (P0.6)
CREATE INDEX ON bank_budgets (category_id);                    -- cascade and reassign scans
ALTER TABLE bank_categories ADD CHECK (parent_id IS DISTINCT FROM id);
                          -- a childless top-level category could otherwise parent itself
ALTER TABLE bank_accounts ADD CHECK (currency ~ '^[A-Z]{3}$');
```

The service already rejects `parent_id = id` with 422
`bank/invalid-category-parent` (P0.4); the CHECK is the backstop.

Category seeds ship as a data migration in the same sequence. `dedup_hash` (future)
= sha256 of `account_id|occurred_at|amount|direction|bank_ref` — computed only by
the import path.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission |
|---|---|---|
| GET | `/api/v1/bank/accounts` | `bank-accounts:read:own` |
| POST | `/api/v1/bank/accounts` | `bank-accounts:write:own` |
| PATCH | `/api/v1/bank/accounts/{id}` | `bank-accounts:write:own` |
| DELETE | `/api/v1/bank/accounts/{id}` | `bank-accounts:delete:own` |
| GET | `/api/v1/bank/transactions?account=&month=&category=&cursor=` | `bank-transactions:read:own` |
| POST | `/api/v1/bank/transactions` | `bank-transactions:write:own` |
| PATCH | `/api/v1/bank/transactions/{id}` | `bank-transactions:write:own`; 409 on transfer legs |
| DELETE | `/api/v1/bank/transactions/{id}` | `bank-transactions:delete:own`; 409 on transfer legs |
| POST | `/api/v1/bank/transfers` | `bank-transactions:write:own` |
| PATCH | `/api/v1/bank/transfers/{transfer_id}` | `bank-transactions:write:own` |
| DELETE | `/api/v1/bank/transfers/{transfer_id}` | `bank-transactions:delete:own` |
| GET | `/api/v1/bank/categories` | `bank-categories:read:own` |
| POST | `/api/v1/bank/categories` | `bank-categories:write:own` |
| PATCH | `/api/v1/bank/categories/{id}` | `bank-categories:write:own`; 404/409/422 per P0.4 |
| DELETE | `/api/v1/bank/categories/{id}` (`?reassign_to=`) | `bank-categories:delete:own`; 404/409/422 per P0.4 |
| GET | `/api/v1/bank/budgets?month=` | `bank-budgets:read:own` |
| PUT | `/api/v1/bank/budgets` (body `{category_id, month, amount}`) | `bank-budgets:write:own` (upserts; `amount: 0\|null` deletes — P0.5) |
| GET | `/api/v1/bank/dashboard?month=` | `bank-accounts:read:own` |
| GET | `/api/v1/bank/report?month=` | `bank-transactions:read:own` (P1.11; composite object, not a list) |

*(Codes reconciled 2026-07-10 to the 2–3-segment grammar — see P0.8.)*

The dashboard is a deliberate composite read gated on `bank-accounts:read:own`,
although it also returns budgets and transactions: all bank read codes are always
granted together to `user` (P0.8). Revisit if they diverge.

**Month rule.** `month` is `YYYY-MM` on the wire everywhere and is stored as the
first-of-month date. The budgets PUT carries it in the body, not the query. An
omitted `?month=` on a GET means the current month in the caller's timezone
(`users.timezone`, read through `accountapi` — specs README Timezone, D-17;
unknown → `Asia/Ho_Chi_Minh`, never UTC). A malformed month is 400
`bank/invalid-month`. *(Code follow-up: the shipped handler defaults to the UTC
month and answers a malformed month with 400 `about:blank`; it needs the
caller's zone from `accountapi`, which does not expose it yet — README
Timezone follow-up.)*

The transactions list paginates by **cursor**, not offset `?page=` (offset
paging duplicates or skips rows under inserts; keyset paging is stable under
inserts, though a row whose `occurred_at` is edited mid-scroll may cross the
cursor — acceptable for a personal list): ordering is
`occurred_at DESC, id DESC` and the response is `{items, next_cursor}` — the
specs README Pagination convention and §8's infinite-list requirement.
`?limit=` keeps the OpenAPI-declared default 50, max 100, and is lenient
per the specs README rule (missing, non-integer or < 1 → 50; above 100 →
clamped to 100; never a Problem). A
malformed cursor is 400 `bank/invalid-cursor`; a param-shape failure 422
`bank/validation`. The non-paginated lists answer `{items}` too:
`GET /bank/accounts` and `GET /bank/categories` return `{items: [...]}`, and
`GET /bank/budgets` returns `{month, items: [...]}` (`month` is an extra
top-level field, not the list). Pre-rule endpoints are retrofitted, not
grandfathered (specs README Pagination, owner decision 2026-09-30); there is no
`transactions`, `accounts`, `categories` or `budgets` list key. *(Code
follow-up: the shipped handlers and `shared/openapi.yaml` still answer
`{transactions: [...], next_cursor?}`, `{accounts}`, `{categories}` and
`{month, budgets}`; the retrofit renames each key to `items` in the handler,
`shared/openapi.yaml` and the `lib/bank.ts` readers in one PR.)* Annotate each operation per the specs README AuthZ **OpenAPI encoding** (combined-method rows split per operation).

Problem types: `bank/account-not-empty`, `bank/account-not-mutable`,
`bank/is-transfer-leg`, `bank/same-account-transfer`, `bank/currency-mismatch`,
`bank/category-in-use`, `bank/category-kind-mismatch`,
`bank/direction-kind-mismatch`, `bank/invalid-amount`,
`bank/invalid-category-parent` (P0.4's non-top-level / wrong-kind / 2-level
violations), `bank/category-immutable` (P0.4's kind-immutability),
`bank/invalid-cursor`, `bank/validation` (generic request-shape violations,
including a manual transaction without `category_id`, P0.2), `bank/not-found`
(404 — a single type for every missing or foreign account, transaction,
transfer or category id, including ids outside the caller's visible set;
existence never leaks), `bank/account-archived` (409, P0.1),
`bank/invalid-month` (400, the month rule above).

All money fields in `bank` request/response bodies are integer minor units (VND
exponent 0) end-to-end — the shared Money helper owns the exponent map. **This
knowingly diverges from the canonical money decisions, and the divergence is
wider than previously acknowledged** *(2026-07-10)*:

- **D-14 (wire + storage)**: D-14 resolves money as `numeric(20,8)` columns +
  `shopspring/decimal` + string amounts on the wire ("never JSON numbers").
  This spec uses `bigint` minor units in storage and JSON-integer minor units
  on the wire — for exponent-0 VND an integer is exact, while a decimal string
  reintroduces the parsing layer the Money helper exists to avoid.
- **D-15 (bookkeeping model)**: D-15 resolves "hybrid double-entry internals"
  (`ledger_entries` with a per-transaction balance CHECK). This spec is
  deliberately single-row (+ paired legs for transfers) — Money-Lover-class,
  not accounting-grade; double-entry returns with the creator-economy scope
  that actually needs it.

**Ratified as D-41** (feature-inventory.md), scoped "v1 personal-ledger only;
D-14/D-15 stand for any multi-currency or creator-economy money". The paired
obligation — reconciling frontend.md §5.3 and its Phase-5 component notes to
integer minor units — is done.

## 8. Frontend (`bank` pages under the `(app)` group — resolves the blocking question)

Pages live under the existing authenticated group —
`app/(app)/bank/page.tsx` and
`app/(app)/bank/{transactions,accounts,budgets,categories}/page.tsx` — so they inherit
the `(app)` shell/nav and the login gate; there is **no** separate `(bank)` route
group (only `(app)` and `(public)` exist, per CLAUDE.md). No Olympus template maps
cleanly to finance, so pages are composed
from the existing shell + `components/ui` primitives plus Phase-5 money components
(frontend.md): `<MoneyDisplay />`, `<MoneyInput />` (display-layer string handling
with VND thousands separators; the wire carries integer minor units per §7),
and budget bars as inline SVG/CSS components (any chart dependency needs a
bundle-budget check).

- `/bank` — dashboard (P0.6), quick-add button (global within the group)
- `/bank/transactions` — filterable infinite list; edit/delete inline; transfer badge
- `/bank/accounts` — list + create/archive
- `/bank/budgets` — month picker + per-category budget editor
- `/bank/reports` — month picker, per-direction donut and breakdown, six-month
  trend (P1.11)
- `/bank/categories` — two-level tree per kind; seeds read-only; own categories
  can be created, renamed and re-parented. Delete opens a reassign picker that
  sends `?reassign_to=`, and 409 `bank/category-in-use` renders inline.

RSC-first shells ("RSC shell" as defined in the specs README Frontend
convention); the quick-add dialog and lists are client islands. Left-menu
entry added to the shell nav.

**Auth gate.** Add `'/bank/:path*'` to `config.matcher` in
`frontend/src/middleware.ts` so the D-34 session gate runs on the new routes and
redirects an unauthenticated visitor to `/login`.

**Template registry.** Every new page view is declared in `TemplateManifest.views`
(`frontend/src/templates/types.ts`), implemented under
`templates/v1/views/...`, and each `app/(app)/bank/<route>/page.tsx` resolves it
via `activeTemplate().views.<x>` — never a version-specific import in `app/` (keeps
the `v2` switch intact). The P0 keys are `bankDashboard`, `bankTransactions`,
`bankAccounts`, `bankBudgets` and `bankCategories` (`bankReports` serves P1.11;
`bankDebts` is SPEC-10's).

## 9. Success metrics (n=1 honest)

- Leading: transactions logged on ≥20 of the first 30 days (friction test) —
  observable straight from the table.
- Leading: median quick-add duration < 10 s (log client-side timing during dogfood).
- Lagging: month-end reconciliation vs real accounts closes within rounding. **If
  reconciliation is painful, the transfer/edit model is wrong — fix before adding
  any P1/P2 feature.**

## 10. Timeline & phasing

1. Migration + seeds + sqlc (1 day)
2. Accounts + transactions CRUD + attachment validation (P0.2) + derived
   balances + RBAC + OpenAPI (1.5 days)
3. Categories: CRUD, hierarchy invariants, delete/reassign matrix (P0.4) (1 day)
4. Transfers (paired semantics + fee-convention tests) (1 day)
5. Budgets + dashboard endpoint (incl. the no-budgeted-ancestor roll-up) (1 day)
6. Frontend: quick-add, transactions list, dashboard, accounts, budgets tree
   (2.5 days)
7. Events + polish (½ day)
P0 ≈ 8.5 dev-days (category semantics and the budget tree grew it past the
original 7); do not start P1 before the first
reconciliation succeeds.

**Backup gate.** Do not begin Goal 1's month of real logging until SPEC-09
P0.2–P0.4 (nightly backup + exercised restore drill) is green.

## 11. Open questions

- **(product, non-blocking)** Life-stream privacy: should `bank:*` events carry
  amounts, or only counts ("logged 3 transactions today")? Payload above carries
  amounts; the *consumer* (SPEC-06's life stream) decides display. SPEC-06 §11
  defaults to show for n=1; revisit at household tenancy.
- **(product, resolved for v1)** Opening-balance date semantics: timeless (applies
  before all transactions) — correct for current balances and monthly flow totals,
  which is all v1 computes (`opening_balance` never enters a flow sum, so P0.6
  totals and P1.11 month-over-month are unaffected). **Technical note for future
  trend charts** (net worth over time): a timeless opening balance back-projects
  today's opening amount into months before the account was tracked — a false
  plateau. A historical series must start each account's line at
  `LEAST(created_at::date, MIN(occurred_at))` (cast in the user's timezone, specs README Timezone;
  backdated entries are legal per P0.2, and an account with no transactions still
  enters at creation), with the opening balance applying **at** that anchor, never
  before it. Corollary while backdating: an entry dated before the account was
  onboarded double-counts against an opening balance that already reflected it —
  adjust `opening_balance` when backfilling pre-onboarding history. Recorded here
  so the chart feature doesn't re-litigate the ledger model.
- **(engineering, non-blocking)** Budgets vs currency groups: `bank_budgets` has no
  currency column, yet budget spent sums transactions across accounts whose
  currency is per-account, and the dashboard groups by currency. Harmless while
  all the user's accounts are VND (the n=1 reality); before a second currency
  appears, either scope budgets to a currency (column + widen the unique) or pin
  "budget spent sums VND-account transactions only."
- **(engineering, non-blocking)** Derived-balance performance: SUM over an index is
  fine for years of personal data; add a materialized running balance only if the
  dashboard ever exceeds budget (measure first).

## 12. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the commits after it changed docs only).
The spec text above is the target; each row below is a place where the shipped
code still diverges from it, verified against that commit. Rows are ordered by
severity: security first, then data integrity (silent wrong numbers or lost
rows), then API contract, then UX. A row closes when the code matches the spec
and the SPEC-03 rows of `docs/reference/TRACEABILITY-MATRIX.md` are regraded.
File paths are relative to `backend/internal/modules/bank/` unless they start
with `backend/`, `frontend/` or `shared/`.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | **Security** — P0.2 account attachment, P0.8 | Derived-balance, dashboard and report queries filter `user_id = caller` as well as `account_id`. | `query/bank.sql` `ListAccountBalances` and `GetAccountBalance` join `bank_transactions t ON t.account_id = a.id` and filter only `a.user_id`; `CountAccountTransactions` filters `account_id` alone. Writes cannot attach a foreign account today (`service.go` calls `repo.GetAccount(ctx, userID, id)` first), and nothing in the schema ties `bank_transactions.user_id` to the account's owner, so the query filter is the only second fence. | backend: add `AND t.user_id = a.user_id` to both balance joins; give `CountAccountTransactions` a `user_id` parameter; regenerate sqlc. test: fake-repo case with a row whose `user_id` differs from the account owner, asserting it never enters the balance. | F053 |
| 2 | **Data integrity** — P0.1 archive enforcement (also P0.3) | An archived account rejects new transactions and new transfer legs with 409 `bank/account-archived`, including a transaction or transfer PATCH that moves a row onto it; edits and deletes of its existing rows stay allowed. | `service.go` `CreateTransaction`, `UpdateTransaction` (when `account_id` changes), `CreateTransfer` and `UpdateTransfer` call `repo.GetAccount` and never read `Account.Archived`. There is no `ErrAccountArchived`; `handler.go` `writeBankErr` has no mapping; `frontend/src/lib/problems.ts` has no `bank/account-archived`. Debt movements inherit the gap through `CreateTransfer`. | backend: add `ErrAccountArchived` → 409 `bank/account-archived`; check it on create, on a transaction PATCH whose `account_id` names a different account, on both legs of a transfer POST, and on a transfer PATCH only for an account that changes. openapi: 409 on POST/PATCH `/bank/transactions` and `/bank/transfers`. frontend: slug and message in `problems.ts`. test: service cases for all four paths, plus "edit and delete on an archived account still succeed". | F122 |
| 3 | **Data integrity** — P0.2 currency on a transaction PATCH | A PATCH that changes `account_id` to an account of another currency is 422 `bank/currency-mismatch` and the row is unchanged. | `service.go` `UpdateTransaction` calls `repo.GetAccount(ctx, in.UserID, *in.AccountID)`, discards the result and never compares currencies, so the integer amount is silently reinterpreted in the new currency. | backend: load the current and the target account, return `ErrCurrencyMismatch` when the currencies differ; make the `writeBankErr` detail generic (today it reads "cross-currency transfers are not supported"). openapi: 422 on PATCH `/bank/transactions/{id}`. test: VND row moved onto a USD account → 422, row unchanged; same-currency move re-derives both balances. | F123 |
| 4 | **Data integrity** — P0.5 budget write semantics | PUT upserts with `amount > 0`; only `amount: 0` or `null` deletes. | `service.go` `SetBudget` deletes on `amount <= 0`, so a negative amount silently deletes the budget instead of being rejected. | backend: `amount < 0` → `ErrInvalidAmount` (422 `bank/invalid-amount`); keep 0/null → delete. test: PUT −5 → 422 and the budget survives. | Verified 2026-10-01 (no F-ID) |
| 5 | **Data integrity** — §6 follow-up migration `000N_bank_integrity` | `(user_id, created_at DESC)` on `bank_transactions`; `(category_id)` on `bank_budgets`; `CHECK (parent_id IS DISTINCT FROM id)` on `bank_categories`; `CHECK (currency ~ '^[A-Z]{3}$')` on `bank_accounts`. | Not shipped: only `backend/db/migrations/0014_bank_core`, `0020_platform_rls_enable` and `0043_bank_debts` touch these tables. `service.go` `CreateAccount` / `UpdateAccount` check only `len(cur) != 3` after `ToUpper`, so `U$D` is accepted. | migration: new `00NN_bank_integrity` up/down with the four statements (pre-check existing rows for a bad currency or a self-parent). backend: validate currency with `^[A-Z]{3}$` (422 `bank/validation`). test: migration up/down; service rejects `U$D`. | F131 |
| 6 | **Contract** — P0.4 `kind` immutability | A category PATCH that changes `kind` is 422 `bank/category-immutable` and the row is unchanged. | `handler.go` `UpdateCategory` decodes a body without a `kind` field and `server.Decode` ignores unknown keys, so `{"kind":"income"}` answers 200 and changes nothing. `ErrCategoryImmutable` is declared and mapped in `writeBankErr` but never returned. | backend: add `Kind *string` to the PATCH body; a value different from the stored kind → `ErrCategoryImmutable`; an equal value is a no-op. openapi: `kind` on the PATCH schema with the 422. test: TC-BANK-074. | F056 |
| 7 | **Contract** — P0.7 event payload | The payload carries `currency` (the account's ISO code). | `api/api.go` `TransactionEvent` has no `Currency` field; `service.go` `emitTx` builds the payload from the `Transaction` alone. | backend: add a `Currency string` field (JSON `currency`) to `TransactionEvent`; pass the account currency into `emitTx` from the account each write path already loads (`DeleteTransaction` / `DeleteTransfer` must load it before deleting). docs: drop "not yet emitted" from the `events.md` bank rows. The consumer side (stream formats by currency) is SPEC-06's follow-up. test: TC-BANK-141 with `currency`. | F016 |
| 8 | **Contract** — P0.6, P1.11 and §7 month rule, P0.2 "today" | An omitted `month` is the current month in the caller's `users.timezone` (unknown → `Asia/Ho_Chi_Minh`, never UTC); a malformed month is 400 `bank/invalid-month`; `month` is `YYYY-MM` only. The entry date defaults to today in the user's zone. | `handler.go` `monthParam` returns `time.Now().UTC()` (used by `ListBudgets`, `Dashboard`, `Report`); a malformed month is `server.Problem(400, "about:blank", …)` there, in `ListTransactions` (`?month=`) and in `SetBudget` (body `month`); `parseMonth` also accepts `YYYY-MM-DD`. `parseDateDefault` defaults an omitted `occurred_at` to the UTC date. `frontend/src/lib/bank.ts` `currentMonth()` uses `toISOString()` (UTC month) and `today()` uses the device clock. Prerequisite missing: `accountapi` exposes no timezone and `0002_account_users` defaults `timezone` to `'UTC'` (specs README Timezone follow-up). | backend (after the account follow-up ships `UserSummary.Timezone`): give `bank.Deps` a zone lookup through `accountapi`; compute the default month and the default `occurred_at` date in the caller's zone; make `parseMonth` strict `2006-01`; answer every malformed month with 400 `bank/invalid-month`. openapi: the 400 on every `month` parameter (including `getBankReport`, which declares only the 401) and on the PUT body. frontend: `currentMonth()` / `today()` use the zone from `GET /auth/me`; add `bank/invalid-month` to `problems.ts`. test: TC-BANK-125 plus a malformed-month case per endpoint. | F057, F128, Decision 2026-09-30 (Timezone) |
| 9 | **Contract** — §7 list envelopes | `GET /bank/transactions` → `{items, next_cursor?}`; `/bank/accounts` and `/bank/categories` → `{items}`; `/bank/budgets` → `{month, items}`. | `handler.go` `ListTransactions` writes `{transactions, next_cursor}`, `ListAccounts` `{accounts}`, `ListCategories` `{categories}`, `ListBudgets` `{month, budgets}`; `shared/openapi.yaml` declares the same keys; `frontend/src/lib/bank.ts` `listTransactions`, `listAccounts`, `listCategories`, `listBudgets` read them (and `BudgetsView` / `TransactionsView` consume `.budgets` / `.transactions`). | backend + openapi + frontend in one PR: rename each list key to `items`. test: TC-BANK-033, TC-BANK-035 as HTTP tests. | Decision 2026-09-30 (Envelopes) |
| 10 | **Contract** — §7 Problem types (param-shape failures) | A body or parameter shape failure without a named type is 422 `bank/validation` (specs README Pagination rule). | `handler.go` answers malformed `account_id`, `category_id`, `from_account`, `to_account`, `parent_id`, `reassign_to`, the `account` / `category` filters and `occurred_at` with `server.Problem(400, "about:blank", …)`; invalid JSON goes through `server.Decode` → 400. Path ids that fail to parse already answer 404 `bank/not-found` (correct). | backend: route these through `writeBankErr(w, ErrValidation)`; wrap `server.Decode` in a bank helper that answers 422 `bank/validation`. openapi: 422 on the affected operations. test: TC-BANK-200 cases for each parameter. | F024 |
| 11 | **Contract** — P0.5 / P0.6 budget entries | Budget lists (GET `/bank/budgets` and the dashboard block) include synthesized header entries (`amount: null`) for unbudgeted parents of budgeted children, so the tree renders without a client-side join; the dashboard block is filtered server-side to budgets with no budgeted ancestor. | `query/bank.sql` `ListBudgetsForMonth` returns budget rows only (`b.amount` always set; `BudgetLine.Amount` is `int64`). `service.go` `Dashboard` returns the same full list; the no-budgeted-ancestor filter runs client-side in `frontend/…/views/bank/DashboardView.tsx` (`topBudgets`). | backend: synthesize header entries (parent of a budgeted child with no budget row of its own, `amount` NULL); make `BudgetLine.Amount` `*int64`; filter the dashboard block server-side. openapi: `amount` nullable. frontend: drop the client filter. test: TC-BANK-104, TC-BANK-105, TC-BANK-106. | Verified 2026-10-01 (no F-ID) |
| 12 | **Contract** — §7 OpenAPI AuthZ encoding | Each operation is annotated per the specs README AuthZ OpenAPI encoding (one literal code per method). | `shared/openapi.yaml` carries no `x-required-permission` on any `/bank/*` operation. | openapi: annotate every bank operation with the §7 code (`/bank/report` with `bank-transactions:read:own`, as `module.go` gates it). test: TC-BANK-204. | F025 |
| 13 | **UX** — P0.5 budgets page | `/bank/budgets` renders an indented tree (child bars nested under their parent; unbudgeted parents as non-bar headers); the percent shown is `round(spent × 100 / amount)` half-up; the >100 % highlight uses `spent > amount`. | `frontend/…/views/bank/BudgetsView.tsx` builds its own tree from `listCategories`, lists budgeted rows first as a flat list with no indentation or header rows, and shows amounts but no percent (the dashboard bars likewise). Its highlight compares the float `pct > 100`. | frontend: render the server's entries as an indented tree with header rows; show the half-up integer percent; highlight on `spent > amount`. test: TC-BANK-103, TC-BANK-104 (vitest). | Verified 2026-10-01 (no F-ID) |
| 14 | **UX** — P0.2 / P0.3 / §8 corrections | A user corrects a transaction inline; transfer legs render with a transfer badge **and the counterparty account name**. | `frontend/src/lib/bank.ts` has no `updateTransaction` or `updateTransfer`; `TransactionsView.tsx` offers delete only. `TransactionRow` (in `DashboardView.tsx`) labels a leg "Chuyển khoản" but never names the other account; the transaction JSON (`handler.go` `transactionJSON`) carries no counterparty. | frontend: inline edit through PATCH `/bank/transactions/{id}`, and through PATCH `/bank/transfers/{transfer_id}` for a leg. backend + openapi: add `counterparty_account_id` to the transaction response (or the client resolves the sibling leg). test: TC-BANK-022, TC-BANK-053, TC-BANK-057. | Verified 2026-10-01 (no F-ID) |
| 15 | **UX** — P0.1 / P0.6 dashboard archived section | Archived accounts appear in the dashboard's collapsed archived section at their current balance. | `DashboardView.tsx` keeps only `!a.archived && isWallet(a)` and has no archived section (only `AccountsView.tsx` lists archived accounts). | frontend: add the collapsed archived group to `/bank`. test: TC-BANK-004, TC-BANK-123. | Verified 2026-10-01 (no F-ID) |
| 16 | **UX** — P0.2 quick-add defaults | Account defaults to the last used, category to the most recently used, date to today in the user's zone. | `frontend/…/components/bank/QuickAddModal.tsx` defaults the account to the first open wallet (`openAccounts[0]`), leaves the category unset (`null`), and takes the date from the device clock (`today()`). | frontend: remember the last account and MRU category (per-viewer storage, or derive from the dashboard's `recent`); take today from the stored zone (row 8). test: TC-BANK-031. | Verified 2026-10-01 (no F-ID) |
| 17 | **Contract** — §7 transactions list `limit` (owner decision 2026-10-01) | Missing, non-integer or < 1 → 50; above 100 → **clamped to 100**; never a Problem. | `service.go` `ListTransactions`: `if limit <= 0 \|\| limit > maxTxLimit { limit = defaultTxLimit }` — `?limit=500` returns 50 (`handler.go` parses with `server.AtoiSafe`). | backend: clamp instead of resetting (`> 100 → 100`, `≤ 0 → 50`), e.g. `platform/server.Limit(r, 50, 100)`. openapi: describe `limit` as defaulted and clamped. test: TC-BANK-036. | Decision 2026-10-01 (limit) |

Not counted above: P1.10 receipts, P1.12 `bank:budget_exceeded` and P1.13
structured fees are unbuilt P1 features, not divergences.

**Already matching on `99b5a0b`:**

- Ownership and existence: `GetAccount`, `GetTransaction`, `ListTransferLegs`
  and every mutation are owner-scoped; a foreign or unknown id is 404
  `bank/not-found` with a body identical to a missing one
  (`http_test.go: TestHTTPAnotherUsersAccountIsNotFound`). Category reads span
  own + seed (`GetVisibleCategory`); seed mutations match zero rows → 404.
- `transfer_id` is server-assigned only: the transaction POST/PATCH bodies have
  no such field and only `CreateTransfer` mints `uuid.New()`; a manual
  transaction without `category_id` is 422 `bank/validation` before the insert.
- The transfer-leg predicate `NOT (transfer_id IS NOT NULL AND category_id IS
  NULL)` is used by `MonthFlowTotals`, `CategorySpendForMonth` and
  `MonthlyFlowSeries`; budget spent drops legs through the category join and
  includes direct children.
- Transfers: create and update run in one DB transaction
  (`repository/adapter.go` `CreateTransfer` / `UpdateTransfer`), delete is one
  statement; PATCH accepts any subset of `{from_account, to_account, amount,
  occurred_at, note}` and re-runs the POST validations; a leg PATCH/DELETE is
  409 `bank/is-transfer-leg` naming `/bank/transfers/{transfer_id}`; each
  transfer write emits one event per leg with `counterparty_account_id`
  (`emitTransferLegs`).
- Categories: parent must be top-level and the same kind, a category with
  children cannot take a parent, self-parent is 422, delete with
  `?reassign_to=` moves rows in the same transaction and emits nothing.
- Dashboard: balances are always current (`ListAccountBalances`), recent is
  `created_at DESC LIMIT 10`; permissions are the eleven 2–3-segment codes
  seeded by `0014` and wired per route in `module.go`; the §6 indexes and the
  P0.9 partial unique index match `0014`.
- P1.11 report: `Service.Report` returns the month totals (`MonthFlowTotals`),
  parent-rolled slices split by kind and sorted by total (`CategorySpendForMonth`)
  and a six-month zero-filled trend (`MonthlyFlowSeries`, `trendMonths = 6`), all
  filtered by `user_id`; `/report` is gated on `bank-transactions:read:own`.
  Only the month handling diverges (row 8).

**Test evidence to add or fix:**

- No test on HEAD asserts superseded behaviour (`bank_test.go`, `http_test.go`
  pass and stay valid). `docs/testing/TEST-CASES-SPEC-03-bank.md` TC-BANK-141
  still lists the payload without `currency`; update it with row 7.
- `TestDerivedBalanceReconciles` is a fixed two-row case, not the P0.2
  property test: add the random create/edit/delete reconciliation test
  (TC-BANK-024).
- `emitTx` is unexercised: add a fake `EventPublisher` and assert one event per
  transaction write, two per transfer write, zero on a bulk reassign, and the
  full payload including `currency` (TC-BANK-140…143).
- `UpdateTransfer` has no test: the 5,000,000 → 4,000,000 AC (TC-BANK-053).
- New cases for rows 2, 3, 4, 6, 8 and 10: archived 409 on all four write
  paths (no TC row yet — add one under P0.1), transaction PATCH currency
  mismatch (extend TC-BANK-022), negative budget amount (no TC row yet),
  TC-BANK-074, TC-BANK-125, TC-BANK-200.
- P1.11: `bank_test.go: TestReportRollsUpChildrenAndSplitsByKind` proves
  TC-BANK-185 and TC-BANK-187; add the fee-row sum (TC-BANK-186), the month
  default and `bank/invalid-month` over HTTP (TC-BANK-188, with row 8), owner
  isolation (TC-BANK-189) and the composite shape (TC-BANK-190).
- `limit` clamping (row 17): TC-BANK-036.
- HTTP tests for the `{items}` envelopes (TC-BANK-033, TC-BANK-035), the budget
  tree and dashboard roll-up (TC-BANK-104…106), the integrity migration
  (TC-BANK-205) and the dedup index (TC-BANK-180, P0.9 has no test).

## 13. Revision history

- **rev 1 · 2026-07-10** — reconciliation pass: permission codes moved to the
  2–3-segment grammar (`bank-accounts:*` etc., P0.8); the money-model divergence
  from D-14/D-15 documented with the `D-41` ratification vehicle (§7); budget
  write semantics defined (P0.5); dashboard recency keyed on `created_at` (P0.6);
  `user_id` identity-anchor FK added to every table (§6); opening-balance date
  semantics resolved as timeless for v1 (§11).
- **2026-09-30** — spec-gap fixes: account ownership and currency rules on
  transaction/transfer writes, server-assigned `transfer_id` (P0.2); archive
  enforcement (P0.1); transfer PATCH contract (P0.3); `kind`-immutability status
  (P0.4); percent rounding (P0.5); dashboard contract and month default (P0.6);
  two events per transfer write (P0.7); no wildcard bypass (P0.8); receipt module
  boundary (P1.10); §6 indexes aligned to `0014` plus a follow-up integrity
  migration; §7 month rule and Problem types `bank/not-found`,
  `bank/account-archived`, `bank/invalid-month`; §8 categories page.
- **2026-10-01** — Added §12 implementation gaps (self-contained follow-up
  list); revision history renumbered to §13. Owner decisions of 2026-10-01:
  the shipped `GET /bank/report` documented as P1.11 (§7 row, contract, ACs);
  the transactions list's `limit` clamps above 100 (§7, §12 row 17).
