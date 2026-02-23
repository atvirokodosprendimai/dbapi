package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite/gormsqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type auditEventModel struct {
	ID                int64     `gorm:"column:id;primaryKey;autoIncrement"`
	EventID           string    `gorm:"column:event_id;not null"`
	SchemaVersion     int       `gorm:"column:schema_version;not null"`
	TenantID          string    `gorm:"column:tenant_id;not null"`
	AggregateType     string    `gorm:"column:aggregate_type;not null"`
	AggregateID       string    `gorm:"column:aggregate_id;not null"`
	AggregateVersion  int64     `gorm:"column:aggregate_version;not null"`
	Action            string    `gorm:"column:action;not null"`
	Actor             string    `gorm:"column:actor;not null"`
	Source            string    `gorm:"column:source;not null"`
	RequestID         string    `gorm:"column:request_id;not null"`
	CorrelationID     string    `gorm:"column:correlation_id;not null"`
	CausationID       string    `gorm:"column:causation_id;not null"`
	IdempotencyKey    string    `gorm:"column:idempotency_key;not null"`
	BeforeJSON        string    `gorm:"column:before_json"`
	AfterJSON         string    `gorm:"column:after_json"`
	ChangedFieldsJSON string    `gorm:"column:changed_fields_json"`
	OccurredAt        time.Time `gorm:"column:occurred_at;not null"`
}

func (auditEventModel) TableName() string {
	return "audit_events"
}

type outboxEventModel struct {
	ID            int64      `gorm:"column:id;primaryKey;autoIncrement"`
	EventID       string     `gorm:"column:event_id;not null"`
	TenantID      string     `gorm:"column:tenant_id;not null"`
	Topic         string     `gorm:"column:topic;not null"`
	PayloadJSON   string     `gorm:"column:payload_json;not null"`
	Status        string     `gorm:"column:status;not null"`
	Attempts      int        `gorm:"column:attempts;not null"`
	NextAttemptAt time.Time  `gorm:"column:next_attempt_at;not null"`
	LastError     string     `gorm:"column:last_error;not null"`
	CreatedAt     time.Time  `gorm:"column:created_at;not null"`
	DispatchedAt  *time.Time `gorm:"column:dispatched_at"`
}

func (outboxEventModel) TableName() string {
	return "outbox_events"
}

type RecordEventStore struct {
	db *gormsqlite.DB
}

func NewRecordEventStore(db *gormsqlite.DB) *RecordEventStore {
	return &RecordEventStore{db: db}
}

func (s *RecordEventStore) UpsertWithEvents(ctx context.Context, rec domain.Record, meta domain.MutationMetadata) (domain.Record, error) {
	meta = meta.Normalize()
	var result domain.Record

	err := s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		key := recordKey(rec.TenantID, rec.Collection, rec.ID)
		category := recordCategory(rec.TenantID, rec.Collection)

		var before *entryModel
		var existing entryModel
		err := tx.Where("key = ?", key).First(&existing).Error
		switch {
		case err == nil:
			before = &existing
		case errors.Is(err, gorm.ErrRecordNotFound):
			before = nil
		default:
			return fmt.Errorf("load existing record: %w", err)
		}

		now := meta.OccurredAt.UTC()
		model := entryModel{
			Key:       key,
			Category:  category,
			Value:     string(rec.Data),
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"category", "value", "updated_at"}),
		}).Create(&model).Error; err != nil {
			return fmt.Errorf("upsert record: %w", err)
		}

		var after entryModel
		if err := tx.Where("key = ?", key).First(&after).Error; err != nil {
			return fmt.Errorf("load updated record: %w", err)
		}

		action := "record.updated"
		if before == nil {
			action = "record.created"
		}

		aggregateVersion, err := nextAggregateVersion(tx.DB, rec.TenantID, rec.Collection, rec.ID)
		if err != nil {
			return err
		}

		eventID := uuid.NewString()
		envelope := domain.EventEnvelope{
			EventID:          eventID,
			EventType:        action,
			SchemaVersion:    domain.CurrentEventSchemaVersion,
			TenantID:         rec.TenantID,
			AggregateType:    rec.Collection,
			AggregateID:      rec.ID,
			AggregateVersion: aggregateVersion,
			OccurredAt:       now,
			CorrelationID:    meta.CorrelationID,
			CausationID:      meta.CausationID,
			Actor:            meta.Actor,
			Source:           meta.Source,
			Payload: mustJSON(map[string]any{
				"record_id":  rec.ID,
				"collection": rec.Collection,
				"data":       json.RawMessage(after.Value),
			}),
		}

		if err := insertAuditAndOutbox(tx.DB, rec, meta, before, &after, envelope); err != nil {
			return err
		}

		result = domain.Record{
			TenantID:   rec.TenantID,
			Collection: rec.Collection,
			ID:         rec.ID,
			Data:       json.RawMessage(after.Value),
			CreatedAt:  after.CreatedAt,
			UpdatedAt:  after.UpdatedAt,
		}
		return nil
	})
	if err != nil {
		return domain.Record{}, err
	}

	return result, nil
}

