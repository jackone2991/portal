// Package middleware holds the tenant module's HTTP middleware. RequireTenant is
// the gatekeeper that opens the per-request tenant-scoped transaction (ADR-07).
package middleware

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/jackc/pgx/v5"
	tenantapi "github.com/portal/backend/internal/modules/tenant/api"

	platformdb "github.com/portal/backend/internal/platform/db"
	"github.com/portal/backend/internal/platform/server"
)

// CurrentUser extracts the authenticated user id + display name from the request
// context. It is injected (not imported from account) so the tenant module never
// depends on account internals — the wiring layer builds it from auth.FromContext.
type CurrentUser func(ctx context.Context) (userID uuid.UUID, displayName string, ok bool)

// TenantScoper opens a transaction with the actor pinned on it (tenant, user,
// admin flag — see platformdb.Scope).
//
// *platformdb.DB satisfies it structurally, so wiring is unchanged; the seam
// exists so the commit/rollback decisions below can be tested without a
// Postgres connection. Two adapters: the real pool, and the fake in
// require_tenant_test.go.
type TenantScoper interface {
	BeginScope(ctx context.Context, s platformdb.Scope) (pgx.Tx, error)
}

// maxBufferedResponse caps the body held back while a mutating request's
// transaction commits. Mutating handlers return small JSON documents; anything
// past this is a handler doing something this middleware was not designed for,
// so it degrades to streaming (and says so) rather than eating memory.
const maxBufferedResponse = 4 << 20 // 4 MiB

// RequireTenant resolves the caller's active tenant and runs the request inside
// a transaction with app.current_tenant pinned. Increment 1: the active tenant
// is the caller's personal org (created on first resolution if absent). Explicit
// /t/{slug} routing lands in a later increment. MUST run AFTER RequireAuth.
//
// The whole handler runs in one transaction; on a 5xx (or panic) it rolls back,
// otherwise it commits.
//
// # Why mutating requests are buffered
//
// The commit can only happen after the handler returns, but the handler has
// already written its status and body by then — so a COMMIT that fails used to
// be swallowed (`_ = tx.Commit`) and the client kept its 201 for a row that no
// longer existed. A deadlock, a deferred-constraint violation, or the
// host.docker.internal NAT dropping the connection all produce exactly that.
//
// For POST/PUT/PATCH/DELETE the response is therefore held in memory until the
// commit succeeds; if it fails, the buffered success is discarded and the client
// gets a 500. Safe methods stream straight through: a read-only transaction that
// fails to commit has lost nothing, so there is no reason to buffer a large
// download or an HLS segment behind it.
func RequireTenant(db TenantScoper, store tenantapi.Store, currentUser CurrentUser) func(http.Handler) http.Handler {
	return scoped(db, store, currentUser, true)
}

// OptionalTenant is RequireTenant for endpoints that serve anonymous callers
// too — today the media module's variant/HLS routes, which must keep serving
// assets marked public without a session.
//
// An anonymous request runs with NO transaction and therefore no GUCs, so the
// media policies (0032) reduce to "public rows only". That is the whole point:
// the route is not trusted to filter, the database is.
//
// A request that carries a valid identity gets the same tenant-scoped
// transaction RequireTenant opens, so the caller also sees their own private
// media. A request whose tenant cannot be resolved is served anonymously rather
// than failed — it degrades to public-only, which is the safe direction.
func OptionalTenant(db TenantScoper, store tenantapi.Store, currentUser CurrentUser) func(http.Handler) http.Handler {
	return scoped(db, store, currentUser, false)
}

func scoped(db TenantScoper, store tenantapi.Store, currentUser CurrentUser, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, displayName, ok := currentUser(r.Context())
			if !ok {
				if required {
					server.Unauthorized(w)
					return
				}
				next.ServeHTTP(w, r) // anonymous: public rows only
				return
			}
			org, err := store.GetOrCreatePersonalOrg(r.Context(), uid, displayName)
			if err != nil || org == nil {
				slog.Error("tenant: could not resolve personal org", "user_id", uid, "err", err)
				if !required {
					next.ServeHTTP(w, r) // degrade to public-only, never to a 500 on an image
					return
				}
				server.Problem(w, http.StatusInternalServerError, "about:blank",
					"Internal Server Error", "could not resolve tenant")
				return
			}
			// Admin = owner of the tenant. For a personal org that is always the
			// caller, so nothing changes for single-user tenants; in a shared org
			// it is what lets the owner administer members' media (0032).
			tx, err := db.BeginScope(r.Context(), platformdb.Scope{
				OrgID:  org.ID,
				UserID: uid,
				Admin:  org.OwnerID == uid,
			})
			if err != nil {
				slog.Error("tenant: could not open tenant scope", "user_id", uid, "org_id", org.ID, "err", err)
				server.Problem(w, http.StatusInternalServerError, "about:blank",
					"Internal Server Error", "could not open tenant scope")
				return
			}
			ctx := platformdb.WithTx(r.Context(), tx)

			if !mutating(r.Method) {
				serveStreaming(w, r, ctx, tx, next)
				return
			}
			serveBuffered(w, r, ctx, tx, next)
		})
	}
}

