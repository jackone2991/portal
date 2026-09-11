package layout

import (
	"context"
	"fmt"
	"strings"
)

// maxMenuItems bounds a whole-set save. The menu is a sidebar, not a CMS: a
// payload with thousands of rows is a bug or an attack, and either way the
// result would be unusable.
const maxMenuItems = 80

const (
	maxLabelLen = 60
	maxHrefLen  = 300
)

// Service holds the layout logic. Construct through the module.
type Service struct {
	repo  Repository
	perms PermissionChecker
}

// Full returns the whole configuration, hidden rows included. For the admin
// console.
func (s *Service) Full(ctx context.Context) (Config, error) {
	menu, err := s.repo.ListMenuItems(ctx)
	if err != nil {
		return Config{}, err
	}
	widgets, err := s.repo.ListWidgets(ctx)
	if err != nil {
		return Config{}, err
	}
	return Config{Menu: menu, Widgets: widgets}, nil
}

// ForCaller returns only what the principal in ctx should actually see: visible
// rows whose permission they hold.
//
// The filtering is here rather than in the client on purpose. Shipping the whole
// menu and hiding rows in React would tell every user which admin screens exist,
// and would put the decision in a bundle anyone can read. `perms` is nil in
// deployments that wire no checker, which degrades to "show the unrestricted
// rows only" — never to "show everything".
func (s *Service) ForCaller(ctx context.Context) (Config, error) {
	full, err := s.Full(ctx)
	if err != nil {
		return Config{}, err
	}

	// One cache-backed permission load serves every row; the engine memoises the
	// set per (user, token_version), so the repeated calls below are map lookups.
	allowed := func(code string) bool {
		if code == "" {
			return true
		}
		if s.perms == nil {
			return false
		}
		return s.perms.HasPermission(ctx, code)
	}

	out := Config{Menu: make([]MenuItem, 0, len(full.Menu)), Widgets: make([]Widget, 0, len(full.Widgets))}
	for _, it := range full.Menu {
		if it.Visible && allowed(it.Permission) {
			out.Menu = append(out.Menu, it)
		}
	}
	for _, w := range full.Widgets {
		if w.Visible && allowed(w.Permission) {
			out.Widgets = append(out.Widgets, w)
		}
	}
	return out, nil
}

// SaveMenu validates and replaces the whole menu.
//
// Positions are renumbered from the submitted ORDER rather than trusted from the
// payload: the admin screen moves rows up and down, and making the array order
// authoritative removes a whole class of "two rows claim position 3" bugs.
func (s *Service) SaveMenu(ctx context.Context, items []MenuItem) error {
	if len(items) == 0 {
		return fmt.Errorf("%w: the menu cannot be empty", ErrValidation)
	}
	if len(items) > maxMenuItems {
		return fmt.Errorf("%w: at most %d menu items", ErrValidation, maxMenuItems)
	}

	seen := make(map[string]bool, len(items))
	cleaned := make([]MenuItem, 0, len(items))
	for i, it := range items {
		it.Key = strings.TrimSpace(it.Key)
		it.Label = strings.TrimSpace(it.Label)
		it.Icon = strings.TrimSpace(it.Icon)
		it.Href = strings.TrimSpace(it.Href)
		it.Permission = strings.TrimSpace(it.Permission)

		if !validKey(it.Key) {
			return fmt.Errorf("%w: %q is not a valid key (lowercase letters, digits, '-' and '_')", ErrValidation, it.Key)
		}
		if seen[it.Key] {
			return fmt.Errorf("%w: duplicate key %q", ErrValidation, it.Key)
		}
		seen[it.Key] = true

		if it.Label == "" || len(it.Label) > maxLabelLen {
			return fmt.Errorf("%w: %q needs a label of 1-%d characters", ErrValidation, it.Key, maxLabelLen)
		}
		if it.Icon == "" {
			return fmt.Errorf("%w: %q needs an icon", ErrValidation, it.Key)
		}
		// Relative paths only. An absolute URL here would turn the app's own
		// navigation into an open redirect surface that every user sees.
		if it.Href != "" {
			if !strings.HasPrefix(it.Href, "/") || strings.HasPrefix(it.Href, "//") {
				return fmt.Errorf("%w: %q must be an in-app path starting with a single '/'", ErrValidation, it.Key)
			}
			if len(it.Href) > maxHrefLen {
				return fmt.Errorf("%w: the link on %q is too long", ErrValidation, it.Key)
			}
		}

		it.Position = (i + 1) * 10
		cleaned = append(cleaned, it)
	}
	return s.repo.SaveMenu(ctx, cleaned)
}

// SaveWidgets validates and applies placement.
//
// Every submitted key must already exist: the catalogue is the frontend's
// component registry, so a key the database has never seen names no component
// and would render an empty hole.
func (s *Service) SaveWidgets(ctx context.Context, widgets []Widget) error {
	existing, err := s.repo.ListWidgets(ctx)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing))
	for _, w := range existing {
		known[w.Key] = true
	}

	// Positions restart per slot, so moving a card between rails does not inherit
	// a number from the rail it left.
	perSlot := map[string]int{}
	seen := make(map[string]bool, len(widgets))
	cleaned := make([]Widget, 0, len(widgets))
	for _, w := range widgets {
		w.Key = strings.TrimSpace(w.Key)
		w.Label = strings.TrimSpace(w.Label)
		w.Slot = strings.TrimSpace(w.Slot)
		w.Permission = strings.TrimSpace(w.Permission)

		if !known[w.Key] {
			return fmt.Errorf("%w: %q", ErrUnknownWidget, w.Key)
		}
		if seen[w.Key] {
			return fmt.Errorf("%w: duplicate widget %q", ErrValidation, w.Key)
		}
		seen[w.Key] = true

		if w.Slot != SlotLeft && w.Slot != SlotRight {
			return fmt.Errorf("%w: %q has an unknown slot %q", ErrValidation, w.Key, w.Slot)
		}
		if w.Label == "" || len(w.Label) > maxLabelLen {
			return fmt.Errorf("%w: %q needs a label of 1-%d characters", ErrValidation, w.Key, maxLabelLen)
		}

		perSlot[w.Slot] += 10
		w.Position = perSlot[w.Slot]
		cleaned = append(cleaned, w)
	}
	return s.repo.SaveWidgets(ctx, cleaned)
}

// validKey mirrors the role-code rule: a stable handle, not a label.
func validKey(s string) bool {
	if len(s) == 0 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}
