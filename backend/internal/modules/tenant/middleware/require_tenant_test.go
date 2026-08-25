package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	tenantapi "github.com/portal/backend/internal/modules/tenant/api"
)

// ══ fakes ═══════════════════════════════════════════════════════════════════

// fakeTx embeds a nil pgx.Tx: the middleware only ever calls Commit and
// Rollback, so any other method reaching this fake panics loudly instead of
// silently returning a zero value.
type fakeTx struct {
	pgx.Tx
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

func (t *fakeTx) Commit(context.Context) error {
	t.committed = true
	return t.commitErr
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rolledBack = true
	return t.rollbackErr
}

type fakeScoper struct {
	tx       *fakeTx
	beginErr error
	orgSeen  uuid.UUID
}

func (s *fakeScoper) BeginTenantScope(_ context.Context, orgID uuid.UUID) (pgx.Tx, error) {
	s.orgSeen = orgID
	if s.beginErr != nil {
		return nil, s.beginErr
	}
	return s.tx, nil
}

type fakeStore struct {
	tenantapi.Store
	org *tenantapi.Organization
	err error
}

func (s *fakeStore) GetOrCreatePersonalOrg(context.Context, uuid.UUID, string) (*tenantapi.Organization, error) {
	return s.org, s.err
}

// ══ harness ═════════════════════════════════════════════════════════════════

var (
	testUser = uuid.MustParse("11111111-1111-4111-8111-111111111111")
	testOrg  = uuid.MustParse("22222222-2222-4222-8222-222222222222")
)

func authedUser(context.Context) (uuid.UUID, string, bool) { return testUser, "Ada", true }
func anonUser(context.Context) (uuid.UUID, string, bool)   { return uuid.Nil, "", false }

// serve runs one request through RequireTenant wrapping handler.
func serve(t *testing.T, method string, tx *fakeTx, user CurrentUser, handler http.HandlerFunc) (*httptest.ResponseRecorder, *fakeScoper) {
	t.Helper()
	scoper := &fakeScoper{tx: tx}
	store := &fakeStore{org: &tenantapi.Organization{ID: testOrg, Kind: "personal"}}
	mw := RequireTenant(scoper, store, user)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/api/v1/comics", strings.NewReader(`{}`))
	mw(handler).ServeHTTP(rec, req)
	return rec, scoper
}

func problemBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not JSON: %v (%q)", err, rec.Body.String())
	}
	return body
}

// ══ the bug this middleware exists to prevent ═══════════════════════════════

// A COMMIT that fails after a mutating handler has produced its 201 must not
// reach the client as a success. This is the regression test for the
// `_ = tx.Commit(ctx)` that returned 201 for a row that was never written.
func TestMutatingRequestCommitFailureBecomes500(t *testing.T) {
	tx := &fakeTx{commitErr: errors.New("could not commit: connection reset")}

	rec, _ := serve(t, http.MethodPost, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"created"}`)
	})

	if !tx.committed {
		t.Fatal("commit was never attempted")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 — a failed commit was reported as success", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "created") {
		t.Fatalf("the handler's success body leaked through a failed commit: %q", rec.Body.String())
	}
	body := problemBody(t, rec)
	if got := body["status"]; got != float64(500) {
		t.Errorf("problem.status = %v, want 500", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
}

func TestMutatingRequestCommitSuccessPassesResponseThrough(t *testing.T) {
	tx := &fakeTx{}

	rec, scoper := serve(t, http.MethodPost, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "kept")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":"abc"}`)
	})

	if !tx.committed || tx.rolledBack {
		t.Fatalf("committed=%v rolledBack=%v, want commit only", tx.committed, tx.rolledBack)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201", rec.Code)
	}
	if got := strings.TrimSpace(rec.Body.String()); got != `{"id":"abc"}` {
		t.Errorf("body = %q, want the handler's body verbatim", got)
	}
	if got := rec.Header().Get("X-Custom"); got != "kept" {
		t.Errorf("X-Custom = %q — buffering dropped a handler header", got)
	}
	if scoper.orgSeen != testOrg {
		t.Errorf("tenant scope opened for %v, want the caller's personal org %v", scoper.orgSeen, testOrg)
	}
}

