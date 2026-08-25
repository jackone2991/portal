package bank

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	bankapi "github.com/portal/backend/internal/modules/bank/api"
	"github.com/portal/backend/internal/platform/server"
)

const (
	defaultTxLimit = 50
	maxTxLimit     = 100
	dateLayout     = "2006-01-02"
)

// Service holds the bank business logic: owner-scoped ledger CRUD, transfer
// paired-leg semantics, the category hierarchy/reassign matrix, budget tree
// roll-up, the dashboard aggregate, and the emit-after-commit bank:transaction_*
// events (P0.7). Construct via the module.
type Service struct {
	repo   Repository
	events EventPublisher
}

// TxListResult is a keyset page of transactions plus the next cursor ("" = last).
type TxListResult struct {
	Items      []Transaction
	NextCursor string
}

// TransferParams / UpdateTransferParams are the service-level transfer requests.
type TransferParams struct {
	UserID      uuid.UUID
	FromAccount uuid.UUID
	ToAccount   uuid.UUID
	Amount      int64
	OccurredAt  time.Time
	Note        *string
}

type UpdateTransferParams struct {
	UserID      uuid.UUID
	TransferID  uuid.UUID
	FromAccount *uuid.UUID
	ToAccount   *uuid.UUID
	Amount      *int64
	OccurredAt  *time.Time
	Note        *string
}

// Dashboard is the /bank/dashboard aggregate (P0.6). Accounts carry current
// derived balances (grouping by currency is the handler's job); the flow totals
// and budget bars are month-scoped. Balances are always current (§11).
type Dashboard struct {
	Month    string
	Accounts []Account
	Income   int64
	Expense  int64
	Budgets  []BudgetLine
	Recent   []Transaction
}

// ══ Accounts (P0.1) ═════════════════════════════════════════════════════

func (s *Service) CreateAccount(ctx context.Context, in CreateAccountInput) (Account, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !validName(in.Name) {
		return Account{}, ErrValidation
	}
	if !accountTypes[in.Type] {
		return Account{}, ErrValidation
	}
	cur := strings.ToUpper(strings.TrimSpace(in.Currency))
	if cur == "" {
		cur = "VND"
	}
	if len(cur) != 3 {
		return Account{}, ErrValidation
	}
	in.Currency = cur
	return s.repo.CreateAccount(ctx, in)
}

func (s *Service) ListAccounts(ctx context.Context, userID uuid.UUID) ([]Account, error) {
	return s.repo.ListAccountBalances(ctx, userID)
}

func (s *Service) UpdateAccount(ctx context.Context, in UpdateAccountInput) (Account, error) {
	existing, err := s.repo.GetAccount(ctx, in.UserID, in.ID)
	if err != nil {
		return Account{}, err
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if !validName(n) {
			return Account{}, ErrValidation
		}
		in.Name = &n
	}
	if in.Currency != nil {
		cur := strings.ToUpper(strings.TrimSpace(*in.Currency))
		if len(cur) != 3 {
			return Account{}, ErrValidation
		}
		if cur != existing.Currency { // currency is immutable once the account has transactions
			cnt, err := s.repo.CountAccountTransactions(ctx, in.ID)
			if err != nil {
				return Account{}, err
			}
			if cnt > 0 {
				return Account{}, ErrAccountNotMutable
			}
		}
		in.Currency = &cur
	}
	return s.repo.UpdateAccount(ctx, in)
}

func (s *Service) DeleteAccount(ctx context.Context, userID, id uuid.UUID) error {
	if _, err := s.repo.GetAccount(ctx, userID, id); err != nil {
		return err
	}
	cnt, err := s.repo.CountAccountTransactions(ctx, id)
	if err != nil {
		return err
	}
	if cnt > 0 {
		return ErrAccountNotEmpty // archive, don't delete (P0.1)
	}
	return s.repo.DeleteAccount(ctx, userID, id)
}

