package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/usecase"
)

// stubSchemaRepo is an in-memory CollectionSchemaRepository for handler tests.
type stubSchemaRepo struct {
	schemas map[string]domain.CollectionSchema
}

func newStubSchemaRepo() *stubSchemaRepo {
	return &stubSchemaRepo{schemas: make(map[string]domain.CollectionSchema)}
}

func (r *stubSchemaRepo) Upsert(_ context.Context, schema domain.CollectionSchema) (domain.CollectionSchema, error) {
	now := time.Now().UTC()
	schema.CreatedAt = now
	schema.UpdatedAt = now
	r.schemas[schema.TenantID+"/"+schema.Collection] = schema
	return schema, nil
}

func (r *stubSchemaRepo) Get(_ context.Context, tenantID, collection string) (domain.CollectionSchema, error) {
	s, ok := r.schemas[tenantID+"/"+collection]
	if !ok {
		return domain.CollectionSchema{}, domain.ErrNotFound
	}
	return s, nil
}

func (r *stubSchemaRepo) Delete(_ context.Context, tenantID, collection string) (bool, error) {
	key := tenantID + "/" + collection
	_, ok := r.schemas[key]
	if !ok {
		return false, nil
	}
	delete(r.schemas, key)
	return true, nil
}

func testRouterWithSchema(schemaRepo *stubSchemaRepo) http.Handler {
	repo := &stubRepo{}
	kv := usecase.NewKVService(repo)
	records := usecase.NewRecordService(&stubRecordStore{})
	auth := usecase.NewAuthService(&stubAPIKeyRepo{})
	audit := usecase.NewAuditService(&stubAuditTrailRepo{})
	schemaSvc := usecase.NewSchemaService(schemaRepo)
	return NewHandler(kv, records, auth, audit, schemaSvc).Router()
}

func testRouterWithSchemaAndRecords(schemaRepo *stubSchemaRepo, recordStore *stubRecordStore) http.Handler {
	repo := &stubRepo{}
	kv := usecase.NewKVService(repo)
	schemaSvc := usecase.NewSchemaService(schemaRepo)
	records := usecase.NewRecordService(recordStore, usecase.WithSchemaService(schemaSvc))
	auth := usecase.NewAuthService(&stubAPIKeyRepo{})
	audit := usecase.NewAuditService(&stubAuditTrailRepo{})
	return NewHandler(kv, records, auth, audit, schemaSvc).Router()
}

func TestPutCollectionSchemaReturns200(t *testing.T) {
	h := testRouterWithSchema(newStubSchemaRepo())
	body := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	req := httptest.NewRequest(http.MethodPut, "/v1/collections/contacts/schema", strings.NewReader(body))
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["collection"] != "contacts" {
		t.Fatalf("unexpected collection: %v", resp["collection"])
	}
}

func TestPutCollectionSchemaRejectsInvalidJSON(t *testing.T) {
	h := testRouterWithSchema(newStubSchemaRepo())
	req := httptest.NewRequest(http.MethodPut, "/v1/collections/contacts/schema", strings.NewReader(`not json`))
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetCollectionSchemaReturns404WhenMissing(t *testing.T) {
	h := testRouterWithSchema(newStubSchemaRepo())
	req := httptest.NewRequest(http.MethodGet, "/v1/collections/contacts/schema", nil)
	withAuth(req)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetCollectionSchemaReturnsSchema(t *testing.T) {
	schemaRepo := newStubSchemaRepo()
	h := testRouterWithSchema(schemaRepo)

	// First put a schema
	body := `{"type":"object"}`
	putReq := httptest.NewRequest(http.MethodPut, "/v1/collections/orders/schema", strings.NewReader(body))
	withAuth(putReq)
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put schema expected 200, got %d", putRec.Code)
	}

	// Then get it
	getReq := httptest.NewRequest(http.MethodGet, "/v1/collections/orders/schema", nil)
	withAuth(getReq)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get schema expected 200, got %d: %s", getRec.Code, getRec.Body.String())
	}
}

func TestDeleteCollectionSchemaReturnsDeleted(t *testing.T) {
	schemaRepo := newStubSchemaRepo()
	h := testRouterWithSchema(schemaRepo)

	// Put a schema first
	body := `{"type":"object"}`
	putReq := httptest.NewRequest(http.MethodPut, "/v1/collections/items/schema", strings.NewReader(body))
	withAuth(putReq)
	putRec := httptest.NewRecorder()
	h.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put schema expected 200, got %d", putRec.Code)
	}

	// Delete it
	delReq := httptest.NewRequest(http.MethodDelete, "/v1/collections/items/schema", nil)
	withAuth(delReq)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("delete schema expected 200, got %d", delRec.Code)
	}

	var resp map[string]bool
	if err := json.Unmarshal(delRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !resp["deleted"] {
		t.Fatal("expected deleted=true")
	}
}

func TestUpsertRecordRejectsSchemaViolation(t *testing.T) {
	schemaRepo := newStubSchemaRepo()
	h := testRouterWithSchemaAndRecords(schemaRepo, &stubRecordStore{})

	// Set a schema requiring "name" field
	schemaBody := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	putReq := httptest.NewRequest(http.MethodPut, "/v1/collections/contacts/schema", strings.NewReader(schemaBody))
	withAuth(putReq)
	h.ServeHTTP(httptest.NewRecorder(), putReq)

	// Try to upsert a record missing the required field
	recReq := httptest.NewRequest(http.MethodPut, "/v1/collections/contacts/records/rec-1", strings.NewReader(`{"age":30}`))
	withAuth(recReq)
	recRec := httptest.NewRecorder()
	h.ServeHTTP(recRec, recReq)
	if recRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for schema violation, got %d: %s", recRec.Code, recRec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(recRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["error"] != "schema validation failed" {
		t.Fatalf("expected schema validation error message, got %v", resp["error"])
	}
	if _, ok := resp["errors"]; !ok {
		t.Fatal("expected 'errors' field in response")
	}
}

func TestUpsertRecordPassesWithValidData(t *testing.T) {
	schemaRepo := newStubSchemaRepo()
	h := testRouterWithSchemaAndRecords(schemaRepo, &stubRecordStore{})

	// Set a schema requiring "name" field
	schemaBody := `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`
	putReq := httptest.NewRequest(http.MethodPut, "/v1/collections/contacts/schema", strings.NewReader(schemaBody))
	withAuth(putReq)
	h.ServeHTTP(httptest.NewRecorder(), putReq)

	// Upsert a valid record
	recReq := httptest.NewRequest(http.MethodPut, "/v1/collections/contacts/records/rec-1", strings.NewReader(`{"name":"Alice"}`))
	withAuth(recReq)
	recRec := httptest.NewRecorder()
	h.ServeHTTP(recRec, recReq)
	if recRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid data, got %d: %s", recRec.Code, recRec.Body.String())
	}
}

func TestUpsertRecordNoSchemaAllowsAnyData(t *testing.T) {
	schemaRepo := newStubSchemaRepo()
	h := testRouterWithSchemaAndRecords(schemaRepo, &stubRecordStore{})

	// No schema configured - any data should be accepted
	recReq := httptest.NewRequest(http.MethodPut, "/v1/collections/events/records/ev-1", strings.NewReader(`{"anything":"goes","nested":{"a":1}}`))
	withAuth(recReq)
	recRec := httptest.NewRecorder()
	h.ServeHTTP(recRec, recReq)
	if recRec.Code != http.StatusOK {
		t.Fatalf("expected 200 when no schema, got %d: %s", recRec.Code, recRec.Body.String())
	}
}
