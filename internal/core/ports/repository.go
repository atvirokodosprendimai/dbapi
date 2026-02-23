package ports

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type KVRepository interface {
	Upsert(ctx context.Context, item domain.Item) (domain.Item, error)
	Get(ctx context.Context, key string) (domain.Item, error)
	Delete(ctx context.Context, key string) (bool, error)
	Scan(ctx context.Context, filter domain.ScanFilter) ([]domain.Item, error)
}
