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

func (s *RecordService) List(ctx context.Context, tenantID, collection, prefix, after string, limit int) ([]domain.Record, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return nil, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return nil, err
	}
	if prefix != "" {
		if err := domain.ValidateKey(prefix); err != nil {
			return nil, err
		}
	}
	if after != "" {
		if err := domain.ValidateKey(after); err != nil {
			return nil, err
		}
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	return s.store.List(ctx, tenantID, collection, prefix, after, limit)
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