func (s *RecordEventStore) DeleteWithEvents(ctx context.Context, tenantID, collection, id string, meta domain.MutationMetadata) (bool, error) {
	meta = meta.Normalize()
	deleted := false

	err := s.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		key := recordKey(tenantID, collection, id)

		var before entryModel
		if err := tx.Where("key = ?", key).First(&before).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				deleted = false
				return nil
			}
			return fmt.Errorf("load record before delete: %w", err)
		}

		if err := tx.Where("key = ?", key).Delete(&entryModel{}).Error; err != nil {
			return fmt.Errorf("delete record: %w", err)
		}
		deleted = true

		aggregateVersion, err := nextAggregateVersion(tx.DB, tenantID, collection, id)
		if err != nil {
			return err
		}

		eventID := uuid.NewString()
		envelope := domain.EventEnvelope{
			EventID:          eventID,
			EventType:        "record.deleted",
			SchemaVersion:    domain.CurrentEventSchemaVersion,
			TenantID:         tenantID,
			AggregateType:    collection,
			AggregateID:      id,
			AggregateVersion: aggregateVersion,
			OccurredAt:       meta.OccurredAt.UTC(),
			CorrelationID:    meta.CorrelationID,
			CausationID:      meta.CausationID,
			Actor:            meta.Actor,
			Source:           meta.Source,
			Payload: mustJSON(map[string]any{
				"record_id":  id,
				"collection": collection,
			}),
		}

		rec := domain.Record{TenantID: tenantID, Collection: collection, ID: id}
		if err := insertAuditAndOutbox(tx.DB, rec, meta, &before, nil, envelope); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	return deleted, nil
}

func (s *RecordEventStore) Get(ctx context.Context, tenantID, collection, id string) (domain.Record, error) {
	key := recordKey(tenantID, collection, id)
	var model entryModel
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("key = ?", key).First(&model).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Record{}, domain.ErrNotFound
		}
		return domain.Record{}, fmt.Errorf("get record: %w", err)
	}

	return domain.Record{
		TenantID:   tenantID,
		Collection: collection,
		ID:         id,
		Data:       json.RawMessage(model.Value),
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}, nil
}