// ══ Categories (P0.4) ═══════════════════════════════════════════════════

func (s *Service) ListCategories(ctx context.Context, userID uuid.UUID) ([]Category, error) {
	return s.repo.ListCategories(ctx, userID)
}

func (s *Service) CreateCategory(ctx context.Context, in CreateCategoryInput) (Category, error) {
	in.Name = strings.TrimSpace(in.Name)
	if !validName(in.Name) {
		return Category{}, ErrValidation
	}
	if in.Kind != KindIncome && in.Kind != KindExpense {
		return Category{}, ErrValidation
	}
	if in.ParentID != nil {
		parent, err := s.repo.GetVisibleCategory(ctx, in.UserID, *in.ParentID)
		if err != nil {
			return Category{}, err // foreign/missing id → 404 (existence never leaks)
		}
		if parent.ParentID != nil { // parent must be top-level (enforces 2-level max)
			return Category{}, ErrInvalidCategoryParent
		}
		if parent.Kind != in.Kind { // child kind must equal parent kind
			return Category{}, ErrInvalidCategoryParent
		}
	}
	return s.repo.CreateCategory(ctx, in)
}

func (s *Service) UpdateCategory(ctx context.Context, in UpdateCategoryInput) (Category, error) {
	existing, err := s.repo.GetVisibleCategory(ctx, in.UserID, in.ID)
	if err != nil {
		return Category{}, err
	}
	if existing.Seed { // seeds are immutable — owner-mutation matches nothing → 404
		return Category{}, ErrCategoryNotFound
	}
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if !validName(n) {
			return Category{}, ErrValidation
		}
		in.Name = &n
	}
	if in.SetParent && in.ParentID != nil {
		if *in.ParentID == in.ID { // cannot parent to itself
			return Category{}, ErrInvalidCategoryParent
		}
		children, err := s.repo.CountCategoryChildren(ctx, in.ID)
		if err != nil {
			return Category{}, err
		}
		if children > 0 { // a category with children cannot be nested (2-level max)
			return Category{}, ErrInvalidCategoryParent
		}
		parent, err := s.repo.GetVisibleCategory(ctx, in.UserID, *in.ParentID)
		if err != nil {
			return Category{}, err
		}
		if parent.ParentID != nil || parent.Kind != existing.Kind {
			return Category{}, ErrInvalidCategoryParent
		}
	}
	return s.repo.UpdateCategory(ctx, in)
}

func (s *Service) DeleteCategory(ctx context.Context, userID, id uuid.UUID, reassignTo *uuid.UUID) error {
	existing, err := s.repo.GetVisibleCategory(ctx, userID, id)
	if err != nil {
		return err
	}
	if existing.Seed { // seeds are undeletable → 404 (never cascade every user's budgets)
		return ErrCategoryNotFound
	}
	if reassignTo != nil {
		if *reassignTo == id {
			return ErrValidation // cannot reassign to the category being deleted
		}
		target, err := s.repo.GetVisibleCategory(ctx, userID, *reassignTo)
		if err != nil {
			return err
		}
		if target.Kind != existing.Kind {
			return ErrCategoryKindMismatch
		}
	} else {
		cnt, err := s.repo.CountCategoryTransactions(ctx, userID, id)
		if err != nil {
			return err
		}
		if cnt > 0 {
			return ErrCategoryInUse // has transactions and no reassign target → 409
		}
	}
	// The bulk reassignment is one user action → v1 emits no per-row events (P0.7 carve-out).
	return s.repo.DeleteCategory(ctx, userID, id, reassignTo)
}

// ══ Transactions (P0.2) ═════════════════════════════════════════════════

