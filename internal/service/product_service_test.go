package service

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
)

type mockProductRepository struct {
	listFn           func(ctx context.Context, filter repository.ProductFilter) ([]*model.Product, error)
	getByIDFn        func(ctx context.Context, id uuid.UUID) (*model.Product, error)
	createFn         func(ctx context.Context, product *model.Product) error
	updateFn         func(ctx context.Context, product *model.Product) error
	deleteFn         func(ctx context.Context, id uuid.UUID) (bool, error)
	listByCategoryFn func(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*model.Product, error)
}

func (m *mockProductRepository) List(ctx context.Context, filter repository.ProductFilter) ([]*model.Product, error) {
	if m.listFn == nil {
		return nil, nil
	}
	return m.listFn(ctx, filter)
}

func (m *mockProductRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Product, error) {
	if m.getByIDFn == nil {
		return nil, nil
	}
	return m.getByIDFn(ctx, id)
}

func (m *mockProductRepository) Create(ctx context.Context, product *model.Product) error {
	if m.createFn == nil {
		return nil
	}
	return m.createFn(ctx, product)
}

func (m *mockProductRepository) Update(ctx context.Context, product *model.Product) error {
	if m.updateFn == nil {
		return nil
	}
	return m.updateFn(ctx, product)
}

func (m *mockProductRepository) Delete(ctx context.Context, id uuid.UUID) (bool, error) {
	if m.deleteFn == nil {
		return false, nil
	}
	return m.deleteFn(ctx, id)
}

func (m *mockProductRepository) ListByCategory(ctx context.Context, categoryID uuid.UUID, limit, offset int) ([]*model.Product, error) {
	if m.listByCategoryFn == nil {
		return nil, nil
	}
	return m.listByCategoryFn(ctx, categoryID, limit, offset)
}

type mockCategoryRepository struct {
	listFn     func(ctx context.Context) ([]*model.Category, error)
	getByIDFn  func(ctx context.Context, id uuid.UUID) (*model.Category, error)
	getByIDsFn func(ctx context.Context, ids []uuid.UUID) ([]*model.Category, error)
	createFn   func(ctx context.Context, category *model.Category) error
	existsFn   func(ctx context.Context, id uuid.UUID) (bool, error)
}

func (m *mockCategoryRepository) List(ctx context.Context) ([]*model.Category, error) {
	if m.listFn == nil {
		return nil, nil
	}
	return m.listFn(ctx)
}

func (m *mockCategoryRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	if m.getByIDFn == nil {
		return nil, nil
	}
	return m.getByIDFn(ctx, id)
}

func (m *mockCategoryRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*model.Category, error) {
	if m.getByIDsFn == nil {
		return nil, nil
	}
	return m.getByIDsFn(ctx, ids)
}

func (m *mockCategoryRepository) Create(ctx context.Context, category *model.Category) error {
	if m.createFn == nil {
		return nil
	}
	return m.createFn(ctx, category)
}

func (m *mockCategoryRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	if m.existsFn == nil {
		return false, nil
	}
	return m.existsFn(ctx, id)
}

func TestCreateProduct_DefaultCurrency(t *testing.T) {
	categoryID := uuid.New()
	productRepo := &mockProductRepository{
		createFn: func(_ context.Context, product *model.Product) error {
			product.ID = uuid.New()
			product.CreatedAt = time.Now().UTC()
			product.UpdatedAt = product.CreatedAt
			return nil
		},
	}
	categoryRepo := &mockCategoryRepository{
		existsFn: func(_ context.Context, id uuid.UUID) (bool, error) {
			return id == categoryID, nil
		},
	}

	svc := NewProductService(productRepo, categoryRepo, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	product, err := svc.CreateProduct(context.Background(), CreateProductInput{
		Name:          "Wireless Mouse",
		Price:         49.99,
		CategoryID:    categoryID.String(),
		StockQuantity: 12,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if product.Currency != "USD" {
		t.Fatalf("expected default currency USD, got %s", product.Currency)
	}
}

func TestCreateProduct_InvalidName(t *testing.T) {
	svc := NewProductService(&mockProductRepository{}, &mockCategoryRepository{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	_, err := svc.CreateProduct(context.Background(), CreateProductInput{
		Name:          "   ",
		Price:         12,
		CategoryID:    uuid.New().String(),
		StockQuantity: 1,
	})

	if err == nil {
		t.Fatal("expected validation error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestListProducts_InvalidPriceRange(t *testing.T) {
	svc := NewProductService(&mockProductRepository{}, &mockCategoryRepository{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	min := 100.0
	max := 10.0

	_, err := svc.ListProducts(context.Background(), ProductFilter{MinPrice: &min, MaxPrice: &max})
	if err == nil {
		t.Fatal("expected validation error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
}

func TestDeleteProduct_NotFound(t *testing.T) {
	productRepo := &mockProductRepository{
		deleteFn: func(_ context.Context, _ uuid.UUID) (bool, error) {
			return false, nil
		},
	}
	svc := NewProductService(productRepo, &mockCategoryRepository{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	_, err := svc.DeleteProduct(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected not found error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %v", err)
	}
}
