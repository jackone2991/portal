// Package ops owns platform operations (SPEC-09 P0): the self-running nightly
// Postgres backup task (pg_dump → off-VPS storage, with retention + a LATEST.json
// manifest), the restore-drill runbook, and the freshness sentinel behind
// GET /ops/status. Its tables are system-scoped (no user_id) — backups are one
// ledger for the whole install. Other modules import only ops/api; the
// service/handler/repository internals stay private. It owns the ops:* prefix.
package ops

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	opsapi "github.com/portal/backend/internal/modules/ops/api"
	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/storage"
)

// Deps are the ops module's dependencies. cmd/api fills the HTTP side (Repo +
// auth/permission middleware — the sentinel needs nothing else); cmd/worker fills
// the backup side (Store + Events + Audit + BackupDatabaseURL). Both construct one.
type Deps struct {
	Repo Repository

	// Worker side (nil on the API server).
	Store  storage.Storage // backup upload target (MinIO dev / R2 prod)
	Events EventPublisher  // ops:backup_completed / :backup_failed fan-out (optional)
	Audit  *audit.Logger   // ops.backup.completed / .failed audit (optional)
	// BackupDatabaseURL is the pg_dump DSN. It MUST connect to Postgres directly
	// (postgres:5432), NOT through PgBouncer — a transaction pooler breaks the
	// session semantics pg_dump relies on (SPEC-09 P0.2).
	BackupDatabaseURL string

	// HTTP side (api).
	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
}

// Module is the runtime handle for the ops domain.
type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

// New constructs the module. Repo is the only hard requirement — the API server
// needs just the read side; the worker adds the backup deps.
func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("ops: Repo is required")
	}
	svc := &Service{repo: d.Repo}
	return &Module{
		deps:    d,
		svc:     svc,
		handler: &Handler{svc: svc},
	}, nil
}

// MountHTTP wires the freshness sentinel (admin-tier: RequireAuth + ops:read):
//
//	GET /ops/status   backup freshness {last_success_at, hours_since_success, state, last_run}  [ops:read]
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/ops", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("ops:read")).Get("/status", m.handler.Status)
	})
}

// RegisterTasks attaches the ops:backup_database handler to the worker mux. The
// nightly SCHEDULE is registered on the shared periodic scheduler in cmd/worker
// (there is no OS cron in this stack — SPEC-01 P0.3 convention).
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(opsapi.TaskBackupDatabase, func(ctx context.Context, _ *asynq.Task) error {
		return m.BackupDatabase(ctx)
	})
}

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}
