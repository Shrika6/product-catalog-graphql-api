package repository

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/pkg/metrics"
	"gorm.io/gorm"
)

type productRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ProductRepository {
	return &productRepository{db: db}
}

func (r *productRepository) List(ctx context.Context, filter ProductFilter) ([]*model.Product, error) {
	var products []*model.Product

	query := r.db.WithContext(ctx).Model(&model.Product{})

	if filter.MinPrice != nil {
		query = query.Where("price >= ?", *filter.MinPrice)
	}
	if filter.MaxPrice != nil {
		query = query.Where("price <= ?", *filter.MaxPrice)
	}
	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}
	if filter.NameSearch != nil && *filter.NameSearch != "" {
		if tsQuery, ok := buildTSQuery(*filter.NameSearch); ok {
			query = query.Where(
				"to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, '')) @@ to_tsquery('english', ?)",
				tsQuery,
			)
		}
	}

	sortBy := resolveSortBy(filter.SortBy)
	sortOrder := resolveSortOrder(filter.SortOrder)
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	start := time.Now()
	if err := query.Limit(limit).Offset(offset).Find(&products).Error; err != nil {
		metrics.DBQueryFinished("product_repository", "List", time.Since(start))
		return nil, err
	}
	metrics.DBQueryFinished("product_repository", "List", time.Since(start))
	return products, nil
}

func (r *productRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	var product model.Product
	start := time.Now()
	if err := r.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
		metrics.DBQueryFinished("product_repository", "GetByID", time.Since(start))
		return nil, err
	}
	metrics.DBQueryFinished("product_repository", "GetByID", time.Since(start))
	return &product, nil
}

func (r *productRepository) Create(ctx context.Context, product *model.Product) error {
	start := time.Now()
	err := r.db.WithContext(ctx).Create(product).Error
	metrics.DBQueryFinished("product_repository", "Create", time.Since(start))
	return err
}

func (r *productRepository) Update(ctx context.Context, product *model.Product) error {
	start := time.Now()
	err := r.db.WithContext(ctx).Save(product).Error
	metrics.DBQueryFinished("product_repository", "Update", time.Since(start))
	return err
}

func (r *productRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	start := time.Now()
	result := r.db.WithContext(ctx).Delete(&model.Product{}, "id = ?", id)
	metrics.DBQueryFinished("product_repository", "Delete", time.Since(start))
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *productRepository) ListByCategory(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*model.Product, error) {
	var products []*model.Product
	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	start := time.Now()
	err := r.db.WithContext(ctx).
		Where("category_id = ?", categoryID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&products).Error
	metrics.DBQueryFinished("product_repository", "ListByCategory", time.Since(start))
	if err != nil {
		return nil, err
	}
	return products, nil
}

func resolveSortBy(sortBy string) string {
	switch sortBy {
	case "name":
		return "name"
	case "price":
		return "price"
	case "updated_at":
		return "updated_at"
	case "stock_quantity":
		return "stock_quantity"
	default:
		return "created_at"
	}
}

func resolveSortOrder(sortOrder string) string {
	if sortOrder == "ASC" {
		return "ASC"
	}
	return "DESC"
}

var nonWordRegex = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func buildTSQuery(input string) (string, bool) {
	normalized := nonWordRegex.ReplaceAllString(strings.TrimSpace(input), " ")
	if normalized == "" {
		return "", false
	}
	tokens := strings.Fields(normalized)
	if len(tokens) == 0 {
		return "", false
	}
	for i, token := range tokens {
		tokens[i] = strings.ToLower(token) + ":*"
	}
	return strings.Join(tokens, " & "), true
}
