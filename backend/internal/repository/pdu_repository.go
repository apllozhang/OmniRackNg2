package repository

import (
	"context"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"omnirack-replica/backend/internal/model"
	"time"
)

type PDUStore interface {
	WithinPDUTransaction(context.Context, func(PDUStore) error) error
	GetRack(context.Context, uuid.UUID) (model.Rack, error)
	GetPDU(context.Context, uuid.UUID) (model.PDU, error)
	ListPDUsByRack(context.Context, uuid.UUID) ([]model.PDU, error)
	CreatePDU(context.Context, *model.PDU) error
	UpdatePDU(context.Context, *model.PDU, uint) error
	DeletePDU(context.Context, uuid.UUID, uint) error
	GetSocket(context.Context, uuid.UUID) (model.PDUSocket, error)
	ListSocketsByPDU(context.Context, uuid.UUID) ([]model.PDUSocket, error)
	CreateSocket(context.Context, *model.PDUSocket) error
	UpdateSocket(context.Context, *model.PDUSocket, uint) error
	SetSocketStatus(context.Context, uuid.UUID, string) error
	DeleteSocket(context.Context, uuid.UUID, uint) error
	GetConnection(context.Context, uuid.UUID) (model.PDUConnection, error)
	GetConnectionBySocket(context.Context, uuid.UUID) (model.PDUConnection, error)
	ListConnectionsByRack(context.Context, uuid.UUID) ([]model.PDUConnection, error)
	CreateConnection(context.Context, *model.PDUConnection) error
	DeleteConnection(context.Context, uuid.UUID, uint) error
	GetDevice(context.Context, uuid.UUID) (model.Device, error)
	GetPositionByDevice(context.Context, uuid.UUID) (model.RackDevicePosition, error)
	CreateAuditLog(context.Context, *model.AuditLog) error
}

type GormPDUStore struct {
	db            *gorm.DB
	transactional bool
}
type pduConnectionRow struct {
	ID             uuid.UUID      `gorm:"column:id"`
	CreatedAt      time.Time      `gorm:"column:created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at"`
	Version        uint           `gorm:"column:version"`
	SocketID       uuid.UUID      `gorm:"column:socket_id"`
	DeviceID       uuid.UUID      `gorm:"column:device_id"`
	PowerW         *float64       `gorm:"column:power_w"`
	Circuit        string         `gorm:"column:circuit"`
	RedundancyRole string         `gorm:"column:redundancy_role"`
	ConnectedAt    time.Time      `gorm:"column:connected_at"`
	ConnectedBy    *uuid.UUID     `gorm:"column:connected_by"`
}

