package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type stubAudit struct{}

func (s *stubAudit) Log(context.Context, domain.AuditEvent) error { return nil }

func TestRecordServiceRejectsInvalidCollection(t *testing.T) {
	kv := NewKVService(&stubRepo{})
	svc := NewRecordService(kv, &stubAudit{})

	_, err := svc.Upsert(context.Background(), domain.Record{
		TenantID:   "tenant-a",
		Collection: "bad collection",
		ID:         "1",
		Data:       json.RawMessage(`{"name":"x"}`),
	}, "actor")
	if !errors.Is(err, domain.ErrInvalidCategory) {
		t.Fatalf("expected invalid category, got %v", err)
	}
}

func TestRecordServiceListUsesTenantScopedCategory(t *testing.T) {
	called := false
	repo := &stubRepo{
		scanFn: func(_ context.Context, filter domain.ScanFilter) ([]domain.Item, error) {
			called = true
			if filter.Category != "tenant-a/contacts" {
				t.Fatalf("unexpected category: %s", filter.Category)
			}
			return []domain.Item{
				{
					Key:      "tenant-a/contacts/1",
					Category: "tenant-a/contacts",
					Value:    json.RawMessage(`{"name":"a"}`),
				},
			}, nil
		},
	}
	kv := NewKVService(repo)
	svc := NewRecordService(kv, &stubAudit{})

	recs, err := svc.List(context.Background(), "tenant-a", "contacts", "", "", 10)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if !called {
		t.Fatal("expected scan call")
	}
	if len(recs) != 1 || recs[0].ID != "1" {
		t.Fatalf("unexpected records: %+v", recs)
	}
}
