package resolver

import (
	"context"
	"errors"

	"github.com/99designs/gqlgen/graphql"
	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func GraphQLErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	gqlErr := graphql.DefaultErrorPresenter(ctx, err)

	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		gqlErr.Message = appErr.Message
		if gqlErr.Extensions == nil {
			gqlErr.Extensions = map[string]any{}
		}
		gqlErr.Extensions["code"] = appErr.Code
		return gqlErr
	}

	if gqlErr.Extensions == nil {
		gqlErr.Extensions = map[string]any{}
	}
	gqlErr.Extensions["code"] = apperrors.CodeInternal
	return gqlErr
}
