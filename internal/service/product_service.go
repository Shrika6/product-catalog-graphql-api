package service

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
	"gorm.io/gorm"
)

type productService struct {
	productRepo  repository.ProductRepository
	categoryRepo repository.CategoryRepository
	logger       *slog.Logger
	cache        *redis.Client
}

func NewProductService(
	productRepo repository.ProductRepository,
	categoryRepo repository.CategoryRepository,
	logger *slog.Logger,
	cache *redis.Client,
) ProductService {
	return &productService{
		productRepo:  productRepo,
		categoryRepo: categoryRepo,
		logger:       logger,
		cache:        cache,
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

	if cachedProducts, ok := s.getCachedProductList(ctx, filter); ok {
		return cachedProducts, nil
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
	s.cacheProductList(ctx, filter, products)
	return products, nil
}

func (s *productService) GetProduct(ctx context.Context, id string) (*model.Product, error) {
	productID, err := uuid.Parse(id)
	if err != nil {
		return nil, apperrors.InvalidArgument("id must be a valid UUID")
	}

	if cachedProduct, ok := s.getCachedProduct(ctx, productID); ok {
		return cachedProduct, nil
	}

	product, err := s.productRepo.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NotFound("product not found")
		}
		s.logger.Error("failed fetching product", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to fetch product", err)
	}
	s.cacheProduct(ctx, productID, product)
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
	s.cacheProduct(ctx, product.ID, product)
	s.bumpProductListVersion(ctx)
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
	s.cacheProduct(ctx, product.ID, product)
	s.bumpProductListVersion(ctx)

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
	s.deleteProductCache(ctx, productID)
	s.bumpProductListVersion(ctx)
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

const (
	productCacheTTL        = 5 * time.Minute
	productListVersionKey  = "products:list:version"
	productByIDKeyPrefix   = "products:by-id:"
	productListKeyPrefix   = "products:list:"
	defaultListVersionSeed = "1"
)

type productListCacheInput struct {
	MinPrice   *float64 `json:"minPrice,omitempty"`
	MaxPrice   *float64 `json:"maxPrice,omitempty"`
	CategoryID *string  `json:"categoryId,omitempty"`
	NameSearch *string  `json:"nameSearch,omitempty"`
	Limit      int      `json:"limit"`
	Offset     int      `json:"offset"`
	SortBy     string   `json:"sortBy"`
	SortOrder  string   `json:"sortOrder"`
}

func (s *productService) cacheEnabled() bool {
	return s.cache != nil
}

func (s *productService) productCacheKey(id uuid.UUID) string {
	return productByIDKeyPrefix + id.String()
}

func (s *productService) productListCacheKey(ctx context.Context, filter ProductFilter) (string, bool) {
	if !s.cacheEnabled() {
		return "", false
	}

	version := s.productListVersion(ctx)
	if version == "" {
		return "", false
	}

	payload := productListCacheInput{
		MinPrice:   filter.MinPrice,
		MaxPrice:   filter.MaxPrice,
		CategoryID: filter.CategoryID,
		NameSearch: filter.NameSearch,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
		SortBy:     filter.SortBy,
		SortOrder:  filter.SortOrder,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		s.logger.Warn("failed to marshal cache key", slog.String("error", err.Error()))
		return "", false
	}

	hash := sha1.Sum(raw)
	return fmt.Sprintf("%s%s:%x", productListKeyPrefix, version, hash), true
}

func (s *productService) productListVersion(ctx context.Context) string {
	if !s.cacheEnabled() {
		return ""
	}

	version, err := s.cache.Get(ctx, productListVersionKey).Result()
	if err == nil {
		return version
	}
	if errors.Is(err, redis.Nil) {
		if setErr := s.cache.Set(ctx, productListVersionKey, defaultListVersionSeed, 0).Err(); setErr != nil {
			s.logger.Warn("failed to seed product list cache version", slog.String("error", setErr.Error()))
			return ""
		}
		return defaultListVersionSeed
	}

	s.logger.Warn("failed to read product list cache version", slog.String("error", err.Error()))
	return ""
}

func (s *productService) bumpProductListVersion(ctx context.Context) {
	if !s.cacheEnabled() {
		return
	}
	if err := s.cache.Incr(ctx, productListVersionKey).Err(); err != nil {
		s.logger.Warn("failed to bump product list cache version", slog.String("error", err.Error()))
	}
}

func (s *productService) getCachedProductList(ctx context.Context, filter ProductFilter) ([]*model.Product, bool) {
	key, ok := s.productListCacheKey(ctx, filter)
	if !ok {
		return nil, false
	}

	payload, err := s.cache.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			s.logger.Warn("failed to read product list cache", slog.String("error", err.Error()))
		}
		return nil, false
	}

	var products []*model.Product
	if err := json.Unmarshal(payload, &products); err != nil {
		s.logger.Warn("failed to decode product list cache", slog.String("error", err.Error()))
		_ = s.cache.Del(ctx, key).Err()
		return nil, false
	}
	return products, true
}

func (s *productService) cacheProductList(ctx context.Context, filter ProductFilter, products []*model.Product) {
	key, ok := s.productListCacheKey(ctx, filter)
	if !ok {
		return
	}
	payload, err := json.Marshal(products)
	if err != nil {
		s.logger.Warn("failed to encode product list cache", slog.String("error", err.Error()))
		return
	}
	if err := s.cache.Set(ctx, key, payload, productCacheTTL).Err(); err != nil {
		s.logger.Warn("failed to set product list cache", slog.String("error", err.Error()))
	}
}

func (s *productService) getCachedProduct(ctx context.Context, id uuid.UUID) (*model.Product, bool) {
	if !s.cacheEnabled() {
		return nil, false
	}

	key := s.productCacheKey(id)
	payload, err := s.cache.Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, redis.Nil) {
			s.logger.Warn("failed to read product cache", slog.String("error", err.Error()))
		}
		return nil, false
	}

	var product model.Product
	if err := json.Unmarshal(payload, &product); err != nil {
		s.logger.Warn("failed to decode product cache", slog.String("error", err.Error()))
		_ = s.cache.Del(ctx, key).Err()
		return nil, false
	}

	return &product, true
}

func (s *productService) cacheProduct(ctx context.Context, id uuid.UUID, product *model.Product) {
	if !s.cacheEnabled() {
		return
	}
	payload, err := json.Marshal(product)
	if err != nil {
		s.logger.Warn("failed to encode product cache", slog.String("error", err.Error()))
		return
	}
	if err := s.cache.Set(ctx, s.productCacheKey(id), payload, productCacheTTL).Err(); err != nil {
		s.logger.Warn("failed to set product cache", slog.String("error", err.Error()))
	}
}

func (s *productService) deleteProductCache(ctx context.Context, id uuid.UUID) {
	if !s.cacheEnabled() {
		return
	}
	if err := s.cache.Del(ctx, s.productCacheKey(id)).Err(); err != nil {
		s.logger.Warn("failed to delete product cache", slog.String("error", err.Error()))
	}
}
