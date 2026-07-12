-- Reverse 0014_bank_core. Revoke grants + permissions, then drop tables in
-- dependency order (dependents first). Category seeds vanish with their table.

DELETE FROM role_permissions WHERE permission_id IN (
    SELECT id FROM permissions WHERE code LIKE 'bank-%'
);
DELETE FROM permissions WHERE code LIKE 'bank-%';

DROP TABLE IF EXISTS bank_budgets;
DROP TABLE IF EXISTS bank_transactions;
DROP TABLE IF EXISTS bank_import_batches;
DROP TABLE IF EXISTS bank_categories;
DROP TABLE IF EXISTS bank_accounts;
