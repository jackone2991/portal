package journal

// HTTP-contract tests: the module's public surface is its router, and these
// drive it the way a client does — a request in, a status and a body out —
// over the real handler and service with fakes underneath (the comic/bank
// pattern, 2026-09-11). They pin what SPEC-12 T1 (#10) promises a client:
//
//   - `asset_ids` is validated AS A WHOLE against the media module: a duplicate,
//     an eleventh element, an unknown id, a non-image, a not-ready Asset or
//     another user's Asset is 422 `journal/invalid-asset` with a `detail` that
//     names the id and the reason, and nothing is stored;
//   - a valid list is stored in the order sent, and PATCH replaces the whole list;
//   - an Entry is text or at least one Attachment: an empty body with one
//     Attachment is created, with none it is 422 `journal/invalid-body`;
//   - the Entry JSON and the stream item JSON carry `asset_ids` in one shape;
//   - the asset lookup runs under the request scope the middleware opened.
//
// Identity is injected the way cmd/api does it — through Deps.RequireAuth and
// Deps.CurrentUser — from a test header, so the handlers under test are the
// real ones. The permission guards are nil on purpose: they belong to cmd/api.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/portal/backend/internal/platform/server/servertest"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// ctxScopeKey stands in for the tenant transaction cmd/api's RequireTenant puts
// on the request context: the test middleware sets it, and the fake media
// lookup checks it arrived — the only observable proof that the service passed
// the request context through rather than a fresh one.
type ctxScopeKey struct{}

// newHTTP mounts a real Module on a chi router over the in-memory fakes.
func newHTTP(t *testing.T) (http.Handler, *fakeRepo, *fakeMedia) {
	t.Helper()
	repo, media := newFakeRepo(), newFakeMedia()
	mod, err := New(Deps{
		Repo:  repo,
		Media: media,
		// servertest's identity, plus the scope marker the fake lookup checks for.
		RequireAuth: func(next http.Handler) http.Handler {
			return servertest.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if id, ok := servertest.CurrentUser(r.Context()); ok {
					r = r.WithContext(context.WithValue(r.Context(), ctxScopeKey{}, id)) // "the tenant scope is open"
				}
				next.ServeHTTP(w, r)
			}))
		},
		CurrentUser: servertest.CurrentUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo, media
}

// entryOut decodes a 2xx Entry body.
func entryOut(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int) map[string]any {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, wantStatus, rec.Body.String())
	}
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body is not JSON: %v — %s", err, rec.Body.String())
	}
	return m
}

func idsJSON(ids ...uuid.UUID) string {
	b, _ := json.Marshal(ids)
	return string(b)
}

func assetIDsOf(t *testing.T, m map[string]any) []string {
	t.Helper()
	raw, ok := m["asset_ids"].([]any)
	if !ok {
		t.Fatalf("asset_ids = %#v, want an array (never null)", m["asset_ids"])
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		out = append(out, v.(string))
	}
	return out
}

