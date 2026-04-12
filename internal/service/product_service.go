package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
	"gorm.io/gorm"
)

type productService struct {
	productRepo  repository.ProductRepository
	categoryRepo repository.CategoryRepository
	logger       *slog.Logger
}

func NewProductService(
	productRepo repository.ProductRepository,
	categoryRepo repository.CategoryRepository,
	logger *slog.Logger,
) ProductService {
	return &productService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		logger:       logger,
	}
}

func (s *productService) ListProducts(ctx context.Context, filter ProductFilter) ([]*model.Product, error) {
	if filter.MinPrice != nil && *filter.MinPrice < 0 {
		return nil, apperrors.InvalidArgument("minPrice must be >= 0")
	}
	if filter.MaxPrice != nil && *filter.MaxPrice < 0 {
		return nil, apperrors.InvalidArgument("maxPrice must be >= 0")
	}
	if filter.MinPrice != nil && filter.MaxPrice != nil && *filter.MinPrice > *filter.MaxPrice {
		return nil, apperrors.InvalidArgument("minPrice cannot be greater than maxPrice")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		return nil, apperrors.InvalidArgument("limit cannot be greater than 100")
	}
	if filter.Offset < 0 {
		return nil, apperrors.InvalidArgument("offset cannot be negative")
	}

	var categoryUUID *uuid.UUID
	if filter.CategoryID != nil && strings.TrimSpace(*filter.CategoryID) != "" {
		id, err := uuid.Parse(*filter.CategoryID)
		if err != nil {
			return nil, apperrors.InvalidArgument("categoryId must be a valid UUID")
		}
		categoryUUID = &id
	}

	products, err := s.productRepo.List(ctx, repository.ProductFilter{
		MinPrice:   filter.MinPrice,
		MaxPrice:   filter.MaxPrice,
		CategoryID: categoryUUID,
		NameSearch: filter.NameSearch,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
		SortBy:     filter.SortBy,
		SortOrder:  filter.SortOrder,
	})
	if err != nil {
		s.logger.Error("failed listing products", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to list products", err)
	}
	return products, nil
}

func (s *productService) GetProduct(ctx context.Context, id string) (*model.Product, error) {
	productID, err := uuid.Parse(id)
	if err != nil {
		return nil, apperrors.InvalidArgument("id must be a valid UUID")
	}

	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NotFound("product not found")
		}
		s.logger.Error("failed fetching product", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to fetch product", err)
	}
	return product, nil
}

func (s *productService) CreateProduct(ctx context.Context, input CreateProductInput) (*model.Product, error) {
	name := normalizeName(input.Name)
	if name == "" {
		return nil, apperrors.InvalidArgument("name is required")
	}
	if input.Price < 0 {
		return nil, apperrors.InvalidArgument("price must be >= 0")
	}
	if input.StockQuantity < 0 {
		return nil, apperrors.InvalidArgument("stockQuantity must be >= 0")
	}

	currency := "USD"
	if input.Currency != nil && strings.TrimSpace(*input.Currency) != "" {
		currency = normalizeCurrency(*input.Currency)
	}
	if !currencyRegex.MatchString(currency) {
		return nil, apperrors.InvalidArgument("currency must be a valid ISO 4217 code")
	}

	categoryID, err := uuid.Parse(input.CategoryID)
	if err != nil {
		return nil, apperrors.InvalidArgument("categoryId must be a valid UUID")
	}

	exists, err := s.categoryRepo.Exists(ctx, categoryID)
	if err != nil {
		s.logger.Error("failed validating category existence", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed validating category", err)
	}
	if !exists {
		return nil, apperrors.InvalidArgument("categoryId does not reference an existing category")
	}

	product := &model.Product{
		Name:          name,
		Description:   input.Description,
		Price:         input.Price,
		Currency:      currency,
		CategoryID:    categoryID,
		StockQuantity: input.StockQuantity,
	}

	if err := s.productRepo.Create(ctx, product); err != nil {
		s.logger.Error("failed creating product", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to create product", err)
	}
	return product, nil
}

func (s *productService) UpdateProduct(ctx context.Context, id string, input UpdateProductInput) (*model.Product, error) {
	productID, err := uuid.Parse(id)
	if err != nil {
		return nil, apperrors.InvalidArgument("id must be a valid UUID")
	}

	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NotFound("product not found")
		}
		s.logger.Error("failed loading product for update", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to update product", err)
	}

	if input.Name != nil {
		name := normalizeName(*input.Name)
		if name == "" {
			return nil, apperrors.InvalidArgument("name cannot be empty")
		}
		product.Name = name
	}
	if input.Description != nil {
		product.Description = input.Description
	}
	if input.Price != nil {
		if *input.Price < 0 {
			return nil, apperrors.InvalidArgument("price must be >= 0")
		}
		product.Price = *input.Price
	}
	if input.Currency != nil {
		currency := normalizeCurrency(*input.Currency)
		if !currencyRegex.MatchString(currency) {
			return nil, apperrors.InvalidArgument("currency must be a valid ISO 4217 code")
		}
		product.Currency = currency
	}
	if input.StockQuantity != nil {
		if *input.StockQuantity < 0 {
			return nil, apperrors.InvalidArgument("stockQuantity must be >= 0")
		}
		product.StockQuantity = *input.StockQuantity
	}
	if input.CategoryID != nil && strings.TrimSpace(*input.CategoryID) != "" {
		categoryID, parseErr := uuid.Parse(*input.CategoryID)
		if parseErr != nil {
			return nil, apperrors.InvalidArgument("categoryId must be a valid UUID")
		}
		exists, existsErr := s.categoryRepo.Exists(ctx, categoryID)
		if existsErr != nil {
			s.logger.Error("failed validating category during product update", slog.String("error", existsErr.Error()))
			return nil, apperrors.Internal("failed validating category", existsErr)
		}
		if !exists {
			return nil, apperrors.InvalidArgument("categoryId does not reference an existing category")
		}
		product.CategoryID = categoryID
	}

	if err := s.productRepo.Update(ctx, product); err != nil {
		s.logger.Error("failed updating product", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to update product", err)
	}

	return product, nil
}

func (s *productService) DeleteProduct(ctx context.Context, id string) (bool, error) {
	productID, err := uuid.Parse(id)
	if err != nil {
		return false, apperrors.InvalidArgument("id must be a valid UUID")
	}

	deleted, err := s.productRepo.Delete(ctx, productID)
	if err != nil {
		s.logger.Error("failed deleting product", slog.String("error", err.Error()))
		return false, apperrors.Internal("failed to delete product", err)
	}
	if !deleted {
		return false, apperrors.NotFound("product not found")
	}
	return true, nil
}

func (s *productService) ListProductsByCategory(ctx context.Context, categoryID uuid.UUID, limit int, offset int) ([]*model.Product, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, apperrors.InvalidArgument("limit cannot be greater than 100")
	}
	if offset < 0 {
		return nil, apperrors.InvalidArgument("offset cannot be negative")
	}

	exists, err := s.categoryRepo.Exists(ctx, categoryID)
	if err != nil {
		return nil, apperrors.Internal("failed validating category", err)
	}
	if !exists {
		return nil, apperrors.NotFound(fmt.Sprintf("category %s not found", categoryID))
	}

	products, err := s.productRepo.ListByCategory(ctx, categoryID, limit, offset)
	if err != nil {
		return nil, apperrors.Internal("failed to list products by category", err)
	}
	return products, nil
}
