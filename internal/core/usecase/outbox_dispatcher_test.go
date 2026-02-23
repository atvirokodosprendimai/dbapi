package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type outboxRepoStub struct {
	events []domain.OutboxEvent

	fetchLimits []int
	failed      []failedMark
	dead        []deadMark
	dispatched  []int64
}

type failedMark struct {
	id           int64
	attempts     int
	nextAttempt  string
	errorMessage string
}

type deadMark struct {
	id           int64
	attempts     int
	errorMessage string
}

func (r *outboxRepoStub) FetchPending(_ context.Context, limit int) ([]domain.OutboxEvent, error) {
	r.fetchLimits = append(r.fetchLimits, limit)
	out := make([]domain.OutboxEvent, 0, limit)
	now := time.Now().UTC()
	for _, e := range r.events {
		if e.Status != "pending" {
			continue
		}
		if e.NextAttemptAt.After(now) {
			continue
		}
		out = append(out, e)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (r *outboxRepoStub) MarkDispatched(_ context.Context, id int64) error {
	r.dispatched = append(r.dispatched, id)
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Status = "dispatched"
			now := time.Now().UTC()
			r.events[i].DispatchedAt = &now
			return nil
		}
	}
	return errors.New("unknown outbox id")
}

func (r *outboxRepoStub) MarkFailed(_ context.Context, id int64, attempts int, nextAttemptAt string, errMsg string) error {
	r.failed = append(r.failed, failedMark{id: id, attempts: attempts, nextAttempt: nextAttemptAt, errorMessage: errMsg})
	parsed, err := time.Parse(time.RFC3339Nano, nextAttemptAt)
	if err != nil {
		return err
	}
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Attempts = attempts
			r.events[i].NextAttemptAt = parsed
			r.events[i].LastError = errMsg
			return nil
		}
	}
	return errors.New("unknown outbox id")
}

func (r *outboxRepoStub) MarkDead(_ context.Context, id int64, attempts int, errMsg string) error {
	r.dead = append(r.dead, deadMark{id: id, attempts: attempts, errorMessage: errMsg})
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Status = "dead"
			r.events[i].Attempts = attempts
			r.events[i].LastError = errMsg
			return nil
		}
	}
	return errors.New("unknown outbox id")
}

type publisherStub struct {
	errByID   map[string]error
	published []domain.EventEnvelope
}

func (p *publisherStub) Publish(_ context.Context, _ string, event domain.EventEnvelope) error {
	p.published = append(p.published, event)
	if err, ok := p.errByID[event.EventID]; ok {
		return err
	}
	return nil
}

func TestOutboxDispatcherDispatchBatchSuccess(t *testing.T) {
	env := domain.EventEnvelope{EventID: "e1", EventType: "record.created", SchemaVersion: 1}
	payload, _ := json.Marshal(env)
	repo := &outboxRepoStub{events: []domain.OutboxEvent{{
		ID:            1,
		EventID:       "e1",
		Status:        "pending",
		NextAttemptAt: time.Now().UTC().Add(-time.Second),
		PayloadJSON:   payload,
		Topic:         "events.t1.record.created",
	}}}
	pub := &publisherStub{}
	d := NewOutboxDispatcher(repo, pub, time.Second, 10)

	if err := d.dispatchBatch(context.Background()); err != nil {
		t.Fatalf("dispatch batch: %v", err)
	}

	if len(repo.fetchLimits) != 1 || repo.fetchLimits[0] != 10 {
		t.Fatalf("expected fetch limit 10, got %v", repo.fetchLimits)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected one published event, got %d", len(pub.published))
	}
	if len(repo.dispatched) != 1 || repo.dispatched[0] != 1 {
		t.Fatalf("expected id=1 marked dispatched, got %v", repo.dispatched)
	}
	if len(repo.failed) != 0 || len(repo.dead) != 0 {
		t.Fatalf("expected no failures/dead marks, got failed=%d dead=%d", len(repo.failed), len(repo.dead))
	}
}

