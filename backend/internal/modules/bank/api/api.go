// Package api is the public surface of the bank module (SPEC-03).
//
// bank owns the personal ledger (accounts, categories, transactions, budgets).
// Its cross-module contract at v1 is the emit-only bank:transaction_* event
// family, announced on the platform/events bus after a write commits (P0.7) —
// the money facet feeds the life stream (SPEC-06) from day one. No synchronous
// call surface yet.
//
// Only this package may be imported by other modules; bank's service/handler/
// repository internals stay private.
package api

import "github.com/google/uuid"

// Event names (events.md). Every money mutation emits exactly one, after commit.
const (
	EventTransactionCreated = "bank:transaction_created"
	EventTransactionUpdated = "bank:transaction_updated"
	EventTransactionDeleted = "bank:transaction_deleted"
)

// TransactionEvent is the bank:transaction_* payload (events.md): ids + the
// minimum a consumer needs to render/decide. OccurredAt is a date string
// (YYYY-MM-DD). For a transfer leg, IsTransfer is true and
// CounterpartyAccountID is the OTHER leg's account, so either leg alone renders
// the same "moved X A→B" card (SPEC-06 owns the direction-normalizing rule). A
// P1.13 fee row emits IsTransfer=false with its TransferID set.
type TransactionEvent struct {
	TransactionID         uuid.UUID  `json:"transaction_id"`
	UserID                uuid.UUID  `json:"user_id"`
	AccountID             uuid.UUID  `json:"account_id"`
	Amount                int64      `json:"amount"`
	Direction             string     `json:"direction"`
	CategoryID            *uuid.UUID `json:"category_id"`
	OccurredAt            string     `json:"occurred_at"`
	IsTransfer            bool       `json:"is_transfer"`
	TransferID            *uuid.UUID `json:"transfer_id"`
	CounterpartyAccountID *uuid.UUID `json:"counterparty_account_id"`
}
