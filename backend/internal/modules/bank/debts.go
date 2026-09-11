package bank

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	notifyapi "github.com/portal/backend/internal/modules/notify/api"
)

// Debts and loans — SPEC-10 phase 1, migration 0043.
//
// The money is not in this file. A debt owns a `bank_accounts` row, and every
// principal movement is an ordinary transfer between that account and a wallet.
// That is the whole design: reporting already excludes pure transfer legs
// (SPEC-03 P0.3), so borrowing cannot be counted as income and repaying cannot
// be counted as an expense — by the predicate that already exists, in every
// query that already uses it.
//
// Interest is the deliberate exception. It IS a flow: money you owe because
// time passed. So an accrual is a normal categorised transaction and shows up
// in the donut, exactly like a transfer fee does.

var (
	ErrDebtNotFound    = errors.New("bank: debt not found")
	ErrDebtNotEmpty    = errors.New("bank: debt still has an outstanding balance")
	ErrDebtClosed      = errors.New("bank: debt is closed")
	ErrDebtNoInterest  = errors.New("bank: debt has no interest terms")
	ErrNothingToAccrue = errors.New("bank: no interest has accrued since the last posting")
)

const (
	DirBorrowed = "borrowed" // I owe them
	DirLent     = "lent"     // they owe me

	InterestNone     = "none"
	InterestSimple   = "simple"
	InterestCompound = "compound"

	// Account types the ledger uses for the two sides. Deliberately absent from
	// `accountTypes`: a liability account without terms beside it would have a
	// balance nobody can explain, so these are creatable only through a debt.
	AccountLoanPayable    = "loan_payable"
	AccountLoanReceivable = "loan_receivable"
)

// Movement kinds, in the user's words rather than the ledger's.
const (
	MoveBorrow  = "borrow"  // principal in  (direction=borrowed)
	MoveRepay   = "repay"   // principal out (direction=borrowed)
	MoveLend    = "lend"    // principal out (direction=lent)
	MoveCollect = "collect" // principal in  (direction=lent)
)

