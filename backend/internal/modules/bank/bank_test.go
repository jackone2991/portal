package bank

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
)

// ── in-memory fake Repository ────────────────────────────────────────

type fAccount struct {
	id, userID uuid.UUID
	name, typ  string
	currency   string
	opening    int64
	archived   bool
	createdAt  time.Time
}

type fCategory struct {
	id       uuid.UUID
	userID   *uuid.UUID // nil = seed
	parentID *uuid.UUID
	name     string
	kind     string
}

type fTxn struct {
	id, userID, accountID  uuid.UUID
	categoryID, transferID *uuid.UUID
	amount                 int64
	direction              string
	occurredAt             time.Time
	note                   *string
	createdAt, updatedAt   time.Time
}

type fakeRepo struct {
	accounts map[uuid.UUID]*fAccount
	cats     map[uuid.UUID]*fCategory
	txns     map[uuid.UUID]*fTxn
	budgets  map[string]int64
	clock    int64
}

func newFake() *fakeRepo {
	return &fakeRepo{
		accounts: map[uuid.UUID]*fAccount{},
		cats:     map[uuid.UUID]*fCategory{},
		txns:     map[uuid.UUID]*fTxn{},
		budgets:  map[string]int64{},
	}
}

func (r *fakeRepo) tick() time.Time { r.clock++; return time.Unix(r.clock, 0).UTC() }

// seedCategory adds a user_id-NULL default (test helper).
func (r *fakeRepo) seedCategory(name, kind string, parent *uuid.UUID) uuid.UUID {
	id := uuid.New()
	r.cats[id] = &fCategory{id: id, userID: nil, parentID: parent, name: name, kind: kind}
	return id
}

func budgetKey(user, cat uuid.UUID, month time.Time) string {
	return user.String() + "|" + cat.String() + "|" + month.Format("2006-01")
}

func sameMonth(a, b time.Time) bool {
	return a.Year() == b.Year() && a.Month() == b.Month()
}

