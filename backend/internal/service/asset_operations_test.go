package service

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"testing"
	"time"
)

func TestWarrantyCalendarBoundariesAndThirdParty(t *testing.T) {
	now := time.Date(2026, 10, 19, 23, 59, 0, 0, chinaZone)
	date := func(days int) *time.Time { v := dateOnly(now).AddDate(0, 0, days); return &v }
	tests := []struct {
		name  string
		d     model.Device
		p     model.AssetProfile
		cover []model.AssetCoverage
		want  string
	}{
		{name: "unknown", want: "UNKNOWN"},
		{name: "explicit none", d: model.Device{WarrantyExpiresAt: date(100)}, p: model.AssetProfile{NoWarranty: true}, want: "NONE"},
		{name: "last day still covered", d: model.Device{WarrantyExpiresAt: date(0)}, want: "SOON"},
		{name: "30 days", d: model.Device{WarrantyExpiresAt: date(30)}, want: "SOON"},
		{name: "31 days", d: model.Device{WarrantyExpiresAt: date(31)}, want: "VALID"},
		{name: "expired", d: model.Device{WarrantyExpiresAt: date(-1)}, want: "EXPIRED"},
		{name: "third party covers expired factory", d: model.Device{WarrantyExpiresAt: date(-20)}, cover: []model.AssetCoverage{{StartsOn: *date(-1), EndsOn: *date(200)}}, want: "VALID"},
		{name: "future service leaves gap", d: model.Device{WarrantyExpiresAt: date(-1)}, cover: []model.AssetCoverage{{StartsOn: *date(2), EndsOn: *date(200)}}, want: "GAP"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateWarranty(tt.d, tt.p, tt.cover, now)
			if got.Status != tt.want {
				t.Fatalf("got %s want %s", got.Status, tt.want)
			}
		})
	}
	utc := time.Date(2026, 10, 19, 17, 0, 0, 0, time.UTC)
	if got := EvaluateWarranty(model.Device{WarrantyExpiresAt: date(0)}, model.AssetProfile{}, nil, utc); got.Status != "EXPIRED" {
		t.Fatalf("Shanghai calendar must already be Oct20, got %s", got.Status)
	}
}

func TestSpareMoveValidation(t *testing.T) {
	destination, device := uuid.New(), uuid.New()
	base := model.SpareStock{BaseModel: model.BaseModel{Version: 3}, SiteID: uuid.New(), Quantity: 5, State: "AVAILABLE", Spec: model.SpareSpec{Tracking: "QUANTITY"}}
	input := SpareMoveInput{Version: 3, SubmissionID: uuid.New(), Operation: "OUT", Quantity: 2, Reason: "维修领用"}
	tests := []struct {
		name      string
		stock     func(*model.SpareStock)
		change    func(*SpareMoveInput)
		wantError bool
	}{
		{name: "valid quantity out"},
		{name: "mark available stock faulty", change: func(i *SpareMoveInput) { i.Operation = "FAULT"; i.Quantity = 5 }},
		{name: "reject partial batch fault", change: func(i *SpareMoveInput) { i.Operation = "FAULT"; i.Quantity = 1 }, wantError: true},
		{name: "no oversell", change: func(i *SpareMoveInput) { i.Quantity = 6 }, wantError: true},
		{name: "positive", change: func(i *SpareMoveInput) { i.Quantity = 0 }, wantError: true},
		{name: "version conflict", change: func(i *SpareMoveInput) { i.Version = 1 }, wantError: true},
		{name: "needs reason", change: func(i *SpareMoveInput) { i.Reason = " " }, wantError: true},
		{name: "needs idempotency key", change: func(i *SpareMoveInput) { i.SubmissionID = uuid.Nil }, wantError: true},
		{name: "reserved cannot consume", stock: func(s *model.SpareStock) { s.State = "RESERVED" }, wantError: true},
		{name: "serial cannot disappear via out", stock: func(s *model.SpareStock) { s.Spec.Tracking = "SERIAL"; s.Quantity = 1 }, change: func(i *SpareMoveInput) { i.Quantity = 1 }, wantError: true},
		{name: "valid transfer", change: func(i *SpareMoveInput) { i.Operation = "TRANSFER"; i.DestinationSiteID = &destination }},
		{name: "transfer needs destination", change: func(i *SpareMoveInput) { i.Operation = "TRANSFER" }, wantError: true},
		{name: "partial state switch refused", change: func(i *SpareMoveInput) { i.Operation = "RESERVE" }, wantError: true},
		{name: "full state switch", change: func(i *SpareMoveInput) { i.Operation = "RESERVE"; i.Quantity = 5 }},
		{name: "install serial", stock: func(s *model.SpareStock) { s.Spec.Tracking = "SERIAL"; s.Quantity = 1 }, change: func(i *SpareMoveInput) { i.Operation = "INSTALL"; i.Quantity = 1; i.DeviceID = &device }},
		{name: "retire requires evidence", stock: func(s *model.SpareStock) { s.State = "FAULTY" }, change: func(i *SpareMoveInput) { i.Operation = "RETIRE"; i.Quantity = 5 }, wantError: true},
		{name: "unknown operation", change: func(i *SpareMoveInput) { i.Operation = "HACK" }, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, i := base, input
			if tt.stock != nil {
				tt.stock(&s)
			}
			if tt.change != nil {
				tt.change(&i)
			}
			e := ValidateSpareMove(s, i)
			if (e != nil) != tt.wantError {
				t.Fatalf("error=%v wantError=%v", e, tt.wantError)
			}
		})
	}
}

