package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeResourceStore struct {
	tree                            []model.DataCenter
	createdRooms                    []model.Room
	createdRacks                    []model.Rack
	roomCount                       int64
	positionsInRack                 int64
	racksInRoom                     int64
	dataCenter                      model.DataCenter
	room                            model.Room
	rack                            model.Rack
	auditCount                      int
	auditLogs                       []model.AuditLog
	createRack                      model.Rack
	updatedDataCenter               model.DataCenter
	updateDataCenterExpectedVersion uint
	updateDataCenterCalled          bool
	updatedRoom                     model.Room
	updateRoomExpectedVersion       uint
	updateRoomCalled                bool
	updatedRack                     model.Rack
	updateRackExpectedVersion       uint
	updateRackCalled                bool
	updateError                     error
}

func (f *fakeResourceStore) WithinTransaction(_ context.Context, fn func(repository.ResourceStore) error) error {
	return fn(f)
}
func (f *fakeResourceStore) ListResourceTree(context.Context) ([]model.DataCenter, error) {
	return f.tree, nil
}
func (f *fakeResourceStore) GetDataCenter(context.Context, uuid.UUID) (model.DataCenter, error) {
	if f.dataCenter.ID == uuid.Nil {
		return model.DataCenter{}, gorm.ErrRecordNotFound
	}
	return f.dataCenter, nil
}
func (f *fakeResourceStore) CountRoomsByDataCenter(context.Context, uuid.UUID) (int64, error) {
	return f.roomCount, nil
}
func (f *fakeResourceStore) CreateDataCenter(_ context.Context, item *model.DataCenter) error {
	item.ID = uuid.New()
	item.Version = 1
	f.dataCenter = *item
	return nil
}
func (f *fakeResourceStore) UpdateDataCenter(_ context.Context, item *model.DataCenter, expectedVersion uint) error {
	f.updateDataCenterCalled = true
	f.updatedDataCenter = *item
	f.updateDataCenterExpectedVersion = expectedVersion
	if f.updateError == nil {
		item.Version = expectedVersion + 1
	}
	return f.updateError
}
func (f *fakeResourceStore) DeleteDataCenter(context.Context, uuid.UUID, uint) error {
	return f.updateError
}
func (f *fakeResourceStore) GetRoom(context.Context, uuid.UUID) (model.Room, error) {
	if f.room.ID == uuid.Nil {
		return model.Room{}, gorm.ErrRecordNotFound
	}
	return f.room, nil
}
func (f *fakeResourceStore) CountRacksByRoom(context.Context, uuid.UUID) (int64, error) {
	return f.racksInRoom, nil
}
func (f *fakeResourceStore) CreateRoom(_ context.Context, item *model.Room) error {
	item.ID = uuid.New()
	item.Version = 1
	f.room = *item
	f.createdRooms = append(f.createdRooms, *item)
	return nil
}
func (f *fakeResourceStore) UpdateRoom(_ context.Context, item *model.Room, expectedVersion uint) error {
	f.updateRoomCalled = true
	f.updatedRoom = *item
	f.updateRoomExpectedVersion = expectedVersion
	if f.updateError == nil {
		item.Version = expectedVersion + 1
	}
	return f.updateError
}
func (f *fakeResourceStore) MoveRoomHierarchy(_ context.Context, item *model.Room, expectedVersion uint) error {
	return f.UpdateRoom(context.Background(), item, expectedVersion)
}
func (f *fakeResourceStore) DeleteRoom(context.Context, uuid.UUID, uint) error { return f.updateError }
func (f *fakeResourceStore) GetRack(context.Context, uuid.UUID) (model.Rack, error) {
	if f.rack.ID == uuid.Nil {
		return model.Rack{}, gorm.ErrRecordNotFound
	}
	return f.rack, nil
}
func (f *fakeResourceStore) CountDevicePositionsByRack(context.Context, uuid.UUID) (int64, error) {
	return f.positionsInRack, nil
}
func (f *fakeResourceStore) CreateRack(_ context.Context, item *model.Rack) error {
	item.ID = uuid.New()
	item.Version = 1
	f.createRack = *item
	f.createdRacks = append(f.createdRacks, *item)
	return nil
}
func (f *fakeResourceStore) UpdateRack(_ context.Context, item *model.Rack, expectedVersion uint) error {
	f.updateRackCalled = true
	f.updatedRack = *item
	f.updateRackExpectedVersion = expectedVersion
	if f.updateError == nil {
		item.Version = expectedVersion + 1
	}
	return f.updateError
}
func (f *fakeResourceStore) MoveRackHierarchy(_ context.Context, item *model.Rack, expectedVersion uint) error {
	return f.UpdateRack(context.Background(), item, expectedVersion)
}
func (f *fakeResourceStore) DeleteRack(context.Context, uuid.UUID, uint) error { return f.updateError }
func (f *fakeResourceStore) CreateAuditLog(_ context.Context, item *model.AuditLog) error {
	f.auditCount++
	f.auditLogs = append(f.auditLogs, *item)
	return nil
}

