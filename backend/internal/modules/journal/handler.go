package journal

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/portal/backend/internal/platform/server"
)

// Handler is the journal HTTP surface. currentUser reads the authenticated user
// from the request context (populated by the account RequireAuth middleware,
// bridged in by cmd/api so journal never imports account/auth).
type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

// entryReq is the create/patch body. asset_ids is decoded as RawMessage only to
// detect its *presence* — any asset_ids is rejected until P1.5 (fail closed).
type entryReq struct {
	BodyMd     *string          `json:"body_md"`
	Mood       *string          `json:"mood"`
	OccurredAt *time.Time       `json:"occurred_at"`
	AssetIDs   *json.RawMessage `json:"asset_ids"`
}

// POST /journal/entries — create an entry.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return
	}
	var body entryReq
	if err := decodeJSON(r, &body); err != nil {
		server.Problem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid JSON body")
		return
	}
	if body.BodyMd == nil {
		writeJournalErr(w, ErrInvalidBody)
		return
	}
	entry, err := h.svc.Create(r.Context(), CreateParams{
		UserID:      uid,
		BodyMd:      *body.BodyMd,
		Mood:        body.Mood,
		OccurredAt:  body.OccurredAt,
		HasAssetIDs: body.AssetIDs != nil,
	})
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, entryJSON(entry))
}

// GET /journal/entries?cursor=&limit= — list the caller's entries.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return
	}
	limit := 0
	if n := r.URL.Query().Get("limit"); n != "" {
		limit = server.AtoiSafe(n)
	}
	res, err := h.svc.List(r.Context(), uid, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Items))
	for _, e := range res.Items {
		items = append(items, entryJSON(e))
	}
	out := map[string]any{"items": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	server.JSON(w, http.StatusOK, out)
}

// GET /journal/entries/{id} — fetch one entry (owner-scoped).
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJournalErr(w, ErrEntryNotFound)
		return
	}
	entry, err := h.svc.Get(r.Context(), uid, id)
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, entryJSON(entry))
}

// PATCH /journal/entries/{id} — partial update.
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJournalErr(w, ErrEntryNotFound)
		return
	}
	var body entryReq
	if err := decodeJSON(r, &body); err != nil {
		server.Problem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid JSON body")
		return
	}
	entry, err := h.svc.Patch(r.Context(), PatchParams{
		UserID:      uid,
		ID:          id,
		BodyMd:      body.BodyMd,
		Mood:        body.Mood,
		OccurredAt:  body.OccurredAt,
		HasAssetIDs: body.AssetIDs != nil,
	})
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, entryJSON(entry))
}

// DELETE /journal/entries/{id} — idempotent delete.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJournalErr(w, ErrEntryNotFound)
		return
	}
	if err := h.svc.Delete(r.Context(), uid, id); err != nil {
		writeJournalErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /stream?cursor=&limit= — the merged life-stream timeline (SPEC-06 P0.2).
func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return
	}
	res, err := h.svc.Stream(r.Context(), uid, r.URL.Query().Get("cursor"), server.AtoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Items))
	for _, c := range res.Items {
		m := map[string]any{
			"id":            c.ID,
			"source_module": c.SourceModule,
			"event_type":    c.EventType,
			"occurred_at":   c.OccurredAt.Format(time.RFC3339),
		}
		if c.SourceModule == "journal" {
			m["body_md"] = c.BodyMd
			m["mood"] = c.Mood
		} else {
			m["title"] = c.Title
			if c.Href != "" {
				m["href"] = c.Href
			}
		}
		items = append(items, m)
	}
	out := map[string]any{"items": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	server.JSON(w, http.StatusOK, out)
}

// ── helpers ─────────────────────────────────────────────────────────

func entryJSON(e Entry) map[string]any {
	ids := make([]string, 0, len(e.AssetIDs))
	for _, a := range e.AssetIDs {
		ids = append(ids, a.String())
	}
	return map[string]any{
		"id":          e.ID,
		"body_md":     e.BodyMd,
		"mood":        e.Mood,
		"asset_ids":   ids,
		"occurred_at": e.OccurredAt.Format(time.RFC3339),
		"created_at":  e.CreatedAt.Format(time.RFC3339),
		"updated_at":  e.UpdatedAt.Format(time.RFC3339),
	}
}

// writeJournalErr maps a service error to its RFC 7807 Problem (§7 type URIs).
func writeJournalErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrEntryNotFound):
		server.Problem(w, http.StatusNotFound, "journal/entry-not-found", "Not Found", "journal entry not found")
	case errors.Is(err, ErrInvalidBody):
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-body", "Invalid body", "body_md must be 1–20000 characters")
	case errors.Is(err, ErrInvalidMood):
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-mood", "Invalid mood", "mood must be 1–80 non-blank characters")
	case errors.Is(err, ErrInvalidAsset):
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-asset", "Invalid asset", "asset_ids are not accepted until photo attachments ship (P1.5)")
	case errors.Is(err, ErrBadCursor):
		server.Problem(w, http.StatusBadRequest, "journal/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
	default:
		server.Problem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
	}
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
