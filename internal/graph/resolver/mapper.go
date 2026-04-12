package resolver

import (
	"github.com/shrika/product-catalog-graphql-api/internal/graph/model"
	domain "github.com/shrika/product-catalog-graphql-api/internal/model"
)

func toGraphQLProduct(product *domain.Product) *model.Product {
	if product == nil {
		return nil
	}

	graphqlProduct := &model.Product{
		ID:            product.ID.String(),
		Name:          product.Name,
		Description:   product.Description,
		Price:         product.Price,
		Currency:      product.Currency,
		CategoryID:    product.CategoryID.String(),
		StockQuantity: product.StockQuantity,
		CreatedAt:     product.CreatedAt,
		UpdatedAt:     product.UpdatedAt,
	}

	if product.Category != nil {
		graphqlProduct.Category = toGraphQLCategory(product.Category)
	}

	return graphqlProduct
}

func toGraphQLProducts(products []*domain.Product) []*model.Product {
	result := make([]*model.Product, 0, len(products))
	for _, product := range products {
		result = append(result, toGraphQLProduct(product))
	}
	return result
}

func toGraphQLCategory(category *domain.Category) *model.Category {
	if category == nil {
		return nil
	}
	return &model.Category{
		ID:        category.ID.String(),
		Name:      category.Name,
		CreatedAt: category.CreatedAt,
	}
}

func toGraphQLCategories(categories []*domain.Category) []*model.Category {
	result := make([]*model.Category, 0, len(categories))
	for _, category := range categories {
		result = append(result, toGraphQLCategory(category))
	}
	return result
}