type fakeRackTemplateReader struct {
	version model.RackTemplateVersion
	err     error
}

func (f *fakeRackTemplateReader) GetRackTemplateVersion(context.Context, uuid.UUID) (model.RackTemplateVersion, error) {
	return f.version, f.err
}

func TestCreateResourcesCanAutoGenerateCode(t *testing.T) {
	dataCenterStore := &fakeResourceStore{}
	dataCenter, err := NewResourceService(dataCenterStore).CreateDataCenter(context.Background(), Actor{}, DataCenterCreateInput{
		DataCenter: model.DataCenter{Name: "自动编号数据中心"}, AutoGenerateCode: true,
	})
	if err != nil {
		t.Fatalf("CreateDataCenter returned error: %v", err)
	}
	if !strings.HasPrefix(dataCenter.Code, "DC-") || len(dataCenter.Code) != 15 {
		t.Fatalf("unexpected generated data center code %q", dataCenter.Code)
	}

	dataCenterID := uuid.New()
	roomStore := &fakeResourceStore{dataCenter: model.DataCenter{
		BaseModel: model.BaseModel{ID: dataCenterID, Version: 1}, Status: model.ResourceStatusOperating,
	}}
	room, err := NewResourceService(roomStore).CreateRoom(context.Background(), Actor{}, dataCenterID, RoomWriteInput{
		Room: model.Room{Name: "自动编号机房"}, AutoGenerateCode: true,
	})
	if err != nil {
		t.Fatalf("CreateRoom returned error: %v", err)
	}
	if !strings.HasPrefix(room.Code, "ROOM-") || len(room.Code) != 17 {
		t.Fatalf("unexpected generated room code %q", room.Code)
	}

	roomID := uuid.New()
	rackStore := &fakeResourceStore{room: model.Room{
		BaseModel: model.BaseModel{ID: roomID, Version: 1}, DataCenterID: dataCenterID, Status: model.ResourceStatusOperating,
	}}
	rack, err := NewResourceService(rackStore).CreateRack(context.Background(), Actor{}, roomID, RackCreateInput{
		Rack: model.Rack{Name: "自动编号机柜"}, AutoGenerateCode: true,
	})
	if err != nil {
		t.Fatalf("CreateRack returned error: %v", err)
	}
	if !strings.HasPrefix(rack.Code, "RACK-") || len(rack.Code) != 17 {
		t.Fatalf("unexpected generated rack code %q", rack.Code)
	}
}

func TestCreateResourcesStillRequireManualCodeWhenAutoGenerationDisabled(t *testing.T) {
	_, err := NewResourceService(&fakeResourceStore{}).CreateDataCenter(context.Background(), Actor{}, DataCenterCreateInput{
		DataCenter: model.DataCenter{Name: "缺少编号的数据中心"},
	})
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
}

func TestCreateDataCenterDoesNotEnforceLegacyQuantityLimit(t *testing.T) {
	// The hierarchy no longer has an application-defined count ceiling.
	_, err := NewResourceService(&fakeResourceStore{}).CreateDataCenter(context.Background(), Actor{}, DataCenterCreateInput{
		DataCenter: model.DataCenter{Code: "dc-over-legacy-limit", Name: "超过历史阈值的数据中心"},
	})
	if err != nil {
		t.Fatalf("expected data center creation to remain available, got %v", err)
	}
}

func TestCreateRoomRejectsDisabledParent(t *testing.T) {
	parentID := uuid.New()
	store := &fakeResourceStore{dataCenter: model.DataCenter{BaseModel: model.BaseModel{ID: parentID, Version: 1}, Status: model.ResourceStatusDisabled}}
	svc := NewResourceService(store)
	_, err := svc.CreateRoom(context.Background(), Actor{}, parentID, RoomWriteInput{Room: model.Room{Code: "room-01", Name: "一号机房"}})
	if !errors.Is(err, ErrParentDisabled) {
		t.Fatalf("expected ErrParentDisabled, got %v", err)
	}
}

