package layout

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/portal/backend/internal/platform/audit"
)

// Deps are the layout module's dependencies, filled by cmd/api.
type Deps struct {
	Repo Repository

	// Perms answers "may the caller in ctx do X" for the server-side filtering in
	// ForCaller. Satisfied by account/api — layout never imports account/rbac.
	// Nil degrades the shell to unrestricted rows only, never to everything.
	Perms PermissionChecker

	Audit *audit.Logger

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
}

// Module is the runtime handle for the layout domain.
type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("layout: missing Repo")
	}
	svc := &Service{repo: d.Repo, perms: d.Perms}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, audit: d.Audit}}, nil
}

// MountHTTP wires the shell read and the admin console:
//
//	GET /layout                  the caller's own menu + widgets   [any signed-in user]
//	GET /admin/layout            everything, hidden rows included  [system:settings:write]
//	PUT /admin/layout/menu       replace the whole menu            [system:settings:write]
//	PUT /admin/layout/widgets    replace widget placement          [system:settings:write]
//
// GET /layout is authenticated but not permission-gated on purpose: the shell
// cannot render without it, and its RESULT is already filtered per caller. A
// permission gate there would only mean nobody has a sidebar.
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/layout", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.Get("/", m.handler.Mine)
	})

	r.Route("/admin/layout", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.Use(m.perm(PermissionWrite))
		r.Get("/", m.handler.Full)
		r.Put("/menu", m.handler.SaveMenu)
		r.Put("/widgets", m.handler.SaveWidgets)
	})
}

// No api/ package yet: nothing outside this module consumes the layout, and an
// empty public surface would be ceremony rather than a contract. Add one (per
// MODULES.md §8) the first time another module needs to read it.

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}
