package people

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

type birthdayReq struct {
	Month    int    `json:"month"`
	Day      int    `json:"day"`
	Year     *int   `json:"year"`
	Calendar string `json:"calendar"`
}

func (b *birthdayReq) toDomain() *Birthday {
	if b == nil {
		return nil
	}
	cal := b.Calendar
	if cal == "" {
		cal = "solar"
	}
	return &Birthday{Month: b.Month, Day: b.Day, Year: b.Year, Calendar: cal}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		DisplayName  string          `json:"display_name"`
		Relationship *string         `json:"relationship"`
		Birthday     *birthdayReq    `json:"birthday"`
		Contact      json.RawMessage `json:"contact"`
		NoteMd       *string         `json:"note_md"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	p, err := h.svc.CreatePerson(r.Context(), CreatePersonInput{
		UserID: uid, DisplayName: body.DisplayName, Relationship: body.Relationship,
		Birthday: body.Birthday.toDomain(), Contact: json.RawMessage(body.Contact), NoteMd: body.NoteMd,
	})
	if err != nil {
		writePeopleErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, personJSON(p))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	res, err := h.svc.ListPeople(r.Context(), uid, r.URL.Query().Get("cursor"), server.AtoiSafe(r.URL.Query().Get("limit")))
	if err != nil {
		writePeopleErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Items))
	for _, p := range res.Items {
		items = append(items, personJSON(p))
	}
	out := map[string]any{"people": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	server.JSON(w, http.StatusOK, out)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	p, err := h.svc.GetPerson(r.Context(), uid, id)
	if err != nil {
		writePeopleErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, personJSON(p))
}

func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		DisplayName  *string         `json:"display_name"`
		Relationship json.RawMessage `json:"relationship"`
		Birthday     json.RawMessage `json:"birthday"`
		Contact      json.RawMessage `json:"contact"`
		NoteMd       json.RawMessage `json:"note_md"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	in := UpdatePersonInput{UserID: uid, ID: id, DisplayName: body.DisplayName, Contact: json.RawMessage(body.Contact)}
	in.Relationship, in.SetRelationship = optStr(body.Relationship)
	in.NoteMd, in.SetNote = optStr(body.NoteMd)
	if len(body.Birthday) > 0 {
		in.SetBirthday = true
		if string(body.Birthday) != "null" {
			var b birthdayReq
			if err := json.Unmarshal(body.Birthday, &b); err != nil {
				server.BadRequest(w, "invalid birthday")
				return
			}
			in.Birthday = b.toDomain()
		}
	}
	p, err := h.svc.UpdatePerson(r.Context(), in)
	if err != nil {
		writePeopleErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, personJSON(p))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeletePerson(r.Context(), uid, id); err != nil {
		writePeopleErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Upcoming(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	days := server.AtoiSafe(r.URL.Query().Get("days"))
	if days == 0 {
		days = 14
	}
	items, err := h.svc.UpcomingBirthdays(r.Context(), uid, days, time.Now())
	if err != nil {
		writePeopleErr(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for _, u := range items {
		m := map[string]any{
			"person_id": u.PersonID, "display_name": u.DisplayName,
			"next_occurrence": u.NextOccurrence.Format("2006-01-02"), "days_until": u.DaysUntil,
		}
		if u.AgeTurning != nil {
			m["age_turning"] = *u.AgeTurning
		}
		out = append(out, m)
	}
	server.JSON(w, http.StatusOK, map[string]any{"upcoming": out})
}

// ── JSON ───────────────────────────────────────────────────────────────

func personJSON(p Person) map[string]any {
	var bday any
	if p.Birthday != nil {
		bday = map[string]any{"month": p.Birthday.Month, "day": p.Birthday.Day, "year": p.Birthday.Year, "calendar": p.Birthday.Calendar}
	}
	var contact any = json.RawMessage(`{}`)
	if len(p.Contact) > 0 {
		contact = p.Contact
	}
	return map[string]any{
		"id": p.ID, "display_name": p.DisplayName, "relationship": p.Relationship,
		"birthday": bday, "contact": contact, "note_md": p.NoteMd,
		"avatar_asset_id": uuidPtrJSON(p.AvatarAssetID),
		"created_at":      p.CreatedAt.Format(time.RFC3339), "updated_at": p.UpdatedAt.Format(time.RFC3339),
	}
}

// ── helpers ────────────────────────────────────────────────────────────

func (h *Handler) auth(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		server.Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return uuid.Nil, false
	}
	return uid, true
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.Problem(w, http.StatusNotFound, "people/person-not-found", "Not Found", "person not found")
		return uuid.Nil, false
	}
	return id, true
}

// optStr interprets an optional JSON field: absent → (nil,false); null → (nil,true,clear); "x" → (&x,true).
func optStr(raw json.RawMessage) (*string, bool) {
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

func writePeopleErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		server.Problem(w, http.StatusNotFound, "people/person-not-found", "Not Found", "person not found")
	case errors.Is(err, ErrInvalidBirthday):
		server.Problem(w, http.StatusUnprocessableEntity, "people/invalid-birthday", "Invalid birthday", "month and day must be a real date; year (if given) 1900..now")
	case errors.Is(err, ErrValidation):
		server.Problem(w, http.StatusUnprocessableEntity, "people/validation", "Validation error", "the request is invalid")
	case errors.Is(err, ErrBadCursor):
		server.Problem(w, http.StatusBadRequest, "people/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
	default:
		server.Problem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
	}
}
