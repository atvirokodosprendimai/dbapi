package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type stubRepo struct {
	upsertFn func(ctx context.Context, item domain.Item) (domain.Item, error)
	getFn    func(ctx context.Context, key string) (domain.Item, error)
	deleteFn func(ctx context.Context, key string) (bool, error)
	scanFn   func(ctx context.Context, filter domain.ScanFilter) ([]domain.Item, error)
}

func (s *stubRepo) Upsert(ctx context.Context, item domain.Item) (domain.Item, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, item)
	}
	return item, nil
}

func (s *stubRepo) Get(ctx context.Context, key string) (domain.Item, error) {
	if s.getFn != nil {
		return s.getFn(ctx, key)
	}
	return domain.Item{}, nil
}

func (s *stubRepo) Delete(ctx context.Context, key string) (bool, error) {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, key)
	}
	return true, nil
}

func (s *stubRepo) Scan(ctx context.Context, filter domain.ScanFilter) ([]domain.Item, error) {
	if s.scanFn != nil {
		return s.scanFn(ctx, filter)
	}
	return nil, nil
}

func TestKVServiceUpsertValidation(t *testing.T) {
	svc := NewKVService(&stubRepo{})

	_, err := svc.Upsert(context.Background(), domain.Item{
		Key:      "",
		Category: "users",
		Value:    json.RawMessage(`{"name":"alice"}`),
	})
	if !errors.Is(err, domain.ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
}

func TestKVServiceScanLimitClamp(t *testing.T) {
	called := false
	svc := NewKVService(&stubRepo{
		scanFn: func(_ context.Context, filter domain.ScanFilter) ([]domain.Item, error) {
			called = true
			if filter.Limit != 1000 {
				t.Fatalf("expected clamped limit 1000, got %d", filter.Limit)
			}
			return nil, nil
		},
	})

	if _, err := svc.Scan(context.Background(), domain.ScanFilter{Limit: 5000}); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if !called {
		t.Fatal("expected scan to be called")
	}
}

func TestKVServiceGetInvalidKey(t *testing.T) {
	svc := NewKVService(&stubRepo{})

	_, err := svc.Get(context.Background(), "bad key with spaces")
	if !errors.Is(err, domain.ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
}

func TestKVServiceDeleteInvalidKey(t *testing.T) {
	svc := NewKVService(&stubRepo{})

	_, err := svc.Delete(context.Background(), "bad key with spaces")
	if !errors.Is(err, domain.ErrInvalidKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
}

func TestKVServiceScanInvalidCategory(t *testing.T) {
	svc := NewKVService(&stubRepo{})

	_, err := svc.Scan(context.Background(), domain.ScanFilter{Category: "bad category", Limit: 10})
	if !errors.Is(err, domain.ErrInvalidCategory) {
		t.Fatalf("expected invalid category, got %v", err)
	}
}
