package events

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

type fakeEnqueuer struct {
	tasks []*asynq.Task
	err   error
}

func (f *fakeEnqueuer) EnqueueContext(_ context.Context, t *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, t)
	return &asynq.TaskInfo{}, nil
}

// The load-bearing case: two distinct consumers of ONE event. A naive
// subscribe-by-task-type worker would panic (one handler per task type); the
// fan-out gives each consumer its own task type instead.
func TestPublishFansOutToEachConsumer(t *testing.T) {
	fe := &fakeEnqueuer{}
	p := NewPublisher(fe)
	p.Subscribe("media:asset_ready", "notify:on_asset_ready")
	p.Subscribe("media:asset_ready", "journal:stream_ingest")
	p.Subscribe("media:asset_ready", "notify:on_asset_ready") // duplicate → ignored

	err := p.Publish(context.Background(), "media:asset_ready",
		map[string]any{"asset_id": "abc", "origin": "upload"})
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if len(fe.tasks) != 2 {
		t.Fatalf("want 2 tasks enqueued (dup ignored), got %d", len(fe.tasks))
	}
	types := map[string]bool{}
	for _, task := range fe.tasks {
		types[task.Type()] = true
		var got map[string]any
		if err := json.Unmarshal(task.Payload(), &got); err != nil {
			t.Fatalf("payload not json: %v", err)
		}
		if got["asset_id"] != "abc" {
			t.Errorf("payload not delivered intact: %v", got)
		}
	}
	if !types["notify:on_asset_ready"] || !types["journal:stream_ingest"] {
		t.Errorf("missing consumer task types: %v", types)
	}
	if got := p.Consumers("media:asset_ready"); len(got) != 2 {
		t.Errorf("Consumers() = %v, want 2", got)
	}
}

func TestPublishNoSubscribersIsNoop(t *testing.T) {
	fe := &fakeEnqueuer{}
	p := NewPublisher(fe)
	if err := p.Publish(context.Background(), "unheard:event", nil); err != nil {
		t.Fatalf("publish with no subscribers must be nil, got %v", err)
	}
	if len(fe.tasks) != 0 {
		t.Fatalf("no tasks expected, got %d", len(fe.tasks))
	}
}

func TestPublishPropagatesEnqueueError(t *testing.T) {
	fe := &fakeEnqueuer{err: errors.New("redis down")}
	p := NewPublisher(fe)
	p.Subscribe("e", "c")
	if err := p.Publish(context.Background(), "e", nil); err == nil {
		t.Fatal("want error when the underlying enqueue fails")
	}
}
