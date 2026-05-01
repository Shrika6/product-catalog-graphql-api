package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/pkg/metrics"
	"gorm.io/gorm"
)

type categoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) CategoryRepository {
	return &categoryRepository{db: db}
}

func (r *categoryRepository) List(ctx context.Context) ([]*model.Category, error) {
	var categories []*model.Category
	start := time.Now()
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&categories).Error; err != nil {
		metrics.DBQueryFinished("category_repository", "List", time.Since(start))
		return nil, err
	}
	metrics.DBQueryFinished("category_repository", "List", time.Since(start))
	return categories, nil
}

func (r *categoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	var category model.Category
	start := time.Now()
	if err := r.db.WithContext(ctx).First(&category, "id = ?", id).Error; err != nil {
		metrics.DBQueryFinished("category_repository", "GetByID", time.Since(start))
		return nil, err
	}
	metrics.DBQueryFinished("category_repository", "GetByID", time.Since(start))
	return &category, nil
}

func (r *categoryRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*model.Category, error) {
	var categories []*model.Category
	if len(ids) == 0 {
		return categories, nil
	}
	start := time.Now()
	if err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&categories).Error; err != nil {
		metrics.DBQueryFinished("category_repository", "GetByIDs", time.Since(start))
		return nil, err
	}
	metrics.DBQueryFinished("category_repository", "GetByIDs", time.Since(start))
	return categories, nil
}

func (r *categoryRepository) Create(ctx context.Context, category *model.Category) error {
	start := time.Now()
	err := r.db.WithContext(ctx).Create(category).Error
	metrics.DBQueryFinished("category_repository", "Create", time.Since(start))
	return err
}

func (r *categoryRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var count int64
	start := time.Now()
	if err := r.db.WithContext(ctx).Model(&model.Category{}).Where("id = ?", id).Count(&count).Error; err != nil {
		metrics.DBQueryFinished("category_repository", "Exists", time.Since(start))
		return false, err
	}
	metrics.DBQueryFinished("category_repository", "Exists", time.Since(start))
	return count > 0, nil
}
