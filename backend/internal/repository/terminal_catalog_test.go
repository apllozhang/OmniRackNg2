package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"regexp"
	"testing"
	"time"
)

func TestTerminalCatalogSeedsSubtypesAndPreservesExistingCustomTypes(t *testing.T) {
	codes := []string{"TERMINAL-MONITOR", "TERMINAL-DESKTOP", "TERMINAL-PRINTER", "TERMINAL-LAPTOP", "TERMINAL-MOUSE", "TERMINAL-KEYBOARD", "TERMINAL-OTHER", "PC", "SERVER", "NETWORK", "STORAGE", "SECURITY", "POWER-ENV", "ACCESSORY"}
	for _, existing := range []bool{false, true} {
		name := "new installation"
		if existing {
			name = "existing custom catalog"
		}
		t.Run(name, func(t *testing.T) {
			store, m := newMockResourceStore(t)
			created := map[string]model.DeviceType{}
			store.db.Callback().Create().Before("gorm:create").Register("capture_terminal_types", func(tx *gorm.DB) {
				if x, ok := tx.Statement.Dest.(*model.DeviceType); ok {
					created[x.Code] = *x
				}
			})
			for _, code := range codes {
				rows := sqlmock.NewRows([]string{"id", "code", "name", "category", "version", "deleted_at"})
				if existing {
					var deletedAt interface{}
					if code == "TERMINAL-MOUSE" {
						deletedAt = time.Now()
					}
					rows.AddRow(uuid.New(), code, "用户自定义名称", "PC", 8, deletedAt)
				}
				m.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "device_types" WHERE code = $1 ORDER BY "device_types"."id" LIMIT $2`)).WithArgs(code, 1).WillReturnRows(rows)
				if !existing {
					m.ExpectExec(`INSERT INTO "device_types"`).WillReturnResult(sqlmock.NewResult(0, 1))
				}
			}
			// Renaming is constrained to the original default name and category; custom
			// names/IDs remain untouched on startup. Version changes invalidate stale forms.
			m.ExpectExec(`UPDATE "device_types" SET .*name.*version.*WHERE \(code = .* AND category = .* AND name = .*\)`).WithArgs("终端（待细分）", sqlmock.AnyArg(), "PC", "PC", "PC 终端").WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(`SELECT count\(\*\).*operation_policies`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			if err := SeedDeviceCatalog(context.Background(), store.db); err != nil {
				t.Fatal(err)
			}
			if existing {
				if len(created) != 0 {
					t.Fatal("existing catalog was recreated")
				}
			} else {
				expected := map[string]string{"TERMINAL-MONITOR": "显示器", "TERMINAL-DESKTOP": "主机", "TERMINAL-PRINTER": "打印机", "TERMINAL-LAPTOP": "笔记本", "TERMINAL-MOUSE": "鼠标", "TERMINAL-KEYBOARD": "键盘", "TERMINAL-OTHER": "其他终端", "PC": "终端（待细分）"}
				for code, label := range expected {
					typ, ok := created[code]
					if !ok || typ.Name != label || typ.Category != model.DeviceCategoryPC {
						t.Fatalf("missing or wrong terminal subtype %s: %+v", code, typ)
					}
				}
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