// A valid list is stored in the order sent and comes back in the Entry JSON in
// that order — array order is display order.
func TestHTTPCreateStoresAttachmentsInOrder(t *testing.T) {
	h, repo, media := newHTTP(t)
	owner := uuid.New()
	a, b, c := media.readyImage(owner), media.readyImage(owner), media.readyImage(owner)

	rec := servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"three pictures","asset_ids":`+idsJSON(c, a, b)+`}`)
	out := entryOut(t, rec, http.StatusCreated)
	if got, want := assetIDsOf(t, out), []string{c.String(), a.String(), b.String()}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("asset_ids = %v, want %v (order preserved)", got, want)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("stored rows = %d, want 1", len(repo.rows))
	}
	for _, e := range repo.rows {
		if len(e.AssetIDs) != 3 || e.AssetIDs[0] != c || e.AssetIDs[1] != a || e.AssetIDs[2] != b {
			t.Fatalf("stored asset_ids = %v, want [%s %s %s]", e.AssetIDs, c, a, b)
		}
	}
}

// Every way the list can be invalid is 422 journal/invalid-asset, the detail
// names the offending id, and nothing is stored (the whole request fails).
func TestHTTPInvalidAttachmentListIsRefusedWhole(t *testing.T) {
	owner, stranger := uuid.New(), uuid.New()

	cases := []struct {
		name string
		ids  func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) // list, and the id the detail must name
	}{
		{"duplicate", func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) {
			a := m.readyImage(owner)
			return []uuid.UUID{a, a}, a
		}},
		{"eleventh element", func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) {
			ids := make([]uuid.UUID, 0, 11)
			for i := 0; i < 11; i++ {
				ids = append(ids, m.readyImage(owner))
			}
			return ids, ids[10]
		}},
		{"unknown id", func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) {
			x := uuid.New()
			return []uuid.UUID{m.readyImage(owner), x}, x
		}},
		{"non-image", func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) {
			v := m.asset(owner, mediaapi.KindVideo, mediaapi.StatusReady)
			return []uuid.UUID{v}, v
		}},
		{"not ready", func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) {
			p := m.asset(owner, mediaapi.KindImage, mediaapi.StatusProcessing)
			return []uuid.UUID{p}, p
		}},
		{"another user's asset", func(m *fakeMedia) ([]uuid.UUID, uuid.UUID) {
			s := m.readyImage(stranger)
			return []uuid.UUID{m.readyImage(owner), s}, s
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, repo, media := newHTTP(t)
			ids, culprit := tc.ids(media)

			rec := servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"x","asset_ids":`+idsJSON(ids...)+`}`)
			p := servertest.Problem(t, rec, http.StatusUnprocessableEntity, "journal/invalid-asset")
			if detail, _ := p["detail"].(string); !strings.Contains(detail, culprit.String()) {
				t.Fatalf("detail = %q, want it to name %s", detail, culprit)
			}
			if len(repo.rows) != 0 || repo.streamCount() != 0 {
				t.Fatalf("stored %d rows / %d stream items, want nothing stored", len(repo.rows), repo.streamCount())
			}
		})
	}
}

// A malformed id is reported the same way — naming the string — not as a 400
// that hides which element was wrong.
func TestHTTPMalformedAssetIDNamesIt(t *testing.T) {
	h, _, _ := newHTTP(t)
	rec := servertest.Do(t, h, uuid.New(), http.MethodPost, "/journal/entries", `{"body_md":"x","asset_ids":["not-a-uuid"]}`)
	p := servertest.Problem(t, rec, http.StatusUnprocessableEntity, "journal/invalid-asset")
	if detail, _ := p["detail"].(string); !strings.Contains(detail, "not-a-uuid") {
		t.Fatalf("detail = %q, want it to name the bad element", detail)
	}
}

// PATCH with asset_ids replaces the whole array — it never merges — and an
// empty array clears it (while text remains).
func TestHTTPPatchReplacesWholeList(t *testing.T) {
	h, repo, media := newHTTP(t)
	owner := uuid.New()
	a, b, c := media.readyImage(owner), media.readyImage(owner), media.readyImage(owner)

	created := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"t","asset_ids":`+idsJSON(a, b)+`}`), http.StatusCreated)
	id := created["id"].(string)

	out := entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"asset_ids":`+idsJSON(c)+`}`), http.StatusOK)
	if got := assetIDsOf(t, out); len(got) != 1 || got[0] != c.String() {
		t.Fatalf("after patch asset_ids = %v, want [%s] (replaced, not merged)", got, c)
	}
	if e := repo.rows[uuid.MustParse(id)]; len(e.AssetIDs) != 1 || e.AssetIDs[0] != c {
		t.Fatalf("stored asset_ids = %v, want [%s]", e.AssetIDs, c)
	}

	// A patch that does not mention asset_ids keeps them.
	out = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"body_md":"edited"}`), http.StatusOK)
	if got := assetIDsOf(t, out); len(got) != 1 || got[0] != c.String() {
		t.Fatalf("body-only patch changed asset_ids to %v", got)
	}

	// An explicit empty list clears them.
	out = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"asset_ids":[]}`), http.StatusOK)
	if got := assetIDsOf(t, out); len(got) != 0 {
		t.Fatalf("after clearing asset_ids = %v, want []", got)
	}

	// An invalid replacement leaves the stored list untouched.
	_ = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"asset_ids":`+idsJSON(a)+`}`), http.StatusOK)
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"asset_ids":`+idsJSON(uuid.New())+`}`), http.StatusUnprocessableEntity, "journal/invalid-asset")
	if e := repo.rows[uuid.MustParse(id)]; len(e.AssetIDs) != 1 || e.AssetIDs[0] != a {
		t.Fatalf("a refused patch changed the stored list to %v", e.AssetIDs)
	}
}

