package sqlite

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/adapters/sqlite/gormsqlite"
	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type apiKeyModel struct {
	TokenHash string    `gorm:"column:token_hash;primaryKey"`
	TenantID  string    `gorm:"column:tenant_id;not null"`
	Name      string    `gorm:"column:name;not null"`
	Active    bool      `gorm:"column:active;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (apiKeyModel) TableName() string {
	return "api_keys"
}

type APIKeyRepository struct {
	db *gormsqlite.DB
}

func NewAPIKeyRepository(db *gormsqlite.DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

func (r *APIKeyRepository) FindByTokenHash(ctx context.Context, tokenHash string) (domain.APIKey, error) {
	var model apiKeyModel
	err := r.db.ReadTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Where("token_hash = ?", tokenHash).First(&model).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.APIKey{}, domain.ErrNotFound
		}
		return domain.APIKey{}, fmt.Errorf("find api key: %w", err)
	}

	return domain.APIKey{
		TokenHash: model.TokenHash,
		TenantID:  model.TenantID,
		Name:      model.Name,
		Active:    model.Active,
		CreatedAt: model.CreatedAt,
	}, nil
}

func (r *APIKeyRepository) Upsert(ctx context.Context, key domain.APIKey) error {
	model := apiKeyModel{
		TokenHash: key.TokenHash,
		TenantID:  key.TenantID,
		Name:      key.Name,
		Active:    key.Active,
		CreatedAt: key.CreatedAt,
	}

	err := r.db.WriteTX(ctx, func(tx *gormsqlite.Tx) error {
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token_hash"}},
			DoUpdates: clause.AssignmentColumns([]string{"tenant_id", "name", "active"}),
		}).Create(&model).Error
	})
	if err != nil {
		return fmt.Errorf("upsert api key: %w", err)
	}
	return nil
}
