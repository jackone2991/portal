// Package tenant owns organizations, organization_memberships, and the
// tenant-context plumbing that pins app.current_tenant on the DB session.
//
// This module is loaded BEFORE every domain module that touches tenant-
// scoped data, because its RequireTenant middleware is the gatekeeper that
// makes RLS work.
package tenant

import (
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	tenantapi "github.com/portal/backend/internal/modules/tenant/api"
)

// Deps for the tenant module — kept minimal until the implementation lands.
type Deps struct {
	// DB        *db.DB
	// AuditLog  audit.Logger
}

type Module struct {
	deps      Deps
	publicAPI tenantapi.API
}

func New(d Deps) (*Module, error) {
	return &Module{
		deps:      d,
		publicAPI: tenantapi.NewImpl(),
	}, nil
}

// MountHTTP — placeholder. Will wire:
//   GET    /me/organizations
//   POST   /auth/switch-tenant
//   POST   /admin/organizations
//   GET    /admin/organizations
func (m *Module) MountHTTP(_ chi.Router) {}

func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}

func (m *Module) API() tenantapi.API { return m.publicAPI }
