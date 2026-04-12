package db

import (
	"log/slog"
	"time"

	"github.com/shrika/product-catalog-graphql-api/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func NewPostgres(cfg config.Config, logger *slog.Logger) (*gorm.DB, error) {
	gormLogLevel := gormlogger.Silent
	if cfg.AppEnv == "development" {
		gormLogLevel = gormlogger.Warn
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxOpenConns(cfg.DBMaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.DBMaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeMins) * time.Minute)

	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	logger.Info("postgres connection established",
		slog.Int("max_open_conns", cfg.DBMaxOpenConns),
		slog.Int("max_idle_conns", cfg.DBMaxIdleConns),
	)

	return db, nil
}
