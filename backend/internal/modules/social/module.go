package social

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	socialapi "github.com/portal/backend/internal/modules/social/api"
)

// Deps are the cross-cutting dependencies the social module needs.
type Deps struct {
	Repo   Repository
	Events EventPublisher
	// Names resolves user ids to display names. cmd/api passes a closure over
	// accountapi.GetUserNames — this module may not read `users`.
	Names       NamesFunc
	RequireAuth func(http.Handler) http.Handler
	// RequirePermission gates each route; built by cmd/api from the RBAC engine,
	// since only account may import account/rbac.
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)
}

// Module is the runtime handle for the social domain.
type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("social: Repo is required")
	}
	svc := &Service{repo: d.Repo, names: d.Names, events: d.Events}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, currentUser: d.CurrentUser}}, nil
}

// MountHTTP wires the connection routes:
//
//	POST   /connections                  ask someone to connect   [social:write:own]
//	GET    /connections?status=…         accepted / incoming / outgoing [social:read:own]
//	GET    /connections/summary          pending-incoming count   [social:read:own]
//	POST   /connections/{id}/accept      agree                    [social:write:own]
//	DELETE /connections/{id}             withdraw / decline / disconnect [social:write:own]
//
// Ownership is not checked in the handlers: the 0037 policies restrict every
// statement to the two parties, so a connection the caller is not part of does
// not exist for their query.
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/connections", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("social:write:own")).Post("/", m.handler.Request)
		r.With(m.perm("social:read:own")).Get("/", m.handler.List)
		r.With(m.perm("social:read:own")).Get("/summary", m.handler.Summary)
		r.With(m.perm("social:write:own")).Post("/{id}/accept", m.handler.Accept)
		r.With(m.perm("social:write:own")).Delete("/{id}", m.handler.Delete)
	})
}

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}

// API exposes the module's public surface to other modules.
func (m *Module) API() socialapi.API { return m.svc }
