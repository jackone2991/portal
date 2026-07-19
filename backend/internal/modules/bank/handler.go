package bank

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler is the bank HTTP surface. currentUser reads the authenticated user
// from the request context (account RequireAuth middleware, bridged by cmd/api).
// All money fields are integer minor units end-to-end (D-41).
type Handler struct {
	svc         *Service
	currentUser func(context.Context) (uuid.UUID, bool)
}

// ══ Accounts ════════════════════════════════════════════════════════════

func (h *Handler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	accts, err := h.svc.ListAccounts(r.Context(), uid)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	out := make([]any, 0, len(accts))
	for _, a := range accts {
		out = append(out, accountJSON(a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"accounts": out})
}

func (h *Handler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Name           string `json:"name"`
		Type           string `json:"type"`
		Currency       string `json:"currency"`
		OpeningBalance int64  `json:"opening_balance"`
	}
	if !decode(w, r, &body) {
		return
	}
	acct, err := h.svc.CreateAccount(r.Context(), CreateAccountInput{
		UserID: uid, Name: body.Name, Type: body.Type,
		Currency: body.Currency, OpeningBalance: body.OpeningBalance,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, accountJSON(acct))
}

func (h *Handler) UpdateAccount(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     *string `json:"name"`
		Archived *bool   `json:"archived"`
		Currency *string `json:"currency"`
	}
	if !decode(w, r, &body) {
		return
	}
	acct, err := h.svc.UpdateAccount(r.Context(), UpdateAccountInput{
		UserID: uid, ID: id, Name: body.Name, Archived: body.Archived, Currency: body.Currency,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accountJSON(acct))
}

func (h *Handler) DeleteAccount(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteAccount(r.Context(), uid, id); err != nil {
		writeBankErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ══ Categories ══════════════════════════════════════════════════════════

func (h *Handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	cats, err := h.svc.ListCategories(r.Context(), uid)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	out := make([]any, 0, len(cats))
	for _, c := range cats {
		out = append(out, categoryJSON(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": out})
}

func (h *Handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     string          `json:"name"`
		Kind     string          `json:"kind"`
		ParentID json.RawMessage `json:"parent_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	parent, _, perr := parseOptUUID(body.ParentID)
	if perr {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid parent_id")
		return
	}
	cat, err := h.svc.CreateCategory(r.Context(), CreateCategoryInput{
		UserID: uid, ParentID: parent, Name: body.Name, Kind: body.Kind,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, categoryJSON(cat))
}

func (h *Handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		Name     *string         `json:"name"`
		ParentID json.RawMessage `json:"parent_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	parent, present, perr := parseOptUUID(body.ParentID)
	if perr {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid parent_id")
		return
	}
	cat, err := h.svc.UpdateCategory(r.Context(), UpdateCategoryInput{
		UserID: uid, ID: id, Name: body.Name, SetParent: present, ParentID: parent,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, categoryJSON(cat))
}

func (h *Handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var reassignTo *uuid.UUID
	if raw := r.URL.Query().Get("reassign_to"); raw != "" {
		rid, err := uuid.Parse(raw)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid reassign_to")
			return
		}
		reassignTo = &rid
	}
	if err := h.svc.DeleteCategory(r.Context(), uid, id, reassignTo); err != nil {
		writeBankErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ══ Transactions ════════════════════════════════════════════════════════

func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	in := TxListInput{UserID: uid, Limit: atoiSafe(q.Get("limit"))}
	if v := q.Get("account"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid account filter")
			return
		}
		in.AccountID = &id
	}
	if v := q.Get("category"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid category filter")
			return
		}
		in.CategoryID = &id
	}
	if v := q.Get("month"); v != "" {
		m, err := parseMonth(v)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid month filter")
			return
		}
		in.Month = &m
	}
	res, err := h.svc.ListTransactions(r.Context(), in, q.Get("cursor"))
	if err != nil {
		writeBankErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Items))
	for _, t := range res.Items {
		items = append(items, transactionJSON(t))
	}
	out := map[string]any{"transactions": items}
	if res.NextCursor != "" {
		out["next_cursor"] = res.NextCursor
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		AccountID  string      `json:"account_id"`
		CategoryID string      `json:"category_id"`
		Amount     json.Number `json:"amount"`
		Direction  string      `json:"direction"`
		OccurredAt string      `json:"occurred_at"`
		Note       *string     `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	amount, ok := parseAmount(w, body.Amount)
	if !ok {
		return
	}
	accountID, err := uuid.Parse(body.AccountID)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid account_id")
		return
	}
	var categoryID *uuid.UUID
	if body.CategoryID != "" {
		cid, err := uuid.Parse(body.CategoryID)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid category_id")
			return
		}
		categoryID = &cid
	}
	occurred, err := parseDateDefault(body.OccurredAt)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid occurred_at (want YYYY-MM-DD)")
		return
	}
	tx, err := h.svc.CreateTransaction(r.Context(), CreateTransactionInput{
		UserID: uid, AccountID: accountID, CategoryID: categoryID,
		Amount: amount, Direction: body.Direction, OccurredAt: occurred, Note: body.Note,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, transactionJSON(tx))
}

func (h *Handler) UpdateTransaction(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	var body struct {
		AccountID  *string      `json:"account_id"`
		CategoryID *string      `json:"category_id"`
		Amount     *json.Number `json:"amount"`
		Direction  *string      `json:"direction"`
		OccurredAt *string      `json:"occurred_at"`
		Note       *string      `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	amount, ok := parseOptAmount(w, body.Amount)
	if !ok {
		return
	}
	in := UpdateTransactionInput{UserID: uid, ID: id, Amount: amount, Direction: body.Direction, Note: body.Note}
	if body.AccountID != nil {
		aid, err := uuid.Parse(*body.AccountID)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid account_id")
			return
		}
		in.AccountID = &aid
	}
	if body.CategoryID != nil {
		cid, err := uuid.Parse(*body.CategoryID)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid category_id")
			return
		}
		in.CategoryID = &cid
	}
	if body.OccurredAt != nil {
		d, err := parseDate(*body.OccurredAt)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid occurred_at")
			return
		}
		in.OccurredAt = &d
	}
	tx, err := h.svc.UpdateTransaction(r.Context(), in)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transactionJSON(tx))
}

