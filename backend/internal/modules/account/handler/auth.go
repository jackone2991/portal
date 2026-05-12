// Package handler holds HTTP handlers. The auth handler implements:
//
//	GET  /auth/login          — start OIDC auth code flow
//	GET  /auth/callback       — finish OIDC, mint access + refresh tokens
//	POST /auth/refresh        — rotate refresh token, mint new access token
//	POST /auth/logout         — revoke current refresh; bumps token_version
//	POST /auth/logout-all     — revoke every refresh; bumps token_version
//	GET  /auth/me             — return identity + roles
//
// Cookies set on the auth domain:
//
//	portal_access   — short-lived access token (5min, HttpOnly Secure SameSite=Strict)
//	portal_refresh  — refresh token plaintext (long-lived, Path=/auth, same flags)
//	portal_oidc     — short-lived state+nonce binding for OIDC callback
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/audit"
	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/middleware"
)

// AuthHandler wires together the auth dependencies. Construct at startup.
type AuthHandler struct {
	OIDC           *auth.OIDC
	Issuer         *auth.Issuer
	Refresh        *auth.RefreshManager
	Users          UserUpserter
	Audit          *audit.Logger

	AccessTTL      time.Duration
	RefreshTTL     time.Duration
	CookieDomain   string
	CookieSecure   bool          // false in dev (http://localhost), true everywhere else
	PostLoginURL   string        // where to send the browser after a successful login
}

// UserUpserter is the subset of the user repository used here.
type UserUpserter interface {
	UpsertUserFromOIDC(ctx context.Context, in UpsertUserInput) (UpsertedUser, error)
	GetUserAuthSnapshot(ctx context.Context, id uuid.UUID) (middleware.UserAuthSnapshot, error)
	BumpUserTokenVersion(ctx context.Context, id uuid.UUID) (int, error)
	ListUserRoleCodes(ctx context.Context, id uuid.UUID) ([]string, error)
}

type UpsertUserInput struct {
	OIDCSubject string
	Email       string
	DisplayName string
	AvatarURL   string
}

type UpsertedUser struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	TokenVersion int
}

// ── /auth/login ────────────────────────────────────────────────────────────
//
// Generates state + nonce, stores both in a short-lived signed cookie, and
// redirects to the IdP. The cookie binds *this* browser to the upcoming
// callback — protects against CSRF and ID-token replay.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	state, err1 := auth.RandomState()
	nonce, err2 := auth.RandomState()
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not start login")
		return
	}

	// 5-minute window to complete the flow; binds state+nonce to the browser.
	bindCookie, err := json.Marshal(map[string]string{"s": state, "n": nonce})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not start login")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "portal_oidc",
		Value:    string(bindCookie),
		Path:     "/auth",
		MaxAge:   int((5 * time.Minute).Seconds()),
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteLaxMode, // Lax: needed because the IdP redirects back on the top-level navigation
	})

	http.Redirect(w, r, h.OIDC.AuthCodeURL(state, nonce), http.StatusFound)
}