// accounts
func (r *fakeRepo) CreateAccount(_ context.Context, in CreateAccountInput) (Account, error) {
	id := uuid.New()
	r.accounts[id] = &fAccount{id: id, userID: in.UserID, name: in.Name, typ: in.Type, currency: in.Currency, opening: in.OpeningBalance, createdAt: r.tick()}
	return r.accountDomain(r.accounts[id]), nil
}
func (r *fakeRepo) accountDomain(a *fAccount) Account {
	return Account{ID: a.id, Name: a.name, Type: a.typ, Currency: a.currency, OpeningBalance: a.opening, Archived: a.archived, CreatedAt: a.createdAt}
}
func (r *fakeRepo) GetAccount(_ context.Context, userID, id uuid.UUID) (Account, error) {
	a, ok := r.accounts[id]
	if !ok || a.userID != userID {
		return Account{}, ErrAccountNotFound
	}
	return r.accountDomain(a), nil
}
func (r *fakeRepo) balanceOf(accountID uuid.UUID) int64 {
	a := r.accounts[accountID]
	bal := a.opening
	for _, t := range r.txns {
		if t.accountID != accountID {
			continue
		}
		if t.direction == DirCredit {
			bal += t.amount
		} else {
			bal -= t.amount
		}
	}
	return bal
}
func (r *fakeRepo) ListAccountBalances(_ context.Context, userID uuid.UUID) ([]Account, error) {
	var out []Account
	for _, a := range r.accounts {
		if a.userID != userID {
			continue
		}
		d := r.accountDomain(a)
		d.Balance = r.balanceOf(a.id)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (r *fakeRepo) UpdateAccount(_ context.Context, in UpdateAccountInput) (Account, error) {
	a, ok := r.accounts[in.ID]
	if !ok || a.userID != in.UserID {
		return Account{}, ErrAccountNotFound
	}
	if in.Name != nil {
		a.name = *in.Name
	}
	if in.Archived != nil {
		a.archived = *in.Archived
	}
	if in.Currency != nil {
		a.currency = *in.Currency
	}
	return r.accountDomain(a), nil
}
func (r *fakeRepo) DeleteAccount(_ context.Context, userID, id uuid.UUID) error {
	a, ok := r.accounts[id]
	if !ok || a.userID != userID {
		return ErrAccountNotFound
	}
	delete(r.accounts, id)
	return nil
}
func (r *fakeRepo) CountAccountTransactions(_ context.Context, id uuid.UUID) (int64, error) {
	var n int64
	for _, t := range r.txns {
		if t.accountID == id {
			n++
		}
	}
	return n, nil
}

// categories
func (r *fakeRepo) CreateCategory(_ context.Context, in CreateCategoryInput) (Category, error) {
	id := uuid.New()
	uid := in.UserID
	r.cats[id] = &fCategory{id: id, userID: &uid, parentID: in.ParentID, name: in.Name, kind: in.Kind}
	return r.catDomain(r.cats[id]), nil
}
func (r *fakeRepo) catDomain(c *fCategory) Category {
	return Category{ID: c.id, ParentID: c.parentID, Name: c.name, Kind: c.kind, Seed: c.userID == nil}
}
func (r *fakeRepo) visible(userID, id uuid.UUID) (*fCategory, bool) {
	c, ok := r.cats[id]
	if !ok {
		return nil, false
	}
	if c.userID == nil || *c.userID == userID {
		return c, true
	}
	return nil, false
}
func (r *fakeRepo) GetVisibleCategory(_ context.Context, userID, id uuid.UUID) (Category, error) {
	c, ok := r.visible(userID, id)
	if !ok {
		return Category{}, ErrCategoryNotFound
	}
	return r.catDomain(c), nil
}
func (r *fakeRepo) ListCategories(_ context.Context, userID uuid.UUID) ([]Category, error) {
	var out []Category
	for _, c := range r.cats {
		if c.userID == nil || *c.userID == userID {
			out = append(out, r.catDomain(c))
		}
	}
	return out, nil
}
func (r *fakeRepo) UpdateCategory(_ context.Context, in UpdateCategoryInput) (Category, error) {
	c, ok := r.cats[in.ID]
	if !ok || c.userID == nil || *c.userID != in.UserID {
		return Category{}, ErrCategoryNotFound
	}
	if in.Name != nil {
		c.name = *in.Name
	}
	if in.SetParent {
		c.parentID = in.ParentID
	}
	return r.catDomain(c), nil
}
func (r *fakeRepo) CountCategoryTransactions(_ context.Context, userID, id uuid.UUID) (int64, error) {
	var n int64
	for _, t := range r.txns {
		if t.userID == userID && t.categoryID != nil && *t.categoryID == id {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) CountCategoryChildren(_ context.Context, id uuid.UUID) (int64, error) {
	var n int64
	for _, c := range r.cats {
		if c.parentID != nil && *c.parentID == id {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) DeleteCategory(_ context.Context, userID, id uuid.UUID, reassignTo *uuid.UUID) error {
	c, ok := r.cats[id]
	if !ok || c.userID == nil || *c.userID != userID {
		return ErrCategoryNotFound
	}
	if reassignTo != nil {
		for _, t := range r.txns {
			if t.userID == userID && t.categoryID != nil && *t.categoryID == id {
				t.categoryID = reassignTo
			}
		}
	}
	for _, ch := range r.cats { // promote children (ON DELETE SET NULL)
		if ch.parentID != nil && *ch.parentID == id {
			ch.parentID = nil
		}
	}
	for k := range r.budgets { // cascade budgets on the deleted category
		if len(k) >= 0 && containsCat(k, id) {
			delete(r.budgets, k)
		}
	}
	delete(r.cats, id)
	return nil
}
func containsCat(key string, id uuid.UUID) bool {
	return len(key) > 0 && (indexOf(key, id.String()) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// transactions
func (r *fakeRepo) CreateTransaction(_ context.Context, in CreateTransactionInput) (Transaction, error) {
	id := uuid.New()
	now := r.tick()
	t := &fTxn{id: id, userID: in.UserID, accountID: in.AccountID, categoryID: in.CategoryID, transferID: in.TransferID,
		amount: in.Amount, direction: in.Direction, occurredAt: in.OccurredAt, note: in.Note, createdAt: now, updatedAt: now}
	r.txns[id] = t
	return r.txnDomain(t), nil
}
func (r *fakeRepo) txnDomain(t *fTxn) Transaction {
	return Transaction{ID: t.id, AccountID: t.accountID, CategoryID: t.categoryID, Amount: t.amount, Direction: t.direction,
		TransferID: t.transferID, OccurredAt: t.occurredAt, Note: t.note, CreatedAt: t.createdAt, UpdatedAt: t.updatedAt}
}
func (r *fakeRepo) GetTransaction(_ context.Context, userID, id uuid.UUID) (Transaction, error) {
	t, ok := r.txns[id]
	if !ok || t.userID != userID {
		return Transaction{}, ErrTransactionNotFound
	}
	return r.txnDomain(t), nil
}
func (r *fakeRepo) ListTransactions(_ context.Context, in TxListInput) ([]Transaction, error) {
	var out []Transaction
	for _, t := range r.txns {
		if t.userID != in.UserID {
			continue
		}
		if in.AccountID != nil && t.accountID != *in.AccountID {
			continue
		}
		if in.CategoryID != nil && (t.categoryID == nil || *t.categoryID != *in.CategoryID) {
			continue
		}
		if in.Month != nil && !sameMonth(t.occurredAt, *in.Month) {
			continue
		}
		out = append(out, r.txnDomain(t))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].OccurredAt.Equal(out[j].OccurredAt) {
			return out[i].OccurredAt.After(out[j].OccurredAt)
		}
		return out[i].ID.String() > out[j].ID.String()
	})
	if in.Limit > 0 && len(out) > in.Limit {
		out = out[:in.Limit]
	}
	return out, nil
}
func (r *fakeRepo) UpdateTransaction(_ context.Context, in UpdateTransactionInput) (Transaction, error) {
	t, ok := r.txns[in.ID]
	if !ok || t.userID != in.UserID {
		return Transaction{}, ErrTransactionNotFound
	}
	if in.AccountID != nil {
		t.accountID = *in.AccountID
	}
	if in.CategoryID != nil {
		t.categoryID = in.CategoryID
	}
	if in.Amount != nil {
		t.amount = *in.Amount
	}
	if in.Direction != nil {
		t.direction = *in.Direction
	}
	if in.OccurredAt != nil {
		t.occurredAt = *in.OccurredAt
	}
	if in.Note != nil {
		t.note = in.Note
	}
	t.updatedAt = r.tick()
	return r.txnDomain(t), nil
}
func (r *fakeRepo) DeleteTransaction(_ context.Context, userID, id uuid.UUID) error {
	t, ok := r.txns[id]
	if !ok || t.userID != userID {
		return ErrTransactionNotFound
	}
	delete(r.txns, id)
	return nil
}
func (r *fakeRepo) RecentTransactions(_ context.Context, userID uuid.UUID, limit int) ([]Transaction, error) {
	var out []Transaction
	for _, t := range r.txns {
		if t.userID == userID {
			out = append(out, r.txnDomain(t))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// transfers
func (r *fakeRepo) CreateTransfer(ctx context.Context, in TransferInput) ([]Transaction, error) {
	tid := in.TransferID
	debit, _ := r.CreateTransaction(ctx, CreateTransactionInput{UserID: in.UserID, AccountID: in.FromAccount, Amount: in.Amount, Direction: DirDebit, TransferID: &tid, OccurredAt: in.OccurredAt, Note: in.Note})
	credit, _ := r.CreateTransaction(ctx, CreateTransactionInput{UserID: in.UserID, AccountID: in.ToAccount, Amount: in.Amount, Direction: DirCredit, TransferID: &tid, OccurredAt: in.OccurredAt, Note: in.Note})
	return []Transaction{debit, credit}, nil
}
func (r *fakeRepo) ListTransferLegs(_ context.Context, userID, transferID uuid.UUID) ([]Transaction, error) {
	var out []Transaction
	for _, t := range r.txns {
		if t.userID == userID && t.transferID != nil && *t.transferID == transferID {
			out = append(out, r.txnDomain(t))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Direction < out[j].Direction })
	return out, nil
}
func (r *fakeRepo) UpdateTransfer(_ context.Context, in UpdateTransferInput) ([]Transaction, error) {
	var out []Transaction
	for _, t := range r.txns {
		if t.userID != in.UserID || t.transferID == nil || *t.transferID != in.TransferID {
			continue
		}
		if t.direction == DirCredit {
			t.accountID = in.ToAccount
		} else {
			t.accountID = in.FromAccount
		}
		t.amount = in.Amount
		t.occurredAt = in.OccurredAt
		t.note = in.Note
		out = append(out, r.txnDomain(t))
	}
	if len(out) == 0 {
		return nil, ErrTransactionNotFound
	}
	return out, nil
}
func (r *fakeRepo) DeleteTransfer(_ context.Context, userID, transferID uuid.UUID) error {
	for id, t := range r.txns {
		if t.userID == userID && t.transferID != nil && *t.transferID == transferID {
			delete(r.txns, id)
		}
	}
	return nil
}

// budgets
func (r *fakeRepo) UpsertBudget(_ context.Context, userID, categoryID uuid.UUID, month time.Time, amount int64) error {
	r.budgets[budgetKey(userID, categoryID, month)] = amount
	return nil
}
func (r *fakeRepo) DeleteBudget(_ context.Context, userID, categoryID uuid.UUID, month time.Time) error {
	delete(r.budgets, budgetKey(userID, categoryID, month))
	return nil
}
func (r *fakeRepo) ListBudgetsForMonth(_ context.Context, userID uuid.UUID, month time.Time) ([]BudgetLine, error) {
	var out []BudgetLine
	for _, c := range r.cats {
		amt, ok := r.budgets[budgetKey(userID, c.id, month)]
		if !ok {
			continue
		}
		spent := int64(0)
		for _, t := range r.txns {
			if t.userID != userID || t.direction != DirDebit || t.categoryID == nil || !sameMonth(t.occurredAt, month) {
				continue
			}
			if *t.categoryID == c.id || (r.cats[*t.categoryID] != nil && r.cats[*t.categoryID].parentID != nil && *r.cats[*t.categoryID].parentID == c.id) {
				spent += t.amount
			}
		}
		var parentName *string
		if c.parentID != nil && r.cats[*c.parentID] != nil {
			n := r.cats[*c.parentID].name
			parentName = &n
		}
		out = append(out, BudgetLine{CategoryID: c.id, ParentID: c.parentID, Name: c.name, ParentName: parentName, Amount: amt, Spent: spent})
	}
	return out, nil
}

// dashboard
func (r *fakeRepo) MonthFlowTotals(_ context.Context, userID uuid.UUID, month time.Time) (int64, int64, error) {
	var income, expense int64
	for _, t := range r.txns {
		if t.userID != userID || !sameMonth(t.occurredAt, month) {
			continue
		}
		if t.transferID != nil && t.categoryID == nil { // pure transfer leg — excluded
			continue
		}
		if t.direction == DirCredit {
			income += t.amount
		} else {
			expense += t.amount
		}
	}
	return income, expense, nil
}

// ── test fixtures ────────────────────────────────────────────────────

func newSvc() (*Service, *fakeRepo) {
	repo := newFake()
	return &Service{repo: repo}, repo
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func mustExpenseCat(t *testing.T, s *Service, user uuid.UUID, name string) uuid.UUID {
	t.Helper()
	c, err := s.CreateCategory(context.Background(), CreateCategoryInput{UserID: user, Name: name, Kind: KindExpense})
	if err != nil {
		t.Fatalf("create category: %v", err)
	}
	return c.ID
}

func mustAccount(t *testing.T, s *Service, user uuid.UUID, name string, opening int64) uuid.UUID {
	t.Helper()
	a, err := s.CreateAccount(context.Background(), CreateAccountInput{UserID: user, Name: name, Type: "cash", Currency: "VND", OpeningBalance: opening})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return a.ID
}

// ── tests ────────────────────────────────────────────────────────────

func TestDerivedBalanceReconciles(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	acct := mustAccount(t, svc, user, "Cash", 1_000_000)

	// spend 200k, then 50k
	expense := mustExpenseCat(t, svc, user, "Chi")
	if _, err := svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &expense, Amount: 200_000, Direction: DirDebit, OccurredAt: date(2026, 6, 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &expense, Amount: 50_000, Direction: DirDebit, OccurredAt: date(2026, 6, 2)}); err != nil {
		t.Fatal(err)
	}
	accts, _ := svc.ListAccounts(ctx, user)
	if len(accts) != 1 || accts[0].Balance != 750_000 {
		t.Fatalf("balance = %d, want 750000", accts[0].Balance)
	}
}

func TestTransferZeroSum(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	from := mustAccount(t, svc, user, "TCB", 10_000_000)
	to := mustAccount(t, svc, user, "Momo", 0)

	legs, err := svc.CreateTransfer(ctx, TransferParams{UserID: user, FromAccount: from, ToAccount: to, Amount: 5_000_000, OccurredAt: date(2026, 6, 10)})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if len(legs) != 2 {
		t.Fatalf("legs = %d, want 2", len(legs))
	}
	accts, _ := svc.ListAccounts(ctx, user)
	bal := map[string]int64{}
	for _, a := range accts {
		bal[a.Name] = a.Balance
	}
	if bal["TCB"] != 5_000_000 || bal["Momo"] != 5_000_000 {
		t.Fatalf("balances = %+v, want TCB 5M / Momo 5M", bal)
	}
	income, expense, _ := svc.repo.MonthFlowTotals(ctx, user, date(2026, 6, 1))
	if income != 0 || expense != 0 {
		t.Fatalf("month flow = income %d expense %d, want 0/0 (legs excluded)", income, expense)
	}
}

func TestTransferLegNotIndividuallyMutable(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	from := mustAccount(t, svc, user, "A", 1_000_000)
	to := mustAccount(t, svc, user, "B", 0)
	legs, _ := svc.CreateTransfer(ctx, TransferParams{UserID: user, FromAccount: from, ToAccount: to, Amount: 100_000, OccurredAt: date(2026, 6, 1)})

	amt := int64(999)
	if _, err := svc.UpdateTransaction(ctx, UpdateTransactionInput{UserID: user, ID: legs[0].ID, Amount: &amt}); !errors.Is(err, ErrIsTransferLeg) {
		t.Fatalf("edit leg = %v, want ErrIsTransferLeg", err)
	}
	if err := svc.DeleteTransaction(ctx, user, legs[0].ID); !errors.Is(err, ErrIsTransferLeg) {
		t.Fatalf("delete leg = %v, want ErrIsTransferLeg", err)
	}
}

func TestTransferDeleteRemovesBothLegs(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()
	user := uuid.New()
	from := mustAccount(t, svc, user, "A", 1_000_000)
	to := mustAccount(t, svc, user, "B", 0)
	legs, _ := svc.CreateTransfer(ctx, TransferParams{UserID: user, FromAccount: from, ToAccount: to, Amount: 100_000, OccurredAt: date(2026, 6, 1)})
	tid := *legs[0].TransferID

	if err := svc.DeleteTransfer(ctx, user, tid); err != nil {
		t.Fatal(err)
	}
	if len(repo.txns) != 0 {
		t.Fatalf("txns after transfer delete = %d, want 0", len(repo.txns))
	}
	if err := svc.DeleteTransfer(ctx, user, tid); !errors.Is(err, ErrTransactionNotFound) {
		t.Fatalf("second delete = %v, want ErrTransactionNotFound", err)
	}
}

func TestSameAccountAndCurrencyGuards(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	a := mustAccount(t, svc, user, "A", 0)
	if _, err := svc.CreateTransfer(ctx, TransferParams{UserID: user, FromAccount: a, ToAccount: a, Amount: 100, OccurredAt: date(2026, 6, 1)}); !errors.Is(err, ErrSameAccountTransfer) {
		t.Fatalf("same-account = %v, want ErrSameAccountTransfer", err)
	}
	usd, _ := svc.CreateAccount(ctx, CreateAccountInput{UserID: user, Name: "USD", Type: "cash", Currency: "USD"})
	if _, err := svc.CreateTransfer(ctx, TransferParams{UserID: user, FromAccount: a, ToAccount: usd.ID, Amount: 100, OccurredAt: date(2026, 6, 1)}); !errors.Is(err, ErrCurrencyMismatch) {
		t.Fatalf("cross-currency = %v, want ErrCurrencyMismatch", err)
	}
}

func TestDirectionKindMismatch(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	acct := mustAccount(t, svc, user, "Cash", 0)
	income, _ := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, Name: "Lương", Kind: KindIncome})

	// debit filed under an income category → mismatch
	if _, err := svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &income.ID, Amount: 100, Direction: DirDebit, OccurredAt: date(2026, 6, 1)}); !errors.Is(err, ErrDirectionKindMismatch) {
		t.Fatalf("debit+income = %v, want ErrDirectionKindMismatch", err)
	}
	// credit under income → ok
	if _, err := svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &income.ID, Amount: 100, Direction: DirCredit, OccurredAt: date(2026, 6, 1)}); err != nil {
		t.Fatalf("credit+income: %v", err)
	}
}

func TestInvalidAmount(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	acct := mustAccount(t, svc, user, "Cash", 0)
	cat := mustExpenseCat(t, svc, user, "Chi")
	if _, err := svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &cat, Amount: 0, Direction: DirDebit, OccurredAt: date(2026, 6, 1)}); !errors.Is(err, ErrInvalidAmount) {
		t.Fatalf("amount 0 = %v, want ErrInvalidAmount", err)
	}
}

func TestCategoryParentRules(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	parent := mustExpenseCat(t, svc, user, "Ăn uống")
	child, err := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, ParentID: &parent, Name: "Cà phê", Kind: KindExpense})
	if err != nil {
		t.Fatalf("child under top-level: %v", err)
	}
	// grandchild (parent is non-top-level) → invalid parent
	if _, err := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, ParentID: &child.ID, Name: "Espresso", Kind: KindExpense}); !errors.Is(err, ErrInvalidCategoryParent) {
		t.Fatalf("grandchild = %v, want ErrInvalidCategoryParent", err)
	}
	// wrong-kind parent → invalid parent
	if _, err := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, ParentID: &parent, Name: "X", Kind: KindIncome}); !errors.Is(err, ErrInvalidCategoryParent) {
		t.Fatalf("kind-mismatch child = %v, want ErrInvalidCategoryParent", err)
	}
	// foreign parent id → 404 (existence never leaks)
	other := uuid.New()
	if _, err := svc.CreateCategory(ctx, CreateCategoryInput{UserID: other, ParentID: &parent, Name: "Y", Kind: KindExpense}); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("foreign parent = %v, want ErrCategoryNotFound", err)
	}
}

