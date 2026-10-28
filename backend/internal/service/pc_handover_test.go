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

func TestPCHandoverStateTransitions(t *testing.T) {
	input := PCHandoverInput{Operation: "ISSUE", Assignee: "新使用人", Department: "研究部", SiteID: uuid.New(), Position: "A-01", Condition: "NORMAL", Note: "管理员确认交接"}
	for _, tt := range []struct {
		name, state, owner, action, condition, want string
		invalid                                     bool
	}{
		{"issue idle", "IDLE", "", "ISSUE", "NORMAL", "IN_USE", false},
		{"legacy idle with owner", "IDLE", "旧备注使用人", "ISSUE", "NORMAL", "IN_USE", false},
		{"cannot issue twice", "IN_USE", "张三", "ISSUE", "NORMAL", "", true},
		{"cannot issue repair", "REPAIR", "", "ISSUE", "NORMAL", "", true},
		{"return", "IN_USE", "张三", "RETURN", "NORMAL", "IDLE", false},
		{"faulty return", "IN_USE", "张三", "RETURN", "FAULTY", "REPAIR", false},
		{"return idle blocked", "IDLE", "", "RETURN", "NORMAL", "", true},
		{"transfer", "IN_USE", "张三", "TRANSFER", "NORMAL", "IN_USE", false},
		{"same owner blocked", "IN_USE", "新使用人", "TRANSFER", "NORMAL", "", true},
		{"transfer idle blocked", "IDLE", "", "TRANSFER", "NORMAL", "", true},
		{"invalid condition", "IN_USE", "张三", "RETURN", "UNKNOWN", "", true},
		{"faulty issue blocked", "IDLE", "", "ISSUE", "FAULTY", "", true},
		{"invalid action", "IDLE", "", "DELETE", "NORMAL", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p := model.AssetProfile{UsageStatus: tt.state, Assignee: tt.owner, Department: "原部门", CPU: "保留CPU", MemoryGB: 32, Disk: "保留硬盘"}
			in := input
			in.Operation, in.Condition = tt.action, tt.condition
			next, err := nextPCCustody(p, in)
			if (err != nil) != tt.invalid {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.invalid {
				return
			}
			if next.UsageStatus != tt.want || next.CPU != p.CPU || next.MemoryGB != 32 || next.Disk != p.Disk || next.SiteID == nil || *next.SiteID != input.SiteID {
				t.Fatalf("wrong next profile %+v", next)
			}
			if tt.action == "RETURN" && (next.Assignee != "" || next.Department != "") {
				t.Fatal("returned asset still assigned")
			}
			if tt.action != "RETURN" && next.Assignee != "新使用人" {
				t.Fatal("new owner not recorded")
			}
		})
	}
	for _, modify := range []func(*PCHandoverInput){func(x *PCHandoverInput) { x.Note = " " }, func(x *PCHandoverInput) { x.SiteID = uuid.Nil }, func(x *PCHandoverInput) { x.Assignee = " " }, func(x *PCHandoverInput) { x.Department = " " }} {
		x := input
		modify(&x)
		if _, err := nextPCCustody(model.AssetProfile{UsageStatus: "IDLE"}, x); err == nil {
			t.Fatal("accepted incomplete handover")
		}
	}
}

