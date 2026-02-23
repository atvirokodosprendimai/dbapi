package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type stubAPIKeyRepo struct {
	findFn func(ctx context.Context, tokenHash string) (domain.APIKey, error)
}

func (s *stubAPIKeyRepo) FindByTokenHash(ctx context.Context, tokenHash string) (domain.APIKey, error) {
	if s.findFn != nil {
		return s.findFn(ctx, tokenHash)
	}
	return domain.APIKey{}, domain.ErrNotFound
}

func (s *stubAPIKeyRepo) Upsert(context.Context, domain.APIKey) error { return nil }

func TestAuthServiceAuthenticateSuccess(t *testing.T) {
	repo := &stubAPIKeyRepo{findFn: func(_ context.Context, tokenHash string) (domain.APIKey, error) {
		if tokenHash != HashToken("token-1") {
			t.Fatalf("unexpected token hash: %s", tokenHash)
		}
		return domain.APIKey{TenantID: "tenant-a", Active: true, CreatedAt: time.Now()}, nil
	}}

	svc := NewAuthService(repo)
	key, err := svc.Authenticate(context.Background(), "token-1")
	if err != nil {
		t.Fatalf("authenticate failed: %v", err)
	}
	if key.TenantID != "tenant-a" {
		t.Fatalf("expected tenant-a, got %s", key.TenantID)
	}
}

func TestAuthServiceAuthenticateUnauthorized(t *testing.T) {
	svc := NewAuthService(&stubAPIKeyRepo{})
	_, err := svc.Authenticate(context.Background(), "")
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got %v", err)
	}
}
