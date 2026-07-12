-- 0014_bank_core: the bank module's personal-ledger schema (SPEC-03 §6) + RBAC
-- and category seeds. Money is integer minor units (bigint) end-to-end per D-41
-- (VND exponent 0). user_id FKs into account's users(id) — the sanctioned
-- identity-anchor exception (matches 0007/0009/0011). Import-ready columns
-- (description_raw / import_batch_id / dedup_hash) ship now, unused by the manual
-- path (P0.9), so statement import lands without a breaking migration.

-- ── accounts (P0.1) ─────────────────────────────────────────────────────
CREATE TABLE bank_accounts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    type            TEXT NOT NULL CHECK (type IN
                       ('cash','checking','savings','credit_card','ewallet','other')),
    currency        CHAR(3) NOT NULL DEFAULT 'VND',
    opening_balance BIGINT NOT NULL DEFAULT 0,   -- minor units; balance is always DERIVED
    archived        BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bank_accounts_user_idx ON bank_accounts (user_id, archived);

-- ── categories (P0.4) — hierarchical, 2 levels max, income|expense ───────
CREATE TABLE bank_categories (
    id        UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id   UUID REFERENCES users(id) ON DELETE CASCADE,          -- NULL = seeded default
    parent_id UUID REFERENCES bank_categories(id) ON DELETE SET NULL, -- parent delete promotes children
    name      TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 100),
    kind      TEXT NOT NULL CHECK (kind IN ('income','expense'))
);
CREATE INDEX bank_categories_user_idx ON bank_categories (user_id);
CREATE INDEX bank_categories_parent_idx ON bank_categories (parent_id) WHERE parent_id IS NOT NULL;

-- ── import batches (P0.9) — schema only; empty until statement import lands ─
CREATE TABLE bank_import_batches (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source     TEXT NOT NULL,
    file_name  TEXT,
    status     TEXT NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ── transactions (P0.2 / P0.3) ──────────────────────────────────────────
CREATE TABLE bank_transactions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id      UUID NOT NULL REFERENCES bank_accounts(id),
    category_id     UUID REFERENCES bank_categories(id),   -- NULL only for transfer legs
    amount          BIGINT NOT NULL CHECK (amount > 0),    -- minor units, strictly positive
    direction       TEXT NOT NULL CHECK (direction IN ('debit','credit')),
    transfer_id     UUID,                                  -- shared by a transfer's paired legs
    occurred_at     DATE NOT NULL,
    note            TEXT,
    description_raw TEXT,                                  -- import-ready (P0.9)
    import_batch_id UUID REFERENCES bank_import_batches(id), -- import-ready
    dedup_hash      TEXT,                                  -- import-ready
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (category_id IS NOT NULL OR transfer_id IS NOT NULL)
);
CREATE INDEX bank_transactions_user_occurred_idx ON bank_transactions (user_id, occurred_at DESC, id DESC);
CREATE INDEX bank_transactions_account_occurred_idx ON bank_transactions (account_id, occurred_at DESC);
CREATE INDEX bank_transactions_category_idx ON bank_transactions (category_id) WHERE category_id IS NOT NULL;
CREATE INDEX bank_transactions_transfer_idx ON bank_transactions (transfer_id) WHERE transfer_id IS NOT NULL;
CREATE UNIQUE INDEX bank_transactions_dedup_idx ON bank_transactions (account_id, dedup_hash)
    WHERE dedup_hash IS NOT NULL;

-- ── monthly budgets (P0.5) — month = first-of-month date ─────────────────
CREATE TABLE bank_budgets (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id UUID NOT NULL REFERENCES bank_categories(id) ON DELETE CASCADE, -- budgets die with the category
    month       DATE NOT NULL CHECK (EXTRACT(day FROM month) = 1),
    amount      BIGINT NOT NULL CHECK (amount > 0),
    UNIQUE (user_id, category_id, month)
);

