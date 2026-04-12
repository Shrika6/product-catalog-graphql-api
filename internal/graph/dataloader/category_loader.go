package dataloader

import (
	"context"

	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/shrika/product-catalog-graphql-api/internal/model"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
)

type contextKey string

const loadersKey contextKey = "dataloaders"

type Loaders struct {
	CategoryByID *dataloader.Loader[string, *model.Category]
}

func New(categoryRepo repository.CategoryRepository) *Loaders {
	batchFn := func(ctx context.Context, keys []string) []*dataloader.Result[*model.Category] {
		ids := make([]uuid.UUID, 0, len(keys))
		for _, key := range keys {
			id, err := uuid.Parse(key)
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}

		categories, err := categoryRepo.GetByIDs(ctx, ids)
		if err != nil {
			results := make([]*dataloader.Result[*model.Category], len(keys))
			for i := range keys {
				results[i] = &dataloader.Result[*model.Category]{Error: err}
			}
			return results
		}

		categoryMap := make(map[string]*model.Category, len(categories))
		for _, category := range categories {
			categoryMap[category.ID.String()] = category
		}

		results := make([]*dataloader.Result[*model.Category], len(keys))
		for i, key := range keys {
			cat, ok := categoryMap[key]
			if !ok {
				results[i] = &dataloader.Result[*model.Category]{Error: nil, Data: nil}
				continue
			}
			results[i] = &dataloader.Result[*model.Category]{Data: cat}
		}
		return results
	}

	return &Loaders{
		CategoryByID: dataloader.NewBatchedLoader(batchFn),
	}
}

func Inject(ctx context.Context, loaders *Loaders) context.Context {
	return context.WithValue(ctx, loadersKey, loaders)
}

func For(ctx context.Context) (*Loaders, bool) {
	loaders, ok := ctx.Value(loadersKey).(*Loaders)
	return loaders, ok
}
