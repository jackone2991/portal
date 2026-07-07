# SPEC-03 — Finance Ledger (module `bank`, ledger scope)

**Module:** `bank` (name reserved in diagrams/MODULES; no code) · **Depends on:** ADR-08 (scope amendment); SPEC-01 only for P1 receipts
**Upstream:** `feature/03-finance-ledger.md` · **Refs:** feature.md §8 (implements a subset of §8.1–8.2), frontend.md Phase 5

---

## 1. Problem statement

The money facet of the life OS. Agreed scope: a **Money-Lover-class personal
ledger** — multiple accounts, manual transactions, hierarchical categories, monthly
budgets, and inter-account transfers — with a schema that is **import-ready from
day one**, even though statement import itself is deferred (owner's bank, TCB,
exports PDF → import means OCR; see `feature/04-deferred.md`).

Scope insight that unblocks this now: a self-hosted **manual** ledger holds no bank
credentials, so the "MFA before bank" gate (D-27/D-28) does not apply. TOTP becomes
the named unlock for *real bank integration* later. ADR-01 currently defers "bank"
wholesale — **ADR-08 must land (or be drafted in the same PR) to make this scope
legitimate.**

## 2. Goals

1. The dev logs daily spending across ≥3 real accounts (TCB, cash, Momo) for a full
   month without friction walls — entry of a routine expense takes < 10 s.
2. Transfers between own accounts never distort income/expense reporting (the #1
   correctness trap identified in the brainstorm).
3. Every transaction mutation emits a bus event — the money facet feeds the life
   stream from day one.
4. A future statement-import feature lands **without a breaking migration**
   (columns, dedup key, and batch table exist now).
5. Month-end reconciliation against real accounts closes within rounding — the
   honest success test of the data model.

## 3. Non-goals (each with rationale)

- **Statement import (any format)** — deferred (TCB=PDF→OCR). Schema readiness only
  (§6). Pre-agreed future design: generic CSV/xlsx + column-mapping templates stored
  as *data not code*, per-bank presets, `dedup_hash` dedup, per-batch rollback.
- **Debts, loans, investments, savings goals** (feature.md §8.3–8.5) — separate
  iterations with their own models; ledger core first. Re-entry: ledger reconciles
  cleanly ≥1 month.
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
Σ(debits)` — never a stored mutable column. Credit cards at v1 are ordinary
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

Entry UX: quick-add dialog reachable from anywhere in the `(bank)` group; ≤4
required fields (amount, direction defaulted to expense, account defaulted to last
used, category defaulted to most-recently-used); date defaults to today;
`MoneyInput` renders VND thousands separators (`1.500.000`).

**Acceptance criteria.**
- Given create/edit/delete of a transaction, then the account's derived balance and
  the month's totals are correct (property test: random sequences reconcile).
- Given amount ≤ 0 or a fractional VND amount, then 422.
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
are rejected (422). Cross-currency transfers are rejected at v1 (422
`bank/currency-mismatch`) — revisit with FX.

**Acceptance criteria.**
- Given a 5,000,000 VND transfer TCB→Momo, then TCB −5M, Momo +5M, and the month's
  income and expense totals each move by exactly 0.
- Given deletion of a transfer, then both legs disappear atomically (no orphan leg
  under any failure — covered by a transaction-rollback test).
- Given a filtered transaction list, then transfer legs render with a distinct
  transfer badge and the counterparty account name.

### P0.4 — Categories

Hierarchical, 2 levels max, `kind` `income|expense`. Seeded Vietnamese-context
defaults (owner's locale), user-extensible:
*expense:* Ăn uống (Đi chợ, Ăn ngoài, Cà phê), Di chuyển (Xăng xe, Grab/Taxi,
Gửi xe), Hóa đơn (Điện, Nước, Internet, Điện thoại), Nhà cửa, Mua sắm, Sức khỏe,
Giải trí, Giáo dục, Khác. *income:* Lương, Thưởng, Quà tặng, Lãi, Thu nhập khác.
Seeds have `user_id NULL`; users may add their own (and rename/hide seeds is P2 —
at v1 seeds are fixed, users add alongside). A category with transactions cannot be
deleted; the API offers reassignment (`?reassign_to=`) — without it, 409.

### P0.5 — Monthly budgets

One amount per (category, month); `month` stored as first-of-month date. Spent =
Σ expense debits in the month for the category **including its children**,
excluding transfer legs. Dashboard shows spent/budget bars with >100% highlighted.
Budgets do not roll over (rollover is future).

**Acceptance criteria.**
- Given a budget of 3,000,000 on Ăn uống and 3,200,000 spent across its children,
  then the bar shows 107% and is highlighted.
- Given no budget on a category, then it simply doesn't appear in the budget list
  (no zero-division states).

### P0.6 — Dashboard

`GET /bank/dashboard?month=` returns, grouped by currency: per-account derived
balances (active; archived collapsed), month income/expense totals (transfers
excluded), budget progress list, and the 10 most recent transactions. Frontend
`/bank` renders it as the `(bank)` group landing page.

### P0.7 — Events

Emit on the bus: `bank:transaction_created`, `bank:transaction_updated`,
`bank:transaction_deleted` — payload `{transaction_id, user_id, account_id,
amount, direction, category_id, occurred_at, is_transfer}`. Emitted for transfer
legs too (with `is_transfer=true`). No consumer required to ship.

### P0.8 — RBAC

All rows carry `user_id`; every query is owner-scoped. Permissions:
`bank:account:*:own`, `bank:transaction:*:own`, `bank:category:*:own`,
`bank:budget:*:own` granted to the base user role. **No cross-user read at any
permission level except explicit admin wildcard** — and even admin access should be
considered deliberately (finance data is the most sensitive in the system; flag in
ADR-08's consequences).

### P0.9 — Import scaffolding (schema only)

`bank_import_batches` table exists (empty until the feature lands);
`import_batch_id` references it; partial unique index on `(account_id, dedup_hash)
WHERE dedup_hash IS NOT NULL` guards future re-imports. Cheap now, painful to
retrofit — this is the whole point of doing it in migration #1.

### P1 — nice to have

- **P1.10 Receipt attachments**: `receipt_asset_id` on a transaction (image asset
  via `mediaapi`; SPEC-01). Thumbnail in the transaction row; lightbox on click.
- **P1.11 Monthly report page**: per-category breakdown (donut or bars) +
  month-over-month comparison; per currency group.
- **P1.12 `bank:budget_exceeded`** event, emitted once per (category, month) on
  first crossing 100% (dedup via a small state table or cache key).

### P2 — future considerations (architectural insurance)

- **Splits**: report queries must aggregate via a join on a (transaction→category)
  relation conceptually — don't bake "exactly one category column" into report SQL
  shapes more than necessary.
- **Recurring transactions**: generated as drafts the user confirms (feature.md
  §8.2); implies a `status` concept — don't preclude adding a status column.
- **Tags**; **statement import** (design pre-agreed, see Non-goals).

## 6. Data model — migration `000N_bank_core`

```sql
CREATE TABLE bank_accounts (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL,
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
  user_id   uuid,                          -- NULL = seeded default
  parent_id uuid REFERENCES bank_categories(id),
  name      text NOT NULL,
  kind      text NOT NULL CHECK (kind IN ('income','expense'))
);

