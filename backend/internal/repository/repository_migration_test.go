package repository

import (
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
		`ALTER TABLE rack_u_occupancies DROP CONSTRAINT IF EXISTS rack_u_occupancies_no_overlap`,
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