func TestCreateRoomRacksPerRowPresence(t *testing.T) {
	parentID := uuid.New()
	newStore := func() *fakeResourceStore {
		return &fakeResourceStore{dataCenter: model.DataCenter{
			BaseModel: model.BaseModel{ID: parentID, Version: 1},
			Status:    model.ResourceStatusOperating,
		}}
	}

	t.Run("omitted uses default eight", func(t *testing.T) {
		store := newStore()
		created, err := NewResourceService(store).CreateRoom(context.Background(), Actor{}, parentID, RoomWriteInput{
			Room: model.Room{Code: "room-default", Name: "默认布局机房"},
		})
		if err != nil {
			t.Fatalf("CreateRoom returned error: %v", err)
		}
		if created.RacksPerRow != 8 {
			t.Fatalf("racksPerRow = %d, want 8", created.RacksPerRow)
		}
	})

	for _, value := range []int{1, 20} {
		value := value
		t.Run(fmt.Sprintf("explicit %d is accepted", value), func(t *testing.T) {
			store := newStore()
			created, err := NewResourceService(store).CreateRoom(context.Background(), Actor{}, parentID, RoomWriteInput{
				Room:        model.Room{Code: fmt.Sprintf("room-%d", value), Name: "边界布局机房"},
				RacksPerRow: &value,
			})
			if err != nil {
				t.Fatalf("CreateRoom returned error: %v", err)
			}
			if created.RacksPerRow != value {
				t.Fatalf("racksPerRow = %d, want %d", created.RacksPerRow, value)
			}
		})
	}

	for _, value := range []int{0, 21} {
		value := value
		t.Run(fmt.Sprintf("explicit %d is rejected", value), func(t *testing.T) {
			_, err := NewResourceService(newStore()).CreateRoom(context.Background(), Actor{}, parentID, RoomWriteInput{
				Room:        model.Room{Code: fmt.Sprintf("room-invalid-%d", value), Name: "非法布局机房"},
				RacksPerRow: &value,
			})
			if !errors.Is(err, ErrInvalidResource) {
				t.Fatalf("expected ErrInvalidResource, got %v", err)
			}
		})
	}
}

func TestUpdateRoomRacksPerRowPresence(t *testing.T) {
	roomID := uuid.New()
	dataCenterID := uuid.New()
	newStore := func() *fakeResourceStore {
		return &fakeResourceStore{room: model.Room{
			BaseModel:       model.BaseModel{ID: roomID, Version: 3},
			DataCenterID:    dataCenterID,
			Code:            "ROOM-01",
			Name:            "一号机房",
			Status:          model.ResourceStatusOperating,
			FloorPlanFormat: "SVG",
			RacksPerRow:     12,
		}}
	}

	t.Run("omitted preserves persisted value", func(t *testing.T) {
		store := newStore()
		updated, err := NewResourceService(store).UpdateRoom(context.Background(), Actor{}, roomID, 3, RoomWriteInput{
			Room: model.Room{BaseModel: model.BaseModel{Version: 3}, Code: "ROOM-01", Name: "一号机房"},
		})
		if err != nil {
			t.Fatalf("UpdateRoom returned error: %v", err)
		}
		if updated.RacksPerRow != 12 || store.updatedRoom.RacksPerRow != 12 {
			t.Fatalf("racksPerRow was not preserved: updated=%d stored=%d", updated.RacksPerRow, store.updatedRoom.RacksPerRow)
		}
		if store.updateRoomExpectedVersion != 3 || updated.Version != 4 {
			t.Fatalf("unexpected optimistic-lock versions: expected=%d updated=%d", store.updateRoomExpectedVersion, updated.Version)
		}
		if store.auditCount != 1 || len(store.auditLogs) != 1 || store.auditLogs[0].ResourceType != "room" || store.auditLogs[0].Action != "UPDATE" {
			t.Fatalf("expected one room update audit, got %+v", store.auditLogs)
		}
	})

	t.Run("repository conflict is mapped and not audited", func(t *testing.T) {
		store := newStore()
		store.updateError = repository.ErrVersionConflict
		_, err := NewResourceService(store).UpdateRoom(context.Background(), Actor{}, roomID, 3, RoomWriteInput{
			Room: model.Room{BaseModel: model.BaseModel{Version: 3}, Code: "ROOM-01", Name: "一号机房"},
		})
		if !errors.Is(err, ErrResourceVersion) {
			t.Fatalf("expected ErrResourceVersion, got %v", err)
		}
		if store.auditCount != 0 {
			t.Fatalf("conflicted update must not write audit log, got %d", store.auditCount)
		}
	})

	t.Run("zero version is rejected before update", func(t *testing.T) {
		store := newStore()
		_, err := NewResourceService(store).UpdateRoom(context.Background(), Actor{}, roomID, 0, RoomWriteInput{
			Room: model.Room{Code: "ROOM-01", Name: "一号机房"},
		})
		if !errors.Is(err, ErrInvalidResource) {
			t.Fatalf("expected ErrInvalidResource, got %v", err)
		}
		if store.updateRoomCalled {
			t.Fatal("repository update must not run for version=0")
		}
	})

	t.Run("explicit zero is rejected before update", func(t *testing.T) {
		store := newStore()
		zero := 0
		_, err := NewResourceService(store).UpdateRoom(context.Background(), Actor{}, roomID, 3, RoomWriteInput{
			Room:        model.Room{BaseModel: model.BaseModel{Version: 3}, Code: "ROOM-01", Name: "一号机房"},
			RacksPerRow: &zero,
		})
		if !errors.Is(err, ErrInvalidResource) {
			t.Fatalf("expected ErrInvalidResource, got %v", err)
		}
		if store.updateRoomCalled {
			t.Fatal("repository update must not run for racksPerRow=0")
		}
	})
}