// A handler that fails on its own must roll back, and its error response must
// survive — the middleware does not replace a 500 the handler chose.
func TestMutatingRequestHandlerErrorRollsBack(t *testing.T) {
	tx := &fakeTx{}

	rec, _ := serve(t, http.MethodPost, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"detail":"handler blew up"}`)
	})

	if !tx.rolledBack || tx.committed {
		t.Fatalf("committed=%v rolledBack=%v, want rollback only", tx.committed, tx.rolledBack)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want the handler's 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "handler blew up") {
		t.Errorf("the handler's own error body was replaced: %q", rec.Body.String())
	}
}

// A 4xx is a deliberate answer, not a failure: it commits, because the handler
// may legitimately have written an audit row before rejecting.
func TestMutatingRequestClientErrorStillCommits(t *testing.T) {
	tx := &fakeTx{}

	rec, _ := serve(t, http.MethodPost, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
	})

	if !tx.committed || tx.rolledBack {
		t.Fatalf("committed=%v rolledBack=%v, want commit on a 4xx", tx.committed, tx.rolledBack)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}

// A panic must roll back and keep propagating so the outer Recoverer answers.
// Nothing may have been flushed, or the Recoverer's 500 would append to a body.
func TestMutatingRequestPanicRollsBackAndRepanics(t *testing.T) {
	tx := &fakeTx{}
	scoper := &fakeScoper{tx: tx}
	store := &fakeStore{org: &tenantapi.Organization{ID: testOrg}}
	mw := RequireTenant(scoper, store, authedUser)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comics", nil)

	defer func() {
		if recover() == nil {
			t.Fatal("panic did not propagate to the outer Recoverer")
		}
		if !tx.rolledBack {
			t.Error("panic did not roll the transaction back")
		}
		if rec.Body.Len() != 0 {
			t.Errorf("a partial body was flushed before the panic: %q", rec.Body.String())
		}
	}()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"partial":`)
		panic("boom")
	})).ServeHTTP(rec, req)
}

// ══ safe methods stream ═════════════════════════════════════════════════════

// GET must not be buffered: a large download or an HLS segment would otherwise
// sit in memory waiting on a commit that cannot lose anything.
func TestSafeMethodStreamsAndCommits(t *testing.T) {
	tx := &fakeTx{}

	rec, _ := serve(t, http.MethodGet, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"items":[]}`)
	})

	if !tx.committed {
		t.Error("read-only transaction was not committed")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

// A commit failure on a read-only request is logged, not surfaced: the response
// is already correct and the transaction wrote nothing.
func TestSafeMethodCommitFailureDoesNotAlterResponse(t *testing.T) {
	tx := &fakeTx{commitErr: errors.New("connection reset")}

	rec, _ := serve(t, http.MethodGet, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"items":[]}`)
	})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — a read-only commit failure is not the client's problem", rec.Code)
	}
}

func TestSafeMethodServerErrorRollsBack(t *testing.T) {
	tx := &fakeTx{}

	_, _ = serve(t, http.MethodGet, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})

	if !tx.rolledBack || tx.committed {
		t.Fatalf("committed=%v rolledBack=%v, want rollback on a 5xx", tx.committed, tx.rolledBack)
	}
}

// ══ gate failures never open a transaction ══════════════════════════════════

func TestUnauthenticatedRequestNeverOpensAScope(t *testing.T) {
	scoper := &fakeScoper{tx: &fakeTx{}}
	store := &fakeStore{org: &tenantapi.Organization{ID: testOrg}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comics", nil)
	called := false
	RequireTenant(scoper, store, anonUser)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rec, req)

	if called {
		t.Error("the handler ran for an unauthenticated request")
	}
	if scoper.orgSeen != uuid.Nil {
		t.Error("a tenant scope was opened for an unauthenticated request")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want RFC 7807 — the legacy {code,message} shape is retired", ct)
	}
	if body := problemBody(t, rec); body["detail"] != "authentication required" {
		t.Errorf("problem.detail = %v", body["detail"])
	}
}

func TestUnresolvableTenantIs500AndRunsNoHandler(t *testing.T) {
	scoper := &fakeScoper{tx: &fakeTx{}}
	store := &fakeStore{err: errors.New("db down")}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/comics", nil)
	called := false
	RequireTenant(scoper, store, authedUser)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	})).ServeHTTP(rec, req)

	if called {
		t.Error("the handler ran without a resolved tenant")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestBeginScopeFailureIs500(t *testing.T) {
	scoper := &fakeScoper{beginErr: errors.New("pool exhausted")}
	store := &fakeStore{org: &tenantapi.Organization{ID: testOrg}}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/comics", nil)
	RequireTenant(scoper, store, authedUser)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("the handler ran without a transaction")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// ══ the overflow escape hatch ═══════════════════════════════════════════════

// Past maxBufferedResponse the buffer gives up and streams. The response is then
// irrevocable, so this asserts the degradation is complete and correct rather
// than truncating the body.
func TestOversizedMutatingResponseStreamsIntact(t *testing.T) {
	tx := &fakeTx{}
	big := strings.Repeat("x", maxBufferedResponse+1024)

	rec, _ := serve(t, http.MethodPost, tx, authedUser, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, big)
	})

	if !tx.committed {
		t.Error("an oversized response skipped the commit")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.Len() != len(big) {
		t.Errorf("body length = %d, want %d — spilling truncated the response", rec.Body.Len(), len(big))
	}
}
