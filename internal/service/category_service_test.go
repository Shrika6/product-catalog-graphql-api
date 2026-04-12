package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
)

func TestCreateCategory_EmptyName(t *testing.T) {
	svc := NewCategoryService(&mockCategoryRepository{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	_, err := svc.CreateCategory(context.Background(), CreateCategoryInput{Name: "   "})
	if err == nil {
		t.Fatal("expected validation error")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.Code != apperrors.CodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %v", err)
	}
}
