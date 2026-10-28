package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
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
	cfg := Config{
		AppEnv:       env("APP_ENV", "development"),
		HTTPAddr:     env("HTTP_ADDR", ":8080"),
		DatabaseURL:  env("DATABASE_URL", "host=localhost user=omnirack_replica password=omnirack_replica dbname=omnirack_replica port=5432 sslmode=disable TimeZone=UTC"),
		JWTSecret:    env("JWT_SECRET", "change-me-in-production"),
		JWTExpiresIn: time.Duration(jwtMinutes) * time.Minute,
		AdminUser:    env("ADMIN_USERNAME", "admin"),
		AdminPass:    env("ADMIN_PASSWORD", "ChangeMe123!"),
	}
	if cfg.JWTSecret == "change-me-in-production" && cfg.AppEnv == "production" {
		return Config{}, fmt.Errorf("JWT_SECRET must be changed in production")
	}
	if cfg.AdminPass == "ChangeMe123!" && cfg.AppEnv == "production" {
		return Config{}, fmt.Errorf("ADMIN_PASSWORD must be changed in production")
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
