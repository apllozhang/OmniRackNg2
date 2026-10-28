package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"
)

func TestDeviceScopeFiltersBothPaginationCountAndRows(t *testing.T) {
	for _, scope := range []string{"PC", "ROOM"} {
		t.Run(scope, func(t *testing.T) {
			resource, m := newMockResourceStore(t)
			store := NewDeviceStore(resource.db)
			comparison := "="
			if scope == "ROOM" {
				comparison = "<>"
			}
			m.ExpectQuery(`SELECT count\(\*\).*devices.*type_id IN \(SELECT id FROM device_types WHERE category ` + comparison + ` .*deleted_at IS NULL\)`).WithArgs("PC").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			m.ExpectQuery(`SELECT .*devices.*type_id IN \(SELECT id FROM device_types WHERE category `+comparison+` .*deleted_at IS NULL\)`).WithArgs("PC", 20).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			_, total, err := store.ListDevices(context.Background(), DeviceQuery{Scope: scope, Page: 1, PageSize: 20})
			if err != nil || total != 0 {
				t.Fatalf("scope %s: %v", scope, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