func (s *Service) CreateTransaction(ctx context.Context, in CreateTransactionInput) (Transaction, error) {
	if in.Amount <= 0 {
		return Transaction{}, ErrInvalidAmount
	}
	if in.Direction != DirDebit && in.Direction != DirCredit {
		return Transaction{}, ErrValidation
	}
	if in.CategoryID == nil { // a manual transaction requires a category (only legs go categoryless)
		return Transaction{}, ErrValidation
	}
	if _, err := s.repo.GetAccount(ctx, in.UserID, in.AccountID); err != nil {
		return Transaction{}, err
	}
	cat, err := s.repo.GetVisibleCategory(ctx, in.UserID, *in.CategoryID)
	if err != nil {
		return Transaction{}, err
	}
	if !directionMatchesKind(in.Direction, cat.Kind) {
		return Transaction{}, ErrDirectionKindMismatch
	}
	tx, err := s.repo.CreateTransaction(ctx, in)
	if err != nil {
		return Transaction{}, err
	}
	s.emitTx(ctx, bankapi.EventTransactionCreated, in.UserID, tx, nil)
	return tx, nil
}

func (s *Service) ListTransactions(ctx context.Context, in TxListInput, cursor string) (TxListResult, error) {
	limit := in.Limit
	if limit <= 0 || limit > maxTxLimit {
		limit = defaultTxLimit
	}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return TxListResult{}, ErrBadCursor
		}
		in.CursorAt, in.CursorID = at, id
	}
	in.Limit = limit + 1 // fetch one extra to detect a following page
	rows, err := s.repo.ListTransactions(ctx, in)
	if err != nil {
		return TxListResult{}, err
	}
	var res TxListResult
	if len(rows) > limit {
		res.NextCursor = encodeCursor(rows[limit-1])
		rows = rows[:limit]
	}
	res.Items = rows
	return res, nil
}

func (s *Service) UpdateTransaction(ctx context.Context, in UpdateTransactionInput) (Transaction, error) {
	existing, err := s.repo.GetTransaction(ctx, in.UserID, in.ID)
	if err != nil {
		return Transaction{}, err
	}
	if existing.TransferID != nil {
		return Transaction{}, ErrIsTransferLeg // manage via /bank/transfers/{id}
	}
	if in.Amount != nil && *in.Amount <= 0 {
		return Transaction{}, ErrInvalidAmount
	}
	if in.Direction != nil && *in.Direction != DirDebit && *in.Direction != DirCredit {
		return Transaction{}, ErrValidation
	}
	if in.AccountID != nil {
		if _, err := s.repo.GetAccount(ctx, in.UserID, *in.AccountID); err != nil {
			return Transaction{}, err
		}
	}
	finalDir := existing.Direction
	if in.Direction != nil {
		finalDir = *in.Direction
	}
	finalCat := existing.CategoryID
	if in.CategoryID != nil {
		finalCat = in.CategoryID
	}
	if finalCat != nil {
		cat, err := s.repo.GetVisibleCategory(ctx, in.UserID, *finalCat)
		if err != nil {
			return Transaction{}, err
		}
		if !directionMatchesKind(finalDir, cat.Kind) {
			return Transaction{}, ErrDirectionKindMismatch
		}
	}
	tx, err := s.repo.UpdateTransaction(ctx, in)
	if err != nil {
		return Transaction{}, err
	}
	s.emitTx(ctx, bankapi.EventTransactionUpdated, in.UserID, tx, nil)
	return tx, nil
}

func (s *Service) DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error {
	existing, err := s.repo.GetTransaction(ctx, userID, id)
	if err != nil {
		return err
	}
	if existing.TransferID != nil {
		return ErrIsTransferLeg
	}
	if err := s.repo.DeleteTransaction(ctx, userID, id); err != nil {
		return err
	}
	s.emitTx(ctx, bankapi.EventTransactionDeleted, userID, existing, nil)
	return nil
}

// ══ Transfers (P0.3) ════════════════════════════════════════════════════

