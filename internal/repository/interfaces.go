package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
)

type ProductFilter struct {
	MinPrice   *float64
	MaxPrice   *float64
	CategoryID *uuid.UUID
	NameSearch *string
	Limit      int
	Offset     int
	SortBy     string
	SortOrder  string
}

type ProductRepository interface {
	List(ctx context.Context, filter ProductFilter) ([]*model.Product, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error)
	Create(ctx context.Context, product *model.Product) error
	Update(ctx context.Context, product *model.Product) error
	Delete(ctx context.Context, id uuid.UUID) (bool, error)
	ListByCategory(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*model.Product, error)
}

type CategoryRepository interface {
	List(ctx context.Context) ([]*model.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*model.Category, error)
	Create(ctx context.Context, category *model.Category) error
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
}
