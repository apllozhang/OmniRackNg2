package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
)

type autoCodeDeviceStore struct {
	repository.DeviceStore
	deviceType model.DeviceType
	created    model.Device
}

func (f *autoCodeDeviceStore) GetDeviceType(context.Context, uuid.UUID) (model.DeviceType, error) {
	return f.deviceType, nil
}
func (f *autoCodeDeviceStore) WithinDeviceTransaction(_ context.Context, fn func(repository.DeviceStore) error) error {
	return fn(f)
}
func (f *autoCodeDeviceStore) CountDevices(context.Context) (int64, error) { return 0, nil }
func (f *autoCodeDeviceStore) CreateDevice(_ context.Context, item *model.Device) error {
	item.ID = uuid.New()
	item.Version = 1
	f.created = *item
	return nil
}
func (f *autoCodeDeviceStore) CreateAuditLog(context.Context, *model.AuditLog) error { return nil }

func TestCreateDeviceCanAutoGenerateCode(t *testing.T) {
	typeID := uuid.New()
	store := &autoCodeDeviceStore{deviceType: model.DeviceType{
		BaseModel: model.BaseModel{ID: typeID, Version: 1},
		Status:    model.DeviceTypeStatusActive, DefaultHeightU: 2,
	}}
	created, err := NewDeviceService(store).CreateDevice(context.Background(), Actor{}, DeviceCreateInput{
		Device: model.Device{TypeID: typeID, Name: "自动编号设备"}, AutoGenerateCode: true,
	})
	if err != nil {
		t.Fatalf("CreateDevice returned error: %v", err)
	}
	if !strings.HasPrefix(created.Code, "DEV-") || len(created.Code) != 16 {
		t.Fatalf("unexpected generated device code %q", created.Code)
	}
	if store.created.Code != created.Code || created.HeightU != 2 {
		t.Fatalf("generated device was not persisted with defaults: %+v", created)
	}
}

func TestCreateDeviceStillRequiresManualCodeWhenAutoGenerationDisabled(t *testing.T) {
	typeID := uuid.New()
	store := &autoCodeDeviceStore{deviceType: model.DeviceType{
		BaseModel: model.BaseModel{ID: typeID, Version: 1},
		Status:    model.DeviceTypeStatusActive, DefaultHeightU: 1,
	}}
	_, err := NewDeviceService(store).CreateDevice(context.Background(), Actor{}, DeviceCreateInput{
		Device: model.Device{TypeID: typeID, Name: "缺少编号设备"},
	})
	if !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("expected ErrInvalidResource, got %v", err)
	}
}

func TestAvailableRanges(t *testing.T) {
	occupancies := []model.RackUOccupancy{
		{StartU: 2, EndU: 3},
		{StartU: 6, EndU: 6},
		{StartU: 4, EndU: 5},
		{StartU: -2, EndU: 0},
		{StartU: 10, EndU: 12},
	}

	got := availableRanges(8, occupancies)
	want := []RackFreeRange{{StartU: 1, EndU: 1, HeightU: 1}, {StartU: 7, EndU: 8, HeightU: 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("availableRanges() = %#v, want %#v", got, want)
	}
}

func TestAvailableRangesWholeRackBoundaries(t *testing.T) {
	if got := availableRanges(1, nil); !reflect.DeepEqual(got, []RackFreeRange{{StartU: 1, EndU: 1, HeightU: 1}}) {
		t.Fatalf("1U empty rack ranges = %#v", got)
	}
	occupancies := []model.RackUOccupancy{{StartU: 1, EndU: 42}}
	if got := availableRanges(42, occupancies); len(got) != 0 {
		t.Fatalf("fully occupied 42U rack should have no free range, got %#v", got)
	}
}

func TestFindConflictsUsesUnifiedRackU(t *testing.T) {
	deviceID := uuid.New()
	other := uuid.New()
	positions := []model.RackDevicePosition{
		{DeviceID: deviceID, StartU: 10, EndU: 12},
		{DeviceID: other, StartU: 12, EndU: 14},
	}
	conflicts := findConflicts(positions, deviceID, PositionChangeInput{StartU: 11, HeightU: 2})
	if len(conflicts) != 1 || conflicts[0].DeviceID != other {
		t.Fatalf("unified U conflicts = %#v, want only overlapping device", conflicts)
	}
}

func TestOccupancyRowsCreatesOneUnifiedRange(t *testing.T) {
	positionID, deviceID, rackID := uuid.New(), uuid.New(), uuid.New()
	position := model.RackDevicePosition{BaseModel: model.BaseModel{ID: positionID}, DeviceID: deviceID, RackID: rackID, StartU: 40, EndU: 42}
	rows := occupancyRows(position)
	if len(rows) != 1 {
		t.Fatalf("occupancy rows = %#v, want one row", rows)
	}
	row := rows[0]
	if row.PositionID != positionID || row.DeviceID != deviceID || row.RackID != rackID || row.StartU != 40 || row.EndU != 42 {
		t.Fatalf("occupancy row did not retain position data: %#v", row)
	}
}

func TestValidatePositionInputRackBoundaries(t *testing.T) {
	base := PositionChangeInput{Version: 1, RackID: uuid.New(), StartU: 1, HeightU: 1, Orientation: model.DeviceOrientationNormal}
	if err := validatePositionInput(base); err != nil {
		t.Fatalf("valid 1U input rejected: %v", err)
	}
	fullRack := base
	fullRack.HeightU = 42
	if err := validatePositionInput(fullRack); err != nil {
		t.Fatalf("valid 42U input rejected: %v", err)
	}
	invalid := base
	invalid.StartU = 0
	if err := validatePositionInput(invalid); err == nil {
		t.Fatal("startU=0 should be rejected")
	}
}
