package music

// HTTP surface for the bulk zip import (0038). Three steps, because a zip is
// too big to ride along with the JSON that describes it:
//
//	POST /tracks/imports            → a job id
//	PUT  /tracks/imports/{id}/upload → the archive, streamed
//	GET  /tracks/imports/{id}        → poll status + the per-file report
//
// Everything is owner-scoped. An import creates tracks for the caller and for
// nobody else, so `music:write:own` — the same permission that gates creating a
// single track — is the whole gate.

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server"
)

// POST /tracks/imports — register a job.
func (h *Handler) CreateImport(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	job, err := h.svc.CreateImport(r.Context(), uid)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, importJSON(job))
}

// GET /tracks/imports — the caller's recent jobs, so a reload does not lose
// sight of an import that is still running.
func (h *Handler) ListImports(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	jobs, err := h.svc.ListImports(r.Context(), uid, server.AtoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	out := make([]map[string]any, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, importJSON(j))
	}
	server.JSON(w, http.StatusOK, map[string]any{"imports": out})
}

// GET /tracks/imports/{id} — poll.
func (h *Handler) GetImport(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := h.importID(w, r)
	if !ok {
		return
	}
	job, err := h.svc.GetImport(r.Context(), id, uid)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, importJSON(job))
}

// PUT /tracks/imports/{id}/upload — the archive itself.
//
// A raw body rather than multipart: the file is the entire request, multipart
// would only add a parse step and a memory copy, and the service spools straight
// to a temp file either way.
func (h *Handler) UploadImportZip(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := h.importID(w, r)
	if !ok {
		return
	}
	job, err := h.svc.SaveImportZip(r.Context(), id, uid, r.Body)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	server.JSON(w, http.StatusAccepted, importJSON(job))
}

func (h *Handler) importID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.NotFound(w, server.ProblemType("music", "import_not_found"), "import not found")
		return uuid.Nil, false
	}
	return id, true
}

// importJSON is the wire shape. `report` is re-emitted as parsed JSON rather
// than a string so the client can render it without a second decode; an
// unreadable blob degrades to an empty list instead of failing the poll.
func importJSON(j ImportJob) map[string]any {
	var report []map[string]any
	if len(j.Report) > 0 {
		if err := json.Unmarshal(j.Report, &report); err != nil {
			report = nil
		}
	}
	if report == nil {
		report = []map[string]any{}
	}
	return map[string]any{
		"id":         j.ID,
		"status":     j.Status,
		"total":      j.Total,
		"succeeded":  j.Succeeded,
		"failed":     j.Failed,
		"report":     report,
		"error":      nilIfBlank(j.Error),
		"created_at": j.CreatedAt,
		"updated_at": j.UpdatedAt,
	}
}

func nilIfBlank(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ── enrichment ─────────────────────────────────────────────────────────────

// POST /tracks/{id}/enrich — fill in cover art and any missing tags.
//
// 202, not 200: the work happens on the worker (ffmpeg extraction plus a wait
// for the image pipeline). Poll the track; `cover_asset_id` appears when it is
// done. Deliberately a separate call from the import so a bulk import stays fast.
func (h *Handler) EnrichTrack(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.NotFound(w, server.ProblemType("music", "not_found"), "track not found")
		return
	}
	if err := h.svc.EnqueueEnrich(r.Context(), id, uid); err != nil {
		writeMusicErr(w, err)
		return
	}
	server.JSON(w, http.StatusAccepted, map[string]any{"queued": 1})
}

// POST /tracks/imports/{id}/enrich — the same, for every track a job created.
//
// One call instead of N: after importing three hundred tracks, asking the client
// to fire three hundred requests would be a worse API than the import it follows.
func (h *Handler) EnrichImport(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := h.importID(w, r)
	if !ok {
		return
	}
	queued, err := h.svc.EnqueueEnrichForImport(r.Context(), id, uid)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	server.JSON(w, http.StatusAccepted, map[string]any{"queued": queued})
}
