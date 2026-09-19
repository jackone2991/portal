// Package servertest is what an HTTP-contract test needs to drive a module's
// real router the way a client does: identity injected the way cmd/api does
// it (a RequireAuth middleware plus a CurrentUser reader), a request helper,
// and an RFC 7807 assertion that knows which members shared/openapi.yaml
// marks required. Nine module test files carried private copies of these
// until 2026-09-19 (backlog 8a); the contract fact inside Problem — media
// type, `type`, `status`, `title` — has one owner now.
//
// Platform-only imports, so any module's tests may use it (depguard:
// platform never imports a module; modules may import platform).
package servertest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// UserHeader carries the caller's id into RequireAuth. Tests set it through
// Do; a request without it is anonymous, exactly as a missing session is.
const UserHeader = "X-Test-User"

type ctxUserKey struct{}

// withUser puts a caller on a context; RequireAuth is its one caller. A module
// that needs more on the context (journal stamps a tenant-scope marker) wraps
// RequireAuth and reads CurrentUser rather than reaching for this.
func withUser(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, ctxUserKey{}, id)
}

// CurrentUser reads what RequireAuth stored: the Deps.CurrentUser a module test
// injects.
func CurrentUser(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(ctxUserKey{}).(uuid.UUID)
	return id, ok
}

// RequireAuth is the Deps.RequireAuth a module test injects: the caller's id
// rides in UserHeader; a malformed or absent header leaves the request
// anonymous rather than failing it, so a handler's own 401 is still reachable.
// The permission guards stay nil — they belong to cmd/api.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if raw := r.Header.Get(UserHeader); raw != "" {
			if id, err := uuid.Parse(raw); err == nil {
				r = r.WithContext(withUser(r.Context(), id))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Do sends one request as `as`. A non-empty body is JSON.
func Do(t testing.TB, h http.Handler, as uuid.UUID, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(UserHeader, as.String())
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Problem decodes an RFC 7807 body and asserts the shape the contract fixes:
// the media type, and the three members shared/openapi.yaml marks required —
// `type` (module-scoped), `status` (equal to the HTTP status) and a non-empty
// `title`. It returns the decoded body for any further, module-specific check
// (a `detail` naming an id, say).
func Problem(t testing.TB, rec *httptest.ResponseRecorder, wantStatus int, wantType string) map[string]any {
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
