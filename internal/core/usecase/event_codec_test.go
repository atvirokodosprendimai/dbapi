package usecase

import (
	"encoding/json"
	"testing"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type testUpcaster struct{}

func (t testUpcaster) FromVersion() int { return 0 }
func (t testUpcaster) ToVersion() int   { return 1 }
func (t testUpcaster) Upcast(payload json.RawMessage) (json.RawMessage, error) {
	var m map[string]any
	_ = json.Unmarshal(payload, &m)
	m["upcasted"] = true
	return mustMarshal(m), nil
}

func TestEventCodecNormalize(t *testing.T) {
	codec := NewEventCodec(testUpcaster{})
	env := domain.EventEnvelope{SchemaVersion: 0, Payload: json.RawMessage(`{"a":1}`)}
	norm, err := codec.Normalize(env)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if norm.SchemaVersion != domain.CurrentEventSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", domain.CurrentEventSchemaVersion, norm.SchemaVersion)
	}
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
