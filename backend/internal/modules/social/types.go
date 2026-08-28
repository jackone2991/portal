// Package social owns connections between accounts on this Portal — the first
// piece of the social layer.
//
// It owns exactly one table, `social_connections` (0037), and one idea: two
// accounts are connected, or one has asked the other, or neither. It does not
// own posts, likes, comments or chat; those build on top of knowing who is
// connected to whom, which is why this comes first.
//
// The module never reads `users`. It stores ids and resolves display names
// through account's api/ package (MODULES.md).
package social

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors surfaced to the handler.
var (
	ErrNotFound = errors.New("social: connection not found")
	// ErrExists covers both "already connected" and "already asked", in either
	// direction — the pair unique index does not distinguish, and neither should
	// the answer: the caller's next step is the same.
	ErrExists = errors.New("social: a connection already exists")
	ErrSelf   = errors.New("social: cannot connect to yourself")
)

// Connection statuses. A declined request is deleted rather than stored, so
// there is no 'declined'.
const (
	StatusPending  = "pending"
	StatusAccepted = "accepted"
)

// Connection is one row of social_connections.
type Connection struct {
	ID          uuid.UUID
	RequesterID uuid.UUID
	AddresseeID uuid.UUID
	Status      string
	CreatedAt   time.Time
	RespondedAt *time.Time
}

// Other returns the party that is not `me`.
func (c Connection) Other(me uuid.UUID) uuid.UUID {
	if c.RequesterID == me {
		return c.AddresseeID
	}
	return c.RequesterID
}

// Party is one connection rendered from the caller's point of view: who the
// other person is, and how the link stands.
type Party struct {
	ID          uuid.UUID // the connection id, not the user id
	UserID      uuid.UUID
	DisplayName string
	Status      string
	// Outgoing distinguishes "you asked them" from "they asked you" on a pending
	// row, which is the difference between a Cancel button and an Accept button.
	Outgoing  bool
	CreatedAt time.Time
}

// Kind selects which list to read.
type Kind string

const (
	KindAccepted Kind = "accepted"
	KindIncoming Kind = "incoming"
	KindOutgoing Kind = "outgoing"
)

// Repository is the persistence surface. Every method is already fenced by the
// 0037 policies; the userID arguments express intent, not isolation.
type Repository interface {
	CreateRequest(ctx context.Context, requester, addressee uuid.UUID) (Connection, error)
	Get(ctx context.Context, id uuid.UUID) (Connection, error)
	FindBetween(ctx context.Context, a, b uuid.UUID) (Connection, error)
	Accept(ctx context.Context, id, addressee uuid.UUID) (Connection, error)
	Delete(ctx context.Context, id, actor uuid.UUID) (Connection, error)
	List(ctx context.Context, me uuid.UUID, kind Kind) ([]Connection, error)
	CounterpartIDs(ctx context.Context, me uuid.UUID) ([]uuid.UUID, error)
	CountIncoming(ctx context.Context, me uuid.UUID) (int, error)
}

// NamesFunc resolves user ids to display names. Wired from
// accountapi.GetUserNames; social must not read `users` itself.
type NamesFunc func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)

// EventPublisher is the platform/events publisher, optional.
type EventPublisher interface {
	Publish(ctx context.Context, event string, payload any) error
}
