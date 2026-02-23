package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/usecase"
)

const testAPIKey = "test-api-key"

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
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
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
	return false, nil
}

func (s *stubRepo) Scan(ctx context.Context, filter domain.ScanFilter) ([]domain.Item, error) {
	if s.scanFn != nil {
		return s.scanFn(ctx, filter)
	}
	return nil, nil
}

type stubRecordStore struct {
	getFn    func(ctx context.Context, tenantID, collection, id string) (domain.Record, error)
	listFn   func(ctx context.Context, tenantID, collection string, filter domain.RecordListFilter) ([]domain.Record, error)
	upsertFn func(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error)
	deleteFn func(ctx context.Context, tenantID, collection, id string, meta domain.MutationMetadata) (bool, error)
}

func (s *stubRecordStore) UpsertWithEvents(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, rec, meta)
	}
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

func (s *stubRecordStore) List(ctx context.Context, tenantID, collection string, filter domain.RecordListFilter) ([]domain.Record, error) {
	if s.listFn != nil {
		return s.listFn(ctx, tenantID, collection, filter)
	}
	return nil, nil
}

type stubAPIKeyRepo struct{}

func (s *stubAPIKeyRepo) FindByTokenHash(context.Context, string) (domain.APIKey, error) {
	return domain.APIKey{TokenHash: usecase.HashToken(testAPIKey), TenantID: "tenant-a", Name: "test-client", Active: true, CreatedAt: time.Now().UTC()}, nil
}
func (s *stubAPIKeyRepo) Upsert(context.Context, domain.APIKey) error { return nil }

type stubAuditTrailRepo struct{}

func (s *stubAuditTrailRepo) List(context.Context, domain.AuditFilter) ([]domain.AuditTrailEvent, error) {
	return nil, nil
}

func testRouter(repo *stubRepo) http.Handler {
	return testRouterWithOptions(repo)
}

func testRouterWithOptions(repo *stubRepo, opts ...Option) http.Handler {
	kv := usecase.NewKVService(repo)
	records := usecase.NewRecordService(&stubRecordStore{})
	auth := usecase.NewAuthService(&stubAPIKeyRepo{})
	audit := usecase.NewAuditService(&stubAuditTrailRepo{})
	return NewHandler(kv, records, auth, audit, nil, opts...).Router()
}

func withAuth(req *http.Request) { req.Header.Set("X-API-Key", testAPIKey) }

func TestProtectedRouteWithoutAuth(t *testing.T) {
	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodGet, "/v1/kv", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUpsertRejectsUnknownFields(t *testing.T) {
	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodPut, "/v1/kv/user:1", strings.NewReader(`{"category":"users","value":{"name":"a"},"extra":1}`))
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpsertRejectsTrailingJSON(t *testing.T) {
	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodPut, "/v1/kv/user:1", strings.NewReader(`{"category":"users","value":{"name":"a"}} {}`))
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestScanBadLimitReturnsBadRequest(t *testing.T) {
	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodGet, "/v1/kv?limit=bad", nil)
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListRecordsInvalidJSONPathFilter(t *testing.T) {
	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodGet, "/v1/collections/contacts/records?json_path=bad%20path&json_op=eq&json_value=x", nil)
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetNotFoundReturns404(t *testing.T) {
	h := testRouter(&stubRepo{getFn: func(context.Context, string) (domain.Item, error) { return domain.Item{}, domain.ErrNotFound }})
	req := httptest.NewRequest(http.MethodGet, "/v1/kv/user:404", nil)
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestWriteJSONEncodeErrorHandled(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusOK, map[string]any{"bad": func() {}})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestHandleDomainErrorInvalidKey(t *testing.T) {
	rec := httptest.NewRecorder()
	handleDomainError(rec, domain.ErrInvalidKey)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] == "" {
		t.Fatal("expected error message")
	}
}

func TestHandleDomainErrorUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	handleDomainError(rec, errors.New("boom"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestOpenAPIEndpoint(t *testing.T) {
	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodGet, "/openapi.json", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadinessEndpoint(t *testing.T) {
	h := testRouterWithOptions(&stubRepo{}, WithReadinessCheck(func(context.Context) error { return nil }))
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestReadinessEndpointFailure(t *testing.T) {
	h := testRouterWithOptions(&stubRepo{}, WithReadinessCheck(func(context.Context) error { return errors.New("db down") }))
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestMetricsEndpointIncludesRequestAndWriteCounters(t *testing.T) {
	h := testRouter(&stubRepo{})

	writeReq := httptest.NewRequest(http.MethodPut, "/v1/kv/user:1", strings.NewReader(`{"category":"users","value":{"name":"a"}}`))
	withAuth(writeReq)
	writeRec := httptest.NewRecorder()
	h.ServeHTTP(writeRec, writeReq)
	if writeRec.Code != http.StatusOK {
		t.Fatalf("expected write 200, got %d", writeRec.Code)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metricsz", nil)
	metricsRec := httptest.NewRecorder()
	h.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("expected metrics 200, got %d", metricsRec.Code)
	}

	var body map[string]int64
	if err := json.Unmarshal(metricsRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if body["http_requests_total"] < 1 {
		t.Fatalf("expected http_requests_total >= 1, got %d", body["http_requests_total"])
	}
	if body["record_write_total"] < 1 {
		t.Fatalf("expected record_write_total >= 1, got %d", body["record_write_total"])
	}
}

func TestRequestLogContainsContractFieldsAndRedactsSecrets(t *testing.T) {
	var logs bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(orig) })

	h := testRouter(&stubRepo{})
	req := httptest.NewRequest(http.MethodGet, "/v1/kv", nil)
	req.Header.Set("X-API-Key", "super-secret-token")
	req.Header.Set("X-Request-Id", "req-123")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	line := logs.String()
	for _, want := range []string{"method=GET", "route=/v1/kv", "status=200", "duration_ms=", "request_id=req-123", "tenant_id=tenant-a"} {
		if !strings.Contains(line, want) {
			t.Fatalf("missing log field %q in %q", want, line)
		}
	}
	if strings.Contains(line, "super-secret-token") {
		t.Fatalf("log leaked API key: %q", line)
	}
}

func TestIdempotencyKeyIsScopedByTenantCollectionAndOperation(t *testing.T) {
	a := idempotencyKey("tenant-a", "users", "bulk-upsert", "k1")
	b := idempotencyKey("tenant-b", "users", "bulk-upsert", "k1")
	c := idempotencyKey("tenant-a", "users", "bulk-delete", "k1")
	d := idempotencyKey("tenant-a", "orders", "bulk-upsert", "k1")

	if a == b {
		t.Fatalf("expected tenant scope separation")
	}
	if a == c {
		t.Fatalf("expected operation scope separation")
	}
	if a == d {
		t.Fatalf("expected collection scope separation")
	}
}
