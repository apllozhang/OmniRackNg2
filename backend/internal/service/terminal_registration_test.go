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

func TestTerminalRegistrationValidationAndLegacyStates(t *testing.T) {
	site := uuid.New()
	for _, state := range []string{"IDLE", "IN_USE", "REPAIR", "RETIRED"} {
		in := TerminalRegistration{Status: state}
		if state == "IN_USE" {
			in.Assignee = "张三"
			in.Department = "研究部"
			in.SiteID = &site
		}
		if err := validateTerminalRegistration(in); err != nil {
			t.Fatal(state, err)
		}
	}
	for _, in := range []TerminalRegistration{{Status: "WAITING_RACK"}, {Status: "RUNNING"}, {Status: "IN_USE"}, {Status: "IN_USE", Assignee: "张三", Department: "研究部"}, {Status: "IDLE", Assignee: "张三"}} {
		if validateTerminalRegistration(in) == nil {
			t.Fatalf("accepted invalid initial state %+v", in)
		}
	}
	for _, tt := range []struct{ core, profile, want string }{{"WAITING_RACK", "IN_USE", "IN_USE"}, {"WAITING_RACK", "REPAIR", "REPAIR"}, {"SCRAPPED", "IN_USE", "RETIRED"}, {"MAINTENANCE", "", "REPAIR"}, {"WAITING_RACK", "", "IDLE"}} {
		if got := terminalUsage(model.Device{LifecycleStatus: tt.core}, model.AssetProfile{UsageStatus: tt.profile}); got != tt.want {
			t.Fatalf("state drift: %s / %s -> %s", tt.core, tt.profile, got)
		}
	}
}

