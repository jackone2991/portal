package bankrepo

// Adapter bridges the sqlc-generated Queries to bank.Repository. Multi-statement
// operations (transfers, category delete-with-reassign) run through an injected
// RunInTx so they execute on the request's tenant-scoped tx when one is open
// (else a fresh pool tx). All pgtype ↔ domain juggling lives here so the module
// code stays sqlc-agnostic.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/bank"
)

type Adapter struct {
	q       *Queries
	runInTx func(context.Context, func(pgx.Tx) error) error
}

// NewAdapter builds the adapter over a DBTX (the context-aware platform/db.Conn)
// plus a RunInTx that opens/reuses the request transaction for multi-statement
// methods. cmd/api and cmd/worker construct it once each.
func NewAdapter(db DBTX, runInTx func(context.Context, func(pgx.Tx) error) error) *Adapter {
	return &Adapter{q: New(db), runInTx: runInTx}
}

var _ bank.Repository = (*Adapter)(nil)

// ── accounts ─────────────────────────────────────────────────────────

func (a *Adapter) CreateAccount(ctx context.Context, in bank.CreateAccountInput) (bank.Account, error) {
	row, err := a.q.CreateAccount(ctx, CreateAccountParams{
		UserID:         pgUUID(in.UserID),
		Name:           in.Name,
		Type:           in.Type,
		Currency:       in.Currency,
		OpeningBalance: in.OpeningBalance,
	})
	if err != nil {
		return bank.Account{}, err
	}
	return toAccount(row), nil
}

func (a *Adapter) GetAccount(ctx context.Context, userID, id uuid.UUID) (bank.Account, error) {
	row, err := a.q.GetAccount(ctx, GetAccountParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Account{}, bank.ErrAccountNotFound
		}
		return bank.Account{}, err
	}
	return toAccount(row), nil
}

func (a *Adapter) ListAccountBalances(ctx context.Context, userID uuid.UUID) ([]bank.Account, error) {
	rows, err := a.q.ListAccountBalances(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]bank.Account, 0, len(rows))
	for _, r := range rows {
		out = append(out, bank.Account{
			ID:             uuidFrom(r.ID),
			Name:           r.Name,
			Type:           r.Type,
			Currency:       r.Currency,
			OpeningBalance: r.OpeningBalance,
			Archived:       r.Archived,
			Balance:        r.Balance,
			CreatedAt:      r.CreatedAt.Time,
		})
	}
	return out, nil
}

func (a *Adapter) UpdateAccount(ctx context.Context, in bank.UpdateAccountInput) (bank.Account, error) {
	row, err := a.q.UpdateAccount(ctx, UpdateAccountParams{
		Name:     in.Name,
		Archived: in.Archived,
		Currency: in.Currency,
		ID:       pgUUID(in.ID),
		UserID:   pgUUID(in.UserID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Account{}, bank.ErrAccountNotFound
		}
		return bank.Account{}, err
	}
	return toAccount(row), nil
}

func (a *Adapter) DeleteAccount(ctx context.Context, userID, id uuid.UUID) error {
	_, err := a.q.DeleteAccount(ctx, DeleteAccountParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.ErrAccountNotFound
		}
		return err
	}
	return nil
}

func (a *Adapter) CountAccountTransactions(ctx context.Context, id uuid.UUID) (int64, error) {
	return a.q.CountAccountTransactions(ctx, pgUUID(id))
}

// ── categories ───────────────────────────────────────────────────────

func (a *Adapter) CreateCategory(ctx context.Context, in bank.CreateCategoryInput) (bank.Category, error) {
	row, err := a.q.CreateCategory(ctx, CreateCategoryParams{
		UserID:   pgUUID(in.UserID),
		Name:     in.Name,
		Kind:     in.Kind,
		ParentID: optUUID(in.ParentID),
	})
	if err != nil {
		return bank.Category{}, err
	}
	return toCategory(row), nil
}

