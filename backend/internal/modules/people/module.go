// Package people owns an owner-scoped address book (SPEC-08): contacts +
// birthdays. A daily people:scan_birthdays task emits people:birthday_upcoming
// (T ∈ {3,0}) once per (person, threshold, year) via an outbox/dedup table.
package people

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	peopleapi "github.com/portal/backend/internal/modules/people/api"
)

// Deps are the people module's dependencies. Timezone is the instance-default
// TZ for the scan + upcoming math (D-17 stores a per-user TZ later); empty ⇒
// Asia/Ho_Chi_Minh, then UTC if that fails to load.
type Deps struct {
	Repo     Repository
	Events   EventPublisher
	Timezone string

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)
}

type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("people: Repo is required")
	}
	tz := d.Timezone
	if tz == "" {
		tz = "Asia/Ho_Chi_Minh"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Warn().Str("tz", tz).Msg("people: timezone load failed, using UTC")
		loc = time.UTC
	}
	svc := &Service{repo: d.Repo, events: d.Events, loc: loc}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, currentUser: d.CurrentUser}}, nil
}

func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/people", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("people:read:own")).Get("/", m.handler.List)
		r.With(m.perm("people:write:own")).Post("/", m.handler.Create)
		r.With(m.perm("people:read:own")).Get("/upcoming-birthdays", m.handler.Upcoming)
		r.With(m.perm("people:read:own")).Get("/{id}", m.handler.Get)
		r.With(m.perm("people:write:own")).Patch("/{id}", m.handler.Patch)
		r.With(m.perm("people:delete:own")).Delete("/{id}", m.handler.Delete)
	})
}

// RegisterTasks registers the daily birthday scan (P0.4). cmd/worker also adds
// the schedule (people:scan_birthdays) to the shared scheduler.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(peopleapi.TaskScanBirthdays, func(ctx context.Context, _ *asynq.Task) error {
		return m.svc.ScanBirthdays(ctx, time.Now())
	})
}

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}