type Debt struct {
	ID uuid.UUID
	// UserID is who the debt belongs to. A request already knows it; the
	// reminder sweep does not, because it walks a tenant rather than a caller.
	UserID          uuid.UUID
	AccountID       uuid.UUID
	Counterparty    string
	Direction       string
	Principal       int64
	InterestRateBps int32
	InterestMethod  string
	OpenedOn        time.Time
	DueOn           *time.Time
	ClosedAt        *time.Time
	Note            *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// DebtView is a Debt plus the numbers the UI cannot compute without the ledger.
type DebtView struct {
	Debt
	// Outstanding is what is still owed, normalised to a positive number for
	// both directions — the raw account balance is negative for money you owe.
	Outstanding int64
	// Settled is principal − outstanding: "đã trả 6/10 triệu". It can exceed the
	// principal once interest has been accrued and paid, which is correct.
	Settled int64
	// ProjectedInterest is what would accrue between the last movement and
	// DueOn, under the debt's own terms. Zero when there are none, or no due
	// date to project to. Never written anywhere — it is a preview.
	ProjectedInterest int64
}

type CreateDebtInput struct {
	UserID          uuid.UUID
	Counterparty    string
	Direction       string
	Principal       int64
	InterestRateBps int32
	InterestMethod  string
	OpenedOn        time.Time
	DueOn           *time.Time
	Note            *string
	// WalletID is the account the principal moves to or from. Required: a debt
	// without the movement that created it is a number with no history.
	WalletID uuid.UUID
	Currency string
}

type UpdateDebtInput struct {
	UserID          uuid.UUID
	ID              uuid.UUID
	Counterparty    *string
	InterestRateBps *int32
	InterestMethod  *string
	SetDueOn        bool
	DueOn           *time.Time
	SetNote         bool
	Note            *string
	SetClosed       bool
	ClosedAt        *time.Time
}

type DebtMovementInput struct {
	UserID     uuid.UUID
	DebtID     uuid.UUID
	Kind       string
	WalletID   uuid.UUID
	Amount     int64
	OccurredAt time.Time
	Note       *string
}

// DebtRepository is the debt persistence surface. The same adapter implements
// it; a separate interface keeps the feature in its own files.
type DebtRepository interface {
	CreateDebt(ctx context.Context, userID, accountID uuid.UUID, in CreateDebtInput) (Debt, error)
	GetDebt(ctx context.Context, userID, id uuid.UUID) (Debt, error)
	ListDebts(ctx context.Context, userID uuid.UUID) ([]Debt, error)
	UpdateDebt(ctx context.Context, in UpdateDebtInput) (Debt, error)
	DeleteDebt(ctx context.Context, userID, id uuid.UUID) (uuid.UUID, error)
	AccountBalance(ctx context.Context, accountID uuid.UUID) (int64, error)
	// LastAccrualOn is nil until interest has been posted once.
	LastAccrualOn(ctx context.Context, accountID uuid.UUID) (*time.Time, error)
	InterestCategory(ctx context.Context, kind string) (uuid.UUID, error)

	// Reminder sweep — called by the worker inside one tenant's scope, never
	// from a request, so neither takes a user id: RLS is the fence.
	DebtsDueBetween(ctx context.Context, from, to time.Time, leadDays int32) ([]Debt, error)
	MarkDebtReminded(ctx context.Context, debtID uuid.UUID, dueOn time.Time, leadDays int32) error
}

/* ── interest ─────────────────────────────────────────────────────── */

// accrueInterest returns the interest, in minor units, that accumulates on
// `outstanding` over `days` at an annual rate of `bps` basis points.
//
//   - simple:   outstanding × r × days/365
//   - compound: outstanding × ((1 + r/365)^days − 1) — daily compounding, which
//     is what consumer lenders in this market actually quote.
//
// Money is integer minor units everywhere in this ledger (D-41); the float is
// confined to this function and rounded once, at the end.
func accrueInterest(outstanding int64, bps int32, method string, days int) int64 {
	if outstanding <= 0 || bps <= 0 || days <= 0 {
		return 0
	}
	r := float64(bps) / 10_000.0
	var growth float64
	switch method {
	case InterestSimple:
		growth = r * float64(days) / 365.0
	case InterestCompound:
		growth = math.Pow(1+r/365.0, float64(days)) - 1
	default:
		return 0
	}
	return int64(math.Round(float64(outstanding) * growth))
}

// outstandingFrom normalises a raw account balance into "how much is still
// owed". Borrowing drives the liability account negative, so the sign flips;
// lending drives it positive and does not.
func outstandingFrom(direction string, balance int64) int64 {
	if direction == DirBorrowed {
		return -balance
	}
	return balance
}

/* ── service ──────────────────────────────────────────────────────── */

func (s *Service) ListDebts(ctx context.Context, userID uuid.UUID) ([]DebtView, error) {
	if s.debts == nil {
		return nil, nil
	}
	rows, err := s.debts.ListDebts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]DebtView, 0, len(rows))
	for _, d := range rows {
		v, err := s.viewOf(ctx, d)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (s *Service) GetDebt(ctx context.Context, userID, id uuid.UUID) (DebtView, error) {
	if s.debts == nil {
		return DebtView{}, ErrDebtNotFound
	}
	d, err := s.debts.GetDebt(ctx, userID, id)
	if err != nil {
		return DebtView{}, err
	}
	return s.viewOf(ctx, d)
}

func (s *Service) viewOf(ctx context.Context, d Debt) (DebtView, error) {
	bal, err := s.debts.AccountBalance(ctx, d.AccountID)
	if err != nil {
		return DebtView{}, err
	}
	out := outstandingFrom(d.Direction, bal)
	v := DebtView{Debt: d, Outstanding: out, Settled: d.Principal - out}
	if d.DueOn != nil && d.InterestMethod != InterestNone && d.ClosedAt == nil {
		// From the same start the posting would use, so the preview and the
		// charge never disagree.
		since := d.OpenedOn
		if last, err := s.debts.LastAccrualOn(ctx, d.AccountID); err == nil && last != nil && last.After(since) {
			since = *last
		}
		days := int(d.DueOn.Sub(since).Hours() / 24)
		v.ProjectedInterest = accrueInterest(out, d.InterestRateBps, d.InterestMethod, days)
	}
	return v, nil
}

// CreateDebt opens the liability/asset account, records the terms, and posts the
// principal movement in one go. The movement is not optional: a debt you cannot
// see in the ledger is a debt the ledger cannot reconcile.
func (s *Service) CreateDebt(ctx context.Context, in CreateDebtInput) (DebtView, error) {
	if s.debts == nil {
		return DebtView{}, ErrDebtNotFound
	}
	in.Counterparty = strings.TrimSpace(in.Counterparty)
	if in.Counterparty == "" || len([]rune(in.Counterparty)) > 120 {
		return DebtView{}, ErrValidation
	}
	if in.Direction != DirBorrowed && in.Direction != DirLent {
		return DebtView{}, ErrValidation
	}
	if in.Principal <= 0 {
		return DebtView{}, ErrInvalidAmount
	}
	switch in.InterestMethod {
	case "", InterestNone:
		in.InterestMethod, in.InterestRateBps = InterestNone, 0
	case InterestSimple, InterestCompound:
		if in.InterestRateBps <= 0 {
			return DebtView{}, ErrValidation // 0043's CHECK would refuse it anyway
		}
	default:
		return DebtView{}, ErrValidation
	}
	if in.OpenedOn.IsZero() {
		return DebtView{}, ErrValidation
	}

	// The wallet must exist and be the caller's — and it fixes the currency, so
	// the debt account cannot be opened in one the transfer would then refuse.
	wallet, err := s.repo.GetAccount(ctx, in.UserID, in.WalletID)
	if err != nil {
		return DebtView{}, err
	}
	in.Currency = wallet.Currency

	accType := AccountLoanPayable
	if in.Direction == DirLent {
		accType = AccountLoanReceivable
	}
	// Straight to the repository: `accountTypes` deliberately excludes the loan
	// types so the wallet endpoint cannot mint one without terms.
	acc, err := s.repo.CreateAccount(ctx, CreateAccountInput{
		UserID:   in.UserID,
		Name:     in.Counterparty,
		Type:     accType,
		Currency: in.Currency,
	})
	if err != nil {
		return DebtView{}, err
	}

	debt, err := s.debts.CreateDebt(ctx, in.UserID, acc.ID, in)
	if err != nil {
		return DebtView{}, err
	}

	if _, err := s.moveDebt(ctx, in.UserID, debt, principalKind(in.Direction), in.WalletID, in.Principal, in.OpenedOn, nil); err != nil {
		return DebtView{}, err
	}
	return s.GetDebt(ctx, in.UserID, debt.ID)
}

func principalKind(direction string) string {
	if direction == DirBorrowed {
		return MoveBorrow
	}
	return MoveLend
}

func (s *Service) UpdateDebt(ctx context.Context, in UpdateDebtInput) (DebtView, error) {
	if s.debts == nil {
		return DebtView{}, ErrDebtNotFound
	}
	if in.Counterparty != nil {
		t := strings.TrimSpace(*in.Counterparty)
		if t == "" || len([]rune(t)) > 120 {
			return DebtView{}, ErrValidation
		}
		in.Counterparty = &t
	}
	if in.InterestMethod != nil {
		switch *in.InterestMethod {
		case InterestNone, InterestSimple, InterestCompound:
		default:
			return DebtView{}, ErrValidation
		}
	}
	d, err := s.debts.UpdateDebt(ctx, in)
	if err != nil {
		return DebtView{}, err
	}
	return s.viewOf(ctx, d)
}

// DeleteDebt refuses while anything is still owed. Deleting cascades the
// account and with it every movement — which for a settled debt is the user
// tidying up, and for a live one is destroying evidence of money.
func (s *Service) DeleteDebt(ctx context.Context, userID, id uuid.UUID) error {
	if s.debts == nil {
		return ErrDebtNotFound
	}
	v, err := s.GetDebt(ctx, userID, id)
	if err != nil {
		return err
	}
	if v.Outstanding != 0 {
		return ErrDebtNotEmpty
	}
	_, err = s.debts.DeleteDebt(ctx, userID, id)
	return err
}

// AddMovement records a repayment or a collection (or, internally, the opening
// principal) as a transfer between the wallet and the debt's account.
func (s *Service) AddMovement(ctx context.Context, in DebtMovementInput) ([]Transaction, error) {
	if s.debts == nil {
		return nil, ErrDebtNotFound
	}
	d, err := s.debts.GetDebt(ctx, in.UserID, in.DebtID)
	if err != nil {
		return nil, err
	}
	if d.ClosedAt != nil {
		return nil, ErrDebtClosed
	}
	return s.moveDebt(ctx, in.UserID, d, in.Kind, in.WalletID, in.Amount, in.OccurredAt, in.Note)
}

func (s *Service) moveDebt(
	ctx context.Context, userID uuid.UUID, d Debt,
	kind string, walletID uuid.UUID, amount int64, on time.Time, note *string,
) ([]Transaction, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}
	// A kind that does not belong to this debt's direction is a bug in the
	// caller, not a transfer to attempt: collecting on money you borrowed would
	// move the balance the wrong way and quietly invent an asset.
	var from, to uuid.UUID
	switch {
	case kind == MoveBorrow && d.Direction == DirBorrowed:
		from, to = d.AccountID, walletID
	case kind == MoveRepay && d.Direction == DirBorrowed:
		from, to = walletID, d.AccountID
	case kind == MoveLend && d.Direction == DirLent:
		from, to = walletID, d.AccountID
	case kind == MoveCollect && d.Direction == DirLent:
		from, to = d.AccountID, walletID
	default:
		return nil, ErrValidation
	}
	if on.IsZero() {
		on = time.Now().UTC()
	}
	// Through the ordinary transfer path: it checks currency, refuses a
	// same-account move, and emits the same events the rest of the ledger does.
	return s.CreateTransfer(ctx, TransferParams{
		UserID:      userID,
		FromAccount: from,
		ToAccount:   to,
		Amount:      amount,
		OccurredAt:  on,
		Note:        note,
	})
}

