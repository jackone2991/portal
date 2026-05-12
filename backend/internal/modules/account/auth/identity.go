package auth

import (
	"github.com/google/uuid"
)

// Identity is the verified, in-process projection of an authenticated user.
// It is populated by the Auth middleware after JWT verification + DB snapshot
// check, and lives on the request context.
//
// Fields here are immutable for the duration of a request. A subsequent
// request will re-verify; never mutate or cache an Identity beyond a request.
type Identity struct {
	UserID       uuid.UUID
	Email        string
	DisplayName  string
	TokenID      string // jti — useful for logout/revocation
	TokenVersion int    // matches users.token_version at verify time
	Roles        []string
}

// IsAnonymous reports whether the request was unauthenticated. Used by
// handlers that allow optional auth.
func (i *Identity) IsAnonymous() bool {
	return i == nil || i.UserID == uuid.Nil
}
