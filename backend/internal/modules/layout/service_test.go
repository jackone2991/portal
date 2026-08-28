package layout

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The service's job is to make a submitted arrangement safe to store: renumber
// it, refuse the shapes that would break the shell, and refuse the ones that
// would turn the app's own navigation into an attack surface.

type fakeRepo struct {
	menu    []MenuItem
	widgets []Widget

	savedMenu    []MenuItem
	savedWidgets []Widget
}

func (f *fakeRepo) ListMenuItems(context.Context) ([]MenuItem, error) { return f.menu, nil }
func (f *fakeRepo) ListWidgets(context.Context) ([]Widget, error)     { return f.widgets, nil }
func (f *fakeRepo) SaveMenu(_ context.Context, items []MenuItem) error {
	f.savedMenu = items
	return nil
}
func (f *fakeRepo) SaveWidgets(_ context.Context, w []Widget) error {
	f.savedWidgets = w
	return nil
}

// stubPerms grants exactly the listed codes, by exact match — enough to prove
// the filter consults it at all. The real grammar lives in account/rbac.
type stubPerms struct{ codes map[string]bool }

func (s stubPerms) HasPermission(_ context.Context, code string) bool { return s.codes[code] }

func newService(repo *fakeRepo, granted ...string) *Service {
	set := map[string]bool{}
	for _, c := range granted {
		set[c] = true
	}
	return &Service{repo: repo, perms: stubPerms{codes: set}}
}

func item(key string) MenuItem {
	return MenuItem{Key: key, Label: key, Icon: "newsfeed-icon", Visible: true}
}

/* ── menu validation ──────────────────────────────────────────────── */

// Position comes from array order, not the payload. Trusting a client-sent
// position is how two rows end up claiming the same slot.
func TestSaveMenuRenumbersFromArrayOrder(t *testing.T) {
	repo := &fakeRepo{}
	items := []MenuItem{item("c"), item("a"), item("b")}
	// Positions in the payload are deliberately wrong and identical.
	for i := range items {
		items[i].Position = 999
	}

	if err := newService(repo).SaveMenu(context.Background(), items); err != nil {
		t.Fatalf("SaveMenu: %v", err)
	}
	got := repo.savedMenu
	if len(got) != 3 {
		t.Fatalf("saved %d items, want 3", len(got))
	}
	for i, want := range []struct {
		key string
		pos int
	}{{"c", 10}, {"a", 20}, {"b", 30}} {
		if got[i].Key != want.key || got[i].Position != want.pos {
			t.Errorf("item %d = (%s, %d), want (%s, %d)", i, got[i].Key, got[i].Position, want.key, want.pos)
		}
	}
}

// An absolute URL in the sidebar would be an open redirect that every signed-in
// user is shown by the app itself.
func TestSaveMenuRejectsOffSiteLinks(t *testing.T) {
	for _, href := range []string{
		"https://evil.test/phish",
		"//evil.test/phish", // protocol-relative: still leaves the origin
		"javascript:alert(1)",
		"relative/path",
	} {
		it := item("x")
		it.Href = href
		err := newService(&fakeRepo{}).SaveMenu(context.Background(), []MenuItem{it})
		if !errors.Is(err, ErrValidation) {
			t.Errorf("href %q accepted (err=%v), want a validation refusal", href, err)
		}
	}
}

func TestSaveMenuAcceptsInAppPaths(t *testing.T) {
	repo := &fakeRepo{}
	it := item("x")
	it.Href = "/library/music"
	if err := newService(repo).SaveMenu(context.Background(), []MenuItem{it}); err != nil {
		t.Fatalf("SaveMenu: %v", err)
	}
	if repo.savedMenu[0].Href != "/library/music" {
		t.Errorf("href = %q, want it preserved", repo.savedMenu[0].Href)
	}
}

func TestSaveMenuRejectsAnEmptyMenu(t *testing.T) {
	err := newService(&fakeRepo{}).SaveMenu(context.Background(), nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("err = %v, want a validation refusal — an empty menu is not recoverable from the UI", err)
	}
}

func TestSaveMenuRejectsDuplicateKeys(t *testing.T) {
	err := newService(&fakeRepo{}).SaveMenu(context.Background(), []MenuItem{item("dup"), item("dup")})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("err = %v, want a validation refusal", err)
	}
}

func TestSaveMenuRejectsMalformedKeys(t *testing.T) {
	for _, key := range []string{"", "Upper", "with space", "colon:key", strings.Repeat("a", 41)} {
		it := item("placeholder")
		it.Key = key
		if err := newService(&fakeRepo{}).SaveMenu(context.Background(), []MenuItem{it}); !errors.Is(err, ErrValidation) {
			t.Errorf("key %q accepted, want a validation refusal", key)
		}
	}
}

// The refusal has to say WHICH row is wrong — an admin editing fifteen entries
// cannot act on "invalid request".
func TestSaveMenuErrorNamesTheOffendingRow(t *testing.T) {
	bad := item("weather")
	bad.Href = "https://evil.test"
	err := newService(&fakeRepo{}).SaveMenu(context.Background(), []MenuItem{item("ok"), bad})
	if err == nil || !strings.Contains(err.Error(), "weather") {
		t.Errorf("err = %v, want it to name \"weather\"", err)
	}
}

/* ── widget validation ────────────────────────────────────────────── */

