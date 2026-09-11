package layout

import (
	"errors"
	"net/http"

	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/server"
)

// Handler serves the layout routes. See module.go for the route→permission map.
type Handler struct {
	svc   *Service
	audit *audit.Logger
}

// GET /layout — what THIS caller should see.
//
// Every authenticated request that renders the shell hits this, so it is the
// hottest read in the module; it is also the one that must not leak. Filtering
// happens server-side (see Service.ForCaller).
func (h *Handler) Mine(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.ForCaller(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	server.JSON(w, http.StatusOK, configJSON(cfg))
}

// GET /admin/layout — everything, hidden rows included.
func (h *Handler) Full(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.svc.Full(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	server.JSON(w, http.StatusOK, configJSON(cfg))
}

// PUT /admin/layout/menu — replace the whole menu.
//
// Whole-set rather than per-row, because reordering is the common edit and a
// sequence of per-row PATCHes would leave the menu in intermediate orders that
// other admins could load.
func (h *Handler) SaveMenu(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Items []struct {
			Key        string `json:"key"`
			Label      string `json:"label"`
			Icon       string `json:"icon"`
			Href       string `json:"href"`
			Permission string `json:"permission"`
			Visible    bool   `json:"visible"`
		} `json:"items"`
	}
	if !server.Decode(w, r, &body) {
		return
	}

	items := make([]MenuItem, 0, len(body.Items))
	for _, it := range body.Items {
		items = append(items, MenuItem{
			Key: it.Key, Label: it.Label, Icon: it.Icon,
			Href: it.Href, Permission: it.Permission, Visible: it.Visible,
		})
	}
	if err := h.svc.SaveMenu(r.Context(), items); err != nil {
		writeSaveError(w, err)
		return
	}

	h.audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionLayoutMenuSaved,
		TargetKind: "layout",
		Metadata:   map[string]any{"items": len(items)},
	})
	h.Full(w, r)
}

// PUT /admin/layout/widgets — replace widget placement.
func (h *Handler) SaveWidgets(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Widgets []struct {
			Key        string `json:"key"`
			Label      string `json:"label"`
			Slot       string `json:"slot"`
			Permission string `json:"permission"`
			Visible    bool   `json:"visible"`
		} `json:"widgets"`
	}
	if !server.Decode(w, r, &body) {
		return
	}

	widgets := make([]Widget, 0, len(body.Widgets))
	for _, x := range body.Widgets {
		widgets = append(widgets, Widget{
			Key: x.Key, Label: x.Label, Slot: x.Slot,
			Permission: x.Permission, Visible: x.Visible,
		})
	}
	if err := h.svc.SaveWidgets(r.Context(), widgets); err != nil {
		writeSaveError(w, err)
		return
	}

	h.audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionLayoutWidgetsSaved,
		TargetKind: "layout",
		Metadata:   map[string]any{"widgets": len(widgets)},
	})
	h.Full(w, r)
}

// writeSaveError keeps the validation message: an admin editing a menu needs to
// know WHICH row is wrong, and a generic "invalid request" would send them
// hunting through fifteen of them.
func writeSaveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		server.Problem(w, http.StatusBadRequest,
			server.ProblemType("layout", "validation"), http.StatusText(http.StatusBadRequest), err.Error())
	case errors.Is(err, ErrUnknownWidget):
		server.Problem(w, http.StatusBadRequest,
			server.ProblemType("layout", "unknown_widget"), http.StatusText(http.StatusBadRequest), err.Error())
	default:
		server.Internal(w)
	}
}

func configJSON(c Config) map[string]any {
	menu := make([]map[string]any, 0, len(c.Menu))
	for _, it := range c.Menu {
		menu = append(menu, map[string]any{
			"id":         it.ID,
			"key":        it.Key,
			"label":      it.Label,
			"icon":       it.Icon,
			"href":       nilIfEmpty(it.Href),
			"permission": nilIfEmpty(it.Permission),
			"position":   it.Position,
			"visible":    it.Visible,
			"is_system":  it.IsSystem,
		})
	}
	widgets := make([]map[string]any, 0, len(c.Widgets))
	for _, x := range c.Widgets {
		widgets = append(widgets, map[string]any{
			"id":         x.ID,
			"key":        x.Key,
			"label":      x.Label,
			"slot":       x.Slot,
			"permission": nilIfEmpty(x.Permission),
			"position":   x.Position,
			"visible":    x.Visible,
		})
	}
	return map[string]any{"menu": menu, "widgets": widgets}
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
