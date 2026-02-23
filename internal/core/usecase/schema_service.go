package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	santhosh "github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/atvirokodosprendimai/dbapi/internal/core/ports"
)

// SchemaService manages per-collection JSON schemas and validates record data.
type SchemaService struct {
	repo  ports.CollectionSchemaRepository
	cache sync.Map // key: "tenantID/collection" → *santhosh.Schema
}

func NewSchemaService(repo ports.CollectionSchemaRepository) *SchemaService {
	return &SchemaService{repo: repo}
}

func (s *SchemaService) Upsert(ctx context.Context, tenantID, collection string, schemaJSON json.RawMessage) (domain.CollectionSchema, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return domain.CollectionSchema{}, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return domain.CollectionSchema{}, err
	}
	if !json.Valid(schemaJSON) {
		return domain.CollectionSchema{}, errors.New("schema must be valid json")
	}
	if err := compilable(schemaJSON); err != nil {
		return domain.CollectionSchema{}, fmt.Errorf("invalid json schema: %w", err)
	}
	s.cache.Delete(tenantID + "/" + collection)
	return s.repo.Upsert(ctx, domain.CollectionSchema{
		TenantID:   tenantID,
		Collection: collection,
		Schema:     schemaJSON,
	})
}

func (s *SchemaService) Get(ctx context.Context, tenantID, collection string) (domain.CollectionSchema, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return domain.CollectionSchema{}, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return domain.CollectionSchema{}, err
	}
	return s.repo.Get(ctx, tenantID, collection)
}

func (s *SchemaService) Delete(ctx context.Context, tenantID, collection string) (bool, error) {
	if err := domain.ValidateKey(tenantID); err != nil {
		return false, err
	}
	if err := domain.ValidateCategory(collection); err != nil {
		return false, err
	}
	s.cache.Delete(tenantID + "/" + collection)
	return s.repo.Delete(ctx, tenantID, collection)
}

// Validate checks data against the collection schema. If no schema is configured
// the data passes validation. Returns *domain.ErrSchemaViolation on failure.
func (s *SchemaService) Validate(ctx context.Context, tenantID, collection string, data json.RawMessage) error {
	cacheKey := tenantID + "/" + collection

	if cached, ok := s.cache.Load(cacheKey); ok {
		return runValidation(cached.(*santhosh.Schema), data)
	}

	cs, err := s.repo.Get(ctx, tenantID, collection)
	if errors.Is(err, domain.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load schema: %w", err)
	}

	compiled, err := compileSchema(cs.Schema)
	if err != nil {
		return fmt.Errorf("compile schema: %w", err)
	}
	s.cache.Store(cacheKey, compiled)
	return runValidation(compiled, data)
}

// compileSchema builds a *santhosh.Schema from raw JSON.
func compileSchema(schemaJSON json.RawMessage) (*santhosh.Schema, error) {
	compiler := santhosh.NewCompiler()
	compiler.Draft = santhosh.Draft7
	if err := compiler.AddResource("schema.json", bytes.NewReader(schemaJSON)); err != nil {
		return nil, err
	}
	return compiler.Compile("schema.json")
}

// runValidation validates data against a pre-compiled schema.
func runValidation(sch *santhosh.Schema, data json.RawMessage) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}
	if err := sch.Validate(v); err != nil {
		var ve *santhosh.ValidationError
		if errors.As(err, &ve) {
			msgs := collectValidationErrors(ve)
			return &domain.ErrSchemaViolation{Errors: msgs}
		}
		return &domain.ErrSchemaViolation{Errors: []string{err.Error()}}
	}
	return nil
}

func collectValidationErrors(ve *santhosh.ValidationError) []string {
	var msgs []string
	for _, cause := range ve.Causes {
		msgs = append(msgs, collectValidationErrors(cause)...)
	}
	if len(ve.Causes) == 0 {
		msgs = append(msgs, ve.Error())
	}
	return msgs
}

// compilable returns an error if schemaJSON is not a valid JSON Schema document.
func compilable(schemaJSON json.RawMessage) error {
	_, err := compileSchema(schemaJSON)
	return err
}
