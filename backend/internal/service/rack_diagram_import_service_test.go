package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

type rackDiagramImportFakeStore struct {
	repository.DeviceStore
	room        model.Room
	racks       []model.Rack
	deviceType  model.DeviceType
	positions   map[uuid.UUID][]model.RackDevicePosition
	serialItems []model.Device
	devices     map[uuid.UUID]model.Device
	created     []model.Device
	occupancies map[uuid.UUID][]model.RackUOccupancy
}

func newRackDiagramImportFakeStore(withDevice bool) *rackDiagramImportFakeStore {
	dcID, roomID, rackID, typeID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	store := &rackDiagramImportFakeStore{
		room:       model.Room{BaseModel: model.BaseModel{ID: roomID, Version: 1}, DataCenterID: dcID, Code: "ROOM-01", Name: "测试机房", Status: model.ResourceStatusOperating},
		racks:      []model.Rack{{BaseModel: model.BaseModel{ID: rackID, Version: 1}, DataCenterID: dcID, RoomID: roomID, Code: "RACK-01", Name: "01号机柜", Type: model.DefaultRackType, UHeight: 8, WidthMm: 600, DepthMm: 1200, HeightMm: 2000, Status: model.RackStatusAvailable}},
		deviceType: model.DeviceType{BaseModel: model.BaseModel{ID: typeID, Version: 1}, Code: "SERVER", Name: "服务器", Category: model.DeviceCategoryServer, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1},
		positions:  map[uuid.UUID][]model.RackDevicePosition{}, devices: map[uuid.UUID]model.Device{}, occupancies: map[uuid.UUID][]model.RackUOccupancy{},
	}
	if withDevice {
		power := 800.0
		device := model.Device{BaseModel: model.BaseModel{ID: uuid.New(), Version: 3}, TypeID: typeID, Type: store.deviceType, Code: "DEV-ORIGINAL", Name: "核心服务器", SerialNumber: "SN-001", RatedPowerW: &power, LifecycleStatus: model.DeviceStatusRunning, HeightU: 2}
		position := model.RackDevicePosition{BaseModel: model.BaseModel{ID: uuid.New(), Version: 2}, DeviceID: device.ID, Device: device, RackID: rackID, RoomID: roomID, DataCenterID: dcID, StartU: 2, EndU: 3, HeightU: 2, Orientation: model.DeviceOrientationNormal}
		store.devices[device.ID] = device
		store.positions[rackID] = []model.RackDevicePosition{position}
		store.occupancies[rackID] = occupancyRows(position)
		store.racks[0].Status = model.RackStatusPartial
	}
	return store
}

