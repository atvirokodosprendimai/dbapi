package domain

import (
	"encoding/json"
	"time"
)

const CurrentEventSchemaVersion = 1

type MutationMetadata struct {
	Actor          string
	Source         string
	RequestID      string
	CorrelationID  string
	CausationID    string
	IdempotencyKey string
	OccurredAt     time.Time
}

func (m MutationMetadata) Normalize() MutationMetadata {
	if m.Actor == "" {
		m.Actor = "api"
	}
	if m.Source == "" {
		m.Source = "api"
	}
	if m.OccurredAt.IsZero() {
		m.OccurredAt = time.Now().UTC()
	}
	return m
}

type EventEnvelope struct {
	EventID          string          `json:"event_id"`
	EventType        string          `json:"event_type"`
	SchemaVersion    int             `json:"schema_version"`
	TenantID         string          `json:"tenant_id"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int64           `json:"aggregate_version"`
	OccurredAt       time.Time       `json:"occurred_at"`
	CorrelationID    string          `json:"correlation_id"`
	CausationID      string          `json:"causation_id"`
	Actor            string          `json:"actor"`
	Source           string          `json:"source"`
	Payload          json.RawMessage `json:"payload"`
}

type AuditTrailEvent struct {
	ID               int64           `json:"id"`
	EventID          string          `json:"event_id"`
	SchemaVersion    int             `json:"schema_version"`
	TenantID         string          `json:"tenant_id"`
	AggregateType    string          `json:"aggregate_type"`
	AggregateID      string          `json:"aggregate_id"`
	AggregateVersion int64           `json:"aggregate_version"`
	Action           string          `json:"action"`
	Actor            string          `json:"actor"`
	Source           string          `json:"source"`
	RequestID        string          `json:"request_id"`
	CorrelationID    string          `json:"correlation_id"`
	CausationID      string          `json:"causation_id"`
	IdempotencyKey   string          `json:"idempotency_key"`
	BeforeJSON       json.RawMessage `json:"before_json,omitempty"`
	AfterJSON        json.RawMessage `json:"after_json,omitempty"`
	ChangedJSON      json.RawMessage `json:"changed_fields_json,omitempty"`
	OccurredAt       time.Time       `json:"occurred_at"`
}

type OutboxEvent struct {
	ID            int64
	EventID       string
	TenantID      string
	Topic         string
	PayloadJSON   json.RawMessage
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	DispatchedAt  *time.Time
}

type AuditFilter struct {
	TenantID      string
	AggregateType string
	AggregateID   string
	Action        string
	AfterID       int64
	Limit         int
}
