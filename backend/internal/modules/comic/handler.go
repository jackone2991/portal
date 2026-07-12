package comic

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

// ══ Comics ══════════════════════════════════════════════════════════════

func (h *Handler) ListComics(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth(w, r); !ok {
		return
	}
	res, err := h.svc.ListPublished(r.Context(), r.URL.Query().Get("cursor"), atoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeComicList(w, res)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ListOwn(r.Context(), uid, r.URL.Query().Get("cursor"), atoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeComicList(w, res)
}

func (h *Handler) CreateComic(w http.ResponseWriter, r *http.Request) {
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
	c, err := h.svc.CreateComic(r.Context(), CreateComicInput{OwnerID: uid, Title: body.Title, Description: body.Description, CoverAssetID: cover})
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, comicJSON(c))
}

func (h *Handler) GetComic(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.svc.GetComic(r.Context(), uid, id)
	if err != nil {
		writeComicErr(w, err)
		return
	}
	chapters, _ := h.svc.ListChapters(r.Context(), id)
	m := comicJSON(c)
	m["chapter_count"] = len(chapters)
	m["chapters"] = chaptersJSON(chapters)
	if p, err := h.svc.GetProgress(r.Context(), uid, id); err == nil {
		m["progress"] = progressJSON(p)
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handler) UpdateComic(w http.ResponseWriter, r *http.Request) {
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
	c, err := h.svc.UpdateComic(r.Context(), UpdateComicInput{ID: id, Title: body.Title, Description: body.Description, SetCover: setCover, CoverAssetID: cover})
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comicJSON(c))
}

func (h *Handler) DeleteComic(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteComic(r.Context(), id); err != nil {
		writeComicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Publish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.svc.Publish(r.Context(), id)
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comicJSON(c))
}

func (h *Handler) Unpublish(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	c, err := h.svc.Unpublish(r.Context(), id)
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, comicJSON(c))
}

// ══ Chapters ════════════════════════════════════════════════════════════

