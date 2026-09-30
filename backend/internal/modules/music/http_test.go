package music

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
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server/servertest"
)

func newHTTP(t *testing.T) (http.Handler, *fakeRepo) {
	t.Helper()
	_, repo, media, events := newSvc()
	mod, err := New(Deps{Repo: repo, Media: media, Events: events, RequireAuth: servertest.RequireAuth, CurrentUser: servertest.CurrentUser})
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
	row, err := repo.CreateTrack(context.Background(), CreateTrackInput{OwnerID: owner, Title: "Draft"})
	if err != nil {
		t.Fatal(err)
	}
	rec := servertest.Do(t, h, stranger, http.MethodGet, "/tracks/"+row.ID.String(), "")
	servertest.Problem(t, rec, http.StatusNotFound, "music/not-found")
	rec2 := servertest.Do(t, h, stranger, http.MethodGet, "/tracks/"+uuid.New().String(), "")
	servertest.Problem(t, rec2, http.StatusNotFound, "music/not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("someone else's and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}
	if rec := servertest.Do(t, h, owner, http.MethodGet, "/tracks/"+row.ID.String(), ""); rec.Code != http.StatusOK {
		t.Fatalf("owner GET = %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestHTTPDeleteTwiceIs404(t *testing.T) {
	h, repo := newHTTP(t)
	owner := uuid.New()
	row, err := repo.CreateTrack(context.Background(), CreateTrackInput{OwnerID: owner, Title: "Gone"})
	if err != nil {
		t.Fatal(err)
	}
	if rec := servertest.Do(t, h, owner, http.MethodDelete, "/tracks/"+row.ID.String(), ""); rec.Code != http.StatusNoContent {
		t.Fatalf("first DELETE = %d (%s)", rec.Code, rec.Body.String())
	}
	servertest.Problem(t, servertest.Do(t, h, owner, http.MethodDelete, "/tracks/"+row.ID.String(), ""), http.StatusNotFound, "music/not-found")
}
