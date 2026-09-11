DROP TABLE IF EXISTS bank_debt_reminders;
DROP TABLE IF EXISTS bank_debts;
DELETE FROM bank_categories WHERE user_id IS NULL AND parent_id IS NULL AND name = 'Lãi vay';
ALTER TABLE bank_accounts DROP CONSTRAINT IF EXISTS bank_accounts_type_check;
ALTER TABLE bank_accounts ADD CONSTRAINT bank_accounts_type_check CHECK (type IN
    ('cash','checking','savings','credit_card','ewallet','other'));