// AccrueInterest posts interest as a REAL transaction, categorised, so it lands
// in the month's expense (or income) totals like any other flow. An accrual you
// cannot see in the donut is an accrual you cannot reconcile.
//
// It moves the debt account further from zero without touching a wallet: no
// cash changed hands, the obligation simply grew.
func (s *Service) AccrueInterest(ctx context.Context, userID, debtID uuid.UUID, upTo time.Time) (Transaction, error) {
	if s.debts == nil {
		return Transaction{}, ErrDebtNotFound
	}
	d, err := s.debts.GetDebt(ctx, userID, debtID)
	if err != nil {
		return Transaction{}, err
	}
	if d.ClosedAt != nil {
		return Transaction{}, ErrDebtClosed
	}
	if d.InterestMethod == InterestNone || d.InterestRateBps <= 0 {
		return Transaction{}, ErrDebtNoInterest
	}
	bal, err := s.debts.AccountBalance(ctx, d.AccountID)
	if err != nil {
		return Transaction{}, err
	}
	out := outstandingFrom(d.Direction, bal)
	if out <= 0 {
		return Transaction{}, ErrDebtNotEmpty // nothing owed: nothing to charge
	}
	if upTo.IsZero() {
		upTo = time.Now().UTC()
	}
	// Charge from the LAST accrual, not from the opening date: accruing a second
	// time must not bill the period the first one already covered. On a debt's
	// own account a categorised transaction can only be an accrual, so the
	// ledger itself carries the answer and nothing needs storing.
	since := d.OpenedOn
	last, err := s.debts.LastAccrualOn(ctx, d.AccountID)
	if err != nil {
		return Transaction{}, err
	}
	if last != nil && last.After(since) {
		since = *last
	}
	days := int(upTo.Sub(since).Hours() / 24)
	amount := accrueInterest(out, d.InterestRateBps, d.InterestMethod, days)
	if amount <= 0 {
		// Almost always "you already accrued up to this date". Saying so beats
		// "amount must be a positive integer" for a form with no amount field.
		return Transaction{}, ErrNothingToAccrue
	}

	// Interest I owe is an expense; interest owed to me is income. Both land on
	// a seeded category, so the donut has somewhere to put them.
	kind, dir := KindExpense, "debit"
	if d.Direction == DirLent {
		kind, dir = KindIncome, "credit"
	}
	catID, err := s.debts.InterestCategory(ctx, kind)
	if err != nil {
		return Transaction{}, err
	}
	return s.CreateTransaction(ctx, CreateTransactionInput{
		UserID:     userID,
		AccountID:  d.AccountID,
		CategoryID: &catID,
		Amount:     amount,
		Direction:  dir,
		OccurredAt: upTo,
		Note:       strPtr("Lãi " + d.Counterparty),
	})
}

