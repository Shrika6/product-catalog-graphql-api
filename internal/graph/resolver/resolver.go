package resolver

import (
	"log/slog"

	"github.com/shrika/product-catalog-graphql-api/internal/service"
)

// Resolver wires GraphQL resolvers to application services.
type Resolver struct {
	ProductService  service.ProductService
	CategoryService service.CategoryService
	Logger          *slog.Logger
}

func New(productService service.ProductService, categoryService service.CategoryService, logger *slog.Logger) *Resolver {
	return &Resolver{
		ProductService:  productService,
		CategoryService: categoryService,
		Logger:          logger,
	}
}
