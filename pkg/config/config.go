package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppEnv                string
	Port                  string
	LogLevel              string
	RequestTimeout        time.Duration
	DBHost                string
	DBPort                string
	DBUser                string
	DBPassword            string
	DBName                string
	DBSSLMode             string
	DBMaxOpenConns        int
	DBMaxIdleConns        int
	DBConnMaxLifetimeMins int
	BasicAuthUsername     string
	BasicAuthPassword     string
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		AppEnv:                getEnv("APP_ENV", "development"),
		Port:                  getEnv("APP_PORT", "8080"),
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		RequestTimeout:        time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 5)) * time.Second,
		DBHost:                getEnv("DB_HOST", "localhost"),
		DBPort:                getEnv("DB_PORT", "5432"),
		DBUser:                getEnv("DB_USER", "catalog_user"),
		DBPassword:            getEnv("DB_PASSWORD", "catalog_password"),
		DBName:                getEnv("DB_NAME", "catalog_db"),
		DBSSLMode:             getEnv("DB_SSLMODE", "disable"),
		DBMaxOpenConns:        getEnvInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:        getEnvInt("DB_MAX_IDLE_CONNS", 10),
		DBConnMaxLifetimeMins: getEnvInt("DB_CONN_MAX_LIFETIME_MINUTES", 30),
		BasicAuthUsername:     os.Getenv("BASIC_AUTH_USERNAME"),
		BasicAuthPassword:     os.Getenv("BASIC_AUTH_PASSWORD"),
	}

	if cfg.DBHost == "" || cfg.DBPort == "" || cfg.DBUser == "" || cfg.DBName == "" {
		return Config{}, fmt.Errorf("database environment variables are incomplete")
	}

	return cfg, nil
}

func (c Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost,
		c.DBPort,
		c.DBUser,
		c.DBPassword,
		c.DBName,
		c.DBSSLMode,
	)
}

func getEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := getEnv(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}
