package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/shrika/product-catalog-graphql-api/internal/graph/generated"
	"github.com/shrika/product-catalog-graphql-api/internal/graph/resolver"
	"github.com/shrika/product-catalog-graphql-api/internal/middleware"
	"github.com/shrika/product-catalog-graphql-api/internal/repository"
	"github.com/shrika/product-catalog-graphql-api/internal/service"
	"github.com/shrika/product-catalog-graphql-api/pkg/config"
	"github.com/shrika/product-catalog-graphql-api/pkg/db"
	"github.com/shrika/product-catalog-graphql-api/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New(cfg.LogLevel)

	gormDB, err := db.NewPostgres(cfg, log)
	if err != nil {
		log.Error("failed to connect database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		log.Error("failed to get sql db handle", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		_ = sqlDB.Close()
	}()

	productRepo := repository.NewProductRepository(gormDB)
	categoryRepo := repository.NewCategoryRepository(gormDB)

	productService := service.NewProductService(productRepo, categoryRepo, log)
	categoryService := service.NewCategoryService(categoryRepo, log)

	resolverRoot := resolver.New(productService, categoryService, log)
	graphqlServer := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{Resolvers: resolverRoot}))
	graphqlServer.SetErrorPresenter(resolver.GraphQLErrorPresenter)

	queryHandler := middleware.Chain(
		graphqlServer,
		middleware.BasicAuth(cfg.BasicAuthUsername, cfg.BasicAuthPassword),
		middleware.RequestTimeout(cfg.RequestTimeout),
		middleware.DataLoader(categoryRepo),
		middleware.RequestLogging(log),
	)

	mux := http.NewServeMux()
	mux.Handle("/query", queryHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	if cfg.AppEnv != "production" {
		mux.Handle("/", playground.Handler("Product Catalog GraphQL", "/query"))
	}

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Info("starting server", slog.String("port", cfg.Port), slog.String("env", cfg.AppEnv))
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Error("server failed", slog.String("error", serveErr.Error()))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	log.Info("server shutdown complete")
}
