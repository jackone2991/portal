// Package handler holds HTTP handlers. The auth handler implements local
// password authentication (ADR-06):
//
//	POST /auth/login          — verify email + password, mint access + refresh
//	POST /auth/register       — create a local account, then mint tokens
//	POST /auth/refresh        — rotate refresh token, mint new access token
//	POST /auth/logout         — revoke current refresh; bumps token_version
//	POST /auth/logout-all     — revoke every refresh; bumps token_version
//	GET  /auth/me             — return identity + roles
//
// Cookies set on the auth domain:
//
//	portal_access   — short-lived access token (5min, HttpOnly Secure SameSite=Strict)
//	portal_refresh  — refresh token plaintext (long-lived, Path=/api/v1/auth, same flags)
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/middleware"
	"github.com/portal/backend/internal/modules/account/rbac"
	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/server"
)

// Brute-force guard on /auth/login. A fixed(ish) window per IP and per account;
// once either counter reaches the cap, further attempts are rejected with 429
// until the window expires. Backed by Redis; a nil client disables it.
const (
	loginMaxFailures = 5
	loginFailWindow  = 15 * time.Minute
	minPasswordLen   = 8
)

// Sentinel errors the UserStore may return so the handler can map them to the
// right status without leaking which one occurred to the client.
var (
	ErrUserNotFound = errors.New("account: user not found")
	ErrEmailTaken   = errors.New("account: email already registered")
)

// AuthHandler wires together the auth dependencies. Construct at startup.
type AuthHandler struct {
	Issuer  *auth.Issuer
	Refresh *auth.RefreshManager
	Users   UserStore
	Audit   *audit.Logger
	Redis   *redis.Client // brute-force counter store; nil disables throttling

	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieDomain string
	CookieSecure bool   // false in dev (http://localhost), true everywhere else
	PostLoginURL string // where the frontend sends the browser after login

	// Password reset (SPEC-04 P0.3). All three must be set for the
	// forgot/reset-password routes to be mounted; nil ResetTokens disables them.
	ResetTokens      *auth.ResetManager
	Dispatch         func(ctx context.Context, intent notifyapi.NotificationIntent) error // enqueues notify:dispatch
	PasswordResetURL string                                                               // reset-link base, e.g. https://portal.localhost/reset-password
	// ApprovalQueueURL is where a "someone is waiting for approval" notification
	// points. Empty just omits the link from the message.
	ApprovalQueueURL string

	// Perms resolves the caller's effective permission codes for /auth/me. Set
	// by account.New from the RBAC engine (the handler must not import rbac
	// itself). Nil simply omits the field — /auth/me still works.
	//
	// The frontend needs it to decide what to *offer*: a nav entry to a screen
	// the API will refuse is a dead end, and role names are the wrong proxy once
	// roles are editable from the matrix.
	Perms func(ctx context.Context, userID uuid.UUID, tokenVersion int) ([]string, error)
}

// UserStore is the subset of the user repository the auth handler needs.
type UserStore interface {
	GetUserByEmail(ctx context.Context, email string) (LocalUser, error)
	CreateLocalUser(ctx context.Context, in CreateLocalUserInput) (LocalUser, error)
	AssignRoleByCode(ctx context.Context, userID uuid.UUID, roleCode string) error
	GetUserAuthSnapshot(ctx context.Context, id uuid.UUID) (middleware.UserAuthSnapshot, error)
	BumpUserTokenVersion(ctx context.Context, id uuid.UUID) (int, error)
	ListUserRoleCodes(ctx context.Context, id uuid.UUID) ([]string, error)
	// SetPassword replaces the Argon2id hash (password reset, SPEC-04 P0.3).
	SetPassword(ctx context.Context, userID uuid.UUID, hash string) error
	// CountUsers + MarkApproved exist for the first-run bootstrap in Register.
	CountUsers(ctx context.Context) (int64, error)
	MarkApproved(ctx context.Context, userID uuid.UUID, approvedBy *uuid.UUID) error
	// ListPermissionHolders returns (user, effective permission code) pairs for
	// every enabled, approved account, PREFILTERED to codes whose resource
	// segment is `resource` or `*`. The caller applies the real permission
	// grammar — see approverIDs.
	ListPermissionHolders(ctx context.Context, resource string) ([]PermissionHolder, error)
}

// PermissionHolder is one (user, effective permission code) pair.
type PermissionHolder struct {
	UserID uuid.UUID
	Code   string
}

