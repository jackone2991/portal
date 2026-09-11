# SPEC-10 — Ledger expansion (module `bank`: debts, goals, recurring, cards, net worth, automation, splits, sharing)

**Status:** phase 1 building, rev 1 · **Drafted:** 2026-09-11
**Module:** `bank` (extends it; no new module) · **Depends on:** SPEC-03 (the ledger this builds on), SPEC-04 (notify, for reminders), SPEC-01 (media, only for P1.10 receipts)
**Refs:** SPEC-03 §5 P0.3 (the transfer-leg predicate everything here keys on), migration 0042 (icon-first categories)

---

## 1. Problem statement

SPEC-03 shipped a Money-Lover-class ledger: accounts, categories, transactions,
transfers, budgets, and a monthly report. It tracks **what you spent**. It cannot
answer **what you are worth**, **what you owe**, **what is coming**, or **what you
are saving for** — and those four questions are the reason people keep a ledger
past the first month.

Eight areas were requested. They are not one feature; they are eight, three of
them touching the transaction model itself. This spec sequences them, fixes the
one data-model decision that most of them depend on, and states the accounting
invariant each must not break.

## 2. The invariant every phase is measured against

SPEC-03 P0.3 established that reporting keys on **leg-ness**, not on the presence
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
dashboard), and SPEC-03's own warning applies: diverge in one place and the donut
stops summing to the month total.

What falls out of the account model for free:

| Need | How it is already solved |
|---|---|
| Outstanding balance of a debt | `bank_accounts` balance is **derived** from transactions (SPEC-03 P0.1) — it cannot drift |
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
  in `cmd/worker` (there is no OS cron in this stack).

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
SPEC-03 P0.9 already shipped the schema (`import_batch_id`, `dedup_hash`,
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

## 5. Non-goals (and why)

- **Bank API integration / open banking.** No credentials in a self-hosted app
  (SPEC-03's own reasoning); manual + import only.
- **Live market prices.** Phase 5 records valuations you enter. A price feed is an
  outbound dependency with rate limits and a contact policy — the MusicBrainz
  lesson — and is not worth it for a monthly net-worth line.
- **Multi-currency FX.** Accounts already carry a currency and transfers refuse to
  cross it (SPEC-03). Debts and goals inherit that refusal rather than inventing
  conversion.
- **Amortisation schedules** beyond simple/compound projection. A mortgage
  planner is a product; this is a ledger.

## 6. API summary (phase 1 only; later phases extend)

| Method | Path | Notes |
|---|---|---|
| GET/POST | `/api/v1/bank/debts` | `bank-debts:read/write:own` |
| GET/PATCH/DELETE | `/api/v1/bank/debts/{id}` | delete refuses while the balance is non-zero |
| POST | `/api/v1/bank/debts/{id}/movements` | `{kind: borrow\|lend\|repay\|collect, account_id, amount, occurred_at}` → a transfer pair |
| POST | `/api/v1/bank/debts/{id}/accrue` | posts interest as a categorised expense |

## 7. Done means

Phase 1 is done when: a borrow of 10,000,000 ₫ raises a wallet balance, creates a
liability of the same size, and moves **neither** income nor expense for the
month; a repayment reduces both; an accrual appears as an expense in the donut;
and the month totals still equal the sum of the report's slices.