func TestPCHandoverTransactionAndHistoryRollback(t *testing.T) {
	for _, failHistory := range []bool{false, true} {
		name := "commit"
		if failHistory {
			name = "rollback"
		}
		t.Run(name, func(t *testing.T) {
			ops, m, closeDB := opsMock(t)
			defer closeDB()
			id, typeID, profileID, siteID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*devices.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "type_id", "version", "code", "name", "serial_number", "lifecycle_status"}).AddRow(id, typeID, 7, "PC-001", "测试PC", "SN-001", "WAITING_RACK"))
			m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(sqlmock.NewRows([]string{"id", "category"}).AddRow(typeID, "PC"))
			m.ExpectQuery(`SELECT count\(\*\).*rack_device_positions`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectQuery(`SELECT .*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"id", "device_id", "version", "usage_status", "assignee", "department", "cpu"}).AddRow(profileID, id, 3, "IN_USE", "张三", "IT", "原CPU"))
			m.ExpectQuery(`SELECT .*asset_sites.*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "region"}).AddRow(siteID, "办公区库房", "上海"))
			m.ExpectExec(`UPDATE .*asset_profiles`).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec(`UPDATE .*devices`).WillReturnResult(sqlmock.NewResult(0, 1))
			expected := errors.New("history insert failed")
			if failHistory {
				m.ExpectExec(`INSERT INTO "pc_handovers"`).WillReturnError(expected)
				m.ExpectRollback()
			} else {
				m.ExpectExec(`INSERT INTO "pc_handovers"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			}
			var saved *model.AssetProfile
			ops.DB.Callback().Update().Before("gorm:update").Register("capture_pc", func(tx *gorm.DB) {
				if x, ok := tx.Statement.Dest.(*model.AssetProfile); ok {
					copy := *x
					saved = &copy
				}
			})
			out, err := ops.ConfirmPCHandover(context.Background(), Actor{}, id, PCHandoverInput{Version: 7, Operation: "RETURN", SiteID: siteID, Condition: "FAULTY", Note: "故障归还，充电器齐全"})
			if failHistory {
				if !errors.Is(err, expected) {
					t.Fatalf("got %v", err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if out.Before.Assignee != "张三" || out.After.Assignee != "" || out.After.UsageStatus != "REPAIR" || out.DeviceCode != "PC-001" || out.SerialNumber != "SN-001" || out.After.SiteName != "上海 / 办公区库房" || saved == nil || saved.Version != 4 || saved.CPU != "原CPU" {
					t.Fatalf("bad handover %+v / %+v", out, saved)
				}
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPCHandoverStaleAndNonPCBlocked(t *testing.T) {
	for _, nonPC := range []bool{false, true} {
		ops, m, closeDB := opsMock(t)
		id, typ := uuid.New(), uuid.New()
		m.ExpectBegin()
		m.ExpectQuery(`SELECT .*devices.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "type_id", "version"}).AddRow(id, typ, 8))
		version := uint(7)
		if nonPC {
			version = 8
			m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(sqlmock.NewRows([]string{"id", "category"}).AddRow(typ, "SERVER"))
		}
		m.ExpectRollback()
		_, err := ops.ConfirmPCHandover(context.Background(), Actor{}, id, PCHandoverInput{Version: version})
		if err == nil || (!nonPC && !errors.Is(err, ErrResourceVersion)) {
			t.Fatalf("invalid handover accepted: %v", err)
		}
		if err := m.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		closeDB()
	}
}

func TestPCProfileCannotBypassHandover(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	id, typeID := uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*devices.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "type_id"}).AddRow(id, typeID))
	m.ExpectQuery(`SELECT .*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "usage_status", "assignee", "department"}).AddRow(uuid.New(), 2, "IN_USE", "张三", "研究部"))
	m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(sqlmock.NewRows([]string{"id", "category"}).AddRow(typeID, "PC"))
	m.ExpectRollback()
	_, err := ops.SaveProfile(context.Background(), Actor{}, id, model.AssetProfile{BaseModel: model.BaseModel{Version: 2}, UsageStatus: "IN_USE", Assignee: "李四", Department: "研究部"})
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("PC owner changed without record: %v", err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPCFirstIssueCreatesProfileAndRecordsConfirmingAdmin(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	id, typ, siteID, actorID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	m.ExpectBegin()
	m.ExpectQuery(`SELECT .*devices.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "type_id", "version", "lifecycle_status"}).AddRow(id, typ, 1, "WAITING_RACK"))
	m.ExpectQuery(`SELECT .*device_types`).WillReturnRows(sqlmock.NewRows([]string{"id", "category"}).AddRow(typ, "PC"))
	m.ExpectQuery(`SELECT count\(\*\).*rack_device_positions`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.ExpectQuery(`SELECT .*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery(`SELECT .*asset_sites.*FOR SHARE`).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "region"}).AddRow(siteID, "办公区", "上海"))
	m.ExpectExec(`INSERT INTO "asset_profiles"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`UPDATE .*devices`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*users`).WillReturnRows(sqlmock.NewRows([]string{"id", "display_name"}).AddRow(actorID, "资产管理员"))
	m.ExpectExec(`INSERT INTO "pc_handovers"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	out, err := ops.ConfirmPCHandover(context.Background(), Actor{UserID: &actorID}, id, PCHandoverInput{Version: 1, Operation: "ISSUE", Assignee: "李四", Department: "IT", SiteID: siteID, Condition: "NORMAL", Note: "新员工领用"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Before.UsageStatus != "IDLE" || out.After.Assignee != "李四" || out.ConfirmedByName != "资产管理员" || out.ConfirmedBy == nil || *out.ConfirmedBy != actorID || out.ID == uuid.Nil {
		t.Fatalf("bad first issue: %+v", out)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPCWithoutProfileAppearsIdleInOverview(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	id := uuid.New()
	m.ExpectQuery(`SELECT .*devices`).WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(id, "新PC"))
	m.ExpectQuery(`SELECT .*rack_device_positions`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery(`SELECT .*asset_profiles`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery(`SELECT .*asset_coverages`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectQuery(`SELECT .*spare_stocks`).WithArgs("INSTALLED").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	rows, err := ops.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Profile.UsageStatus != "IDLE" || rows[0].Profile.DeviceID != id {
		t.Fatalf("new PC cannot be issued: %+v", rows)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
