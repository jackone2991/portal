package people

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

// httpPeople puts an owner-scoped in-memory store under the three person
// methods the contract needs; everything else (birthday notices, suggestions)
// is the birthday tests' stub, untouched.
type httpPeople struct {
	*fakePeople
	rows map[uuid.UUID]ownedPerson
}

type ownedPerson struct {
	owner uuid.UUID
	p     Person
}

func (r *httpPeople) CreatePerson(_ context.Context, in CreatePersonInput) (Person, error) {
	p := Person{ID: uuid.New(), DisplayName: in.DisplayName, Circle: CircleOther}
	r.rows[p.ID] = ownedPerson{owner: in.UserID, p: p}
	return p, nil
}

func (r *httpPeople) GetPerson(_ context.Context, userID, id uuid.UUID) (Person, error) {
	row, ok := r.rows[id]
	if !ok || row.owner != userID {
		return Person{}, ErrNotFound
	}
	return row.p, nil
}

func (r *httpPeople) DeletePerson(_ context.Context, userID, id uuid.UUID) error {
	row, ok := r.rows[id]
	if !ok || row.owner != userID {
		return ErrNotFound
	}
	delete(r.rows, id)
	return nil
}

func newHTTP(t *testing.T) (http.Handler, *httpPeople) {
	t.Helper()
	repo := &httpPeople{fakePeople: newFakePeople(), rows: map[uuid.UUID]ownedPerson{}}
	mod, err := New(Deps{Repo: repo, Timezone: "UTC", RequireAuth: servertest.RequireAuth, CurrentUser: servertest.CurrentUser})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo
}

func TestHTTPPersonIsNotFoundToAStranger(t *testing.T) {
	h, repo := newHTTP(t)
	owner, stranger := uuid.New(), uuid.New()
	p, err := repo.CreatePerson(context.Background(), CreatePersonInput{UserID: owner, DisplayName: "Mẹ"})
	if err != nil {
		t.Fatal(err)
	}
	rec := servertest.Do(t, h, stranger, http.MethodGet, "/people/"+p.ID.String(), "")
	servertest.Problem(t, rec, http.StatusNotFound, "people/person-not-found")
	rec2 := servertest.Do(t, h, stranger, http.MethodGet, "/people/"+uuid.New().String(), "")
	servertest.Problem(t, rec2, http.StatusNotFound, "people/person-not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("someone else's and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}
	// …and a stranger's DELETE is the same 404, and changes nothing.
	servertest.Problem(t, servertest.Do(t, h, stranger, http.MethodDelete, "/people/"+p.ID.String(), ""), http.StatusNotFound, "people/person-not-found")
	if rec := servertest.Do(t, h, owner, http.MethodGet, "/people/"+p.ID.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("owner GET after a stranger's DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHTTPDeleteTwiceIs404(t *testing.T) {
	h, repo := newHTTP(t)
	owner := uuid.New()
	p, err := repo.CreatePerson(context.Background(), CreatePersonInput{UserID: owner, DisplayName: "Gone"})
	if err != nil {
		t.Fatal(err)
	}
	if rec := servertest.Do(t, h, owner, http.MethodDelete, "/people/"+p.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodDelete, "/people/"+p.ID.String(), ""), http.StatusNotFound, "people/person-not-found")
}
