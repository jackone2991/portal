package social

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server"
)

// RFC 7807 problem types for this module.
const (
	probNotFound = "social/connection-not-found"
	probExists   = "social/connection-exists"
	probSelf     = "social/cannot-connect-to-self"
)

// Handler is the social HTTP surface.
type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

// POST /connections {"user_id": "..."} — ask someone to connect.
func (h *Handler) Request(w http.ResponseWriter, r *http.Request) {
	me, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		UserID string `json:"user_id"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	target, err := uuid.Parse(body.UserID)
	if err != nil {
		server.BadRequest(w, "invalid user_id")
		return
	}
	c, err := h.svc.Request(r.Context(), me, target)
	if err != nil {
		writeSocialErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, connectionJSON(c, me))
}

// GET /connections?status=accepted|incoming|outgoing
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	me, ok := h.auth(w, r)
	if !ok {
		return
	}
	kind := Kind(r.URL.Query().Get("status"))
	switch kind {
	case KindAccepted, KindIncoming, KindOutgoing:
	case "":
		kind = KindAccepted
	default:
		server.BadRequest(w, `status must be "accepted", "incoming" or "outgoing"`)
		return
	}
	parties, err := h.svc.List(r.Context(), me, kind)
	if err != nil {
		writeSocialErr(w, err)
		return
	}
	items := make([]any, 0, len(parties))
	for _, p := range parties {
		items = append(items, map[string]any{
			"id":           p.ID,
			"user_id":      p.UserID,
			"display_name": p.DisplayName,
			"status":       p.Status,
			"outgoing":     p.Outgoing,
			"created_at":   p.CreatedAt.Format(time.RFC3339),
		})
	}
	server.JSON(w, http.StatusOK, map[string]any{"connections": items})
}

// GET /connections/summary — the header badge's count.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	me, ok := h.auth(w, r)
	if !ok {
		return
	}
	n, err := h.svc.CountIncoming(r.Context(), me)
	if err != nil {
		writeSocialErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, map[string]any{"incoming": n})
}

// POST /connections/{id}/accept
func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	me, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	c, err := h.svc.Accept(r.Context(), me, id)
	if err != nil {
		writeSocialErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, connectionJSON(c, me))
}

// DELETE /connections/{id} — withdraw, decline or disconnect.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	me, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := h.parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.Remove(r.Context(), me, id); err != nil {
		writeSocialErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

/* ── helpers ─────────────────────────────────────────────────────── */

func (h *Handler) auth(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	me, ok := h.currentUser(r.Context())
	if !ok {
		server.Unauthorized(w)
		return uuid.Nil, false
	}
	return me, true
}

func (h *Handler) parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.Problem(w, http.StatusNotFound, probNotFound, "Not found", "invalid connection id")
		return uuid.Nil, false
	}
	return id, true
}

func connectionJSON(c Connection, me uuid.UUID) map[string]any {
	m := map[string]any{
		"id":         c.ID,
		"user_id":    c.Other(me),
		"status":     c.Status,
		"outgoing":   c.RequesterID == me,
		"created_at": c.CreatedAt.Format(time.RFC3339),
	}
	if c.RespondedAt != nil {
		m["responded_at"] = c.RespondedAt.Format(time.RFC3339)
	}
	return m
}

// writeSocialErr maps domain errors to problems. A row the caller may not touch
// is reported as not-found, never forbidden: the policies hide it, so confirming
// it exists would be the only way to learn it does.
func writeSocialErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		server.Problem(w, http.StatusNotFound, probNotFound, "Not found", "no such connection")
	case errors.Is(err, ErrExists):
		server.Problem(w, http.StatusConflict, probExists, "Conflict", "you are already connected, or a request is pending")
	case errors.Is(err, ErrSelf):
		server.Problem(w, http.StatusUnprocessableEntity, probSelf, "Unprocessable Entity", "you cannot connect to yourself")
	default:
		server.Internal(w)
	}
}
