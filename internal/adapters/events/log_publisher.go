package events

import (
	"context"
	"log"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type LogPublisher struct{}

func NewLogPublisher() *LogPublisher {
	return &LogPublisher{}
}

func (p *LogPublisher) Publish(_ context.Context, topic string, event domain.EventEnvelope) error {
	log.Printf("outbox publish topic=%s event_id=%s event_type=%s tenant=%s aggregate=%s/%s version=%d", topic, event.EventID, event.EventType, event.TenantID, event.AggregateType, event.AggregateID, event.AggregateVersion)
	return nil
}
