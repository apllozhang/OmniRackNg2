package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

func TestPDUConnectChecksLockedStateAndUpdatesSocketVersionAtomically(t *testing.T) {
	for _, scenario := range []string{"connect", "moved to other rack", "PDU disabled"} {
		t.Run(scenario, func(t *testing.T) {
			ops, m, closeDB := opsMock(t)
			defer closeDB()
			device, socket, pdu, rack := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*devices.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "lifecycle_status"}).AddRow(device, model.DeviceStatusRunning))
			m.ExpectQuery(`SELECT .*pdu_sockets.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "pd_uid", "version", "status"}).AddRow(socket, pdu, 5, model.PDUSocketStatusAvailable))
			status := model.PDUStatusActive
			if scenario == "PDU disabled" {
				status = model.PDUStatusDisabled
			}
			m.ExpectQuery(`SELECT .*pdus.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "rack_id", "status"}).AddRow(pdu, rack, status))
			m.ExpectQuery(`SELECT .*pdu_sockets`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			if scenario != "PDU disabled" {
				m.ExpectQuery(`SELECT .*pdu_connections`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
				deviceRack := rack
				if scenario == "moved to other rack" {
					deviceRack = uuid.New()
				}
				m.ExpectQuery(`SELECT .*rack_device_positions`).WillReturnRows(sqlmock.NewRows([]string{"id", "rack_id", "device_id"}).AddRow(uuid.New(), deviceRack, device))
			}
			if scenario == "connect" {
				m.ExpectExec(`INSERT INTO "pdu_connections"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`UPDATE "pdu_sockets".*"version"=version \+ 1`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			_, err := NewPDUService(repository.NewPDUStore(ops.DB)).Connect(context.Background(), Actor{}, socket, PDUConnectionInput{DeviceID: device})
			if (err == nil) != (scenario == "connect") {
				t.Fatalf("%s: %v", scenario, err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPDUSocketCannotLieAboutConnectionOrIgnoreDatabaseFailure(t *testing.T) {
	for _, scenario := range []string{"connected marked available", "unconnected marked connected", "database failure"} {
		t.Run(scenario, func(t *testing.T) {
			ops, m, closeDB := opsMock(t)
			defer closeDB()
			socket, pdu := uuid.New(), uuid.New()
			m.ExpectBegin()
			m.ExpectQuery(`SELECT .*pdu_sockets.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "pd_uid", "version", "status"}).AddRow(socket, pdu, 4, model.PDUSocketStatusConnected))
			q := m.ExpectQuery(`SELECT .*pdu_connections`)
			databaseFailure := errors.New("connection lookup failed")
			if scenario == "connected marked available" {
				q.WillReturnRows(sqlmock.NewRows([]string{"id", "socket_id"}).AddRow(uuid.New(), socket))
			} else if scenario == "database failure" {
				q.WillReturnError(databaseFailure)
			} else {
				q.WillReturnRows(sqlmock.NewRows([]string{"id"}))
			}
			m.ExpectRollback()
			status := model.PDUSocketStatusAvailable
			if scenario == "unconnected marked connected" {
				status = model.PDUSocketStatusConnected
			}
			_, err := NewPDUService(repository.NewPDUStore(ops.DB)).UpdateSocket(context.Background(), Actor{}, socket, 4, model.PDUSocket{PDUID: pdu, SocketNo: 1, Standard: "CN", AmperageA: 10, Status: status})
			if err == nil || (scenario == "database failure" && !errors.Is(err, databaseFailure)) {
				t.Fatalf("socket invariant lost: %v", err)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type poweredPositionStore struct {
	repository.DeviceStore
	device   model.Device
	position model.RackDevicePosition
}

func (s poweredPositionStore) GetDeviceForUpdate(context.Context, uuid.UUID) (model.Device, error) {
	return s.device, nil
}
func (s poweredPositionStore) GetPositionByDevice(context.Context, uuid.UUID) (model.RackDevicePosition, error) {
	return s.position, nil
}
func (s poweredPositionStore) CountActivePowerConnections(context.Context, uuid.UUID) (int64, error) {
	return 1, nil
}

func TestConnectedPowerPreventsCrossRackMoveAndDecommission(t *testing.T) {
	device := model.Device{BaseModel: model.BaseModel{ID: uuid.New(), Version: 3}, LifecycleStatus: model.DeviceStatusRunning}
	store := poweredPositionStore{device: device, position: model.RackDevicePosition{RackID: uuid.New(), DeviceID: device.ID}}
	in := PositionChangeInput{Version: 3, RackID: uuid.New(), StartU: 1, HeightU: 1, Reason: "迁移"}
	for _, operation := range []string{model.PositionOperationMove, model.PositionOperationDecommission} {
		_, err := executePositionChange(context.Background(), store, Actor{}, device.ID, operation, in)
		if !errors.Is(err, ErrInvalidResource) {
			t.Fatalf("%s left a live power connection behind: %v", operation, err)
		}
	}
}

func TestDevicePaginationRejectsHugeOffsetBeforeRepositoryAccess(t *testing.T) {
	for _, page := range []int{10001, int(^uint(0) >> 1)} {
		_, err := NewDeviceService(nil).ListDevices(context.Background(), repository.DeviceQuery{Page: page, PageSize: 100})
		if !errors.Is(err, ErrInvalidResource) {
			t.Fatalf("unsafe page %d reached repository: %v", page, err)
		}
	}
}

func TestPDUCreateDoesNotPersistClientSuppliedNestedResources(t *testing.T) {
	ops, mock, closeDB := opsMock(t)
	defer closeDB()
	rackID, forgedID := uuid.New(), uuid.New()
	mock.ExpectQuery(`SELECT .*racks`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(rackID))
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO "pdus"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	in := model.PDU{
		BaseModel: model.BaseModel{ID: forgedID, Version: 99, CreatedAt: time.Unix(1, 0)},
		Code:      "PDU-1", Name: "配电单元", Status: model.PDUStatusActive,
		Rack:    model.Rack{BaseModel: model.BaseModel{ID: uuid.New()}, Name: "伪造机柜"},
		Sockets: []model.PDUSocket{{SocketNo: 1, Status: model.PDUSocketStatusConnected}},
	}
	out, err := NewPDUService(repository.NewPDUStore(ops.DB)).Create(context.Background(), Actor{}, rackID, PDUCreateInput{PDU: in})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID == forgedID || out.Version != 1 || out.CreatedAt.Equal(in.CreatedAt) || out.Rack.ID != uuid.Nil || len(out.Sockets) != 0 {
		t.Fatalf("PDU create trusted client-owned identities or relations: %+v", out)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