func TestCategoryDeleteReassignMatrix(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()
	user := uuid.New()
	acct := mustAccount(t, svc, user, "Cash", 0)
	src := mustExpenseCat(t, svc, user, "Cũ")
	dst := mustExpenseCat(t, svc, user, "Mới")
	income, _ := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, Name: "Lương", Kind: KindIncome})

	tx, _ := svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &src, Amount: 100, Direction: DirDebit, OccurredAt: date(2026, 6, 1)})

	// in use + no reassign → 409
	if err := svc.DeleteCategory(ctx, user, src, nil); !errors.Is(err, ErrCategoryInUse) {
		t.Fatalf("delete in-use = %v, want ErrCategoryInUse", err)
	}
	// reassign to a different kind → 422
	if err := svc.DeleteCategory(ctx, user, src, &income.ID); !errors.Is(err, ErrCategoryKindMismatch) {
		t.Fatalf("reassign wrong kind = %v, want ErrCategoryKindMismatch", err)
	}
	// valid reassign moves the transaction and deletes the category
	if err := svc.DeleteCategory(ctx, user, src, &dst); err != nil {
		t.Fatalf("reassign delete: %v", err)
	}
	if _, ok := repo.cats[src]; ok {
		t.Fatal("source category should be gone")
	}
	if repo.txns[tx.ID].categoryID == nil || *repo.txns[tx.ID].categoryID != dst {
		t.Fatalf("transaction not reassigned: %+v", repo.txns[tx.ID].categoryID)
	}
}

