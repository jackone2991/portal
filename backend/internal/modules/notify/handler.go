package notify

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RFC 7807 Problem type URIs (SPEC-04 §7).
const probNotificationNotFound = "notify/notification-not-found"

// Handler is the notify HTTP surface (/me/notifications). currentUser reads the
// authenticated user out of the request context (populated by account's
// RequireAuth, bridged in by cmd/api so notify never imports account/auth).
type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

// GET /me/notifications?status=&cursor=&limit= — the caller's notifications
// (default status=all), newest first, plus the unread badge.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	q := r.URL.Query()
	unreadOnly := q.Get("status") == "unread"
	limit := 0
	if n := q.Get("limit"); n != "" {
		if v, err := parsePositiveInt(n); err == nil {
			limit = v
		}
	}

	res, err := h.svc.List(r.Context(), uid, unreadOnly, q.Get("cursor"), limit)
	if err != nil {
		if errors.Is(err, ErrBadCursor) {
			writeErr(w, http.StatusBadRequest, "bad_request", "invalid cursor")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "could not list notifications")
		return
	}

	items := make([]any, 0, len(res.Items))
	for _, n := range res.Items {
		items = append(items, notificationJSON(n))
	}
	body := map[string]any{"items": items, "unread_count": res.UnreadCount}
	if res.NextCursor != "" {
		body["next_cursor"] = res.NextCursor
	}
	writeJSON(w, http.StatusOK, body)
}

// POST /me/notifications/{id}/read — idempotent mark-read → 200 {unread_count}.
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, probNotificationNotFound, "Notification not found", "invalid notification id")
		return
	}
	unread, err := h.svc.MarkRead(r.Context(), uid, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			writeProblem(w, http.StatusNotFound, probNotificationNotFound, "Notification not found", "notification not found")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "could not mark read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread_count": unread})
}

// POST /me/notifications/read-all?before= — mark all (≤ watermark) read →
// 200 {unread_count}.
func (h *Handler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	unread, err := h.svc.MarkAllRead(r.Context(), uid, r.URL.Query().Get("before"))
	if err != nil {
		if errors.Is(err, ErrBadCursor) {
			writeErr(w, http.StatusBadRequest, "bad_request", "invalid before cursor")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "could not mark all read")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unread_count": unread})
}

// ── helpers ─────────────────────────────────────────────────────────

func notificationJSON(n Notification) map[string]any {
	m := map[string]any{
		"id":         n.ID,
		"type":       n.Type,
		"title":      n.Title,
		"data":       n.Data,
		"created_at": n.CreatedAt.Format(time.RFC3339),
	}
	if n.Body != "" {
		m["body"] = n.Body
	}
	if n.ReadAt != nil {
		m["read_at"] = n.ReadAt.Format(time.RFC3339)
	} else {
		m["read_at"] = nil
	}
	return m
}

func parsePositiveInt(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(r-'0')
		if n > 1_000_000 {
			break
		}
	}
	return n, nil
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"code": code, "message": msg})
}