// An Entry is text, or at least one Attachment, or both — never neither.
func TestHTTPTextOrAttachment(t *testing.T) {
	h, repo, media := newHTTP(t)
	owner := uuid.New()
	a := media.readyImage(owner)

	// photo-only: created, body is "".
	out := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"asset_ids":`+idsJSON(a)+`}`), http.StatusCreated)
	if out["body_md"] != "" {
		t.Fatalf("photo-only body_md = %#v, want \"\"", out["body_md"])
	}
	photoOnly := out["id"].(string)

	// nothing at all, three spellings: absent body, empty body, blank body.
	for _, body := range []string{`{}`, `{"body_md":""}`, `{"body_md":"  \n ","asset_ids":[]}`} {
		servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", body), http.StatusUnprocessableEntity, "journal/invalid-body")
	}
	if len(repo.rows) != 1 {
		t.Fatalf("stored rows = %d, want only the photo-only entry", len(repo.rows))
	}

	// The rule is judged on the RESULT of a patch: clearing the only photo of a
	// photo-only Entry is refused, as is blanking the only text of a text-only one.
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+photoOnly, `{"asset_ids":[]}`), http.StatusUnprocessableEntity, "journal/invalid-body")
	textOnly := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"words"}`), http.StatusCreated)["id"].(string)
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+textOnly, `{"body_md":""}`), http.StatusUnprocessableEntity, "journal/invalid-body")
	// …but swapping text for a photo in one request is fine.
	entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+textOnly, `{"body_md":"","asset_ids":`+idsJSON(a)+`}`), http.StatusOK)
}

// The Entry JSON and the stream item JSON carry asset_ids in the same shape —
// one renderer serves both (SPEC-12 story 32). A text-only Entry carries [],
// never null.
func TestHTTPEntryAndStreamItemShareAssetIDsShape(t *testing.T) {
	h, _, media := newHTTP(t)
	owner := uuid.New()
	a, b := media.readyImage(owner), media.readyImage(owner)

	withPhotos := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"p","asset_ids":`+idsJSON(a, b)+`}`), http.StatusCreated)
	textOnly := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"t"}`), http.StatusCreated)
	if got := assetIDsOf(t, textOnly); len(got) != 0 {
		t.Fatalf("text-only entry asset_ids = %v, want []", got)
	}

	// GET /journal/entries/{id} agrees with the create response.
	fetched := entryOut(t, servertest.Do(t, h, owner, http.MethodGet, "/journal/entries/"+withPhotos["id"].(string), ""), http.StatusOK)
	if fmt.Sprint(assetIDsOf(t, fetched)) != fmt.Sprint(assetIDsOf(t, withPhotos)) {
		t.Fatalf("GET asset_ids = %v, want %v", assetIDsOf(t, fetched), assetIDsOf(t, withPhotos))
	}

	// The stream carries the same array on the journal item.
	rec := servertest.Do(t, h, owner, http.MethodGet, "/stream", "")
	page := entryOut(t, rec, http.StatusOK)
	items, _ := page["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("stream items = %d, want 2", len(items))
	}
	seen := 0
	for _, it := range items {
		m := it.(map[string]any)
		if m["source_module"] != "journal" {
			continue
		}
		got := assetIDsOf(t, m) // fails on null
		switch m["ref_id"] {
		case withPhotos["id"]:
			if len(got) != 2 || got[0] != a.String() || got[1] != b.String() {
				t.Fatalf("stream item asset_ids = %v, want [%s %s]", got, a, b)
			}
			seen++
		case textOnly["id"]:
			if len(got) != 0 {
				t.Fatalf("text-only stream item asset_ids = %v, want []", got)
			}
			seen++
		}
	}
	if seen != 2 {
		t.Fatalf("matched %d journal stream items to entries, want 2", seen)
	}
}

