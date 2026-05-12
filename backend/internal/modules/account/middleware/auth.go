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
}

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
func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"code":"` + code + `","message":"` + msg + `"}`))
}

// errorMatches is a small helper used by tests; kept exported in case other
// middleware needs to discriminate. (Unused at runtime.)
func errorMatches(err, target error) bool { return errors.Is(err, target) }
