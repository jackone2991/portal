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

	"github.com/portal/backend/internal/modules/account/auth"
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

	// ListDirectory returns other people with an account on this Portal —
	// approved, not disabled, excluding `exclude` (the caller). Names and ids
	// only: this is the roster a "people you may know" suggestion is built from,
	// not a profile feed.
	//
	// Deliberately NOT permission-gated at this layer. The caller decides who may
	// see it; today the only caller is people/suggestions, which any signed-in
	// user may read — on a self-hosted instance the accounts are a household or a
	// small team, and knowing who else is here is the point.
	ListDirectory(ctx context.Context, exclude uuid.UUID, limit int) ([]UserSummary, error)

	// GetUserNames resolves ids to display names in one round trip. Modules that
	// reference a user by id — social's connections, for one — use this instead
	// of joining `users`, which MODULES.md forbids.
	GetUserNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// snapshotFetcher is the subset of account/middleware.AuthSnapshotFetcher
// that the API needs. Duplicated here to avoid importing middleware from
// the api package (which would create a cycle).
type snapshotFetcher interface {
	GetUserSummaryByID(ctx context.Context, id uuid.UUID) (*UserSummary, error)
	ListUserDirectory(ctx context.Context, exclude uuid.UUID, limit int) ([]UserSummary, error)
	GetUsersByIDs(ctx context.Context, ids []uuid.UUID) ([]UserSummary, error)
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

func (a *Impl) GetUserNames(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	users, err := a.users.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		out[u.ID] = u.DisplayName
	}
	return out, nil
}

func (a *Impl) ListDirectory(ctx context.Context, exclude uuid.UUID, limit int) ([]UserSummary, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return a.users.ListUserDirectory(ctx, exclude, limit)
}

// HasPermission resolves the principal from the request context and asks the
// engine. It was a stub returning false until the layout module needed it — a
// contract that silently denies everything is worse than one that is missing,
// because a caller cannot tell "no" from "not implemented".
//
// Fail-closed on every failure path (no identity, malformed code, loader error),
// which is what the doc comment on the interface promises.
func (a *Impl) HasPermission(ctx context.Context, code string) bool {
	if a.engine == nil {
		return false
	}
	id, ok := auth.FromContext(ctx)
	if !ok || id.IsAnonymous() {
		return false
	}
	required, err := rbac.Parse(code)
	if err != nil {
		return false
	}
	return a.engine.Authorize(ctx, rbac.Principal{
		UserID:       id.UserID,
		TokenVersion: id.TokenVersion,
	}, required) == nil
}
