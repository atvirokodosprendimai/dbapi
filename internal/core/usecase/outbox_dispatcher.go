package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

type OutboxDispatcher struct {
	repo      ports.OutboxRepository
	publisher ports.EventPublisher
	interval  time.Duration
	batchSize int
	maxRetry  int

	mu     sync.Mutex
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewOutboxDispatcher(repo ports.OutboxRepository, publisher ports.EventPublisher, interval time.Duration, batchSize int) *OutboxDispatcher {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &OutboxDispatcher{repo: repo, publisher: publisher, interval: interval, batchSize: batchSize, maxRetry: 5}
}

func (d *OutboxDispatcher) Start(parent context.Context) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	d.cancel = cancel
	d.wg.Add(1)
	go d.loop(ctx)
}

func (d *OutboxDispatcher) Close() error {
	d.mu.Lock()
	cancel := d.cancel
	d.cancel = nil
	d.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	d.wg.Wait()
	return nil
}

func (d *OutboxDispatcher) loop(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		if err := d.dispatchBatch(ctx); err != nil {
			log.Printf("outbox dispatch batch error: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *OutboxDispatcher) dispatchBatch(ctx context.Context) error {
	events, err := d.repo.FetchPending(ctx, d.batchSize)
	if err != nil {
		return err
	}

	for _, event := range events {
		var envelope domain.EventEnvelope
		if err := json.Unmarshal(event.PayloadJSON, &envelope); err != nil {
			_ = d.markFailure(ctx, event, fmt.Sprintf("decode payload: %v", err))
			continue
		}

		if err := d.publisher.Publish(ctx, event.Topic, envelope); err != nil {
			_ = d.markFailure(ctx, event, err.Error())
			continue
		}

		if err := d.repo.MarkDispatched(ctx, event.ID); err != nil {
			return err
		}
	}

	return nil
}

func (d *OutboxDispatcher) markFailure(ctx context.Context, event domain.OutboxEvent, errMsg string) error {
	attempts := event.Attempts + 1
	if attempts >= d.maxRetry {
		return d.repo.MarkDead(ctx, event.ID, attempts, errMsg)
	}
	next := time.Now().UTC().Add(backoffDuration(attempts)).Format(time.RFC3339Nano)
	return d.repo.MarkFailed(ctx, event.ID, attempts, next, errMsg)
}

func backoffDuration(attempt int) time.Duration {
	if attempt <= 1 {
		return 1 * time.Second
	}
	d := time.Duration(attempt*attempt) * time.Second
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}
