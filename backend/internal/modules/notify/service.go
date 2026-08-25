package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	notifyapi "github.com/portal/backend/internal/modules/notify/api"
	"github.com/portal/backend/internal/platform/server"
)

const (
	defaultListLimit = 50
	maxListLimit     = 100
)

// Service holds the notify business logic (dispatch fan-out, the in-app store
// read API, the email channel, and the media:asset_ready consumer). Construct
// via the module.
type Service struct {
	repo    Repository
	enqueue Enqueuer      // schedules notify:email / notify:web_push (worker side)
	email   EmailSender   // email transport (worker side)
	users   UserResolver  // recipient lookup for the email channel (worker side)
	redis   *redis.Client // global email send-ceiling counter (nil disables it)

	emailHourlyCap int // 0 = uncapped
	// runInUserTenant scopes the worker in-app INSERT to the recipient's personal
	// org (ADR-07 1b) so notifications.tenant_id's DEFAULT current_setting is set.
	// nil (API read side / tests) → fn runs directly.
	runInUserTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error
}

// runScoped runs fn in the target user's tenant scope (ADR-07 1b) on the worker;
// a nil runInUserTenant (API side / tests) runs fn directly.
func (s *Service) runScoped(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error {
	if s.runInUserTenant == nil {
		return fn(ctx)
	}
	return s.runInUserTenant(ctx, userID, fn)
}

// ── P0.2 dispatch fan-out ───────────────────────────────────────────

// Dispatch is the notify:dispatch handler. A malformed intent fails fast with
// asynq.SkipRetry (straight to the archive — a payload that can never become
// valid must not burn retries); transient DB errors return a plain error so
// Asynq retries.
func (s *Service) Dispatch(ctx context.Context, task *asynq.Task) error {
	var intent notifyapi.NotificationIntent
	if err := json.Unmarshal(task.Payload(), &intent); err != nil {
		log.Error().Err(err).Msg("notify:dispatch: undecodable payload")
		return fmt.Errorf("notify:dispatch: undecodable payload: %w", asynq.SkipRetry)
	}
	return s.dispatchIntent(ctx, intent)
}

// dispatchIntent runs the P0.2 fan-out for a decoded intent. Shared by the
// notify:dispatch task and the media:asset_ready consumer (P0.4).
func (s *Service) dispatchIntent(ctx context.Context, intent notifyapi.NotificationIntent) error {
	if intent.UserID == uuid.Nil || intent.Type == "" {
		log.Error().Str("type", intent.Type).Msg("notify:dispatch: malformed intent (missing user_id/type)")
		return fmt.Errorf("notify:dispatch: malformed intent: %w", asynq.SkipRetry)
	}

	// 1. mute precedence — the single "deliver nothing" switch, unless the type
	// is non-mutable (password reset / security alert).
	pref, found, err := s.repo.GetPreference(ctx, intent.UserID, intent.Type)
	if err != nil {
		return fmt.Errorf("notify:dispatch: load pref: %w", err)
	}
	if !found {
		pref = defaultPreference()
	}
	nonMutable := isNonMutable(intent.Type)
	if pref.Muted && !nonMutable {
		return nil // stop: no row, no channel tasks
	}

	// 2. resolve channels = stored pref ∪ intent override.
	channels := resolveChannels(pref, intent.Channels)

	// 3. in-app store — only when in-app is enabled AND the type persists.
	// A dedup_key collision (redelivered dispatch) makes the whole fan-out a
	// no-op: the store row is the single idempotency gate.
	if channels.inApp && persistsInApp(intent.Type) {
		var inserted bool
		if err := s.runScoped(ctx, intent.UserID, func(ctx context.Context) error {
			ins, _, err := s.repo.InsertNotification(ctx, InsertNotificationInput{
				UserID:   intent.UserID,
				Type:     intent.Type,
				Title:    intent.Title,
				Body:     intent.Body,
				Data:     intent.Data,
				DedupKey: intent.DedupKey,
			})
			inserted = ins
			return err
		}); err != nil {
			return fmt.Errorf("notify:dispatch: insert: %w", err)
		}
		if !inserted {
			return nil // dedup conflict: skip channel fan-out too
		}
	}

	// 4. channel fan-out on the weight-1 "default" queue.
	if channels.email {
		if err := s.enqueueChannel(ctx, notifyapi.TaskEmail, intent); err != nil {
			return err
		}
	}
	if channels.push {
		if err := s.enqueueChannel(ctx, notifyapi.TaskWebPush, intent); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) enqueueChannel(ctx context.Context, taskType string, intent notifyapi.NotificationIntent) error {
	if s.enqueue == nil {
		// No enqueuer (e.g. the API server constructs notify for the read API
		// only). Channel fan-out is a worker responsibility; nothing to do.
		return nil
	}
	body, err := json.Marshal(channelPayload{
		UserID: intent.UserID,
		Type:   intent.Type,
		Title:  intent.Title,
		Data:   intent.Data,
	})
	if err != nil {
		return fmt.Errorf("notify:dispatch: marshal %s: %w", taskType, err)
	}
	_ = ctx
	if _, err := s.enqueue.Enqueue(asynq.NewTask(taskType, body), asynq.Queue("default")); err != nil {
		return fmt.Errorf("notify:dispatch: enqueue %s: %w", taskType, err)
	}
	return nil
}

// ── P0.1 store read API ─────────────────────────────────────────────

// ListResult is a page of notifications plus the unread badge count.
type ListResult struct {
	Items       []Notification
	UnreadCount int
	NextCursor  string
}

// List returns a keyset page (newest first) plus the unread_count badge.
func (s *Service) List(ctx context.Context, userID uuid.UUID, unreadOnly bool, cursor string, limit int) (ListResult, error) {
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}
	in := ListInput{
		UserID:     userID,
		UnreadOnly: unreadOnly,
		Limit:      limit + 1, // fetch one extra to detect a following page
	}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return ListResult{}, ErrBadCursor
		}
		in.CursorAt, in.CursorID = at, id
	}

	rows, err := s.repo.ListNotifications(ctx, in)
	if err != nil {
		return ListResult{}, err
	}

	var res ListResult
	if len(rows) > limit {
		res.NextCursor = encodeCursor(rows[limit-1])
		rows = rows[:limit]
	}
	res.Items = rows

	unread, err := s.repo.UnreadCount(ctx, userID)
	if err != nil {
		return ListResult{}, err
	}
	res.UnreadCount = unread
	return res, nil
}

