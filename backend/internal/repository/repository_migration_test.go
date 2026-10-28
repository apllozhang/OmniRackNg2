package repository

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestEnsureDevicePositionConstraintsDoesNotRequireRemovedFaceColumns(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New: %v", err)
	}
	defer sqlDB.Close()

	db, err := gorm.Open(postgres.New(postgres.Config{
		Conn:       sqlDB,
		DriverName: "postgres",
	}), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS btree_gist`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_asset_active`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_device_positions_device_active`,
		`DO $$ BEGIN`,
	}
	for _, statement := range statements {
		mock.ExpectExec(regexp.QuoteMeta(statement)).WillReturnResult(sqlmock.NewResult(0, 0))
	}

	if err := ensureDevicePositionConstraints(db); err != nil {
		t.Fatalf("ensureDevicePositionConstraints: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected migration SQL: %v", err)
	}
}

func TestMigrateAcquiresTransactionLockBeforeSchemaChanges(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	lockFailure := errors.New("database shutting down")
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("SELECT pg_advisory_xact_lock($1, $2)")).WithArgs(20261028, 1).WillReturnError(lockFailure)
	mock.ExpectRollback()
	if err := Migrate(db); !errors.Is(err, lockFailure) {
		t.Fatalf("migration continued without serialization: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
