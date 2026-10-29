// Package integration contains opt-in tests against a disposable PostgreSQL
// database. It never uses the application's configured DATABASE_URL.
package integration

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

const databaseEnv = "OMNIRACK_INTEGRATION_DATABASE_URL"
const schemaPrefixEnv = "OMNIRACK_INTEGRATION_SCHEMA_PREFIX"

var disposablePrefix = regexp.MustCompile(`^omnirack_it_[a-z0-9_]{0,24}$`)

// Each test creates and drops only a randomly named schema. The dedicated DB
// name and explicit prefix acknowledge the destructive nature of schema cleanup.
// btree_gist must already exist in public because an extension is database-wide.
func isolatedPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	raw := strings.TrimSpace(os.Getenv(databaseEnv))
	if raw == "" {
		t.Skip("real PostgreSQL tests are opt-in: set OMNIRACK_INTEGRATION_DATABASE_URL for the disposable omnirack_integration database")
	}
	prefix := strings.TrimSpace(os.Getenv(schemaPrefixEnv))
	if !disposablePrefix.MatchString(prefix) {
		t.Fatal("set OMNIRACK_INTEGRATION_SCHEMA_PREFIX to omnirack_it_ or that prefix followed by at most 24 lowercase letters, digits, or underscores")
	}
	cfg, err := integrationConfig(raw)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	bootstrap, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal("cannot connect to the explicitly configured disposable PostgreSQL database")
	}
	schema := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	quotedSchema := pgx.Identifier{schema}.Sanitize()
	created := false
	var pool *sql.DB
	t.Cleanup(func() {
		if pool != nil {
			_ = pool.Close()
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if created {
			if _, err := bootstrap.Exec(cleanupCtx, "DROP SCHEMA "+quotedSchema+" CASCADE"); err != nil {
				t.Errorf("could not clean up owned disposable schema %s", schema)
			}
		}
		_ = bootstrap.Close(cleanupCtx)
	})
	var extensionSchema string
	if err := bootstrap.QueryRow(ctx, `SELECT n.nspname FROM pg_extension e JOIN pg_namespace n ON n.oid = e.extnamespace WHERE e.extname = 'btree_gist'`).Scan(&extensionSchema); err != nil || extensionSchema != "public" {
		t.Fatal("prerequisite: install btree_gist in public in the disposable database before running; tests do not install database-wide extensions")
	}
	if _, err := bootstrap.Exec(ctx, "CREATE SCHEMA "+quotedSchema); err != nil {
		t.Fatal("could not create a fresh random disposable schema")
	}
	created = true
	t.Logf("isolated disposable schema: %s", schema)
	appCfg := cfg.Copy()
	appCfg.RuntimeParams = make(map[string]string, len(cfg.RuntimeParams)+1)
	for key, value := range cfg.RuntimeParams {
		appCfg.RuntimeParams[key] = value
	}
	appCfg.RuntimeParams["search_path"] = quotedSchema + ",public"
	appCfg.RuntimeParams["statement_timeout"] = "15000"
	appCfg.RuntimeParams["lock_timeout"] = "10000"
	pool = stdlib.OpenDB(*appCfg)
	pool.SetMaxOpenConns(8)
	pool.SetMaxIdleConns(4)
	if err := pool.PingContext(ctx); err != nil {
		t.Fatal("cannot open the disposable-schema connection pool")
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal("cannot initialize GORM in the disposable schema")
	}
	var actualSchema string
	if err := db.Raw("SELECT current_schema()").Scan(&actualSchema).Error; err != nil || actualSchema != schema {
		t.Fatal("refusing migration because PostgreSQL search_path is not the newly created disposable schema")
	}
	if err := repository.Migrate(db.WithContext(ctx)); err != nil {
		t.Fatalf("migration in owned disposable schema failed: %v", err)
	}
	return db
}

