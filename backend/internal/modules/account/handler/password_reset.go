package handler

// Password reset (SPEC-04 P0.3). Two unauthenticated endpoints that mint DB rows
// / send email, so they extend ADR-06's brute-force-defence responsibility:
//
//	POST /auth/forgot-password  — enumeration-safe 202 always; per-email throttle
//	                              (≥60s gap, ≤3/h) + per-IP throttle; disabled
//	                              accounts are a silent no-op.
//	POST /auth/reset-password   — consumes a hashed reset token, sets the new
//	                              Argon2id password, bumps token_version + revokes
//	                              refresh tokens (all-sessions revocation).

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/modules/account/auth"
	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	"github.com/portal/backend/internal/platform/audit"
)

// Abuse controls for the public reset endpoints.
const (
	resetEmailCooldown   = 60 * time.Second // ≥60s between sends for one email
	resetEmailMaxPerHour = 3                // ≤3 sends per email per hour
	resetIPWindow        = time.Minute      // per-IP window
	resetIPMax           = 10               // per-IP requests per window → 429
)

// RFC 7807 Problem type URIs (SPEC-04 §7).
const (
	probInvalidResetToken = "account/invalid-reset-token"
	probPasswordPolicy    = "account/password-policy"
	probRateLimited       = "account/rate-limited"
)

// ── POST /auth/forgot-password ──────────────────────────────────────
//
// Always answers 202 (enumeration-safe), and answers BEFORE the lookup/mint/
// enqueue so response time never depends on whether the email is registered
// (uniform timing). The actual work runs out of band.
func (h *AuthHandler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	ua := r.UserAgent()
	// Per-IP throttle (IP-keyed leaks nothing about any email).
	if h.resetIPThrottled(r.Context(), ip) {
		writeProblem(w, http.StatusTooManyRequests, probRateLimited, "Too many requests", "please slow down and try again shortly")
		return
	}

	var body struct {
		Email string `json:"email"`
	}
	_ = decodeJSON(r, &body) // a malformed body is still an enumeration-safe 202
	email := normalizeEmail(body.Email)

	writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})

	// Out-of-band: the request context is cancelled once we return, so use a
	// fresh background context for the mint/dispatch work.
	go h.processForgotPassword(context.Background(), email, ip, ua)
}

func (h *AuthHandler) processForgotPassword(ctx context.Context, email string, ip net.IP, ua string) {
	if email == "" || !validEmail(email) {
		return
	}
	// Per-email throttle (silent — a 429 keyed on the email would leak existence).
	if !h.resetEmailAllowed(ctx, email) {
		return
	}
	user, err := h.Users.GetUserByEmail(ctx, email)
	if err != nil {
		return // unknown email → silent no-op (already answered 202)
	}
	if user.Disabled {
		return // disabled accounts: silent no-op
	}

	plaintext, err := h.ResetTokens.Issue(ctx, user.ID)
	if err != nil {
		log.Error().Err(err).Msg("forgot-password: mint reset token failed")
		return
	}

	sep := "?"
	if strings.Contains(h.PasswordResetURL, "?") {
		sep = "&"
	}
	resetURL := h.PasswordResetURL + sep + "token=" + url.QueryEscape(plaintext)

	intent := notifyapi.NotificationIntent{
		UserID:   user.ID,
		Type:     notifyapi.TypePasswordReset,
		Title:    "Reset your password",
		Channels: []string{notifyapi.ChannelEmail}, // override the email-off default
		Data:     map[string]any{"reset_url": resetURL},
	}
	if h.Dispatch != nil {
		if err := h.Dispatch(ctx, intent); err != nil {
			log.Error().Err(err).Msg("forgot-password: enqueue dispatch failed")
		}
	}
	h.Audit.Write(ctx, audit.Event{
		Action:    audit.ActionPasswordResetRequested,
		ActorID:   &user.ID,
		IP:        ip,
		UserAgent: ua,
	})
}

