package usecase

import (
	"context"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type replayAuditRepo struct {
	events []domain.AuditTrailEvent
}

func (r *replayAuditRepo) List(_ context.Context, filter domain.AuditFilter) ([]domain.AuditTrailEvent, error) {
	items := make([]domain.AuditTrailEvent, 0, filter.Limit)
	for _, e := range r.events {
		if e.TenantID != filter.TenantID {
			continue
		}
		if filter.AfterID > 0 && !(e.ID < filter.AfterID) {
			continue
		}
		items = append(items, e)
		if len(items) >= filter.Limit {
			break
		}
	}
	return items, nil
}

func TestReplayTenantEvents(t *testing.T) {
	audit := NewAuditService(&replayAuditRepo{events: []domain.AuditTrailEvent{
		{ID: 3, EventID: "e3", TenantID: "t1", Action: "record.updated", SchemaVersion: 1, OccurredAt: time.Now()},
		{ID: 2, EventID: "e2", TenantID: "t1", Action: "record.updated", SchemaVersion: 1, OccurredAt: time.Now()},
	}})

	seen := 0
	err := ReplayTenantEvents(context.Background(), audit, NewEventCodec(), "t1", 100, func(ReplayEvent) error {
		seen++
		return nil
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if seen != 2 {
		t.Fatalf("expected 2 events, got %d", seen)
	}
}