func strPtr(s string) *string { return &s }

/* ── due-date reminders ───────────────────────────────────────────── */

// reminderLeads are the days-before-due a debt is announced at. Three is a
// judgement, not a setting: a week to arrange the money, a day to remember, and
// the day itself. Each fires at most once per (debt, due date, lead).
var reminderLeads = []int32{7, 1, 0}

// ScanDueDebts is the daily sweep behind "nhắc nợ". It runs inside ONE tenant's
// scope — the worker calls it once per tenant — so it takes no user id and
// relies on RLS for the fence, exactly as a request would.
//
// It is idempotent twice over: bank_debt_reminders stops the work being redone,
// and the notification carries a DedupKey so an at-least-once redelivery is a
// no-op at the store as well.
func (s *Service) ScanDueDebts(ctx context.Context, now time.Time) error {
	if s.debts == nil || s.notify == nil {
		return nil
	}
	today := now.UTC().Truncate(24 * time.Hour)
	for _, lead := range reminderLeads {
		on := today.AddDate(0, 0, int(lead))
		due, err := s.debts.DebtsDueBetween(ctx, on, on, lead)
		if err != nil {
			return err
		}
		for _, d := range due {
			v, err := s.viewOf(ctx, d)
			if err != nil {
				return err
			}
			// A debt already paid off but not closed should not nag.
			if v.Outstanding <= 0 {
				continue
			}
			if err := s.notifyDue(ctx, v, lead); err != nil {
				return err
			}
			if err := s.debts.MarkDebtReminded(ctx, d.ID, *d.DueOn, lead); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) notifyDue(ctx context.Context, v DebtView, lead int32) error {
	when := "hôm nay"
	switch lead {
	case 1:
		when = "ngày mai"
	case 7:
		when = "trong 7 ngày"
	}
	title := "Đến hạn trả nợ " + when
	if v.Direction == DirLent {
		title = "Đến hạn thu nợ " + when
	}
	return notifyapi.Enqueue(ctx, s.notify, notifyapi.NotificationIntent{
		UserID: v.UserID,
		Type:   "bank.debt_due",
		Title:  title,
		Body:   v.Counterparty,
		Data: map[string]any{
			"debt_id":      v.ID,
			"counterparty": v.Counterparty,
			"outstanding":  v.Outstanding,
			"due_on":       v.DueOn.Format("2006-01-02"),
			"direction":    v.Direction,
		},
		// (debt, due date, lead) — the same triple the reminders table keys on,
		// so the two guards agree on what "already sent" means.
		DedupKey: v.ID.String() + "|" + v.DueOn.Format("2006-01-02") + "|" + itoa(lead),
	})
}

func itoa(n int32) string { return strconv.Itoa(int(n)) }
