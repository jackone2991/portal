-- 0036_layout_core: the shell's navigation menu and dashboard widgets become data.
-- Owning module: layout.
--
-- Until now both were hardcoded React arrays (SidebarLeft's ITEMS, HomeView's two
-- widget rails), so reordering a menu entry or hiding a widget meant a rebuild
-- and a redeploy. The seed below reproduces that shell EXACTLY, row for row, so
-- applying this migration changes nothing visible — it only moves the decision
-- out of the bundle.
--
-- The two tables look alike but are not interchangeable:
--
--   MENU items are free-form. A link is just data, so an admin can add, rename
--   and delete them at will.
--   WIDGETS are registry-backed. `key` has to match a React component in
--   frontend/src/templates/v1/components/widget — the database cannot conjure a
--   component — so rows are seeded and an admin controls placement and
--   visibility, not existence. Hence no is_system column here: every widget is
--   effectively a system row.

-- ── menu ──────────────────────────────────────────────────────────────────
CREATE TABLE layout_menu_items (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- Stable handle that survives a rename. Referenced by nothing today, but a
    -- label is not an identity and future deep links will want one.
    key         TEXT UNIQUE NOT NULL,
    label       TEXT NOT NULL,
    icon        TEXT NOT NULL,              -- sprite name; see components/ui/Icon
    -- NULL href = a row that renders but navigates nowhere (the template's
    -- decorative entries). Kept rather than dropped so the seeded shell matches.
    href        TEXT,
    -- Permission code required to SEE the row. NULL = every signed-in user.
    -- Filtering happens server-side, so an admin-only entry is not merely hidden
    -- in the client bundle.
    permission  TEXT,
    position    INTEGER NOT NULL,
    visible     BOOLEAN NOT NULL DEFAULT true,
    -- Undeletable. Reserved for the ONE row an admin must not be able to remove:
    -- the link to this editor itself. Losing it does not brick anything — the URL
    -- still works — but "delete the door you are standing in" is a mistake worth
    -- refusing rather than one to explain afterwards.
    --
    -- Everything else is deletable on purpose. Half the seeded menu is inherited
    -- template decoration (Friend Groups, Community Badges, Account Stats) with no
    -- link behind it, and removing that is the single most likely reason anyone
    -- opens this screen. The guard against wiping the menu entirely is the
    -- "cannot be empty" check in the service, not a flag on every row.
    is_system   BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX layout_menu_items_position_idx ON layout_menu_items (position, id);

-- ── widgets ───────────────────────────────────────────────────────────────
CREATE TABLE layout_widgets (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    -- MUST match a key in the frontend widget registry. An unknown key renders
    -- nothing at all rather than crashing the dashboard.
    key         TEXT UNIQUE NOT NULL,
    label       TEXT NOT NULL,              -- what the admin screen calls it
    slot        TEXT NOT NULL,              -- which rail it sits in
    permission  TEXT,
    position    INTEGER NOT NULL,
    visible     BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT layout_widgets_slot_check CHECK (slot IN ('left', 'right'))
);

CREATE INDEX layout_widgets_slot_position_idx ON layout_widgets (slot, position, id);

-- ── seed: today's sidebar, in today's order ───────────────────────────────
-- "Manage Widgets" was a dead entry with no href — it is now what it always
-- claimed to be, pointing at the screen this migration exists for, and it is the
-- only row flagged is_system.
INSERT INTO layout_menu_items (key, label, icon, href, permission, position, is_system) VALUES
    ('newsfeed',       'Newsfeed',            'newsfeed-icon',       '/',                NULL,               10, false),
    ('upload',         'Upload Video',        'multimedia-icon',     '/upload',          NULL,               20, false),
    ('ledger',         'Ledger',              'stats-icon',          '/bank',            NULL,               30, false),
    ('people',         'People',              'happy-faces-icon',    '/people',          NULL,               40, false),
    ('comic',          'Commic',              'albums-icon',         '/library/comic',   NULL,               50, false),
    ('friend-groups',  'Friend Groups',       'happy-faces-icon',    NULL,               NULL,               60, false),
    ('music',          'Music & Playlists',   'headphones-icon',     '/library/music',   NULL,               70, false),
    ('weather',        'Weather App',         'weather-icon',        '/weather',         NULL,               80, false),
    ('calendar',       'Calendar and Events', 'calendar-icon',       '/calendar',        NULL,               90, false),
    ('badges',         'Community Badges',    'badge-icon',          NULL,               NULL,              100, false),
    ('birthdays',      'Friends Birthdays',   'cupcake-icon',        NULL,               NULL,              110, false),
    ('account-stats',  'Account Stats',       'stats-icon',          NULL,               NULL,              120, false),
    ('admin-users',    'Người dùng',          'happy-faces-icon',    '/admin/users',     'users:read:any',  130, false),
    ('admin-roles',    'Vai trò & quyền',     'badge-icon',          '/admin/roles',     'rbac:role:read',  140, false),
    ('admin-layout',   'Menu & Widget',       'manage-widgets-icon', '/admin/layout',    'system:settings:write', 150, true);

-- ── seed: today's dashboard rails ─────────────────────────────────────────
-- `permission` mirrors what each widget's own query needs, so a user who cannot
-- read the ledger is not served an empty finance card.
INSERT INTO layout_widgets (key, label, slot, permission, position) VALUES
    ('finance',            'Tài chính',        'left',  'bank-accounts:read:own', 10),
    ('continue',           'Xem tiếp',         'left',  NULL,                     20),
    ('music',              'Nhạc',             'left',  'music:read',             30),
    ('weather',            'Thời tiết',        'left',  NULL,                     40),
    ('calendar',           'Lịch',             'left',  NULL,                     50),
    ('pages',              'Trang',            'left',  NULL,                     60),
    ('birthdays',          'Sinh nhật',        'right', 'people:read:own',        10),
    ('friend-suggestions', 'Gợi ý kết bạn',    'right', NULL,                     20),
    ('activity-feed',      'Hoạt động',        'right', NULL,                     30);
