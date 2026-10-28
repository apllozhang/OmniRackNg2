package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newMockResourceStore(t *testing.T) (*GormResourceStore, sqlmock.Sqlmock) {
	t.Helper()
	database, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	db, err := gorm.Open(postgres.New(postgres.Config{Conn: database}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open gorm with sql mock: %v", err)
	}
	return NewResourceStore(db), mock
}

func updateSQLPattern(table string) string {
	return `UPDATE "` + regexp.QuoteMeta(table) + `" SET .* WHERE \(id = \$[0-9]+ AND version = \$[0-9]+\) AND "` + regexp.QuoteMeta(table) + `"\."deleted_at" IS NULL`
}

func TestResourceRepositoryUpdatesUseOptimisticLock(t *testing.T) {
	tests := []struct {
		name            string
		table           string
		expectedVersion uint
		update          func(*GormResourceStore) (uint, error)
	}{
		{
			name: "data center", table: "data_centers", expectedVersion: 4,
			update: func(store *GormResourceStore) (uint, error) {
				item := &model.DataCenter{BaseModel: model.BaseModel{ID: uuid.New()}, Code: "DC-01", Name: "中心", Status: model.ResourceStatusOperating}
				err := store.UpdateDataCenter(context.Background(), item, 4)
				return item.Version, err
			},
		},
		{
			name: "room", table: "rooms", expectedVersion: 7,
			update: func(store *GormResourceStore) (uint, error) {
				item := &model.Room{BaseModel: model.BaseModel{ID: uuid.New()}, Code: "ROOM-01", Name: "机房", Status: model.ResourceStatusOperating, FloorPlanFormat: "SVG", RacksPerRow: 0}
				err := store.UpdateRoom(context.Background(), item, 7)
				return item.Version, err
			},
		},
		{
			name: "rack", table: "racks", expectedVersion: 2,
			update: func(store *GormResourceStore) (uint, error) {
				item := &model.Rack{BaseModel: model.BaseModel{ID: uuid.New()}, Code: "RACK-01", Name: "机柜", Type: "STANDARD", Status: model.RackStatusAvailable, UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000}
				err := store.UpdateRack(context.Background(), item, 2)
				return item.Version, err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name+" success increments version", func(t *testing.T) {
			store, mock := newMockResourceStore(t)
			mock.ExpectExec(updateSQLPattern(tt.table)).WillReturnResult(sqlmock.NewResult(0, 1))

			version, err := tt.update(store)
			if err != nil {
				t.Fatalf("update returned error: %v", err)
			}
			if want := tt.expectedVersion + 1; version != want {
				t.Fatalf("updated version = %d, want %d", version, want)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("SQL expectations were not met: %v", err)
			}
		})

		t.Run(tt.name+" zero rows maps to version conflict", func(t *testing.T) {
			store, mock := newMockResourceStore(t)
			mock.ExpectExec(updateSQLPattern(tt.table)).WillReturnResult(sqlmock.NewResult(0, 0))

			version, err := tt.update(store)
			if !errors.Is(err, ErrVersionConflict) {
				t.Fatalf("expected ErrVersionConflict, got %v", err)
			}
			if version != 0 {
				t.Fatalf("version changed after conflict: %d", version)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("SQL expectations were not met: %v", err)
			}
		})
	}
}
func TestResourceUpdateValuesKeepExplicitZeroAndEmptyValues(t *testing.T) {
	dataCenterValues := dataCenterUpdateValues(&model.DataCenter{})
	assertUpdateValue(t, dataCenterValues, "sort_order", 0)
	assertUpdateValue(t, dataCenterValues, "remarks", "")
	assertUpdateValue(t, dataCenterValues, "longitude", (*float64)(nil))

	roomValues := roomUpdateValues(&model.Room{RacksPerRow: 0})
	assertUpdateValue(t, roomValues, "racks_per_row", 0)
	assertUpdateValue(t, roomValues, "floor_plan_enabled", false)
	assertUpdateValue(t, roomValues, "used_power_kw", (*float64)(nil))

	rackValues := rackUpdateValues(&model.Rack{})
	assertUpdateValue(t, rackValues, "dual_power", false)
	assertUpdateValue(t, rackValues, "input_circuits", 0)
	assertUpdateValue(t, rackValues, "x_coordinate", (*float64)(nil))
}

func assertUpdateValue(t *testing.T, updates map[string]interface{}, key string, want interface{}) {
	t.Helper()
	got, ok := updates[key]
	if !ok {
		t.Fatalf("update values must include %q", key)
	}
	if got != want {
		t.Fatalf("update value %q = %#v, want %#v", key, got, want)
	}
}

func TestMoveRoomHierarchyUpdatesRoomChildrenAndPositions(t *testing.T) {
	store, mock := newMockResourceStore(t)
	roomID := uuid.New()
	targetDataCenterID := uuid.New()
	item := &model.Room{
		BaseModel:    model.BaseModel{ID: roomID},
		DataCenterID: targetDataCenterID,
		Code:         "ROOM-MOVED",
	}

	mock.ExpectExec(`UPDATE "rooms" SET .* WHERE \(id = \$[0-9]+ AND version = \$[0-9]+\).*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "racks" SET .*"data_center_id".*"version".* WHERE room_id = \$[0-9]+.*`).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec(`UPDATE "rack_device_positions" SET .*"data_center_id".* WHERE room_id = \$[0-9]+.*`).
		WillReturnResult(sqlmock.NewResult(0, 3))

	if err := store.MoveRoomHierarchy(context.Background(), item, 5); err != nil {
		t.Fatalf("move room hierarchy returned error: %v", err)
	}
	if item.Version != 6 {
		t.Fatalf("moved room version = %d, want 6", item.Version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations were not met: %v", err)
	}
}

func TestMoveRoomHierarchyZeroRowsMapsToVersionConflict(t *testing.T) {
	store, mock := newMockResourceStore(t)
	item := &model.Room{
		BaseModel:    model.BaseModel{ID: uuid.New()},
		DataCenterID: uuid.New(),
		Code:         "ROOM-MOVED",
	}

	mock.ExpectExec(`UPDATE "rooms" SET .* WHERE \(id = \$[0-9]+ AND version = \$[0-9]+\).*`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.MoveRoomHierarchy(context.Background(), item, 5)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
	if item.Version != 0 {
		t.Fatalf("version changed after conflict: %d", item.Version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations were not met: %v", err)
	}
}

func TestMoveRackHierarchyUpdatesRackAndPositions(t *testing.T) {
	store, mock := newMockResourceStore(t)
	x, y := 10.0, 20.0
	item := &model.Rack{
		BaseModel:    model.BaseModel{ID: uuid.New()},
		DataCenterID: uuid.New(),
		RoomID:       uuid.New(),
		Code:         "RACK-MOVED",
		XCoordinate:  &x,
		YCoordinate:  &y,
		SortOrder:    7,
	}

	mock.ExpectExec(`UPDATE "racks" SET .*"data_center_id".*"room_id".*"version".* WHERE \(id = \$[0-9]+ AND version = \$[0-9]+\).*`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "rack_device_positions" SET .*"data_center_id".*"room_id".* WHERE rack_id = \$[0-9]+.*`).
		WillReturnResult(sqlmock.NewResult(0, 4))

	if err := store.MoveRackHierarchy(context.Background(), item, 3); err != nil {
		t.Fatalf("move rack hierarchy returned error: %v", err)
	}
	if item.Version != 4 {
		t.Fatalf("moved rack version = %d, want 4", item.Version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations were not met: %v", err)
	}
}

func TestMoveRackHierarchyZeroRowsMapsToVersionConflict(t *testing.T) {
	store, mock := newMockResourceStore(t)
	item := &model.Rack{
		BaseModel:    model.BaseModel{ID: uuid.New()},
		DataCenterID: uuid.New(),
		RoomID:       uuid.New(),
		Code:         "RACK-MOVED",
	}

	mock.ExpectExec(`UPDATE "racks" SET .* WHERE \(id = \$[0-9]+ AND version = \$[0-9]+\).*`).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := store.MoveRackHierarchy(context.Background(), item, 3)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict, got %v", err)
	}
	if item.Version != 0 {
		t.Fatalf("version changed after conflict: %d", item.Version)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations were not met: %v", err)
	}
}