func TestSeedCategoryImmutable(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()
	user := uuid.New()
	seed := repo.seedCategory("Khác", KindExpense, nil)

	// read: visible to the user
	if _, err := svc.repo.GetVisibleCategory(ctx, user, seed); err != nil {
		t.Fatalf("seed not visible: %v", err)
	}
	// mutate: 404 (owner-mutation matches nothing)
	name := "Hacked"
	if _, err := svc.UpdateCategory(ctx, UpdateCategoryInput{UserID: user, ID: seed, Name: &name}); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("update seed = %v, want ErrCategoryNotFound", err)
	}
	if err := svc.DeleteCategory(ctx, user, seed, nil); !errors.Is(err, ErrCategoryNotFound) {
		t.Fatalf("delete seed = %v, want ErrCategoryNotFound", err)
	}
}

func TestAccountDeleteAndCurrencyGuards(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	acct := mustAccount(t, svc, user, "Cash", 0)
	cat := mustExpenseCat(t, svc, user, "Chi")
	_, _ = svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &cat, Amount: 100, Direction: DirDebit, OccurredAt: date(2026, 6, 1)})

	if err := svc.DeleteAccount(ctx, user, acct); !errors.Is(err, ErrAccountNotEmpty) {
		t.Fatalf("delete non-empty = %v, want ErrAccountNotEmpty", err)
	}
	usd := "USD"
	if _, err := svc.UpdateAccount(ctx, UpdateAccountInput{UserID: user, ID: acct, Currency: &usd}); !errors.Is(err, ErrAccountNotMutable) {
		t.Fatalf("currency change on non-empty = %v, want ErrAccountNotMutable", err)
	}
}