// ── POST /auth/reset-password ───────────────────────────────────────
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ip := clientIP(r)
	if h.resetIPThrottled(ctx, ip) {
		writeProblem(w, http.StatusTooManyRequests, probRateLimited, "Too many requests", "please slow down and try again shortly")
		return
	}

	var body struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeProblem(w, http.StatusBadRequest, probInvalidResetToken, "Invalid reset token", "the reset token is invalid or has expired")
		return
	}
	if body.Token == "" {
		writeProblem(w, http.StatusBadRequest, probInvalidResetToken, "Invalid reset token", "the reset token is invalid or has expired")
		return
	}
	if len(body.NewPassword) < minPasswordLen {
		writeProblem(w, http.StatusBadRequest, probPasswordPolicy, "Weak password", "password must be at least 8 characters")
		return
	}

	row, err := h.ResetTokens.Verify(ctx, body.Token)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, probInvalidResetToken, "Invalid reset token", "the reset token is invalid or has expired")
		return
	}
	// A token belonging to a since-disabled user is rejected too.
	snap, err := h.Users.GetUserAuthSnapshot(ctx, row.UserID)
	if err != nil || snap.Disabled {
		writeProblem(w, http.StatusBadRequest, probInvalidResetToken, "Invalid reset token", "the reset token is invalid or has expired")
		return
	}

	hash, err := auth.HashPassword(body.NewPassword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not reset password")
		return
	}
	if err := h.Users.SetPassword(ctx, row.UserID, hash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", "could not reset password")
		return
	}
	// Consume the token, then revoke every session: bump token_version (kills
	// live access tokens) + revoke all refresh tokens (ADR-06).
	_ = h.ResetTokens.MarkUsed(ctx, row.ID)
	if _, err := h.Users.BumpUserTokenVersion(ctx, row.UserID); err != nil {
		log.Error().Err(err).Msg("reset-password: bump token_version failed")
	}
	_ = h.Refresh.RevokeAllForUser(ctx, row.UserID, "password_reset")

	h.Audit.Write(ctx, audit.Event{
		Action:    audit.ActionPasswordResetCompleted,
		ActorID:   &row.UserID,
		IP:        ip,
		UserAgent: r.UserAgent(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "password_updated"})
}

// ── throttles ───────────────────────────────────────────────────────

// resetEmailAllowed enforces the per-email quota (≥60s gap + ≤3/h) on Dragonfly,
// the same live pattern as the login throttle. Fail-open on a Redis outage.
func (h *AuthHandler) resetEmailAllowed(ctx context.Context, email string) bool {
	if h.Redis == nil {
		return true
	}
	cooldownKey := "pwreset:cooldown:" + email
	countKey := "pwreset:count:" + email

	// SET NX marks the 60s cooldown; a pre-existing key means we sent recently.
	set, err := h.Redis.SetNX(ctx, cooldownKey, "1", resetEmailCooldown).Result()
	if err != nil {
		return true // fail-open
	}
	n, err := h.Redis.Get(ctx, countKey).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return true // fail-open
	}
	if !resetSendAllowed(!set, n, resetEmailMaxPerHour) {
		return false
	}
	if newN, err := h.Redis.Incr(ctx, countKey).Result(); err == nil && newN == 1 {
		_ = h.Redis.Expire(ctx, countKey, time.Hour).Err()
	}
	return true
}

// resetSendAllowed is the pure per-email decision: deny within the cooldown gap,
// deny past the hourly cap.
func resetSendAllowed(recentlySent bool, sentThisHour, maxPerHour int) bool {
	if recentlySent {
		return false
	}
	if sentThisHour >= maxPerHour {
		return false
	}
	return true
}

// resetIPThrottled enforces a coarse per-IP request cap (429). Fail-open.
func (h *AuthHandler) resetIPThrottled(ctx context.Context, ip net.IP) bool {
	if h.Redis == nil || ip == nil {
		return false
	}
	key := "pwreset:ip:" + ip.String()
	n, err := h.Redis.Incr(ctx, key).Result()
	if err != nil {
		return false // fail-open
	}
	if n == 1 {
		_ = h.Redis.Expire(ctx, key, resetIPWindow).Err()
	}
	return n > int64(resetIPMax)
}

func writeProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   typ,
		"title":  title,
		"status": status,
		"detail": detail,
	})
}
