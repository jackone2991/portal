-- bank debt queries (SPEC-10 phase 1, migration 0043). sqlc input only.
--
-- The money is NOT here: it lives in the linked bank_accounts row and its
-- transactions. These statements own the TERMS and nothing else, which is why
-- there is no "outstanding" column to keep in step with anything.

-- name: CreateDebt :one
INSERT INTO bank_debts (
    user_id, account_id, counterparty, direction, principal,
    interest_rate_bps, interest_method, opened_on, due_on, note
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, sqlc.narg('due_on'), sqlc.narg('note'))
RETURNING *;

-- name: GetDebt :one
SELECT * FROM bank_debts WHERE id = $1 AND user_id = $2;

-- name: ListDebts :many
-- Open first, then by due date (undated last), then newest. A settled debt is
-- kept — its transactions are real history — so it sorts to the bottom rather
-- than disappearing.
SELECT * FROM bank_debts
WHERE user_id = $1
ORDER BY (closed_at IS NOT NULL), due_on NULLS LAST, opened_on DESC, id;

-- name: UpdateDebt :one
-- Terms only. The principal is immutable after creation: it is the agreed
-- amount, and editing it would silently rewrite what "đã trả 6/10" means for
-- every payment already recorded.
UPDATE bank_debts
SET counterparty      = COALESCE(sqlc.narg('counterparty'), counterparty),
    interest_rate_bps = COALESCE(sqlc.narg('interest_rate_bps'), interest_rate_bps),
    interest_method   = COALESCE(sqlc.narg('interest_method'), interest_method),
    due_on            = CASE WHEN @set_due_on::boolean THEN sqlc.narg('due_on') ELSE due_on END,
    note              = CASE WHEN @set_note::boolean THEN sqlc.narg('note') ELSE note END,
    closed_at         = CASE WHEN @set_closed::boolean THEN sqlc.narg('closed_at') ELSE closed_at END,
    updated_at        = now()
WHERE id = @id AND user_id = @user_id
RETURNING *;

-- name: DeleteDebt :one
-- The linked account cascades (0043). The service refuses this while the
-- balance is non-zero — deleting money you still owe is not a UI affordance.
DELETE FROM bank_debts WHERE id = $1 AND user_id = $2 RETURNING account_id;

-- name: DebtOutstanding :one
-- What is still owed, DERIVED from the linked account exactly as a wallet
-- balance is (SPEC-03 P0.1) — same arithmetic as ListAccountBalances, so the two
-- can never disagree about what a movement did. Sign is normalised by the caller
-- against `direction`.
SELECT (a.opening_balance
      + COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'credit'), 0)
      - COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'debit'), 0))::bigint AS balance
FROM bank_accounts a
LEFT JOIN bank_transactions t ON t.account_id = a.id
WHERE a.id = $1
GROUP BY a.id;

-- name: DebtsDueBetween :many
-- Reminder sweep: open, dated debts falling due in a window, minus the ones a
-- reminder has already gone out for at this lead time. No user filter: the
-- worker runs inside ONE tenant's scope at a time (ForEachTenant), and RLS is
-- what bounds the rows — the same fence a request gets.
SELECT d.* FROM bank_debts d
WHERE d.closed_at IS NULL
  AND d.due_on IS NOT NULL
  AND d.due_on BETWEEN @from_on::date AND @to_on::date
  AND NOT EXISTS (
      SELECT 1 FROM bank_debt_reminders r
      WHERE r.debt_id = d.id AND r.due_on = d.due_on AND r.lead_days = @lead_days::int
  );

-- name: MarkDebtReminded :exec
-- Idempotent by primary key: an hourly sweep must not announce the same debt
-- twenty-four times a day.
-- tenant_id is omitted on purpose: the sweep runs inside a tenant scope, so the
-- column DEFAULT resolves it, exactly like every other write in a request.
INSERT INTO bank_debt_reminders (debt_id, due_on, lead_days)
VALUES ($1, $2, $3)
ON CONFLICT (debt_id, due_on, lead_days) DO NOTHING;

-- name: SeedIncomeInterestCategory :one
-- The income side of an accrual: interest owed TO me. 0042 already seeds 'Lãi',
-- so there is nothing to create — only to find.
SELECT id FROM bank_categories
WHERE user_id IS NULL AND parent_id IS NULL AND name = 'Lãi' AND kind = 'income'
LIMIT 1;

-- name: SeedInterestCategory :one
-- The shared 'Lãi vay' seed (0043). Returned so an accrual has a category to
-- attach to without the caller hard-coding an id.
SELECT id FROM bank_categories
WHERE user_id IS NULL AND parent_id IS NULL AND name = 'Lãi vay' AND kind = 'expense'
LIMIT 1;

-- name: LastAccrualOn :one
-- When interest was last posted on this debt, or NULL if never.
--
-- Derived, not stored: on a debt's own account a CATEGORISED transaction can
-- only be an accrual, because every principal movement is a transfer leg and
-- those carry no category. Accruing from opened_on every time would charge the
-- same period twice on the second accrual.
SELECT MAX(occurred_at)::date AS last_on
FROM bank_transactions
WHERE account_id = $1 AND category_id IS NOT NULL;