// LocalUser is the projection needed to authenticate and mint a session.
type LocalUser struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	PasswordHash string // Argon2id PHC string; empty if no password set
	TokenVersion int
	Disabled     bool
	// ApprovalStatus gates login (migration 0031). Credentials being correct is
	// not enough — a pending or rejected account gets 403, never a session.
	ApprovalStatus string
	ApprovalNote   string // reviewer's reason; echoed back on a rejection
}

type CreateLocalUserInput struct {
	Email        string
	DisplayName  string
	PasswordHash string
}

// ── POST /auth/login ────────────────────────────────────────────────────────
//
// Verifies email + password against users.password_hash (Argon2id). On any
// credential failure it returns a generic 401 (no user enumeration) and
// increments the brute-force counter. On success it clears the counter and
// issues the session.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Remember bool   `json:"remember"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	email := normalizeEmail(body.Email)
	if email == "" || body.Password == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "email and password are required")
		return
	}

	ctx := r.Context()
	ip := clientIP(r)
	if h.loginThrottled(ctx, ip, email) {
		writeError(w, http.StatusTooManyRequests, "too_many_attempts", "too many failed attempts; try again later")
		return
	}

	user, err := h.Users.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			h.recordLoginFailure(ctx, ip, email)
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "could not process login")
		return
	}

	ok, verr := auth.VerifyPassword(body.Password, user.PasswordHash)
	if user.PasswordHash == "" || verr != nil || !ok {
		h.recordLoginFailure(ctx, ip, email)
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
		return
	}

	if user.Disabled {
		writeError(w, http.StatusForbidden, "account_disabled", "this account is disabled")
		return
	}

	// The approval gate. Checked AFTER the password so an unapproved account
	// cannot be used to probe which emails are registered — a wrong password
	// still answers the generic 401 either way.
	//
	// The failure counter is cleared first: the credentials were right, and
	// leaving the counter armed would let repeated honest attempts by someone
	// waiting on approval lock their own account out.
	if user.ApprovalStatus != "" && user.ApprovalStatus != ApprovalApproved {
		h.clearLoginFailures(ctx, ip, email)
		h.Audit.Write(ctx, audit.Event{
			Action:    audit.ActionUserPendingAttempt,
			ActorID:   &user.ID,
			IP:        ip,
			UserAgent: r.UserAgent(),
			Metadata:  map[string]any{"approval_status": user.ApprovalStatus},
		})
		code, msg := "account_pending", "this account is waiting for an administrator to approve it"
		if user.ApprovalStatus == ApprovalRejected {
			code, msg = "account_rejected", "this registration was not approved"
			if user.ApprovalNote != "" {
				msg = msg + ": " + user.ApprovalNote
			}
		}
		writeError(w, http.StatusForbidden, code, msg)
		return
	}

	h.clearLoginFailures(ctx, ip, email)
	h.completeSession(w, r, user, body.Remember)
}

// ── POST /auth/register ─────────────────────────────────────────────────────
//
// Creates a local account (Argon2id hash), seeds the default `user` role, and
// returns 201 WITHOUT a session. The account is born PENDING (migration 0031):
// the credentials work, but Login refuses them until somebody holding
// `users:approve` — superadmin out of the box — approves the registration.
//
// FIRST-RUN EXCEPTION: if this is the very first account on the install, it is
// approved on the spot and given `superadmin`. Without that a fresh deployment
// deadlocks — every registration needs an approver, and there is no approver
// until somebody registers. The window is exactly one account wide and closes
// the moment the users table is non-empty.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email       string `json:"email"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	email := normalizeEmail(body.Email)
	if !validEmail(email) {
		writeError(w, http.StatusBadRequest, "invalid_email", "a valid email is required")
		return
	}
	if len(body.Password) < minPasswordLen {
		writeError(w, http.StatusBadRequest, "weak_password", "password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not create account")
		return
	}

	ctx := r.Context()
	user, err := h.Users.CreateLocalUser(ctx, CreateLocalUserInput{
		Email:        email,
		DisplayName:  nonEmpty(strings.TrimSpace(body.DisplayName), emailLocalPart(email)),
		PasswordHash: hash,
	})
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			writeError(w, http.StatusConflict, "email_taken", "an account with this email already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal", "could not create account")
		return
	}

	// Seed the baseline role. Best-effort: a role-assign hiccup must not fail
	// the registration outright.
	roleErr := h.Users.AssignRoleByCode(ctx, user.ID, "user")

	meta := map[string]any{}
	if roleErr != nil {
		meta["role_seed_error"] = roleErr.Error()
	}

	// First-run bootstrap. `total == 1` means the row we just inserted is the
	// only one, so this is a virgin install and there is nobody to ask.
	founder := false
	if total, err := h.Users.CountUsers(ctx); err == nil && total == 1 {
		founder = true
		if err := h.Users.AssignRoleByCode(ctx, user.ID, SuperadminRole); err != nil {
			meta["superadmin_seed_error"] = err.Error()
			founder = false
		} else if err := h.Users.MarkApproved(ctx, user.ID, &user.ID); err != nil {
			meta["bootstrap_approve_error"] = err.Error()
			founder = false
		}
	}
	meta["founder"] = founder

	h.Audit.Write(ctx, audit.Event{
		Action:    audit.ActionAuthRegister,
		ActorID:   &user.ID,
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
		Metadata:  meta,
	})

	// Tell the people who can act on it. Skipped for the founder, whose account
	// approved itself and who would otherwise be notified about themselves.
	if !founder {
		h.notifyApprovers(ctx, user)
	}

	// No session is issued either way. `approval_status` tells the frontend
	// which of the two things to say: "sign in" for the founder, "wait for an
	// administrator" for everyone after them.
	status := ApprovalPending
	if founder {
		status = ApprovalApproved
	}
	server.JSON(w, http.StatusCreated, map[string]any{
		"status":          "registered",
		"email":           user.Email,
		"approval_status": status,
	})
}

