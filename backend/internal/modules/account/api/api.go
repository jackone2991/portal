// Package api is the public face of the account module.
//
// Other modules import this package — and only this package — when they
// need to call into account-domain functionality. Internals (auth, rbac,
// handler, service) are private to the module and may not be imported
// from outside.
//
// Test doubles: implement the API interface in your test, no need to
// stand up the whole module.
package api

import (
	"context"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/rbac"
)

// UserSummary is the projection of a user safe to share across modules.
// Sensitive fields (token_version, disabled_at, totp_*) intentionally absent.
type UserSummary struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
}

// API is the contract other modules program against.
type API interface {
	// GetUserByID returns a small projection of the user. Returns
	// (nil, nil) if the user does not exist or is disabled — callers
	// MUST handle the nil case explicitly.
	GetUserByID(ctx context.Context, id uuid.UUID) (*UserSummary, error)

	// HasPermission reports whether the principal currently in ctx has
	// the given permission code. Wraps rbac.Engine.Authorize so callers
	// do not need to import rbac directly.
	//
	// Returns false on any error (fail-closed) — callers do not need to
	// distinguish "user does not have it" from "lookup failed".
	HasPermission(ctx context.Context, code string) bool
}

// snapshotFetcher is the subset of account/middleware.AuthSnapshotFetcher
// that the API needs. Duplicated here to avoid importing middleware from
// the api package (which would create a cycle).
type snapshotFetcher interface {
	GetUserSummaryByID(ctx context.Context, id uuid.UUID) (*UserSummary, error)
}

// Impl is the concrete API implementation. Constructed by account.New;
// no other code creates one. Exported so account.Module can return it.
type Impl struct {
	engine *rbac.Engine
	users  snapshotFetcher
}

// NewImpl is internal to the module's wiring; only account.New calls it.
func NewImpl(engine *rbac.Engine, users snapshotFetcher) *Impl {
	return &Impl{engine: engine, users: users}
}

func (a *Impl) GetUserByID(ctx context.Context, id uuid.UUID) (*UserSummary, error) {
	return a.users.GetUserSummaryByID(ctx, id)
}

func (a *Impl) HasPermission(ctx context.Context, code string) bool {
	// Re-use the engine's principal-from-context helper once it lands.
	// For now: needs auth.FromContext(ctx) → engine.Authorize.
	// Implementation deferred until cmd/api wiring is in place.
	_ = ctx
	_ = code
	return false
}
