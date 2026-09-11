-- 0042_bank_category_presentation: give every ledger category a face.
--
-- A category list that is only text is unreadable at a glance — which is the
-- whole point of a spending log you open several times a day. An icon and a
-- colour are what make a row scannable, what makes a donut slice identifiable
-- without a legend, and what lets a user tell "Ăn ngoài" from "Đi chợ" while
-- scrolling past. Money-tracking apps are icon-first for this reason.
--
-- The icon is an EMOJI, deliberately, rather than a sprite id or an uploaded
-- image: the sprite shipped with this template is a social-network set with no
-- money/food/transport glyphs, an image per category means an asset pipeline for
-- a 16px chip, and emoji renders in every browser, needs no request, and lets a
-- user creating "Cà phê" pick ☕ themselves. Stored as text and length-capped —
-- one emoji can be several code points (ZWJ sequences, skin tones), so the cap
-- is generous rather than 1.
--
-- Both columns are NULLable: a category with no icon is a normal state (the UI
-- falls back to the first letter on a neutral chip), not a defect.

ALTER TABLE bank_categories
    ADD COLUMN icon  TEXT,
    ADD COLUMN color TEXT;

ALTER TABLE bank_categories
    ADD CONSTRAINT bank_categories_icon_len CHECK (icon IS NULL OR char_length(icon) BETWEEN 1 AND 16),
    -- 6-digit hex only. Accepting arbitrary CSS here would put user text straight
    -- into a style attribute; a fixed shape means the value can never be anything
    -- but a colour.
    ADD CONSTRAINT bank_categories_color_hex CHECK (color IS NULL OR color ~ '^#[0-9a-fA-F]{6}$');

-- ── seed presentation for the built-in categories (user_id IS NULL) ──────
UPDATE bank_categories AS c
SET icon = v.icon, color = v.color
FROM (VALUES
    ('Ăn uống',       'expense', '🍜', '#f97316'),
    ('Di chuyển',     'expense', '🚌', '#0ea5e9'),
    ('Hóa đơn',       'expense', '🧾', '#64748b'),
    ('Nhà cửa',       'expense', '🏠', '#8b5cf6'),
    ('Mua sắm',       'expense', '🛍️', '#ec4899'),
    ('Sức khỏe',      'expense', '💊', '#ef4444'),
    ('Giải trí',      'expense', '🎬', '#a855f7'),
    ('Giáo dục',      'expense', '📚', '#14b8a6'),
    ('Phí & Lệ phí',  'expense', '🏦', '#78716c'),
    ('Khác',          'expense', '📦', '#6b7280'),
    ('Lương',         'income',  '💰', '#22c55e'),
    ('Thưởng',        'income',  '🎉', '#16a34a'),
    ('Quà tặng',      'income',  '🎁', '#f43f5e'),
    ('Lãi',           'income',  '📈', '#10b981'),
    ('Hoàn tiền',     'income',  '↩️', '#06b6d4'),
    ('Thu nhập khác', 'income',  '💵', '#4ade80')
) AS v(name, kind, icon, color)
WHERE c.user_id IS NULL AND c.parent_id IS NULL AND c.name = v.name AND c.kind = v.kind;

-- Children get their own emoji but INHERIT the parent's colour, so a donut slice
-- and its breakdown rows read as one family instead of sixteen unrelated hues.
UPDATE bank_categories AS c
SET icon = v.icon
FROM (VALUES
    ('Đi chợ',     '🛒'),
    ('Ăn ngoài',   '🍽️'),
    ('Cà phê',     '☕'),
    ('Xăng xe',    '⛽'),
    ('Grab/Taxi',  '🚕'),
    ('Gửi xe',     '🅿️'),
    ('Điện',       '💡'),
    ('Nước',       '🚰'),
    ('Internet',   '🌐'),
    ('Điện thoại', '📱')
) AS v(name, icon)
WHERE c.user_id IS NULL AND c.parent_id IS NOT NULL AND c.name = v.name;

UPDATE bank_categories AS c
SET color = p.color
FROM bank_categories p
WHERE c.parent_id = p.id AND c.user_id IS NULL AND c.color IS NULL;