func (s *Service) CreateTransfer(ctx context.Context, p TransferParams) ([]Transaction, error) {
	if p.Amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if p.FromAccount == p.ToAccount {
		return nil, ErrSameAccountTransfer
	}
	from, err := s.repo.GetAccount(ctx, p.UserID, p.FromAccount)
	if err != nil {
		return nil, err
	}
	to, err := s.repo.GetAccount(ctx, p.UserID, p.ToAccount)
	if err != nil {
		return nil, err
	}
	if from.Currency != to.Currency {
		return nil, ErrCurrencyMismatch
	}
	legs, err := s.repo.CreateTransfer(ctx, TransferInput{
		UserID:      p.UserID,
		TransferID:  uuid.New(),
		FromAccount: p.FromAccount,
		ToAccount:   p.ToAccount,
		Amount:      p.Amount,
		OccurredAt:  p.OccurredAt,
		Note:        p.Note,
	})
	if err != nil {
		return nil, err
	}
	s.emitTransferLegs(ctx, bankapi.EventTransactionCreated, p.UserID, legs)
	return legs, nil
}

func (s *Service) UpdateTransfer(ctx context.Context, p UpdateTransferParams) ([]Transaction, error) {
	legs, err := s.repo.ListTransferLegs(ctx, p.UserID, p.TransferID)
	if err != nil {
		return nil, err
	}
	if len(legs) == 0 {
		return nil, ErrTransactionNotFound
	}
	curFrom, curTo, curAmount, curOccurred, curNote := transferState(legs)

	from, to := curFrom, curTo
	if p.FromAccount != nil {
		from = *p.FromAccount
	}
	if p.ToAccount != nil {
		to = *p.ToAccount
	}
	amount := curAmount
	if p.Amount != nil {
		amount = *p.Amount
	}
	occurred := curOccurred
	if p.OccurredAt != nil {
		occurred = *p.OccurredAt
	}
	note := curNote
	if p.Note != nil {
		note = p.Note
	}

	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	if from == to {
		return nil, ErrSameAccountTransfer
	}
	fromAcct, err := s.repo.GetAccount(ctx, p.UserID, from)
	if err != nil {
		return nil, err
	}
	toAcct, err := s.repo.GetAccount(ctx, p.UserID, to)
	if err != nil {
		return nil, err
	}
	if fromAcct.Currency != toAcct.Currency {
		return nil, ErrCurrencyMismatch
	}
	updated, err := s.repo.UpdateTransfer(ctx, UpdateTransferInput{
		UserID:      p.UserID,
		TransferID:  p.TransferID,
		FromAccount: from,
		ToAccount:   to,
		Amount:      amount,
		OccurredAt:  occurred,
		Note:        note,
	})
	if err != nil {
		return nil, err
	}
	s.emitTransferLegs(ctx, bankapi.EventTransactionUpdated, p.UserID, updated)
	return updated, nil
}

func (s *Service) DeleteTransfer(ctx context.Context, userID, transferID uuid.UUID) error {
	legs, err := s.repo.ListTransferLegs(ctx, userID, transferID)
	if err != nil {
		return err
	}
	if len(legs) == 0 {
		return ErrTransactionNotFound
	}
	if err := s.repo.DeleteTransfer(ctx, userID, transferID); err != nil {
		return err
	}
	s.emitTransferLegs(ctx, bankapi.EventTransactionDeleted, userID, legs)
	return nil
}

// ══ Budgets (P0.5) ══════════════════════════════════════════════════════

// SetBudget upserts one (category, month, amount). amount ≤ 0 deletes the row —
// the only removal path (P0.5). The category must be visible and expense-kind.
func (s *Service) SetBudget(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount int64) error {
	cat, err := s.repo.GetVisibleCategory(ctx, userID, categoryID)
	if err != nil {
		return err
	}
	if cat.Kind != KindExpense { // spent is defined only over expense debits
		return ErrCategoryKindMismatch
	}
	month = firstOfMonth(month)
	if amount <= 0 {
		return s.repo.DeleteBudget(ctx, userID, categoryID, month)
	}
	return s.repo.UpsertBudget(ctx, userID, categoryID, month, amount)
}