// mutating reports whether the method can write, and therefore whether a failed
// commit is data loss the client must hear about.
func mutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// serveStreaming is the safe-method path: the handler writes straight to the
// wire and the transaction is settled afterwards. A commit error here is logged
// but not surfaced — the transaction wrote nothing, so there is nothing to lose.
func serveStreaming(w http.ResponseWriter, r *http.Request, ctx context.Context, tx pgx.Tx, next http.Handler) {
	ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
	settled := false
	settle := func(rollback bool) {
		if settled {
			return
		}
		settled = true
		if rollback {
			if err := tx.Rollback(ctx); err != nil {
				slog.Error("tenant: rollback failed", "path", r.URL.Path, "err", err)
			}
			return
		}
		if err := tx.Commit(ctx); err != nil {
			slog.Error("tenant: read-only commit failed", "path", r.URL.Path, "err", err)
		}
	}
	defer func() {
		if p := recover(); p != nil {
			settle(true)
			panic(p) // let the outer Recoverer handle it
		}
		settle(ww.Status() >= 500)
	}()
	next.ServeHTTP(ww, r.WithContext(ctx))
}

// serveBuffered is the mutating path: the handler's response is captured, the
// transaction is committed, and only then is the response released. A failed
// commit replaces it with a 500, so the client is never told a write succeeded
// that did not.
func serveBuffered(w http.ResponseWriter, r *http.Request, ctx context.Context, tx pgx.Tx, next http.Handler) {
	buf := &bufferedWriter{header: http.Header{}, real: w}

	var panicked any
	func() {
		defer func() {
			if p := recover(); p != nil {
				panicked = p
			}
		}()
		next.ServeHTTP(buf, r.WithContext(ctx))
	}()

	if panicked != nil {
		if err := tx.Rollback(ctx); err != nil {
			slog.Error("tenant: rollback after panic failed", "path", r.URL.Path, "err", err)
		}
		panic(panicked) // nothing was flushed; the outer Recoverer writes the 500
	}

	// The handler already failed; roll back and pass its own error response on.
	if buf.status >= 500 {
		if err := tx.Rollback(ctx); err != nil {
			slog.Error("tenant: rollback failed", "path", r.URL.Path, "err", err)
		}
		buf.flush()
		return
	}

	// A handler that outgrew the buffer has already streamed. It cannot be
	// un-sent, so this degrades to the old behaviour — logged, not silent.
	if buf.overflowed {
		if err := tx.Commit(ctx); err != nil {
			slog.Error("tenant: commit failed AFTER an oversized response was streamed — the client was told this write succeeded",
				"path", r.URL.Path, "method", r.Method, "err", err)
		}
		return
	}

	if err := tx.Commit(ctx); err != nil {
		slog.Error("tenant: commit failed, discarding the handler's success response",
			"path", r.URL.Path, "method", r.Method, "status", buf.statusOr200(), "err", err)
		server.Problem(w, http.StatusInternalServerError, "about:blank",
			"Internal Server Error", "the change could not be committed")
		return
	}
	buf.flush()
}

// bufferedWriter holds a handler's response until the caller decides whether it
// may be sent. Past maxBufferedResponse it gives up and streams — see the
// overflowed branch in serveBuffered.
type bufferedWriter struct {
	header      http.Header
	body        bytes.Buffer
	status      int
	real        http.ResponseWriter
	overflowed  bool
	wroteHeader bool
}

func (b *bufferedWriter) Header() http.Header { return b.header }

func (b *bufferedWriter) WriteHeader(status int) {
	if b.wroteHeader {
		return
	}
	b.wroteHeader = true
	b.status = status
	if b.overflowed {
		b.real.WriteHeader(status)
	}
}

func (b *bufferedWriter) Write(p []byte) (int, error) {
	if !b.wroteHeader {
		b.WriteHeader(http.StatusOK)
	}
	if b.overflowed {
		return b.real.Write(p)
	}
	if b.body.Len()+len(p) > maxBufferedResponse {
		b.spill()
		return b.real.Write(p)
	}
	return b.body.Write(p)
}

// spill copies everything captured so far to the real writer and switches to
// pass-through. Once this happens the response is irrevocably on the wire.
func (b *bufferedWriter) spill() {
	b.overflowed = true
	copyHeader(b.real.Header(), b.header)
	b.real.WriteHeader(b.statusOr200())
	_, _ = b.real.Write(b.body.Bytes())
	b.body.Reset()
}

// flush releases the captured response. A handler that wrote nothing at all
// yields a bare 200, matching net/http's own default.
func (b *bufferedWriter) flush() {
	if b.overflowed {
		return
	}
	copyHeader(b.real.Header(), b.header)
	b.real.WriteHeader(b.statusOr200())
	_, _ = b.real.Write(b.body.Bytes())
}

func (b *bufferedWriter) statusOr200() int {
	if b.status == 0 {
		return http.StatusOK
	}
	return b.status
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		dst[k] = append([]string(nil), vv...)
	}
}
