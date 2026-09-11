package bank

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/platform/server"
)

// HTTP for debts and loans (SPEC-10 phase 1).

// GET /bank/debts
func (h *Handler) ListDebts(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	items, err := h.svc.ListDebts(r.Context(), uid)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for _, d := range items {
		out = append(out, debtJSON(d))
	}
	server.JSON(w, http.StatusOK, map[string]any{"debts": out})
}

// GET /bank/debts/{id}
func (h *Handler) GetDebt(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseDebtID(w, r)
	if !ok {
		return
	}
	d, err := h.svc.GetDebt(r.Context(), uid, id)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, debtJSON(d))
}

// POST /bank/debts — opens the debt AND posts the principal movement.
func (h *Handler) CreateDebt(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	var body struct {
		Counterparty    string  `json:"counterparty"`
		Direction       string  `json:"direction"`
		Principal       int64   `json:"principal"`
		InterestRateBps int32   `json:"interest_rate_bps"`
		InterestMethod  string  `json:"interest_method"`
		OpenedOn        string  `json:"opened_on"`
		DueOn           *string `json:"due_on"`
		Note            *string `json:"note"`
		WalletID        string  `json:"wallet_id"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	opened, ok := parseDay(w, body.OpenedOn, true)
	if !ok {
		return
	}
	due, ok := parseOptDay(w, body.DueOn)
	if !ok {
		return
	}
	wallet, err := uuid.Parse(body.WalletID)
	if err != nil {
		server.BadRequest(w, "wallet_id must be a uuid")
		return
	}
	d, err := h.svc.CreateDebt(r.Context(), CreateDebtInput{
		UserID:          uid,
		Counterparty:    body.Counterparty,
		Direction:       body.Direction,
		Principal:       body.Principal,
		InterestRateBps: body.InterestRateBps,
		InterestMethod:  body.InterestMethod,
		OpenedOn:        *opened,
		DueOn:           due,
		Note:            body.Note,
		WalletID:        wallet,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, debtJSON(d))
}

// PATCH /bank/debts/{id} — terms only; the principal is immutable.
func (h *Handler) UpdateDebt(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseDebtID(w, r)
	if !ok {
		return
	}
	var body struct {
		Counterparty    *string         `json:"counterparty"`
		InterestRateBps *int32          `json:"interest_rate_bps"`
		InterestMethod  *string         `json:"interest_method"`
		DueOn           json.RawMessage `json:"due_on"`
		Note            json.RawMessage `json:"note"`
		Closed          *bool           `json:"closed"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	// Raw, not *string: an absent key must leave the field alone while an
	// explicit null clears it, and *string collapses those two into the same nil
	// (same reasoning as UpdateCategory).
	dueStr, setDue, derr := parseOptString(body.DueOn)
	noteStr, setNote, nerr := parseOptString(body.Note)
	if derr || nerr {
		server.BadRequest(w, "invalid due_on or note")
		return
	}
	in := UpdateDebtInput{
		UserID: uid, ID: id,
		Counterparty:    body.Counterparty,
		InterestRateBps: body.InterestRateBps,
		InterestMethod:  body.InterestMethod,
	}
	if setDue {
		due, ok := parseOptDay(w, dueStr)
		if !ok {
			return
		}
		in.SetDueOn, in.DueOn = true, due
	}
	if setNote {
		in.SetNote, in.Note = true, noteStr
	}
	if body.Closed != nil {
		in.SetClosed = true
		if *body.Closed {
			now := time.Now().UTC()
			in.ClosedAt = &now
		}
	}
	d, err := h.svc.UpdateDebt(r.Context(), in)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	server.JSON(w, http.StatusOK, debtJSON(d))
}

// DELETE /bank/debts/{id}
func (h *Handler) DeleteDebt(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseDebtID(w, r)
	if !ok {
		return
	}
	if err := h.svc.DeleteDebt(r.Context(), uid, id); err != nil {
		writeBankErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /bank/debts/{id}/movements — a repayment or a collection.
func (h *Handler) AddDebtMovement(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseDebtID(w, r)
	if !ok {
		return
	}
	var body struct {
		Kind       string  `json:"kind"`
		WalletID   string  `json:"wallet_id"`
		Amount     int64   `json:"amount"`
		OccurredAt string  `json:"occurred_at"`
		Note       *string `json:"note"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	wallet, err := uuid.Parse(body.WalletID)
	if err != nil {
		server.BadRequest(w, "wallet_id must be a uuid")
		return
	}
	on, ok := parseDay(w, body.OccurredAt, false)
	if !ok {
		return
	}
	var when time.Time
	if on != nil {
		when = *on
	}
	legs, err := h.svc.AddMovement(r.Context(), DebtMovementInput{
		UserID: uid, DebtID: id, Kind: body.Kind,
		WalletID: wallet, Amount: body.Amount, OccurredAt: when, Note: body.Note,
	})
	if err != nil {
		writeBankErr(w, err)
		return
	}
	out := make([]any, 0, len(legs))
	for _, t := range legs {
		out = append(out, transactionJSON(t))
	}
	server.JSON(w, http.StatusCreated, map[string]any{"legs": out})
}

// POST /bank/debts/{id}/accrue — post interest as a real, categorised flow.
func (h *Handler) AccrueDebtInterest(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.auth(w, r)
	if !ok {
		return
	}
	id, ok := parseDebtID(w, r)
	if !ok {
		return
	}
	var body struct {
		UpTo string `json:"up_to"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	upTo, ok := parseDay(w, body.UpTo, false)
	if !ok {
		return
	}
	var when time.Time
	if upTo != nil {
		when = *upTo
	}
	tx, err := h.svc.AccrueInterest(r.Context(), uid, id, when)
	if err != nil {
		writeBankErr(w, err)
		return
	}
	server.JSON(w, http.StatusCreated, transactionJSON(tx))
}

/* ── helpers ──────────────────────────────────────────────────────── */

func parseDebtID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.Problem(w, http.StatusNotFound, "bank/not-found", "Not Found", "resource not found")
		return uuid.Nil, false
	}
	return id, true
}

// parseDay reads a YYYY-MM-DD. `required` decides whether an empty string is an
// error or simply "unset" — an absent occurred_at means today, an absent
// opened_on is a debt with no start.
func parseDay(w http.ResponseWriter, s string, required bool) (*time.Time, bool) {
	if s == "" {
		if required {
			server.BadRequest(w, "date must be YYYY-MM-DD")
			return nil, false
		}
		return nil, true
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		server.BadRequest(w, "date must be YYYY-MM-DD")
		return nil, false
	}
	return &t, true
}

func parseOptDay(w http.ResponseWriter, s *string) (*time.Time, bool) {
	if s == nil || *s == "" {
		return nil, true
	}
	return parseDay(w, *s, true)
}

func debtJSON(d DebtView) map[string]any {
	m := map[string]any{
		"id":                 d.ID,
		"account_id":         d.AccountID,
		"counterparty":       d.Counterparty,
		"direction":          d.Direction,
		"principal":          d.Principal,
		"outstanding":        d.Outstanding,
		"settled":            d.Settled,
		"interest_rate_bps":  d.InterestRateBps,
		"interest_method":    d.InterestMethod,
		"projected_interest": d.ProjectedInterest,
		"opened_on":          d.OpenedOn.Format("2006-01-02"),
		"note":               d.Note,
		"closed":             d.ClosedAt != nil,
	}
	if d.DueOn != nil {
		m["due_on"] = d.DueOn.Format("2006-01-02")
	} else {
		m["due_on"] = nil
	}
	return m
}
