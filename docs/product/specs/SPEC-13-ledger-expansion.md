# SPEC-13 — Ledger expansion (module `bank`: debts, goals, recurring, cards, net worth, automation, splits, sharing)

**Status:** phase 1 building, rev 1 · **Drafted:** 2026-09-11 · **Last verified:** 2026-10-01
**Module:** `bank` (extends it; no new module) · **Depends on:** SPEC-12 (the ledger this builds on), SPEC-05 (notify, for reminders), SPEC-04 (media, only for P1.10 receipts)
**Refs:** SPEC-12 §5 P0.3 (the transfer-leg predicate everything here keys on), migration 0042 (icon-first categories)

---

## 1. Problem statement

SPEC-12 shipped a Money-Lover-class ledger: accounts, categories, transactions,
transfers, budgets, and a monthly report. It tracks **what you spent**. It cannot
answer **what you are worth**, **what you owe**, **what is coming**, or **what you
are saving for** — and those four questions are the reason people keep a ledger
past the first month.

Eight areas were requested. They are not one feature; they are eight, three of
them touching the transaction model itself. This spec sequences them, fixes the
one data-model decision that most of them depend on, and states the accounting
invariant each must not break.

## 2. The invariant every phase is measured against

SPEC-12 P0.3 established that reporting keys on **leg-ness**, not on the presence
of `transfer_id`:

```sql
-- a PURE leg: moved money, not spending
NOT (transfer_id IS NOT NULL AND category_id IS NULL)
```

Moving money between your own wallets is not spending; a transfer **fee** (both
columns set) is. Every feature below moves money that is **not** income or
expense — borrowing, repaying, allocating to a savings jar, buying shares — and
every one of them must land on the correct side of that predicate.

**The failure mode is silent.** A loan of 10,000,000 ₫ miscounted as income does
not error; it inflates one month's income for ever, and the donut still sums.
That is why this spec exists before the code.

## 3. The decision that unblocks phases 1, 2 and 5

> **A debt, a savings jar, and an investment holding are all *accounts*.**
> They get a `bank_accounts` row and their money moves by ordinary transfer.

The alternative — a `debt_id` column on `bank_transactions` plus a widened
exclusion predicate — was rejected. It would mean editing the leg predicate in
every reporting query (`MonthFlowTotals`, the report donut, budget spend,
dashboard), and SPEC-12's own warning applies: diverge in one place and the donut
stops summing to the month total.

What falls out of the account model for free:

| Need | How it is already solved |
|---|---|
| Outstanding balance of a debt | `bank_accounts` balance is **derived** from transactions (SPEC-12 P0.1) — it cannot drift |
| Borrowing is not income | it is a transfer leg: `category_id IS NULL` ⇒ already excluded |
| Interest **is** an expense | a normal categorised transaction — exactly the fee-row precedent |
| Net worth (phase 5) | `Σ asset accounts − Σ liability accounts`, both already derived |
| Repaying picks a target | the debt is in the account picker, because it is an account |

New `bank_accounts.type` values: `loan_payable` (I owe), `loan_receivable` (owed
to me), `savings_goal`, `investment`. The type decides the **sign convention** in
net worth and which pickers show it — nothing else changes.

A debt is more than a balance, so the terms live beside it:

```sql
bank_debts(id, user_id, account_id →bank_accounts, counterparty, direction,
           principal, interest_rate_bps, interest_method, opened_on, due_on,
           closed_at, note)
```

`account_id` is 1:1 and `NOT NULL`: the terms and the money are one thing, and a
debt without an account would have a balance nobody derives.

## 4. Phases, in build order

Order is by **dependency and blast radius**, not by appeal. The three that change
the transaction model come last, because everything else is additive.

### Phase 1 — Debts & loans (requirement 1) · *building*

- Record who owes whom, principal, rate, due date; balance derived, never stored.
- Four money movements, all transfers, all invisible to income/expense:
  borrow / lend (principal out or in) and repay / collect.
- Interest: simple and compound **projected** in the UI; an explicit *accrue*
  action posts a real categorised expense (seed category *Lãi vay*), because an
  accrual you cannot see in the ledger is an accrual you cannot reconcile.
- Due reminders through `notify:dispatch`, driven by the existing Asynq scheduler
  in `cmd/worker` (there is no OS cron in this stack) — the
  `bank:scan_debts_due` sweep, §4a (5).

### Phase 2 — Savings goals & vaults (requirement 2)