func integrationConfig(raw string) (*pgx.ConnConfig, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() == "" {
		return nil, errors.New("integration database must be an explicit postgres:// or postgresql:// loopback URL")
	}
	// Numeric loopback avoids DNS indirection and Unix sockets/SSH host aliases.
	if address := net.ParseIP(u.Hostname()); address == nil || !address.IsLoopback() {
		return nil, errors.New("integration database host must be a numeric loopback address such as 127.0.0.1 or ::1")
	}
	for _, key := range []string{"host", "hostaddr", "port", "dbname", "service", "options", "search_path"} {
		if _, exists := u.Query()[key]; exists {
			return nil, errors.New("integration URL must not override connection destinations or search_path in query parameters")
		}
	}
	cfg, err := pgx.ParseConfig(raw)
	if err != nil {
		return nil, errors.New("invalid integration PostgreSQL URL")
	}
	if cfg.Database != "omnirack_integration" {
		return nil, errors.New("integration URL must target the dedicated disposable database named omnirack_integration")
	}
	if address := net.ParseIP(cfg.Host); address == nil || !address.IsLoopback() {
		return nil, errors.New("parsed integration PostgreSQL host is not loopback")
	}
	for _, fallback := range cfg.Fallbacks {
		if address := net.ParseIP(fallback.Host); address == nil || !address.IsLoopback() {
			return nil, errors.New("integration PostgreSQL fallbacks must all remain on loopback")
		}
	}
	cfg.ConnectTimeout = 5 * time.Second
	return cfg, nil
}

func TestPostgresRepeatedMigratePreservesExclusionConstraintOID(t *testing.T) {
	db := isolatedPostgres(t)
	constraintOID := func() int64 {
		t.Helper()
		var oid int64
		err := db.Raw(`SELECT oid::bigint FROM pg_constraint WHERE conname = ? AND conrelid = 'rack_u_occupancies'::regclass`, "rack_u_occupancies_no_overlap").Scan(&oid).Error
		if err != nil || oid == 0 {
			t.Fatalf("expected an installed rack-U exclusion constraint, oid=%d err=%v", oid, err)
		}
		return oid
	}
	before := constraintOID()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := repository.Migrate(db.WithContext(ctx)); err != nil {
		t.Fatalf("repeated migration failed: %v", err)
	}
	if after := constraintOID(); after != before {
		t.Fatalf("startup replaced the existing exclusion constraint: before OID=%d after OID=%d", before, after)
	}
	t.Logf("PASS: repeated Migrate retained exclusion-constraint OID %d", before)
}

func TestPostgresConcurrentOverlappingOccupancyOnlyOneCommits(t *testing.T) {
	db := isolatedPostgres(t)
	rackID := uuid.New()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ready, start, results := make(chan struct{}, 2), make(chan struct{}), make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			tx := db.WithContext(ctx).Begin()
			ready <- struct{}{}
			<-start
			if tx.Error != nil {
				results <- tx.Error
				return
			}
			// [10,12] and [12,15] overlap at the inclusive boundary.
			first, last := 10, 12
			if i == 1 {
				first, last = 12, 15
			}
			occupancy := model.RackUOccupancy{PositionID: uuid.New(), DeviceID: uuid.New(), RackID: rackID, StartU: first, EndU: last}
			err := tx.Create(&occupancy).Error
			if err != nil {
				_ = tx.Rollback().Error
			} else {
				err = tx.Commit().Error
			}
			// Exclusion-index arbitration may abort one simultaneous insertion with
			// 40P01. Retry that entire single-write transaction, not the assertion.
			// We still require a real 23P01 conflict and exactly one committed row.
			for attempt := 0; attempt < 2; attempt++ {
				var deadlock *pgconn.PgError
				if !errors.As(err, &deadlock) || deadlock.Code != "40P01" {
					break
				}
				t.Log("retrying owned single-write transaction after PostgreSQL 40P01")
				err = db.WithContext(ctx).Create(&occupancy).Error
			}
			results <- err
		}(i)
	}
	<-ready
	<-ready
	close(start)
	succeeded, excluded := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			succeeded++
			continue
		}
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23P01" && pgError.ConstraintName == "rack_u_occupancies_no_overlap" {
			excluded++
		} else {
			t.Fatalf("concurrent insert failed for a reason other than U-overlap enforcement: %v", err)
		}
	}
	var count int64
	if err := db.Model(&model.RackUOccupancy{}).Where("rack_id = ?", rackID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if succeeded != 1 || excluded != 1 || count != 1 {
		t.Fatalf("expected exactly one committed occupancy and one exclusion conflict: success=%d excluded=%d rows=%d", succeeded, excluded, count)
	}
	t.Log("PASS: two concurrent overlapping writes produced one commit and one PostgreSQL 23P01 exclusion conflict")
}

