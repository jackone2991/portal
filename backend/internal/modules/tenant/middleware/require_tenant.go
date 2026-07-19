// Package middleware holds the tenant module's HTTP middleware. RequireTenant is
// the gatekeeper that opens the per-request tenant-scoped transaction (ADR-07).
package middleware

import (
	"context"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	tenantapi "github.com/portal/backend/internal/modules/tenant/api"
	platformdb "github.com/portal/backend/internal/platform/db"
)

// CurrentUser extracts the authenticated user id + display name from the request
// context. It is injected (not imported from account) so the tenant module never
// depends on account internals — the wiring layer builds it from auth.FromContext.
type CurrentUser func(ctx context.Context) (userID uuid.UUID, displayName string, ok bool)

// RequireTenant resolves the caller's active tenant and runs the request inside
// a transaction with app.current_tenant pinned. Increment 1: the active tenant
// is the caller's personal org (created on first resolution if absent). Explicit
// /t/{slug} routing lands in a later increment. MUST run AFTER RequireAuth.
//
// The whole handler runs in one transaction; on a 5xx (or panic) it rolls back,
// otherwise it commits. (RLS reads app.current_tenant in a later increment;
// today it is set but unenforced because the app connects as a superuser.)
func RequireTenant(db *platformdb.DB, store tenantapi.Store, currentUser CurrentUser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, displayName, ok := currentUser(r.Context())
			if !ok {
				writeErr(w, http.StatusUnauthorized, "unauthorized", "authentication required")
				return
			}
			org, err := store.GetOrCreatePersonalOrg(r.Context(), uid, displayName)
			if err != nil || org == nil {
				writeErr(w, http.StatusInternalServerError, "internal", "could not resolve tenant")
				return
			}
			tx, err := db.BeginTenantScope(r.Context(), org.ID)
			if err != nil {
				writeErr(w, http.StatusInternalServerError, "internal", "could not open tenant scope")
				return
			}
			ctx := platformdb.WithTx(r.Context(), tx)
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			committed := false
			finish := func(rollback bool) {
				if committed {
					return
				}
				committed = true
				if rollback {
					_ = tx.Rollback(ctx)
					return
				}
				_ = tx.Commit(ctx)
			}
			defer func() {
				if p := recover(); p != nil {
					finish(true)
					panic(p) // let the outer Recoverer handle it
				}
				finish(ww.Status() >= 500)
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))
		})
	}
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"code":"` + code + `","message":"` + msg + `"}`))
}
