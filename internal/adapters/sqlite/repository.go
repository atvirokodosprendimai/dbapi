package sqlite

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/atvirokodosprendimai/dbapi/internal/core/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type entryModel struct {
	Key       string    `gorm:"column:key;primaryKey"`
	Category  string    `gorm:"column:category;not null"`
	Value     string    `gorm:"column:value;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

func (entryModel) TableName() string {
	return "kv_entries"
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Upsert(ctx context.Context, item domain.Item) (domain.Item, error) {
	now := time.Now().UTC()
	model := entryModel{
		Key:       item.Key,
		Category:  item.Category,
		Value:     string(item.Value),
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"category", "value", "updated_at"}),
		}).
		Create(&model).Error
	if err != nil {
		return domain.Item{}, fmt.Errorf("upsert entry: %w", err)
	}

	return r.Get(ctx, item.Key)
}

func (r *Repository) Get(ctx context.Context, key string) (domain.Item, error) {
	var model entryModel
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Item{}, domain.ErrNotFound
		}
		return domain.Item{}, fmt.Errorf("get entry: %w", err)
	}

	return toDomain(model), nil
}

func (r *Repository) Delete(ctx context.Context, key string) (bool, error) {
	res := r.db.WithContext(ctx).Where("key = ?", key).Delete(&entryModel{})
	if res.Error != nil {
		return false, fmt.Errorf("delete entry: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *Repository) Scan(ctx context.Context, filter domain.ScanFilter) ([]domain.Item, error) {
	query := r.db.WithContext(ctx).Model(&entryModel{})

	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if filter.Prefix != "" {
		prefixUpper := filter.Prefix + "\uffff"
		query = query.Where("key >= ? AND key < ?", filter.Prefix, prefixUpper)
	}
	if filter.AfterKey != "" {
		query = query.Where("key > ?", filter.AfterKey)
	}

	var models []entryModel
	err := query.Order("key ASC").Limit(filter.Limit).Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("scan entries: %w", err)
	}

	items := make([]domain.Item, 0, len(models))
	for _, model := range models {
		items = append(items, toDomain(model))
	}
	return items, nil
}

func toDomain(model entryModel) domain.Item {
	return domain.Item{
		Key:       model.Key,
		Category:  model.Category,
		Value:     json.RawMessage(model.Value),
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