func TestTerminalRegistrationIsAtomicForAllInitialStates(t *testing.T) {
	for _, state := range []string{"IDLE", "IN_USE", "REPAIR", "RETIRED", "ROLLBACK"} {
		t.Run(state, func(t *testing.T) {
			ops, m, closeDB := opsMock(t)
			defer closeDB()
			typID, siteID := uuid.New(), uuid.New()
			in := TerminalRegistration{Status: state}
			if state == "IN_USE" {
				in.Assignee = "张三"
				in.Department = "研究部"
				in.SiteID = &siteID
			}
			failHistory := state == "ROLLBACK"
			if failHistory {
				in.Status = "IDLE"
			}
			typeRows := func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"id", "category", "status", "default_height_u"}).AddRow(typID, "PC", "ACTIVE", 1)
			}
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(typeRows())
			if in.SiteID != nil {
				m.ExpectQuery(`SELECT .*asset_sites.*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "region"}).AddRow(siteID, "总部", "上海"))
			}
			m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(typeRows())
			m.ExpectExec(`SAVEPOINT sp`).WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(`SELECT count\(\*\).*devices`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectExec(`INSERT INTO "devices"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`INSERT INTO "asset_profiles"`).WillReturnResult(sqlmock.NewResult(0, 1))
			historyErr := errors.New("history unavailable")
			if failHistory {
				m.ExpectExec(`INSERT INTO "pc_handovers"`).WillReturnError(historyErr)
				m.ExpectRollback()
			} else {
				m.ExpectExec(`INSERT INTO "pc_handovers"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			var profile model.AssetProfile
			var history model.PCHandover
			ops.DB.Callback().Create().Before("gorm:create").Register("capture_registration", func(tx *gorm.DB) {
				if x, ok := tx.Statement.Dest.(*model.AssetProfile); ok {
					profile = *x
				}
				if x, ok := tx.Statement.Dest.(*model.PCHandover); ok {
					history = *x
				}
			})
			out, err := ops.RegisterTerminal(context.Background(), Actor{}, DeviceCreateInput{Device: model.Device{TypeID: typID, Code: "TERMINAL-1", Name: "已使用终端", LifecycleStatus: "RUNNING", HeightU: 42}}, in)
			if failHistory {
				if !errors.Is(err, historyErr) {
					t.Fatalf("expected rollback, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				wantCore := "WAITING_RACK"
				if in.Status == "RETIRED" {
					wantCore = "SCRAPPED"
				}
				if out.ID == uuid.Nil || out.LifecycleStatus != wantCore || out.HeightU != 1 || profile.DeviceID != out.ID || profile.UsageStatus != in.Status || history.Operation != "REGISTER" || history.After.UsageStatus != in.Status || history.Before.UsageStatus != "" {
					t.Fatalf("inconsistent registration: %+v / %+v / %+v", out, profile, history)
				}
				if in.Status == "IN_USE" && (profile.Assignee != "张三" || history.After.SiteName != "上海 / 总部") {
					t.Fatal("owner or location lost")
				}
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTerminalMaintenanceTransitions(t *testing.T) {
	site := uuid.New()
	for _, tt := range []struct {
		state, action, condition, want string
		bad                            bool
	}{
		{"IDLE", "REPAIR", "FAULTY", "REPAIR", false},
		{"REPAIR", "RESTORE", "NORMAL", "IDLE", false},
		{"IDLE", "RETIRE", "NORMAL", "RETIRED", false},
		{"REPAIR", "RETIRE", "FAULTY", "RETIRED", false},
		{"IN_USE", "RETIRE", "NORMAL", "", true},
		{"IN_USE", "REPAIR", "FAULTY", "", true},
		{"IDLE", "RESTORE", "NORMAL", "", true},
		{"RETIRED", "RESTORE", "NORMAL", "", true},
		{"IDLE", "REPAIR", "NORMAL", "", true},
	} {
		p := model.AssetProfile{UsageStatus: tt.state, CPU: "保留配置"}
		out, err := nextPCCustody(p, PCHandoverInput{Operation: tt.action, Condition: tt.condition, SiteID: site, Note: "管理员确认"})
		if (err != nil) != tt.bad {
			t.Fatalf("%+v: %v", tt, err)
		}
		if !tt.bad && (out.UsageStatus != tt.want || out.Assignee != "" || out.CPU != p.CPU) {
			t.Fatalf("bad transition: %+v", out)
		}
	}
}

func TestTerminalDisposalCommitsProfileDeviceAndHistoryTogether(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	id, typeID, siteID := uuid.New(), uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*devices.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "type_id", "version", "lifecycle_status"}).AddRow(id, typeID, 5, "WAITING_RACK"))
	m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(sqlmock.NewRows([]string{"id", "category"}).AddRow(typeID, "PC"))
	m.ExpectQuery(`SELECT count\(\*\).*rack_device_positions`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.ExpectQuery(`SELECT .*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"id", "device_id", "version", "usage_status"}).AddRow(uuid.New(), id, 2, "REPAIR"))
	m.ExpectQuery(`SELECT .*asset_sites.*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "region"}).AddRow(siteID, "报废存放区", "上海"))
	m.ExpectExec(`UPDATE .*asset_profiles`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`UPDATE .*devices`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "pc_handovers"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	var coreState interface{}
	ops.DB.Callback().Update().Before("gorm:update").Register("capture_disposal", func(tx *gorm.DB) {
		if tx.Statement.Table == "devices" {
			if x, ok := tx.Statement.Dest.(map[string]interface{}); ok {
				coreState = x["lifecycle_status"]
			}
		}
	})
	out, err := ops.ConfirmPCHandover(context.Background(), Actor{}, id, PCHandoverInput{Version: 5, Operation: "RETIRE", SiteID: siteID, Condition: "FAULTY", Note: "故障无法修复，确认报废"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Before.UsageStatus != "REPAIR" || out.After.UsageStatus != "RETIRED" || out.Operation != "RETIRE" || coreState != "SCRAPPED" {
		t.Fatalf("disposal state drift: %+v / %v", out, coreState)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