func TestBudgetKindGuardAndDelete(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()
	user := uuid.New()
	expense := mustExpenseCat(t, svc, user, "Ăn uống")
	income, _ := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, Name: "Lương", Kind: KindIncome})
	month := date(2026, 6, 1)

	// income-kind budget → 422
	if err := svc.SetBudget(ctx, user, income.ID, month, 1_000_000); !errors.Is(err, ErrCategoryKindMismatch) {
		t.Fatalf("income budget = %v, want ErrCategoryKindMismatch", err)
	}
	// set then delete (amount 0)
	if err := svc.SetBudget(ctx, user, expense, month, 3_000_000); err != nil {
		t.Fatal(err)
	}
	if repo.budgets[budgetKey(user, expense, month)] != 3_000_000 {
		t.Fatal("budget not set")
	}
	if err := svc.SetBudget(ctx, user, expense, month, 0); err != nil {
		t.Fatal(err)
	}
	if _, ok := repo.budgets[budgetKey(user, expense, month)]; ok {
		t.Fatal("budget amount 0 should delete the row")
	}
}

func TestBudgetSpentIncludesChildren(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	acct := mustAccount(t, svc, user, "Cash", 10_000_000)
	parent := mustExpenseCat(t, svc, user, "Ăn uống")
	child, _ := svc.CreateCategory(ctx, CreateCategoryInput{UserID: user, ParentID: &parent, Name: "Ăn ngoài", Kind: KindExpense})
	month := date(2026, 6, 1)

	_, _ = svc.CreateTransaction(ctx, CreateTransactionInput{UserID: user, AccountID: acct, CategoryID: &child.ID, Amount: 1_000_000, Direction: DirDebit, OccurredAt: date(2026, 6, 5)})
	_ = svc.SetBudget(ctx, user, parent, month, 3_000_000)

	budgets, _ := svc.ListBudgets(ctx, user, month)
	if len(budgets) != 1 {
		t.Fatalf("budgets = %d, want 1", len(budgets))
	}
	if budgets[0].Spent != 1_000_000 { // parent spend includes the child's expense
		t.Fatalf("parent spent = %d, want 1000000 (includes child)", budgets[0].Spent)
	}
}

func TestOwnerScoping(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()
	acctA := mustAccount(t, svc, userA, "A-Cash", 0)

	// B cannot see A's account
	if _, err := svc.repo.GetAccount(ctx, userB, acctA); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("cross-user account = %v, want ErrAccountNotFound", err)
	}
	// B's account list excludes A's
	list, _ := svc.ListAccounts(ctx, userB)
	if len(list) != 0 {
		t.Fatalf("userB sees %d accounts, want 0", len(list))
	}
}