func TestOutboxDispatcherPublishFailureMarksFailedWithRetry(t *testing.T) {
	env := domain.EventEnvelope{EventID: "e2", EventType: "record.updated", SchemaVersion: 1}
	payload, _ := json.Marshal(env)
	repo := &outboxRepoStub{events: []domain.OutboxEvent{{
		ID:            2,
		EventID:       "e2",
		Status:        "pending",
		Attempts:      0,
		NextAttemptAt: time.Now().UTC().Add(-time.Second),
		PayloadJSON:   payload,
		Topic:         "events.t1.record.updated",
	}}}
	pub := &publisherStub{errByID: map[string]error{"e2": errors.New("publisher down")}}
	d := NewOutboxDispatcher(repo, pub, time.Second, 10)

	if err := d.dispatchBatch(context.Background()); err != nil {
		t.Fatalf("dispatch batch: %v", err)
	}

	if len(repo.failed) != 1 {
		t.Fatalf("expected one failed mark, got %d", len(repo.failed))
	}
	if repo.failed[0].attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", repo.failed[0].attempts)
	}
	if repo.failed[0].errorMessage != "publisher down" {
		t.Fatalf("unexpected error message: %q", repo.failed[0].errorMessage)
	}
	if len(repo.dispatched) != 0 {
		t.Fatalf("expected no dispatched marks, got %v", repo.dispatched)
	}
	if len(repo.dead) != 0 {
		t.Fatalf("expected no dead marks, got %v", repo.dead)
	}
}

func TestOutboxDispatcherRetryBudgetMovesToDead(t *testing.T) {
	env := domain.EventEnvelope{EventID: "e3", EventType: "record.updated", SchemaVersion: 1}
	payload, _ := json.Marshal(env)
	repo := &outboxRepoStub{events: []domain.OutboxEvent{{
		ID:            3,
		EventID:       "e3",
		Status:        "pending",
		Attempts:      4,
		NextAttemptAt: time.Now().UTC().Add(-time.Second),
		PayloadJSON:   payload,
		Topic:         "events.t1.record.updated",
	}}}
	pub := &publisherStub{errByID: map[string]error{"e3": errors.New("still failing")}}
	d := NewOutboxDispatcher(repo, pub, time.Second, 10)

	if err := d.dispatchBatch(context.Background()); err != nil {
		t.Fatalf("dispatch batch: %v", err)
	}

	if len(repo.dead) != 1 {
		t.Fatalf("expected one dead mark, got %d", len(repo.dead))
	}
	if repo.dead[0].attempts != 5 {
		t.Fatalf("expected attempts=5, got %d", repo.dead[0].attempts)
	}
	if len(repo.failed) != 0 {
		t.Fatalf("expected no failed marks when dead-lettered, got %d", len(repo.failed))
	}
}

func TestOutboxDispatcherRestartResumeDispatchesRemainingPending(t *testing.T) {
	env1 := domain.EventEnvelope{EventID: "e4", EventType: "record.created", SchemaVersion: 1}
	env2 := domain.EventEnvelope{EventID: "e5", EventType: "record.updated", SchemaVersion: 1}
	payload1, _ := json.Marshal(env1)
	payload2, _ := json.Marshal(env2)
	repo := &outboxRepoStub{events: []domain.OutboxEvent{
		{ID: 4, EventID: "e4", Status: "pending", NextAttemptAt: time.Now().UTC().Add(-time.Second), PayloadJSON: payload1, Topic: "events.t1.record.created"},
		{ID: 5, EventID: "e5", Status: "pending", NextAttemptAt: time.Now().UTC().Add(-time.Second), PayloadJSON: payload2, Topic: "events.t1.record.updated"},
	}}

	pub := &publisherStub{errByID: map[string]error{"e4": errors.New("transient")}}
	d1 := NewOutboxDispatcher(repo, pub, time.Second, 10)
	if err := d1.dispatchBatch(context.Background()); err != nil {
		t.Fatalf("first dispatch batch: %v", err)
	}
	if len(repo.dispatched) != 1 || repo.dispatched[0] != 5 {
		t.Fatalf("expected only id=5 dispatched after first run, got %v", repo.dispatched)
	}

	repo.events[0].NextAttemptAt = time.Now().UTC().Add(-time.Second)
	pub.errByID = map[string]error{}
	d2 := NewOutboxDispatcher(repo, pub, time.Second, 10)
	if err := d2.dispatchBatch(context.Background()); err != nil {
		t.Fatalf("second dispatch batch: %v", err)
	}

	if len(repo.dispatched) != 2 {
		t.Fatalf("expected two dispatched marks after resume, got %v", repo.dispatched)
	}
	if repo.dispatched[1] != 4 {
		t.Fatalf("expected resumed dispatch of id=4, got %d", repo.dispatched[1])
	}
}
