package usecase

import (
	"encoding/json"
	"fmt"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type Upcaster interface {
	FromVersion() int
	ToVersion() int
	Upcast(payload json.RawMessage) (json.RawMessage, error)
}

type EventCodec struct {
	upcasters map[int]Upcaster
}

func NewEventCodec(upcasters ...Upcaster) *EventCodec {
	m := make(map[int]Upcaster, len(upcasters))
	for _, up := range upcasters {
		m[up.FromVersion()] = up
	}
	return &EventCodec{upcasters: m}
}

func (c *EventCodec) Normalize(envelope domain.EventEnvelope) (domain.EventEnvelope, error) {
	v := envelope.SchemaVersion
	payload := envelope.Payload
	for v < domain.CurrentEventSchemaVersion {
		up, ok := c.upcasters[v]
		if !ok {
			return domain.EventEnvelope{}, fmt.Errorf("missing upcaster from version %d", v)
		}
		next, err := up.Upcast(payload)
		if err != nil {
			return domain.EventEnvelope{}, fmt.Errorf("upcast %d->%d: %w", up.FromVersion(), up.ToVersion(), err)
		}
		payload = next
		v = up.ToVersion()
	}

	envelope.SchemaVersion = v
	envelope.Payload = payload
	return envelope, nil
}
