package bank

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// Errors surfaced to the handler (mapped to RFC 7807 Problems in handler.go).
// Owner-scoping resolves a foreign id to "not found" — existence never leaks.
var (
	ErrAccountNotFound       = errors.New("bank: account not found")
	ErrAccountNotEmpty       = errors.New("bank: account not empty")
	ErrAccountNotMutable     = errors.New("bank: account not mutable")
	ErrCategoryNotFound      = errors.New("bank: category not found")
	ErrCategoryInUse         = errors.New("bank: category in use")
	ErrCategoryKindMismatch  = errors.New("bank: category kind mismatch")
	ErrCategoryImmutable     = errors.New("bank: category immutable")
	ErrInvalidCategoryParent = errors.New("bank: invalid category parent")
	ErrTransactionNotFound   = errors.New("bank: transaction not found")
	ErrIsTransferLeg         = errors.New("bank: is a transfer leg")
	ErrSameAccountTransfer   = errors.New("bank: same-account transfer")
	ErrCurrencyMismatch      = errors.New("bank: currency mismatch")
	ErrDirectionKindMismatch = errors.New("bank: direction/kind mismatch")
	ErrInvalidAmount         = errors.New("bank: invalid amount")
	ErrValidation            = errors.New("bank: validation error")
	ErrBadCursor             = errors.New("bank: invalid cursor")
)

// Enumerations (mirror the 0014 CHECK constraints).
var accountTypes = map[string]bool{
	"cash": true, "checking": true, "savings": true,
	"credit_card": true, "ewallet": true, "other": true,
}

const (
	KindIncome  = "income"
	KindExpense = "expense"

	DirDebit  = "debit"
	DirCredit = "credit"

	maxNameLen = 100
)

// Account is the bank module's internal account record. Balance is DERIVED
// (opening_balance + Σcredits − Σdebits); it is only populated by the balance-
// bearing reads (ListAccountBalances), zero otherwise.
type Account struct {
	ID             uuid.UUID
	Name           string
	Type           string
	Currency       string
	OpeningBalance int64
	Archived       bool
	Balance        int64
	CreatedAt      time.Time
}

// Category is a hierarchical (2-level) income/expense bucket. Seed reports a
// user_id-NULL default (visible to everyone, mutable by no one). ParentID nil =
// top-level.
type Category struct {
	ID       uuid.UUID
	ParentID *uuid.UUID
	Name     string
	Kind     string
	Seed     bool
}

