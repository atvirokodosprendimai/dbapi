package usecase

import (
	"context"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

type KVService struct {
	repo ports.KVRepository
}

func NewKVService(repo ports.KVRepository) *KVService {
	return &KVService{repo: repo}
}

func (s *KVService) Upsert(ctx context.Context, item domain.Item) (domain.Item, error) {
	if err := item.Validate(); err != nil {
		return domain.Item{}, err
	}
	return s.repo.Upsert(ctx, item)
}

func (s *KVService) Get(ctx context.Context, key string) (domain.Item, error) {
	if err := domain.ValidateKey(key); err != nil {
		return domain.Item{}, err
	}
	return s.repo.Get(ctx, key)
}

func (s *KVService) Delete(ctx context.Context, key string) (bool, error) {
	if err := domain.ValidateKey(key); err != nil {
		return false, err
	}
	return s.repo.Delete(ctx, key)
}

func (s *KVService) Scan(ctx context.Context, filter domain.ScanFilter) ([]domain.Item, error) {
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}
	return s.repo.Scan(ctx, filter)
}
