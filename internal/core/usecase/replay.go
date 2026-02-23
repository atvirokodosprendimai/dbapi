package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type ReplayEvent struct {
	Envelope domain.EventEnvelope `json:"envelope"`
	AuditID  int64                `json:"audit_id"`
}

func ReplayTenantEvents(ctx context.Context, audit *AuditService, codec *EventCodec, tenantID string, batchSize int, applyFn func(ReplayEvent) error) error {
	afterID := int64(0)
	for {
		events, err := audit.List(ctx, domain.AuditFilter{TenantID: tenantID, AfterID: afterID, Limit: batchSize})
		if err != nil {
			return fmt.Errorf("list audit events: %w", err)
		}
		if len(events) == 0 {
			return nil
		}

		for _, e := range events {
			envelope := domain.EventEnvelope{
				EventID:          e.EventID,
				EventType:        e.Action,
				SchemaVersion:    e.SchemaVersion,
				TenantID:         e.TenantID,
				AggregateType:    e.AggregateType,
				AggregateID:      e.AggregateID,
				AggregateVersion: e.AggregateVersion,
				OccurredAt:       e.OccurredAt,
				CorrelationID:    e.CorrelationID,
				CausationID:      e.CausationID,
				Actor:            e.Actor,
				Source:           e.Source,
			}
			if len(e.AfterJSON) > 0 {
				envelope.Payload = e.AfterJSON
			} else {
				envelope.Payload = json.RawMessage(`{}`)
			}

			normalized, err := codec.Normalize(envelope)
			if err != nil {
				return fmt.Errorf("normalize event %s: %w", e.EventID, err)
			}

			if err := applyFn(ReplayEvent{Envelope: normalized, AuditID: e.ID}); err != nil {
				return fmt.Errorf("apply replay event %s: %w", e.EventID, err)
			}
			afterID = e.ID
		}
	}
}
