package ports

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type APIKeyRepository interface {
	FindByTokenHash(ctx context.Context, tokenHash string) (domain.APIKey, error)
	Upsert(ctx context.Context, key domain.APIKey) error
}
