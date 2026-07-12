-- bank module queries (SPEC-03). sqlc input only — regenerate with `make sqlc`;
-- never hand-edit the *.sql.go output. Money is integer minor units (bigint).
-- Every row is owner-scoped by user_id (P0.8), with one carve-out: category
-- READS span own + seed (user_id IS NULL); category MUTATIONS stay owner-scoped
-- (which is what makes seeds immutable/undeletable, P0.4).

-- ══ Accounts (P0.1) ═════════════════════════════════════════════════════

-- name: CreateAccount :one
INSERT INTO bank_accounts (user_id, name, type, currency, opening_balance)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetAccount :one
SELECT * FROM bank_accounts WHERE id = $1 AND user_id = $2;

-- name: ListAccounts :many
-- All of the caller's accounts (archived included; the service/handler hides
-- archived from pickers). Ordered active-first then name.
SELECT * FROM bank_accounts WHERE user_id = $1 ORDER BY archived, name;

-- name: UpdateAccount :one
-- Partial update of {name, archived}. currency is immutable (P0.1) so it is not
-- updatable here. NULL arg leaves the column unchanged.
UPDATE bank_accounts
SET name     = COALESCE(sqlc.narg('name'), name),
    archived = COALESCE(sqlc.narg('archived'), archived),
    currency = COALESCE(sqlc.narg('currency'), currency)
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteAccount :one
-- Owner-scoped. The service refuses when the account still has transactions
-- (409 bank/account-not-empty); the plain FK is the DB backstop.
DELETE FROM bank_accounts WHERE id = $1 AND user_id = $2 RETURNING id;

-- name: CountAccountTransactions :one
SELECT count(*) FROM bank_transactions WHERE account_id = $1;

-- name: GetAccountBalance :one
-- Derived balance = opening_balance + Σcredits − Σdebits (never stored, P0.1).
-- ::bigint keeps sqlc from inferring int4 (VND balances overflow int32).
SELECT (a.opening_balance
     + COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'credit'), 0)
     - COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'debit'), 0))::bigint AS balance
FROM bank_accounts a
LEFT JOIN bank_transactions t ON t.account_id = a.id
WHERE a.id = $1 AND a.user_id = $2
GROUP BY a.id, a.opening_balance;

-- name: ListAccountBalances :many
-- Every account with its current derived balance (dashboard P0.6 + list). Always
-- current — never month-scoped (§11 opening-balance rule).
SELECT a.id, a.name, a.type, a.currency, a.opening_balance, a.archived, a.created_at,
       (a.opening_balance
     + COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'credit'), 0)
     - COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'debit'), 0))::bigint AS balance
FROM bank_accounts a
LEFT JOIN bank_transactions t ON t.account_id = a.id
WHERE a.user_id = $1
GROUP BY a.id
ORDER BY a.archived, a.currency, a.name;

-- ══ Categories (P0.4) ═══════════════════════════════════════════════════

-- name: CreateCategory :one
INSERT INTO bank_categories (user_id, parent_id, name, kind)
VALUES ($1, sqlc.narg('parent_id'), $2, $3)
RETURNING *;

-- name: GetVisibleCategory :one
-- Own or seed (user_id IS NULL). Used for attach validation (P0.2) and GET; a
-- foreign category id resolves to no row → 404 (existence never leaks).
SELECT * FROM bank_categories
WHERE id = $1 AND (user_id = $2 OR user_id IS NULL);

-- name: ListCategories :many
-- Own + seed, ordered so parents precede their children for tree assembly.
SELECT * FROM bank_categories
WHERE user_id = $1 OR user_id IS NULL
ORDER BY kind, COALESCE(parent_id, id), (parent_id IS NOT NULL), name;

-- name: UpdateCategory :one
-- Owner-scoped (seeds have user_id NULL so they never match → immutable, P0.4).
-- kind is immutable; only {name, parent_id} change. A NULL name leaves it; the
-- service passes parent_id explicitly (clearing to top-level is a real value).
UPDATE bank_categories
SET name      = COALESCE(sqlc.narg('name'), name),
    parent_id = CASE WHEN @set_parent::boolean THEN sqlc.narg('parent_id') ELSE parent_id END
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteCategory :one
-- Owner-scoped delete (seeds never match). Children are promoted to top level by
-- the ON DELETE SET NULL FK; budgets cascade. The service handles reassignment
-- of this category's own transactions in the same tx before calling this.
DELETE FROM bank_categories WHERE id = $1 AND user_id = $2 RETURNING id;

-- name: CountCategoryTransactions :one
SELECT count(*) FROM bank_transactions WHERE category_id = $1 AND user_id = $2;

-- name: CountCategoryChildren :one
SELECT count(*) FROM bank_categories WHERE parent_id = $1;

-- name: ReassignCategoryTransactions :exec
-- Move this category's own transactions to the target (P0.4 delete ?reassign_to).
UPDATE bank_transactions
SET category_id = @target_id, updated_at = now()
WHERE category_id = @from_id AND user_id = @user_id;

-- ══ Transactions (P0.2 / P0.3) ══════════════════════════════════════════

-- name: CreateTransaction :one
INSERT INTO bank_transactions (
    user_id, account_id, category_id, amount, direction, transfer_id, occurred_at, note
) VALUES (
    $1, $2, sqlc.narg('category_id'), $3, $4, sqlc.narg('transfer_id'), $5, sqlc.narg('note')
)
RETURNING *;