func (h *Handler) DeleteTransaction(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteTransaction(r.Context(), uid, id); err != nil {
		writeBankErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ══ Transfers ═══════════════════════════════════════════════════════════

func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		FromAccount string      `json:"from_account"`
		ToAccount   string      `json:"to_account"`
		Amount      json.Number `json:"amount"`
		OccurredAt  string      `json:"occurred_at"`
		Note        *string     `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	amount, ok := parseAmount(w, body.Amount)
	if !ok {
		return
	}
	from, err := uuid.Parse(body.FromAccount)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid from_account")
		return
	}
	to, err := uuid.Parse(body.ToAccount)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid to_account")
		return
	}
	occurred, err := parseDateDefault(body.OccurredAt)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid occurred_at")
		return
	}
	legs, err := h.svc.CreateTransfer(r.Context(), TransferParams{
		UserID: uid, FromAccount: from, ToAccount: to, Amount: amount, OccurredAt: occurred, Note: body.Note,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, transferJSON(legs))
}

func (h *Handler) UpdateTransfer(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	tid, ok := parseTransferID(w, r)
	if !ok {
		return
	}
	var body struct {
		FromAccount *string      `json:"from_account"`
		ToAccount   *string      `json:"to_account"`
		Amount      *json.Number `json:"amount"`
		OccurredAt  *string      `json:"occurred_at"`
		Note        *string      `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	amount, ok := parseOptAmount(w, body.Amount)
	if !ok {
		return
	}
	p := UpdateTransferParams{UserID: uid, TransferID: tid, Amount: amount, Note: body.Note}
	if body.FromAccount != nil {
		id, err := uuid.Parse(*body.FromAccount)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid from_account")
			return
		}
		p.FromAccount = &id
	}
	if body.ToAccount != nil {
		id, err := uuid.Parse(*body.ToAccount)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid to_account")
			return
		}
		p.ToAccount = &id
	}
	if body.OccurredAt != nil {
		d, err := parseDate(*body.OccurredAt)
		if err != nil {
			writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid occurred_at")
			return
		}
		p.OccurredAt = &d
	}
	legs, err := h.svc.UpdateTransfer(r.Context(), p)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, transferJSON(legs))
}