func TestCreateRackAppliesDefaultsAndParent(t *testing.T) {
	dataCenterID := uuid.New()
	roomID := uuid.New()
	store := &fakeResourceStore{room: model.Room{BaseModel: model.BaseModel{ID: roomID, Version: 1}, DataCenterID: dataCenterID, Status: model.ResourceStatusOperating}}
	svc := NewResourceService(store)
	created, err := svc.CreateRack(context.Background(), Actor{RequestID: "req-test"}, roomID, RackCreateInput{Rack: model.Rack{Code: "rack-01", Name: "一号机柜"}})
	if err != nil {
		t.Fatalf("CreateRack returned error: %v", err)
	}
	if created.UHeight != 42 || created.WidthMm != 600 || created.DepthMm != 1200 || created.HeightMm != 2000 {
		t.Fatalf("unexpected rack defaults: %+v", created)
	}
	if store.createRack.DataCenterID != dataCenterID || store.createRack.RoomID != roomID {
		t.Fatalf("rack hierarchy was not assigned")
	}
	if store.auditCount != 1 {
		t.Fatalf("expected one audit log, got %d", store.auditCount)
	}
}

func TestCreateRackFromTemplateCopiesImmutableSnapshot(t *testing.T) {
	dataCenterID := uuid.New()
	roomID := uuid.New()
	templateID := uuid.New()
	versionID := uuid.New()
	ratedVoltage := 220.0
	store := &fakeResourceStore{room: model.Room{
		BaseModel:    model.BaseModel{ID: roomID, Version: 1},
		DataCenterID: dataCenterID,
		Status:       model.ResourceStatusOperating,
	}}
	reader := &fakeRackTemplateReader{version: model.RackTemplateVersion{
		BaseModel:     model.BaseModel{ID: versionID, Version: 1},
		TemplateID:    templateID,
		Revision:      2,
		Type:          "NETWORK",
		Manufacturer:  "Example Vendor",
		ModelNumber:   "RACK-45U",
		UHeight:       45,
		WidthMm:       800,
		DepthMm:       1200,
		HeightMm:      2200,
		DualPower:     true,
		InputCircuits: 2,
		RatedVoltage:  &ratedVoltage,
		PDUCount:      4,
		Template: model.RackTemplate{
			BaseModel: model.BaseModel{ID: templateID, Version: 2},
			Code:      "NETWORK-45U",
			Name:      "网络机柜 45U",
			Status:    model.RackTemplateStatusActive,
		},
	}}
	svc := NewResourceService(store, reader)
	created, err := svc.CreateRack(context.Background(), Actor{}, roomID, RackCreateInput{Rack: model.Rack{
		Code: "rack-template-01", Name: "模板机柜", TemplateVersionID: &versionID,
		UHeight: 99, WidthMm: 999, DepthMm: 999, HeightMm: 999,
	}})
	if err != nil {
		t.Fatalf("CreateRack returned error: %v", err)
	}
	if created.TemplateID == nil || *created.TemplateID != templateID || created.TemplateVersionID == nil || *created.TemplateVersionID != versionID {
		t.Fatalf("template source was not copied: %+v", created)
	}
	if created.TemplateCode != "NETWORK-45U" || created.TemplateName != "网络机柜 45U" || created.TemplateRevision != 2 {
		t.Fatalf("template identity was not copied: %+v", created)
	}
	if created.UHeight != 45 || created.WidthMm != 800 || created.DepthMm != 1200 || created.HeightMm != 2200 || created.PDUCount != 4 {
		t.Fatalf("template specification did not override request values: %+v", created)
	}
	if created.TemplateSnapshot.VersionID != versionID || created.TemplateSnapshot.Revision != 2 || created.TemplateSnapshot.UHeight != 45 || created.TemplateSnapshot.RatedVoltage == nil || *created.TemplateSnapshot.RatedVoltage != ratedVoltage {
		t.Fatalf("template snapshot was not persisted correctly: %+v", created.TemplateSnapshot)
	}
}

