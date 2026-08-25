package movie

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/portal/backend/internal/platform/server"
)

type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

func (h *Handler) ListMovies(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth(w, r); !ok {
		return
	}
	res, err := h.svc.ListPublished(r.Context(), r.URL.Query().Get("cursor"), server.AtoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	writeMovieList(w, res)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ListOwn(r.Context(), uid, r.URL.Query().Get("cursor"), server.AtoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	writeMovieList(w, res)
}

func (h *Handler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Title         string  `json:"title"`
		Description   *string `json:"description"`
		VideoAssetID  *string `json:"video_asset_id"`
		PosterAssetID *string `json:"poster_asset_id"`
		ReleaseYear   *int    `json:"release_year"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	video, perr := parseOptID(body.VideoAssetID)
	if perr {
		server.BadRequest(w, "invalid video_asset_id")
		return
	}
	poster, perr := parseOptID(body.PosterAssetID)
	if perr {
		server.BadRequest(w, "invalid poster_asset_id")
		return
	}
	m, err := h.svc.CreateMovie(r.Context(), CreateMovieInput{
		OwnerID: uid, Title: body.Title, Description: body.Description,
		VideoAssetID: video, PosterAssetID: poster, ReleaseYear: body.ReleaseYear,
	})
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, movieJSON(m))
}

func (h *Handler) GetMovie(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	m, err := h.svc.GetMovie(r.Context(), uid, id)
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, movieJSON(m))
}

func (h *Handler) UpdateMovie(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title         *string         `json:"title"`
		Description   *string         `json:"description"`
		VideoAssetID  json.RawMessage `json:"video_asset_id"`
		PosterAssetID json.RawMessage `json:"poster_asset_id"`
		ReleaseYear   *int            `json:"release_year"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	video, setVideo, perr := parseRawOptID(body.VideoAssetID)
	if perr {
		server.BadRequest(w, "invalid video_asset_id")
		return
	}
	poster, setPoster, perr := parseRawOptID(body.PosterAssetID)
	if perr {
		server.BadRequest(w, "invalid poster_asset_id")
		return
	}
	m, err := h.svc.UpdateMovie(r.Context(), UpdateMovieInput{
		ID: id, Title: body.Title, Description: body.Description,
		SetVideo: setVideo, VideoAssetID: video, SetPoster: setPoster, PosterAssetID: poster, ReleaseYear: body.ReleaseYear,
	})
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, movieJSON(m))
}

func (h *Handler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteMovie(r.Context(), id); err != nil {
		writeMovieErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	m, err := h.svc.Publish(r.Context(), id)
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, movieJSON(m))
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	m, err := h.svc.Unpublish(r.Context(), id)
	if err != nil {
		writeMovieErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, movieJSON(m))
}

// ── JSON shapes ───────────────────────────────────────────────────────

func movieJSON(m Movie) map[string]any {
	return map[string]any{
		"id": m.ID, "owner_id": m.OwnerID, "title": m.Title, "description": m.Description,
		"video_asset_id": uuidPtrJSON(m.VideoAssetID), "poster_asset_id": uuidPtrJSON(m.PosterAssetID),
		"release_year": m.ReleaseYear, "status": m.Status,
		"created_at": m.CreatedAt.Format(time.RFC3339), "updated_at": m.UpdatedAt.Format(time.RFC3339),
	}
}

func writeMovieList(w http.ResponseWriter, res ListResult) {
	items := make([]any, 0, len(res.Items))
	for _, m := range res.Items {
		items = append(items, movieJSON(m))
	}
	out := map[string]any{"movies": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	server.JSON(w, http.StatusOK, out)
}

// ── request/response helpers ──────────────────────────────────────────

func (h *Handler) auth(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return uuid.Nil, false
	}
	return uid, true
}

func parseID(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		server.Problem(w, http.StatusNotFound, "movie/not-found", "Not Found", "not found")
		return uuid.Nil, false
	}
	return id, true
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

func uuidPtrJSON(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}

func writeMovieErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		server.Problem(w, http.StatusNotFound, "movie/not-found", "Not Found", "not found")
	case errors.Is(err, ErrNotPublishable):
		server.Problem(w, http.StatusUnprocessableEntity, "movie/not-publishable", "Not publishable", "a movie needs a ready video asset to publish")
	case errors.Is(err, ErrInvalidVideoAsset):
		server.Problem(w, http.StatusUnprocessableEntity, "movie/invalid-video-asset", "Invalid video asset", "the video must be a ready video asset you own")
	case errors.Is(err, ErrInvalidPosterAsset):
		server.Problem(w, http.StatusUnprocessableEntity, "movie/invalid-poster-asset", "Invalid poster asset", "the poster must be a ready image asset you own")
	case errors.Is(err, ErrValidation):
		server.Problem(w, http.StatusUnprocessableEntity, "movie/validation", "Validation error", "the request is invalid")
	case errors.Is(err, ErrBadCursor):
		server.Problem(w, http.StatusBadRequest, "movie/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
	default:
		server.Problem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
	}
}
