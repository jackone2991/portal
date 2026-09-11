package comic

// HTTP-contract tests: the module's public surface is its router, and these
// drive it the way a client does — a request in, a status and a body out.
// They assert the three cross-cutting rules every domain module promises
// (TEST-PLAN CC-1, CC-3, CC-8) and nothing about internals:
//
//   - CC-3  a resource you may not see is 404, never 403 — existence does not leak;
//   - CC-1  every non-2xx body is RFC 7807 application/problem+json with the
//           stable `type` the frontend catalogue keys on;
//   - CC-8  DELETE twice is 404 the second time, never 500.
//
// Identity is injected the way cmd/api does it — through Deps.RequireAuth and
// Deps.CurrentUser — from a test header, so the handlers under test are the
// real ones. The owner-or-permission guards are nil here on purpose: they
// belong to cmd/api, and the point is that the module holds these contracts
// on its own.

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

// newHTTP mounts a real Module on a chi router over the in-memory fakes.
func newHTTP(t *testing.T) (http.Handler, *fakeRepo, *fakeMedia) {
	t.Helper()
	_, repo, media := newSvc()
	mod, err := New(Deps{
		Repo:  repo,
		Media: media,
		RequireAuth: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if raw := r.Header.Get(testUserHeader); raw != "" {
					if id, err := uuid.Parse(raw); err == nil {
						r = r.WithContext(context.WithValue(r.Context(), ctxUserKey{}, id))
					}
				}
				next.ServeHTTP(w, r)
			})
		},
		CurrentUser: func(ctx context.Context) (uuid.UUID, bool) {
			id, ok := ctx.Value(ctxUserKey{}).(uuid.UUID)
			return id, ok
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo, media
}

func do(t *testing.T, h http.Handler, as uuid.UUID, method, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, rd)
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

// CC-3 + CC-1: a stranger asking for someone else's draft gets the same answer
// as for a comic that does not exist — 404, problem+json, type comic/not-found.
// Not 403: a 403 would confirm the draft is there.
func TestHTTPDraftIsNotFoundToAStranger(t *testing.T) {
	h, repo, _ := newHTTP(t)
	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()
	c, _ := repo.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "Draft"})

	rec := do(t, h, stranger, http.MethodGet, "/comics/"+c.ID.String(), "")
	problem(t, rec, http.StatusNotFound, "comic/not-found")

	// and the answer for a comic that never existed is indistinguishable
	rec2 := do(t, h, stranger, http.MethodGet, "/comics/"+uuid.New().String(), "")
	p2 := problem(t, rec2, http.StatusNotFound, "comic/not-found")
	if rec.Body.String() != rec2.Body.String() || p2["detail"] == nil {
		t.Fatalf("draft and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}

	// the owner, of course, sees it
	if rec := do(t, h, owner, http.MethodGet, "/comics/"+c.ID.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("owner GET = %d (%s)", rec.Code, rec.Body.String())
	}
}

// CC-8: deleting twice is 404 the second time — the row is gone, and the
// module says so rather than answering 204 to a no-op (or 500). The first
// answer is the spec's 204.
func TestHTTPDeleteTwiceIs404(t *testing.T) {
	h, repo, _ := newHTTP(t)
	ctx := context.Background()
	owner := uuid.New()
	c, _ := repo.CreateComic(ctx, CreateComicInput{OwnerID: owner, Title: "Gone"})

	if rec := do(t, h, owner, http.MethodDelete, "/comics/"+c.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	rec := do(t, h, owner, http.MethodDelete, "/comics/"+c.ID.String(), "")
	problem(t, rec, http.StatusNotFound, "comic/not-found")
}