func (h *Handler) CreateChapter(w http.ResponseWriter, r *http.Request) {
	comicID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title     string `json:"title"`
		SortOrder int    `json:"sort_order"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, err := h.svc.CreateChapter(r.Context(), comicID, body.Title, body.SortOrder)
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, chapterJSON(c))
}

func (h *Handler) UpdateChapter(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Title *string `json:"title"`
	}
	if !decode(w, r, &body) {
		return
	}
	c, err := h.svc.UpdateChapter(r.Context(), id, body.Title)
	if err != nil {
		writeComicErr(w, err)
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
		writeComicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderChapters(w http.ResponseWriter, r *http.Request) {
	comicID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ids, ok := decodeOrder(w, r)
	if !ok {
		return
	}
	if err := h.svc.ReorderChapters(r.Context(), comicID, ids); err != nil {
		writeComicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ══ Pages ═══════════════════════════════════════════════════════════════

func (h *Handler) CreatePages(w http.ResponseWriter, r *http.Request) {
	chapterID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Pages []struct {
			AssetID   string `json:"asset_id"`
			SortOrder int    `json:"sort_order"`
		} `json:"pages"`
	}
	if !decode(w, r, &body) {
		return
	}
	items := make([]PageInput, 0, len(body.Pages))
	for _, p := range body.Pages {
		aid, err := uuid.Parse(p.AssetID)
		if err != nil {
			badReq(w, "invalid asset_id")
			return
		}
		items = append(items, PageInput{AssetID: aid, SortOrder: p.SortOrder})
	}
	pages, err := h.svc.CreatePages(r.Context(), chapterID, items)
	if err != nil {
		writeComicErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"pages": pagesJSON(pages)})
}

func (h *Handler) ReorderPages(w http.ResponseWriter, r *http.Request) {
	chapterID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	ids, ok := decodeOrder(w, r)
	if !ok {
		return
	}
	if err := h.svc.ReorderPages(r.Context(), chapterID, ids); err != nil {
		writeComicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) DeletePage(w http.ResponseWriter, r *http.Request) {
	id, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeletePage(r.Context(), id); err != nil {
		writeComicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReaderPages(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	chapterID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	pages, err := h.svc.ReaderPagesVisible(r.Context(), uid, chapterID)
	if err != nil {
		writeComicErr(w, err)
		return
	}
	out := make([]any, 0, len(pages))
	for _, p := range pages {
		out = append(out, map[string]any{"page_id": p.PageID, "asset_id": p.AssetID, "width": p.Width, "height": p.Height})
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": out})
}

// ══ Progress ════════════════════════════════════════════════════════════

func (h *Handler) SaveProgress(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	comicID, ok := parseID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		ChapterID string  `json:"chapter_id"`
		PageID    *string `json:"page_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	chapterID, err := uuid.Parse(body.ChapterID)
	if err != nil {
		badReq(w, "invalid chapter_id")
		return
	}
	pageID, perr := parseOptID(body.PageID)
	if perr {
		badReq(w, "invalid page_id")
		return
	}
	if err := h.svc.SaveProgress(r.Context(), uid, comicID, chapterID, pageID); err != nil {
		writeComicErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── JSON shapes ───────────────────────────────────────────────────────

func comicJSON(c Comic) map[string]any {
	m := map[string]any{
		"id": c.ID, "owner_id": c.OwnerID, "title": c.Title, "description": c.Description,
		"cover_asset_id": uuidPtrJSON(c.CoverAssetID), "status": c.Status,
		"created_at": c.CreatedAt.Format(time.RFC3339), "updated_at": c.UpdatedAt.Format(time.RFC3339),
	}
	if c.ChapterCount > 0 {
		m["chapter_count"] = c.ChapterCount
	}
	return m
}

func chapterJSON(c Chapter) map[string]any {
	return map[string]any{"id": c.ID, "comic_id": c.ComicID, "title": c.Title, "sort_order": c.SortOrder, "created_at": c.CreatedAt.Format(time.RFC3339)}
}

func chaptersJSON(cs []Chapter) []any {
	out := make([]any, 0, len(cs))
	for _, c := range cs {
		out = append(out, chapterJSON(c))
	}
	return out
}

func pageJSON(p Page) map[string]any {
	return map[string]any{"id": p.ID, "chapter_id": p.ChapterID, "asset_id": p.AssetID, "sort_order": p.SortOrder}
}

func pagesJSON(ps []Page) []any {
	out := make([]any, 0, len(ps))
	for _, p := range ps {
		out = append(out, pageJSON(p))
	}
	return out
}

func progressJSON(p Progress) map[string]any {
	return map[string]any{"chapter_id": p.ChapterID, "page_id": uuidPtrJSON(p.PageID), "updated_at": p.UpdatedAt.Format(time.RFC3339)}
}

func writeComicList(w http.ResponseWriter, res ListResult) {
	items := make([]any, 0, len(res.Items))
	for _, c := range res.Items {
		items = append(items, comicJSON(c))
	}
	out := map[string]any{"comics": items}
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
		writeProblem(w, http.StatusNotFound, "comic/not-found", "Not Found", "not found")
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

func writeComicErr(w http.ResponseWriter, err error) {
	var np *NotPublishableError
	switch {
	case errors.As(err, &np):
		w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type": "comic/not-publishable", "title": "Not publishable", "status": 422,
			"detail": "every chapter must have at least one page", "chapters": np.Chapters,
		})
	case errors.Is(err, ErrNotFound):
		writeProblem(w, http.StatusNotFound, "comic/not-found", "Not Found", "not found")
	case errors.Is(err, ErrInvalidCoverAsset):
		writeProblem(w, http.StatusUnprocessableEntity, "comic/invalid-cover-asset", "Invalid cover asset", "cover must be a ready image asset you own")
	case errors.Is(err, ErrInvalidPageAsset):
		writeProblem(w, http.StatusUnprocessableEntity, "comic/invalid-page-asset", "Invalid page asset", "each page must be a ready image asset you own")
	case errors.Is(err, ErrInvalidProgressTarget):
		writeProblem(w, http.StatusUnprocessableEntity, "comic/invalid-progress-target", "Invalid progress target", "chapter/page does not belong to this comic")
	case errors.Is(err, ErrValidation):
		writeProblem(w, http.StatusUnprocessableEntity, "comic/validation", "Validation error", "the request is invalid")
	case errors.Is(err, ErrBadCursor):
		writeProblem(w, http.StatusBadRequest, "comic/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
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
