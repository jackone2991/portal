// Package socialrepo adapts the sqlc-generated queries to the social module's
// Repository interface. Generated files in this directory are not hand-edited.
package socialrepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/social"
)

// Adapter implements social.Repository over the generated Queries.
type Adapter struct{ q *Queries }

// NewAdapter builds the adapter over a DBTX (the platform pool or a request tx).
func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

func (a *Adapter) CreateRequest(ctx context.Context, requester, addressee uuid.UUID) (social.Connection, error) {
	row, err := a.q.CreateRequest(ctx, CreateRequestParams{
		RequesterID: pgUUID(requester),
		AddresseeID: pgUUID(addressee),
	})
	if err != nil {
		// 23505 is the pair unique index: a relationship already exists in one
		// direction or the other. That is a conflict, not a server error.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return social.Connection{}, social.ErrExists
		}
		return social.Connection{}, err
	}
	return toConnection(row), nil
}

func (a *Adapter) Get(ctx context.Context, id uuid.UUID) (social.Connection, error) {
	row, err := a.q.GetConnection(ctx, pgUUID(id))
	return one(row, err)
}

func (a *Adapter) FindBetween(ctx context.Context, x, y uuid.UUID) (social.Connection, error) {
	row, err := a.q.FindBetween(ctx, FindBetweenParams{A: pgUUID(x), B: pgUUID(y)})
	return one(row, err)
}

func (a *Adapter) Accept(ctx context.Context, id, addressee uuid.UUID) (social.Connection, error) {
	row, err := a.q.AcceptRequest(ctx, AcceptRequestParams{ID: pgUUID(id), AddresseeID: pgUUID(addressee)})
	return one(row, err)
}

func (a *Adapter) Delete(ctx context.Context, id, actor uuid.UUID) (social.Connection, error) {
	row, err := a.q.DeleteConnection(ctx, DeleteConnectionParams{ID: pgUUID(id), RequesterID: pgUUID(actor)})
	return one(row, err)
}

func (a *Adapter) List(ctx context.Context, me uuid.UUID, kind social.Kind) ([]social.Connection, error) {
	var (
		rows []SocialConnection
		err  error
	)
	switch kind {
	case social.KindIncoming:
		rows, err = a.q.ListIncoming(ctx, pgUUID(me))
	case social.KindOutgoing:
		rows, err = a.q.ListOutgoing(ctx, pgUUID(me))
	default:
		rows, err = a.q.ListAccepted(ctx, pgUUID(me))
	}
	if err != nil {
		return nil, err
	}
	out := make([]social.Connection, 0, len(rows))
	for _, r := range rows {
		out = append(out, toConnection(r))
	}
	return out, nil
}

func (a *Adapter) CounterpartIDs(ctx context.Context, me uuid.UUID) ([]uuid.UUID, error) {
	rows, err := a.q.ListCounterpartIDs(ctx, pgUUID(me))
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, uuidFrom(r))
	}
	return out, nil
}

func (a *Adapter) CountIncoming(ctx context.Context, me uuid.UUID) (int, error) {
	n, err := a.q.CountIncoming(ctx, pgUUID(me))
	return int(n), err
}

/* ── helpers ─────────────────────────────────────────────────────── */

// one maps a single-row result. No rows means either "gone" or "hidden by the
// 0037 policies"; both answer ErrNotFound, which is what keeps a connection the
// caller is not part of from being confirmed to exist.
func one(row SocialConnection, err error) (social.Connection, error) {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return social.Connection{}, social.ErrNotFound
		}
		return social.Connection{}, err
	}
	return toConnection(row), nil
}

func toConnection(r SocialConnection) social.Connection {
	c := social.Connection{
		ID:          uuidFrom(r.ID),
		RequesterID: uuidFrom(r.RequesterID),
		AddresseeID: uuidFrom(r.AddresseeID),
		Status:      r.Status,
		CreatedAt:   r.CreatedAt.Time,
	}
	if r.RespondedAt.Valid {
		t := r.RespondedAt.Time
		c.RespondedAt = &t
	}
	return c
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}
