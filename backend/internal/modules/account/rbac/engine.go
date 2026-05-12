package rbac

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrDenied is returned by Engine.Authorize when the principal lacks the
// requested permission. Callers translate this into HTTP 403.
var ErrDenied = errors.New("rbac: permission denied")

// PermissionLoader fetches the effective permission set for a user.
// Implementations should aggregate over all assigned roles + ancestors.
//
// The token-version argument lets the loader namespace cache entries so a
// bumped token_version automatically misses the cache and re-resolves.
type PermissionLoader interface {
	LoadEffective(ctx context.Context, userID uuid.UUID, tokenVersion int) (Set, error)
}

// Engine is the central authorization decision point. It is intentionally
// stateless; mutable state lives in the underlying loader/cache.
//
// Use one Engine per process. Construct via NewEngine.
type Engine struct {
	loader PermissionLoader
}

func NewEngine(loader PermissionLoader) *Engine {
	return &Engine{loader: loader}
}

// Principal is the smallest projection of identity needed for a perm check.
// The auth middleware builds this from a verified JWT.
type Principal struct {
	UserID       uuid.UUID
	TokenVersion int
}

// Authorize resolves the principal's effective permissions and returns
// ErrDenied if `required` is not satisfied. Other errors propagate.
//
// For "owner OR perm" decisions, see AuthorizeOwnerOr.
func (e *Engine) Authorize(ctx context.Context, p Principal, required Permission) error {
	set, err := e.loader.LoadEffective(ctx, p.UserID, p.TokenVersion)
	if err != nil {
		return err
	}
	if !set.Allows(required) {
		return ErrDenied
	}
	return nil
}

// AuthorizeOwnerOr permits the call if either:
//   1. the principal owns the resource (ownerID == principal), or
//   2. the principal has the elevated permission (e.g. ":any" variant).
//
// This is the canonical pattern for endpoints like DELETE /assets/{id}:
// owners can delete their own; admins (with assets:delete:any) can delete any.
func (e *Engine) AuthorizeOwnerOr(
	ctx context.Context,
	p Principal,
	ownerID uuid.UUID,
	elevated Permission,
) error {
	if ownerID == p.UserID {
		return nil
	}
	return e.Authorize(ctx, p, elevated)
}
