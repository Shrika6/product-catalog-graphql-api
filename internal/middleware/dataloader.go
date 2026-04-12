package middleware

import (
	"net/http"

	"github.com/shrika/product-catalog-graphql-api/internal/graph/dataloader"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
)

func DataLoader(categoryRepo repository.CategoryRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			loaders := dataloader.New(categoryRepo)
			ctx := dataloader.Inject(r.Context(), loaders)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