func (h *Handler) DeleteTransfer(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	tid, ok := parseTransferID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteTransfer(r.Context(), uid, tid); err != nil {
		writeBankErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ══ Budgets ═════════════════════════════════════════════════════════════

func (h *Handler) ListBudgets(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	month, err := monthParam(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid month")
		return
	}
	budgets, err := h.svc.ListBudgets(r.Context(), uid, month)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	out := make([]any, 0, len(budgets))
	for _, b := range budgets {
		out = append(out, budgetJSON(b))
	}
	writeJSON(w, http.StatusOK, map[string]any{"month": month.Format("2006-01"), "budgets": out})
}

func (h *Handler) SetBudget(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		CategoryID string       `json:"category_id"`
		Month      string       `json:"month"`
		Amount     *json.Number `json:"amount"` // 0 or null deletes
	}
	if !decode(w, r, &body) {
		return
	}
	amountPtr, ok := parseOptAmount(w, body.Amount)
	if !ok {
		return
	}
	categoryID, err := uuid.Parse(body.CategoryID)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid category_id")
		return
	}
	month, err := parseMonth(body.Month)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid month (want YYYY-MM)")
		return
	}
	var amount int64
	if amountPtr != nil {
		amount = *amountPtr
	}
	if err := h.svc.SetBudget(r.Context(), uid, categoryID, month, amount); err != nil {
		writeBankErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ══ Dashboard ═══════════════════════════════════════════════════════════

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	month, err := monthParam(r)
	if err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid month")
		return
	}
	dash, err := h.svc.Dashboard(r.Context(), uid, month)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboardJSON(dash))
}

// ── JSON shapes ───────────────────────────────────────────────────────

func accountJSON(a Account) map[string]any {
	return map[string]any{
		"id":              a.ID,
		"name":            a.Name,
		"type":            a.Type,
		"currency":        a.Currency,
		"opening_balance": a.OpeningBalance,
		"balance":         a.Balance,
		"archived":        a.Archived,
		"created_at":      a.CreatedAt.Format(time.RFC3339),
	}
}

func categoryJSON(c Category) map[string]any {
	return map[string]any{
		"id":        c.ID,
		"parent_id": uuidPtrJSON(c.ParentID),
		"name":      c.Name,
		"kind":      c.Kind,
		"seed":      c.Seed,
	}
}

func transactionJSON(t Transaction) map[string]any {
	return map[string]any{
		"id":          t.ID,
		"account_id":  t.AccountID,
		"category_id": uuidPtrJSON(t.CategoryID),
		"amount":      t.Amount,
		"direction":   t.Direction,
		"transfer_id": uuidPtrJSON(t.TransferID),
		"is_transfer": t.IsTransferLeg(),
		"occurred_at": t.OccurredAt.Format(dateLayout),
		"note":        t.Note,
		"created_at":  t.CreatedAt.Format(time.RFC3339),
		"updated_at":  t.UpdatedAt.Format(time.RFC3339),
	}
}

func transferJSON(legs []Transaction) map[string]any {
	out := make([]any, 0, len(legs))
	for _, l := range legs {
		out = append(out, transactionJSON(l))
	}
	var transferID any
	if len(legs) > 0 && legs[0].TransferID != nil {
		transferID = legs[0].TransferID
	}
	return map[string]any{"transfer_id": transferID, "legs": out}
}

func budgetJSON(b BudgetLine) map[string]any {
	return map[string]any{
		"category_id": b.CategoryID,
		"parent_id":   uuidPtrJSON(b.ParentID),
		"name":        b.Name,
		"parent_name": b.ParentName,
		"amount":      b.Amount,
		"spent":       b.Spent,
	}
}

func dashboardJSON(d Dashboard) map[string]any {
	accts := make([]any, 0, len(d.Accounts))
	for _, a := range d.Accounts {
		accts = append(accts, accountJSON(a))
	}
	budgets := make([]any, 0, len(d.Budgets))
	for _, b := range d.Budgets {
		budgets = append(budgets, budgetJSON(b))
	}
	recent := make([]any, 0, len(d.Recent))
	for _, t := range d.Recent {
		recent = append(recent, transactionJSON(t))
	}
	return map[string]any{
		"month":    d.Month,
		"accounts": accts,
		"income":   d.Income,
		"expense":  d.Expense,
		"budgets":  budgets,
		"recent":   recent,
	}
}

// ── request/response helpers ──────────────────────────────────────────

func (h *Handler) auth(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	uid, ok := h.currentUser(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
		return uuid.Nil, false
	}
	return uid, true
}

func parseID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "bank/not-found", "Not Found", "resource not found")
		return uuid.Nil, false
	}
	return id, true
}

func parseTransferID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "transfer_id"))
	if err != nil {
		writeProblem(w, http.StatusNotFound, "bank/not-found", "Not Found", "transfer not found")
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v); err != nil {
		writeProblem(w, http.StatusBadRequest, "about:blank", "Bad Request", "invalid JSON body")
		return false
	}
	return true
}

// parseAmount converts a required JSON amount (integer minor units — SPEC-03) to
// int64. A fractional/out-of-range value is rejected as 422 bank/invalid-amount
// and returns ok=false (the handler has already written the response — return).
func parseAmount(w http.ResponseWriter, n json.Number) (int64, bool) {
	v, err := n.Int64()
	if err != nil {
		writeBankErr(w, ErrInvalidAmount)
		return 0, false
	}
	return v, true
}

