package layoutrepo

// Adapter bridges the sqlc-generated Queries to layout.Repository. All pgtype ↔
// domain juggling lives here so the module stays sqlc-agnostic.

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/portal/backend/internal/modules/layout"
)

var _ layout.Repository = (*Adapter)(nil)

// Adapter needs the POOL, not just a DBTX, because the whole-set saves run in a
// transaction: upsert-then-prune has to be atomic or a dropped connection
// halfway through leaves the shell missing rows the client still thinks exist.
type Adapter struct {
	pool *pgxpool.Pool
	q    *Queries
}

func NewAdapter(pool *pgxpool.Pool) *Adapter {
	return &Adapter{pool: pool, q: New(pool)}
}

func (a *Adapter) ListMenuItems(ctx context.Context) ([]layout.MenuItem, error) {
	rows, err := a.q.ListMenuItems(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]layout.MenuItem, 0, len(rows))
	for _, r := range rows {
		out = append(out, layout.MenuItem{
			ID:         uuidFrom(r.ID),
			Key:        r.Key,
			Label:      r.Label,
			Icon:       r.Icon,
			Href:       derefStr(r.Href),
			Permission: derefStr(r.Permission),
			Position:   int(r.Position),
			Visible:    r.Visible,
			IsSystem:   r.IsSystem,
		})
	}
	return out, nil
}

func (a *Adapter) ListWidgets(ctx context.Context) ([]layout.Widget, error) {
	rows, err := a.q.ListWidgets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]layout.Widget, 0, len(rows))
	for _, r := range rows {
		out = append(out, layout.Widget{
			ID:         uuidFrom(r.ID),
			Key:        r.Key,
			Label:      r.Label,
			Slot:       r.Slot,
			Permission: derefStr(r.Permission),
			Position:   int(r.Position),
			Visible:    r.Visible,
		})
	}
	return out, nil
}

// SaveMenu upserts every submitted row, then deletes the non-system rows that
// were not submitted — in one transaction, so the menu is never observed
// half-saved.
func (a *Adapter) SaveMenu(ctx context.Context, items []layout.MenuItem) error {
	return a.inTx(ctx, func(q *Queries) error {
		keys := make([]string, 0, len(items))
		for _, it := range items {
			keys = append(keys, it.Key)
			if err := q.UpsertMenuItem(ctx, UpsertMenuItemParams{
				Key:        it.Key,
				Label:      it.Label,
				Icon:       it.Icon,
				Href:       strPtrOrNil(it.Href),
				Permission: strPtrOrNil(it.Permission),
				Position:   int32(it.Position),
				Visible:    it.Visible,
			}); err != nil {
				return err
			}
		}
		return q.DeleteMenuItemsExcept(ctx, keys)
	})
}

func (a *Adapter) SaveWidgets(ctx context.Context, widgets []layout.Widget) error {
	return a.inTx(ctx, func(q *Queries) error {
		for _, w := range widgets {
			if err := q.UpdateWidget(ctx, UpdateWidgetParams{
				Key:        w.Key,
				Label:      w.Label,
				Slot:       w.Slot,
				Permission: strPtrOrNil(w.Permission),
				Position:   int32(w.Position),
				Visible:    w.Visible,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Adapter) inTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := a.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once Commit succeeded
	if err := fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ── conversions ────────────────────────────────────────────────────────────

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// strPtrOrNil maps "" to SQL NULL — an empty href or permission means "none",
// not "the empty string".
func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
