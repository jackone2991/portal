package ops

import (
	"encoding/json"
	"net/http"
	"time"
)

// Handler is the ops HTTP surface — the freshness sentinel (P0.5). It carries no
// currentUser: /ops/status is admin-tier (ops:read), not owner-scoped.
type Handler struct {
	svc *Service
}

// Status handles GET /ops/status →
// {last_success_at, hours_since_success, state, last_run}.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Status(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "could not read ops status")
		return
	}
	writeJSON(w, http.StatusOK, statusJSON(res))
}

// ── JSON shaping ─────────────────────────────────────────────────────

func statusJSON(res StatusResult) map[string]any {
	out := map[string]any{
		"last_success_at":     nil,
		"hours_since_success": nil,
		"state":               string(res.State),
		"last_run":            nil,
	}
	if res.LastSuccessAt != nil {
		out["last_success_at"] = res.LastSuccessAt.UTC().Format(time.RFC3339)
	}
	if res.HoursSinceSuccess != nil {
		out["hours_since_success"] = *res.HoursSinceSuccess
	}
	if res.LastRun != nil {
		out["last_run"] = runJSON(*res.LastRun)
	}
	return out
}

func runJSON(r BackupRun) map[string]any {
	m := map[string]any{
		"id":          r.ID.String(),
		"status":      r.Status,
		"started_at":  r.StartedAt.UTC().Format(time.RFC3339),
		"finished_at": nil,
		"size_bytes":  nil,
		"storage_key": nil,
		"error":       nil,
	}
	if r.FinishedAt != nil {
		m["finished_at"] = r.FinishedAt.UTC().Format(time.RFC3339)
	}
	if r.SizeBytes != nil {
		m["size_bytes"] = *r.SizeBytes
	}
	if r.StorageKey != nil {
		m["storage_key"] = *r.StorageKey
	}
	if r.Error != nil {
		m["error"] = *r.Error
	}
	return m
}

// ── HTTP helpers (match journal's shape) ─────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": typ, "title": title, "status": status, "detail": detail,
	})
}
