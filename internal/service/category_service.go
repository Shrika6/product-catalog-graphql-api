package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
	"gorm.io/gorm"
)

type categoryService struct {
	categoryRepo repository.CategoryRepository
	logger       *slog.Logger
}

func NewCategoryService(categoryRepo repository.CategoryRepository, logger *slog.Logger) CategoryService {
	return &categoryService{categoryRepo: categoryRepo, logger: logger}
}

func (s *categoryService) ListCategories(ctx context.Context) ([]*model.Category, error) {
	categories, err := s.categoryRepo.List(ctx)
	if err != nil {
		s.logger.Error("failed listing categories", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to list categories", err)
	}
	return categories, nil
}

func (s *categoryService) GetCategory(ctx context.Context, id string) (*model.Category, error) {
	categoryID, err := uuid.Parse(id)
	if err != nil {
		return nil, apperrors.InvalidArgument("id must be a valid UUID")
	}

	category, err := s.categoryRepo.GetByID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NotFound("category not found")
		}
		s.logger.Error("failed fetching category", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to fetch category", err)
	}
	return category, nil
}

func (s *categoryService) CreateCategory(ctx context.Context, input CreateCategoryInput) (*model.Category, error) {
	name := normalizeName(input.Name)
	if name == "" {
		return nil, apperrors.InvalidArgument("name is required")
	}

	category := &model.Category{Name: name}
	if err := s.categoryRepo.Create(ctx, category); err != nil {
		s.logger.Error("failed creating category", slog.String("error", err.Error()))
		return nil, apperrors.Internal("failed to create category", err)
	}
	return category, nil
}