func TestCreateRackFromTemplateAllowsInstanceSpecOverride(t *testing.T) {
	dataCenterID := uuid.New()
	roomID := uuid.New()
	templateID := uuid.New()
	versionID := uuid.New()
	templateVoltage := 220.0
	requestedVoltage := 110.0
	store := &fakeResourceStore{room: model.Room{
		BaseModel: model.BaseModel{ID: roomID, Version: 1}, DataCenterID: dataCenterID, Status: model.ResourceStatusOperating,
	}}
	reader := &fakeRackTemplateReader{version: model.RackTemplateVersion{
		BaseModel: model.BaseModel{ID: versionID, Version: 1}, TemplateID: templateID, Revision: 3,
		Type: "NETWORK", UHeight: 45, WidthMm: 800, DepthMm: 1200, HeightMm: 2200,
		DualPower: true, InputCircuits: 2, RatedVoltage: &templateVoltage, PDUCount: 4,
		Template: model.RackTemplate{BaseModel: model.BaseModel{ID: templateID}, Code: "NETWORK-45U", Name: "网络模板", Status: model.RackTemplateStatusActive},
	}}
	svc := NewResourceService(store, reader)
	created, err := svc.CreateRack(context.Background(), Actor{}, roomID, RackCreateInput{
		OverrideTemplateSpec: true,
		Rack: model.Rack{
			Code: "rack-custom-size", Name: "实例覆盖机柜", TemplateVersionID: &versionID,
			Type: "CUSTOM", UHeight: 48, WidthMm: 900, DepthMm: 1300, HeightMm: 2300,
			DualPower: false, InputCircuits: 0, RatedVoltage: &requestedVoltage, PDUCount: 0,
		},
	})
	if err != nil {
		t.Fatalf("CreateRack returned error: %v", err)
	}
	if created.Type != "CUSTOM" || created.UHeight != 48 || created.WidthMm != 900 || created.DepthMm != 1300 || created.HeightMm != 2300 {
		t.Fatalf("instance dimensions were overwritten by template: %+v", created)
	}
	if created.DualPower || created.InputCircuits != 0 || created.PDUCount != 0 || created.RatedVoltage == nil || *created.RatedVoltage != requestedVoltage {
		t.Fatalf("instance power overrides were not preserved: %+v", created)
	}
	if created.TemplateSnapshot.WidthMm != 800 || created.TemplateSnapshot.HeightMm != 2200 || !created.TemplateSnapshot.DualPower || created.TemplateSnapshot.PDUCount != 4 {
		t.Fatalf("template snapshot must remain immutable template data: %+v", created.TemplateSnapshot)
	}
}

func TestDeleteDataCenterRejectsChildren(t *testing.T) {
	id := uuid.New()
	store := &fakeResourceStore{dataCenter: model.DataCenter{BaseModel: model.BaseModel{ID: id, Version: 1}, Code: "DC01", Name: "中心", Status: model.ResourceStatusOperating}, roomCount: 1}
	svc := NewResourceService(store)
	err := svc.DeleteDataCenter(context.Background(), Actor{}, id, 1)
	if !errors.Is(err, ErrHasChildren) {
		t.Fatalf("expected ErrHasChildren, got %v", err)
	}
}

func TestValidateRackRejectsInvalidRotation(t *testing.T) {
	item := model.Rack{Code: "R01", Name: "机柜", Status: model.RackStatusAvailable, UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000, Rotation: 45}
	if err := validateRack(item); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("expected invalid resource error, got %v", err)
	}
}