func (r pduConnectionRow) model() model.PDUConnection {
	return model.PDUConnection{
		BaseModel: model.BaseModel{ID: r.ID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt, Version: r.Version},
		SocketID:  r.SocketID, DeviceID: r.DeviceID, PowerW: r.PowerW, Circuit: r.Circuit,
		RedundancyRole: r.RedundancyRole, ConnectedAt: r.ConnectedAt, ConnectedBy: r.ConnectedBy,
	}
}
func NewPDUStore(db *gorm.DB) *GormPDUStore { return &GormPDUStore{db: db} }
func (s *GormPDUStore) WithinPDUTransaction(ctx context.Context, fn func(PDUStore) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&GormPDUStore{db: tx, transactional: true}) })
}
func (s *GormPDUStore) readForMutation(ctx context.Context) *gorm.DB {
	db := s.db.WithContext(ctx)
	if s.transactional {
		db = db.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	return db
}
func (s *GormPDUStore) GetRack(ctx context.Context, id uuid.UUID) (model.Rack, error) {
	var x model.Rack
	err := s.db.WithContext(ctx).First(&x, "id = ?", id).Error
	return x, err
}
func (s *GormPDUStore) GetPDU(ctx context.Context, id uuid.UUID) (model.PDU, error) {
	var x model.PDU
	if err := s.readForMutation(ctx).Preload("Sockets", "deleted_at IS NULL").First(&x, "id = ?", id).Error; err != nil {
		return x, err
	}
	if err := s.populateSocketConnections(ctx, x.Sockets); err != nil {
		return x, err
	}
	return x, nil
}
func (s *GormPDUStore) ListPDUsByRack(ctx context.Context, rackID uuid.UUID) ([]model.PDU, error) {
	var xs []model.PDU
	if err := s.db.WithContext(ctx).Where("rack_id = ?", rackID).Preload("Sockets", "deleted_at IS NULL").Order("code ASC").Find(&xs).Error; err != nil {
		return xs, err
	}
	for i := range xs {
		if err := s.populateSocketConnections(ctx, xs[i].Sockets); err != nil {
			return xs, err
		}
	}
	return xs, nil
}

func (s *GormPDUStore) CreatePDU(ctx context.Context, x *model.PDU) error {
	return s.db.WithContext(ctx).Omit("Rack", "Sockets").Create(x).Error
}
func (s *GormPDUStore) UpdatePDU(ctx context.Context, x *model.PDU, v uint) error {
	r := s.db.WithContext(ctx).Model(&model.PDU{}).Where("id = ? AND version = ?", x.ID, v).Updates(map[string]interface{}{"code": x.Code, "name": x.Name, "manufacturer": x.Manufacturer, "model_number": x.ModelNumber, "serial_number": x.SerialNumber, "input_voltage": x.InputVoltage, "rated_power_w": x.RatedPowerW, "rated_current_a": x.RatedCurrentA, "status": x.Status, "remarks": x.Remarks, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return ErrVersionConflict
	}
	x.Version = v + 1
	return nil
}
func (s *GormPDUStore) DeletePDU(ctx context.Context, id uuid.UUID, v uint) error {
	r := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, v).Delete(&model.PDU{})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}
func (s *GormPDUStore) GetSocket(ctx context.Context, id uuid.UUID) (model.PDUSocket, error) {
	var x model.PDUSocket
	err := s.readForMutation(ctx).First(&x, "id = ?", id).Error
	return x, err
}
func (s *GormPDUStore) ListSocketsByPDU(ctx context.Context, id uuid.UUID) ([]model.PDUSocket, error) {
	var xs []model.PDUSocket
	if err := s.db.WithContext(ctx).Where("pd_uid = ?", id).Order("socket_no ASC").Find(&xs).Error; err != nil {
		return xs, err
	}
	if err := s.populateSocketConnections(ctx, xs); err != nil {
		return xs, err
	}
	return xs, nil
}

// populateSocketConnections avoids relying on GORM to infer the has-one relation
// from PDUSocket.Connection. The explicit query also keeps soft-deleted connections
// out of API responses and works consistently across GORM versions.
func (s *GormPDUStore) populateSocketConnections(ctx context.Context, sockets []model.PDUSocket) error {
	if len(sockets) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(sockets))
	for _, socket := range sockets {
		ids = append(ids, socket.ID)
	}
	connections, err := s.loadConnections(ctx, s.db.WithContext(ctx).Where("deleted_at IS NULL AND socket_id IN ?", ids))
	if err != nil {
		return err
	}
	if err := s.populateConnectionDetails(ctx, connections); err != nil {
		return err
	}
	bySocket := make(map[uuid.UUID]*model.PDUConnection, len(connections))
	for i := range connections {
		bySocket[connections[i].SocketID] = &connections[i]
	}
	for i := range sockets {
		sockets[i].Connection = bySocket[sockets[i].ID]
	}
	return nil
}

func (s *GormPDUStore) loadConnections(ctx context.Context, query *gorm.DB) ([]model.PDUConnection, error) {
	var rows []pduConnectionRow
	if err := query.Table("pdu_connections").Find(&rows).Error; err != nil {
		return nil, err
	}
	connections := make([]model.PDUConnection, len(rows))
	for i := range rows {
		connections[i] = rows[i].model()
	}
	return connections, nil
}

