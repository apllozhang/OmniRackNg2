package service

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"testing"
)

func TestSpareReceiptCreatesStockAndMovementAtomically(t *testing.T) {
	for _, mode := range []string{"existing", "first registration", "TESTING", "FAULTY", "REPAIR", "rollback"} {
		fail := mode == "rollback"
		manual := mode == "first registration"
		state := "AVAILABLE"
		if mode == "TESTING" || mode == "FAULTY" || mode == "REPAIR" {
			state = mode
		}
		t.Run(mode, func(t *testing.T) {
			s, m, closeDB := opsMock(t)
			defer closeDB()
			actorID, specID, siteID := uuid.New(), uuid.New(), uuid.New()
			stockID := uuid.New()
			s.DB.Callback().Create().Before("gorm:create").Register("receipt_stock_id", func(tx *gorm.DB) {
				if x, ok := tx.Statement.Dest.(*model.SpareSpec); ok {
					x.ID = specID
				}
				if x, ok := tx.Statement.Dest.(*model.SpareMovement); ok {
					if x.BeforeState != "AVAILABLE" || x.AfterState != state {
						t.Errorf("wrong movement states: %+v", x)
					}
				}
				if x, ok := tx.Statement.Dest.(*model.SpareStock); ok {
					x.ID = stockID
				}
			})
			specRows := func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"id", "name", "category", "tracking", "unit"}).AddRow(specID, "内存", "内存", "QUANTITY", "件")
			}
			m.ExpectBegin()
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			if manual {
				m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				m.ExpectExec(`INSERT INTO "spare_specs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
			} else {
				m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(specRows())
			}
			m.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(specRows())
			m.ExpectQuery(`SELECT .*asset_sites`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(siteID))
			m.ExpectExec(`INSERT INTO "spare_stocks"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "spec_id", "site_id", "quantity", "state", "notes"}).AddRow(stockID, 1, specID, siteID, 0, "AVAILABLE", "机房备用"))
			m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(specRows())
			m.ExpectExec(`UPDATE "spare_stocks"`).WillReturnResult(sqlmock.NewResult(0, 1))
			if fail {
				m.ExpectExec(`INSERT INTO "spare_movements"`).WillReturnError(errors.New("movement unavailable"))
				m.ExpectExec(`ROLLBACK TO SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
				m.ExpectRollback()
			} else {
				m.ExpectExec(`INSERT INTO "spare_movements"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			in := SpareReceiptInput{SubmissionID: uuid.New(), SpecID: &specID, SiteID: siteID, Quantity: 3, Notes: "机房备用", State: state}
			if manual {
				in.SpecID = nil
				in.Name = "内存"
				in.Category = "内存"
				in.ModelNumber = "32GB"
			}
			out, err := s.ReceiveSpare(context.Background(), Actor{UserID: &actorID}, in)
			if fail {
				if err == nil {
					t.Fatal("receipt ignored movement failure")
				}
			} else if err != nil || out.Quantity != 3 || out.Version != 2 || out.State != state || out.Notes != "机房备用" {
				t.Fatalf("wrong receipt: %+v %v", out, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSpareReceiptRetryDoesNotDoubleStock(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	actorID, stockID, specID, siteID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id", "stock_id", "actor_id", "operation", "quantity", "reason"}).AddRow(uuid.New(), stockID, actorID, "IN", 3, "备件入库登记"))
	m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id", "spec_id", "site_id", "quantity", "state"}).AddRow(stockID, specID, siteID, 3, "AVAILABLE"))
	// GORM preloads associations alphabetically.
	m.ExpectQuery(`SELECT .*asset_sites`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(siteID))
	m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(specID))
	m.ExpectCommit()
	out, err := s.ReceiveSpare(context.Background(), Actor{UserID: &actorID}, SpareReceiptInput{SubmissionID: uuid.New(), SpecID: &specID, SiteID: siteID, Quantity: 3})
	if err != nil || out.Quantity != 3 {
		t.Fatalf("retry failed: %+v %v", out, err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSpareReceiptRejectsInvalidQuantityStateAndIdentity(t *testing.T) {
	actorID, specID, siteID := uuid.New(), uuid.New(), uuid.New()
	base := SpareReceiptInput{SubmissionID: uuid.New(), SpecID: &specID, SiteID: siteID, Quantity: 1}
	for _, kind := range []string{"negative", "serial count", "missing site", "missing type", "installed"} {
		t.Run(kind, func(t *testing.T) {
			in := base
			switch kind {
			case "negative":
				in.Quantity = -1
			case "serial count":
				in.Quantity = 2
				in.SerialNumber = "SN1"
			case "missing site":
				in.SiteID = uuid.Nil
			case "missing type":
				in.SpecID = nil
			case "installed":
				in.State = "INSTALLED"
			}
			if _, err := NewAssetOperations(nil).ReceiveSpare(context.Background(), Actor{UserID: &actorID}, in); err == nil {
				t.Fatal("invalid receipt accepted")
			}
		})
	}
}

func TestReceiptRetryRejectsDifferentInitialState(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	actorID, siteID := uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*spare_movements`).WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "operation", "quantity", "reason", "after_state"}).AddRow(uuid.New(), actorID, "IN", 1, "备件入库登记", "FAULTY"))
	m.ExpectRollback()
	_, err := s.ReceiveSpare(context.Background(), Actor{UserID: &actorID}, SpareReceiptInput{SubmissionID: uuid.New(), SiteID: siteID, Name: "硬盘", Category: "硬盘", Quantity: 1, State: "REPAIR"})
	if err == nil {
		t.Fatal("accepted a retry with a different initial state")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
