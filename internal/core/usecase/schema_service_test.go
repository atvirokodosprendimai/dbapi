package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

// stubSchemaRepo is an in-memory CollectionSchemaRepository for tests.
type stubSchemaRepo struct {
	schemas map[string]domain.CollectionSchema
}

func newStubSchemaRepo() *stubSchemaRepo {
	return &stubSchemaRepo{schemas: make(map[string]domain.CollectionSchema)}
}

func (r *stubSchemaRepo) Upsert(_ context.Context, schema domain.CollectionSchema) (domain.CollectionSchema, error) {
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

// ---- SchemaService tests ----

func TestSchemaServiceUpsertAndGet(t *testing.T) {
	svc := NewSchemaService(newStubSchemaRepo())
	schemaJSON := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)

	cs, err := svc.Upsert(context.Background(), "tenant-a", "contacts", schemaJSON)
	if err != nil {
		t.Fatalf("upsert schema: %v", err)
	}
	if cs.Collection != "contacts" {
		t.Fatalf("unexpected collection: %s", cs.Collection)
	}

	got, err := svc.Get(context.Background(), "tenant-a", "contacts")
	if err != nil {
		t.Fatalf("get schema: %v", err)
	}
	if string(got.Schema) != string(schemaJSON) {
		t.Fatalf("unexpected schema: %s", got.Schema)
	}
}

func TestSchemaServiceUpsertRejectsInvalidJSON(t *testing.T) {
	svc := NewSchemaService(newStubSchemaRepo())
	_, err := svc.Upsert(context.Background(), "tenant-a", "contacts", json.RawMessage(`not json`))
	if err == nil {
		t.Fatal("expected error for invalid json schema")
	}
}

func TestSchemaServiceUpsertRejectsInvalidSchemaDocument(t *testing.T) {
	svc := NewSchemaService(newStubSchemaRepo())
	// Valid JSON but not a valid JSON Schema (type value must be a string or array)
	_, err := svc.Upsert(context.Background(), "tenant-a", "contacts", json.RawMessage(`{"type":123}`))
	if err == nil {
		t.Fatal("expected error for invalid json schema document")
	}
}

func TestSchemaServiceGetMissingReturnsNotFound(t *testing.T) {
	svc := NewSchemaService(newStubSchemaRepo())
	_, err := svc.Get(context.Background(), "tenant-a", "contacts")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestSchemaServiceDelete(t *testing.T) {
	repo := newStubSchemaRepo()
	svc := NewSchemaService(repo)
	schemaJSON := json.RawMessage(`{"type":"object"}`)

	if _, err := svc.Upsert(context.Background(), "tenant-a", "orders", schemaJSON); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	deleted, err := svc.Delete(context.Background(), "tenant-a", "orders")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if !deleted {
		t.Fatal("expected deleted=true")
	}

	_, err = svc.Get(context.Background(), "tenant-a", "orders")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected not found after delete, got %v", err)
	}
}

// ---- RecordService schema validation tests ----

func TestRecordServiceValidatesAgainstSchema(t *testing.T) {
	repo := newStubSchemaRepo()
	schemaSvc := NewSchemaService(repo)
	svc := NewRecordService(&stubRecordStore{}, WithSchemaService(schemaSvc))

	schemaJSON := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)
	if _, err := schemaSvc.Upsert(context.Background(), "tenant-a", "contacts", schemaJSON); err != nil {
		t.Fatalf("upsert schema: %v", err)
	}

	// Valid data should pass
	_, err := svc.Upsert(context.Background(), domain.Record{
		TenantID:   "tenant-a",
		Collection: "contacts",
		ID:         "1",
		Data:       json.RawMessage(`{"name":"Alice"}`),
	}, domain.MutationMetadata{Actor: "actor"})
	if err != nil {
		t.Fatalf("expected no error for valid data, got %v", err)
	}
}

func TestRecordServiceRejectsDataViolatingSchema(t *testing.T) {
	repo := newStubSchemaRepo()
	schemaSvc := NewSchemaService(repo)
	svc := NewRecordService(&stubRecordStore{}, WithSchemaService(schemaSvc))

	schemaJSON := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)
	if _, err := schemaSvc.Upsert(context.Background(), "tenant-a", "contacts", schemaJSON); err != nil {
		t.Fatalf("upsert schema: %v", err)
	}

	// Data missing required "name" field
	_, err := svc.Upsert(context.Background(), domain.Record{
		TenantID:   "tenant-a",
		Collection: "contacts",
		ID:         "2",
		Data:       json.RawMessage(`{"age":30}`),
	}, domain.MutationMetadata{Actor: "actor"})

	var schemaErr *domain.ErrSchemaViolation
	if !errors.As(err, &schemaErr) {
		t.Fatalf("expected ErrSchemaViolation, got %v", err)
	}
	if len(schemaErr.Errors) == 0 {
		t.Fatal("expected non-empty validation errors")
	}
}

func TestRecordServiceNoSchemaAllowsAllData(t *testing.T) {
	repo := newStubSchemaRepo()
	schemaSvc := NewSchemaService(repo)
	svc := NewRecordService(&stubRecordStore{}, WithSchemaService(schemaSvc))

	// No schema configured for collection "events"
	_, err := svc.Upsert(context.Background(), domain.Record{
		TenantID:   "tenant-a",
		Collection: "events",
		ID:         "1",
		Data:       json.RawMessage(`{"anything":"goes"}`),
	}, domain.MutationMetadata{Actor: "actor"})
	if err != nil {
		t.Fatalf("expected no error when no schema, got %v", err)
	}
}

func TestRecordServiceBulkUpsertValidatesAgainstSchema(t *testing.T) {
	repo := newStubSchemaRepo()
	schemaSvc := NewSchemaService(repo)
	svc := NewRecordService(&stubRecordStore{}, WithSchemaService(schemaSvc))

	schemaJSON := json.RawMessage(`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)
	if _, err := schemaSvc.Upsert(context.Background(), "tenant-a", "contacts", schemaJSON); err != nil {
		t.Fatalf("upsert schema: %v", err)
	}

	// One valid and one invalid item
	_, err := svc.BulkUpsert(context.Background(), "tenant-a", "contacts", []BulkUpsertItem{
		{ID: "1", Data: json.RawMessage(`{"name":"Bob"}`)},
		{ID: "2", Data: json.RawMessage(`{"age":25}`)}, // missing required name
	}, domain.MutationMetadata{Actor: "actor"})
	if err == nil {
		t.Fatal("expected error for bulk upsert with invalid data")
	}
	// The wrapped error should contain ErrSchemaViolation
	var schemaErr *domain.ErrSchemaViolation
	if !errors.As(err, &schemaErr) {
		t.Fatalf("expected ErrSchemaViolation in bulk upsert error chain, got %v", err)
	}
}

func TestSchemaServiceValidateNoSchema(t *testing.T) {
	svc := NewSchemaService(newStubSchemaRepo())
	err := svc.Validate(context.Background(), "tenant-a", "things", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatalf("expected nil error when no schema configured, got %v", err)
	}
}
