package ports

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type EventPublisher interface {
	Publish(ctx context.Context, topic string, event domain.EventEnvelope) error
}
