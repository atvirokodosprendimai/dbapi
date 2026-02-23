package ports

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type CollectionSchemaRepository interface {
	Upsert(ctx context.Context, schema domain.CollectionSchema) (domain.CollectionSchema, error)
	Get(ctx context.Context, tenantID, collection string) (domain.CollectionSchema, error)
	Delete(ctx context.Context, tenantID, collection string) (bool, error)
}