func (a *Adapter) GetVisibleCategory(ctx context.Context, userID, id uuid.UUID) (bank.Category, error) {
	row, err := a.q.GetVisibleCategory(ctx, GetVisibleCategoryParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Category{}, bank.ErrCategoryNotFound
		}
		return bank.Category{}, err
	}
	return toCategory(row), nil
}

func (a *Adapter) ListCategories(ctx context.Context, userID uuid.UUID) ([]bank.Category, error) {
	rows, err := a.q.ListCategories(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]bank.Category, 0, len(rows))
	for _, r := range rows {
		out = append(out, toCategory(r))
	}
	return out, nil
}

func (a *Adapter) UpdateCategory(ctx context.Context, in bank.UpdateCategoryInput) (bank.Category, error) {
	row, err := a.q.UpdateCategory(ctx, UpdateCategoryParams{
		Name:      in.Name,
		SetParent: in.SetParent,
		ParentID:  optUUID(in.ParentID),
		ID:        pgUUID(in.ID),
		UserID:    pgUUID(in.UserID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Category{}, bank.ErrCategoryNotFound
		}
		return bank.Category{}, err
	}
	return toCategory(row), nil
}

func (a *Adapter) CountCategoryTransactions(ctx context.Context, userID, id uuid.UUID) (int64, error) {
	return a.q.CountCategoryTransactions(ctx, CountCategoryTransactionsParams{
		CategoryID: pgUUID(id), UserID: pgUUID(userID),
	})
}

func (a *Adapter) CountCategoryChildren(ctx context.Context, id uuid.UUID) (int64, error) {
	return a.q.CountCategoryChildren(ctx, pgUUID(id))
}

func (a *Adapter) DeleteCategory(ctx context.Context, userID, id uuid.UUID, reassignTo *uuid.UUID) error {
	return a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		if reassignTo != nil {
			if err := q.ReassignCategoryTransactions(ctx, ReassignCategoryTransactionsParams{
				TargetID: pgUUID(*reassignTo),
				FromID:   pgUUID(id),
				UserID:   pgUUID(userID),
			}); err != nil {
				return err
			}
		}
		if _, err := q.DeleteCategory(ctx, DeleteCategoryParams{ID: pgUUID(id), UserID: pgUUID(userID)}); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return bank.ErrCategoryNotFound
			}
			return err
		}
		return nil
	})
}

// ── transactions ─────────────────────────────────────────────────────

func (a *Adapter) CreateTransaction(ctx context.Context, in bank.CreateTransactionInput) (bank.Transaction, error) {
	row, err := a.q.CreateTransaction(ctx, createTxParams(in))
	if err != nil {
		return bank.Transaction{}, err
	}
	return toTransaction(row), nil
}

func (a *Adapter) GetTransaction(ctx context.Context, userID, id uuid.UUID) (bank.Transaction, error) {
	row, err := a.q.GetTransaction(ctx, GetTransactionParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Transaction{}, bank.ErrTransactionNotFound
		}
		return bank.Transaction{}, err
	}
	return toTransaction(row), nil
}

func (a *Adapter) ListTransactions(ctx context.Context, in bank.TxListInput) ([]bank.Transaction, error) {
	p := ListTransactionsByUserCursorParams{
		UserID:     pgUUID(in.UserID),
		AccountID:  optUUID(in.AccountID),
		CategoryID: optUUID(in.CategoryID),
		Month:      optDate(in.Month),
		Lim:        int32(in.Limit),
	}
	if !in.CursorAt.IsZero() {
		p.CursorOccurredAt = pgDate(in.CursorAt)
		p.CursorID = pgUUID(in.CursorID)
	}
	rows, err := a.q.ListTransactionsByUserCursor(ctx, p)
	if err != nil {
		return nil, err
	}
	return toTransactions(rows), nil
}

