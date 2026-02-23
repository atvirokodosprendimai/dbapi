package usecase

import (
	"context"
	"encoding/json"
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

func TestReplayTenantEventsProjectionFixtures(t *testing.T) {
	fixtures := []struct {
		name    string
		events  []domain.AuditTrailEvent
		wantIDs map[string]string
	}{
		{
			name: "create update delete sequence",
			events: []domain.AuditTrailEvent{
				{ID: 30, EventID: "e30", TenantID: "t1", AggregateType: "users", AggregateID: "u1", Action: "record.created", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"A"}`), OccurredAt: time.Now()},
				{ID: 20, EventID: "e20", TenantID: "t1", AggregateType: "users", AggregateID: "u1", Action: "record.updated", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"B"}`), OccurredAt: time.Now()},
				{ID: 10, EventID: "e10", TenantID: "t1", AggregateType: "users", AggregateID: "u1", Action: "record.deleted", SchemaVersion: 1, OccurredAt: time.Now()},
			},
			wantIDs: map[string]string{},
		},
		{
			name: "create and update multiple records",
			events: []domain.AuditTrailEvent{
				{ID: 60, EventID: "e60", TenantID: "t1", AggregateType: "users", AggregateID: "u1", Action: "record.created", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"A"}`), OccurredAt: time.Now()},
				{ID: 50, EventID: "e50", TenantID: "t1", AggregateType: "users", AggregateID: "u2", Action: "record.created", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"B"}`), OccurredAt: time.Now()},
				{ID: 40, EventID: "e40", TenantID: "t1", AggregateType: "users", AggregateID: "u1", Action: "record.updated", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"A2"}`), OccurredAt: time.Now()},
			},
			wantIDs: map[string]string{"u1": "A2", "u2": "B"},
		},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			audit := NewAuditService(&replayAuditRepo{events: tt.events})
			projection := map[string]string{}

			err := ReplayTenantEvents(context.Background(), audit, NewEventCodec(), "t1", 2, func(ev ReplayEvent) error {
				switch ev.Envelope.EventType {
				case "record.created", "record.updated":
					var payload map[string]any
					if err := json.Unmarshal(ev.Envelope.Payload, &payload); err != nil {
						return err
					}
					name, _ := payload["name"].(string)
					projection[ev.Envelope.AggregateID] = name
				case "record.deleted":
					delete(projection, ev.Envelope.AggregateID)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("replay failed: %v", err)
			}

			if len(projection) != len(tt.wantIDs) {
				t.Fatalf("unexpected projection size: got %d want %d", len(projection), len(tt.wantIDs))
			}
			for id, wantName := range tt.wantIDs {
				if projection[id] != wantName {
					t.Fatalf("unexpected projection value for %s: got %q want %q", id, projection[id], wantName)
				}
			}
		})
	}
}

func TestReplayTenantEventsCrossTenantIsolation(t *testing.T) {
	audit := NewAuditService(&replayAuditRepo{events: []domain.AuditTrailEvent{
		{ID: 5, EventID: "e5", TenantID: "t1", AggregateType: "users", AggregateID: "u1", Action: "record.created", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"A"}`), OccurredAt: time.Now()},
		{ID: 4, EventID: "e4", TenantID: "t2", AggregateType: "users", AggregateID: "u9", Action: "record.created", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"X"}`), OccurredAt: time.Now()},
		{ID: 3, EventID: "e3", TenantID: "t1", AggregateType: "users", AggregateID: "u2", Action: "record.updated", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"B"}`), OccurredAt: time.Now()},
		{ID: 2, EventID: "e2", TenantID: "t2", AggregateType: "users", AggregateID: "u8", Action: "record.updated", SchemaVersion: 1, AfterJSON: json.RawMessage(`{"name":"Y"}`), OccurredAt: time.Now()},
	}})

	seen := []string{}
	err := ReplayTenantEvents(context.Background(), audit, NewEventCodec(), "t1", 1, func(ev ReplayEvent) error {
		seen = append(seen, ev.Envelope.EventID)
		if ev.Envelope.TenantID != "t1" {
			t.Fatalf("cross-tenant leak: got tenant %s", ev.Envelope.TenantID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if len(seen) != 2 {
		t.Fatalf("expected 2 tenant events, got %d (%v)", len(seen), seen)
	}
	if seen[0] != "e5" || seen[1] != "e3" {
		t.Fatalf("unexpected replay order for tenant stream: %v", seen)
	}
}
