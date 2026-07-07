# 03 — Finance Ledger (module `bank`, ledger scope)

**Module:** `bank` (name reserved in diagrams/MODULES; no code yet).
**Ref:** [feature.md §8](../feature.md) — this spec implements a **subset** (§8.1–8.2 core).
**Depends on:** nothing hard; receipt attachments reuse spec 01.
**Requires:** ADR-08 (spec 00) — amends ADR-01, which currently defers "bank" wholesale.

## Problem statement

The money facet of the life OS. Scope agreed in brainstorm: **Money-Lover-class
personal ledger** — multiple accounts, manual transactions, categories, monthly
budgets, transfers — with a schema that is **import-ready from day one** even though
statement import itself is deferred (TCB exports PDF → needs OCR; see
[04-deferred.md](04-deferred.md)).

Key insight from the brainstorm: a self-hosted manual ledger holds **no bank
credentials**, so the "MFA before bank" rule (D-27/D-28) does **not** gate this
scope. TOTP becomes the unlock condition for *real bank integration* later.

## Goals

- The dev records daily spending across ≥3 real accounts (e.g. TCB, cash, Momo)
  for a full month without hitting a wall.
- Transfers between own accounts never distort income/expense reports.
- Every transaction emits a bus event — the money facet feeds the life stream from day one.
- The schema absorbs a future statement import **without a breaking migration**.

## Non-goals (each with why)

- **Statement import (any format)** — deferred; TCB=PDF needs OCR (see 04). Schema readiness only.
- **Debts, loans, investments, savings goals** (§8.3–8.5) — separate iterations; ledger first.
- **Splits, recurring transactions, tags** — P2 below; core loop works without them.
- **Multi-currency FX reporting** — each account has a currency; cross-currency
  totals are out (no FX-rate infrastructure at v1). Single-currency users (VND) unaffected.
- **Real bank connections / TOTP** — explicitly the *next* unlock, not this spec.

## User stories

- As the user, I create accounts of different types (cash, checking, e-wallet,
  credit card) with opening balances, so my ledger mirrors reality.
- As the user, I record an expense in seconds — amount, account, category, note —
  because friction kills daily logging.
- As the user, I record a transfer TCB → Momo once, and it changes both balances
  without appearing as income or expense.
- As the user, I set a monthly budget per category and see progress against it.
- As the user, I archive a closed account without losing its history.

## Requirements

### P0 — must have

1. **Accounts**: type (cash|checking|savings|credit_card|ewallet|other), currency
   (default VND), opening balance, active/archived. Balance is **derived**
   (opening + Σ transactions), never a mutable stored field.
   - [ ] Archiving hides the account from pickers but keeps history and reports intact.
2. **Transactions (manual)**: amount (BIGINT minor units — VND minor unit = 1;
   render via a Money helper, never float), direction debit|credit, account,
   category, occurred_at, note. Import-ready columns present from migration #1:
   `description_raw text null`, `import_batch_id uuid null`, `dedup_hash text null`.
   - [ ] Create/edit/delete a transaction updates the account's derived balance correctly.
   - [ ] Entry form ≤ 4 required fields; category picker defaults to most-recently-used.
3. **Transfers**: modeled as a **paired debit+credit sharing `transfer_id`**;
   excluded from income/expense reports; included in per-account balances.
   Editing/deleting one leg edits/deletes the pair atomically.
   - [ ] A 5,000,000 VND transfer changes both balances and moves monthly
         income/expense totals by exactly 0.
4. **Categories**: hierarchical (2 levels is enough), seeded Vietnamese-relevant
   defaults (Ăn uống, Di chuyển, Hóa đơn, Lương…), user-extensible.
5. **Monthly budgets**: amount per category per month; dashboard shows spent/budget.
6. **Dashboard**: per-account balances, current-month income/expense, budget bars.
7. **Events**: emit `bank:transaction_created` (and `bank:transaction_deleted`) on
   the bus from day one.
8. **RBAC**: all data strictly owner-scoped (`bank:transaction:*:own` etc.);
   there is no cross-user read at any permission level except explicit admin wildcard.
9. **`import_batches` table exists** (id, source, file_name, created_at, status) —
   empty until the import feature lands; `import_batch_id` references it. Cheap now,
   painful to retrofit.

### P1 — nice to have

10. Receipt attachment on a transaction (image asset via `mediaapi` — spec 01).
11. Simple monthly report page (per-category breakdown, month-over-month).
12. `bank:budget_exceeded` event when a category crosses 100%.

### P2 — future considerations (architectural insurance)

13. Splits (one transaction, many categories) — keep the door open: report queries
    should aggregate via a category join, not assume 1 category column forever.
14. Recurring transactions (drafts the user confirms).
15. Tags.
16. **Statement import** (see 04): generic CSV/xlsx + column-mapping templates as
    data-not-code; per-bank presets; dedup via `dedup_hash`
    (account + date + amount + ref); batch rollback via `import_batch_id`.

## Data model sketch (next free migrations, `000N_bank_*`)

```
bank_accounts(id, user_id, name, type, currency char(3) default 'VND',
              opening_balance bigint, archived bool, created_at)
bank_categories(id, user_id null /* null = seeded default */, parent_id null,
                name, kind check in ('income','expense'))
bank_transactions(id, user_id, account_id fk, category_id fk null,
                  amount bigint check (amount > 0), direction check in ('debit','credit'),
                  transfer_id uuid null, occurred_at date, note text,
                  description_raw text null, import_batch_id uuid null,
                  dedup_hash text null, created_at, updated_at)
  -- partial unique index on (account_id, dedup_hash) where dedup_hash is not null
bank_budgets(id, user_id, category_id fk, month date /* first of month */,
             amount bigint, unique(user_id, category_id, month))
bank_import_batches(id, user_id, source, file_name, status, created_at)
```

Transfers carry `category_id = null` and `transfer_id = shared uuid` on both legs.

## API sketch

```
GET/POST           /api/v1/bank/accounts        PATCH /accounts/{id} (archive)
GET/POST           /api/v1/bank/transactions    ?account=&month=&category=
PATCH/DELETE       /api/v1/bank/transactions/{id}
POST               /api/v1/bank/transfers        {from,to,amount,occurred_at,note}
GET/POST           /api/v1/bank/categories
GET/PUT            /api/v1/bank/budgets          ?month=
GET                /api/v1/bank/dashboard        ?month=
```

## Success signal (n=1 honest metrics)

- Leading: the dev logs transactions on ≥20 of the first 30 days (friction test).
- Lagging: month-end balances reconcile with real accounts within rounding;
  if reconciliation is painful, the transfer/edit model is wrong — fix before adding features.

## Open questions

- **(product, non-blocking)** Credit-card accounts: v1 treats them as a plain
  account with negative balance; statement-cycle logic (due dates) is future.
- **(engineering, blocking)** Frontend surface: new `(bank)` route group — which
  Olympus screens/templates map to it (frontend.md Phase 5 lists `<MoneyDisplay />`,
  `<MoneyInput />`)? Decide before building the dashboard.
- **(product, non-blocking)** Should the life stream show amounts, or just
  "logged 3 transactions today" (privacy-on-screen)? Decide when the stream lands.
