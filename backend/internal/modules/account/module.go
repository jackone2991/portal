// Package account is the registration entry-point for the account module.
//
// The account module owns: users, authentication (OIDC + JWT + refresh), the
// RBAC engine (roles, permissions, policies), 2FA/TOTP, sessions, and audit
// logging. Other modules talk to this one only via the api/ subpackage.
//
// Wiring contract:
//
//	m, err := account.New(deps)
//	m.MountHTTP(apiRouter)        // wires /auth/*, /me/sessions
//	m.RegisterTasks(asynqMux)     // wires notify:* tasks
//
// The module is constructed once per binary; routes/tasks are registered by
// the cmd/api and cmd/worker entry points respectively.
package account

import (
	"errors"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	accountapi "github.com/portal/backend/internal/modules/account/api"
	"github.com/portal/backend/internal/modules/account/audit"
	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/handler"
	accountmw "github.com/portal/backend/internal/modules/account/middleware"
	"github.com/portal/backend/internal/modules/account/rbac"
)

// Deps are the cross-cutting infrastructure dependencies the account module
// needs. Provided by cmd/api or cmd/worker at construction time. No globals.
type Deps struct {
	Redis           *redis.Client
	Issuer          *auth.Issuer
	Verifier        *auth.Verifier
	Refresh         *auth.RefreshManager
	OIDC            *auth.OIDC
	SnapshotFetcher accountmw.AuthSnapshotFetcher
	PermFetcher     rbac.PermissionFetcher
	UserUpserter    handler.UserUpserter
	AuditStore      audit.EventStore
	CacheTTL        time.Duration
}

// Module is the runtime handle for the account domain.
type Module struct {
	deps      Deps
	engine    *rbac.Engine
	logger    *audit.Logger
	handler   *handler.AuthHandler
	publicAPI accountapi.API
}

// New constructs the module. Wires up dependent objects (engine, loader).
// Returns an error if required dependencies are missing.
func New(d Deps) (*Module, error) {
	if d.Issuer == nil || d.Verifier == nil || d.Refresh == nil {
		return nil, errors.New("account: missing Issuer/Verifier/Refresh dependency")
	}
	if d.SnapshotFetcher == nil || d.PermFetcher == nil {
		return nil, errors.New("account: missing repository adapters")
	}

	logger := audit.New(d.AuditStore)
	loader := rbac.NewCachedLoader(d.Redis, d.PermFetcher, d.CacheTTL)
	engine := rbac.NewEngine(loader)

	h := &handler.AuthHandler{
		OIDC:    d.OIDC,
		Issuer:  d.Issuer,
		Refresh: d.Refresh,
		Users:   d.UserUpserter,
		Audit:   logger,
	}

	return &Module{
		deps:      d,
		engine:    engine,
		logger:    logger,
		handler:   h,
		publicAPI: accountapi.NewImpl(engine, d.SnapshotFetcher),
	}, nil
}

// MountHTTP attaches the module's HTTP routes onto r. Caller is responsible
// for the surrounding middleware chain (request ID, CORS, rate limit, tenant).
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Get("/login", m.handler.Login)
		r.Get("/callback", m.handler.Callback)
		r.Post("/refresh", m.handler.Refresh)

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(accountmw.RequireAuth(m.deps.Verifier, m.deps.SnapshotFetcher))
			r.Post("/logout", m.handler.Logout)
			r.Post("/logout-all", m.handler.LogoutAll)
			r.Get("/me", m.handler.Me)
		})
	})
}

// RegisterTasks attaches the module's Asynq handlers (notifications,
// auth.refresh.reuse alerting, etc.). Placeholder — wired when notifications
// subpackage lands.
func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}

// API exposes the account module's public surface to other modules.
func (m *Module) API() accountapi.API { return m.publicAPI }

// Engine exposes the RBAC engine. cmd/api needs the concrete *Engine to
// build module-specific RequirePermission middleware. This is the one
// documented exception to the api-only rule — other modules MUST NOT
// import account/rbac for any other purpose.
func (m *Module) Engine() *rbac.Engine { return m.engine }

// Logger exposes the audit logger so other modules can write audit events
// scoped to the shared audit_log table (e.g. tenant.organization.created).
func (m *Module) Logger() *audit.Logger { return m.logger }
