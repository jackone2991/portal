package bankrepo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/bank"
)

// Debt persistence (SPEC-10 phase 1, migration 0043). Kept in its own file so
// the feature adds nothing to adapter.go.

func (a *Adapter) CreateDebt(ctx context.Context, userID, accountID uuid.UUID, in bank.CreateDebtInput) (bank.Debt, error) {
	row, err := a.q.CreateDebt(ctx, CreateDebtParams{
		UserID:          pgUUID(userID),
		AccountID:       pgUUID(accountID),
		Counterparty:    in.Counterparty,
		Direction:       in.Direction,
		Principal:       in.Principal,
		InterestRateBps: in.InterestRateBps,
		InterestMethod:  in.InterestMethod,
		OpenedOn:        pgDate(in.OpenedOn),
		DueOn:           optDate(in.DueOn),
		Note:            in.Note,
	})
	if err != nil {
		return bank.Debt{}, err
	}
	return toDebt(row), nil
}

func (a *Adapter) GetDebt(ctx context.Context, userID, id uuid.UUID) (bank.Debt, error) {
	row, err := a.q.GetDebt(ctx, GetDebtParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Debt{}, bank.ErrDebtNotFound
		}
		return bank.Debt{}, err
	}
	return toDebt(row), nil
}

func (a *Adapter) ListDebts(ctx context.Context, userID uuid.UUID) ([]bank.Debt, error) {
	rows, err := a.q.ListDebts(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]bank.Debt, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDebt(r))
	}
	return out, nil
}

func (a *Adapter) UpdateDebt(ctx context.Context, in bank.UpdateDebtInput) (bank.Debt, error) {
	row, err := a.q.UpdateDebt(ctx, UpdateDebtParams{
		ID:              pgUUID(in.ID),
		UserID:          pgUUID(in.UserID),
		Counterparty:    in.Counterparty,
		InterestRateBps: in.InterestRateBps,
		InterestMethod:  in.InterestMethod,
		SetDueOn:        in.SetDueOn,
		DueOn:           optDate(in.DueOn),
		SetNote:         in.SetNote,
		Note:            in.Note,
		SetClosed:       in.SetClosed,
		ClosedAt:        optTimestamptz(in.ClosedAt),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bank.Debt{}, bank.ErrDebtNotFound
		}
		return bank.Debt{}, err
	}
	return toDebt(row), nil
}

func (a *Adapter) DeleteDebt(ctx context.Context, userID, id uuid.UUID) (uuid.UUID, error) {
	accID, err := a.q.DeleteDebt(ctx, DeleteDebtParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, bank.ErrDebtNotFound
		}
		return uuid.Nil, err
	}
	return uuidFrom(accID), nil
}

// AccountBalance is the derived balance of the debt's own account — the same
// arithmetic a wallet's balance uses, so the two can never disagree about what
// a movement did.
func (a *Adapter) AccountBalance(ctx context.Context, accountID uuid.UUID) (int64, error) {
	return a.q.DebtOutstanding(ctx, pgUUID(accountID))
}

// LastAccrualOn returns nil when interest has never been posted on this debt.
func (a *Adapter) LastAccrualOn(ctx context.Context, accountID uuid.UUID) (*time.Time, error) {
	d, err := a.q.LastAccrualOn(ctx, pgUUID(accountID))
	if err != nil {
		return nil, err
	}
	if !d.Valid {
		return nil, nil
	}
	t := d.Time
	return &t, nil
}

// InterestCategory resolves the seeded category an accrual attaches to: the
// 0043 'Lãi vay' expense for money owed, 0042's 'Lãi' income for money lent.
func (a *Adapter) InterestCategory(ctx context.Context, kind string) (uuid.UUID, error) {
	if kind == bank.KindIncome {
		id, err := a.q.SeedIncomeInterestCategory(ctx)
		if err != nil {
			return uuid.Nil, err
		}
		return uuidFrom(id), nil
	}
	id, err := a.q.SeedInterestCategory(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return uuidFrom(id), nil
}

/* ── reminder sweep (used by the worker, not by a request) ────────── */

func (a *Adapter) DebtsDueBetween(ctx context.Context, from, to time.Time, leadDays int32) ([]bank.Debt, error) {
	rows, err := a.q.DebtsDueBetween(ctx, DebtsDueBetweenParams{
		FromOn:   pgDate(from),
		ToOn:     pgDate(to),
		LeadDays: leadDays,
	})
	if err != nil {
		return nil, err
	}
	out := make([]bank.Debt, 0, len(rows))
	for _, r := range rows {
		out = append(out, toDebt(r))
	}
	return out, nil
}

func (a *Adapter) MarkDebtReminded(ctx context.Context, debtID uuid.UUID, dueOn time.Time, leadDays int32) error {
	return a.q.MarkDebtReminded(ctx, MarkDebtRemindedParams{
		DebtID:   pgUUID(debtID),
		DueOn:    pgDate(dueOn),
		LeadDays: leadDays,
	})
}

func toDebt(r BankDebt) bank.Debt {
	d := bank.Debt{
		ID:              uuidFrom(r.ID),
		UserID:          uuidFrom(r.UserID),
		AccountID:       uuidFrom(r.AccountID),
		Counterparty:    r.Counterparty,
		Direction:       r.Direction,
		Principal:       r.Principal,
		InterestRateBps: r.InterestRateBps,
		InterestMethod:  r.InterestMethod,
		OpenedOn:        r.OpenedOn.Time,
		Note:            r.Note,
		CreatedAt:       r.CreatedAt.Time,
		UpdatedAt:       r.UpdatedAt.Time,
	}
	if r.DueOn.Valid {
		t := r.DueOn.Time
		d.DueOn = &t
	}
	if r.ClosedAt.Valid {
		t := r.ClosedAt.Time
		d.ClosedAt = &t
	}
	return d
}

func optTimestamptz(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}