-- name: GetTransaction :one
SELECT * FROM bank_transactions WHERE id = $1 AND user_id = $2;

-- name: ListTransactionsByUserCursor :many
-- Keyset page, newest first (occurred_at DESC, id DESC). Optional account/month/
-- category filters (a NULL arg skips its clause). A NULL cursor starts at the top.
SELECT * FROM bank_transactions
WHERE user_id = @user_id
  AND (@account_id::uuid IS NULL OR account_id = @account_id::uuid)
  AND (@category_id::uuid IS NULL OR category_id = @category_id::uuid)
  AND (@month::date IS NULL OR date_trunc('month', occurred_at)::date = @month::date)
  AND (
        @cursor_occurred_at::date IS NULL
        OR occurred_at < @cursor_occurred_at::date
        OR (occurred_at = @cursor_occurred_at::date AND id < @cursor_id::uuid)
      )
ORDER BY occurred_at DESC, id DESC
LIMIT @lim::int;

-- name: UpdateTransaction :one
-- Owner-scoped partial update. The service blocks transfer legs (409) before
-- calling this, so it only ever edits ordinary rows.
UPDATE bank_transactions
SET account_id  = COALESCE(sqlc.narg('account_id'), account_id),
    category_id = COALESCE(sqlc.narg('category_id'), category_id),
    amount      = COALESCE(sqlc.narg('amount'), amount),
    direction   = COALESCE(sqlc.narg('direction'), direction),
    occurred_at = COALESCE(sqlc.narg('occurred_at'), occurred_at),
    note        = COALESCE(sqlc.narg('note'), note),
    updated_at  = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteTransaction :one
DELETE FROM bank_transactions WHERE id = $1 AND user_id = $2 RETURNING id;

-- name: RecentTransactions :many
-- Dashboard: the 10 most recently ENTERED rows (created_at DESC — a future-dated
-- entry must not pin the list, P0.6).
SELECT * FROM bank_transactions
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2;

-- ══ Transfers (P0.3) ════════════════════════════════════════════════════

-- name: ListTransferLegs :many
-- All rows sharing a transfer_id, owner-scoped (the pair, plus any P1.13 fee row).
SELECT * FROM bank_transactions
WHERE transfer_id = $1 AND user_id = $2
ORDER BY direction;

-- name: DeleteTransferByID :exec
-- Remove every row of a transfer atomically (both legs, plus a fee row if any).
DELETE FROM bank_transactions WHERE transfer_id = $1 AND user_id = $2;

-- ══ Budgets (P0.5) ══════════════════════════════════════════════════════

-- name: UpsertBudget :one
-- One amount per (category, month). amount > 0 (the §6 CHECK); sending 0/null is
-- a delete, handled by the service via DeleteBudget.
INSERT INTO bank_budgets (user_id, category_id, month, amount)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id, category_id, month)
DO UPDATE SET amount = EXCLUDED.amount
RETURNING *;

-- name: DeleteBudget :exec
DELETE FROM bank_budgets WHERE user_id = $1 AND category_id = $2 AND month = $3;

-- name: ListBudgetsForMonth :many
-- Each budgeted category for the month with its amount, name, parent (id+name for
-- header synthesis), and spent = Σ expense debits in the month for the category
-- INCLUDING its direct children, excluding pure transfer legs (a leg has
-- category_id NULL so the category join already drops it; a P1.13 fee row keeps
-- its category and counts — reporting keys on leg-ness, not transfer_id).
SELECT b.category_id,
       c.parent_id,
       c.name       AS name,
       p.name       AS parent_name,
       b.amount     AS budget_amount,
       COALESCE((
           SELECT SUM(t.amount)
           FROM bank_transactions t
           JOIN bank_categories cc ON t.category_id = cc.id
           WHERE t.user_id = b.user_id
             AND t.direction = 'debit'
             AND date_trunc('month', t.occurred_at)::date = b.month
             AND (cc.id = b.category_id OR cc.parent_id = b.category_id)
       ), 0)::bigint AS spent
FROM bank_budgets b
JOIN bank_categories c ON c.id = b.category_id
LEFT JOIN bank_categories p ON p.id = c.parent_id
WHERE b.user_id = $1 AND b.month = $2
ORDER BY COALESCE(c.parent_id, c.id), (c.parent_id IS NOT NULL), c.name;

-- ══ Dashboard (P0.6) ════════════════════════════════════════════════════

-- name: MonthFlowTotals :one
-- Month income/expense totals, excluding pure transfer legs (transfer_id set AND
-- category_id NULL) — NOT `WHERE transfer_id IS NULL`, which would wrongly drop a
-- P1.13 fee row. A fee row (both set) counts as an ordinary expense.
SELECT
    COALESCE(SUM(amount) FILTER (
        WHERE direction = 'credit' AND NOT (transfer_id IS NOT NULL AND category_id IS NULL)), 0)::bigint AS income,
    COALESCE(SUM(amount) FILTER (
        WHERE direction = 'debit'  AND NOT (transfer_id IS NOT NULL AND category_id IS NULL)), 0)::bigint AS expense
FROM bank_transactions
WHERE user_id = $1 AND date_trunc('month', occurred_at)::date = $2;
