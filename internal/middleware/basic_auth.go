package middleware

import (
	"net/http"

	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
)

func BasicAuth(username, password string) func(http.Handler) http.Handler {
	if username == "" || password == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			providedUser, providedPass, ok := r.BasicAuth()
			if !ok || providedUser != username || providedPass != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="catalog-api"`)
				http.Error(w, apperrors.Unauthorized("unauthorized").Message, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