func (s *GormPDUStore) populateConnectionDetails(ctx context.Context, connections []model.PDUConnection) error {
	if len(connections) == 0 {
		return nil
	}
	socketIDs := make([]uuid.UUID, 0, len(connections))
	deviceIDs := make([]uuid.UUID, 0, len(connections))
	for _, connection := range connections {
		socketIDs = append(socketIDs, connection.SocketID)
		deviceIDs = append(deviceIDs, connection.DeviceID)
	}
	var sockets []model.PDUSocket
	if err := s.db.WithContext(ctx).Where("id IN ?", socketIDs).Find(&sockets).Error; err != nil {
		return err
	}
	var devices []model.Device
	if err := s.db.WithContext(ctx).Where("id IN ?", deviceIDs).Find(&devices).Error; err != nil {
		return err
	}
	socketsByID := make(map[uuid.UUID]model.PDUSocket, len(sockets))
	for _, socket := range sockets {
		socketsByID[socket.ID] = socket
	}
	devicesByID := make(map[uuid.UUID]model.Device, len(devices))
	for _, device := range devices {
		devicesByID[device.ID] = device
	}
	for i := range connections {
		connections[i].Socket = socketsByID[connections[i].SocketID]
		connections[i].Device = devicesByID[connections[i].DeviceID]
	}
	return nil
}
func (s *GormPDUStore) CreateSocket(ctx context.Context, x *model.PDUSocket) error {
	return s.db.WithContext(ctx).Omit("PDU", "Connection").Create(x).Error
}
func (s *GormPDUStore) UpdateSocket(ctx context.Context, x *model.PDUSocket, v uint) error {
	r := s.db.WithContext(ctx).Model(&model.PDUSocket{}).Where("id = ? AND version = ?", x.ID, v).Updates(map[string]interface{}{"socket_no": x.SocketNo, "standard": x.Standard, "amperage_a": x.AmperageA, "label": x.Label, "status": x.Status, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return ErrVersionConflict
	}
	x.Version = v + 1
	return nil
}
func (s *GormPDUStore) SetSocketStatus(ctx context.Context, id uuid.UUID, status string) error {
	r := s.db.WithContext(ctx).Model(&model.PDUSocket{}).Where("id = ?", id).Updates(map[string]interface{}{"status": status, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s *GormPDUStore) DeleteSocket(ctx context.Context, id uuid.UUID, v uint) error {
	r := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, v).Delete(&model.PDUSocket{})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}
func (s *GormPDUStore) GetConnection(ctx context.Context, id uuid.UUID) (model.PDUConnection, error) {
	connections, err := s.loadConnections(ctx, s.db.WithContext(ctx).Where("deleted_at IS NULL AND id = ?", id))
	if err != nil {
		return model.PDUConnection{}, err
	}
	if len(connections) == 0 {
		return model.PDUConnection{}, gorm.ErrRecordNotFound
	}
	if err = s.populateConnectionDetails(ctx, connections); err != nil {
		return model.PDUConnection{}, err
	}
	return connections[0], nil
}
func (s *GormPDUStore) GetConnectionBySocket(ctx context.Context, id uuid.UUID) (model.PDUConnection, error) {
	connections, err := s.loadConnections(ctx, s.db.WithContext(ctx).Where("deleted_at IS NULL AND socket_id = ?", id))
	if err != nil {
		return model.PDUConnection{}, err
	}
	if len(connections) == 0 {
		return model.PDUConnection{}, gorm.ErrRecordNotFound
	}
	return connections[0], nil
}
func (s *GormPDUStore) ListConnectionsByRack(ctx context.Context, id uuid.UUID) ([]model.PDUConnection, error) {
	connections, err := s.loadConnections(ctx, s.db.WithContext(ctx).Where("pdu_connections.deleted_at IS NULL").Joins("JOIN pdu_sockets ON pdu_sockets.id = pdu_connections.socket_id AND pdu_sockets.deleted_at IS NULL").Joins("JOIN pdus ON pdus.id = pdu_sockets.pd_uid AND pdus.deleted_at IS NULL").Where("pdus.rack_id = ?", id))
	if err != nil {
		return connections, err
	}
	if err = s.populateConnectionDetails(ctx, connections); err != nil {
		return connections, err
	}
	return connections, nil
}
func (s *GormPDUStore) CreateConnection(ctx context.Context, x *model.PDUConnection) error {
	return s.db.WithContext(ctx).Create(x).Error
}
func (s *GormPDUStore) DeleteConnection(ctx context.Context, id uuid.UUID, v uint) error {
	r := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, v).Delete(&model.PDUConnection{})
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}
func (s *GormPDUStore) GetDevice(ctx context.Context, id uuid.UUID) (model.Device, error) {
	var x model.Device
	err := s.readForMutation(ctx).First(&x, "id = ?", id).Error
	return x, err
}
func (s *GormPDUStore) GetPositionByDevice(ctx context.Context, id uuid.UUID) (model.RackDevicePosition, error) {
	var x model.RackDevicePosition
	err := s.db.WithContext(ctx).Where("device_id = ?", id).First(&x).Error
	return x, err
}
func (s *GormPDUStore) CreateAuditLog(ctx context.Context, x *model.AuditLog) error {
	return s.db.WithContext(ctx).Create(x).Error
}