func TestPostgresConcurrentAdminChangesPreserveOneEnabledAdmin(t *testing.T) {
	for _, scenario := range []string{"demotion", "deletion", "mixed demotion and deletion", "disabling"} {
		t.Run(scenario, func(t *testing.T) {
			db := isolatedPostgres(t)
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			if err := repository.SeedRoles(ctx, db); err != nil {
				t.Fatal(err)
			}
			store := repository.NewUserStore(db)
			users := []model.User{
				{Username: "admin-a", DisplayName: "Integration admin A", Enabled: true, AuthSource: "local"},
				{Username: "admin-b", DisplayName: "Integration admin B", Enabled: true, AuthSource: "local"},
			}
			for i := range users {
				if err := users[i].SetPassword("Disposable-integration-only"); err != nil {
					t.Fatal(err)
				}
				if err := store.CreateUser(ctx, &users[i], []string{"system_admin"}); err != nil {
					t.Fatal(err)
				}
			}
			ready, start, results := make(chan struct{}, 2), make(chan struct{}), make(chan error, 2)
			for i := range users {
				go func(i int) {
					user := users[i]
					ready <- struct{}{}
					<-start
					if scenario == "deletion" || (scenario == "mixed demotion and deletion" && i == 1) {
						results <- store.DeleteUser(ctx, user.ID, user.Version)
						return
					}
					roles := []string{"user"}
					if scenario == "disabling" {
						user.Enabled = false
						roles = []string{"system_admin"}
					}
					results <- store.UpdateUser(ctx, &user, user.Version, roles)
				}(i)
			}
			<-ready
			<-ready
			close(start)
			succeeded, protected := 0, 0
			for i := 0; i < 2; i++ {
				err := <-results
				if err == nil {
					succeeded++
				} else if errors.Is(err, repository.ErrLastEnabledAdmin) {
					protected++
				} else {
					t.Fatalf("unexpected concurrent administrator-mutation error: %v", err)
				}
			}
			count, err := store.CountEnabledAdmins(ctx, uuid.Nil)
			if err != nil {
				t.Fatal(err)
			}
			if succeeded != 1 || protected != 1 || count != 1 {
				t.Fatalf("last-admin invariant failed: success=%d protected=%d enabledAdmins=%d", succeeded, protected, count)
			}
			t.Logf("PASS: concurrent %s retained exactly one enabled administrator; one mutation returned ErrLastEnabledAdmin", scenario)
		})
	}
}

func TestIntegrationConnectionGuardRejectsUnsafeDestinations(t *testing.T) {
	for _, raw := range []string{
		"postgres://test:test@192.0.2.1/omnirack_integration?sslmode=disable",
		"postgres://test:test@localhost/omnirack_integration?sslmode=disable",
		"postgres://test:test@127.0.0.1/production?sslmode=disable",
		"postgres://test:test@127.0.0.1/omnirack_integration?host=192.0.2.1",
		"postgres://test:test@127.0.0.1/omnirack_integration?search_path=public",
		"host=127.0.0.1 dbname=omnirack_integration",
	} {
		if _, err := integrationConfig(raw); err == nil {
			t.Fatal("accepted an unsafe integration database destination")
		}
	}
	if _, err := integrationConfig("postgres://test:test@127.0.0.1:55432/omnirack_integration?sslmode=disable"); err != nil {
		t.Fatalf("rejected the expected disposable loopback URL: %v", err)
	}
}
