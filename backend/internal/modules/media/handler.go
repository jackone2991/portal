package media

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler is the media HTTP surface. currentUser reads the authenticated user
// out of the request context (populated by the account RequireAuth middleware,
// bridged in by cmd/api so media never imports account/auth).
type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

// POST /assets — create an upload session (presigned PUT).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	var body struct {
		Filename    string `json:"filename"`
		ContentType string `json:"content_type"`
		SizeBytes   int64  `json:"size_bytes"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid JSON body")
		return
	}
	if body.ContentType == "" {
		body.ContentType = "application/octet-stream"
	}

	sess, err := h.svc.CreateUploadSession(r.Context(), uid, body.Filename, body.ContentType, body.SizeBytes)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not create upload session")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"asset":  assetJSON(sess.Asset, ""),
		"upload": map[string]any{"url": sess.URL, "method": sess.Method, "headers": sess.Headers},
	})
}

// POST /assets/{id}/complete — confirm upload, enqueue transcode.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid asset id")
		return
	}
	if err := h.svc.CompleteUpload(r.Context(), uid, id); err != nil {
		writeMediaErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "processing"})
}

// PUT /assets/{id}/source — API-proxied upload of the original (dev path).
func (h *Handler) UploadSource(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid asset id")
		return
	}
	body := http.MaxBytesReader(w, r.Body, 1<<30) // 1 GiB cap
	if err := h.svc.UploadSource(r.Context(), uid, id, body, r.Header.Get("Content-Type")); err != nil {
		writeMediaErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /assets/{id} — asset metadata (+ hls_url when ready).
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "bad_request", "invalid asset id")
		return
	}
	asset, hls, err := h.svc.Get(r.Context(), uid, id)
	if err != nil {
		writeMediaErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, assetJSON(asset, hls))
}

// GET /assets — list the caller's assets.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	assets, err := h.svc.List(r.Context(), uid)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "could not list assets")
		return
	}
	out := make([]any, 0, len(assets))
	for _, a := range assets {
		out = append(out, assetJSON(a, ""))
	}
	writeJSON(w, http.StatusOK, map[string]any{"assets": out})
}

// GET /assets/{id}/hls/* — PUBLIC HLS proxy (manifest + segments).
func (h *Handler) HLS(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	sub := chi.URLParam(r, "*")
	rc, ct, err := h.svc.HLSObject(r.Context(), id, sub)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=30")
	_, _ = io.Copy(w, rc)
}

// ── helpers ─────────────────────────────────────────────────────────

func assetJSON(a Asset, hlsURL string) map[string]any {
	m := map[string]any{
		"id":          a.ID,
		"status":      a.Status,
		"kind":        a.Kind,
		"mime_type":   a.MimeType,
		"size_bytes":  a.SizeBytes,
		"duration_ms": a.DurationMs,
		"width":       a.Width,
		"height":      a.Height,
		"created_at":  a.CreatedAt.Format(time.RFC3339),
	}
	if hlsURL != "" {
		m["hls_url"] = hlsURL
	}
	if a.Status == StatusFailed && a.ErrorMessage != "" {
		m["error"] = a.ErrorMessage
	}
	return m
}

func writeMediaErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		writeErr(w, http.StatusForbidden, "forbidden", "not your asset")
	case errors.Is(err, ErrNotFound):
		writeErr(w, http.StatusNotFound, "not_found", "asset not found")
	case errors.Is(err, ErrNotReady):
		writeErr(w, http.StatusConflict, "not_ready", "asset upload is not complete")
	default:
		writeErr(w, http.StatusInternalServerError, "internal", "unexpected error")
	}
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
