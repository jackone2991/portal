// Package api is the public surface of the tenant module.
package api

import (
	"context"

	"github.com/google/uuid"
)

// Organization is a small projection safe to share across modules.
type Organization struct {
	ID     uuid.UUID
	Code   string
	Name   string
	Tier   string
	Active bool
}

// API is what other modules import to interact with the tenant domain.
type API interface {
	// GetOrganization returns (nil, nil) if the org does not exist or is
	// suspended. Callers MUST handle the nil case explicitly.
	GetOrganization(ctx context.Context, id uuid.UUID) (*Organization, error)

	// IsMember reports whether the user belongs to the org. Returns
	// false on any error (fail-closed).
	IsMember(ctx context.Context, userID, orgID uuid.UUID) (bool, error)
}

// Impl is the concrete implementation. NewImpl is internal to wiring.
type Impl struct{}

func NewImpl() *Impl { return &Impl{} }

func (a *Impl) GetOrganization(_ context.Context, _ uuid.UUID) (*Organization, error) {
	return nil, nil // placeholder until 0004_tenant_organizations migration lands
}

func (a *Impl) IsMember(_ context.Context, _, _ uuid.UUID) (bool, error) {
	return false, nil
}