// parseOptAmount is parseAmount for an optional field: nil (absent/null) stays nil
// with ok=true; a present non-integer value is rejected as 422 bank/invalid-amount.
func parseOptAmount(w http.ResponseWriter, n *json.Number) (*int64, bool) {
	if n == nil {
		return nil, true
	}
	v, err := n.Int64()
	if err != nil {
		writeBankErr(w, ErrInvalidAmount)
		return nil, false
	}
	return &v, true
}

// parseOptUUID interprets an optional JSON field: nil raw = absent (present=false);
// `null` = present with nil value; a quoted uuid = present with that value. Any
// other content sets the error flag.
func parseOptUUID(raw json.RawMessage) (id *uuid.UUID, present bool, parseErr bool) {
	if len(raw) == 0 {
		return nil, false, false
	}
	if string(raw) == "null" {
		return nil, true, false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, true, true
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return nil, true, true
	}
	return &u, true, false
}

// monthParam reads ?month=YYYY-MM (default: current month).
func monthParam(r *http.Request) (time.Time, error) {
	v := r.URL.Query().Get("month")
	if v == "" {
		return time.Now().UTC(), nil
	}
	return parseMonth(v)
}

func parseMonth(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01", s); err == nil {
		return t, nil
	}
	return time.Parse(dateLayout, s)
}

func parseDate(s string) (time.Time, error) {
	return time.Parse(dateLayout, s)
}

func parseDateDefault(s string) (time.Time, error) {
	if s == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}
	return parseDate(s)
}

func uuidPtrJSON(p *uuid.UUID) any {
	if p == nil {
		return nil
	}
	return p.String()
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > 1_000_000 {
			return 1_000_000
		}
	}
	return n
}

// writeBankErr maps a service error to its RFC 7807 Problem (SPEC-03 §7).
func writeBankErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrAccountNotFound), errors.Is(err, ErrCategoryNotFound), errors.Is(err, ErrTransactionNotFound):
		writeProblem(w, http.StatusNotFound, "bank/not-found", "Not Found", "resource not found")
	case errors.Is(err, ErrAccountNotEmpty):
		writeProblem(w, http.StatusConflict, "bank/account-not-empty", "Account not empty", "archive the account instead of deleting it")
	case errors.Is(err, ErrAccountNotMutable):
		writeProblem(w, http.StatusConflict, "bank/account-not-mutable", "Account not mutable", "currency is immutable once the account has transactions")
	case errors.Is(err, ErrIsTransferLeg):
		writeProblem(w, http.StatusConflict, "bank/is-transfer-leg", "Transfer leg", "edit or delete this via /bank/transfers/{transfer_id}")
	case errors.Is(err, ErrCategoryInUse):
		writeProblem(w, http.StatusConflict, "bank/category-in-use", "Category in use", "reassign its transactions with ?reassign_to= before deleting")
	case errors.Is(err, ErrSameAccountTransfer):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/same-account-transfer", "Same-account transfer", "from and to accounts must differ")
	case errors.Is(err, ErrCurrencyMismatch):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/currency-mismatch", "Currency mismatch", "cross-currency transfers are not supported")
	case errors.Is(err, ErrCategoryKindMismatch):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/category-kind-mismatch", "Category kind mismatch", "the target category is a different kind")
	case errors.Is(err, ErrDirectionKindMismatch):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/direction-kind-mismatch", "Direction/kind mismatch", "a debit needs an expense category, a credit an income category")
	case errors.Is(err, ErrInvalidAmount):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/invalid-amount", "Invalid amount", "amount must be a positive integer (minor units)")
	case errors.Is(err, ErrInvalidCategoryParent):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/invalid-category-parent", "Invalid category parent", "parent must be a top-level category of the same kind, and the category must have no children")
	case errors.Is(err, ErrCategoryImmutable):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/category-immutable", "Category immutable", "kind cannot be changed after creation")
	case errors.Is(err, ErrValidation):
		writeProblem(w, http.StatusUnprocessableEntity, "bank/validation", "Validation error", "the request is invalid")
	case errors.Is(err, ErrBadCursor):
		writeProblem(w, http.StatusBadRequest, "bank/invalid-cursor", "Invalid cursor", "the pagination cursor is malformed")
	default:
		writeProblem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeProblem(w http.ResponseWriter, status int, typ, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type": typ, "title": title, "status": status, "detail": detail,
	})
}
