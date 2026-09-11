-- 0043_bank_debts: debts and loans (SPEC-10 phase 1).
--
-- The money lives in a bank_accounts row; this table holds the TERMS.
--
-- Why an account and not a `debt_id` column on bank_transactions: reporting keys
-- on leg-ness (SPEC-03 P0.3) — `NOT (transfer_id IS NOT NULL AND category_id IS
-- NULL)`. A borrow is a transfer leg, so it is already excluded from income and
-- expense by the predicate that exists, in every query that already uses it. The
-- alternative meant widening that predicate in MonthFlowTotals, the report
-- rollup, budget spend and the dashboard, and SPEC-03's own warning applies:
-- diverge in one place and the donut stops summing to the month total.
--
-- The outstanding balance is therefore DERIVED, exactly like a wallet's, and
-- cannot drift from the transactions that produced it.

-- Two new account types. `loan_payable` = money I owe, `loan_receivable` = money
-- owed to me. The type decides the sign in net worth (SPEC-10 phase 5) and which
-- pickers list it — nothing else in the ledger changes.
ALTER TABLE bank_accounts DROP CONSTRAINT IF EXISTS bank_accounts_type_check;
ALTER TABLE bank_accounts ADD CONSTRAINT bank_accounts_type_check CHECK (type IN
    ('cash','checking','savings','credit_card','ewallet','other',
     'loan_payable','loan_receivable'));

CREATE TABLE bank_debts (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- 1:1 and NOT NULL: the terms and the money are one thing. A debt without an
    -- account would have a balance nobody derives.
    account_id  UUID NOT NULL UNIQUE REFERENCES bank_accounts(id) ON DELETE CASCADE,

    counterparty TEXT NOT NULL CHECK (char_length(btrim(counterparty)) BETWEEN 1 AND 120),
    -- 'borrowed' = I owe them; 'lent' = they owe me. Mirrors the account type and
    -- is kept here too so a query about terms never has to join to learn it.
    direction   TEXT NOT NULL CHECK (direction IN ('borrowed','lent')),

    -- The agreed principal, in minor units. This is the ORIGINAL amount, not the
    -- outstanding one — outstanding is derived from the account. Keeping it lets
    -- the UI say "đã trả 6/10 triệu" without replaying history.
    principal   BIGINT NOT NULL CHECK (principal > 0),

    -- Annual rate in basis points (1% = 100), so no floating point reaches the
    -- database. 0 is the common case: money lent to a friend.
    interest_rate_bps INTEGER NOT NULL DEFAULT 0 CHECK (interest_rate_bps BETWEEN 0 AND 1000000),
    interest_method   TEXT NOT NULL DEFAULT 'none' CHECK (interest_method IN ('none','simple','compound')),

    opened_on   DATE NOT NULL,
    due_on      DATE,
    -- Set when the user closes it out. A settled debt is kept, not deleted: its
    -- transactions are real history and deleting the account would orphan them.
    closed_at   TIMESTAMPTZ,
    note        TEXT,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- A rate without a method (or the reverse) is a half-written term that the
    -- projection would silently read as zero.
    CONSTRAINT bank_debts_rate_needs_method
        CHECK ((interest_method = 'none') = (interest_rate_bps = 0))
);

CREATE INDEX bank_debts_user_idx ON bank_debts (user_id, closed_at, due_on);

-- Reminder bookkeeping: which (debt, due date) has already been announced, so a
-- scheduler that runs hourly does not notify the same debt twenty-four times a
-- day. Sent rows are never updated, only inserted.
CREATE TABLE bank_debt_reminders (
    debt_id  UUID NOT NULL REFERENCES bank_debts(id) ON DELETE CASCADE,
    due_on   DATE NOT NULL,
    -- How many days before due_on this reminder was for (0 = on the day).
    lead_days INTEGER NOT NULL,
    sent_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (debt_id, due_on, lead_days)
);

-- ── seed the interest category, so an accrual has somewhere to land ──────
-- Owned by nobody (user_id NULL) like every other seed, and only if it is
-- missing — re-running must not create a second one.
-- tenant_id is written EXPLICITLY as NULL, not omitted. The column is nullable
-- for exactly this reason (0020 keeps the shared seed taxonomy tenant-less), but
-- its DEFAULT calls current_setting('app.current_tenant') — which a migration
-- session does not have, so omitting the column fails the whole migration.
INSERT INTO bank_categories (user_id, tenant_id, parent_id, name, kind, icon, color)
SELECT NULL, NULL, NULL, 'Lãi vay', 'expense', '💸', '#dc2626'
WHERE NOT EXISTS (
    SELECT 1 FROM bank_categories WHERE user_id IS NULL AND parent_id IS NULL AND name = 'Lãi vay'
);

-- ── tenant_id + FORCE RLS, uniform with every other tenant-scoped table (0020) ──
ALTER TABLE bank_debts ADD COLUMN tenant_id UUID;
ALTER TABLE bank_debts ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_debts ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bank_debts ADD CONSTRAINT bank_debts_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_debts ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_debts FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_debts
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_debts_tenant_idx ON bank_debts (tenant_id);

ALTER TABLE bank_debt_reminders ADD COLUMN tenant_id UUID;
ALTER TABLE bank_debt_reminders ALTER COLUMN tenant_id SET DEFAULT current_setting('app.current_tenant')::uuid;
ALTER TABLE bank_debt_reminders ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bank_debt_reminders ADD CONSTRAINT bank_debt_reminders_tenant_fk FOREIGN KEY (tenant_id) REFERENCES organizations(id);
ALTER TABLE bank_debt_reminders ENABLE ROW LEVEL SECURITY;
ALTER TABLE bank_debt_reminders FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON bank_debt_reminders
    USING (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
CREATE INDEX bank_debt_reminders_tenant_idx ON bank_debt_reminders (tenant_id);
