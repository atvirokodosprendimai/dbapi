package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type stubRecordStore struct {
	upsertFn func(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error)
	deleteFn func(ctx context.Context, tenantID, collection, id string, meta domain.MutationMetadata) (bool, error)
	getFn    func(ctx context.Context, tenantID, collection, id string) (domain.Record, error)
	listFn   func(ctx context.Context, tenantID, collection, prefix, after string, limit int) ([]domain.Record, error)
}

func (s *stubRecordStore) UpsertWithEvents(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, rec, meta)
	}
	now := time.Now().UTC()
	rec.CreatedAt = now
	rec.UpdatedAt = now
	return rec, nil
}

func (s *stubRecordStore) DeleteWithEvents(ctx context.Context, tenantID, collection, id string, meta domain.MutationMetadata) (bool, error) {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, tenantID, collection, id, meta)
	}
	return true, nil
}

func (s *stubRecordStore) Get(ctx context.Context, tenantID, collection, id string) (domain.Record, error) {
	if s.getFn != nil {
		return s.getFn(ctx, tenantID, collection, id)
	}
	return domain.Record{}, nil
}

func (s *stubRecordStore) List(ctx context.Context, tenantID, collection, prefix, after string, limit int) ([]domain.Record, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, collection, prefix, after, limit)
	}
	return nil, nil
}

func TestRecordServiceRejectsInvalidCollection(t *testing.T) {
	svc := NewRecordService(&stubRecordStore{})

	_, err := svc.Upsert(context.Background(), domain.Record{
		TenantID:   "tenant-a",
		Collection: "bad collection",
		ID:         "1",
		Data:       json.RawMessage(`{"name":"x"}`),
	}, domain.MutationMetadata{Actor: "actor"})
	if !errors.Is(err, domain.ErrInvalidCategory) {
		t.Fatalf("expected invalid category, got %v", err)
	}
}

func TestRecordServiceListCallsStore(t *testing.T) {
	called := false
	svc := NewRecordService(&stubRecordStore{listFn: func(_ context.Context, tenantID, collection, prefix, after string, limit int) ([]domain.Record, error) {
		called = true
		if tenantID != "tenant-a" || collection != "contacts" || limit != 10 {
			t.Fatalf("unexpected params: %s %s %d", tenantID, collection, limit)
		}
		return []domain.Record{{TenantID: tenantID, Collection: collection, ID: "1", Data: json.RawMessage(`{"name":"a"}`)}}, nil
	}})

	recs, err := svc.List(context.Background(), "tenant-a", "contacts", "", "", 10)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !called {
		t.Fatal("expected list call")
	}
	if len(recs) != 1 || recs[0].ID != "1" {
		t.Fatalf("unexpected records: %+v", recs)
	}
}