func (f *rackDiagramImportFakeStore) WithinDeviceTransaction(_ context.Context, fn func(repository.DeviceStore) error) error {
	return fn(f)
}
func (f *rackDiagramImportFakeStore) CountActivePowerConnections(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}
func (f *rackDiagramImportFakeStore) GetRoom(context.Context, uuid.UUID) (model.Room, error) {
	return f.room, nil
}
func (f *rackDiagramImportFakeStore) ListRacksByRoom(context.Context, uuid.UUID) ([]model.Rack, error) {
	return append([]model.Rack(nil), f.racks...), nil
}
func (f *rackDiagramImportFakeStore) GetDeviceType(context.Context, uuid.UUID) (model.DeviceType, error) {
	return f.deviceType, nil
}
func (f *rackDiagramImportFakeStore) ListDevicesBySerialNumbers(_ context.Context, values []string) ([]model.Device, error) {
	wanted := map[string]bool{}
	for _, value := range values {
		wanted[strings.ToLower(value)] = true
	}
	result := []model.Device{}
	for _, device := range f.serialItems {
		if wanted[strings.ToLower(device.SerialNumber)] {
			result = append(result, device)
		}
	}
	for _, device := range f.devices {
		if wanted[strings.ToLower(device.SerialNumber)] {
			result = append(result, device)
		}
	}
	return result, nil
}
func (f *rackDiagramImportFakeStore) ListPositionsByRack(_ context.Context, rackID uuid.UUID) ([]model.RackDevicePosition, error) {
	return append([]model.RackDevicePosition(nil), f.positions[rackID]...), nil
}
func (f *rackDiagramImportFakeStore) CountDevices(context.Context) (int64, error) {
	return int64(len(f.devices)), nil
}
func (f *rackDiagramImportFakeStore) CreateDevice(_ context.Context, item *model.Device) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.Version == 0 {
		item.Version = 1
	}
	f.devices[item.ID] = *item
	f.created = append(f.created, *item)
	return nil
}
func (f *rackDiagramImportFakeStore) CreateAuditLog(context.Context, *model.AuditLog) error {
	return nil
}
func (f *rackDiagramImportFakeStore) CreatePosition(_ context.Context, item *model.RackDevicePosition) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	if item.Version == 0 {
		item.Version = 1
	}
	item.Device = f.devices[item.DeviceID]
	f.positions[item.RackID] = append(f.positions[item.RackID], *item)
	return nil
}
func (f *rackDiagramImportFakeStore) CreateOccupancies(_ context.Context, items []model.RackUOccupancy) error {
	for _, item := range items {
		f.occupancies[item.RackID] = append(f.occupancies[item.RackID], item)
	}
	return nil
}
func (f *rackDiagramImportFakeStore) CreatePositionHistory(context.Context, *model.DevicePositionHistory) error {
	return nil
}
func (f *rackDiagramImportFakeStore) GetRackForUpdate(_ context.Context, rackID uuid.UUID) (model.Rack, error) {
	for _, rack := range f.racks {
		if rack.ID == rackID {
			return rack, nil
		}
	}
	return model.Rack{}, errors.New("rack not found")
}
func (f *rackDiagramImportFakeStore) CountPositionsByRack(_ context.Context, rackID uuid.UUID) (int64, error) {
	return int64(len(f.positions[rackID])), nil
}
func (f *rackDiagramImportFakeStore) ListOccupanciesByRack(_ context.Context, rackID uuid.UUID) ([]model.RackUOccupancy, error) {
	return append([]model.RackUOccupancy(nil), f.occupancies[rackID]...), nil
}
func (f *rackDiagramImportFakeStore) UpdateRackOperationalStatus(_ context.Context, rackID uuid.UUID, status string) error {
	for index := range f.racks {
		if f.racks[index].ID == rackID {
			f.racks[index].Status = status
		}
	}
	return nil
}

func rackDiagramValidateInput(store *rackDiagramImportFakeStore, devices []RackDiagramImportDeviceInput) RackDiagramImportValidateInput {
	return RackDiagramImportValidateInput{FormatVersion: "1", DataCenterID: store.room.DataCenterID, RoomID: store.room.ID, DefaultTypeID: store.deviceType.ID, CoveredRackIDs: []uuid.UUID{store.racks[0].ID}, Devices: devices}
}
func sourceImportItem(store *rackDiagramImportFakeStore, name string) RackDiagramImportDeviceInput {
	position := store.positions[store.racks[0].ID][0]
	power := 800.0
	deviceID, sourceRackID := position.DeviceID, position.RackID
	return RackDiagramImportDeviceInput{ClientID: "item-1", RackID: position.RackID, RackCode: store.racks[0].Code, StartU: position.StartU, EndU: position.EndU, Name: name, SerialNumber: position.Device.SerialNumber, RatedPowerW: &power, TypeID: &position.Device.TypeID, SourceDeviceID: &deviceID, SourceDeviceVersion: position.Device.Version, SourceDeviceCode: position.Device.Code, SourceRackID: &sourceRackID, SourceStartU: position.StartU, SourceEndU: position.EndU}
}

func TestRackDiagramImportRoundTripKeepsExistingDeviceCode(t *testing.T) {
	store := newRackDiagramImportFakeStore(true)
	service := NewDeviceService(store)
	validation, err := service.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{sourceImportItem(store, "核心服务器")}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if len(validation.Items) != 1 || validation.Items[0].Kind != RackDiagramImportUnchanged {
		t.Fatalf("unexpected validation: %+v", validation.Items)
	}
	if validation.Items[0].ExistingDeviceCode != "DEV-ORIGINAL" || validation.Items[0].RequiresDecision {
		t.Fatalf("existing code was not retained: %+v", validation.Items[0])
	}
}

