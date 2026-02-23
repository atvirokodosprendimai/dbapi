package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"gorm.io/gorm"
)

type auditModel struct {
	ID         uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID   string    `gorm:"column:tenant_id;not null"`
	Collection string    `gorm:"column:collection;not null"`
	RecordID   string    `gorm:"column:record_id;not null"`
	Action     string    `gorm:"column:action;not null"`
	Actor      string    `gorm:"column:actor;not null"`
	At         time.Time `gorm:"column:at;not null"`
}

func (auditModel) TableName() string {
	return "audit_logs"
}

type AuditRepository struct {
	db *gorm.DB
}

func NewAuditRepository(db *gorm.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

func (r *AuditRepository) Log(ctx context.Context, event domain.AuditEvent) error {
	model := auditModel{
		TenantID:   event.TenantID,
		Collection: event.Collection,
		RecordID:   event.RecordID,
		Action:     event.Action,
		Actor:      event.Actor,
		At:         event.At,
	}
	if model.At.IsZero() {
		model.At = time.Now().UTC()
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}