// The asset lookup runs under the request context the middleware scoped — the
// same transaction RequireTenant opened in cmd/api — never a bare one (ADR-07).
func TestHTTPAssetLookupRunsInsideRequestScope(t *testing.T) {
	h, _, media := newHTTP(t)
	owner := uuid.New()
	a := media.readyImage(owner)

	entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"x","asset_ids":`+idsJSON(a)+`}`), http.StatusCreated)
	if len(media.calls) != 1 {
		t.Fatalf("GetAsset calls = %d, want 1", len(media.calls))
	}
	if got, _ := media.calls[0].Value(ctxScopeKey{}).(uuid.UUID); got != owner {
		t.Fatalf("GetAsset ran outside the request scope (scope = %v, want %s)", got, owner)
	}
}

// A text-only create sends no lookup at all — attaching only reads an Asset,
// and there is nothing to read.
func TestHTTPTextOnlyCreateSkipsLookup(t *testing.T) {
	h, _, media := newHTTP(t)
	entryOut(t, servertest.Do(t, h, uuid.New(), http.MethodPost, "/journal/entries", `{"body_md":"x"}`), http.StatusCreated)
	if len(media.calls) != 0 {
		t.Fatalf("GetAsset calls = %d, want 0", len(media.calls))
	}
}

// ── Location (SPEC-12 T3, #12) ──────────────────────────────────────

// locationOf reads the `location` member, which must be PRESENT on every Entry
// and journal stream item — an object or null, never absent.
func locationOf(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	raw, present := m["location"]
	if !present {
		t.Fatalf("location is absent from %v; want an object or null", m)
	}
	if raw == nil {
		return nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("location = %#v, want an object or null", raw)
	}
	return obj
}

func wantLocation(t *testing.T, got map[string]any, name string, lat, lon float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("location = null, want {%q %v %v}", name, lat, lon)
	}
	if got["name"] != name || got["lat"] != lat || got["lon"] != lon {
		t.Fatalf("location = %v, want {%q %v %v}", got, name, lat, lon)
	}
}

// A valid Location is stored, comes back on the create response, on GET, and
// on the stream item in the same shape; an Entry without one carries null.
func TestHTTPLocationStoredAndSharedShape(t *testing.T) {
	h, repo, _ := newHTTP(t)
	owner := uuid.New()

	placed := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries",
		`{"body_md":"phở sáng","location":{"name":"  Phở Thìn  ","lat":21.0285,"lon":105.8542}}`), http.StatusCreated)
	wantLocation(t, locationOf(t, placed), "Phở Thìn", 21.0285, 105.8542) // name trimmed
	e := repo.rows[uuid.MustParse(placed["id"].(string))]
	if e.Location == nil || e.Location.Name != "Phở Thìn" || e.Location.Lat != 21.0285 || e.Location.Lon != 105.8542 {
		t.Fatalf("stored location = %+v", e.Location)
	}

	plain := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"no place"}`), http.StatusCreated)
	if locationOf(t, plain) != nil {
		t.Fatalf("location = %v, want null", plain["location"])
	}

	fetched := entryOut(t, servertest.Do(t, h, owner, http.MethodGet, "/journal/entries/"+placed["id"].(string), ""), http.StatusOK)
	wantLocation(t, locationOf(t, fetched), "Phở Thìn", 21.0285, 105.8542)

	page := entryOut(t, servertest.Do(t, h, owner, http.MethodGet, "/stream", ""), http.StatusOK)
	items, _ := page["items"].([]any)
	seen := 0
	for _, it := range items {
		m := it.(map[string]any)
		switch m["ref_id"] {
		case placed["id"]:
			wantLocation(t, locationOf(t, m), "Phở Thìn", 21.0285, 105.8542)
			seen++
		case plain["id"]:
			if locationOf(t, m) != nil {
				t.Fatalf("stream item location = %v, want null", m["location"])
			}
			seen++
		}
	}
	if seen != 2 {
		t.Fatalf("matched %d journal stream items, want 2", seen)
	}
}