Target amount + date; a `savings_goal` account holds the allocation. Allocating
is a transfer from a wallet, so **no fake expense is invented** — the requirement's
own wording. Progress = derived balance ÷ target. Un-allocating is the reverse
transfer.

*Open question to settle at build:* whether an allocated jar should reduce the
wallet's spendable balance shown on the dashboard, or sit beside it. Reducing it
is honest and is what "ủy thác số dư" implies; it also surprises anyone who opens
their bank app and sees a different number.

### Phase 3 — Credit-card cycle (requirement 4)

`bank_accounts` already has `type='credit_card'`. Add statement day, due day,
credit limit, and (optional) cashback rate; derive the current statement window,
available credit, and the interest-free deadline. Reminders reuse phase 1's
scheduler hook. No new money mechanics — a card is already a negative-balance
account.

### Phase 4 — Recurring & subscriptions (requirement 3)

A schedule table (rrule-lite: every N days/weeks/months, optional end) that the
scheduler materialises into **draft** transactions the user confirms, not silent
posts — a ledger that writes rows you did not approve is a ledger you stop
trusting. Cash-flow projection then = derived balances + scheduled rows over the
next 30/60/90 days, with phase 1's due dates and phase 3's card dues folded in.

### Phase 5 — Investments & net worth (requirement 5)

`investment` accounts hold cost basis; a valuation history table records mark-to-
market points (manual entry — no price feed; that would be the first outbound
dependency in the ledger). Net worth chart = the phase-3 decision's free lunch:
`Σ assets − Σ liabilities` sampled per month.

### Phase 6 — Splits & tags (requirement 7) · *invasive*

Tags are additive and cheap (`bank_tags` + a join table) and could land earlier;
they are grouped here because **splits are not**. A split turns one transaction
into a parent plus N category legs, which changes what every reporting query
sums over. Done wrong it double-counts the month. Requires re-deriving
`MonthFlowTotals`, the report rollup, and budget spend against split legs, with
tests that assert the parent is never counted alongside its legs.

### Phase 7 — Automation & rules (requirement 6)

Rule engine first (`contains "Grab" → category Di chuyển`), because it is pure
and testable and improves manual entry immediately. **CSV/PDF import second** —
SPEC-12 P0.9 already shipped the schema (`import_batch_id`, `dedup_hash`,
`description_raw`) for exactly this. **SMS/notification capture is out of scope
until a mobile client exists**: there is no Android/iOS app in this repo, and
reading notifications requires one. Saying so here is cheaper than discovering it
mid-build.

### Phase 8 — Shared / household ledgers (requirement 8) · *architectural*

Deliberately last. Every bank table is `user_id`-scoped and every query filters on
the caller; sharing replaces that with membership, which is a rewrite of the
module's access model, not a feature on top of it. It also overlaps the existing
`tenant` module (organizations, memberships, RLS) — the right move is likely to
express a shared ledger as an organization rather than invent a second membership
system, and that deserves an ADR before any code.

## 4a. Event policy

SPEC-12 P0.7 emits one `bank:transaction_*` event per written row. The new write
paths here would flood the stream, project unconfirmed drafts, or double-count
splits if they followed it literally, so each states its policy:

1. **Debt movements** (borrow / lend / repay / collect) are ordinary transfer
   pairs through the SPEC-12 transfer path and emit
   `bank:transaction_created|updated|deleted` per P0.7 (`is_transfer=true`, a
   shared `transfer_id`, so SPEC-09 collapses them into one item). An **accrual**
   is a normal categorised expense and emits per P0.7. (Shipped.)
2. **Recurring** (phase 4): a draft emits nothing. Confirming a draft emits one
   `bank:transaction_created`; discarding one emits nothing.
3. **Splits** (phase 6): only the parent emits. Creating, editing or deleting
   any leg emits one `bank:transaction_created|updated|deleted` for the parent;
   legs never emit.
4. **Import** (phase 7): import-batch rows emit no per-row `bank:transaction_*`
   event — the same carve-out rationale as SPEC-12 P0.7's bulk reassign (one user
   action, not N mutations). The batch emits one `bank:import_completed
   {import_batch_id, user_id, account_id, row_count}` after commit. It has no
   stream consumer (the stream projects moments); the bell may consume it later.