// Transaction is one ledger row. A transfer leg has TransferID set and
// CategoryID nil (P0.3). OccurredAt is a date (time-of-day out of scope).
type Transaction struct {
	ID         uuid.UUID
	AccountID  uuid.UUID
	CategoryID *uuid.UUID
	Amount     int64
	Direction  string
	TransferID *uuid.UUID
	OccurredAt time.Time
	Note       *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// IsTransferLeg is the reporting predicate (P0.3): transfer_id set AND
// category_id nil. A P1.13 fee row (both set) is NOT a leg.
func (t Transaction) IsTransferLeg() bool {
	return t.TransferID != nil && t.CategoryID == nil
}

// BudgetLine is one budgeted category for a month with its derived spend
// (Σ expense debits incl. children, excluding transfer legs — P0.5). ParentName
// lets the tree render a header for an unbudgeted parent without a client join.
type BudgetLine struct {
	CategoryID uuid.UUID
	ParentID   *uuid.UUID
	Name       string
	ParentName *string
	Amount     int64
	Spent      int64
}

// ── input structs ───────────────────────────────────────────────────

type CreateAccountInput struct {
	UserID         uuid.UUID
	Name           string
	Type           string
	Currency       string
	OpeningBalance int64
}

type UpdateAccountInput struct {
	UserID   uuid.UUID
	ID       uuid.UUID
	Name     *string
	Archived *bool
	Currency *string
}

type CreateCategoryInput struct {
	UserID   uuid.UUID
	ParentID *uuid.UUID
	Name     string
	Kind     string
}

type UpdateCategoryInput struct {
	UserID    uuid.UUID
	ID        uuid.UUID
	Name      *string
	SetParent bool       // whether parent_id is being changed at all
	ParentID  *uuid.UUID // the new parent (nil = promote to top-level) when SetParent
}

type CreateTransactionInput struct {
	UserID     uuid.UUID
	AccountID  uuid.UUID
	CategoryID *uuid.UUID
	Amount     int64
	Direction  string
	TransferID *uuid.UUID
	OccurredAt time.Time
	Note       *string
}

type UpdateTransactionInput struct {
	UserID     uuid.UUID
	ID         uuid.UUID
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Amount     *int64
	Direction  *string
	OccurredAt *time.Time
	Note       *string
}

// TransferInput creates a paired debit+credit sharing TransferID (set by the
// service). Legs carry no category.
type TransferInput struct {
	UserID      uuid.UUID
	TransferID  uuid.UUID
	FromAccount uuid.UUID
	ToAccount   uuid.UUID
	Amount      int64
	OccurredAt  time.Time
	Note        *string
}

// UpdateTransferInput carries the RESOLVED final values (the service merges the
// patch against the current legs and validates before calling). FromAccount is
// the debit leg's account, ToAccount the credit leg's.
type UpdateTransferInput struct {
	UserID      uuid.UUID
	TransferID  uuid.UUID
	FromAccount uuid.UUID
	ToAccount   uuid.UUID
	Amount      int64
	OccurredAt  time.Time
	Note        *string
}

// TxListInput is the keyset transaction query (P0.2). A nil filter is "any"; a
// zero CursorAt is "first page".
type TxListInput struct {
	UserID     uuid.UUID
	AccountID  *uuid.UUID
	CategoryID *uuid.UUID
	Month      *time.Time // first-of-month
	CursorAt   time.Time
	CursorID   uuid.UUID
	Limit      int
}

// Repository is the persistence surface. The sqlc-backed adapter implements it
// (transactional methods run in a pgx tx); tests use an in-memory fake.
type Repository interface {
	// accounts
	CreateAccount(ctx context.Context, in CreateAccountInput) (Account, error)
	GetAccount(ctx context.Context, userID, id uuid.UUID) (Account, error)
	ListAccountBalances(ctx context.Context, userID uuid.UUID) ([]Account, error)
	UpdateAccount(ctx context.Context, in UpdateAccountInput) (Account, error)
	DeleteAccount(ctx context.Context, userID, id uuid.UUID) error
	CountAccountTransactions(ctx context.Context, id uuid.UUID) (int64, error)

	// categories
	CreateCategory(ctx context.Context, in CreateCategoryInput) (Category, error)
	GetVisibleCategory(ctx context.Context, userID, id uuid.UUID) (Category, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]Category, error)
	UpdateCategory(ctx context.Context, in UpdateCategoryInput) (Category, error)
	CountCategoryTransactions(ctx context.Context, userID, id uuid.UUID) (int64, error)
	CountCategoryChildren(ctx context.Context, id uuid.UUID) (int64, error)
	// DeleteCategory reassigns this category's own transactions to reassignTo (if
	// non-nil) and deletes the category in one tx. Children promote via ON DELETE
	// SET NULL; budgets cascade.
	DeleteCategory(ctx context.Context, userID, id uuid.UUID, reassignTo *uuid.UUID) error

	// transactions
	CreateTransaction(ctx context.Context, in CreateTransactionInput) (Transaction, error)
	GetTransaction(ctx context.Context, userID, id uuid.UUID) (Transaction, error)
	ListTransactions(ctx context.Context, in TxListInput) ([]Transaction, error)
	UpdateTransaction(ctx context.Context, in UpdateTransactionInput) (Transaction, error)
	DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error
	RecentTransactions(ctx context.Context, userID uuid.UUID, limit int) ([]Transaction, error)

	// transfers (transactional)
	CreateTransfer(ctx context.Context, in TransferInput) ([]Transaction, error)
	ListTransferLegs(ctx context.Context, userID, transferID uuid.UUID) ([]Transaction, error)
	UpdateTransfer(ctx context.Context, in UpdateTransferInput) ([]Transaction, error)
	DeleteTransfer(ctx context.Context, userID, transferID uuid.UUID) error

	// budgets
	UpsertBudget(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount int64) error
	DeleteBudget(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error
	ListBudgetsForMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]BudgetLine, error)

	// dashboard
	MonthFlowTotals(ctx context.Context, userID uuid.UUID, month time.Time) (income, expense int64, err error)
}

// EventPublisher fans a domain event out to its subscribers (platform/events).
// Optional on the module — a nil publisher skips emission.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
