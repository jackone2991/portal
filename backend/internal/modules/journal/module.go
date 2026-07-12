// Package journal owns human-authored life-stream entries (SPEC-05): owner-scoped
// entry CRUD plus the emit-only journal:entry_created event. It also owns SPEC-06's
// stream_items projection (a later migration); at v1 only entries exist. Other
// modules import only journal/api — the service/handler/repository stay private.
package journal

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	journalapi "github.com/portal/backend/internal/modules/journal/api"
)

// Deps are the journal module's dependencies. cmd/api fills the HTTP side (Repo,
// auth/permission middleware, CurrentUser, and the Events publisher for the
// emit-only journal:entry_created); cmd/worker constructs it with just Repo (P0
// registers no tasks — the wiring exists for SPEC-06's future consumers).
type Deps struct {
	Repo   Repository
	Events EventPublisher // optional: journal:entry_created emit (nil → no-op)

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)
}

// Module is the runtime handle for the journal domain.
type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

// New constructs the module. Repo is the only hard requirement.
func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("journal: Repo is required")
	}
	svc := &Service{repo: d.Repo, events: d.Events}
	return &Module{
		deps:    d,
		svc:     svc,
		handler: &Handler{svc: svc, currentUser: d.CurrentUser},
	}, nil
}

// MountHTTP wires the entry routes (all under RequireAuth + RequirePermission):
//
//	POST   /journal/entries        create             [journal:write:own]
//	GET    /journal/entries        list (cursor)      [journal:read:own]
//	GET    /journal/entries/{id}   fetch              [journal:read:own]
//	PATCH  /journal/entries/{id}   partial update     [journal:write:own]
//	DELETE /journal/entries/{id}   idempotent delete  [journal:delete:own]
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/journal/entries", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("journal:write:own")).Post("/", m.handler.Create)
		r.With(m.perm("journal:read:own")).Get("/", m.handler.List)
		r.With(m.perm("journal:read:own")).Get("/{id}", m.handler.Get)
		r.With(m.perm("journal:write:own")).Patch("/{id}", m.handler.Patch)
		r.With(m.perm("journal:delete:own")).Delete("/{id}", m.handler.Delete)
	})
	// Life-stream merged read (SPEC-06 P0.2). Mounted at /stream, not /journal.
	r.Route("/stream", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("stream:read:own")).Get("/", m.handler.Stream)
	})
}

// RegisterTasks registers the life-stream consumers (SPEC-06 P0.1b). cmd/worker
// subscribes each source event to the matching task on the shared publisher.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	reg := func(task string, fn func(context.Context, []byte) error) {
		mux.HandleFunc(task, func(ctx context.Context, t *asynq.Task) error { return fn(ctx, t.Payload()) })
	}
	reg(journalapi.TaskStreamAssetReady, m.svc.OnAssetReady)
	reg(journalapi.TaskStreamPlaybackCompleted, m.svc.OnPlaybackCompleted)
	reg(journalapi.TaskStreamAssetDeleted, m.svc.OnAssetDeleted)
	reg(journalapi.TaskStreamBankCreated, m.svc.OnBankCreated)
	reg(journalapi.TaskStreamBankUpdated, m.svc.OnBankUpdated)
	reg(journalapi.TaskStreamBankDeleted, m.svc.OnBankDeleted)
	reg(journalapi.TaskStreamBirthday, m.svc.OnBirthdayUpcoming)
	reg(journalapi.TaskStreamComicPublished, m.svc.OnComicPublished)
}

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}
