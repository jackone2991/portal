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
	"net/http"
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
	ListUserDirectory(ctx context.Context, exclude uuid.UUID, limit int) ([]accountapi.UserSummary, error)
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]accountapi.UserSummary, error)
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
	Admin           handler.AdminStore
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
	// ApprovalQueueURL is the link carried by the registration-pending
	// notification (migration 0031). Empty omits it.
	ApprovalQueueURL string
}

// Module is the runtime handle for the account domain.
type Module struct {
	deps      Deps
	engine    *rbac.Engine
	logger    *audit.Logger
	handler   *handler.AuthHandler
	admin     *handler.AdminHandler
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
		ApprovalQueueURL: d.ApprovalQueueURL,

		// /auth/me reports the caller's effective permissions so the UI can hide
		// what the API would refuse. account is the one module allowed to reach
		// its own rbac package, so the closure is built here rather than passed in.
		Perms: func(ctx context.Context, userID uuid.UUID, tokenVersion int) ([]string, error) {
			set, err := engine.Effective(ctx, rbac.Principal{UserID: userID, TokenVersion: tokenVersion})
			if err != nil {
				return nil, err
			}
			return set.Codes(), nil
		},
	}

	// The admin console is mounted only when a store is wired. A binary that
	// does not pass one (a worker, a test harness) simply has no /admin routes,
	// rather than routes that panic on first use.
	var adminH *handler.AdminHandler
	if d.Admin != nil {
		adminH = &handler.AdminHandler{Store: d.Admin, Engine: engine, Audit: logger}
	}

	return &Module{
		deps:      d,
		engine:    engine,
		logger:    logger,
		handler:   h,
		admin:     adminH,
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

	m.mountAdmin(r)
}

// mountAdmin wires the console under /admin. Every route is authenticated and
// then permission-gated; the map below IS the authorization policy for this
// surface, so read it as the spec rather than looking for checks inside the
// handlers.
//
//	users:read:any    — see the directory and the approval queue
//	users:approve     — approve / reject / revoke a registration  (superadmin only
//	                    by default: migration 0031 grants it to no role, and
//	                    superadmin reaches it through the '*' wildcard)
//	users:write:any   — create an account, edit one, disable / enable it
//	users:delete:any  — delete an account permanently (cascades: see DeleteUser)
//	rbac:role:assign  — change which roles a user holds
//	rbac:role:read    — read roles, permissions and the matrix
//	rbac:role:write   — create / edit / delete roles and edit the matrix
//
// account is the one module allowed to reach into its own rbac package, so it
// builds this middleware itself instead of taking it from cmd/api.
func (m *Module) mountAdmin(r chi.Router) {
	if m.admin == nil {
		return
	}
	perm := func(code string) func(http.Handler) http.Handler {
		return accountmw.RequirePermission(m.engine, code)
	}

	r.Route("/admin", func(r chi.Router) {
		r.Use(accountmw.RequireAuth(m.deps.Verifier, m.deps.SnapshotFetcher))

		r.Group(func(r chi.Router) {
			r.Use(perm("users:read:any"))
			r.Get("/users", m.admin.ListUsers)
			r.Get("/users/{id}", m.admin.GetUser)
		})
		r.Group(func(r chi.Router) {
			r.Use(perm("users:approve"))
			r.Post("/users/{id}/approve", m.admin.Approve)
			r.Post("/users/{id}/reject", m.admin.Reject)
			r.Post("/users/{id}/revoke-approval", m.admin.RevokeApproval)
		})
		r.Group(func(r chi.Router) {
			r.Use(perm("users:write:any"))
			r.Post("/users", m.admin.CreateUser)
			r.Patch("/users/{id}", m.admin.UpdateUser)
			r.Post("/users/{id}/disable", m.admin.SetDisabled(true))
			r.Post("/users/{id}/enable", m.admin.SetDisabled(false))
		})
		r.With(perm("users:delete:any")).Delete("/users/{id}", m.admin.DeleteUser)
		r.With(perm("rbac:role:assign")).Put("/users/{id}/roles", m.admin.SetUserRoles)

		r.With(perm("rbac:role:read")).Get("/permission-matrix", m.admin.PermissionMatrix)
		r.Group(func(r chi.Router) {
			r.Use(perm("rbac:role:write"))
			r.Post("/roles", m.admin.CreateRole)
			r.Patch("/roles/{id}", m.admin.UpdateRole)
			r.Delete("/roles/{id}", m.admin.DeleteRole)
			r.Put("/roles/{id}/permissions", m.admin.SetRolePermissions)
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
