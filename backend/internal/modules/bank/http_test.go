package bank

// HTTP-contract tests for the bank module — the same three cross-cutting
// rules the comic suite pins (TEST-PLAN CC-1, CC-3, CC-8), driven through the
// real router over the in-memory fake. Identity comes in through
// Deps.RequireAuth / Deps.CurrentUser from a test header, as cmd/api wires it.
// Money is the most sensitive data in the system, so "does another user's
// account leak its existence" is the question that matters most here.

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
	mod, err := New(Deps{
		Repo:        repo,
		RequireAuth: servertest.RequireAuth,
		CurrentUser: servertest.CurrentUser,
	})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo
}

// CC-3 + CC-1: another user's account is 404 to me — on a write, where a 403
// would be the "natural" answer and would confirm the account exists. The body
// is identical to the one for an account that never existed.
func TestHTTPAnotherUsersAccountIsNotFound(t *testing.T) {
	h, repo := newHTTP(t)
	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()
	acc, _ := repo.CreateAccount(ctx, CreateAccountInput{UserID: owner, Name: "Ví", Type: "cash", Currency: "VND"})

	rec := servertest.Do(t, h, stranger, http.MethodPatch, "/bank/accounts/"+acc.ID.String(), `{"name":"mine now"}`)
	servertest.Problem(t, rec, http.StatusNotFound, "bank/not-found")

	rec2 := servertest.Do(t, h, stranger, http.MethodPatch, "/bank/accounts/"+uuid.New().String(), `{"name":"x"}`)
	servertest.Problem(t, rec2, http.StatusNotFound, "bank/not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("foreign and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}

	// the owner's rename lands, and the stranger's did not
	if rec := servertest.Do(t, h, owner, http.MethodPatch, "/bank/accounts/"+acc.ID.String(), `{"name":"Ví chính"}`); rec.Code != http.StatusOK {
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

	if rec := servertest.Do(t, h, owner, http.MethodDelete, "/bank/accounts/"+acc.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	rec := servertest.Do(t, h, owner, http.MethodDelete, "/bank/accounts/"+acc.ID.String(), "")
	servertest.Problem(t, rec, http.StatusNotFound, "bank/not-found")
}
