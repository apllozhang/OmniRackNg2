package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestCatalogDeleteRejectsReferencesAndStockHistory(t *testing.T) {
	for _, tt := range []struct {
		name, kind, table string
		quantity          int
		state             string
		counts            []string
		values            []int
	}{
		{"site assigned to asset", "asset_site", "asset_sites", 0, "", []string{"asset_profiles"}, []int{1}},
		{"spec used in minimum rule", "spare_spec", "spare_specs", 0, "", []string{"spare_stocks", "spare_minimums"}, []int{0, 1}},
		{"stock on hand", "spare_stock", "spare_stocks", 1, "AVAILABLE", nil, nil},
		{"installed zero is not empty draft", "spare_stock", "spare_stocks", 0, "INSTALLED", nil, nil},
		{"stock has movement", "spare_stock", "spare_stocks", 0, "AVAILABLE", []string{"spare_movements"}, []int{1}},
		{"stock has check snapshot", "spare_stock", "spare_stocks", 0, "AVAILABLE", []string{"spare_movements", "asset_check_lines"}, []int{0, 1}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			s, m, closeDB := opsMock(t)
			defer closeDB()
			id := uuid.New()
			m.ExpectBegin()
			rows := sqlmock.NewRows([]string{"id", "version", "quantity", "state"}).AddRow(id, 3, tt.quantity, tt.state)
			m.ExpectQuery(`SELECT .*` + tt.table).WillReturnRows(rows)
			for i, table := range tt.counts {
				m.ExpectQuery(`SELECT count.*` + table).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.values[i]))
			}
			m.ExpectRollback()
			if err := s.DeleteCatalog(context.Background(), Actor{}, tt.kind, id, 3); !errors.Is(err, ErrInvalidResource) {
				t.Fatal("unsafe delete accepted", err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogDeleteUnusedSiteAuditsAndDatabaseRejectsConcurrentReference(t *testing.T) {
	for _, concurrent := range []bool{false, true} {
		t.Run(map[bool]string{false: "unused", true: "concurrent foreign key"}[concurrent], func(t *testing.T) {
			s, m, closeDB := opsMock(t)
			defer closeDB()
			id := uuid.New()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*asset_sites`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "name"}).AddRow(id, 1, "unused site"))
			for _, table := range []string{"asset_profiles", "spare_stocks", "spare_minimums", "asset_check_tasks", "spare_movements"} {
				m.ExpectQuery(`SELECT count.*` + table).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			}
			if concurrent {
				m.ExpectExec(`DELETE FROM "asset_sites"`).WillReturnError(gorm.ErrForeignKeyViolated)
				m.ExpectRollback()
			} else {
				m.ExpectExec(`DELETE FROM "asset_sites"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			err := s.DeleteCatalog(context.Background(), Actor{}, "asset_site", id, 1)
			if concurrent && !errors.Is(err, ErrInvalidResource) {
				t.Fatal("foreign key failure not mapped", err)
			}
			if !concurrent && err != nil {
				t.Fatal(err)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogDeleteRequiresCurrentVersion(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	id := uuid.New()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*spare_minimums`).WillReturnRows(sqlmock.NewRows([]string{"id", "version"}).AddRow(id, 5))
	m.ExpectRollback()
	if err := s.DeleteCatalog(context.Background(), Actor{}, "spare_minimum", id, 4); !errors.Is(err, ErrResourceVersion) {
		t.Fatal("stale deletion accepted", err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSiteRebindingBlockedButMetadataCorrectionAllowed(t *testing.T) {
	for _, rebind := range []bool{false, true} {
		t.Run(map[bool]string{false: "rename", true: "room rebind"}[rebind], func(t *testing.T) {
			s, m, closeDB := opsMock(t)
			defer closeDB()
			id, room, next := uuid.New(), uuid.New(), uuid.New()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*asset_sites`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "name", "region", "kind", "room_id"}).AddRow(id, 2, "before", "上海", "ROOM", room))
			in := model.AssetSite{BaseModel: model.BaseModel{Version: 2}, Name: "after", Region: "上海", Kind: "ROOM", RoomID: &room, Address: "corrected"}
			if rebind {
				in.RoomID = &next
				m.ExpectQuery(`SELECT count.*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				m.ExpectRollback()
			} else {
				m.ExpectQuery(`SELECT .*rooms`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(room))
				m.ExpectExec(`UPDATE "asset_sites"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			out, err := s.UpdateSite(context.Background(), Actor{}, id, in)
			if rebind && !errors.Is(err, ErrInvalidResource) {
				t.Fatal("rebound a used location", err)
			}
			if !rebind && (err != nil || out.Name != "after" || out.Version != 3) {
				t.Fatal("metadata not updated", out, err)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStockEditDoesNotRewriteQuantityOrState(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	id, spec, site := uuid.New(), uuid.New(), uuid.New()
	var in StockUpdateInput
	raw := `{"version":4,"specId":"` + spec.String() + `","siteId":"` + site.String() + `","bin":"S02","batch":"corrected","quantity":999,"state":"RETIRED"}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "spec_id", "site_id", "quantity", "state"}).AddRow(id, 4, spec, site, 12, "AVAILABLE"))
	m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(sqlmock.NewRows([]string{"id", "tracking"}).AddRow(spec, "QUANTITY"))
	m.ExpectQuery(`SELECT .*asset_sites`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(site))
	m.ExpectExec(`UPDATE "spare_stocks"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	out, err := s.UpdateStock(context.Background(), Actor{}, id, in)
	if err != nil || out.Quantity != 12 || out.State != "AVAILABLE" || out.Bin != "S02" || out.Version != 5 {
		t.Fatal("stock edit bypassed movement flow", out, err)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestStockNotesPreserveOmittedAndAllowExplicitEditOrClear(t *testing.T) {
	for _, mode := range []string{"omitted", "edited", "cleared"} {
		t.Run(mode, func(t *testing.T) {
			s, m, closeDB := opsMock(t)
			defer closeDB()
			id, spec, site := uuid.New(), uuid.New(), uuid.New()
			in := StockUpdateInput{Version: 4, SpecID: spec, SiteID: site}
			expected := "旧备注"
			if mode != "omitted" {
				value := " 新备注 "
				expected = "新备注"
				if mode == "cleared" {
					value = ""
					expected = ""
				}
				in.Notes = &value
			}
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "spec_id", "site_id", "quantity", "state", "notes"}).AddRow(id, 4, spec, site, 12, "AVAILABLE", "旧备注"))
			m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(sqlmock.NewRows([]string{"id", "tracking"}).AddRow(spec, "QUANTITY"))
			m.ExpectQuery(`SELECT .*asset_sites`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(site))
			m.ExpectExec(`UPDATE "spare_stocks"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
			out, err := s.UpdateStock(context.Background(), Actor{}, id, in)
			if err != nil || out.Notes != expected || out.Quantity != 12 || out.State != "AVAILABLE" {
				t.Fatalf("notes changed stock semantics: %+v %v", out, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUsedSpecCannotChangeTrackingOrUnit(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	id := uuid.New()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*spare_specs`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "tracking", "unit"}).AddRow(id, 1, "SERIAL", "件"))
	m.ExpectQuery(`SELECT count.*spare_stocks`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	m.ExpectRollback()
	_, err := s.UpdateSpec(context.Background(), Actor{}, id, model.SpareSpec{BaseModel: model.BaseModel{Version: 1}, Name: "memory", Category: "内存", PartNumber: "MEM-32G", Specification: "32GB", Tracking: "QUANTITY", Unit: "件"})
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatal("changed tracking of referenced spec", err)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMinimumDeleteDeactivatesReminderAndPreservesAudit(t *testing.T) {
	s, m, closeDB := opsMock(t)
	defer closeDB()
	id := uuid.New()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*spare_minimums`).WillReturnRows(sqlmock.NewRows([]string{"id", "version"}).AddRow(id, 2))
	m.ExpectExec(`DELETE FROM "spare_minimums"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`UPDATE "asset_reminders"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	if err := s.DeleteCatalog(context.Background(), Actor{}, "spare_minimum", id, 2); err != nil {
		t.Fatal(err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