// MarkRead marks one notification read (idempotent, owner-scoped) and returns
// the fresh unread_count. ErrNotFound for a missing/other-user id.
func (s *Service) MarkRead(ctx context.Context, userID, id uuid.UUID) (int, error) {
	found, err := s.repo.MarkRead(ctx, userID, id)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrNotFound
	}
	return s.repo.UnreadCount(ctx, userID)
}

// MarkAllRead marks unread rows at/older than the before watermark (absent = all)
// and returns the fresh unread_count.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID, before string) (int, error) {
	var at time.Time
	var id uuid.UUID
	if before != "" {
		decAt, decID, err := decodeCursor(before)
		if err != nil {
			return 0, ErrBadCursor
		}
		at, id = decAt, decID
	}
	if err := s.repo.MarkAllRead(ctx, userID, at, id); err != nil {
		return 0, err
	}
	return s.repo.UnreadCount(ctx, userID)
}

// ── P0.4 media:asset_ready consumer ─────────────────────────────────

// assetReadyEvent mirrors the media:asset_ready payload (events.md).
type assetReadyEvent struct {
	AssetID     string `json:"asset_id"`
	Kind        string `json:"kind"`
	OwnerUserID string `json:"owner_user_id"`
	Title       string `json:"title"`
	Origin      string `json:"origin"`
}

// OnAssetReady is the notify:on_asset_ready handler — the FIRST real event
// consumer. It skips imports (a zip import creates up to 300 assets; the bell
// must not flood) and otherwise builds a media.asset_ready intent and runs the
// P0.2 dispatch. dedup_key = asset_id makes an Asynq redelivery idempotent.
func (s *Service) OnAssetReady(ctx context.Context, task *asynq.Task) error {
	var ev assetReadyEvent
	if err := json.Unmarshal(task.Payload(), &ev); err != nil {
		log.Error().Err(err).Msg("notify:on_asset_ready: undecodable payload")
		return fmt.Errorf("notify:on_asset_ready: undecodable payload: %w", asynq.SkipRetry)
	}
	if ev.Origin == "import" {
		return nil // suppressed: SPEC-02 batch import
	}
	ownerID, err := uuid.Parse(ev.OwnerUserID)
	if err != nil || ownerID == uuid.Nil {
		log.Error().Str("owner", ev.OwnerUserID).Msg("notify:on_asset_ready: bad owner id")
		return fmt.Errorf("notify:on_asset_ready: bad owner id: %w", asynq.SkipRetry)
	}

	intent := notifyapi.NotificationIntent{
		UserID:   ownerID,
		Type:     notifyapi.TypeMediaAssetReady,
		Title:    assetReadyTitle(ev.Title, ev.Kind),
		DedupKey: ev.AssetID,
		Data: map[string]any{
			"asset_id": ev.AssetID,
			"kind":     ev.Kind,
			"href":     "/library/" + ev.AssetID, // click-through contract (P0.4)
		},
	}
	return s.dispatchIntent(ctx, intent)
}

