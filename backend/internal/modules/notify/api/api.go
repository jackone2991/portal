// Package api is the public surface of the notify module (SPEC-04).
//
// Producers (account password-reset, the media→asset_ready consumer, and every
// future life-stream source) enqueue a single typed NotificationIntent through
// Enqueue — they never hand-build the task payload and never send mail/push
// themselves (MODULES.md §5.2). The notify worker owns the fan-out from there.
//
// Only this package may be imported by other modules; notify's service/handler/
// repository internals stay private.
package api

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// Asynq task types owned by the notify module (docs/reference/events.md).
const (
	TaskDispatch     = "notify:dispatch"
	TaskEmail        = "notify:email"
	TaskWebPush      = "notify:web_push" // P1.1
	TaskOnAssetReady = "notify:on_asset_ready"
	TaskPurgeOld     = "notify:purge_old" // P2
)

// Notification type strings — an OPEN registry (see notify/README.md). New types
// slot in with zero schema change; only these have real producers today.
const (
	TypePasswordReset   = "account.password_reset" // email-only, non-mutable, never persisted
	TypeSecurityAlert   = "account.security_alert" // P1.4 — email + in-app, non-mutable
	TypeMediaAssetReady = "media.asset_ready"
)

// Channel identifiers used in NotificationIntent.Channels and preferences.
const (
	ChannelInApp = "in_app"
	ChannelEmail = "email"
	ChannelPush  = "push"
)

// NotificationIntent is the single ingress payload for the delivery fan-out
// (notify:dispatch). Channels is an optional override unioned with the stored
// per-type preference — it exists so a transactional send (password reset)
// reaches the user regardless of their email-off default. DedupKey is an
// optional event-derived natural id (e.g. asset_id) that makes a redelivered
// dispatch a no-op against the in-app store under at-least-once delivery.
type NotificationIntent struct {
	UserID   uuid.UUID      `json:"user_id"`
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Body     string         `json:"body,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Channels []string       `json:"channels,omitempty"`
	DedupKey string         `json:"dedup_key,omitempty"`
}

// Enqueuer is the subset of *asynq.Client the producer helper needs. Kept
// minimal so callers can fake it; *asynq.Client satisfies it.
type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Enqueue schedules a notify:dispatch task carrying intent. This is the ONLY
// sanctioned way for a producer to reach the notification system.
func Enqueue(ctx context.Context, client Enqueuer, intent NotificationIntent) error {
	body, err := json.Marshal(intent)
	if err != nil {
		return err
	}
	_, err = client.EnqueueContext(ctx, asynq.NewTask(TaskDispatch, body))
	return err
}
