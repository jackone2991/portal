// Package auth implements authentication for Portal: local password login
// (Argon2id), JWT access tokens with key rotation and token-version revocation,
// and refresh tokens with rotation + reuse detection.
//
// Authorization (role/permission decisions) lives in package rbac; auth only
// answers "who is this principal?", not "what may they do?".
package auth

import "errors"

// Sentinel errors returned by Verify and refresh exchange. Handlers translate
// these to HTTP 401/403; *do not* leak the underlying reason to the client.
var (
	ErrTokenInvalid = errors.New("auth: token invalid")
	ErrTokenExpired = errors.New("auth: token expired")
	ErrTokenRevoked = errors.New("auth: token revoked")
	ErrUserDisabled = errors.New("auth: user disabled")
	// ErrUserNotApproved: the credentials and the token are both fine, but the
	// registration has not been approved (or was rejected / revoked). Distinct
	// from ErrUserDisabled because the remedy is different — a reviewer acts,
	// not the user — and the middleware answers 403, not 401.
	ErrUserNotApproved = errors.New("auth: user not approved")
	ErrTokenReused     = errors.New("auth: refresh token reuse detected")
	ErrResetTokenUsed  = errors.New("auth: password reset token already used")
	ErrUnknownKey      = errors.New("auth: unknown signing key id")
	ErrUnsupportedAlg  = errors.New("auth: unsupported signing algorithm")
)
