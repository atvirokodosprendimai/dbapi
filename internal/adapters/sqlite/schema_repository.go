package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite/gormsqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type collectionSchemaModel struct {
	TenantID   string    `gorm:"column:tenant_id;primaryKey"`
	Collection string    `gorm:"column:collection;primaryKey"`
	SchemaJSON string    `gorm:"column:schema_json;not null"`
	CreatedAt  time.Time `gorm:"column:created_at;not null"`
	UpdatedAt  time.Time `gorm:"column:updated_at;not null"`
}

func (collectionSchemaModel) TableName() string {
	return "collection_schemas"
}

type SchemaRepository struct {
	db *gormsqlite.DB
}

func NewSchemaRepository(db *gormsqlite.DB) *SchemaRepository {
	return &SchemaRepository{db: db}
}

func (r *SchemaRepository) Upsert(ctx context.Context, schema domain.CollectionSchema) (domain.CollectionSchema, error) {
	now := time.Now().UTC()
	model := collectionSchemaModel{
		TenantID:   schema.TenantID,
		Collection: schema.Collection,
		SchemaJSON: string(schema.Schema),
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	var out domain.CollectionSchema
	err := r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tenant_id"}, {Name: "collection"}},
			DoUpdates: clause.AssignmentColumns([]string{"schema_json", "updated_at"}),
		}).Create(&model).Error
		if err != nil {
			return fmt.Errorf("upsert schema: %w", err)
		}

		var saved collectionSchemaModel
		if err := tx.Where("tenant_id = ? AND collection = ?", schema.TenantID, schema.Collection).First(&saved).Error; err != nil {
			return fmt.Errorf("load upserted schema: %w", err)
		}
		out = toSchemaDomain(saved)
		return nil
	})
	if err != nil {
		return domain.CollectionSchema{}, err
	}
	return out, nil
}

func (r *SchemaRepository) Get(ctx context.Context, tenantID, collection string) (domain.CollectionSchema, error) {
	var model collectionSchemaModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("tenant_id = ? AND collection = ?", tenantID, collection).First(&model).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.CollectionSchema{}, domain.ErrNotFound
		}
		return domain.CollectionSchema{}, fmt.Errorf("get schema: %w", err)
	}
	return toSchemaDomain(model), nil
}

func (r *SchemaRepository) Delete(ctx context.Context, tenantID, collection string) (bool, error) {
	var affected int64
	err := r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		res := tx.Where("tenant_id = ? AND collection = ?", tenantID, collection).Delete(&collectionSchemaModel{})
		if res.Error != nil {
			return fmt.Errorf("delete schema: %w", res.Error)
		}
		affected = res.RowsAffected
		return nil
	})
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func toSchemaDomain(model collectionSchemaModel) domain.CollectionSchema {
	return domain.CollectionSchema{
		TenantID:   model.TenantID,
		Collection: model.Collection,
		Schema:     json.RawMessage(model.SchemaJSON),
		CreatedAt:  model.CreatedAt,
		UpdatedAt:  model.UpdatedAt,
	}
}
