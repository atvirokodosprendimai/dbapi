package ports

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type RecordMutationStore interface {
	UpsertWithEvents(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error)
	DeleteWithEvents(ctx context.Context, tenantID, collection, id string, meta domain.MutationMetadata) (bool, error)
	Get(ctx context.Context, tenantID, collection, id string) (domain.Record, error)
	List(ctx context.Context, tenantID, collection string, filter domain.RecordListFilter) ([]domain.Record, error)
}

type AuditTrailRepository interface {
	List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditTrailEvent, error)
}

type OutboxRepository interface {
	FetchPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error)
	MarkDispatched(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, attempts int, nextAttemptAt string, errMsg string) error
	MarkDead(ctx context.Context, id int64, attempts int, errMsg string) error
}