func (a *Adapter) UpdateTransaction(ctx context.Context, in bank.UpdateTransactionInput) (bank.Transaction, error) {
	row, err := a.q.UpdateTransaction(ctx, UpdateTransactionParams{
		AccountID:  optUUID(in.AccountID),
		CategoryID: optUUID(in.CategoryID),
		Amount:     in.Amount,
		Direction:  in.Direction,
		OccurredAt: optDate(in.OccurredAt),
		Note:       in.Note,
		ID:         pgUUID(in.ID),
		UserID:     pgUUID(in.UserID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Transaction{}, bank.ErrTransactionNotFound
		}
		return bank.Transaction{}, err
	}
	return toTransaction(row), nil
}

func (a *Adapter) DeleteTransaction(ctx context.Context, userID, id uuid.UUID) error {
	_, err := a.q.DeleteTransaction(ctx, DeleteTransactionParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.ErrTransactionNotFound
		}
		return err
	}
	return nil
}

func (a *Adapter) RecentTransactions(ctx context.Context, userID uuid.UUID, limit int) ([]bank.Transaction, error) {
	rows, err := a.q.RecentTransactions(ctx, RecentTransactionsParams{UserID: pgUUID(userID), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	return toTransactions(rows), nil
}

// ── transfers (transactional) ────────────────────────────────────────

func (a *Adapter) CreateTransfer(ctx context.Context, in bank.TransferInput) ([]bank.Transaction, error) {
	var out []bank.Transaction
	err := a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		tid := in.TransferID
		debit, err := q.CreateTransaction(ctx, createTxParams(bank.CreateTransactionInput{
			UserID: in.UserID, AccountID: in.FromAccount, Amount: in.Amount,
			Direction: bank.DirDebit, TransferID: &tid, OccurredAt: in.OccurredAt, Note: in.Note,
		}))
		if err != nil {
			return err
		}
		credit, err := q.CreateTransaction(ctx, createTxParams(bank.CreateTransactionInput{
			UserID: in.UserID, AccountID: in.ToAccount, Amount: in.Amount,
			Direction: bank.DirCredit, TransferID: &tid, OccurredAt: in.OccurredAt, Note: in.Note,
		}))
		if err != nil {
			return err
		}
		out = []bank.Transaction{toTransaction(debit), toTransaction(credit)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (a *Adapter) ListTransferLegs(ctx context.Context, userID, transferID uuid.UUID) ([]bank.Transaction, error) {
	rows, err := a.q.ListTransferLegs(ctx, ListTransferLegsParams{TransferID: pgUUID(transferID), UserID: pgUUID(userID)})
	if err != nil {
		return nil, err
	}
	return toTransactions(rows), nil
}

func (a *Adapter) UpdateTransfer(ctx context.Context, in bank.UpdateTransferInput) ([]bank.Transaction, error) {
	var out []bank.Transaction
	err := a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		legs, err := q.ListTransferLegs(ctx, ListTransferLegsParams{TransferID: pgUUID(in.TransferID), UserID: pgUUID(in.UserID)})
		if err != nil {
			return err
		}
		if len(legs) == 0 {
			return bank.ErrTransactionNotFound
		}
		amt := in.Amount
		out = make([]bank.Transaction, 0, len(legs))
		for _, leg := range legs {
			acct := in.FromAccount
			if leg.Direction == bank.DirCredit {
				acct = in.ToAccount
			}
			updated, err := q.UpdateTransaction(ctx, UpdateTransactionParams{
				AccountID:  pgUUID(acct),
				Amount:     &amt,
				OccurredAt: pgDate(in.OccurredAt),
				Note:       in.Note,
				ID:         leg.ID,
				UserID:     pgUUID(in.UserID),
			})
			if err != nil {
				return err
			}
			out = append(out, toTransaction(updated))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (a *Adapter) DeleteTransfer(ctx context.Context, userID, transferID uuid.UUID) error {
	return a.q.DeleteTransferByID(ctx, DeleteTransferByIDParams{TransferID: pgUUID(transferID), UserID: pgUUID(userID)})
}

// ── budgets ──────────────────────────────────────────────────────────

func (a *Adapter) UpsertBudget(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount int64) error {
	_, err := a.q.UpsertBudget(ctx, UpsertBudgetParams{
		UserID:     pgUUID(userID),
		CategoryID: pgUUID(categoryID),
		Month:      pgDate(month),
		Amount:     amount,
	})
	return err
}

func (a *Adapter) DeleteBudget(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error {
	return a.q.DeleteBudget(ctx, DeleteBudgetParams{
		UserID:     pgUUID(userID),
		CategoryID: pgUUID(categoryID),
		Month:      pgDate(month),
	})
}

func (a *Adapter) ListBudgetsForMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]bank.BudgetLine, error) {
	rows, err := a.q.ListBudgetsForMonth(ctx, ListBudgetsForMonthParams{UserID: pgUUID(userID), Month: pgDate(month)})
	if err != nil {
		return nil, err
	}
	out := make([]bank.BudgetLine, 0, len(rows))
	for _, r := range rows {
		out = append(out, bank.BudgetLine{
			CategoryID: uuidFrom(r.CategoryID),
			ParentID:   uuidPtr(r.ParentID),
			Name:       r.Name,
			ParentName: r.ParentName,
			Amount:     r.BudgetAmount,
			Spent:      r.Spent,
		})
	}
	return out, nil
}

// ── dashboard ────────────────────────────────────────────────────────

func (a *Adapter) MonthFlowTotals(ctx context.Context, userID uuid.UUID, month time.Time) (int64, int64, error) {
	row, err := a.q.MonthFlowTotals(ctx, MonthFlowTotalsParams{UserID: pgUUID(userID), OccurredAt: pgDate(month)})
	if err != nil {
		return 0, 0, err
	}
	return row.Income, row.Expense, nil
}

// ── mapping helpers ──────────────────────────────────────────────────

func createTxParams(in bank.CreateTransactionInput) CreateTransactionParams {
	return CreateTransactionParams{
		UserID:     pgUUID(in.UserID),
		AccountID:  pgUUID(in.AccountID),
		CategoryID: optUUID(in.CategoryID),
		Amount:     in.Amount,
		Direction:  in.Direction,
		TransferID: optUUID(in.TransferID),
		OccurredAt: pgDate(in.OccurredAt),
		Note:       in.Note,
	}
}

func toAccount(r BankAccount) bank.Account {
	return bank.Account{
		ID:             uuidFrom(r.ID),
		Name:           r.Name,
		Type:           r.Type,
		Currency:       r.Currency,
		OpeningBalance: r.OpeningBalance,
		Archived:       r.Archived,
		CreatedAt:      r.CreatedAt.Time,
	}
}

func toCategory(r BankCategory) bank.Category {
	return bank.Category{
		ID:       uuidFrom(r.ID),
		ParentID: uuidPtr(r.ParentID),
		Name:     r.Name,
		Kind:     r.Kind,
		Seed:     !r.UserID.Valid,
	}
}

func toTransactions(rows []BankTransaction) []bank.Transaction {
	out := make([]bank.Transaction, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTransaction(r))
	}
	return out
}

func toTransaction(r BankTransaction) bank.Transaction {
	return bank.Transaction{
		ID:         uuidFrom(r.ID),
		AccountID:  uuidFrom(r.AccountID),
		CategoryID: uuidPtr(r.CategoryID),
		Amount:     r.Amount,
		Direction:  r.Direction,
		TransferID: uuidPtr(r.TransferID),
		OccurredAt: r.OccurredAt.Time,
		Note:       r.Note,
		CreatedAt:  r.CreatedAt.Time,
		UpdatedAt:  r.UpdatedAt.Time,
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

func optUUID(p *uuid.UUID) pgtype.UUID {
	if p == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *p, Valid: true}
}

func pgDate(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

func optDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}