// Every way a Location can be wrong is 422 journal/invalid-location and
// nothing is stored.
func TestHTTPInvalidLocationIsRefused(t *testing.T) {
	cases := []struct{ name, location string }{
		{"name only", `{"name":"Hà Nội"}`},
		{"coordinates only", `{"lat":21.0285,"lon":105.8542}`},
		{"empty name", `{"name":"","lat":21.0285,"lon":105.8542}`},
		{"blank name", `{"name":"   ","lat":21.0285,"lon":105.8542}`},
		{"latitude too far north", `{"name":"x","lat":90.0001,"lon":0}`},
		{"latitude too far south", `{"name":"x","lat":-91,"lon":0}`},
		{"longitude too far east", `{"name":"x","lat":0,"lon":180.5}`},
		{"longitude too far west", `{"name":"x","lat":0,"lon":-181}`},
		{"latitude not a number", `{"name":"x","lat":"21","lon":105}`},
		{"not an object", `"Hà Nội"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, repo, _ := newHTTP(t)
			rec := servertest.Do(t, h, uuid.New(), http.MethodPost, "/journal/entries", `{"body_md":"x","location":`+tc.location+`}`)
			servertest.Problem(t, rec, http.StatusUnprocessableEntity, "journal/invalid-location")
			if len(repo.rows) != 0 {
				t.Fatalf("stored %d rows, want nothing stored", len(repo.rows))
			}
		})
	}
}

// PATCH: an object sets, null clears, absent keeps — and a refused patch
// leaves the stored Location untouched.
func TestHTTPPatchLocationSetClearKeep(t *testing.T) {
	h, repo, _ := newHTTP(t)
	owner := uuid.New()
	id := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"t"}`), http.StatusCreated)["id"].(string)

	out := entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"location":{"name":"Đà Lạt","lat":11.9404,"lon":108.4583}}`), http.StatusOK)
	wantLocation(t, locationOf(t, out), "Đà Lạt", 11.9404, 108.4583)

	out = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"body_md":"edited"}`), http.StatusOK)
	wantLocation(t, locationOf(t, out), "Đà Lạt", 11.9404, 108.4583) // absent keeps

	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"location":{"name":"","lat":1,"lon":1}}`), http.StatusUnprocessableEntity, "journal/invalid-location")
	if e := repo.rows[uuid.MustParse(id)]; e.Location == nil || e.Location.Name != "Đà Lạt" {
		t.Fatalf("a refused patch changed the stored location to %+v", e.Location)
	}

	out = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"location":null}`), http.StatusOK)
	if locationOf(t, out) != nil {
		t.Fatalf("after clearing location = %v, want null", out["location"])
	}
	if e := repo.rows[uuid.MustParse(id)]; e.Location != nil {
		t.Fatalf("stored location after clear = %+v, want nil", e.Location)
	}
}

// A Location alone does not make an Entry (SPEC-12 story 7): with neither text
// nor an Attachment it is the same 422 journal/invalid-body as an empty one,
// on create and on a patch whose result would be place-only.
func TestHTTPLocationAloneIsNotAnEntry(t *testing.T) {
	h, repo, media := newHTTP(t)
	owner := uuid.New()
	loc := `{"name":"Hà Nội","lat":21.0285,"lon":105.8542}`

	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"location":`+loc+`}`), http.StatusUnprocessableEntity, "journal/invalid-body")
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"  ","asset_ids":[],"location":`+loc+`}`), http.StatusUnprocessableEntity, "journal/invalid-body")
	if len(repo.rows) != 0 {
		t.Fatalf("stored %d rows, want none", len(repo.rows))
	}

	a := media.readyImage(owner)
	id := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"asset_ids":`+idsJSON(a)+`,"location":`+loc+`}`), http.StatusCreated)["id"].(string)
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"asset_ids":[]}`), http.StatusUnprocessableEntity, "journal/invalid-body")
}

