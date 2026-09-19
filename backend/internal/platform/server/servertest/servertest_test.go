package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server"
)

// The helpers are themselves a contract nine test files rely on; pin them.

func TestRequireAuthCarriesTheHeaderUser(t *testing.T) {
	var seen uuid.UUID
	var ok bool
	h := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, ok = CurrentUser(r.Context())
	}))
	me := uuid.New()
	Do(t, h, me, http.MethodGet, "/", "")
	if !ok || seen != me {
		t.Fatalf("handler saw %v (ok=%v), want %s", seen, ok, me)
	}

	// No header, or a header that is not a uuid: anonymous, not a failure.
	for _, raw := range []string{"", "not-a-uuid"} {
		ok = true
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if raw != "" {
			req.Header.Set(UserHeader, raw)
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		if ok {
			t.Fatalf("header %q: CurrentUser ok = true, want anonymous", raw)
		}
	}
}

func TestDoSendsJSONOnlyWithABody(t *testing.T) {
	var ct string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { ct = r.Header.Get("Content-Type") })
	Do(t, h, uuid.New(), http.MethodPost, "/", `{"a":1}`)
	if ct != "application/json" {
		t.Fatalf("Content-Type with a body = %q", ct)
	}
	Do(t, h, uuid.New(), http.MethodDelete, "/", "")
	if ct != "" {
		t.Fatalf("Content-Type without a body = %q, want none", ct)
	}
}

// recorder is a testing.TB whose Fatal panics with itself instead of stopping
// the test, so Problem's refusals can be asserted (the only faithful stand-in
// for Fatalf's Goexit).
type recorder struct{ testing.TB }

func (r *recorder) Helper()                           {}
func (r *recorder) Fatalf(format string, args ...any) { panic(r) }
func (r *recorder) Fatal(args ...any)                 { panic(r) }

func refuses(t *testing.T, rec *httptest.ResponseRecorder, status int, typ string) bool {
	t.Helper()
	r := &recorder{TB: t}
	defused := func() (refused bool) {
		defer func() {
			if x := recover(); x != nil {
				if x != r {
					panic(x)
				}
				refused = true
			}
		}()
		Problem(r, rec, status, typ)
		return false
	}
	return defused()
}

func TestProblemAcceptsTheRealWriterAndRefusesTheRest(t *testing.T) {
	rec := httptest.NewRecorder()
	server.Problem(rec, http.StatusNotFound, "journal/entry-not-found", "Not Found", "journal entry not found")
	p := Problem(t, rec, http.StatusNotFound, "journal/entry-not-found")
	if p["detail"] != "journal entry not found" {
		t.Fatalf("decoded body = %v", p)
	}

	if !refuses(t, rec, http.StatusNotFound, "journal/other") {
		t.Fatal("a wrong type was accepted")
	}
	if !refuses(t, rec, http.StatusForbidden, "journal/entry-not-found") {
		t.Fatal("a wrong status was accepted")
	}
	plain := httptest.NewRecorder()
	http.Error(plain, "not found", http.StatusNotFound)
	if !refuses(t, plain, http.StatusNotFound, "journal/entry-not-found") {
		t.Fatal("a text/plain 404 was accepted as a problem")
	}

	// The two members only a hand-written body can get wrong: a `status` that
	// disagrees with the HTTP status, and an empty `title`.
	for name, body := range map[string]string{
		"status member disagrees": `{"type":"journal/entry-not-found","status":500,"title":"Not Found"}`,
		"empty title":             `{"type":"journal/entry-not-found","status":404,"title":""}`,
		"no title":                `{"type":"journal/entry-not-found","status":404}`,
	} {
		hand := httptest.NewRecorder()
		hand.Header().Set("Content-Type", "application/problem+json")
		hand.WriteHeader(http.StatusNotFound)
		hand.Body.WriteString(body)
		if !refuses(t, hand, http.StatusNotFound, "journal/entry-not-found") {
			t.Fatalf("%s: accepted %s", name, body)
		}
	}
}
