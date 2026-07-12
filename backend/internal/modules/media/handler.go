package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RFC 7807 Problem type URIs (SPEC-01 §7).
const (
	probUnsupportedFormat = "media/unsupported-format"
	probFileTooLarge      = "media/file-too-large"
	probAssetNotFound     = "media/asset-not-found"
	probAssetNotReady     = "media/asset-not-ready"
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
		"asset":  h.assetJSON(sess.Asset, ""),
		"upload": map[string]any{"url": sess.URL, "method": sess.Method, "headers": sess.Headers},
	})
}

// POST /assets/{id}/complete — sniff + confirm upload, enqueue processing.
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "invalid asset id")
		return
	}
	if err := h.svc.CompleteUpload(r.Context(), uid, id); err != nil {
		writeMediaProblem(w, err)
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
	writeJSON(w, http.StatusOK, h.assetJSON(asset, hls))
}

// GET /assets — list the caller's assets (?kind=&status=&cursor=&limit=).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	q := r.URL.Query()
	opts := ListOptions{
		Kind:   q.Get("kind"),
		Status: q.Get("status"),
		Cursor: q.Get("cursor"),
	}
	if n := q.Get("limit"); n != "" {
		if v, err := parsePositiveInt(n); err == nil {
			opts.Limit = v
		}
	}

	res, err := h.svc.List(r.Context(), uid, opts)
	if err != nil {
		if errors.Is(err, ErrBadCursor) {
			writeErr(w, http.StatusBadRequest, "bad_request", "invalid cursor")
			return
		}
		writeErr(w, http.StatusInternalServerError, "internal", "could not list assets")
		return
	}

	out := make([]any, 0, len(res.Assets))
	for _, a := range res.Assets {
		out = append(out, h.assetJSON(a, h.svc.hlsURL(a)))
	}
	body := map[string]any{"assets": out}
	if res.NextCursor != "" {
		body["next_cursor"] = res.NextCursor
	}
	writeJSON(w, http.StatusOK, body)
}

// DELETE /assets/{id} — delete an asset + all its storage objects (P0.3).
// Ownership is enforced by RequireOwnerOrPermission at the route.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "invalid asset id")
		return
	}
	if err := h.svc.DeleteAsset(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "asset not found")
			return
		}
		writeProblem(w, http.StatusInternalServerError, "about:blank", "Internal error", "could not delete asset")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /assets/{id}/original — owner-authenticated download of the original (P0.5).
func (h *Handler) DownloadOriginal(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "invalid asset id")
		return
	}
	rc, ct, filename, err := h.svc.DownloadOriginal(r.Context(), uid, id)
	if err != nil {
		writeMediaProblem(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = io.Copy(w, rc)
}

// GET /assets/{id}/variants/{variant} — PUBLIC variant proxy (WebP) (P0.1).
func (h *Handler) ServeVariant(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "invalid asset id")
		return
	}
	variant := chi.URLParam(r, "variant")
	rc, ct, err := h.svc.ServeVariant(r.Context(), id, variant)
	if err != nil {
		writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "variant not found")
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = io.Copy(w, rc)
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

func (h *Handler) assetJSON(a Asset, hlsURL string) map[string]any {
	m := map[string]any{
		"id":                a.ID,
		"status":            a.Status,
		"kind":              a.Kind,
		"mime_type":         a.MimeType,
		"size_bytes":        a.SizeBytes,
		"duration_ms":       a.DurationMs,
		"width":             a.Width,
		"height":            a.Height,
		"title":             a.Title,
		"original_filename": a.OriginalFilename,
		"origin":            a.Origin,
		"created_at":        a.CreatedAt.Format(time.RFC3339),
	}
	if hlsURL != "" {
		m["hls_url"] = hlsURL
	}
	if a.Status == StatusFailed && a.ErrorMessage != "" {
		m["error"] = a.ErrorMessage
	}
	return m
}

func parsePositiveInt(s string) (int, error) {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errors.New("negative")
	}
	return n, nil
}

// writeMediaProblem maps a service error to an RFC 7807 problem+json response
// with the SPEC-01 §7 Problem type URIs.
func writeMediaProblem(w http.ResponseWriter, err error) {
	var ufe *UnsupportedFormatError
	switch {
	case errors.As(err, &ufe):
		writeProblem(w, http.StatusUnprocessableEntity, probUnsupportedFormat, "Unsupported media format", ufe.Detail)
	case errors.Is(err, ErrFileTooLarge):
		writeProblem(w, http.StatusRequestEntityTooLarge, probFileTooLarge, "File too large", "the uploaded file exceeds the 50 MB limit")
	case errors.Is(err, ErrNotReady):
		writeProblem(w, http.StatusConflict, probAssetNotReady, "Asset not ready", "the asset upload is not complete")
	case errors.Is(err, ErrForbidden):
		writeProblem(w, http.StatusForbidden, "about:blank", "Forbidden", "not your asset")
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, probAssetNotFound, "Asset not found", "asset not found")
	default:
		writeProblem(w, http.StatusInternalServerError, "about:blank", "Internal error", "unexpected error")
	}
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

// writeMediaErr is the legacy JSON error shape kept for the untouched handlers
// (ADR-01: retrofit to problem+json as touched).
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
