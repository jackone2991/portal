// Package layout owns the app shell's composition: the navigation menu and the
// dashboard widget rails (migration 0036). Both used to be hardcoded arrays in
// the React bundle, so changing them meant a redeploy.
//
// What it does NOT own: the widgets and pages themselves. A widget row is a
// pointer into the frontend's component registry, and a menu row is a link — the
// module stores placement and visibility, never behaviour.
//
// Other modules import only layout/api. It owns no Asynq tasks and emits no
// events: nothing downstream cares when a menu is reordered.
package layout

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// Slots a widget can occupy. Mirrors the layout_widgets_slot_check constraint.
const (
	SlotLeft  = "left"
	SlotRight = "right"
)

// PermissionWrite gates every mutation. Reads are open to any signed-in user —
// the shell cannot render without them.
const PermissionWrite = "system:settings:write"

var (
	// ErrValidation is any malformed payload. The handler maps it to 400 with
	// the message attached, so the reason reaches the admin unchanged.
	ErrValidation = errors.New("layout: validation")
	// ErrUnknownWidget is a save naming a widget key that is not seeded. Widgets
	// are registry-backed; the API cannot invent one.
	ErrUnknownWidget = errors.New("layout: unknown widget")
)

// MenuItem is one navigation row.
type MenuItem struct {
	ID    uuid.UUID
	Key   string
	Label string
	Icon  string
	// Href empty = renders but navigates nowhere (the template's decorative rows).
	Href string
	// Permission empty = visible to every signed-in user.
	Permission string
	Position   int
	Visible    bool
	// IsSystem rows can be renamed, reordered and hidden, but not deleted.
	IsSystem bool
}

// Widget is one dashboard card's placement.
type Widget struct {
	ID         uuid.UUID
	Key        string
	Label      string
	Slot       string
	Permission string
	Position   int
	Visible    bool
}

// Config is the whole layout, as the admin console sees it.
type Config struct {
	Menu    []MenuItem
	Widgets []Widget
}

// Repository is the persistence surface. The sqlc-backed adapter implements it.
type Repository interface {
	ListMenuItems(ctx context.Context) ([]MenuItem, error)
	ListWidgets(ctx context.Context) ([]Widget, error)
	// SaveMenu replaces the whole menu: upsert everything in `items` by key, then
	// delete every non-system row whose key is absent. One transaction — a
	// half-applied menu is a broken shell, not a glitch.
	SaveMenu(ctx context.Context, items []MenuItem) error
	// SaveWidgets updates placement for the given keys. Never inserts or deletes.
	SaveWidgets(ctx context.Context, widgets []Widget) error
}

// PermissionChecker answers "may the caller in ctx do X". Satisfied by
// account/api's Impl; injected so layout never imports account/rbac.
type PermissionChecker interface {
	HasPermission(ctx context.Context, code string) bool
}