func TestUpdateDataCenterPreservesIdentityNormalizesAndAudits(t *testing.T) {
	id := uuid.New()
	createdAt := time.Date(2026, time.August, 20, 9, 30, 0, 0, time.UTC)
	actorID := uuid.New()
	store := &fakeResourceStore{dataCenter: model.DataCenter{
		BaseModel: model.BaseModel{ID: id, Version: 6, CreatedAt: createdAt},
		Code:      "DC-OLD",
		Name:      "旧中心",
		Status:    model.ResourceStatusOperating,
	}}

	updated, err := NewResourceService(store).UpdateDataCenter(context.Background(), Actor{
		UserID: &actorID, RequestID: "req-update-dc", Source: "api",
	}, id, 6, model.DataCenter{
		BaseModel: model.BaseModel{ID: uuid.New(), Version: 999},
		Code:      " dc-new ",
		Name:      " 新中心 ",
		Status:    " maintenance ",
		Manager:   " 张三 ",
	})
	if err != nil {
		t.Fatalf("UpdateDataCenter returned error: %v", err)
	}
	if !store.updateDataCenterCalled || store.updateDataCenterExpectedVersion != 6 {
		t.Fatalf("repository update call was not captured correctly: called=%v version=%d", store.updateDataCenterCalled, store.updateDataCenterExpectedVersion)
	}
	if updated.ID != id || updated.CreatedAt != createdAt || updated.Version != 7 {
		t.Fatalf("identity/version was not preserved and incremented: %+v", updated.BaseModel)
	}
	if updated.Code != "DC-NEW" || updated.Name != "新中心" || updated.Status != model.ResourceStatusMaintenance || updated.Manager != "张三" {
		t.Fatalf("data center was not normalized: %+v", updated)
	}
	if store.updatedDataCenter.ID != id || store.updatedDataCenter.Version != 6 {
		t.Fatalf("repository received wrong identity/version snapshot: %+v", store.updatedDataCenter.BaseModel)
	}
	if store.auditCount != 1 || len(store.auditLogs) != 1 {
		t.Fatalf("expected one audit log, got count=%d logs=%d", store.auditCount, len(store.auditLogs))
	}
	audit := store.auditLogs[0]
	if audit.Action != "UPDATE" || audit.ResourceType != "data_center" || audit.ResourceID == nil || *audit.ResourceID != id || audit.RequestID != "req-update-dc" || audit.Source != "api" {
		t.Fatalf("unexpected audit metadata: %+v", audit)
	}
	if !strings.Contains(audit.BeforeJSON, `"code":"DC-OLD"`) || !strings.Contains(audit.AfterJSON, `"code":"DC-NEW"`) || !strings.Contains(audit.AfterJSON, `"version":7`) {
		t.Fatalf("unexpected audit snapshots: before=%s after=%s", audit.BeforeJSON, audit.AfterJSON)
	}
}

func TestUpdateDataCenterMapsVersionConflict(t *testing.T) {
	id := uuid.New()
	store := &fakeResourceStore{
		dataCenter:  model.DataCenter{BaseModel: model.BaseModel{ID: id, Version: 3}, Code: "DC-01", Name: "中心", Status: model.ResourceStatusOperating},
		updateError: repository.ErrVersionConflict,
	}
	_, err := NewResourceService(store).UpdateDataCenter(context.Background(), Actor{}, id, 3, model.DataCenter{Code: "DC-01", Name: "中心", Status: model.ResourceStatusOperating})
	if !errors.Is(err, ErrResourceVersion) {
		t.Fatalf("expected ErrResourceVersion, got %v", err)
	}
	if store.auditCount != 0 {
		t.Fatalf("conflicted update must not write audit log, got %d", store.auditCount)
	}
}

func TestUpdateDataCenterRejectsZeroVersionBeforeRepository(t *testing.T) {
	store := &fakeResourceStore{}
	_, err := NewResourceService(store).UpdateDataCenter(context.Background(), Actor{}, uuid.New(), 0, model.DataCenter{Code: "DC-01", Name: "中心"})
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
	if store.updateDataCenterCalled {
		t.Fatal("repository update must not run for version=0")
	}
}

func TestUpdateRackPreservesHierarchyTemplateAndAudits(t *testing.T) {
	id := uuid.New()
	dataCenterID := uuid.New()
	roomID := uuid.New()
	templateID := uuid.New()
	templateVersionID := uuid.New()
	createdAt := time.Date(2026, time.August, 21, 10, 0, 0, 0, time.UTC)
	current := model.Rack{
		BaseModel:         model.BaseModel{ID: id, Version: 8, CreatedAt: createdAt},
		DataCenterID:      dataCenterID,
		RoomID:            roomID,
		TemplateID:        &templateID,
		TemplateVersionID: &templateVersionID,
		TemplateCode:      "TPL-42U",
		TemplateName:      "标准模板",
		TemplateRevision:  3,
		TemplateSnapshot: model.RackTemplateSnapshot{
			TemplateID: templateID, VersionID: templateVersionID, Code: "TPL-42U", Name: "标准模板", Revision: 3, UHeight: 42,
		},
		Code: "RACK-OLD", Name: "旧机柜", Type: "STANDARD", Status: model.RackStatusAvailable,
		UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000,
	}
	store := &fakeResourceStore{rack: current}

	updated, err := NewResourceService(store).UpdateRack(context.Background(), Actor{RequestID: "req-update-rack"}, id, 8, model.Rack{
		BaseModel:    model.BaseModel{ID: uuid.New(), Version: 100},
		Code:         " rack-new ",
		Name:         " 新机柜 ",
		Type:         " network ",
		Status:       " maintenance ",
		UHeight:      45,
		WidthMm:      800,
		DepthMm:      1200,
		HeightMm:     2200,
		DataCenterID: uuid.New(),
		RoomID:       uuid.New(),
	})
	if err != nil {
		t.Fatalf("UpdateRack returned error: %v", err)
	}
	if !store.updateRackCalled || store.updateRackExpectedVersion != 8 {
		t.Fatalf("repository update call was not captured correctly: called=%v version=%d", store.updateRackCalled, store.updateRackExpectedVersion)
	}
	if updated.ID != id || updated.DataCenterID != dataCenterID || updated.RoomID != roomID || updated.CreatedAt != createdAt || updated.Version != 9 {
		t.Fatalf("rack identity/hierarchy/version was not preserved: %+v", updated)
	}
	if updated.TemplateID == nil || *updated.TemplateID != templateID || updated.TemplateVersionID == nil || *updated.TemplateVersionID != templateVersionID || updated.TemplateCode != current.TemplateCode || updated.TemplateSnapshot.VersionID != templateVersionID {
		t.Fatalf("rack template source was not preserved: %+v", updated)
	}
	if updated.Code != "RACK-NEW" || updated.Name != "新机柜" || updated.Type != "NETWORK" || updated.Status != model.RackStatusMaintenance {
		t.Fatalf("rack was not normalized: %+v", updated)
	}
	if store.updatedRack.Version != 8 || store.updatedRack.DataCenterID != dataCenterID || store.updatedRack.RoomID != roomID {
		t.Fatalf("repository received wrong preserved values: %+v", store.updatedRack)
	}
	if store.auditCount != 1 || len(store.auditLogs) != 1 || !strings.Contains(store.auditLogs[0].AfterJSON, `"version":9`) {
		t.Fatalf("expected one successful update audit, got %+v", store.auditLogs)
	}
}