func (s *RecordEventStore) List(ctx context.Context, tenantID, collection string, filter domain.RecordListFilter) ([]domain.Record, error) {
	keyPrefix := recordPrefix(tenantID, collection)
	var models []entryModel
	err := s.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Model(&entryModel{}).Where("category = ?", recordCategory(tenantID, collection))
		if filter.Prefix != "" {
			start := keyPrefix + filter.Prefix
			query = query.Where("key >= ? AND key < ?", start, start+"\uffff")
		} else {
			query = query.Where("key >= ? AND key < ?", keyPrefix, keyPrefix+"\uffff")
		}
		if filter.After != "" {
			query = query.Where("key > ?", keyPrefix+filter.After)
		}

		if filter.JSON.Path != "" {
			jsonPath := dotPathToSQLiteJSONPath(filter.JSON.Path)
			switch filter.JSON.Op {
			case "eq":
				query = query.Where("CAST(json_extract(value, ?) AS TEXT) = ?", jsonPath, filter.JSON.Value)
			case "ne":
				query = query.Where("CAST(json_extract(value, ?) AS TEXT) <> ?", jsonPath, filter.JSON.Value)
			case "contains":
				query = query.Where("instr(lower(CAST(json_extract(value, ?) AS TEXT)), lower(?)) > 0", jsonPath, filter.JSON.Value)
			case "exists":
				query = query.Where("json_type(value, ?) IS NOT NULL", jsonPath)
			default:
				return domain.ErrInvalidFilter
			}
		}
		return query.Order("key ASC").Limit(filter.Limit).Find(&models).Error
	})
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}

	result := make([]domain.Record, 0, len(models))
	for _, model := range models {
		id := model.Key[len(keyPrefix):]
		result = append(result, domain.Record{
			TenantID:   tenantID,
			Collection: collection,
			ID:         id,
			Data:       json.RawMessage(model.Value),
			CreatedAt:  model.CreatedAt,
			UpdatedAt:  model.UpdatedAt,
		})
	}
	return result, nil
}

func dotPathToSQLiteJSONPath(path string) string {
	segments := domain.SplitJSONPath(path)
	if len(segments) == 0 {
		return "$"
	}
	jsonPath := "$"
	for _, seg := range segments {
		jsonPath += "." + seg
	}
	return jsonPath
}

func nextAggregateVersion(tx *gorm.DB, tenantID, collection, id string) (int64, error) {
	var maxVersion int64
	err := tx.Model(&auditEventModel{}).
		Where("tenant_id = ? AND aggregate_type = ? AND aggregate_id = ?", tenantID, collection, id).
		Select("COALESCE(MAX(aggregate_version), 0)").
		Scan(&maxVersion).Error
	if err != nil {
		return 0, fmt.Errorf("query aggregate version: %w", err)
	}
	return maxVersion + 1, nil
}

func insertAuditAndOutbox(tx *gorm.DB, rec domain.Record, meta domain.MutationMetadata, before *entryModel, after *entryModel, envelope domain.EventEnvelope) error {
	var beforeJSON string
	var afterJSON string
	if before != nil {
		beforeJSON = before.Value
	}
	if after != nil {
		afterJSON = after.Value
	}
	changed := mustJSON(map[string]any{"changed": beforeJSON != afterJSON})

	audit := auditEventModel{
		EventID:           envelope.EventID,
		SchemaVersion:     envelope.SchemaVersion,
		TenantID:          rec.TenantID,
		AggregateType:     rec.Collection,
		AggregateID:       rec.ID,
		AggregateVersion:  envelope.AggregateVersion,
		Action:            envelope.EventType,
		Actor:             meta.Actor,
		Source:            meta.Source,
		RequestID:         meta.RequestID,
		CorrelationID:     meta.CorrelationID,
		CausationID:       meta.CausationID,
		IdempotencyKey:    meta.IdempotencyKey,
		BeforeJSON:        beforeJSON,
		AfterJSON:         afterJSON,
		ChangedFieldsJSON: string(changed),
		OccurredAt:        envelope.OccurredAt,
	}
	if err := tx.Create(&audit).Error; err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}

	payload, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}

	outbox := outboxEventModel{
		EventID:       envelope.EventID,
		TenantID:      rec.TenantID,
		Topic:         "events." + rec.TenantID + "." + envelope.EventType,
		PayloadJSON:   string(payload),
		Status:        "pending",
		Attempts:      0,
		NextAttemptAt: envelope.OccurredAt,
		LastError:     "",
		CreatedAt:     envelope.OccurredAt,
	}
	if err := tx.Create(&outbox).Error; err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	return nil
}

func recordKey(tenantID, collection, id string) string {
	return tenantID + "/" + collection + "/" + id
}

func recordPrefix(tenantID, collection string) string {
	return tenantID + "/" + collection + "/"
}

func recordCategory(tenantID, collection string) string {
	return tenantID + "/" + collection
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