CREATE TABLE bank_import_batches (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id    uuid NOT NULL,
  source     text NOT NULL,
  file_name  text,
  status     text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE bank_transactions (
  id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id         uuid NOT NULL,
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
  user_id     uuid NOT NULL,
  category_id uuid NOT NULL REFERENCES bank_categories(id),
  month       date NOT NULL CHECK (date_trunc('month', month) = month),
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
| GET/POST | `/api/v1/bank/accounts` | `bank:account:read/create:own` |
| PATCH/DELETE | `/api/v1/bank/accounts/{id}` | `bank:account:update/delete:own` |
| GET/POST | `/api/v1/bank/transactions?account=&month=&category=&page=` | `bank:transaction:read/create:own` |
| PATCH/DELETE | `/api/v1/bank/transactions/{id}` | `bank:transaction:update/delete:own`; 409 on transfer legs |
| POST | `/api/v1/bank/transfers` | `bank:transaction:create:own` |
| PATCH/DELETE | `/api/v1/bank/transfers/{transfer_id}` | `bank:transaction:update/delete:own` |
| GET/POST | `/api/v1/bank/categories` (`?reassign_to=` on delete) | `bank:category:*:own` |
| GET/PUT | `/api/v1/bank/budgets?month=` | `bank:budget:*:own` |
| GET | `/api/v1/bank/dashboard?month=` | `bank:account:read:own` |

Problem types: `bank/account-not-empty`, `bank/account-not-mutable`,
`bank/is-transfer-leg`, `bank/currency-mismatch`, `bank/category-in-use`,
`bank/invalid-amount`.

## 8. Frontend (`(bank)` route group — resolves the blocking question)

New route group; no Olympus template maps cleanly to finance, so pages are composed
from the existing shell + `components/ui` primitives plus Phase-5 money components
(frontend.md): `<MoneyDisplay />`, `<MoneyInput />` (string-amount-aware, VND
separators), Recharts for budget bars/report.

- `/bank` — dashboard (P0.6), quick-add button (global within the group)
- `/bank/transactions` — filterable infinite list; edit/delete inline; transfer badge
- `/bank/accounts` — list + create/archive
- `/bank/budgets` — month picker + per-category budget editor

RSC-first shells; the quick-add dialog and lists are client islands. Left-menu
entry added to the shell nav.

## 9. Success metrics (n=1 honest)

- Leading: transactions logged on ≥20 of the first 30 days (friction test) —
  observable straight from the table.
- Leading: median quick-add duration < 10 s (log client-side timing during dogfood).
- Lagging: month-end reconciliation vs real accounts closes within rounding. **If
  reconciliation is painful, the transfer/edit model is wrong — fix before adding
  any P1/P2 feature.**

## 10. Timeline & phasing

1. ADR-08 draft + migration + seeds + sqlc (1 day)
2. Accounts + transactions CRUD + derived balances + RBAC + OpenAPI (1.5 days)
3. Transfers (paired semantics + tests) (1 day)
4. Budgets + dashboard endpoint (1 day)
5. Frontend: quick-add, transactions list, dashboard, accounts (2 days)
6. Events + polish (½ day)
P0 ≈ 7 dev-days — the largest of the three specs; do not start P1 before the first
reconciliation succeeds.

## 11. Open questions

- **(product, non-blocking)** Life-stream privacy: should `bank:*` events carry
  amounts, or only counts ("logged 3 transactions today")? Payload above carries
  amounts; the *consumer* (notification module) decides display — revisit there.
- **(product, non-blocking)** Opening-balance date semantics: v1 treats opening
  balance as timeless (applies before all transactions). Fine for n=1; document it.
- **(engineering, non-blocking)** Derived-balance performance: SUM over an index is
  fine for years of personal data; add a materialized running balance only if the
  dashboard ever exceeds budget (measure first).
