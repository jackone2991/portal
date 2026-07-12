package notify

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	notifyapi "github.com/portal/backend/internal/modules/notify/api"
)

// Errors surfaced to the handler.
var (
	ErrNotFound  = errors.New("notify: notification not found")
	ErrBadCursor = errors.New("notify: invalid cursor")
)

// Notification is the notify module's internal record of an in-app row.
type Notification struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Type      string
	Title     string
	Body      string
	Data      map[string]any
	DedupKey  string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// Preference is a user's per-type delivery preference. An absent row means the
// module default (defaultPreference).
type Preference struct {
	InApp bool
	Email bool
	Push  bool
	Muted bool
}

// defaultPreference is applied when no notification_preferences row exists for a
// (user, type): in-app on, email off, push off, not muted (§6, P0.2 step 2).
func defaultPreference() Preference {
	return Preference{InApp: true}
}

// InsertNotificationInput is the write side of the in-app store.
type InsertNotificationInput struct {
	UserID   uuid.UUID
	Type     string
	Title    string
	Body     string
	Data     map[string]any
	DedupKey string // "" → NULL (no dedup)
}

// ListInput is the keyset read query for the bell.
type ListInput struct {
	UserID     uuid.UUID
	UnreadOnly bool
	CursorAt   time.Time
	CursorID   uuid.UUID
	Limit      int
}

// Repository is the persistence surface. The sqlc-backed adapter implements it;
// tests use an in-memory fake.
type Repository interface {
	// InsertNotification writes an in-app row. inserted is false when a
	// dedup_key collision made the insert a no-op (redelivered dispatch).
	InsertNotification(ctx context.Context, in InsertNotificationInput) (inserted bool, n Notification, err error)
	ListNotifications(ctx context.Context, in ListInput) ([]Notification, error)
	UnreadCount(ctx context.Context, userID uuid.UUID) (int, error)
	// MarkRead is idempotent + owner-scoped; found is false for a missing or
	// other-user id (→ 404, never leaks existence).
	MarkRead(ctx context.Context, userID, id uuid.UUID) (found bool, err error)
	// MarkAllRead marks unread rows at/older than the before watermark; a zero
	// before marks all.
	MarkAllRead(ctx context.Context, userID uuid.UUID, beforeAt time.Time, beforeID uuid.UUID) error
	GetPreference(ctx context.Context, userID uuid.UUID, typ string) (pref Preference, found bool, err error)
	PurgeOld(ctx context.Context) error
}

// EmailMessage is a rendered email ready for a transport.
type EmailMessage struct {
	To       string
	ToName   string
	Subject  string
	TextBody string
	HTMLBody string
}

// EmailSender delivers a rendered email. Implementations: SMTPSender (Mailpit in
// dev / SMTP in prod) and LogSender (tests + a no-SMTP dev fallback).
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// UserResolver resolves the recipient address for a notify:email task. Satisfied
// by an adapter over account/api in the wiring layer — notify never imports
// account internals.
type UserResolver interface {
	ResolveRecipient(ctx context.Context, userID uuid.UUID) (email, displayName string, err error)
}

// Enqueuer schedules the channel follow-up tasks (notify:email / notify:web_push).
// *asynq.Client satisfies it.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// channelPayload is the notify:email / notify:web_push task body: the channel
// handler re-resolves the recipient and renders the template from type + data.
type channelPayload struct {
	UserID uuid.UUID      `json:"user_id"`
	Type   string         `json:"type"`
	Title  string         `json:"title,omitempty"`
	Data   map[string]any `json:"data,omitempty"`
}

// isNonMutable reports whether a type ignores the muted switch — a user who muted
// resets/security alerts must still be able to recover/secure their account
// (P0.2 steps 1–2, P0.3, P1.4).
func isNonMutable(typ string) bool {
	return typ == notifyapi.TypePasswordReset || typ == notifyapi.TypeSecurityAlert
}

// persistsInApp reports whether a type is stored as an in-app row. password_reset
// never persists — the reset link rides the email task only (P0.3).
func persistsInApp(typ string) bool {
	return typ != notifyapi.TypePasswordReset
}

// channelSet is the resolved delivery target after unioning prefs + override.
type channelSet struct {
	inApp bool
	email bool
	push  bool
}

// resolveChannels unions the stored preference with the intent's channel override
// (P0.2 step 2). Override can only turn a channel ON — it is how a transactional
// send bypasses the email-off default; it never disables a channel.
func resolveChannels(pref Preference, override []string) channelSet {
	cs := channelSet{inApp: pref.InApp, email: pref.Email, push: pref.Push}
	for _, c := range override {
		switch c {
		case notifyapi.ChannelInApp:
			cs.inApp = true
		case notifyapi.ChannelEmail:
			cs.email = true
		case notifyapi.ChannelPush:
			cs.push = true
		}
	}
	return cs
}
