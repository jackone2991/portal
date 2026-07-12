// Package notify owns the notification system (SPEC-04): the durable in-app
// store + read API, the delivery fan-out (one typed intent → in-app row + enabled
// channel tasks), per-type preferences, the email channel, and the first event
// consumer (media:asset_ready). It OWNS the notify:* Asynq task prefix
// (MODULES.md §5.2) — producers enqueue via notify/api, never sending mail/push
// themselves. The notification `type` string is an open registry (see README).
package notify

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	notifyapi "github.com/portal/backend/internal/modules/notify/api"
)

// Deps are the cross-cutting dependencies the notify module needs. cmd/api fills
// the HTTP side (Repo + auth/permission middleware + CurrentUser); cmd/worker
// fills the delivery side (Enqueuer + Email + Users + Redis). Both construct one.
type Deps struct {
	Repo     Repository
	Enqueuer Enqueuer     // schedules notify:email / notify:web_push (worker)
	Email    EmailSender  // email transport (worker)
	Users    UserResolver // recipient lookup for the email channel (worker)
	Redis    *redis.Client
	// EmailHourlyCap is the global per-hour send ceiling (budget insurance,
	// P0.3). 0 = uncapped.
	EmailHourlyCap int

	// HTTP side (api).
	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)
}

// Module is the runtime handle for the notify domain.
type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

// New constructs the module. Repo is the only hard requirement — the API server
// needs just the read API; the worker adds the delivery deps.
func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("notify: Repo is required")
	}
	svc := &Service{
		repo:           d.Repo,
		enqueue:        d.Enqueuer,
		email:          d.Email,
		users:          d.Users,
		redis:          d.Redis,
		emailHourlyCap: d.EmailHourlyCap,
	}
	return &Module{
		deps:    d,
		svc:     svc,
		handler: &Handler{svc: svc, currentUser: d.CurrentUser},
	}, nil
}

// MountHTTP wires the self-service bell routes (all under RequireAuth +
// RequirePermission):
//
//	GET  /me/notifications            list + unread_count       [notifications:read:own]
//	POST /me/notifications/read-all   mark all (≤ before) read  [notifications:write:own]
//	POST /me/notifications/{id}/read  idempotent mark-read      [notifications:write:own]
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/me/notifications", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("notifications:read:own")).Get("/", m.handler.List)
		r.With(m.perm("notifications:write:own")).Post("/read-all", m.handler.MarkAllRead)
		r.With(m.perm("notifications:write:own")).Post("/{id}/read", m.handler.MarkRead)
	})
}

// RegisterTasks attaches the notify:* handlers to the worker mux. notify OWNS
// the notify:* prefix (account's old stub is removed).
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(notifyapi.TaskDispatch, m.svc.Dispatch)
	mux.HandleFunc(notifyapi.TaskEmail, m.svc.SendEmail)
	mux.HandleFunc(notifyapi.TaskWebPush, m.svc.SendWebPush)
	mux.HandleFunc(notifyapi.TaskOnAssetReady, m.svc.OnAssetReady)
}

// PurgeOld is the notify:purge_old body (P2). cmd/worker registers it on the
// shared scheduler + light mux when the janitor lands.
func (m *Module) PurgeOld(ctx context.Context) error { return m.svc.PurgeOld(ctx) }

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}