// ── /auth/callback ─────────────────────────────────────────────────────────
//
// Validates state (CSRF) + nonce (ID token replay), exchanges code, upserts
// the user, mints access + refresh tokens, sets cookies, redirects to the app.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	state := q.Get("state")
	code := q.Get("code")
	if state == "" || code == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "missing state or code")
		return
	}

	bind, err := r.Cookie("portal_oidc")
	if err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "missing oidc binding")
		return
	}
	clearOIDCCookie(w, h.CookieSecure)

	var bound struct{ S, N string }
	if err := json.Unmarshal([]byte(bind.Value), &bound); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid oidc binding")
		return
	}
	if subtleEqual(bound.S, state) != true {
		writeError(w, http.StatusBadRequest, "bad_request", "state mismatch")
		return
	}

	claims, err := h.OIDC.Exchange(r.Context(), code, bound.N)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized", "oidc exchange failed")
		return
	}

	user, err := h.Users.UpsertUserFromOIDC(r.Context(), UpsertUserInput{
		OIDCSubject: claims.Subject,
		Email:       claims.Email,
		DisplayName: nonEmpty(claims.Name, claims.Email),
		AvatarURL:   claims.Picture,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not provision user")
		return
	}

	roles, _ := h.Users.ListUserRoleCodes(r.Context(), user.ID)

	access, err := h.Issuer.Issue(auth.IssueInput{
		UserID:       user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		Roles:        roles,
		TokenVersion: user.TokenVersion,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not issue token")
		return
	}

	ip := clientIP(r)
	refresh, err := h.Refresh.Issue(r.Context(), user.ID, ip, r.UserAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not issue refresh")
		return
	}

	h.setSessionCookies(w, access, refresh.Plaintext)
	h.Audit.Write(r.Context(), audit.Event{
		Action:    audit.ActionAuthLogin,
		ActorID:   &user.ID,
		IP:        ip,
		UserAgent: r.UserAgent(),
	})

	redirect := h.PostLoginURL
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// ── /auth/refresh ──────────────────────────────────────────────────────────
//
// Reads refresh cookie OR `refresh_token` body field (for non-browser clients),
// rotates it, mints a new access token. Detects reuse and burns the chain.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	plaintext := h.extractRefreshToken(r)
	if plaintext == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized", "missing refresh token")
		return
	}

	ip := clientIP(r)
	res, userID, err := h.Refresh.Rotate(r.Context(), plaintext, ip, r.UserAgent())
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrTokenReused):
			h.Audit.Write(r.Context(), audit.Event{
				Action:    audit.ActionAuthRefreshReuse,
				ActorID:   &userID,
				IP:        ip,
				UserAgent: r.UserAgent(),
			})
		}
		clearSessionCookies(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "unauthorized", "refresh failed")
		return
	}

	snap, err := h.Users.GetUserAuthSnapshot(r.Context(), res.Row.UserID)
	if err != nil || snap.Disabled {
		clearSessionCookies(w, h.CookieSecure)
		writeError(w, http.StatusUnauthorized, "unauthorized", "user unavailable")
		return
	}
	roles, _ := h.Users.ListUserRoleCodes(r.Context(), snap.ID)

	access, err := h.Issuer.Issue(auth.IssueInput{
		UserID:       snap.ID,
		Email:        snap.Email,
		DisplayName:  snap.DisplayName,
		Roles:        roles,
		TokenVersion: snap.TokenVersion,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not issue token")
		return
	}

	h.setSessionCookies(w, access, res.Plaintext)
	h.Audit.Write(r.Context(), audit.Event{
		Action:    audit.ActionAuthRefresh,
		ActorID:   &snap.ID,
		IP:        ip,
		UserAgent: r.UserAgent(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"expires_in":   int(h.AccessTTL.Seconds()),
		"token_type":   "Bearer",
	})
}

// ── /auth/logout ───────────────────────────────────────────────────────────
//
// Revokes the current refresh token and clears cookies. Bumps token_version
// so the still-valid access token can no longer be used (DB check rejects it).
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	plaintext := h.extractRefreshToken(r)
	if plaintext != "" {
		_ = h.Refresh.Revoke(r.Context(), plaintext, "logout")
	}
	if id, ok := auth.FromContext(r.Context()); ok && !id.IsAnonymous() {
		_, _ = h.Users.BumpUserTokenVersion(r.Context(), id.UserID)
		h.Audit.Write(r.Context(), audit.Event{
			Action:    audit.ActionAuthLogout,
			ActorID:   &id.UserID,
			IP:        clientIP(r),
			UserAgent: r.UserAgent(),
		})
	}
	clearSessionCookies(w, h.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// ── /auth/logout-all ──────────────────────────────────────────────────────
//
// Revokes every refresh token for the user (across all devices) AND bumps
// token_version. Use after suspected compromise or password change.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || id.IsAnonymous() {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	_ = h.Refresh.RevokeAllForUser(r.Context(), id.UserID, "logout_all")
	_, _ = h.Users.BumpUserTokenVersion(r.Context(), id.UserID)
	h.Audit.Write(r.Context(), audit.Event{
		Action:    audit.ActionAuthLogout,
		ActorID:   &id.UserID,
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
		Metadata:  map[string]any{"scope": "all_sessions"},
	})
	clearSessionCookies(w, h.CookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

// ── /auth/me ──────────────────────────────────────────────────────────────
//
// Returns identity, roles, and the canonical permission codes that the
// current access token bears. The frontend uses this to decide which UI
// affordances to render (e.g. show an "Upload" button only if perm exists).
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || id.IsAnonymous() {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":           id.UserID,
		"email":        id.Email,
		"display_name": id.DisplayName,
		"roles":        id.Roles,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────

func (h *AuthHandler) setSessionCookies(w http.ResponseWriter, access, refresh string) {
	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AccessCookieName,
		Value:    access,
		Path:     "/",
		Domain:   h.CookieDomain,
		MaxAge:   int(h.AccessTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "portal_refresh",
		Value:    refresh,
		Path:     "/auth",
		Domain:   h.CookieDomain,
		MaxAge:   int(h.RefreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearSessionCookies(w http.ResponseWriter, secure bool) {
	for _, c := range []http.Cookie{
		{Name: middleware.AccessCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode},
		{Name: "portal_refresh", Path: "/auth", MaxAge: -1, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode},
	} {
		c := c
		http.SetCookie(w, &c)
	}
}

func clearOIDCCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "portal_oidc",
		Path:     "/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *AuthHandler) extractRefreshToken(r *http.Request) string {
	if c, err := r.Cookie("portal_refresh"); err == nil && c.Value != "" {
		return c.Value
	}
	if r.Method == http.MethodPost && r.Header.Get("Content-Type") == "application/json" {
		var body struct{ RefreshToken string `json:"refresh_token"` }
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			return body.RefreshToken
		}
	}
	return ""
}
