// Package api is the public surface of the social module. Other modules import
// only this package.
package api

import (
	"context"

	"github.com/google/uuid"
)

// Events other modules can subscribe to. Payloads are the structs below.
const (
	// EventConnectionRequested fires when someone asks to connect. notify turns
	// it into the addressee's bell entry.
	EventConnectionRequested = "social:connection_requested"
	// EventConnectionAccepted fires when the addressee agrees. Both parties are
	// now connected; notify tells the requester.
	EventConnectionAccepted = "social:connection_accepted"
)

// ConnectionEvent is the body of both events. RequesterID/AddresseeID keep their
// meaning after acceptance: who asked, and who was asked.
type ConnectionEvent struct {
	ConnectionID  string `json:"connection_id"`
	RequesterID   string `json:"requester_id"`
	AddresseeID   string `json:"addressee_id"`
	RequesterName string `json:"requester_name"`
	AddresseeName string `json:"addressee_name"`
}

// API is what other modules program against.
type API interface {
	// CounterpartIDs lists every account the user already has a relationship
	// with, pending or accepted. people/suggestions subtracts these so it stops
	// offering someone you have already asked.
	CounterpartIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}
