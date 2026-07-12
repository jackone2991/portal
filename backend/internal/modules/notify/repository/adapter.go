package notifyrepo

// Adapter bridges the sqlc-generated Queries to notify.Repository. cmd/api and
// cmd/worker construct one with NewAdapter and pass it into notify.Deps. All
// pgtype ↔ domain juggling (and the dedup-conflict → inserted=false mapping)
// lives here so the module code stays sqlc-agnostic.

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/notify"
)

type Adapter struct {
	q *Queries
}

func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

// Compile-time proof the adapter satisfies the notify persistence surface.
var _ notify.Repository = (*Adapter)(nil)

func (a *Adapter) InsertNotification(ctx context.Context, in notify.InsertNotificationInput) (bool, notify.Notification, error) {
	row, err := a.q.InsertNotification(ctx, InsertNotificationParams{
		UserID:   pgUUID(in.UserID),
		Type:     in.Type,
		Title:    in.Title,
		Body:     toNullStr(in.Body),
		Data:     marshalData(in.Data),
		DedupKey: toNullStr(in.DedupKey),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, notify.Notification{}, nil // dedup conflict: DO NOTHING → no row
		}
		return false, notify.Notification{}, err
	}
	return true, toNotification(row), nil
}

func (a *Adapter) ListNotifications(ctx context.Context, in notify.ListInput) ([]notify.Notification, error) {
	params := ListNotificationsParams{
		UserID:     pgUUID(in.UserID),
		UnreadOnly: in.UnreadOnly,
		Lim:        int32(in.Limit),
	}
	if !in.CursorAt.IsZero() {
		params.CursorCreatedAt = pgtype.Timestamptz{Time: in.CursorAt, Valid: true}
		params.CursorID = pgUUID(in.CursorID)
	}
	rows, err := a.q.ListNotifications(ctx, params)
	if err != nil {
		return nil, err
	}
	out := make([]notify.Notification, 0, len(rows))
	for _, r := range rows {
		out = append(out, toNotification(r))
	}
	return out, nil
}

func (a *Adapter) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	n, err := a.q.CountUnreadNotifications(ctx, pgUUID(userID))
	return int(n), err
}

func (a *Adapter) MarkRead(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	_, err := a.q.MarkNotificationRead(ctx, MarkNotificationReadParams{
		ID:     pgUUID(id),
		UserID: pgUUID(userID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil // missing or other-user id → 404
		}
		return false, err
	}
	return true, nil
}

func (a *Adapter) MarkAllRead(ctx context.Context, userID uuid.UUID, beforeAt time.Time, beforeID uuid.UUID) error {
	params := MarkAllNotificationsReadParams{UserID: pgUUID(userID)}
	if !beforeAt.IsZero() {
		params.BeforeCreatedAt = pgtype.Timestamptz{Time: beforeAt, Valid: true}
		params.BeforeID = pgUUID(beforeID)
	}
	return a.q.MarkAllNotificationsRead(ctx, params)
}

func (a *Adapter) GetPreference(ctx context.Context, userID uuid.UUID, typ string) (notify.Preference, bool, error) {
	row, err := a.q.GetNotificationPreference(ctx, GetNotificationPreferenceParams{
		UserID: pgUUID(userID),
		Type:   typ,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notify.Preference{}, false, nil
		}
		return notify.Preference{}, false, err
	}
	return notify.Preference{
		InApp: row.InApp,
		Email: row.Email,
		Push:  row.Push,
		Muted: row.Muted,
	}, true, nil
}

func (a *Adapter) PurgeOld(ctx context.Context) error {
	return a.q.PurgeOldNotifications(ctx)
}

// ── mapping helpers ─────────────────────────────────────────────────

func toNotification(r Notification) notify.Notification {
	return notify.Notification{
		ID:        uuidFrom(r.ID),
		UserID:    uuidFrom(r.UserID),
		Type:      r.Type,
		Title:     r.Title,
		Body:      derefStr(r.Body),
		Data:      unmarshalData(r.Data),
		DedupKey:  derefStr(r.DedupKey),
		ReadAt:    timePtrFrom(r.ReadAt),
		CreatedAt: r.CreatedAt.Time,
	}
}

func marshalData(m map[string]any) []byte {
	if len(m) == 0 {
		return []byte("{}")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return []byte("{}")
	}
	return b
}

func unmarshalData(b []byte) map[string]any {
	if len(b) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return map[string]any{}
	}
	return m
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func timePtrFrom(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	tt := t.Time
	return &tt
}

func toNullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
