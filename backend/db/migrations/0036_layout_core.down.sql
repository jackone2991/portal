-- Down: the shell falls back to its hardcoded defaults. The frontend keeps a
-- static copy of this exact seed for the pre-fetch and error paths, so dropping
-- these tables degrades the menu to "not editable", not to "empty".
DROP TABLE IF EXISTS layout_widgets;
DROP TABLE IF EXISTS layout_menu_items;
