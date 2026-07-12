package notify

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	notifyapi "github.com/portal/backend/internal/modules/notify/api"
)

// ── fakes ───────────────────────────────────────────────────────────

type fakeRepo struct {
	rows  []Notification
	dedup map[string]bool       // "<user>|<type>|<dedup>" already present
	prefs map[string]Preference // keyed by "<user>|<type>"; absent → default
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{dedup: map[string]bool{}, prefs: map[string]Preference{}}
}

func prefKey(user uuid.UUID, typ string) string { return user.String() + "|" + typ }

func (r *fakeRepo) InsertNotification(_ context.Context, in InsertNotificationInput) (bool, Notification, error) {
	if in.DedupKey != "" {
		k := in.UserID.String() + "|" + in.Type + "|" + in.DedupKey
		if r.dedup[k] {
			return false, Notification{}, nil // simulate the partial unique index
		}
		r.dedup[k] = true
	}
	n := Notification{
		ID: uuid.New(), UserID: in.UserID, Type: in.Type, Title: in.Title,
		Body: in.Body, Data: in.Data, DedupKey: in.DedupKey, CreatedAt: time.Now(),
	}
	r.rows = append(r.rows, n)
	return true, n, nil
}

func (r *fakeRepo) ListNotifications(_ context.Context, in ListInput) ([]Notification, error) {
	var out []Notification
	for _, n := range r.rows {
		if n.UserID == in.UserID && (!in.UnreadOnly || n.ReadAt == nil) {
			out = append(out, n)
		}
	}
	return out, nil
}

func (r *fakeRepo) UnreadCount(_ context.Context, userID uuid.UUID) (int, error) {
	c := 0
	for _, n := range r.rows {
		if n.UserID == userID && n.ReadAt == nil {
			c++
		}
	}
	return c, nil
}

func (r *fakeRepo) MarkRead(_ context.Context, userID, id uuid.UUID) (bool, error) {
	for i := range r.rows {
		if r.rows[i].ID == id && r.rows[i].UserID == userID {
			if r.rows[i].ReadAt == nil {
				now := time.Now()
				r.rows[i].ReadAt = &now
			}
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeRepo) MarkAllRead(_ context.Context, userID uuid.UUID, _ time.Time, _ uuid.UUID) error {
	now := time.Now()
	for i := range r.rows {
		if r.rows[i].UserID == userID && r.rows[i].ReadAt == nil {
			r.rows[i].ReadAt = &now
		}
	}
	return nil
}

func (r *fakeRepo) GetPreference(_ context.Context, userID uuid.UUID, typ string) (Preference, bool, error) {
	p, ok := r.prefs[prefKey(userID, typ)]
	return p, ok, nil
}

func (r *fakeRepo) PurgeOld(_ context.Context) error { return nil }

type fakeEnqueuer struct{ tasks []*asynq.Task }

func (e *fakeEnqueuer) Enqueue(t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	e.tasks = append(e.tasks, t)
	return &asynq.TaskInfo{}, nil
}

func (e *fakeEnqueuer) typeCount(taskType string) int {
	n := 0
	for _, t := range e.tasks {
		if t.Type() == taskType {
			n++
		}
	}
	return n
}

func newDispatchSvc() (*Service, *fakeRepo, *fakeEnqueuer) {
	repo := newFakeRepo()
	enq := &fakeEnqueuer{}
	return &Service{repo: repo, enqueue: enq}, repo, enq
}

func dispatchTask(t *testing.T, intent notifyapi.NotificationIntent) *asynq.Task {
	t.Helper()
	body, err := json.Marshal(intent)
	if err != nil {
		t.Fatalf("marshal intent: %v", err)
	}
	return asynq.NewTask(notifyapi.TaskDispatch, body)
}

// ── tests ───────────────────────────────────────────────────────────

// Default prefs (no pref row) → exactly one in-app row, no channel tasks.
func TestDispatchDefaultPrefs(t *testing.T) {
	svc, repo, enq := newDispatchSvc()
	user := uuid.New()

	err := svc.Dispatch(context.Background(), dispatchTask(t, notifyapi.NotificationIntent{
		UserID: user, Type: notifyapi.TypeMediaAssetReady, Title: "Ready",
	}))
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(repo.rows))
	}
	if enq.typeCount(notifyapi.TaskEmail) != 0 {
		t.Fatalf("email tasks = %d, want 0", enq.typeCount(notifyapi.TaskEmail))
	}
}

// Email preference on → one in-app row AND one notify:email task.
func TestDispatchEmailPref(t *testing.T) {
	svc, repo, enq := newDispatchSvc()
	user := uuid.New()
	repo.prefs[prefKey(user, notifyapi.TypeMediaAssetReady)] = Preference{InApp: true, Email: true}

	if err := svc.Dispatch(context.Background(), dispatchTask(t, notifyapi.NotificationIntent{
		UserID: user, Type: notifyapi.TypeMediaAssetReady, Title: "Ready",
	})); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(repo.rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(repo.rows))
	}
	if enq.typeCount(notifyapi.TaskEmail) != 1 {
		t.Fatalf("email tasks = %d, want 1", enq.typeCount(notifyapi.TaskEmail))
	}
}

