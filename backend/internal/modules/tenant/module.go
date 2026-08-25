// Package tenant owns organizations, organization_memberships, and the
// tenant-context plumbing that pins app.current_tenant on the DB session.
//
// It is loaded BEFORE the domain modules because its RequireTenant middleware is
// the gatekeeper that will make RLS work (ADR-07). Increment 1 wires the
// foundation without enforcing RLS: RequireTenant resolves the caller's personal
// org and opens a tenant-scoped transaction, but only /me/organizations is
// mounted through it — the domain modules keep their current (pool) behavior.
package tenant

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	tenantapi "github.com/portal/backend/internal/modules/tenant/api"
	tenantmw "github.com/portal/backend/internal/modules/tenant/middleware"
	platformdb "github.com/portal/backend/internal/platform/db"
	"github.com/portal/backend/internal/platform/server"
)

// Deps for the tenant module.
type Deps struct {
	DB          *platformdb.DB
	Store       tenantapi.Store
	RequireAuth func(http.Handler) http.Handler
	CurrentUser tenantmw.CurrentUser
}

type Module struct {
	deps      Deps
	publicAPI tenantapi.API
}

func New(d Deps) (*Module, error) {
	if d.DB == nil || d.Store == nil {
		return nil, errors.New("tenant: DB and Store are required")
	}
	return &Module{deps: d, publicAPI: tenantapi.NewImpl(d.Store)}, nil
}

// RequireTenant returns the middleware that opens the per-request tenant scope.
// Domain modules wrap their authenticated routes with it in a later increment;
// today it guards /me/organizations as the reference wiring.
func (m *Module) RequireTenant() func(http.Handler) http.Handler {
	return tenantmw.RequireTenant(m.deps.DB, m.deps.Store, m.deps.CurrentUser)
}

// MountHTTP wires GET /me/organizations (the caller's orgs). It runs through
// RequireAuth → RequireTenant, exercising the full BeginTenantScope path.
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/me/organizations", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.Use(m.RequireTenant())
		r.Get("/", m.listOrganizations)
	})
}

func (m *Module) listOrganizations(w http.ResponseWriter, r *http.Request) {
	uid, _, ok := m.deps.CurrentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	orgs, err := m.deps.Store.ListForUser(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not list organizations")
		return
	}
	out := make([]map[string]any, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, map[string]any{"id": o.ID, "kind": o.Kind, "slug": o.Slug, "name": o.Name})
	}
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(map[string]any{"organizations": out})
	_, _ = w.Write(b)
}

func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}

func (m *Module) API() tenantapi.API { return m.publicAPI }

// writeErr answers with RFC 7807. The legacy {code, message} body this used to
// write is retired (ADR-10); `code` is carried through as the problem type so
// every existing call site keeps its vocabulary and gains the standard shape.
func writeErr(w http.ResponseWriter, status int, code, msg string) {
	server.Problem(w, status, server.ProblemType("tenant", code), http.StatusText(status), msg)
}
