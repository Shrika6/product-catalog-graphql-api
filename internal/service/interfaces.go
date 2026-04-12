package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
)

type ProductService interface {
	ListProducts(ctx context.Context, filter ProductFilter) ([]*model.Product, error)
	GetProduct(ctx context.Context, id string) (*model.Product, error)
	CreateProduct(ctx context.Context, input CreateProductInput) (*model.Product, error)
	UpdateProduct(ctx context.Context, id string, input UpdateProductInput) (*model.Product, error)
	DeleteProduct(ctx context.Context, id string) (bool, error)
	ListProductsByCategory(ctx context.Context, categoryID uuid.UUID, limit int, offset int) ([]*model.Product, error)
}

type CategoryService interface {
	ListCategories(ctx context.Context) ([]*model.Category, error)
	GetCategory(ctx context.Context, id string) (*model.Category, error)
	CreateCategory(ctx context.Context, input CreateCategoryInput) (*model.Category, error)
}