// A muted MUTABLE type → nothing (no row, no channel tasks).
func TestDispatchMutedMutable(t *testing.T) {
	svc, repo, enq := newDispatchSvc()
	user := uuid.New()
	repo.prefs[prefKey(user, notifyapi.TypeMediaAssetReady)] = Preference{InApp: true, Email: true, Muted: true}

	if err := svc.Dispatch(context.Background(), dispatchTask(t, notifyapi.NotificationIntent{
		UserID: user, Type: notifyapi.TypeMediaAssetReady, Title: "Ready",
	})); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(repo.rows) != 0 || len(enq.tasks) != 0 {
		t.Fatalf("muted mutable delivered rows=%d tasks=%d, want 0/0", len(repo.rows), len(enq.tasks))
	}
}

// Non-mutable password_reset with a channels override → email task despite the
// email-off default (and muted), and NO in-app row (it never persists).
func TestDispatchNonMutableOverride(t *testing.T) {
	svc, repo, enq := newDispatchSvc()
	user := uuid.New()
	// Even muted, the reset must still send.
	repo.prefs[prefKey(user, notifyapi.TypePasswordReset)] = Preference{Muted: true}

	if err := svc.Dispatch(context.Background(), dispatchTask(t, notifyapi.NotificationIntent{
		UserID:   user,
		Type:     notifyapi.TypePasswordReset,
		Title:    "Reset your password",
		Channels: []string{notifyapi.ChannelEmail},
		Data:     map[string]any{"reset_url": "https://x/reset?token=abc"},
	})); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("password_reset persisted %d rows, want 0", len(repo.rows))
	}
	if enq.typeCount(notifyapi.TaskEmail) != 1 {
		t.Fatalf("email tasks = %d, want 1", enq.typeCount(notifyapi.TaskEmail))
	}
}

// A redelivered dispatch (same dedup_key) → exactly one row, and the channel
// fan-out is skipped on the redelivery (store row is the idempotency gate).
func TestDispatchDedupRedelivery(t *testing.T) {
	svc, repo, enq := newDispatchSvc()
	user := uuid.New()
	repo.prefs[prefKey(user, notifyapi.TypeMediaAssetReady)] = Preference{InApp: true, Email: true}

	intent := notifyapi.NotificationIntent{
		UserID: user, Type: notifyapi.TypeMediaAssetReady, Title: "Ready", DedupKey: "asset-123",
	}
	for i := 0; i < 3; i++ {
		if err := svc.Dispatch(context.Background(), dispatchTask(t, intent)); err != nil {
			t.Fatalf("dispatch %d: %v", i, err)
		}
	}
	if len(repo.rows) != 1 {
		t.Fatalf("rows = %d, want 1 (dedup)", len(repo.rows))
	}
	if enq.typeCount(notifyapi.TaskEmail) != 1 {
		t.Fatalf("email tasks = %d, want 1 (fan-out skipped on redelivery)", enq.typeCount(notifyapi.TaskEmail))
	}
}

// A malformed intent (missing user_id/type) fails fast with asynq.SkipRetry.
func TestDispatchMalformedSkipsRetry(t *testing.T) {
	svc, repo, enq := newDispatchSvc()

	// valid JSON, but no user_id / type
	err := svc.Dispatch(context.Background(), dispatchTask(t, notifyapi.NotificationIntent{Title: "x"}))
	if err == nil || !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v, want wrapped asynq.SkipRetry", err)
	}
	if len(repo.rows) != 0 || len(enq.tasks) != 0 {
		t.Fatalf("malformed intent delivered something: rows=%d tasks=%d", len(repo.rows), len(enq.tasks))
	}

	// undecodable payload → also SkipRetry
	bad := asynq.NewTask(notifyapi.TaskDispatch, []byte("{not json"))
	if err := svc.Dispatch(context.Background(), bad); err == nil || !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("undecodable err = %v, want wrapped asynq.SkipRetry", err)
	}
}

// The media:asset_ready consumer skips imports and otherwise dispatches a
// media.asset_ready intent (dedup_key = asset_id, default prefs → one row).
func TestOnAssetReady(t *testing.T) {
	svc, repo, _ := newDispatchSvc()
	owner := uuid.New()

	mk := func(origin string) *asynq.Task {
		body, _ := json.Marshal(assetReadyEvent{
			AssetID: uuid.New().String(), Kind: "video", OwnerUserID: owner.String(),
			Title: "Clip", Origin: origin,
		})
		return asynq.NewTask(notifyapi.TaskOnAssetReady, body)
	}

	// import → suppressed
	if err := svc.OnAssetReady(context.Background(), mk("import")); err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(repo.rows) != 0 {
		t.Fatalf("import produced %d rows, want 0", len(repo.rows))
	}
	// upload → one row for the owner
	if err := svc.OnAssetReady(context.Background(), mk("upload")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if len(repo.rows) != 1 || repo.rows[0].UserID != owner {
		t.Fatalf("upload rows = %+v, want 1 for owner", repo.rows)
	}
	if repo.rows[0].Type != notifyapi.TypeMediaAssetReady {
		t.Fatalf("type = %q", repo.rows[0].Type)
	}
	if href, _ := repo.rows[0].Data["href"].(string); href == "" {
		t.Fatalf("missing data.href click-through, data = %v", repo.rows[0].Data)
	}
}
