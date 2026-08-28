// Package middleware contains HTTP middleware shared across handlers.
//
// The auth middleware extracts and verifies the JWT, then loads the user's
// auth snapshot from the DB to check token_version + disabled state. This
// double-check is what gives us instant revocation: even with a still-valid
// JWT, a bumped token_version or a disabled_at stamp denies access.
package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/platform/server"
)

// AuthSnapshotFetcher loads the user record fields relevant to access
// decisions. Implemented by the sqlc-generated repository.
type AuthSnapshotFetcher interface {
	GetUserAuthSnapshot(ctx context.Context, id uuid.UUID) (UserAuthSnapshot, error)
}

type UserAuthSnapshot struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	TokenVersion int
	Disabled     bool
	// ApprovalStatus is "pending" | "approved" | "rejected" (migration 0031).
	// Checked on every request, not only at login, so that revoking somebody's
	// approval ends their session immediately instead of at token expiry.
	ApprovalStatus string
}

// ApprovalApproved is the only approval state that may hold a session. Declared
// here rather than imported from handler because middleware must not depend on
// the handler package — handler already imports middleware.
const ApprovalApproved = "approved"

// AccessCookieName is the HttpOnly Secure SameSite=Strict cookie that holds
// the access token for browser clients. API clients send Authorization headers.
const AccessCookieName = "portal_access"

// RequireAuth verifies the bearer token (header or cookie) and attaches the
// resulting Identity to the request context. Anonymous requests are rejected
// with 401.
//
// On any failure mode (bad token, expired, revoked, disabled user) the same
// generic 401 + JSON error is emitted; specifics live only in the audit log.
// This avoids tipping off attackers about token state.
func RequireAuth(verifier *auth.Verifier, fetcher AuthSnapshotFetcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := authenticate(r, verifier, fetcher)
			if err != nil {
				// An unapproved account is the one failure mode worth naming: the
				// caller has already proved who they are, so saying so leaks
				// nothing, and a bare 401 would send the frontend to the login
				// screen where signing in again cannot possibly help.
				if errors.Is(err, auth.ErrUserNotApproved) {
					writeJSONError(w, http.StatusForbidden, "account_not_approved",
						"this account is awaiting approval")
					return
				}
				writeJSONError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			ctx := auth.WithIdentity(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth runs the same verification as RequireAuth but lets anonymous
// requests through (no Identity attached). Use for endpoints that have both
// public and authenticated behavior (e.g. movie detail page with edit button).
func OptionalAuth(verifier *auth.Verifier, fetcher AuthSnapshotFetcher) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := authenticate(r, verifier, fetcher)
			if err == nil {
				r = r.WithContext(auth.WithIdentity(r.Context(), id))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func authenticate(r *http.Request, verifier *auth.Verifier, fetcher AuthSnapshotFetcher) (*auth.Identity, error) {
	raw, ok := extractToken(r)
	if !ok {
		return nil, auth.ErrTokenInvalid
	}
	claims, err := verifier.Verify(raw)
	if err != nil {
		return nil, err
	}

	// Sub must parse as a UUID — any deviation is malformed.
	uid, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, auth.ErrTokenInvalid
	}

	// DB-side revocation check: confirms token_version still matches and the
	// user has not been disabled. Single indexed PK lookup.
	snap, err := fetcher.GetUserAuthSnapshot(r.Context(), uid)
	if err != nil {
		return nil, auth.ErrTokenInvalid
	}
	if snap.Disabled {
		return nil, auth.ErrUserDisabled
	}
	if snap.ApprovalStatus != "" && snap.ApprovalStatus != ApprovalApproved {
		return nil, auth.ErrUserNotApproved
	}
	if snap.TokenVersion != claims.TokenVersion {
		return nil, auth.ErrTokenRevoked
	}

	return &auth.Identity{
		UserID:       uid,
		Email:        snap.Email,
		DisplayName:  snap.DisplayName,
		TokenID:      claims.ID,
		TokenVersion: snap.TokenVersion,
		Roles:        claims.Roles,
	}, nil
}

// extractToken pulls a bearer token from the Authorization header or the
// access cookie, in that order. Returns false if neither is present.
func extractToken(r *http.Request) (string, bool) {
	if h := r.Header.Get("Authorization"); h != "" {
		const prefix = "Bearer "
		if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
			tok := strings.TrimSpace(h[len(prefix):])
			if tok != "" {
				return tok, true
			}
		}
	}
	if c, err := r.Cookie(AccessCookieName); err == nil && c.Value != "" {
		return c.Value, true
	}
	return "", false
}

// writeJSONError is shared by middleware error paths. Defined here to avoid
// a circular import on the handler package.
//
// It answers with RFC 7807, like every handler does. These middleware 401/403s
// were the last place still writing the legacy {code, message} body, and they
// are the most-hit error path in the API — so the frontend's
// problemDisplayMessage found no `detail` and fell back to a generic string on
// every auth failure. `code` carries through as the problem type, which keeps
// the existing vocabulary and drops the hand-concatenated JSON.
func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	server.Problem(w, status, server.ProblemType("account", code), http.StatusText(status), msg)
}

// errorMatches is a small helper used by tests; kept exported in case other
// middleware needs to discriminate. (Unused at runtime.)
func errorMatches(err, target error) bool { return errors.Is(err, target) }
