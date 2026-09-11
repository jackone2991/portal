package bank

// HTTP-contract tests for the bank module — the same three cross-cutting
// rules the comic suite pins (TEST-PLAN CC-1, CC-3, CC-8), driven through the
// real router over the in-memory fake. Identity comes in through
// Deps.RequireAuth / Deps.CurrentUser from a test header, as cmd/api wires it.
// Money is the most sensitive data in the system, so "does another user's
// account leak its existence" is the question that matters most here.

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

func newHTTP(t *testing.T) (http.Handler, *fakeRepo) {
	t.Helper()
	_, repo := newSvc()
	mod, err := New(Deps{
		Repo: repo,
		RequireAuth: func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if id, err := uuid.Parse(r.Header.Get(testUserHeader)); err == nil {
					r = r.WithContext(context.WithValue(r.Context(), ctxUserKey{}, id))
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
	return r, repo
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

// CC-3 + CC-1: another user's account is 404 to me — on a write, where a 403
// would be the "natural" answer and would confirm the account exists. The body
// is identical to the one for an account that never existed.
func TestHTTPAnotherUsersAccountIsNotFound(t *testing.T) {
	h, repo := newHTTP(t)
	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()
	acc, _ := repo.CreateAccount(ctx, CreateAccountInput{UserID: owner, Name: "Ví", Type: "cash", Currency: "VND"})

	rec := do(t, h, stranger, http.MethodPatch, "/bank/accounts/"+acc.ID.String(), `{"name":"mine now"}`)
	problem(t, rec, http.StatusNotFound, "bank/not-found")

	rec2 := do(t, h, stranger, http.MethodPatch, "/bank/accounts/"+uuid.New().String(), `{"name":"x"}`)
	problem(t, rec2, http.StatusNotFound, "bank/not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("foreign and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}

	// the owner's rename lands, and the stranger's did not
	if rec := do(t, h, owner, http.MethodPatch, "/bank/accounts/"+acc.ID.String(), `{"name":"Ví chính"}`); rec.Code != http.StatusOK {
		t.Fatalf("owner PATCH = %d (%s)", rec.Code, rec.Body.String())
	}
	if got := repo.accounts[acc.ID].name; got != "Ví chính" {
		t.Fatalf("name after the two PATCHes = %q, want the owner's", got)
	}
}

// CC-8: DELETE twice — 204, then 404. (An account with transactions is 409
// "archive instead"; that path has its own service test.)
func TestHTTPDeleteAccountTwiceIs404(t *testing.T) {
	h, repo := newHTTP(t)
	ctx := context.Background()
	owner := uuid.New()
	acc, _ := repo.CreateAccount(ctx, CreateAccountInput{UserID: owner, Name: "Tạm", Type: "cash", Currency: "VND"})

	if rec := do(t, h, owner, http.MethodDelete, "/bank/accounts/"+acc.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	rec := do(t, h, owner, http.MethodDelete, "/bank/accounts/"+acc.ID.String(), "")
	problem(t, rec, http.StatusNotFound, "bank/not-found")
}
