package middleware

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/rbac"
)

// RequirePermission rejects requests whose principal lacks the given perm.
// Must be chained AFTER RequireAuth (otherwise 401 is returned, since no
// Identity is on the context).
//
// The required code is parsed once at middleware build time; a malformed
// code panics — surfacing a programmer error before the server starts.
func RequirePermission(engine *rbac.Engine, code string) func(http.Handler) http.Handler {
	required := rbac.MustParse(code)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.FromContext(r.Context())
			if !ok || id.IsAnonymous() {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			err := engine.Authorize(r.Context(), rbac.Principal{
				UserID:       id.UserID,
				TokenVersion: id.TokenVersion,
			}, required)
			switch {
			case err == nil:
				next.ServeHTTP(w, r)
			case errors.Is(err, rbac.ErrDenied):
				writeJSONError(w, http.StatusForbidden, "forbidden", "permission denied")
			default:
				writeJSONError(w, http.StatusInternalServerError, "internal", "authorization error")
			}
		})
	}
}

// OwnerExtractor pulls the resource owner's UUID from a request. Most often
// this is a database lookup keyed by URL parameter.
type OwnerExtractor func(r *http.Request) (uuid.UUID, error)

// RequireOwnerOrPermission is the canonical "user can act on their own
// resource OR an admin with elevated perm can act on anyone's" pattern.
//
// The handler wires:
//
//	RequireOwnerOrPermission(engine, "assets:delete:any", extractAssetOwner)
//
// extractAssetOwner reads {id} from chi, looks up the asset, returns owner.
func RequireOwnerOrPermission(
	engine *rbac.Engine,
	elevatedCode string,
	extractor OwnerExtractor,
) func(http.Handler) http.Handler {
	elevated := rbac.MustParse(elevatedCode)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.FromContext(r.Context())
			if !ok || id.IsAnonymous() {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			ownerID, err := extractor(r)
			if err != nil {
				writeJSONError(w, http.StatusNotFound, "not_found", "resource not found")
				return
			}
			err = engine.AuthorizeOwnerOr(r.Context(),
				rbac.Principal{UserID: id.UserID, TokenVersion: id.TokenVersion},
				ownerID, elevated)
			switch {
			case err == nil:
				next.ServeHTTP(w, r)
			case errors.Is(err, rbac.ErrDenied):
				writeJSONError(w, http.StatusForbidden, "forbidden", "permission denied")
			default:
				writeJSONError(w, http.StatusInternalServerError, "internal", "authorization error")
			}
		})
	}
}

// RequireRole checks role membership without translating to permissions.
// Prefer RequirePermission for normal access decisions; use this only for
// audit-style filters ("admin-only dashboard").
func RequireRole(roleCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := auth.FromContext(r.Context())
			if !ok || id.IsAnonymous() {
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			for _, code := range id.Roles {
				if code == roleCode {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeJSONError(w, http.StatusForbidden, "forbidden", "role required")
		})
	}
}