func TestUpdateRackMapsVersionConflict(t *testing.T) {
	id := uuid.New()
	store := &fakeResourceStore{
		rack:        model.Rack{BaseModel: model.BaseModel{ID: id, Version: 2}, Code: "RACK-01", Name: "机柜", Type: "STANDARD", Status: model.RackStatusAvailable, UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000},
		updateError: repository.ErrVersionConflict,
	}
	_, err := NewResourceService(store).UpdateRack(context.Background(), Actor{}, id, 2, model.Rack{Code: "RACK-01", Name: "机柜", Type: "STANDARD", Status: model.RackStatusAvailable, UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000})
	if !errors.Is(err, ErrResourceVersion) {
		t.Fatalf("expected ErrResourceVersion, got %v", err)
	}
	if store.auditCount != 0 {
		t.Fatalf("conflicted update must not write audit log, got %d", store.auditCount)
	}
}

func TestUpdateRackRejectsZeroVersionBeforeRepository(t *testing.T) {
	store := &fakeResourceStore{}
	_, err := NewResourceService(store).UpdateRack(context.Background(), Actor{}, uuid.New(), 0, model.Rack{Code: "RACK-01", Name: "机柜"})
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
	if store.updateRackCalled {
		t.Fatal("repository update must not run for version=0")
	}
}

func TestCopyDataCenterCopiesStructureWithoutAssets(t *testing.T) {
	dcID, roomID, rackID := uuid.New(), uuid.New(), uuid.New()
	store := &fakeResourceStore{tree: []model.DataCenter{{
		BaseModel: model.BaseModel{ID: dcID, Version: 3}, Code: "DC-OLD", Name: "主中心", Status: model.ResourceStatusOperating,
		Rooms: []model.Room{{BaseModel: model.BaseModel{ID: roomID, Version: 2}, DataCenterID: dcID, Code: "ROOM-OLD", Name: "一号机房", Status: model.ResourceStatusOperating, FloorPlanFormat: "SVG", RacksPerRow: 8,
			Racks: []model.Rack{{BaseModel: model.BaseModel{ID: rackID, Version: 4}, DataCenterID: dcID, RoomID: roomID, Code: "RACK-OLD", Name: "A01", Status: model.RackStatusAvailable, Type: "STANDARD", UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000, SerialNumber: "SN-1", AssetNumber: "ASSET-1"}},
		}},
	}}}
	result, err := NewResourceService(store).CopyDataCenter(context.Background(), Actor{RequestID: "copy-dc"}, dcID, ResourceCopyInput{Version: 3, Name: "灾备中心", AutoGenerateCode: true})
	if err != nil {
		t.Fatalf("CopyDataCenter returned error: %v", err)
	}
	if result.CopiedRooms != 1 || result.CopiedRacks != 1 {
		t.Fatalf("unexpected copy counts: %+v", result)
	}
	if len(store.createdRooms) != 1 || len(store.createdRacks) != 1 {
		t.Fatalf("expected copied children, rooms=%d racks=%d", len(store.createdRooms), len(store.createdRacks))
	}
	copiedRack := store.createdRacks[0]
	if copiedRack.ID == rackID || copiedRack.RoomID != store.createdRooms[0].ID || copiedRack.SerialNumber != "" || copiedRack.AssetNumber != "" {
		t.Fatalf("copied rack retained identity/asset fields: %+v", copiedRack)
	}
	if copiedRack.Status != model.RackStatusPlanning {
		t.Fatalf("copied rack status=%s", copiedRack.Status)
	}
}