// ── Mood on PATCH (SPEC-12 T5, #14) ─────────────────────────────────

// PATCH mood follows the location's three states — a string sets, null
// clears, absent keeps — so the edit surface can take a mood away, not only
// change it. A blank string is still 422 journal/invalid-mood.
func TestHTTPPatchMoodSetClearKeep(t *testing.T) {
	h, repo, _ := newHTTP(t)
	owner := uuid.New()
	id := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"t","mood":"calm"}`), http.StatusCreated)["id"].(string)

	out := entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"body_md":"edited"}`), http.StatusOK)
	if out["mood"] != "calm" {
		t.Fatalf("absent mood changed it to %#v; want kept", out["mood"])
	}

	out = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"mood":"  tired  "}`), http.StatusOK)
	if out["mood"] != "tired" {
		t.Fatalf("mood = %#v, want \"tired\" (set, trimmed)", out["mood"])
	}

	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"mood":"   "}`), http.StatusUnprocessableEntity, "journal/invalid-mood")
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"mood":123}`), http.StatusUnprocessableEntity, "journal/invalid-mood") // wrong type, same problem
	if e := repo.rows[uuid.MustParse(id)]; e.Mood == nil || *e.Mood != "tired" {
		t.Fatalf("a refused mood changed the stored one to %v", e.Mood)
	}

	out = entryOut(t, servertest.Do(t, h, owner, http.MethodPatch, "/journal/entries/"+id, `{"mood":null}`), http.StatusOK)
	if out["mood"] != nil {
		t.Fatalf("after clearing mood = %#v, want null", out["mood"])
	}
	if e := repo.rows[uuid.MustParse(id)]; e.Mood != nil {
		t.Fatalf("stored mood after clear = %v, want nil", *e.Mood)
	}
}

// ── the two promises every module keeps (backlog P1 #8, CC-3 / CC-8) ────

// An Entry that is not yours answers 404, never 403 — the same body as a
// missing id, so the endpoint never confirms that it exists.
func TestHTTPEntryIsNotFoundToAStranger(t *testing.T) {
	h, _, _ := newHTTP(t)
	owner, stranger := uuid.New(), uuid.New()
	id := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"mine"}`), http.StatusCreated)["id"].(string)
	rec := servertest.Do(t, h, stranger, http.MethodGet, "/journal/entries/"+id, "")
	servertest.Problem(t, rec, http.StatusNotFound, "journal/entry-not-found")
	rec2 := servertest.Do(t, h, stranger, http.MethodGet, "/journal/entries/"+uuid.NewString(), "")
	servertest.Problem(t, rec2, http.StatusNotFound, "journal/entry-not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("someone else's and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}
	// …and a stranger's PATCH or DELETE changes nothing, with the same answer.
	servertest.Problem(t, servertest.Do(t, h, stranger, http.MethodPatch, "/journal/entries/"+id, `{"body_md":"theirs"}`), http.StatusNotFound, "journal/entry-not-found")
	servertest.Problem(t, servertest.Do(t, h, stranger, http.MethodDelete, "/journal/entries/"+id, ""), http.StatusNotFound, "journal/entry-not-found")
	if out := entryOut(t, servertest.Do(t, h, owner, http.MethodGet, "/journal/entries/"+id, ""), http.StatusOK); out["body_md"] != "mine" {
		t.Fatalf("owner's Entry after a stranger's writes = %v", out["body_md"])
	}
}

func TestHTTPDeleteTwiceIs404(t *testing.T) {
	h, _, _ := newHTTP(t)
	owner := uuid.New()
	id := entryOut(t, servertest.Do(t, h, owner, http.MethodPost, "/journal/entries", `{"body_md":"gone"}`), http.StatusCreated)["id"].(string)
	if rec := servertest.Do(t, h, owner, http.MethodDelete, "/journal/entries/"+id, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodDelete, "/journal/entries/"+id, ""), http.StatusNotFound, "journal/entry-not-found")
}