// Widgets are registry-backed: a key the catalogue has never seen names no
// component, so storing it would render an invisible hole.
func TestSaveWidgetsRejectsAnUnknownKey(t *testing.T) {
	repo := &fakeRepo{widgets: []Widget{{Key: "music", Label: "Music", Slot: SlotLeft}}}
	err := newService(repo).SaveWidgets(context.Background(), []Widget{
		{Key: "not-a-widget", Label: "Nope", Slot: SlotLeft, Visible: true},
	})
	if !errors.Is(err, ErrUnknownWidget) {
		t.Fatalf("err = %v, want ErrUnknownWidget", err)
	}
	if repo.savedWidgets != nil {
		t.Error("widgets were written despite the refusal")
	}
}

// Moving a card between rails must not carry a position over from the rail it
// left, so numbering restarts per slot.
func TestSaveWidgetsNumbersEachSlotIndependently(t *testing.T) {
	repo := &fakeRepo{widgets: []Widget{
		{Key: "a"}, {Key: "b"}, {Key: "c"},
	}}
	err := newService(repo).SaveWidgets(context.Background(), []Widget{
		{Key: "a", Label: "A", Slot: SlotLeft, Visible: true},
		{Key: "b", Label: "B", Slot: SlotRight, Visible: true},
		{Key: "c", Label: "C", Slot: SlotLeft, Visible: true},
	})
	if err != nil {
		t.Fatalf("SaveWidgets: %v", err)
	}
	want := map[string]int{"a": 10, "c": 20, "b": 10}
	for _, w := range repo.savedWidgets {
		if w.Position != want[w.Key] {
			t.Errorf("%s position = %d, want %d", w.Key, w.Position, want[w.Key])
		}
	}
}

func TestSaveWidgetsRejectsAnUnknownSlot(t *testing.T) {
	repo := &fakeRepo{widgets: []Widget{{Key: "a"}}}
	err := newService(repo).SaveWidgets(context.Background(), []Widget{
		{Key: "a", Label: "A", Slot: "middle", Visible: true},
	})
	if !errors.Is(err, ErrValidation) {
		t.Errorf("err = %v, want a validation refusal", err)
	}
}

/* ── per-caller filtering ─────────────────────────────────────────── */

// The shell read must not hand an admin-only entry to a user who cannot open
// it. Filtering server-side is the point: a client-side filter would still ship
// the row to a bundle anyone can read.
func TestForCallerDropsRowsTheCallerCannotSee(t *testing.T) {
	open := item("newsfeed")
	gated := item("admin-users")
	gated.Permission = "users:read:any"
	hidden := item("wip")
	hidden.Visible = false

	repo := &fakeRepo{
		menu: []MenuItem{open, gated, hidden},
		widgets: []Widget{
			{Key: "music", Slot: SlotLeft, Visible: true},
			{Key: "finance", Slot: SlotLeft, Visible: true, Permission: "bank-accounts:read:own"},
		},
	}

	cfg, err := newService(repo).ForCaller(context.Background()) // grants nothing
	if err != nil {
		t.Fatalf("ForCaller: %v", err)
	}
	if len(cfg.Menu) != 1 || cfg.Menu[0].Key != "newsfeed" {
		t.Errorf("menu = %v, want only the unrestricted visible row", keys(cfg.Menu))
	}
	if len(cfg.Widgets) != 1 || cfg.Widgets[0].Key != "music" {
		t.Errorf("widgets = %v, want only the unrestricted one", cfg.Widgets)
	}

	cfg, err = newService(repo, "users:read:any", "bank-accounts:read:own").ForCaller(context.Background())
	if err != nil {
		t.Fatalf("ForCaller: %v", err)
	}
	if len(cfg.Menu) != 2 {
		t.Errorf("menu = %v, want the gated row included for a holder", keys(cfg.Menu))
	}
	if len(cfg.Widgets) != 2 {
		t.Errorf("widgets = %d, want both for a holder", len(cfg.Widgets))
	}
}

// A deployment that wires no checker must fall back to "unrestricted rows only",
// never to "everything" — this is a fail-closed path, not a degraded one.
func TestForCallerWithNoPermissionCheckerHidesGatedRows(t *testing.T) {
	gated := item("admin-users")
	gated.Permission = "users:read:any"
	svc := &Service{repo: &fakeRepo{menu: []MenuItem{item("newsfeed"), gated}}}

	cfg, err := svc.ForCaller(context.Background())
	if err != nil {
		t.Fatalf("ForCaller: %v", err)
	}
	if len(cfg.Menu) != 1 || cfg.Menu[0].Key != "newsfeed" {
		t.Errorf("menu = %v, want the gated row withheld", keys(cfg.Menu))
	}
}

// Full is the admin view: it must show what ForCaller hides, or the console
// could not un-hide anything.
func TestFullKeepsHiddenAndGatedRows(t *testing.T) {
	gated := item("admin-users")
	gated.Permission = "users:read:any"
	hidden := item("wip")
	hidden.Visible = false

	cfg, err := newService(&fakeRepo{menu: []MenuItem{gated, hidden}}).Full(context.Background())
	if err != nil {
		t.Fatalf("Full: %v", err)
	}
	if len(cfg.Menu) != 2 {
		t.Errorf("menu = %v, want every row", keys(cfg.Menu))
	}
}

func keys(items []MenuItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Key)
	}
	return out
}