5. **Due reminders** (phases 1 and 3): the periodic task
   `bank:scan_debts_due` runs **hourly** on the shared scheduler in
   `cmd/worker`, on the `default` queue, once per tenant via `forEachTenant`.
   "Days away" is counted from **each owner's local date** in that owner's
   `users.timezone`, read through `accountapi` (specs README Timezone, D-17's
   hourly per-TZ pattern; unknown → `Asia/Ho_Chi_Minh`), and an owner is
   notified only once their local time has reached 07:00. For each
   lead of 7, 1 and 0 days it notifies the owner of every open debt with a
   non-zero balance whose `due_on` is exactly that many days away, through
   `notify:dispatch` (type `bank.debt_due`), with
   `dedup_key = <debt_id>|<due_on>|<lead>`; `bank_debt_reminders` records each
   sent (debt, due date, lead). A day the sweep misses skips that lead (no
   catch-up). Phase 3's card dues reuse the sweep, keyed on the card account id.
   Hourly re-runs are safe: the `dedup_key` and `bank_debt_reminders` make
   each (debt, due date, lead) fire at most once. (Shipped for debts. *Code
   follow-up: HEAD runs the sweep once a day at 07:00 UTC and counts days from
   the UTC date for every owner.*)

## 5. Non-goals (and why)

