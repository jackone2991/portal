# SPEC-03 — Finance Ledger (module `bank`, ledger scope)

**Status:** ready to build, rev 1 · **Drafted / last-verified:** 2026-07-10
**Module:** `bank` (name reserved in diagrams/MODULES; no code) · **Depends on:** ADR-08 (scope amendment); SPEC-01 only for P1 receipts
**Upstream:** [briefs/03-finance-ledger.md](../briefs/03-finance-ledger.md) · **Refs:** feature-inventory.md §8 (implements a subset of §8.1–8.2 plus monthly budgets from §8.7), frontend.md Phase 5

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

**Acceptance criteria.**
- Given an archived account, then it is absent from transaction-entry pickers but
  present in historical reports and the dashboard's archived section.
- Given an account with 1 transaction, when currency change or DELETE is attempted,
  then 409 Problem `bank/account-not-mutable` / `bank/account-not-empty`.

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

Entry UX: quick-add dialog reachable from anywhere in the `(bank)` group; ≤4
required fields (amount, direction defaulted to expense, account defaulted to last
used, category defaulted to most-recently-used); date defaults to today;
`MoneyInput` renders VND thousands separators (`1.500.000`).

**Acceptance criteria.**
- Given create/edit/delete of a transaction, then the account's derived balance and
  the month's totals are correct (property test: random sequences reconcile).
- Given amount ≤ 0 or a fractional VND amount, then 422 `bank/invalid-amount`.
- Given a debit attached to an income-kind category (or a credit to an
  expense-kind one), then 422 `bank/direction-kind-mismatch`.
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
`bank/currency-mismatch`) — revisit with FX.

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
category ids. `kind` is immutable after creation (mirror of P0.1's currency
immutability). `parent_id` is mutable via PATCH, but a category that has children
cannot be assigned a parent (422 `bank/invalid-category-parent`) — together with
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

### P0.5 — Monthly budgets

One amount per (category, month); `month` stored as first-of-month date. Spent =
Σ expense debits in the month for the category **including its children**,
excluding transfer legs (leg predicate defined in P0.3). Dashboard shows
spent/budget bars with >100% highlighted. Budgets do not roll over (rollover is
future).

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

`GET /bank/dashboard?month=` returns, grouped by currency: per-account derived
balances (active; archived collapsed) — always **current** balances; `month` scopes
only the flow numbers and budget progress, never back-projects balances (see the
§11 opening-balance note) — month income/expense totals (**transfer legs**
excluded — the P0.3 predicate, *not* `WHERE transfer_id IS NULL`, which would
wrongly drop P1.13 fee rows), budget progress list (the P0.5
**no-budgeted-ancestor** roll-up — *not* a `parent_id IS NULL` filter, which would
drop child-only budgets; the full nested tree lives on `/bank/budgets`), and the
10 most recently **entered** transactions (`created_at DESC` — keying recency on
`occurred_at` would let one future-dated entry pin the list; resolved
2026-07-10). Frontend `/bank` renders it as the `(bank)` group landing page.

**Acceptance criteria.**
- Given VND-only accounts, then the dashboard returns one currency group; given
  `?month=2026-06`, then only flow totals and budget bars change — balances stay
  current (the §11 opening-balance rule).
- Given an archived account with history, then it appears in the archived
  section at its current balance and its transactions still count in past
  months' flows.

### P0.7 — Events

Emit on the bus: `bank:transaction_created`, `bank:transaction_updated`,
`bank:transaction_deleted` — payload `{transaction_id, user_id, account_id,
amount, direction, category_id, occurred_at, is_transfer, transfer_id,
counterparty_account_id}`.
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