func TestRackDiagramImportRenameRequiresExplicitIdentityDecision(t *testing.T) {
	store := newRackDiagramImportFakeStore(true)
	service := NewDeviceService(store)
	validation, err := service.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{sourceImportItem(store, "核心数据库服务器")}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	item := validation.Items[0]
	if item.Kind != RackDiagramImportNeedsDecision || !item.RequiresDecision {
		t.Fatalf("rename should require a decision: %+v", item)
	}
	if len(item.AllowedDecisions) != 1 || item.AllowedDecisions[0] != RackDiagramDecisionUpdateExisting {
		t.Fatalf("same SN must only allow updating existing: %+v", item.AllowedDecisions)
	}
	_, err = service.CommitRackDiagramImport(context.Background(), Actor{}, RackDiagramImportCommitInput{RoomID: store.room.ID, Token: validation.Token})
	if !errors.Is(err, ErrRackDiagramImportDecision) {
		t.Fatalf("commit without decision = %v, want ErrRackDiagramImportDecision", err)
	}
}

func TestRackDiagramImportDetectsDuplicateSerialAndMissingDeviceRemoval(t *testing.T) {
	store := newRackDiagramImportFakeStore(false)
	service := NewDeviceService(store)
	first := RackDiagramImportDeviceInput{ClientID: "a", RackID: store.racks[0].ID, RackCode: store.racks[0].Code, StartU: 1, EndU: 1, Name: "设备A", SerialNumber: "SN-DUP"}
	second := RackDiagramImportDeviceInput{ClientID: "b", RackID: store.racks[0].ID, RackCode: store.racks[0].Code, StartU: 2, EndU: 2, Name: "设备B", SerialNumber: "sn-dup"}
	validation, err := service.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{first, second}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if validation.Summary.Errors != 1 || validation.Items[1].Kind != RackDiagramImportError {
		t.Fatalf("duplicate SN was not blocked: %+v", validation)
	}

	occupied := newRackDiagramImportFakeStore(true)
	removal, err := NewDeviceService(occupied).ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(occupied, nil))
	if err != nil {
		t.Fatalf("removal validate returned error: %v", err)
	}
	if len(removal.Items) != 1 || removal.Items[0].Kind != RackDiagramImportRemoveMissing || !removal.Items[0].RequiresDecision {
		t.Fatalf("missing device should require removal confirmation: %+v", removal.Items)
	}
}

