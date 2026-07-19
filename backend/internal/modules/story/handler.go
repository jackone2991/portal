package story

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

// ══ Stories ══════════════════════════════════════════════════════════════

func (h *Handler) ListStories(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth(w, r); !ok {
		return
	}
	res, err := h.svc.ListPublished(r.Context(), r.URL.Query().Get("cursor"), atoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeStoryList(w, res)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ListOwn(r.Context(), uid, r.URL.Query().Get("cursor"), atoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeStoryList(w, res)
}

func (h *Handler) CreateStory(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Title        string  `json:"title"`
		Description  *string `json:"description"`
		CoverAssetID *string `json:"cover_asset_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	cover, perr := parseOptID(body.CoverAssetID)
	if perr {
		badReq(w, "invalid cover_asset_id")
		return
	}
	st, err := h.svc.CreateStory(r.Context(), CreateStoryInput{OwnerID: uid, Title: body.Title, Description: body.Description, CoverAssetID: cover})
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, storyJSON(st))
}

func (h *Handler) GetStory(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	st, err := h.svc.GetStory(r.Context(), uid, id)
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	chapters, _ := h.svc.ListChapters(r.Context(), id)
	m := storyJSON(st)
	m["chapter_count"] = len(chapters)
	m["chapters"] = chapterSummaries(chapters)
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) UpdateStory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title        *string         `json:"title"`
		Description  *string         `json:"description"`
		CoverAssetID json.RawMessage `json:"cover_asset_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	cover, setCover, perr := parseRawOptID(body.CoverAssetID)
	if perr {
		badReq(w, "invalid cover_asset_id")
		return
	}
	st, err := h.svc.UpdateStory(r.Context(), UpdateStoryInput{ID: id, Title: body.Title, Description: body.Description, SetCover: setCover, CoverAssetID: cover})
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, storyJSON(st))
}

func (h *Handler) DeleteStory(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteStory(r.Context(), id); err != nil {
		writeStoryErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	st, err := h.svc.Publish(r.Context(), id)
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, storyJSON(st))
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	st, err := h.svc.Unpublish(r.Context(), id)
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, storyJSON(st))
}

// ══ Chapters ═════════════════════════════════════════════════════════════

func (h *Handler) CreateChapter(w http.ResponseWriter, r *http.Request) {
	storyID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title     string `json:"title"`
		BodyMd    string `json:"body_md"`
		SortOrder int    `json:"sort_order"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, err := h.svc.CreateChapter(r.Context(), storyID, body.Title, body.BodyMd, body.SortOrder)
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, chapterJSON(c))
}

// Chapters is the reader payload: the story's chapters with bodies (published-or-owner).
func (h *Handler) Chapters(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	storyID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	chapters, err := h.svc.ChaptersVisible(r.Context(), uid, storyID)
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	out := make([]any, 0, len(chapters))
	for _, c := range chapters {
		out = append(out, chapterJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"chapters": out})
}

func (h *Handler) ReorderChapters(w http.ResponseWriter, r *http.Request) {
	storyID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ids, ok := decodeOrder(w, r)
	if !ok {
		return
	}
	if err := h.svc.ReorderChapters(r.Context(), storyID, ids); err != nil {
		writeStoryErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) UpdateChapter(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title  *string `json:"title"`
		BodyMd *string `json:"body_md"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateChapter(r.Context(), id, body.Title, body.BodyMd)
	if err != nil {
		writeStoryErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, chapterJSON(c))
}

func (h *Handler) DeleteChapter(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteChapter(r.Context(), id); err != nil {
		writeStoryErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── JSON shapes ───────────────────────────────────────────────────────

func storyJSON(st Story) map[string]any {
	m := map[string]any{
		"id": st.ID, "owner_id": st.OwnerID, "title": st.Title, "description": st.Description,
		"cover_asset_id": uuidPtrJSON(st.CoverAssetID), "status": st.Status,
		"created_at": st.CreatedAt.Format(time.RFC3339), "updated_at": st.UpdatedAt.Format(time.RFC3339),
	}
	if st.ChapterCount > 0 {
		m["chapter_count"] = st.ChapterCount
	}
	return m
}

func chapterJSON(c Chapter) map[string]any {
	return map[string]any{
		"id": c.ID, "story_id": c.StoryID, "title": c.Title, "body_md": c.BodyMd, "sort_order": c.SortOrder,
		"created_at": c.CreatedAt.Format(time.RFC3339), "updated_at": c.UpdatedAt.Format(time.RFC3339),
	}
}

// chapterSummaries omits body_md (list view) — the reader endpoint returns bodies.
func chapterSummaries(cs []Chapter) []any {
	out := make([]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, map[string]any{"id": c.ID, "title": c.Title, "sort_order": c.SortOrder})
	}
	return out
}

func writeStoryList(w http.ResponseWriter, res ListResult) {
	items := make([]any, 0, len(res.Items))
	for _, st := range res.Items {
		items = append(items, storyJSON(st))
	}
	out := map[string]any{"stories": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	writeJSON(w, http.StatusOK, out)
}

// ── request/response helpers ──────────────────────────────────────────

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
		writeProblem(w, http.StatusNotFound, "story/not-found", "Not Found", "not found")
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(v); err != nil {
		badReq(w, "invalid JSON body")
		return false
	}
	return true
}

func decodeOrder(w http.ResponseWriter, r *http.Request) ([]uuid.UUID, bool) {
	var body struct {
		Order []string `json:"order"`
	}
	if !decode(w, r, &body) {
		return nil, false
	}
	ids := make([]uuid.UUID, 0, len(body.Order))
	for _, s := range body.Order {
		id, err := uuid.Parse(s)
		if err != nil {
			badReq(w, "invalid id in order")
			return nil, false
		}
		ids = append(ids, id)
	}
	return ids, true
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

func writeStoryErr(w http.ResponseWriter, err error) {
	var np *NotPublishableError
	switch {
	case errors.As(err, &np):
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "story/not-publishable", "title": "Not publishable", "status": 422,
			"detail": "a story needs at least one chapter, each with a non-empty body", "chapters": np.Chapters,
		})
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "story/not-found", "Not Found", "not found")
	case errors.Is(err, ErrInvalidCoverAsset):
		writeProblem(w, http.StatusUnprocessableEntity, "story/invalid-cover-asset", "Invalid cover asset", "cover must be a ready image asset you own")
	case errors.Is(err, ErrValidation):
		writeProblem(w, http.StatusUnprocessableEntity, "story/validation", "Validation error", "the request is invalid")
	case errors.Is(err, ErrBadCursor):
		writeProblem(w, http.StatusBadRequest, "story/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
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