- **Bank API integration / open banking.** No credentials in a self-hosted app
  (SPEC-12's own reasoning); manual + import only.
- **Live market prices.** Phase 5 records valuations you enter. A price feed is an
  outbound dependency with rate limits and a contact policy — the MusicBrainz
  lesson — and is not worth it for a monthly net-worth line.
- **Multi-currency FX.** Accounts already carry a currency and transfers refuse to
  cross it (SPEC-12). Debts and goals inherit that refusal rather than inventing
  conversion.
- **Amortisation schedules** beyond simple/compound projection. A mortgage
  planner is a product; this is a ledger.

## 6. API summary (phase 1 only; later phases extend)

| Method | Path | Permission | Notes |
|---|---|---|---|
| GET | `/api/v1/bank/debts` | `bank-transactions:read:own` | non-paginated; returns `{items: [...]}` (specs README Pagination; *code follow-up: HEAD answers `{debts: [...]}`*) |
| POST | `/api/v1/bank/debts` | `bank-transactions:write:own` | `{counterparty, direction: borrowed\|lent, principal, interest_rate_bps, interest_method, opened_on, due_on?, note?, wallet_id}` → 201 the debt; opens the debt account and moves the principal through `wallet_id` |
| GET | `/api/v1/bank/debts/{id}` | `bank-transactions:read:own` | |
| PATCH | `/api/v1/bank/debts/{id}` | `bank-transactions:write:own` | |
| DELETE | `/api/v1/bank/debts/{id}` | `bank-transactions:delete:own` | refuses while the balance is non-zero |
| POST | `/api/v1/bank/debts/{id}/movements` | `bank-transactions:write:own` | `{kind: borrow\|lend\|repay\|collect, wallet_id, amount, occurred_at?, note?}` → 201 `{legs: [Transaction, Transaction]}`, a transfer pair (a write result, not a list endpoint) |
| POST | `/api/v1/bank/debts/{id}/accrue` | `bank-transactions:write:own` | posts interest as a categorised expense |

**`wallet_id`** *(owner decision 2026-10-01; the shipped name)*: the user's own
**non-debt** account the money moves through — the wallet the principal leaves
(lend, repay) or lands in (borrow, collect). It is deliberately not called
`account_id`, which names the debt's **own** account (§3, `bank_debts.account_id`).
It must be one of the caller's accounts (else 404 `bank/not-found`), not an
account of type `loan_payable` or `loan_receivable` (else 422 `bank/validation`)
and in the debt's currency (else 422 `bank/currency-mismatch`; on POST
`/bank/debts` the wallet fixes the new debt's currency). `occurred_at` defaults to
today in the owner's `users.timezone` (specs README Timezone; row 2); `note` is
copied to both legs. A malformed `wallet_id` is 422
`bank/validation` (row 5).

One literal code per method (specs README AuthZ). Debts reuse the
`bank-transactions:*` codes, as shipped (`bank/module.go`); there is no
`bank-debts:*` code, because a debt is an account whose money moves by transfer,
and migration `0043_bank_debts` therefore seeds no new permission. A later phase
that adds a non-transaction surface seeds `bank-<noun>:read|write|delete:own` →
`user` in its own migration (specs README seeding rule). Annotate each operation
per the specs README AuthZ **OpenAPI encoding**.

## 7. Done means

Phase 1 is done when: a borrow of 10,000,000 ₫ raises a wallet balance, creates a
liability of the same size, and moves **neither** income nor expense for the
month; a repayment reduces both; an accrual appears as an expense in the donut;
and the month totals still equal the sum of the report's slices.

## 8. Implementation gaps vs shipped code (as of 2026-10-01)

The baseline is `main` @ `99b5a0b` (the commits after it changed docs only).
The spec text above is the target; each row below is a place where the shipped
phase 1 (debts and loans) still diverges from it, verified against that commit.
Rows are ordered by severity: data integrity first, then scheduling, then API
contract. A row closes when the code matches the spec and the SPEC-13 row of
`docs/reference/TRACEABILITY-MATRIX.md` is regraded. File paths are relative to
`backend/internal/modules/bank/` unless they start with `backend/`, `frontend/`
or `shared/`. Gaps in the SPEC-12 paths a debt reuses (archive check, event
`currency`, month and date defaults) are tracked once, in SPEC-12 §12.

| # | Requirement (§) | Spec requires | Shipped code today (file / function) | Change needed (migration · backend · openapi · frontend · test) | Source |
|---|---|---|---|---|---|
| 1 | **Data integrity** — §3 (the terms and the money are one thing), §7 | A debt is its account plus its terms: `account_id` is 1:1 and `NOT NULL`, and opening a debt moves the principal, so a borrow of 10,000,000 ₫ always raises the wallet and creates a liability of the same size. | `debts.go` `Service.CreateDebt` runs three independent writes: `repo.CreateAccount` (the `loan_payable` / `loan_receivable` account), `debts.CreateDebt` (the terms row), then `moveDebt` → `CreateTransfer` (its own DB transaction). Nothing encloses them, so a failure after the first leaves a liability account with no terms, and a failure after the second leaves a debt with no principal movement and a zero balance. | backend: one DB transaction for the account insert, the terms insert and both principal legs (an adapter method such as `CreateDebtWithPrincipal` in `repository/debts_adapter.go`), with the two `bank:transaction_created` events emitted after commit. test: a fake that fails at each step leaves no account, no debt and no legs. | Verified 2026-10-01 (no F-ID) |
| 2 | **Scheduling** — §4a (5) due reminders; §6 movement date default | `bank:scan_debts_due` runs **hourly**; "days away" is counted from each owner's local date in `users.timezone` (read through `accountapi`; unknown → `Asia/Ho_Chi_Minh`); an owner is notified only once their local time has reached 07:00. An omitted movement `occurred_at` is today in the owner's zone. | `backend/cmd/worker/main.go` registers the task with cron `0 7 * * *` (daily, 07:00 UTC). `module.go` `RegisterTasks` passes `time.Now()`; `debts.go` `ScanDueDebts` sets `today := now.UTC().Truncate(24 * time.Hour)` for every owner and asks `DebtsDueBetween` for one exact date per lead across the whole tenant. `debts.go` `moveDebt` defaults an omitted `occurred_at` to `time.Now().UTC()`. Prerequisite missing: `accountapi` exposes no timezone (specs README Timezone follow-up). | backend: register `0 * * * *`; per owner, resolve the zone through `accountapi` (`UserSummary.Timezone`), skip the owner until local 07:00, and compute each lead against the owner's local date (widen the `DebtsDueBetween` window by a day each side, then filter per owner). The dedup key and `bank_debt_reminders` stay as they are, so hourly re-runs fire each (debt, due date, lead) once. `moveDebt` takes today from the owner's zone. docs: drop the code follow-up from the `events.md` row. test: an owner at UTC+7 at 2026-06-30 23:30 UTC (07-01 06:30 local) gets nothing; at 2026-07-01 00:30 UTC (07:30 local) the 7-day lead for a debt due 2026-07-08 fires once, and a second run sends nothing. | Decision 2026-09-30 (Timezone) |
| 3 | **Contract** — §6 `GET /bank/debts` envelope | Non-paginated list answers `{items: [...]}`. | `debts_handler.go` `ListDebts` writes `{"debts": out}`; `frontend/src/lib/bank.ts` `listDebts` reads `r.debts`. | backend + openapi (row 5) + frontend in one PR: rename the key to `items`. test: HTTP test on the list shape. | Decision 2026-09-30 (Envelopes) |
| 4 | **Contract** — §6 `wallet_id` is a non-debt account | `wallet_id` (on `POST /bank/debts` and on movements) names one of the caller's own accounts that is **not** `loan_payable` / `loan_receivable`; a debt account there is 422 `bank/validation`. | `debts.go` `CreateDebt` loads the wallet with `repo.GetAccount` and checks only ownership (and takes its currency); `moveDebt` passes `walletID` straight to `CreateTransfer`, which refuses only the debt's own account (`ErrSameAccountTransfer`) and a currency mismatch. Another debt's account is accepted as the wallet, so a repayment can move principal from one debt into another without touching any wallet. (The body field names already match §6: `debts_handler.go`, `frontend/src/lib/bank.ts` `addDebtMovement` and `DebtsView.tsx` use `wallet_id`.) | backend: load the wallet in `moveDebt` (and use the account `CreateDebt` already loads) and return `ErrValidation` when its type is a loan type. openapi: with row 5. test: a movement and a create naming a loan account → 422, nothing written. | Decision 2026-10-01 (`wallet_id`); found while documenting it (no F-ID) |
| 5 | **Contract** — §6 OpenAPI and Problem convention | Every §6 operation is in `shared/openapi.yaml`, annotated per the specs README AuthZ OpenAPI encoding; a body or parameter shape failure is 422 `bank/validation` (specs README Pagination rule). | `shared/openapi.yaml` has no `/bank/debts` path at all, so the seven routes in `module.go` are undocumented. `debts_handler.go` answers a malformed `wallet_id` or date with `server.BadRequest` (400 `about:blank`). The shipped debt Problem types (`bank/debt-not-settled`, `bank/debt-closed`, `bank/nothing-to-accrue`, `bank/debt-no-interest`) are missing from `frontend/src/lib/problems.ts`. | openapi: add the seven operations with schemas (`{items}` list, the §6 bodies with `wallet_id`, the movement's `{legs}` result) and one `x-required-permission` each, as in the §6 table. backend: shape failures → `writeBankErr(w, ErrValidation)`. frontend: add the four slugs and messages to `problems.ts`. test: handler↔OpenAPI drift check; 422 on a malformed id. | F025, F024 |

**Already matching on `99b5a0b`:**

- §3 account model: migration `0043_bank_debts` adds the `loan_payable` and
  `loan_receivable` types and `bank_debts.account_id UUID NOT NULL UNIQUE`
  (`ON DELETE CASCADE`); the outstanding balance is derived
  (`query/debts.sql` `DebtOutstanding`, the same arithmetic as
  `ListAccountBalances`), never stored.
- Phase 1 money movements: `moveDebt` sends borrow / lend / repay / collect
  through `Service.CreateTransfer`, so every principal movement is a pure
  transfer pair, excluded from income and expense by the SPEC-12 leg predicate,
  and emits per SPEC-12 P0.7 (§4a (1)). A kind that does not match the debt's
  direction is 422 `bank/validation`; the wallet fixes the debt's currency.
- Accrual: `AccrueInterest` posts a categorised transaction on the seed
  *Lãi vay* (expense, borrowed) or *Lãi* (income, lent), charged from the last
  accrual, and refuses a closed debt (409) or an empty period (409
  `bank/nothing-to-accrue`).
- `DELETE /bank/debts/{id}` refuses while the outstanding balance is non-zero
  (409 `bank/debt-not-settled`).
- §6 permissions: `module.go` gates the debt routes on
  `bank-transactions:read|write|delete:own` per method; `0043` seeds no new
  permission.
- §4a (5) apart from cadence and zone: leads 7, 1 and 0 days, exact-day
  matching (a missed day skips that lead), `notify:dispatch` type
  `bank.debt_due`, `dedup_key = <debt_id>|<due_on>|<lead>`,
  `bank_debt_reminders` keyed on (debt, due date, lead), closed and settled
  debts skipped, once per tenant via `ForEachTenant` on the `default` queue.

**Test evidence to add or fix:**

- On HEAD, `debts_test.go: TestAccrueInterest, TestOutstandingFrom` cover the
  arithmetic only; neither asserts superseded behaviour.
- There is no TEST-CASES document for SPEC-13 yet (TRACEABILITY-MATRIX: "no
  case document yet"); create `docs/testing/TEST-CASES-SPEC-13-ledger.md` and
  give each test below an ID.
- §7 "Done means" as one service test: borrow 10,000,000 ₫ → wallet +10M,
  liability −10M, month income and expense +0; a repayment reduces both; an
  accrual appears as an expense; month totals equal the sum of the report's
  slices.
- `CreateDebt` atomicity (row 1), the hourly per-zone sweep (row 2), movement
  kind/direction mismatch, `nothing-to-accrue`, delete refused while
  outstanding, the `{items}` / `wallet_id` HTTP contract (rows 3 and 5) and the
  non-debt wallet rule (row 4).
