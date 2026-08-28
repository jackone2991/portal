package music

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server"
)

// HTTP for playlists and for the library's bulk actions (0041).
//
// Both live here because they are the same idea from the user's side: the music
// library lists hundreds of imported tracks, and everything you want to do with
// them you want to do to a selection, not to one row at a time.

const (
	probPlaylistNotFound = "music/playlist-not-found"
	probPlaylistExists   = "music/playlist-exists"
	probPlaylistInvalid  = "music/invalid-playlist"
)

// GET /playlists
func (h *Handler) ListPlaylists(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListPlaylists(r.Context(), uid)
	if err != nil {
		writePlaylistErr(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for _, p := range items {
		out = append(out, playlistJSON(p))
	}
	server.JSON(w, http.StatusOK, map[string]any{"playlists": out})
}

// POST /playlists {"name": "...", "description": "..."}
func (h *Handler) CreatePlaylist(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Name        string  `json:"name"`
		Description *string `json:"description"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	p, err := h.svc.CreatePlaylist(r.Context(), uid, body.Name, body.Description)
	if err != nil {
		writePlaylistErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, playlistJSON(p))
}

// GET /playlists/{id} — the playlist and its tracks, in playlist order.
func (h *Handler) GetPlaylist(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parsePlaylistID(w, r)
	if !ok {
		return
	}
	p, tracks, err := h.svc.GetPlaylist(r.Context(), uid, id)
	if err != nil {
		writePlaylistErr(w, err)
		return
	}
	items := make([]any, 0, len(tracks))
	for _, t := range tracks {
		items = append(items, trackJSON(t))
	}
	body := playlistJSON(p)
	body["track_count"] = len(tracks)
	body["tracks"] = items
	server.JSON(w, http.StatusOK, body)
}

// PATCH /playlists/{id}
func (h *Handler) UpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parsePlaylistID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	// A description key present in the body means "set it" — including to null,
	// which clears it. Absent means leave it alone.
	p, err := h.svc.RenamePlaylist(r.Context(), uid, id, body.Name, body.Description != nil, body.Description)
	if err != nil {
		writePlaylistErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, playlistJSON(p))
}

// DELETE /playlists/{id} — the playlist only; its tracks are untouched.
func (h *Handler) DeletePlaylist(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parsePlaylistID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeletePlaylist(r.Context(), uid, id); err != nil {
		writePlaylistErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /playlists/{id}/tracks {"track_ids": [...]} — add a whole selection.
//
// Reports how many actually landed. Selecting thirty and adding twenty-eight
// because two were already in the playlist is worth saying out loud, rather
// than reporting success and leaving the user to count.
func (h *Handler) AddPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parsePlaylistID(w, r)
	if !ok {
		return
	}
	var body struct {
		TrackIDs []string `json:"track_ids"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	ids, ok := parseIDList(w, body.TrackIDs)
	if !ok {
		return
	}
	added, err := h.svc.AddToPlaylist(r.Context(), uid, id, ids)
	if err != nil {
		writePlaylistErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, map[string]any{"added": added, "requested": len(ids)})
}

// DELETE /playlists/{id}/tracks/{trackId}
func (h *Handler) RemovePlaylistTrack(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parsePlaylistID(w, r)
	if !ok {
		return
	}
	trackID, err := uuid.Parse(chi.URLParam(r, "trackId"))
	if err != nil {
		server.Problem(w, http.StatusNotFound, probPlaylistNotFound, "Not found", "invalid track id")
		return
	}
	if err := h.svc.RemoveFromPlaylist(r.Context(), uid, id, trackID); err != nil {
		writePlaylistErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /tracks/bulk-status {"track_ids": [...], "status": "published"|"draft"}
//
// The per-track publish route has ownership middleware in front of it; this one
// cannot, because the ids arrive in the body. The service filters them through
// the caller's own tracks before touching anything — see BulkSetStatus.
func (h *Handler) BulkStatus(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		TrackIDs []string `json:"track_ids"`
		Status   string   `json:"status"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	if body.Status != StatusPublished && body.Status != StatusDraft {
		server.Problem(w, http.StatusUnprocessableEntity, probPlaylistInvalid,
			"Unprocessable Entity", `status must be "published" or "draft"`)
		return
	}
	ids, ok := parseIDList(w, body.TrackIDs)
	if !ok {
		return
	}
	changed, err := h.svc.BulkSetStatus(r.Context(), uid, ids, body.Status)
	if err != nil {
		writePlaylistErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, map[string]any{
		"changed":   len(changed),
		"requested": len(ids),
		"status":    body.Status,
	})
}

/* ── helpers ─────────────────────────────────────────────────────── */

func parsePlaylistID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.Problem(w, http.StatusNotFound, probPlaylistNotFound, "Not found", "invalid playlist id")
		return uuid.Nil, false
	}
	return id, true
}

// parseIDList rejects the whole request on one malformed id rather than
// silently dropping it: a selection that half-applies is worse than one that
// does not apply at all, because nobody can tell which half.
func parseIDList(w http.ResponseWriter, raw []string) ([]uuid.UUID, bool) {
	if len(raw) == 0 {
		server.BadRequest(w, "track_ids is empty")
		return nil, false
	}
	if len(raw) > 500 {
		server.BadRequest(w, "track_ids is limited to 500 per request")
		return nil, false
	}
	out := make([]uuid.UUID, 0, len(raw))
	for _, s := range raw {
		id, err := uuid.Parse(s)
		if err != nil {
			server.BadRequest(w, "track_ids contains an invalid id")
			return nil, false
		}
		out = append(out, id)
	}
	return out, true
}

func playlistJSON(p Playlist) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"track_count": p.TrackCount,
		"created_at":  p.CreatedAt.Format(time.RFC3339),
		"updated_at":  p.UpdatedAt.Format(time.RFC3339),
	}
}

func writePlaylistErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrPlaylistNotFound):
		server.Problem(w, http.StatusNotFound, probPlaylistNotFound, "Not found", "no such playlist")
	case errors.Is(err, ErrPlaylistExists):
		server.Problem(w, http.StatusConflict, probPlaylistExists, "Conflict", "you already have a playlist with that name")
	case errors.Is(err, ErrPlaylistInvalid):
		server.Problem(w, http.StatusUnprocessableEntity, probPlaylistInvalid, "Unprocessable Entity", "the playlist name must be 1–120 characters")
	default:
		server.Internal(w)
	}
}
