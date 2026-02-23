package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ErrSchemaViolation is returned when a record's data does not conform to the
// collection's JSON schema. The Errors field contains machine-readable details.
type ErrSchemaViolation struct {
	Errors []string
}

func (e *ErrSchemaViolation) Error() string {
	return fmt.Sprintf("schema validation failed: %s", strings.Join(e.Errors, "; "))
}

// CollectionSchema holds the JSON Schema document configured for a collection.
type CollectionSchema struct {
	TenantID   string
	Collection string
	Schema     json.RawMessage
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
