// Package api is the public surface of the tenant module (ADR-07 Phase 1).
// Other modules import ONLY this package to interact with the tenant domain.
package api

import (
	"context"

	"github.com/google/uuid"
)

// Organization is a small projection safe to share across modules.
type Organization struct {
	ID      uuid.UUID
	Kind    string // 'org' | 'household' | 'personal'
	Slug    string
	Name    string
	OwnerID uuid.UUID
}

// Store is the persistence the tenant API needs. The repository adapter
// (tenantrepo) implements it; wiring injects one concrete value. It is defined
// here (not in the module package) so api.Impl can depend on it without an
// import cycle.
type Store interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Organization, error)
	GetBySlug(ctx context.Context, slug string) (*Organization, error)
	PersonalOrg(ctx context.Context, userID uuid.UUID) (*Organization, error)
	GetOrCreatePersonalOrg(ctx context.Context, userID uuid.UUID, name string) (*Organization, error)
	ListForUser(ctx context.Context, userID uuid.UUID) ([]Organization, error)
	IsMember(ctx context.Context, userID, orgID uuid.UUID) (bool, error)
	// ListAllIDs returns every tenant. Used only by the worker's periodic sweeps,
	// which are cross-tenant by nature and iterate one scope at a time rather
	// than relying on a BYPASSRLS role (ADR-07 step 7 defers cmd/sysjobs).
	ListAllIDs(ctx context.Context) ([]uuid.UUID, error)
}

// API is what other modules import to interact with the tenant domain.
type API interface {
	// GetOrganization returns (nil, nil) if the org does not exist.
	GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error)
	// IsMember reports whether the user belongs to the org (fail-closed).
	IsMember(ctx context.Context, userID, orgID uuid.UUID) (bool, error)
	// PersonalOrg returns the user's personal org, or (nil, nil) if absent.
	PersonalOrg(ctx context.Context, userID uuid.UUID) (*Organization, error)
	// GetOrCreatePersonalOrg returns the user's personal org, creating it (+ an
	// owner membership) if absent. Used at first tenant resolution for a user
	// that registered after the 0018 backfill.
	GetOrCreatePersonalOrg(ctx context.Context, userID uuid.UUID, name string) (*Organization, error)
}

// Impl implements API over a Store.
type Impl struct{ store Store }

// NewImpl constructs the API implementation over a Store.
func NewImpl(store Store) *Impl { return &Impl{store: store} }

func (a *Impl) GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error) {
	return a.store.GetByID(ctx, id)
}

func (a *Impl) IsMember(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	ok, err := a.store.IsMember(ctx, userID, orgID)
	if err != nil {
		return false, nil // fail-closed
	}
	return ok, nil
}

func (a *Impl) PersonalOrg(ctx context.Context, userID uuid.UUID) (*Organization, error) {
	return a.store.PersonalOrg(ctx, userID)
}

func (a *Impl) GetOrCreatePersonalOrg(ctx context.Context, userID uuid.UUID, name string) (*Organization, error) {
	return a.store.GetOrCreatePersonalOrg(ctx, userID, name)
}
