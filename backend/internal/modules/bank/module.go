// Package bank owns the personal ledger (SPEC-03): owner-scoped accounts,
// hierarchical categories, transactions, inter-account transfers (paired legs),
// and monthly budgets — money as integer minor units (D-41). It emits the
// bank:transaction_* event family after each write commits (P0.7). Other modules
// import only bank/api; the service/handler/repository stay private.
package bank

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Deps are the bank module's dependencies. cmd/api fills the HTTP side (Repo,
// auth/permission middleware, CurrentUser, and the Events publisher for the
// emit-only bank:transaction_* family); cmd/worker constructs it with just Repo
// (P0 registers no tasks — the wiring exists for SPEC-06's future consumers).
type Deps struct {
	Repo   Repository
	Events EventPublisher // optional: bank:transaction_* (nil → no-op)

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)
}

// Module is the runtime handle for the bank domain.
type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

// New constructs the module. Repo is the only hard requirement.
func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("bank: Repo is required")
	}
	svc := &Service{repo: d.Repo, events: d.Events}
	return &Module{
		deps:    d,
		svc:     svc,
		handler: &Handler{svc: svc, currentUser: d.CurrentUser},
	}, nil
}

// MountHTTP wires the ledger routes (all under RequireAuth + RequirePermission).
// Transfers are transaction writes, so they carry bank-transactions:* codes; the
// dashboard reads accounts (bank-accounts:read:own).
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/bank", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}

		r.Route("/accounts", func(r chi.Router) {
			r.With(m.perm("bank-accounts:read:own")).Get("/", m.handler.ListAccounts)
			r.With(m.perm("bank-accounts:write:own")).Post("/", m.handler.CreateAccount)
			r.With(m.perm("bank-accounts:write:own")).Patch("/{id}", m.handler.UpdateAccount)
			r.With(m.perm("bank-accounts:delete:own")).Delete("/{id}", m.handler.DeleteAccount)
		})

		r.Route("/categories", func(r chi.Router) {
			r.With(m.perm("bank-categories:read:own")).Get("/", m.handler.ListCategories)
			r.With(m.perm("bank-categories:write:own")).Post("/", m.handler.CreateCategory)
			r.With(m.perm("bank-categories:write:own")).Patch("/{id}", m.handler.UpdateCategory)
			r.With(m.perm("bank-categories:delete:own")).Delete("/{id}", m.handler.DeleteCategory)
		})

		r.Route("/transactions", func(r chi.Router) {
			r.With(m.perm("bank-transactions:read:own")).Get("/", m.handler.ListTransactions)
			r.With(m.perm("bank-transactions:write:own")).Post("/", m.handler.CreateTransaction)
			r.With(m.perm("bank-transactions:write:own")).Patch("/{id}", m.handler.UpdateTransaction)
			r.With(m.perm("bank-transactions:delete:own")).Delete("/{id}", m.handler.DeleteTransaction)
		})

		r.Route("/transfers", func(r chi.Router) {
			r.With(m.perm("bank-transactions:write:own")).Post("/", m.handler.CreateTransfer)
			r.With(m.perm("bank-transactions:write:own")).Patch("/{transfer_id}", m.handler.UpdateTransfer)
			r.With(m.perm("bank-transactions:delete:own")).Delete("/{transfer_id}", m.handler.DeleteTransfer)
		})

		r.Route("/budgets", func(r chi.Router) {
			r.With(m.perm("bank-budgets:read:own")).Get("/", m.handler.ListBudgets)
			r.With(m.perm("bank-budgets:write:own")).Put("/", m.handler.SetBudget)
		})

		r.With(m.perm("bank-accounts:read:own")).Get("/dashboard", m.handler.Dashboard)
	})
}

// RegisterTasks registers no worker tasks at P0. The wiring exists so SPEC-06's
// stream consumer of bank:transaction_* can attach without touching cmd/worker.
func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return m.deps.RequirePermission(code)
}
