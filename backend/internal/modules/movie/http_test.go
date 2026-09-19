package movie

// HTTP-contract tests: the module's public surface is its router, and these
// drive it the way a client does — a request in, a status and a body out —
// over the real handler and service with fakes underneath (the comic/bank
// pattern, 2026-09-11; backlog P1 #8). Two promises every module keeps:
//
//   - a draft that is not yours reads as 404, never 403 — the same body as a
//     missing id, so the endpoint never confirms that it exists (the write
//     routes are owner-guarded by cmd/api's middleware, outside this test);
//   - deleting twice is 204 then 404 — idempotent, and the 404 is an RFC 7807
//     problem with the module's own type.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ctxUserKey struct{}

const testUserHeader = "X-Test-User"

// requireAuth is the test stand-in for cmd/api's RequireAuth: the caller's id
// rides in a header. The permission guards stay nil — they belong to cmd/api.
func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if raw := r.Header.Get(testUserHeader); raw != "" {
			if id, err := uuid.Parse(raw); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), ctxUserKey{}, id))
			}
		}
		next.ServeHTTP(w, r)
	})
}

func currentUser(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxUserKey{}).(uuid.UUID)
	return id, ok
}

func do(t *testing.T, h http.Handler, as uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(testUserHeader, as.String())
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// problem decodes an RFC 7807 body and asserts the shape the contract fixes:
// the media type, and the three members shared/openapi.yaml marks required.
func problem(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantType string) map[string]any {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var p map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %v — %s", err, rec.Body.String())
	}
	if p["type"] != wantType {
		t.Fatalf("type = %v, want %q", p["type"], wantType)
	}
	if s, _ := p["status"].(float64); int(s) != wantStatus {
		t.Fatalf("status member = %v, want %d", p["status"], wantStatus)
	}
	if title, _ := p["title"].(string); title == "" {
		t.Fatal("title is missing or empty")
	}
	return p
}

func newHTTP(t *testing.T) (http.Handler, *fakeRepo) {
	t.Helper()
	_, repo, media, events := newSvc()
	mod, err := New(Deps{Repo: repo, Media: media, Events: events, RequireAuth: requireAuth, CurrentUser: currentUser})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo
}

// A draft is visible to its owner and to nobody else — and "nobody else" gets
// exactly the answer a missing id gets.
func TestHTTPDraftIsNotFoundToAStranger(t *testing.T) {
	h, repo := newHTTP(t)
	owner, stranger := uuid.New(), uuid.New()
	row, err := repo.CreateMovie(context.Background(), CreateMovieInput{OwnerID: owner, Title: "Draft"})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, stranger, http.MethodGet, "/movies/"+row.ID.String(), "")
	problem(t, rec, http.StatusNotFound, "movie/not-found")
	rec2 := do(t, h, stranger, http.MethodGet, "/movies/"+uuid.New().String(), "")
	problem(t, rec2, http.StatusNotFound, "movie/not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("someone else's and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}
	if rec := do(t, h, owner, http.MethodGet, "/movies/"+row.ID.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("owner GET = %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHTTPDeleteTwiceIs404(t *testing.T) {
	h, repo := newHTTP(t)
	owner := uuid.New()
	row, err := repo.CreateMovie(context.Background(), CreateMovieInput{OwnerID: owner, Title: "Gone"})
	if err != nil {
		t.Fatal(err)
	}
	if rec := do(t, h, owner, http.MethodDelete, "/movies/"+row.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	problem(t, do(t, h, owner, http.MethodDelete, "/movies/"+row.ID.String(), ""), http.StatusNotFound, "movie/not-found")
}
