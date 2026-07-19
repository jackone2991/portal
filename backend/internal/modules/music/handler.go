package music

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

type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

func (h *Handler) ListTracks(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth(w, r); !ok {
		return
	}
	res, err := h.svc.ListPublished(r.Context(), r.URL.Query().Get("cursor"), atoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeTrackList(w, res)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ListOwn(r.Context(), uid, r.URL.Query().Get("cursor"), atoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeTrackList(w, res)
}

func (h *Handler) CreateTrack(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Title        string  `json:"title"`
		Artist       *string `json:"artist"`
		Album        *string `json:"album"`
		Description  *string `json:"description"`
		AudioAssetID *string `json:"audio_asset_id"`
		CoverAssetID *string `json:"cover_asset_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	audio, perr := parseOptID(body.AudioAssetID)
	if perr {
		badReq(w, "invalid audio_asset_id")
		return
	}
	cover, perr := parseOptID(body.CoverAssetID)
	if perr {
		badReq(w, "invalid cover_asset_id")
		return
	}
	t, err := h.svc.CreateTrack(r.Context(), CreateTrackInput{
		OwnerID: uid, Title: body.Title, Artist: body.Artist, Album: body.Album, Description: body.Description,
		AudioAssetID: audio, CoverAssetID: cover,
	})
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, trackJSON(t))
}

func (h *Handler) GetTrack(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	t, err := h.svc.GetTrack(r.Context(), uid, id)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trackJSON(t))
}

func (h *Handler) UpdateTrack(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title        *string         `json:"title"`
		Artist       json.RawMessage `json:"artist"`
		Album        json.RawMessage `json:"album"`
		Description  *string         `json:"description"`
		AudioAssetID json.RawMessage `json:"audio_asset_id"`
		CoverAssetID json.RawMessage `json:"cover_asset_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	artist, setArtist := parseRawOptStr(body.Artist)
	album, setAlbum := parseRawOptStr(body.Album)
	audio, setAudio, perr := parseRawOptID(body.AudioAssetID)
	if perr {
		badReq(w, "invalid audio_asset_id")
		return
	}
	cover, setCover, perr := parseRawOptID(body.CoverAssetID)
	if perr {
		badReq(w, "invalid cover_asset_id")
		return
	}
	t, err := h.svc.UpdateTrack(r.Context(), UpdateTrackInput{
		ID: id, Title: body.Title, Description: body.Description,
		SetArtist: setArtist, Artist: artist, SetAlbum: setAlbum, Album: album,
		SetAudio: setAudio, AudioAssetID: audio, SetCover: setCover, CoverAssetID: cover,
	})
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trackJSON(t))
}

func (h *Handler) DeleteTrack(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteTrack(r.Context(), id); err != nil {
		writeMusicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	t, err := h.svc.Publish(r.Context(), id)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trackJSON(t))
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	t, err := h.svc.Unpublish(r.Context(), id)
	if err != nil {
		writeMusicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, trackJSON(t))
}

// ── JSON ──────────────────────────────────────────────────────────────

func trackJSON(t Track) map[string]any {
	return map[string]any{
		"id": t.ID, "owner_id": t.OwnerID, "title": t.Title, "artist": t.Artist, "album": t.Album,
		"description": t.Description, "audio_asset_id": uuidPtrJSON(t.AudioAssetID), "cover_asset_id": uuidPtrJSON(t.CoverAssetID),
		"status": t.Status, "created_at": t.CreatedAt.Format(time.RFC3339), "updated_at": t.UpdatedAt.Format(time.RFC3339),
	}
}

func writeTrackList(w http.ResponseWriter, res ListResult) {
	items := make([]any, 0, len(res.Items))
	for _, t := range res.Items {
		items = append(items, trackJSON(t))
	}
	out := map[string]any{"tracks": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	writeJSON(w, http.StatusOK, out)
}

// ── helpers ───────────────────────────────────────────────────────────

func (h *Handler) auth(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return uuid.Nil, false
	}
	return uid, true
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "music/not-found", "Not Found", "not found")
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		badReq(w, "invalid JSON body")
		return false
	}
	return true
}

func parseOptID(s *string) (*uuid.UUID, bool) {
	if s == nil || *s == "" {
		return nil, false
	}
	id, err := uuid.Parse(*s)
	if err != nil {
		return nil, true
	}
	return &id, false
}

func parseRawOptID(raw json.RawMessage) (id *uuid.UUID, present bool, parseErr bool) {
	if len(raw) == 0 {
		return nil, false, false
	}
	if string(raw) == "null" {
		return nil, true, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, true, true
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil, true, true
	}
	return &u, true, false
}

// parseRawOptStr: absent field → (nil, false); null or a string → (val, true).
func parseRawOptStr(raw json.RawMessage) (*string, bool) {
	if len(raw) == 0 {
		return nil, false
	}
	if string(raw) == "null" {
		return nil, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, true
	}
	return &s, true
}

func uuidPtrJSON(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}

func badReq(w http.ResponseWriter, detail string) {
	writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", detail)
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 1_000_000 {
			return 1_000_000
		}
	}
	return n
}

func writeMusicErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "music/not-found", "Not Found", "not found")
	case errors.Is(err, ErrNotPublishable):
		writeProblem(w, http.StatusUnprocessableEntity, "music/not-publishable", "Not publishable", "a track needs a ready audio asset to publish")
	case errors.Is(err, ErrInvalidAudioAsset):
		writeProblem(w, http.StatusUnprocessableEntity, "music/invalid-audio-asset", "Invalid audio asset", "the audio must be a ready audio asset you own")
	case errors.Is(err, ErrInvalidCoverAsset):
		writeProblem(w, http.StatusUnprocessableEntity, "music/invalid-cover-asset", "Invalid cover asset", "the cover must be a ready image asset you own")
	case errors.Is(err, ErrValidation):
		writeProblem(w, http.StatusUnprocessableEntity, "music/validation", "Validation error", "the request is invalid")
	case errors.Is(err, ErrBadCursor):
		writeProblem(w, http.StatusBadRequest, "music/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
	default:
		writeProblem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
	}
}

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
	_ = json.NewEncoder(w).Encode(map[string]any{"type": typ, "title": title, "status": status, "detail": detail})
}
