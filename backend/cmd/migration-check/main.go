// migration-check runs only schema migration in an explicitly disposable clone.
// It never seeds accounts, starts HTTP/background jobs or contacts integrations.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"gorm.io/gorm/logger"
	"omnirack-replica/backend/internal/repository"
)

func cloneURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" {
		return errors.New("DATABASE_URL must be an explicit PostgreSQL URL for the disposable clone")
	}
	for _, key := range []string{"host", "hostaddr", "port", "dbname", "service", "options", "search_path"} {
		if _, present := u.Query()[key]; present {
			return errors.New("connection destination/search_path overrides are forbidden")
		}
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		return errors.New("cannot parse disposable clone URL")
	}
	loopback := func(host string) bool {
		address := net.ParseIP(host)
		return address != nil && address.IsLoopback()
	}
	if !loopback(u.Hostname()) || !loopback(cfg.Host) || cfg.Database != "omnirack_integration" {
		return errors.New("migration-check accepts only numeric loopback and database omnirack_integration")
	}
	for _, fallback := range cfg.Fallbacks {
		if !loopback(fallback.Host) {
			return errors.New("all fallback hosts must be numeric loopback")
		}
	}
	return nil
}

func run() error {
	raw := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if err := cloneURL(raw); err != nil {
		return err
	}
	if os.Getenv("OMNIRACK_DISPOSABLE_MIGRATION_ACK") != "owned-restored-clone" {
		return errors.New("set OMNIRACK_DISPOSABLE_MIGRATION_ACK=owned-restored-clone")
	}
	db, err := repository.Open(raw)
	if err != nil {
		return errors.New("cannot connect to the disposable restored clone")
	}
	db.Logger = logger.Default.LogMode(logger.Silent)
	pool, err := db.DB()
	if err != nil {
		return errors.New("cannot obtain clone connection pool")
	}
	defer pool.Close()
	pool.SetMaxOpenConns(4)
	pool.SetMaxIdleConns(2)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var schema string
	if err := db.WithContext(ctx).Raw("SELECT current_schema()").Scan(&schema).Error; err != nil || schema != "public" {
		return errors.New("refusing migration: clone current_schema must be public")
	}
	if err := repository.Migrate(db.WithContext(ctx)); err != nil {
		return fmt.Errorf("migration in the owned restored clone failed: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
		"passed": true, "mode": "migration-only", "database": "omnirack_integration",
		"seeds_started": false, "workers_started": false,
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