func TestRackDiagramImportCreatesNewDeviceWithAutomaticCode(t *testing.T) {
	store := newRackDiagramImportFakeStore(false)
	service := NewDeviceService(store)
	input := RackDiagramImportDeviceInput{ClientID: "new-1", RackID: store.racks[0].ID, RackCode: store.racks[0].Code, StartU: 4, EndU: 5, Name: "新导入服务器", SerialNumber: "SN-NEW"}
	validation, err := service.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{input}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if validation.Items[0].Kind != RackDiagramImportCreateNew {
		t.Fatalf("new device classification = %+v", validation.Items[0])
	}
	result, err := service.CommitRackDiagramImport(context.Background(), Actor{}, RackDiagramImportCommitInput{RoomID: store.room.ID, Token: validation.Token})
	if err != nil {
		t.Fatalf("commit returned error: %v", err)
	}
	if result.Created != 1 || len(store.created) != 1 {
		t.Fatalf("unexpected commit result: %+v, created=%d", result, len(store.created))
	}
	if !strings.HasPrefix(store.created[0].Code, "DEV-") || store.created[0].Code == "" {
		t.Fatalf("device code was not generated: %q", store.created[0].Code)
	}
	if len(store.positions[store.racks[0].ID]) != 1 || store.positions[store.racks[0].ID][0].StartU != 4 {
		t.Fatalf("new position not persisted: %+v", store.positions)
	}
}
func TestRackDiagramImportCommitRejectsDifferentRoomRoute(t *testing.T) {
	store := newRackDiagramImportFakeStore(false)
	service := NewDeviceService(store)
	input := RackDiagramImportDeviceInput{ClientID: "new-room-check", RackID: store.racks[0].ID, RackCode: store.racks[0].Code, StartU: 1, EndU: 1, Name: "路由校验设备"}
	validation, err := service.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{input}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	_, err = service.CommitRackDiagramImport(context.Background(), Actor{}, RackDiagramImportCommitInput{RoomID: uuid.New(), Token: validation.Token})
	if !errors.Is(err, ErrRackDiagramImportDraft) {
		t.Fatalf("commit with another room = %v, want ErrRackDiagramImportDraft", err)
	}
	if len(store.created) != 0 {
		t.Fatalf("different room route must not create devices: %+v", store.created)
	}
}
func TestRackDiagramImportRejectsOneSourceDeviceMappedToMultipleSlots(t *testing.T) {
	store := newRackDiagramImportFakeStore(true)
	service := NewDeviceService(store)
	first := sourceImportItem(store, "核心服务器")
	first.SerialNumber = ""
	second := first
	second.ClientID = "duplicate-source"
	second.StartU = 5
	second.EndU = 6
	validation, err := service.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{first, second}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if validation.Summary.Errors != 1 || validation.Items[1].Kind != RackDiagramImportError {
		t.Fatalf("duplicate source device was not blocked: %+v", validation)
	}
}

func TestRackDiagramDraftCanOnlyBeReservedOnce(t *testing.T) {
	store := newRackDiagramDraftStore()
	draft := rackDiagramDraft{Token: "single-use", ExpiresAt: time.Now().UTC().Add(time.Minute)}
	store.put(draft)
	if _, ok := store.take(draft.Token); !ok {
		t.Fatal("first draft reservation should succeed")
	}
	if _, ok := store.take(draft.Token); ok {
		t.Fatal("second draft reservation must fail")
	}
}

func TestRackDiagramImportDoesNotGuessExistingDeviceByNameOnly(t *testing.T) {
	store := newRackDiagramImportFakeStore(true)
	position := store.positions[store.racks[0].ID][0]
	device := store.devices[position.DeviceID]
	device.SerialNumber = ""
	device.RatedPowerW = nil
	store.devices[device.ID] = device
	position.Device = device
	store.positions[store.racks[0].ID] = []model.RackDevicePosition{position}

	input := RackDiagramImportDeviceInput{
		ClientID: "same-name-new-device", RackID: store.racks[0].ID, RackCode: store.racks[0].Code,
		StartU: 5, EndU: 5, Name: device.Name,
	}
	validation, err := NewDeviceService(store).ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{input}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if validation.Summary.Create != 1 || validation.Summary.Removals != 1 {
		t.Fatalf("name-only input must be treated as a new device plus explicit removal decision: %+v", validation)
	}
	if validation.Items[0].Kind != RackDiagramImportCreateNew || validation.Items[0].ExistingDeviceID != nil {
		t.Fatalf("name-only input was incorrectly matched to an existing device: %+v", validation.Items[0])
	}
}

func TestRackDiagramImportRejectsOverlongSerialDuringValidation(t *testing.T) {
	store := newRackDiagramImportFakeStore(false)
	input := RackDiagramImportDeviceInput{
		ClientID: "long-serial", RackID: store.racks[0].ID, RackCode: store.racks[0].Code,
		StartU: 1, EndU: 1, Name: "设备A", SerialNumber: strings.Repeat("S", 121),
	}
	validation, err := NewDeviceService(store).ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(store, []RackDiagramImportDeviceInput{input}))
	if err != nil {
		t.Fatalf("validate returned error: %v", err)
	}
	if validation.Summary.Errors != 1 || validation.Items[0].Kind != RackDiagramImportError {
		t.Fatalf("overlong serial must be blocked before commit: %+v", validation)
	}
}