func (s *Service) ListBudgets(ctx context.Context, userID uuid.UUID, month time.Time) ([]BudgetLine, error) {
	return s.repo.ListBudgetsForMonth(ctx, userID, firstOfMonth(month))
}

// ══ Dashboard (P0.6) ════════════════════════════════════════════════════

func (s *Service) Dashboard(ctx context.Context, userID uuid.UUID, month time.Time) (Dashboard, error) {
	month = firstOfMonth(month)
	accts, err := s.repo.ListAccountBalances(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}
	income, expense, err := s.repo.MonthFlowTotals(ctx, userID, month)
	if err != nil {
		return Dashboard{}, err
	}
	budgets, err := s.repo.ListBudgetsForMonth(ctx, userID, month)
	if err != nil {
		return Dashboard{}, err
	}
	recent, err := s.repo.RecentTransactions(ctx, userID, 10)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{
		Month:    month.Format("2006-01"),
		Accounts: accts,
		Income:   income,
		Expense:  expense,
		Budgets:  budgets,
		Recent:   recent,
	}, nil
}

// ── events ───────────────────────────────────────────────────────────

// emitTx publishes one bank:transaction_* event (best-effort, after commit).
func (s *Service) emitTx(ctx context.Context, name string, userID uuid.UUID, tx Transaction, counterparty *uuid.UUID) {
	if s.events == nil {
		return
	}
	ev := bankapi.TransactionEvent{
		TransactionID:         tx.ID,
		UserID:                userID,
		AccountID:             tx.AccountID,
		Amount:                tx.Amount,
		Direction:             tx.Direction,
		CategoryID:            tx.CategoryID,
		OccurredAt:            tx.OccurredAt.Format(dateLayout),
		IsTransfer:            tx.IsTransferLeg(),
		TransferID:            tx.TransferID,
		CounterpartyAccountID: counterparty,
	}
	if err := s.events.Publish(ctx, name, ev); err != nil {
		log.Warn().Err(err).Str("transaction", tx.ID.String()).Msg("bank: transaction event publish failed")
	}
}

// emitTransferLegs emits one event per leg; each leg's counterparty is the other
// leg's account, so either payload alone renders the same "moved X A→B" card.
func (s *Service) emitTransferLegs(ctx context.Context, name string, userID uuid.UUID, legs []Transaction) {
	for i, leg := range legs {
		var counterparty *uuid.UUID
		for j, other := range legs {
			if i != j {
				cp := other.AccountID
				counterparty = &cp
				break
			}
		}
		s.emitTx(ctx, name, userID, leg, counterparty)
	}
}

// ── helpers ───────────────────────────────────────────────────────────

func directionMatchesKind(dir, kind string) bool {
	return (dir == DirDebit && kind == KindExpense) || (dir == DirCredit && kind == KindIncome)
}

func validName(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= maxNameLen
}

func firstOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
}

// transferState extracts the current (from, to, amount, occurred_at, note) of a
// transfer from its legs: from = the debit leg's account, to = the credit leg's.
func transferState(legs []Transaction) (from, to uuid.UUID, amount int64, occurred time.Time, note *string) {
	for _, leg := range legs {
		switch leg.Direction {
		case DirDebit:
			from = leg.AccountID
		case DirCredit:
			to = leg.AccountID
		}
		amount = leg.Amount
		occurred = leg.OccurredAt
		note = leg.Note
	}
	return from, to, amount, occurred, note
}

// keyset cursor "<occurred_at date>|<id>", base64url.
func encodeCursor(t Transaction) string {
	return server.EncodeCursor(t.OccurredAt.UTC().Format(dateLayout), t.ID)
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	key, id, err := server.DecodeCursor(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	at, err := time.Parse(dateLayout, key)
	if err != nil {
		return time.Time{}, uuid.Nil, server.ErrBadCursor
	}
	return at, id, nil
}