// completeSession mints access + refresh tokens, sets cookies, writes the login
// audit event, and returns 200 with the access token + a small user summary.
func (h *AuthHandler) completeSession(w http.ResponseWriter, r *http.Request, user LocalUser, remember bool) {
	ctx := r.Context()
	roles, _ := h.Users.ListUserRoleCodes(ctx, user.ID)

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
	refresh, err := h.Refresh.Issue(ctx, user.ID, ip, r.UserAgent())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not issue refresh")
		return
	}

	h.setSessionCookies(w, r, access, refresh.Plaintext, remember)
	h.Audit.Write(ctx, audit.Event{
		Action:    audit.ActionAuthLogin,
		ActorID:   &user.ID,
		IP:        ip,
		UserAgent: r.UserAgent(),
	})

	server.JSON(w, http.StatusOK, map[string]any{
		"access_token": access,
		"expires_in":   int(h.AccessTTL.Seconds()),
		"token_type":   "Bearer",
		"user": map[string]any{
			"id":           user.ID,
			"email":        user.Email,
			"display_name": user.DisplayName,
			"roles":        roles,
		},
	})
}

// ── registration approval fan-out ──────────────────────────────────────────

// maxApproverFanout bounds how many people one registration can notify. A
// deployment that grants `users:approve` broadly would otherwise turn a single
// signup into an unbounded burst of enqueues inside the request. Truncation is
// logged rather than silent — a queue nobody was told about is worse than a
// noisy log.
const maxApproverFanout = 50

// notifyApprovers tells everyone who can approve a registration that one is
// waiting. Best-effort throughout, exactly like the audit log: a signup must not
// fail because a mail queue is unreachable.
func (h *AuthHandler) notifyApprovers(ctx context.Context, user LocalUser) {
	if h.Dispatch == nil || h.Users == nil {
		return
	}
	holders, err := h.Users.ListPermissionHolders(ctx, approvalResource)
	if err != nil {
		log.Error().Err(err).Msg("register: could not resolve approvers")
		return
	}

	ids := approverIDs(holders, PermApproveUser)
	if len(ids) == 0 {
		// Worth a warning, not an error: it is a real operational hole (nobody
		// can let this person in) but the registration itself is fine.
		log.Warn().Str("email", user.Email).
			Msg("register: nobody holds users:approve — the account cannot be approved by anyone")
		return
	}
	if len(ids) > maxApproverFanout {
		log.Warn().Int("holders", len(ids)).Int("notified", maxApproverFanout).
			Msg("register: approver fan-out truncated")
		ids = ids[:maxApproverFanout]
	}

	title := user.Email + " is waiting for approval"
	for _, id := range ids {
		if id == user.ID {
			continue // a self-approving account is refused anyway
		}
		intent := notifyapi.NotificationIntent{
			UserID: id,
			Type:   notifyapi.TypeRegistrationPending,
			Title:  title,
			Body:   "A new account cannot sign in until an administrator approves it.",
			Data: map[string]any{
				"user_id":      user.ID.String(),
				"email":        user.Email,
				"display_name": user.DisplayName,
				"href":         h.ApprovalQueueURL,
			},
			// in_app is the default; email is added so the alert reaches an
			// approver who is not signed in — the whole point is that somebody
			// else is blocked until they act. Muting the type still silences both.
			Channels: []string{notifyapi.ChannelInApp, notifyapi.ChannelEmail},
			// One row per (registrant, approver): an at-least-once redelivery of
			// the same signup must not stack up duplicate bells.
			DedupKey: "registration:" + user.ID.String(),
		}
		if err := h.Dispatch(ctx, intent); err != nil {
			log.Error().Err(err).Str("approver", id.String()).
				Msg("register: could not enqueue approval notification")
		}
	}
}