func TestCheckDoesNotTreatUnverifiedConfigurationAsVerified(t *testing.T) {
	line := model.AssetCheckLine{Kind: "DEVICE", Expected: 1, TargetVersion: 4}
	in := CheckScanInput{Actual: 1}
	result, e := CheckResult(line, in, 4)
	if e != nil || result != "NORMAL" {
		t.Fatalf("result %s err %v", result, e)
	}
	if in.ConfigurationVerified {
		t.Fatal("default must remain unverified")
	}
	result, e = CheckResult(line, in, 5)
	if e != nil || result != "CONFLICT" {
		t.Fatalf("changed target result %s err %v", result, e)
	}
	in.Actual = 0
	if _, e = CheckResult(line, in, 4); e == nil {
		t.Fatal("difference needs note")
	}
	in.Note = "现场未找到"
	if got, e := CheckResult(line, in, 4); e != nil || got != "DIFFERENCE" {
		t.Fatalf("%s %v", got, e)
	}
	in.Actual = 2
	if _, e = CheckResult(line, in, 4); e == nil {
		t.Fatal("single asset count cannot exceed 1")
	}
}

func opsMock(t *testing.T) (*AssetOperations, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	db, e := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{TranslateError: true, Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	return NewAssetOperations(db), m, func() { sqlDB.Close() }
}

func TestDuplicateStockRequestReturnsCommittedMovementWithoutAnotherWrite(t *testing.T) {
	svc, m, close := opsMock(t)
	defer close()
	actor, stock, key, record := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id", "stock_id", "actor_id", "submission_id"}).AddRow(record, stock, actor, key))
	m.ExpectCommit()
	got, e := svc.MoveStock(context.Background(), Actor{UserID: &actor}, stock, SpareMoveInput{SubmissionID: key})
	if e != nil || got.ID != record {
		t.Fatalf("id=%s err=%v", got.ID, e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestInsufficientStockRollsBackBeforeWritingMovement(t *testing.T) {
	svc, m, close := opsMock(t)
	defer close()
	actor, stock, key, spec, site := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id", "spec_id", "site_id", "quantity", "state", "version"}).AddRow(stock, spec, site, 2, "AVAILABLE", 1))
	m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(sqlmock.NewRows([]string{"id", "tracking"}).AddRow(spec, "QUANTITY"))
	m.ExpectRollback()
	_, e := svc.MoveStock(context.Background(), Actor{UserID: &actor}, stock, SpareMoveInput{SubmissionID: key, Version: 1, Operation: "OUT", Quantity: 3, Reason: "超领"})
	if !errors.Is(e, ErrInvalidResource) {
		t.Fatalf("got %v", e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
