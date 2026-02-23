package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

type RecordService struct {
	kv    *KVService
	audit ports.AuditRepository
}

func NewRecordService(kv *KVService, audit ports.AuditRepository) *RecordService {
	return &RecordService{kv: kv, audit: audit}
}

func (s *RecordService) Upsert(ctx context.Context, rec domain.Record, actor string) (domain.Record, error) {
	if err := rec.Validate(); err != nil {
		return domain.Record{}, err
	}

	item, err := s.kv.Upsert(ctx, domain.Item{
		Key:      buildRecordKey(rec.TenantID, rec.Collection, rec.ID),
		Category: buildCategory(rec.TenantID, rec.Collection),
		Value:    rec.Data,
	})
	if err != nil {
		return domain.Record{}, err
	}

	_ = s.audit.Log(ctx, domain.AuditEvent{
		TenantID:   rec.TenantID,
		Collection: rec.Collection,
		RecordID:   rec.ID,
		Action:     "upsert",
		Actor:      actor,
		At:         time.Now().UTC(),
	})

	return domain.Record{
		TenantID:   rec.TenantID,
		Collection: rec.Collection,
		ID:         rec.ID,
		Data:       item.Value,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}, nil
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

	item, err := s.kv.Get(ctx, buildRecordKey(tenantID, collection, id))
	if err != nil {
		return domain.Record{}, err
	}

	return domain.Record{
		TenantID:   tenantID,
		Collection: collection,
		ID:         id,
		Data:       item.Value,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	}, nil
}

func (s *RecordService) Delete(ctx context.Context, tenantID, collection, id, actor string) (bool, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return false, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return false, err
	}
	if err := domain.ValidateKey(id); err != nil {
		return false, err
	}

	deleted, err := s.kv.Delete(ctx, buildRecordKey(tenantID, collection, id))
	if err != nil {
		return false, err
	}
	if deleted {
		_ = s.audit.Log(ctx, domain.AuditEvent{
			TenantID:   tenantID,
			Collection: collection,
			RecordID:   id,
			Action:     "delete",
			Actor:      actor,
			At:         time.Now().UTC(),
		})
	}
	return deleted, nil
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

	keyPrefix := buildCollectionPrefix(tenantID, collection)
	filter := domain.ScanFilter{
		Category: buildCategory(tenantID, collection),
		Prefix:   keyPrefix + prefix,
		AfterKey: "",
		Limit:    limit,
	}
	if after != "" {
		filter.AfterKey = keyPrefix + after
	}

	items, err := s.kv.Scan(ctx, filter)
	if err != nil {
		return nil, err
	}

	records := make([]domain.Record, 0, len(items))
	for _, item := range items {
		id := strings.TrimPrefix(item.Key, keyPrefix)
		if id == item.Key {
			continue
		}
		records = append(records, domain.Record{
			TenantID:   tenantID,
			Collection: collection,
			ID:         id,
			Data:       item.Value,
			CreatedAt:  item.CreatedAt,
			UpdatedAt:  item.UpdatedAt,
		})
	}
	return records, nil
}

func (s *RecordService) BulkUpsert(ctx context.Context, tenantID, collection string, items []BulkUpsertItem, actor string) ([]domain.Record, error) {
	result := make([]domain.Record, 0, len(items))
	for _, item := range items {
		rec, err := s.Upsert(ctx, domain.Record{
			TenantID:   tenantID,
			Collection: collection,
			ID:         item.ID,
			Data:       item.Data,
		}, actor)
		if err != nil {
			return nil, fmt.Errorf("bulk upsert %s: %w", item.ID, err)
		}
		result = append(result, rec)
	}
	return result, nil
}

func (s *RecordService) BulkDelete(ctx context.Context, tenantID, collection string, ids []string, actor string) (int, error) {
	deletedCount := 0
	for _, id := range ids {
		deleted, err := s.Delete(ctx, tenantID, collection, id, actor)
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

func buildRecordKey(tenantID, collection, id string) string {
	return buildCollectionPrefix(tenantID, collection) + id
}

func buildCollectionPrefix(tenantID, collection string) string {
	return tenantID + "/" + collection + "/"
}

func buildCategory(tenantID, collection string) string {
	return tenantID + "/" + collection
}
