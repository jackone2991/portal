ALTER TABLE bank_categories
    DROP CONSTRAINT IF EXISTS bank_categories_icon_len,
    DROP CONSTRAINT IF EXISTS bank_categories_color_hex;

ALTER TABLE bank_categories
    DROP COLUMN IF EXISTS icon,
    DROP COLUMN IF EXISTS color;
