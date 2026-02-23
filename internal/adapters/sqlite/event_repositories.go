package sqlite

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite/gormsqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
)

type AuditTrailRepository struct {
	db *gormsqlite.DB
}

func NewAuditTrailRepository(db *gormsqlite.DB) *AuditTrailRepository {
	return &AuditTrailRepository{db: db}
}

func (r *AuditTrailRepository) List(ctx context.Context, filter domain.AuditFilter) ([]domain.AuditTrailEvent, error) {
	var rows []auditEventModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		query := tx.Model(&auditEventModel{}).Where("tenant_id = ?", filter.TenantID)
		if filter.AggregateType != "" {
			query = query.Where("aggregate_type = ?", filter.AggregateType)
		}
		if filter.AggregateID != "" {
			query = query.Where("aggregate_id = ?", filter.AggregateID)
		}
		if filter.Action != "" {
			query = query.Where("action = ?", filter.Action)
		}
		if filter.AfterID > 0 {
			query = query.Where("id < ?", filter.AfterID)
		}
		return query.Order("id DESC").Limit(filter.Limit).Find(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("list audit events: %w", err)
	}

	result := make([]domain.AuditTrailEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.AuditTrailEvent{
			ID:               row.ID,
			EventID:          row.EventID,
			SchemaVersion:    row.SchemaVersion,
			TenantID:         row.TenantID,
			AggregateType:    row.AggregateType,
			AggregateID:      row.AggregateID,
			AggregateVersion: row.AggregateVersion,
			Action:           row.Action,
			Actor:            row.Actor,
			Source:           row.Source,
			RequestID:        row.RequestID,
			CorrelationID:    row.CorrelationID,
			CausationID:      row.CausationID,
			IdempotencyKey:   row.IdempotencyKey,
			BeforeJSON:       json.RawMessage(row.BeforeJSON),
			AfterJSON:        json.RawMessage(row.AfterJSON),
			ChangedJSON:      json.RawMessage(row.ChangedFieldsJSON),
			OccurredAt:       row.OccurredAt,
		})
	}

	return result, nil
}

type OutboxRepository struct {
	db *gormsqlite.DB
}

func NewOutboxRepository(db *gormsqlite.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]domain.OutboxEvent, error) {
	if limit <= 0 {
		limit = 50
	}
	var rows []outboxEventModel
	now := time.Now().UTC()
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("status = ? AND next_attempt_at <= ?", "pending", now).
			Order("id ASC").
			Limit(limit).
			Find(&rows).Error
	})
	if err != nil {
		return nil, fmt.Errorf("fetch pending outbox: %w", err)
	}

	result := make([]domain.OutboxEvent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.OutboxEvent{
			ID:            row.ID,
			EventID:       row.EventID,
			TenantID:      row.TenantID,
			Topic:         row.Topic,
			PayloadJSON:   json.RawMessage(row.PayloadJSON),
			Status:        row.Status,
			Attempts:      row.Attempts,
			NextAttemptAt: row.NextAttemptAt,
			LastError:     row.LastError,
			CreatedAt:     row.CreatedAt,
			DispatchedAt:  row.DispatchedAt,
		})
	}
	return result, nil
}

func (r *OutboxRepository) MarkDispatched(ctx context.Context, id int64) error {
	now := time.Now().UTC()
	err := r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Model(&outboxEventModel{}).
			Where("id = ?", id).
			Updates(map[string]any{"status": "dispatched", "dispatched_at": &now, "last_error": ""}).Error
	})
	if err != nil {
		return fmt.Errorf("mark outbox dispatched: %w", err)
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id int64, attempts int, nextAttemptAt string, errMsg string) error {
	parsed, err := time.Parse(time.RFC3339Nano, nextAttemptAt)
	if err != nil {
		return fmt.Errorf("parse next attempt: %w", err)
	}
	err = r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Model(&outboxEventModel{}).
			Where("id = ?", id).
			Updates(map[string]any{"attempts": attempts, "next_attempt_at": parsed, "last_error": errMsg}).Error
	})
	if err != nil {
		return fmt.Errorf("mark outbox failed: %w", err)
	}
	return nil
}
