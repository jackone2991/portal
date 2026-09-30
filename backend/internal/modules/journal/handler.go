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

// entryReq is the create/patch body. Every field is a pointer so absence can be
// told from an explicit value: on PATCH a nil field is "keep", and a present
// asset_ids (even `[]`) replaces the whole list (SPEC-12 T1). Location and
// mood are kept raw because they have THREE wire states — absent (keep),
// `null` (clear), a value (set) — and a pointer collapses the first two
// (SPEC-12 T3, T5).
type entryReq struct {
	BodyMd     *string         `json:"body_md"`
	Mood       json.RawMessage `json:"mood"`
	OccurredAt *time.Time      `json:"occurred_at"`
	AssetIDs   *[]string       `json:"asset_ids"`
	Location   json.RawMessage `json:"location"`
}

// mood decodes the raw member: (set=false) when absent, (set=true, nil) for
// `null`, (set=true, &s) for a string. Anything else is ErrInvalidMood — the
// service then judges the string itself (1–80 characters after trimming).
func (b *entryReq) mood() (set bool, mood *string, err error) {
	if len(b.Mood) == 0 {
		return false, nil, nil
	}
	if string(b.Mood) == "null" {
		return true, nil, nil
	}
	var s string
	if err := json.Unmarshal(b.Mood, &s); err != nil {
		return true, nil, ErrInvalidMood
	}
	return true, &s, nil
}

// locationReq is the wire Location with every field optional, so a name-only
// or coordinates-only object is detectable as such rather than defaulting to
// "" / 0 and slipping through as a place at the equator.
type locationReq struct {
	Name *string  `json:"name"`
	Lat  *float64 `json:"lat"`
	Lon  *float64 `json:"lon"`
}

// location decodes the raw member: (set=false) when absent, (set=true, nil)
// for `null`, (set=true, loc) for a whole object. Anything else — a partial
// object, a wrong type, a bare string — is ErrInvalidLocation (422), because a
// client that sent a location and got a 400 could not tell which field was
// wrong; the bounds are the service's.
func (b *entryReq) location() (set bool, loc *Location, err error) {
	if len(b.Location) == 0 {
		return false, nil, nil
	}
	if string(b.Location) == "null" {
		return true, nil, nil
	}
	var l locationReq
	if err := json.Unmarshal(b.Location, &l); err != nil || l.Name == nil || l.Lat == nil || l.Lon == nil {
		return true, nil, ErrInvalidLocation
	}
	return true, &Location{Name: *l.Name, Lat: *l.Lat, Lon: *l.Lon}, nil
}

// assetIDs parses the wire list. A string that is not a uuid cannot be an Asset
// of ours, so it is reported the way any other invalid Attachment is — an
// AssetError naming it — rather than as a 400 that hides which element.
func (b *entryReq) assetIDs() (*[]uuid.UUID, error) {
	if b.AssetIDs == nil {
		return nil, nil
	}
	ids := make([]uuid.UUID, 0, len(*b.AssetIDs))
	for _, raw := range *b.AssetIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, &AssetError{ID: raw, Reason: "is not a uuid"}
		}
		ids = append(ids, id)
	}
	return &ids, nil
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
	ids, err := body.assetIDs()
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	_, loc, err := body.location() // on create, absent and null are the same: no Location
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	_, mood, err := body.mood() // likewise: absent and null are both "no mood"
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	p := CreateParams{UserID: uid, Mood: mood, OccurredAt: body.OccurredAt, Location: loc}
	if body.BodyMd != nil {
		p.BodyMd = *body.BodyMd // absent = "" — legal only with an Attachment; the service decides
	}
	if ids != nil {
		p.AssetIDs = *ids
	}
	entry, err := h.svc.Create(r.Context(), p)
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
	ids, err := body.assetIDs()
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	setLoc, loc, err := body.location()
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	setMood, mood, err := body.mood()
	if err != nil {
		writeJournalErr(w, err)
		return
	}
	entry, err := h.svc.Patch(r.Context(), PatchParams{
		UserID:      uid,
		ID:          id,
		BodyMd:      body.BodyMd,
		SetMood:     setMood,
		Mood:        mood,
		OccurredAt:  body.OccurredAt,
		AssetIDs:    ids,
		SetLocation: setLoc,
		Location:    loc,
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
			"ref_id":        c.RefID,
			"occurred_at":   c.OccurredAt.Format(time.RFC3339),
		}
		if c.SourceModule == "journal" {
			m["body_md"] = c.BodyMd
			m["mood"] = c.Mood
			m["asset_ids"] = uuidStrings(c.AssetIDs) // same shape as the Entry (SPEC-12 story 32)
			m["location"] = locationJSON(c.Location)
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
	return map[string]any{
		"id":          e.ID,
		"body_md":     e.BodyMd,
		"mood":        e.Mood,
		"asset_ids":   uuidStrings(e.AssetIDs),
		"location":    locationJSON(e.Location),
		"occurred_at": e.OccurredAt.Format(time.RFC3339),
		"created_at":  e.CreatedAt.Format(time.RFC3339),
		"updated_at":  e.UpdatedAt.Format(time.RFC3339),
	}
}

// uuidStrings renders an id list as JSON strings — always an array, never null,
// so a client can index it without a guard.
func uuidStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

// locationJSON renders the Location member: an object, or an explicit null —
// the key is always present so a client can read it without a guard.
func locationJSON(l *Location) any {
	if l == nil {
		return nil
	}
	return map[string]any{"name": l.Name, "lat": l.Lat, "lon": l.Lon}
}

// writeJournalErr maps a service error to its RFC 7807 Problem (§7 type URIs).
func writeJournalErr(w http.ResponseWriter, err error) {
	var assetErr *AssetError
	switch {
	case errors.Is(err, ErrEntryNotFound):
		server.Problem(w, http.StatusNotFound, "journal/entry-not-found", "Not Found", "journal entry not found")
	case errors.Is(err, ErrInvalidBody):
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-body", "Invalid body", "an entry needs text or at least one attachment, and body_md is at most 20000 characters")
	case errors.Is(err, ErrInvalidMood):
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-mood", "Invalid mood", "mood must be 1–80 non-blank characters")
	case errors.As(err, &assetErr):
		// ErrInvalidAsset only ever travels as an *AssetError: the detail names
		// the id and the reason (SPEC-12 story 28) so the client can fix the
		// right element of the list.
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-asset", "Invalid asset", "asset "+assetErr.ID+" "+assetErr.Reason)
	case errors.Is(err, ErrInvalidLocation):
		server.Problem(w, http.StatusUnprocessableEntity, "journal/invalid-location", "Invalid location", "location must be an object with a non-empty name, lat in [-90, 90] and lon in [-180, 180] — or null to clear it")
	case errors.Is(err, ErrBadCursor):
		server.Problem(w, http.StatusBadRequest, "journal/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
	default:
		server.Problem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
	}
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}