// approverIDs groups (user, code) pairs per user and keeps those whose set
// actually satisfies `required`.
//
// The grammar lives in rbac.Set — wildcards, scopes and the fail-closed handling
// of malformed codes all come along for free, which is why the SQL side only
// prefilters. A superadmin holds the literal `*`, so a `contains` test would
// have found nobody: exactly the person the notification is for.
func approverIDs(holders []PermissionHolder, required string) []uuid.UUID {
	byUser := map[uuid.UUID][]string{}
	order := []uuid.UUID{}
	for _, h := range holders {
		if _, seen := byUser[h.UserID]; !seen {
			order = append(order, h.UserID)
		}
		byUser[h.UserID] = append(byUser[h.UserID], h.Code)
	}

	out := make([]uuid.UUID, 0, len(order))
	for _, id := range order {
		if rbac.NewSet(byUser[id]).AllowsCode(required) {
			out = append(out, id)
		}
	}
	return out
}

// ── /auth/refresh ──────────────────────────────────────────────────────────
//
// Reads refresh cookie OR `refresh_token` body field (for non-browser clients),
// rotates it, mints a new access token. Detects reuse and burns the chain.
func (h *AuthHandler) HandleRefresh(w http.ResponseWriter, r *http.Request) {
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
		h.clearSessionCookies(w, r)
		writeError(w, http.StatusUnauthorized, "unauthorized", "refresh failed")
		return
	}

	snap, err := h.Users.GetUserAuthSnapshot(r.Context(), res.Row.UserID)
	// Approval is re-checked on every rotation, not only at login. Without it,
	// revoking an approval would leave the browser happily minting fresh access
	// tokens that then 403 on every call — a session that renews forever and can
	// do nothing. Failing the refresh instead sends SessionKeeper to /login,
	// where the user is told why.
	if err != nil || snap.Disabled ||
		(snap.ApprovalStatus != "" && snap.ApprovalStatus != ApprovalApproved) {
		h.clearSessionCookies(w, r)
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

	h.setSessionCookies(w, r, access, res.Plaintext, rememberFromRequest(r))
	h.Audit.Write(r.Context(), audit.Event{
		Action:    audit.ActionAuthRefresh,
		ActorID:   &snap.ID,
		IP:        ip,
		UserAgent: r.UserAgent(),
	})

	server.JSON(w, http.StatusOK, map[string]any{
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
	h.clearSessionCookies(w, r)
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
	h.clearSessionCookies(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// ── /auth/me ──────────────────────────────────────────────────────────────
//
// Returns identity + roles borne by the current access token. The frontend
// uses this to decide which UI affordances to render.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	id, ok := auth.FromContext(r.Context())
	if !ok || id.IsAnonymous() {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	out := map[string]any{
		"id":           id.UserID,
		"email":        id.Email,
		"display_name": id.DisplayName,
		"roles":        id.Roles,
		"permissions":  []string{},
	}
	if h.Perms != nil {
		// Best-effort, like the audit log: a cache blip should degrade the menu,
		// not fail the identity call the whole app boots from.
		if codes, err := h.Perms(r.Context(), id.UserID, id.TokenVersion); err == nil {
			out["permissions"] = codes
		}
	}
	server.JSON(w, http.StatusOK, out)
}

// ── brute-force throttle ───────────────────────────────────────────────────

func (h *AuthHandler) loginThrottled(ctx context.Context, ip net.IP, email string) bool {
	if h.Redis == nil {
		return false
	}
	for _, k := range loginFailKeys(ip, email) {
		if n, err := h.Redis.Get(ctx, k).Int(); err == nil && n >= loginMaxFailures {
			return true
		}
	}
	return false
}

func (h *AuthHandler) recordLoginFailure(ctx context.Context, ip net.IP, email string) {
	if h.Redis == nil {
		return
	}
	for _, k := range loginFailKeys(ip, email) {
		pipe := h.Redis.TxPipeline()
		pipe.Incr(ctx, k)
		pipe.Expire(ctx, k, loginFailWindow)
		_, _ = pipe.Exec(ctx)
	}
}

func (h *AuthHandler) clearLoginFailures(ctx context.Context, ip net.IP, email string) {
	if h.Redis == nil {
		return
	}
	_ = h.Redis.Del(ctx, loginFailKeys(ip, email)...).Err()
}

func loginFailKeys(ip net.IP, email string) []string {
	return []string{"login:fail:ip:" + ip.String(), "login:fail:email:" + email}
}

// ── cookies / helpers ───────────────────────────────────────────────────────

// sessionCookieName is a durable, non-secret presence marker read by the
// frontend middleware (which cannot see the tight-path refresh cookie). Its
// value encodes whether "remember me" was chosen: "p" = persistent, "s" =
// session-only. Persistence of the refresh + marker cookies tracks that choice;
// the access cookie stays short-lived and is re-minted by /auth/refresh.
const sessionCookieName = "portal_session"

// cookieDomainFor picks the Set-Cookie Domain from the host the request actually
// arrived on, so ONE deployment serves two access shapes:
//
//   - api.<domain> (behind Traefik): return <domain> so the marker cookie is shared
//     with the frontend on the sibling <domain> subdomain (else the middleware gate
//     never sees portal_session and redirect-loops).
//   - a bare IP / localhost (a phone hitting 192.168.1.53:8080 directly): return ""
//     for a host-only cookie — an IP can't carry a Domain attribute, and the frontend
//     on the same IP (different port) receives host-only cookies anyway.
//
// h.CookieDomain remains the fallback for any other host shape.
func (h *AuthHandler) cookieDomainFor(r *http.Request) string {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 {
		host = host[:i] // strip port
	}
	if host == "localhost" || net.ParseIP(host) != nil {
		return "" // host-only: IPs/localhost can't scope a Domain usefully
	}
	if parent, ok := strings.CutPrefix(host, "api."); ok {
		return parent // api.portal.localhost → portal.localhost (shared with frontend)
	}
	return h.CookieDomain
}

func (h *AuthHandler) setSessionCookies(w http.ResponseWriter, r *http.Request, access, refresh string, remember bool) {
	// remember → persistent (Max-Age = refresh TTL); otherwise a session cookie
	// (Max-Age omitted → cleared when the browser closes).
	durableMaxAge := 0
	if remember {
		durableMaxAge = int(h.RefreshTTL.Seconds())
	}
	markerValue := "s"
	if remember {
		markerValue = "p"
	}
	domain := h.cookieDomainFor(r)

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AccessCookieName,
		Value:    access,
		Path:     "/",
		Domain:   domain,
		MaxAge:   int(h.AccessTTL.Seconds()),
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "portal_refresh",
		Value:    refresh,
		Path:     "/api/v1/auth",
		Domain:   domain,
		MaxAge:   durableMaxAge,
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    markerValue,
		Path:     "/",
		Domain:   domain,
		MaxAge:   durableMaxAge,
		HttpOnly: true,
		Secure:   h.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// rememberFromRequest recovers the "remember me" choice from the marker cookie
// so a token rotation preserves the original persistence.
func rememberFromRequest(r *http.Request) bool {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		return c.Value == "p"
	}
	return false
}

// clearSessionCookies expires every session cookie. It MUST mirror the Domain
// used when setting them (h.CookieDomain) — a browser only deletes a cookie when
// the clearing Set-Cookie matches name + domain + path, so omitting Domain here
// would leave the durable portal_session marker in place and cause a redirect loop.
func (h *AuthHandler) clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	domain := h.cookieDomainFor(r)
	for _, c := range []http.Cookie{
		{Name: middleware.AccessCookieName, Path: "/", Domain: domain, MaxAge: -1, HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteStrictMode},
		{Name: "portal_refresh", Path: "/api/v1/auth", Domain: domain, MaxAge: -1, HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteStrictMode},
		{Name: sessionCookieName, Path: "/", Domain: domain, MaxAge: -1, HttpOnly: true, Secure: h.CookieSecure, SameSite: http.SameSiteStrictMode},
	} {
		c := c
		http.SetCookie(w, &c)
	}
}

func (h *AuthHandler) extractRefreshToken(r *http.Request) string {
	if c, err := r.Cookie("portal_refresh"); err == nil && c.Value != "" {
		return c.Value
	}
	if r.Method == http.MethodPost && strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			return body.RefreshToken
		}
	}
	return ""
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(v)
}

func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func validEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at >= len(s)-1 || len(s) > 320 {
		return false
	}
	return strings.IndexByte(s[at+1:], '.') >= 0
}

func emailLocalPart(s string) string {
	if i := strings.IndexByte(s, '@'); i > 0 {
		return s[:i]
	}
	return s
}
