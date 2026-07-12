// Package events is the cross-cutting event fan-out helper (SPEC-01 P0.6).
//
// A producer calls Publish(ctx, name, payload). The publisher looks up every
// consumer subscribed to that event name and enqueues one Asynq task per
// consumer — each consumer gets its OWN task type. That is the whole point:
// Asynq's ServeMux allows exactly one handler per task type, so if two modules
// both tried to handle the raw event as a single task type the worker would
// panic at startup (see docs/reference/events.md "Delivery mechanics"). By
// giving each consumer a distinct task type and fanning out here, an event can
// gain a second, third consumer without touching the producer.
//
// Wiring contract: construct one *Publisher per binary that emits or fans out
// events (cmd/api, cmd/worker), call Subscribe(...) for every event→consumer
// edge at startup, then hand the Publisher to producers. Publishing an event
// with no subscribers is a deliberate no-op — modules emit from day one even
// before a consumer exists (ADR-08).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/hibiken/asynq"
)

// Enqueuer is the subset of *asynq.Client the publisher needs. Kept minimal so
// callers can fake it in tests. *asynq.Client satisfies it.
type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// subscription binds one consumer task type (+ its enqueue options) to an event.
type subscription struct {
	taskType string
	opts     []asynq.Option
}

// Publisher fans one domain event out to N consumer tasks.
type Publisher struct {
	client Enqueuer
	mu     sync.RWMutex
	subs   map[string][]subscription
}

// NewPublisher builds a Publisher over an Asynq client (or any Enqueuer).
func NewPublisher(client Enqueuer) *Publisher {
	return &Publisher{client: client, subs: make(map[string][]subscription)}
}

// Subscribe registers consumerTaskType as a consumer of eventName. Call at
// wiring time, before Publish. Registering the same (event, task) pair twice is
// harmless (ignored) so double-wiring across binaries can't duplicate delivery.
// opts are applied to every task enqueued for this edge (e.g. asynq.Queue).
func (p *Publisher) Subscribe(eventName, consumerTaskType string, opts ...asynq.Option) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, s := range p.subs[eventName] {
		if s.taskType == consumerTaskType {
			return
		}
	}
	p.subs[eventName] = append(p.subs[eventName], subscription{taskType: consumerTaskType, opts: opts})
}

// Publish enqueues one task per consumer subscribed to eventName, each carrying
// the same JSON-marshalled payload. No subscribers → no-op (nil error). The
// payload is marshalled once; an enqueue failure aborts and returns (at-least-
// once delivery means a partial fan-out is retried by the caller's own path).
func (p *Publisher) Publish(ctx context.Context, eventName string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("events: marshal %q payload: %w", eventName, err)
	}

	p.mu.RLock()
	subs := append([]subscription(nil), p.subs[eventName]...)
	p.mu.RUnlock()

	for _, s := range subs {
		task := asynq.NewTask(s.taskType, body)
		if _, err := p.client.EnqueueContext(ctx, task, s.opts...); err != nil {
			return fmt.Errorf("events: fan-out %s→%s: %w", eventName, s.taskType, err)
		}
	}
	return nil
}

// Consumers returns the consumer task types registered for an event, sorted —
// for startup logging / introspection.
func (p *Publisher) Consumers(eventName string) []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]string, 0, len(p.subs[eventName]))
	for _, s := range p.subs[eventName] {
		out = append(out, s.taskType)
	}
	sort.Strings(out)
	return out
}
