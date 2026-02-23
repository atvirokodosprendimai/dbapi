package usecase

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

type AuditService struct {
	repo ports.AuditTrailRepository
}

func NewAuditService(repo ports.AuditTrailRepository) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditTrailEvent, error) {
	if err := domain.ValidateKey(filter.TenantID); err != nil {
		return nil, err
	}
	if filter.AggregateType != "" {
		if err := domain.ValidateCategory(filter.AggregateType); err != nil {
			return nil, err
		}
	}
	if filter.AggregateID != "" {
		if err := domain.ValidateKey(filter.AggregateID); err != nil {
			return nil, err
		}
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}
	return s.repo.List(ctx, filter)
}
