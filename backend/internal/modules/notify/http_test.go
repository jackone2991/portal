package notify

// HTTP-contract tests: the module's public surface is its router, and these
// drive it the way a client does — a request in, a status and a body out —
// over the real handler and service with fakes underneath (the comic/bank
// pattern, 2026-09-11; backlog P1 #8). Two promises every module keeps:
//
//   - a resource that is not yours answers 404, never 403 — the same body as
//     a missing id, so the endpoint never confirms that it exists;
//   - the one write, mark-read, is idempotent: a second read of the same
//     notification is the same 200, not a 404 — the only "twice" this module has.

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
	repo := newFakeRepo()
	mod, err := New(Deps{Repo: repo, RequireAuth: servertest.RequireAuth, CurrentUser: servertest.CurrentUser})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	mod.MountHTTP(r)
	return r, repo
}

func seedNotification(t *testing.T, repo *fakeRepo, user uuid.UUID) uuid.UUID {
	t.Helper()
	_, n, err := repo.InsertNotification(context.Background(), InsertNotificationInput{UserID: user, Type: "test.ping", Title: "ping"})
	if err != nil {
		t.Fatal(err)
	}
	return n.ID
}

func TestHTTPNotificationIsNotFoundToAStranger(t *testing.T) {
	h, repo := newHTTP(t)
	owner, stranger := uuid.New(), uuid.New()
	id := seedNotification(t, repo, owner)
	rec := servertest.Do(t, h, stranger, http.MethodPost, "/me/notifications/"+id.String()+"/read", "")
	servertest.Problem(t, rec, http.StatusNotFound, "notify/notification-not-found")
	rec2 := servertest.Do(t, h, stranger, http.MethodPost, "/me/notifications/"+uuid.New().String()+"/read", "")
	servertest.Problem(t, rec2, http.StatusNotFound, "notify/notification-not-found")
	if rec.Body.String() != rec2.Body.String() {
		t.Fatalf("someone else's and missing must answer identically:\n%s\n%s", rec.Body.String(), rec2.Body.String())
	}
}

func TestHTTPMarkReadTwiceIsIdempotent(t *testing.T) {
	h, repo := newHTTP(t)
	owner := uuid.New()
	id := seedNotification(t, repo, owner)
	first := servertest.Do(t, h, owner, http.MethodPost, "/me/notifications/"+id.String()+"/read", "")
	second := servertest.Do(t, h, owner, http.MethodPost, "/me/notifications/"+id.String()+"/read", "")
	if first.Code != http.StatusOK || second.Code != http.StatusOK {
		t.Fatalf("mark-read twice = %d then %d, want 200 and 200 (%s / %s)", first.Code, second.Code, first.Body.String(), second.Body.String())
	}
	if first.Body.String() != second.Body.String() {
		t.Fatalf("second read answered differently:\n%s\n%s", first.Body.String(), second.Body.String())
	}
}
