package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"omnirack-replica/backend/internal/model"
)

type Config struct {
	AppEnv       string
	HTTPAddr     string
	DatabaseURL  string
	JWTSecret    string
	JWTExpiresIn time.Duration
	AdminUser    string
	AdminPass    string
}

func Load() (Config, error) {
	jwtMinutes, err := intEnv("JWT_EXPIRES_MINUTES", 120)
	if err != nil {
		return Config{}, err
	}
	if jwtMinutes > 7*24*60 {
		return Config{}, fmt.Errorf("JWT_EXPIRES_MINUTES must not exceed 10080 (7 days)")
	}
	cfg := Config{
		AppEnv:       strings.ToLower(strings.TrimSpace(env("APP_ENV", "development"))),
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		DatabaseURL:  env("DATABASE_URL", "host=localhost user=omnirack_replica password=omnirack_replica dbname=omnirack_replica port=5432 sslmode=disable TimeZone=UTC"),
		JWTSecret:    env("JWT_SECRET", "change-me-in-production"),
		JWTExpiresIn: time.Duration(jwtMinutes) * time.Minute,
		AdminUser:    strings.TrimSpace(env("ADMIN_USERNAME", "admin")),
		AdminPass:    env("ADMIN_PASSWORD", "ChangeMe123!"),
	}
	if cfg.AppEnv != "development" && cfg.AppEnv != "test" && cfg.AppEnv != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test, or production")
	}
	if cfg.AppEnv == "production" && (len(strings.TrimSpace(cfg.JWTSecret)) < 32 || cfg.JWTSecret == "change-me-in-production") {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 bytes in production")
	}
	if cfg.AdminPass == "ChangeMe123!" && cfg.AppEnv == "production" {
		return Config{}, fmt.Errorf("ADMIN_PASSWORD must be changed in production")
	}
	if err := model.ValidatePassword(cfg.AdminPass); err != nil {
		return Config{}, fmt.Errorf("ADMIN_PASSWORD: %w", err)
	}
	if strings.TrimSpace(cfg.AdminUser) == "" || len(cfg.AdminUser) > 100 {
		return Config{}, fmt.Errorf("ADMIN_USERNAME must contain 1 to 100 bytes")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return parsed, nil
}
