package tenantrepo

// Adapter bridges the sqlc-generated Queries to tenantapi.Store. Organizations
// and memberships are NOT tenant-scoped (no RLS), so these run on whatever DBTX
// wiring provides — the platform/db.Conn, which uses the request tx if one is
// open, else the pool.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	tenantapi "github.com/portal/backend/internal/modules/tenant/api"
)

type Adapter struct{ q *Queries }

// NewAdapter builds the adapter over any pgx-compatible handle (pool or the
// context-aware platform/db.Conn).
func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

var _ tenantapi.Store = (*Adapter)(nil)

func (a *Adapter) GetByID(ctx context.Context, id uuid.UUID) (*tenantapi.Organization, error) {
	return orgOrNil(a.q.GetOrganizationByID(ctx, pgUUID(id)))
}

func (a *Adapter) GetBySlug(ctx context.Context, slug string) (*tenantapi.Organization, error) {
	return orgOrNil(a.q.GetOrganizationBySlug(ctx, slug))
}

func (a *Adapter) PersonalOrg(ctx context.Context, userID uuid.UUID) (*tenantapi.Organization, error) {
	return orgOrNil(a.q.GetPersonalOrgForUser(ctx, pgUUID(userID)))
}

// GetOrCreatePersonalOrg is idempotent and race-safe: the unique partial index
// organizations_personal_owner_idx (0018) makes a concurrent second INSERT a
// no-op (zero rows returned), in which case we re-fetch the winner.
func (a *Adapter) GetOrCreatePersonalOrg(ctx context.Context, userID uuid.UUID, name string) (*tenantapi.Organization, error) {
	// Fast path: already exists (backfilled or created earlier).
	if row, err := a.q.GetPersonalOrgForUser(ctx, pgUUID(userID)); err == nil {
		org := toOrg(row)
		return &org, nil
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	row, err := a.q.CreatePersonalOrg(ctx, CreatePersonalOrgParams{OwnerID: pgUUID(userID), Name: name})
	if errors.Is(err, pgx.ErrNoRows) {
		// Lost the race to a concurrent create — re-fetch the existing row.
		row, err = a.q.GetPersonalOrgForUser(ctx, pgUUID(userID))
	}
	if err != nil {
		return nil, err
	}
	if err := a.q.CreateMembership(ctx, CreateMembershipParams{OrgID: row.ID, UserID: pgUUID(userID), Role: "owner"}); err != nil {
		return nil, err
	}
	org := toOrg(row)
	return &org, nil
}

func (a *Adapter) ListForUser(ctx context.Context, userID uuid.UUID) ([]tenantapi.Organization, error) {
	rows, err := a.q.ListOrganizationsForUser(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]tenantapi.Organization, 0, len(rows))
	for _, r := range rows {
		out = append(out, toOrg(r))
	}
	return out, nil
}

func (a *Adapter) IsMember(ctx context.Context, userID, orgID uuid.UUID) (bool, error) {
	return a.q.IsMember(ctx, IsMemberParams{OrgID: pgUUID(orgID), UserID: pgUUID(userID)})
}

func orgOrNil(row Organization, err error) (*tenantapi.Organization, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	o := toOrg(row)
	return &o, nil
}

func toOrg(r Organization) tenantapi.Organization {
	return tenantapi.Organization{
		ID:      uuidFrom(r.ID),
		Kind:    r.Kind,
		Slug:    r.Slug,
		Name:    r.Name,
		OwnerID: uuidFrom(r.OwnerID),
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

// ListAllIDs returns every organization id, oldest first.
func (a *Adapter) ListAllIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := a.q.ListAllOrganizationIDs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, uuidFrom(r))
	}
	return out, nil
}
