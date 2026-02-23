package ports

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type AuditRepository interface {
	Log(ctx context.Context, event domain.AuditEvent) error
}
