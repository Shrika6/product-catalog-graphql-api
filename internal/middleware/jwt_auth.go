package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	apperrors "github.com/shrika/product-catalog-graphql-api/pkg/errors"
)

type authContextKey string

const userContextKey authContextKey = "auth_user"

type AuthUser struct {
	Subject string
	Email   string
	Roles   []string
}

type Claims struct {
	Email string   `json:"email,omitempty"`
	Roles []string `json:"roles,omitempty"`
	jwt.RegisteredClaims
}

func WithUser(ctx context.Context, user *AuthUser) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*AuthUser, bool) {
	user, ok := ctx.Value(userContextKey).(*AuthUser)
	return user, ok
}

func JWTAuth(secret, issuer, audience string, logger *slog.Logger) func(http.Handler) http.Handler {
	if strings.TrimSpace(secret) == "" {
		logger.Warn("JWT_SECRET not set; mutations will be rejected")
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r)
			})
		}
	}

	parserOptions := []jwt.ParserOption{
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	}
	if issuer != "" {
		parserOptions = append(parserOptions, jwt.WithIssuer(issuer))
	}
	if audience != "" {
		parserOptions = append(parserOptions, jwt.WithAudience(audience))
	}

	parser := jwt.NewParser(parserOptions...)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				next.ServeHTTP(w, r)
				return
			}

			token, err := extractBearer(authHeader)
			if err != nil {
				http.Error(w, apperrors.Unauthorized("invalid authorization header").Message, http.StatusUnauthorized)
				return
			}

			claims := &Claims{}
			parsed, err := parser.ParseWithClaims(token, claims, func(token *jwt.Token) (any, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, apperrors.Unauthorized("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil || !parsed.Valid {
				http.Error(w, apperrors.Unauthorized("invalid token").Message, http.StatusUnauthorized)
				return
			}

			user := &AuthUser{
				Subject: claims.Subject,
				Email:   claims.Email,
				Roles:   claims.Roles,
			}

			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), user)))
		})
	}
}

func extractBearer(header string) (string, error) {
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 {
		return "", errors.New("invalid authorization header")
	}
	if !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization scheme")
	}
	if strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("empty token")
	}
	return parts[1], nil
}