-- ── seed: bank permission catalog + grants to base `user` (P0.8 / §7) ────
-- 2-3-segment grammar; kebab-compound resources (bank-accounts etc.) per the
-- SPEC-04 notification-prefs precedent. write covers create+update (+archive).
INSERT INTO permissions (code, description) VALUES
    ('bank-accounts:read:own',       'Read own bank accounts'),
    ('bank-accounts:write:own',      'Create, edit and archive own bank accounts'),
    ('bank-accounts:delete:own',     'Delete own empty bank accounts'),
    ('bank-transactions:read:own',   'Read own transactions'),
    ('bank-transactions:write:own',  'Create and edit own transactions and transfers'),
    ('bank-transactions:delete:own', 'Delete own transactions and transfers'),
    ('bank-categories:read:own',     'Read own and seed categories'),
    ('bank-categories:write:own',    'Create and edit own categories'),
    ('bank-categories:delete:own',   'Delete and reassign own categories'),
    ('bank-budgets:read:own',        'Read own budgets'),
    ('bank-budgets:write:own',       'Upsert and delete own budgets')
ON CONFLICT (code) DO NOTHING;

WITH grants(role_code, perm_code) AS (VALUES
    ('user', 'bank-accounts:read:own'),
    ('user', 'bank-accounts:write:own'),
    ('user', 'bank-accounts:delete:own'),
    ('user', 'bank-transactions:read:own'),
    ('user', 'bank-transactions:write:own'),
    ('user', 'bank-transactions:delete:own'),
    ('user', 'bank-categories:read:own'),
    ('user', 'bank-categories:write:own'),
    ('user', 'bank-categories:delete:own'),
    ('user', 'bank-budgets:read:own'),
    ('user', 'bank-budgets:write:own')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM grants g
JOIN roles r       ON r.code = g.role_code
JOIN permissions p ON p.code = g.perm_code
ON CONFLICT DO NOTHING;

-- ── seed: Vietnamese-context default categories (P0.4; user_id NULL) ──────
-- Top-level first, then children (2-level tree). Seeds are fixed at v1; users
-- add their own alongside (rename/hide seeds is P2).
INSERT INTO bank_categories (user_id, parent_id, name, kind) VALUES
    (NULL, NULL, 'Ăn uống',       'expense'),
    (NULL, NULL, 'Di chuyển',     'expense'),
    (NULL, NULL, 'Hóa đơn',       'expense'),
    (NULL, NULL, 'Nhà cửa',       'expense'),
    (NULL, NULL, 'Mua sắm',       'expense'),
    (NULL, NULL, 'Sức khỏe',      'expense'),
    (NULL, NULL, 'Giải trí',      'expense'),
    (NULL, NULL, 'Giáo dục',      'expense'),
    (NULL, NULL, 'Phí & Lệ phí',  'expense'),
    (NULL, NULL, 'Khác',          'expense'),
    (NULL, NULL, 'Lương',         'income'),
    (NULL, NULL, 'Thưởng',        'income'),
    (NULL, NULL, 'Quà tặng',      'income'),
    (NULL, NULL, 'Lãi',           'income'),
    (NULL, NULL, 'Hoàn tiền',     'income'),
    (NULL, NULL, 'Thu nhập khác', 'income');

INSERT INTO bank_categories (user_id, parent_id, name, kind)
SELECT NULL, p.id, c.name, 'expense'
FROM (VALUES
    ('Đi chợ',     'Ăn uống'),
    ('Ăn ngoài',   'Ăn uống'),
    ('Cà phê',     'Ăn uống'),
    ('Xăng xe',    'Di chuyển'),
    ('Grab/Taxi',  'Di chuyển'),
    ('Gửi xe',     'Di chuyển'),
    ('Điện',       'Hóa đơn'),
    ('Nước',       'Hóa đơn'),
    ('Internet',   'Hóa đơn'),
    ('Điện thoại', 'Hóa đơn')
) AS c(name, parent_name)
JOIN bank_categories p
  ON p.name = c.parent_name AND p.user_id IS NULL AND p.parent_id IS NULL AND p.kind = 'expense';