func assetReadyTitle(title, kind string) string {
	if strings.TrimSpace(title) == "" {
		switch kind {
		case "image":
			return "Your photo is ready"
		case "audio":
			return "Your audio is ready"
		default:
			return "Your video is ready"
		}
	}
	return "\"" + title + "\" is ready to watch"
}

// ── P0.3 email channel ──────────────────────────────────────────────

// SendEmail is the notify:email handler: enforce the global hourly ceiling,
// resolve the recipient, render the type→template, and send. A ceiling breach
// or transport error returns a plain error so Asynq re-queues (channel pauses);
// an undecodable payload skips retry.
func (s *Service) SendEmail(ctx context.Context, task *asynq.Task) error {
	var p channelPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		log.Error().Err(err).Msg("notify:email: undecodable payload")
		return fmt.Errorf("notify:email: undecodable payload: %w", asynq.SkipRetry)
	}
	if s.email == nil || s.users == nil {
		return errors.New("notify:email: channel not configured")
	}
	if s.emailCeilingReached(ctx) {
		log.Error().Int("cap", s.emailHourlyCap).Msg("notify:email: hourly send ceiling reached — pausing channel")
		return errors.New("notify:email: hourly send ceiling reached")
	}

	addr, name, err := s.users.ResolveRecipient(ctx, p.UserID)
	if err != nil {
		return fmt.Errorf("notify:email: resolve recipient: %w", err)
	}
	if addr == "" {
		log.Warn().Str("user", p.UserID.String()).Msg("notify:email: no recipient address; dropping")
		return nil
	}

	msg := renderEmail(p.Type, p.Title, name, addr, p.Data)
	if err := s.email.Send(ctx, msg); err != nil {
		return fmt.Errorf("notify:email: send: %w", err)
	}
	s.recordEmailSent(ctx)
	return nil
}

// SendWebPush is the notify:web_push handler stub (P1.1). Registered so a
// push-enabled preference does not enqueue onto an unhandled task type (which
// Asynq would silently never process); delivery lands when P1.1 ships.
func (s *Service) SendWebPush(_ context.Context, _ *asynq.Task) error {
	// TODO(SPEC-04 P1.1): VAPID Web Push delivery + 410-Gone endpoint pruning.
	return nil
}

// PurgeOld is the notify:purge_old janitor body (P2). Registered on the shared
// scheduler by cmd/worker.
func (s *Service) PurgeOld(ctx context.Context) error {
	// TODO(SPEC-04 P2): batch the deletes + measure before promising "indexed".
	return s.repo.PurgeOld(ctx)
}

func (s *Service) emailHourKey() string {
	return "notify:email:sent:" + time.Now().UTC().Format("2006010215")
}

// emailCeilingReached reports whether the hour's send count is at/over the cap.
// Fail-open: a nil client or a Redis error never blocks a send.
func (s *Service) emailCeilingReached(ctx context.Context) bool {
	if s.redis == nil || s.emailHourlyCap <= 0 {
		return false
	}
	n, err := s.redis.Get(ctx, s.emailHourKey()).Int()
	if err != nil {
		return false // key missing (redis.Nil) or outage → proceed
	}
	return n >= s.emailHourlyCap
}

func (s *Service) recordEmailSent(ctx context.Context) {
	if s.redis == nil || s.emailHourlyCap <= 0 {
		return
	}
	key := s.emailHourKey()
	pipe := s.redis.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, time.Hour+5*time.Minute)
	_, _ = pipe.Exec(ctx)
}

// ── cursor helpers (keyset "<created_at>|<id>", base64url) ──────────

func encodeCursor(n Notification) string {
	return server.EncodeCursor(n.CreatedAt.UTC().Format(time.RFC3339Nano), n.ID)
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	key, id, err := server.DecodeCursor(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, key)
	if err != nil {
		return time.Time{}, uuid.Nil, server.ErrBadCursor
	}
	return at, id, nil
}
