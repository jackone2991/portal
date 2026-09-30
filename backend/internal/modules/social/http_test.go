package social

// HTTP-contract tests: the module's public surface is its router, and these
// drive it the way a client does — a request in, a status and a body out —
// over the real handler and service with fakes underneath (the comic/bank
// pattern, 2026-09-11; backlog P1 #8). Two promises every module keeps:
//
//   - a resource that is not yours answers 404, never 403 — the same body as
//     a missing id, so the endpoint never confirms that it exists;
//   - deleting twice is 204 then 404 — idempotent, and the 404 is an RFC 7807
//     problem with the module's own type.

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server/servertest"
)

func newHTTP(t *testing.T) (http.Handler, *fakeRepo) {
	t.Helper()
	_, repo := newSvc()
	mod, err := New(Deps{Repo: repo, RequireAuth: servertest.RequireAuth, CurrentUser: servertest.CurrentUser})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo
}

// A connection is visible to its two parties; a third account deleting it
// gets the answer a missing id gets — and the row stays.
func TestHTTPConnectionIsNotFoundToAThirdParty(t *testing.T) {
	h, repo := newHTTP(t)
	a, b, third := uuid.New(), uuid.New(), uuid.New()
	c, err := repo.CreateRequest(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	rec := servertest.Do(t, h, third, http.MethodDelete, "/connections/"+c.ID.String(), "")
	servertest.Problem(t, rec, http.StatusNotFound, "social/connection-not-found")
	rec2 := servertest.Do(t, h, third, http.MethodDelete, "/connections/"+uuid.New().String(), "")
	servertest.Problem(t, rec2, http.StatusNotFound, "social/connection-not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("someone else's and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}
	if _, err := repo.Get(context.Background(), c.ID); err != nil {
		t.Fatalf("a third party's DELETE removed the row: %v", err)
	}
}

func TestHTTPDeleteTwiceIs404(t *testing.T) {
	h, repo := newHTTP(t)
	a, b := uuid.New(), uuid.New()
	c, err := repo.CreateRequest(context.Background(), a, b)
	if err != nil {
		t.Fatal(err)
	}
	if rec := servertest.Do(t, h, a, http.MethodDelete, "/connections/"+c.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	servertest.Problem(t, servertest.Do(t, h, a, http.MethodDelete, "/connections/"+c.ID.String(), ""), http.StatusNotFound, "social/connection-not-found")
}
