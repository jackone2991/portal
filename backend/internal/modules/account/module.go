// Package account is the registration entry-point for the account module.
//
// The account module owns: users, authentication (local password + JWT + refresh), the
// RBAC engine (roles, permissions, policies), 2FA/TOTP, sessions, and audit
// logging. Other modules talk to this one only via the api/ subpackage.
//
// Wiring contract:
//
//	m, err := account.New(deps)
//	m.MountHTTP(apiRouter)        // wires /auth/*
//
// The module is constructed once per binary; routes are registered by the
// cmd/api entry point. account owns no Asynq tasks — password-reset and
// security-alert delivery are enqueued through notify/api (SPEC-04); the notify
// module owns the notify:* prefix.
package account

import (
	"context"
	"errors"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	accountapi "github.com/portal/backend/internal/modules/account/api"
	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/handler"
	accountmw "github.com/portal/backend/internal/modules/account/middleware"
	"github.com/portal/backend/internal/modules/account/rbac"
	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	"github.com/portal/backend/internal/platform/audit"
)

// APIUserFetcher is the projection the cross-module API (accountapi.Impl) needs.
// The same repository adapter that implements SnapshotFetcher provides it.
type APIUserFetcher interface {
	GetUserSummaryByID(ctx context.Context, id uuid.UUID) (*accountapi.UserSummary, error)
}

// Deps are the cross-cutting infrastructure dependencies the account module
// needs. Provided by cmd/api or cmd/worker at construction time. No globals.
type Deps struct {
	Redis           *redis.Client
	Issuer          *auth.Issuer
	Verifier        *auth.Verifier
	Refresh         *auth.RefreshManager
	SnapshotFetcher accountmw.AuthSnapshotFetcher
	PermFetcher     rbac.PermissionFetcher
	Users           handler.UserStore
	AuditStore      audit.EventStore
	APIUsers        APIUserFetcher
	CacheTTL        time.Duration

	// Session settings applied to the auth handler (cookie flags + token TTLs).
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieDomain string
	CookieSecure bool
	PostLoginURL string

	// Password reset (SPEC-04 P0.3). ResetTokens nil disables the forgot/reset
	// routes; Dispatch enqueues notify:dispatch via notify/api (set by cmd/api).
	ResetTokens      *auth.ResetManager
	Dispatch         func(ctx context.Context, intent notifyapi.NotificationIntent) error
	PasswordResetURL string
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
		Issuer:  d.Issuer,
		Refresh: d.Refresh,
		Users:   d.Users,
		Audit:   logger,
		Redis:   d.Redis,

		AccessTTL:    d.AccessTTL,
		RefreshTTL:   d.RefreshTTL,
		CookieDomain: d.CookieDomain,
		CookieSecure: d.CookieSecure,
		PostLoginURL: d.PostLoginURL,

		ResetTokens:      d.ResetTokens,
		Dispatch:         d.Dispatch,
		PasswordResetURL: d.PasswordResetURL,
	}

	return &Module{
		deps:      d,
		engine:    engine,
		logger:    logger,
		handler:   h,
		publicAPI: accountapi.NewImpl(engine, d.APIUsers),
	}, nil
}

// MountHTTP attaches the module's HTTP routes onto r. Caller is responsible
// for the surrounding middleware chain (request ID, CORS, rate limit, tenant).
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", m.handler.Login)
		r.Post("/register", m.handler.Register)
		r.Post("/refresh", m.handler.HandleRefresh)

		// Password reset (SPEC-04 P0.3) — public, mounted only when wired.
		if m.handler.ResetTokens != nil {
			r.Post("/forgot-password", m.handler.ForgotPassword)
			r.Post("/reset-password", m.handler.ResetPassword)
		}

		// Authenticated routes
		r.Group(func(r chi.Router) {
			r.Use(accountmw.RequireAuth(m.deps.Verifier, m.deps.SnapshotFetcher))
			r.Post("/logout", m.handler.Logout)
			r.Post("/logout-all", m.handler.LogoutAll)
			r.Get("/me", m.handler.Me)
		})
	})
}

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