func TestMoveRoomUpdatesTargetHierarchy(t *testing.T) {
	roomID, sourceDCID, targetDCID := uuid.New(), uuid.New(), uuid.New()
	store := &fakeResourceStore{
		room:       model.Room{BaseModel: model.BaseModel{ID: roomID, Version: 7}, DataCenterID: sourceDCID, Code: "ROOM-01", Name: "生产机房", Status: model.ResourceStatusOperating, FloorPlanFormat: "SVG", RacksPerRow: 8},
		dataCenter: model.DataCenter{BaseModel: model.BaseModel{ID: targetDCID, Version: 1}, Code: "DC-02", Name: "目标中心", Status: model.ResourceStatusOperating},
	}
	moved, err := NewResourceService(store).MoveRoom(context.Background(), Actor{RequestID: "move-room"}, roomID, ResourceMoveInput{Version: 7, TargetDataCenterID: targetDCID, AutoGenerateCode: true})
	if err != nil {
		t.Fatalf("MoveRoom returned error: %v", err)
	}
	if moved.DataCenterID != targetDCID || !strings.HasPrefix(moved.Code, "ROOM-") || moved.Version != 8 {
		t.Fatalf("unexpected moved room: %+v", moved)
	}
	if !store.updateRoomCalled || store.updatedRoom.DataCenterID != targetDCID {
		t.Fatalf("hierarchy move was not persisted: %+v", store.updatedRoom)
	}
}

func TestMoveRackPreservesIdentityAndClearsScreenCoordinates(t *testing.T) {
	rackID, sourceRoomID, targetRoomID, targetDCID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	x, y := 100.0, 200.0
	store := &fakeResourceStore{
		rack: model.Rack{BaseModel: model.BaseModel{ID: rackID, Version: 5}, DataCenterID: uuid.New(), RoomID: sourceRoomID, Code: "RACK-01", Name: "A01", Status: model.RackStatusPartial, Type: "STANDARD", UHeight: 42, WidthMm: 600, DepthMm: 1200, HeightMm: 2000, XCoordinate: &x, YCoordinate: &y},
		room: model.Room{BaseModel: model.BaseModel{ID: targetRoomID, Version: 1}, DataCenterID: targetDCID, Code: "ROOM-02", Name: "目标机房", Status: model.ResourceStatusOperating, FloorPlanFormat: "SVG", RacksPerRow: 8},
	}
	moved, err := NewResourceService(store).MoveRack(context.Background(), Actor{RequestID: "move-rack"}, rackID, ResourceMoveInput{Version: 5, TargetRoomID: targetRoomID, Code: "RACK-NEW"})
	if err != nil {
		t.Fatalf("MoveRack returned error: %v", err)
	}
	if moved.ID != rackID || moved.RoomID != targetRoomID || moved.DataCenterID != targetDCID || moved.XCoordinate != nil || moved.YCoordinate != nil {
		t.Fatalf("unexpected moved rack: %+v", moved)
	}
	if moved.Status != model.RackStatusPartial || moved.Code != "RACK-NEW" || moved.Version != 6 {
		t.Fatalf("move changed business fields incorrectly: %+v", moved)
	}
}

func TestCopyAndMoveRejectStaleVersion(t *testing.T) {
	rackID := uuid.New()
	store := &fakeResourceStore{rack: model.Rack{BaseModel: model.BaseModel{ID: rackID, Version: 4}, Code: "R", Name: "R"}, room: model.Room{BaseModel: model.BaseModel{ID: uuid.New(), Version: 1}, DataCenterID: uuid.New(), Status: model.ResourceStatusOperating}}
	_, copyErr := NewResourceService(store).CopyRack(context.Background(), Actor{}, rackID, ResourceCopyInput{Version: 3, TargetRoomID: store.room.ID, AutoGenerateCode: true, Name: "副本"})
	if !errors.Is(copyErr, ErrResourceVersion) {
		t.Fatalf("expected copy version conflict, got %v", copyErr)
	}
	_, moveErr := NewResourceService(store).MoveRack(context.Background(), Actor{}, rackID, ResourceMoveInput{Version: 3, TargetRoomID: store.room.ID})
	if !errors.Is(moveErr, ErrResourceVersion) {
		t.Fatalf("expected move version conflict, got %v", moveErr)
	}
}
