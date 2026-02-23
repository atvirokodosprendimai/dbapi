package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

type RecordService struct {
	store ports.RecordMutationStore
}

func NewRecordService(store ports.RecordMutationStore) *RecordService {
	return &RecordService{store: store}
}

func (s *RecordService) Upsert(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error) {
	if err := rec.Validate(); err != nil {
		return domain.Record{}, err
	}
	return s.store.UpsertWithEvents(ctx, rec, meta)
}

func (s *RecordService) Get(ctx context.Context, tenantID, collection, id string) (domain.Record, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return domain.Record{}, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return domain.Record{}, err
	}
	if err := domain.ValidateKey(id); err != nil {
		return domain.Record{}, err
	}
	return s.store.Get(ctx, tenantID, collection, id)
}

func (s *RecordService) Delete(ctx context.Context, tenantID, collection, id string, meta domain.MutationMetadata) (bool, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return false, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return false, err
	}
	if err := domain.ValidateKey(id); err != nil {
		return false, err
	}
	return s.store.DeleteWithEvents(ctx, tenantID, collection, id, meta)
}

func (s *RecordService) List(ctx context.Context, tenantID, collection string, filter domain.RecordListFilter) ([]domain.Record, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return nil, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return nil, err
	}
	if filter.Prefix != "" {
		if err := domain.ValidateKey(filter.Prefix); err != nil {
			return nil, err
		}
	}
	if filter.After != "" {
		if err := domain.ValidateKey(filter.After); err != nil {
			return nil, err
		}
	}
	if err := filter.JSON.Validate(); err != nil {
		return nil, err
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}
	if filter.Limit > 1000 {
		filter.Limit = 1000
	}
	if filter.JSON.Op == "" && filter.JSON.Path != "" {
		filter.JSON.Op = "eq"
	}

	return s.store.List(ctx, tenantID, collection, filter)
}

func (s *RecordService) BulkUpsert(ctx context.Context, tenantID, collection string, items []BulkUpsertItem, meta domain.MutationMetadata) ([]domain.Record, error) {
	result := make([]domain.Record, 0, len(items))
	for _, item := range items {
		rec, err := s.Upsert(ctx, domain.Record{
			TenantID:   tenantID,
			Collection: collection,
			ID:         item.ID,
			Data:       item.Data,
		}, meta)
		if err != nil {
			return nil, fmt.Errorf("bulk upsert %s: %w", item.ID, err)
		}
		result = append(result, rec)
	}
	return result, nil
}

func (s *RecordService) BulkDelete(ctx context.Context, tenantID, collection string, ids []string, meta domain.MutationMetadata) (int, error) {
	deletedCount := 0
	for _, id := range ids {
		deleted, err := s.Delete(ctx, tenantID, collection, id, meta)
		if err != nil {
			return deletedCount, fmt.Errorf("bulk delete %s: %w", id, err)
		}
		if deleted {
			deletedCount++
		}
	}
	return deletedCount, nil
}

type BulkUpsertItem struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}