`bank-accounts:read|write|delete:own`, `bank-transactions:read|write|delete:own`,
`bank-categories:read|write|delete:own`, `bank-budgets:read|write:own` — all
granted to the base `user` role; the bank migration seeds the permission rows
and grants (0003 pattern). `write` covers create + update (+ archive for
accounts; transfers are transaction writes). **No cross-user read at any
permission level except explicit admin wildcard** — and even admin access should be
considered deliberately (finance data is the most sensitive in the system; flag in
ADR-08's consequences).

**Acceptance criteria.**
- Given user B's accounts/transactions/categories/budgets, then user A's list
  endpoints return none of them and direct fetches are 404.
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

- **P1.10 Receipt attachments**: `receipt_asset_id` on a transaction (image asset
  via `mediaapi`; SPEC-01). Thumbnail in the transaction row; lightbox on click.
- **P1.11 Monthly report page**: per-category breakdown (donut or bars) +
  month-over-month comparison; per currency group.
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
CREATE INDEX ON bank_transactions (user_id, occurred_at DESC);
CREATE INDEX ON bank_transactions (account_id, occurred_at DESC);
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

Category seeds ship as a data migration in the same sequence. `dedup_hash` (future)
= sha256 of `account_id|occurred_at|amount|direction|bank_ref` — computed only by
the import path.

## 7. API summary (add to `shared/openapi.yaml`)

| Method | Path | Permission |
|---|---|---|
| GET/POST | `/api/v1/bank/accounts` | `bank-accounts:read/write:own` |
| PATCH/DELETE | `/api/v1/bank/accounts/{id}` | `bank-accounts:write/delete:own` |
| GET/POST | `/api/v1/bank/transactions?account=&month=&category=&cursor=` | `bank-transactions:read/write:own` |
| PATCH/DELETE | `/api/v1/bank/transactions/{id}` | `bank-transactions:write/delete:own`; 409 on transfer legs |
| POST | `/api/v1/bank/transfers` | `bank-transactions:write:own` |
| PATCH/DELETE | `/api/v1/bank/transfers/{transfer_id}` | `bank-transactions:write/delete:own` |
| GET/POST | `/api/v1/bank/categories` | `bank-categories:read/write:own` |
| PATCH/DELETE | `/api/v1/bank/categories/{id}` (`?reassign_to=` on DELETE) | `bank-categories:write/delete:own`; 404/409/422 per P0.4 |
| GET/PUT | `/api/v1/bank/budgets?month=` | `bank-budgets:read/write:own` (PUT upserts; `amount: 0\|null` deletes — P0.5) |
| GET | `/api/v1/bank/dashboard?month=` | `bank-accounts:read:own` |

*(Codes reconciled 2026-07-10 to the 2–3-segment grammar — see P0.8.)*

The transactions list paginates by **cursor**, not offset `?page=` (which
duplicates/skips rows under inserts and `occurred_at` edits): ordering is
`occurred_at DESC, id DESC` and the response carries a `next_cursor` field —
matching SPEC-01's cursor convention and §8's infinite-list requirement.

Problem types: `bank/account-not-empty`, `bank/account-not-mutable`,
`bank/is-transfer-leg`, `bank/same-account-transfer`, `bank/currency-mismatch`,
`bank/category-in-use`, `bank/category-kind-mismatch`,
`bank/direction-kind-mismatch`, `bank/invalid-amount`,
`bank/invalid-category-parent` (P0.4's non-top-level / wrong-kind / 2-level
violations), `bank/category-immutable` (P0.4's kind-immutability).

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

**Ratification vehicle** *(ADR-08 landed without carrying this — the original
plan is stale)*: record both divergences as a **new decision entry (propose
`D-41`) in feature-inventory.md's resolved list**, scoped "v1 personal-ledger
only; D-14/D-15 stand for any multi-currency or creator-economy money", and
reconcile frontend.md §5.3 + its Phase-5 component notes — all in the SPEC-03
implementation PR.

## 8. Frontend (`bank` pages under the `(app)` group — resolves the blocking question)

Pages live under the existing authenticated group —
`app/(app)/bank/{page,transactions,accounts,budgets}/page.tsx` — so they inherit
the `(app)` shell/nav and the login gate; there is **no** separate `(bank)` route
group (only `(app)` and `(public)` exist, per CLAUDE.md). No Olympus template maps
cleanly to finance, so pages are composed
from the existing shell + `components/ui` primitives plus Phase-5 money components
(frontend.md): `<MoneyDisplay />`, `<MoneyInput />` (display-layer string handling
with VND thousands separators; the wire carries integer minor units per §7),
Recharts for budget bars/report.

- `/bank` — dashboard (P0.6), quick-add button (global within the group)
- `/bank/transactions` — filterable infinite list; edit/delete inline; transfer badge
- `/bank/accounts` — list + create/archive
- `/bank/budgets` — month picker + per-category budget editor

RSC-first shells; the quick-add dialog and lists are client islands. Left-menu
entry added to the shell nav.

**Auth gate.** Add `'/bank/:path*'` to `config.matcher` in
`frontend/src/middleware.ts` so the D-34 session gate runs on the new routes and
redirects an unauthenticated visitor to `/login`.

**Template registry.** Every new page view is declared in `TemplateManifest.views`
(`frontend/src/templates/types.ts`), implemented under
`templates/v1/views/...`, and each `app/(app)/bank/<route>/page.tsx` resolves it
via `activeTemplate().views.<x>` — never a version-specific import in `app/` (keeps
the `v2` switch intact).

## 9. Success metrics (n=1 honest)

- Leading: transactions logged on ≥20 of the first 30 days (friction test) —
  observable straight from the table.
- Leading: median quick-add duration < 10 s (log client-side timing during dogfood).
- Lagging: month-end reconciliation vs real accounts closes within rounding. **If
  reconciliation is painful, the transfer/edit model is wrong — fix before adding
  any P1/P2 feature.**

## 10. Timeline & phasing

1. D-41 decision entry (money-model divergence, §7) + migration + seeds + sqlc (1 day)
2. Accounts + transactions CRUD + attachment validation (P0.2) + derived
   balances + RBAC + OpenAPI (1.5 days)
3. Categories: CRUD, hierarchy invariants, delete/reassign matrix (P0.4) (1 day)
4. Transfers (paired semantics + fee-convention tests) (1 day)
5. Budgets + dashboard endpoint (incl. the no-budgeted-ancestor roll-up) (1 day)
6. Frontend: quick-add, transactions list, dashboard, accounts, budgets tree
   (2.5 days)
7. Events + polish (½ day)
P0 ≈ 8.5 dev-days — the largest of the three specs (category semantics and the
budget tree grew it past the original 7); do not start P1 before the first
reconciliation succeeds.

## 11. Open questions

- **(product, non-blocking)** Life-stream privacy: should `bank:*` events carry
  amounts, or only counts ("logged 3 transactions today")? Payload above carries
  amounts; the *consumer* (notification module) decides display — revisit there.
- **(product, resolved for v1)** Opening-balance date semantics: timeless (applies
  before all transactions) — correct for current balances and monthly flow totals,
  which is all v1 computes (`opening_balance` never enters a flow sum, so P0.6
  totals and P1.11 month-over-month are unaffected). **Technical note for future
  trend charts** (net worth over time): a timeless opening balance back-projects
  today's opening amount into months before the account was tracked — a false
  plateau. A historical series must start each account's line at
  `LEAST(created_at::date, MIN(occurred_at))` (cast in the user's timezone;
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

## 12. Revision history

- **rev 1 · 2026-07-10** — reconciliation pass: permission codes moved to the
  2–3-segment grammar (`bank-accounts:*` etc., P0.8); the money-model divergence
  from D-14/D-15 documented with the `D-41` ratification vehicle (§7); budget
  write semantics defined (P0.5); dashboard recency keyed on `created_at` (P0.6);
  `user_id` identity-anchor FK added to every table (§6); opening-balance date
  semantics resolved as timeless for v1 (§11).
