package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrUPositionConflict = errors.New("rack u position conflict")

// DeviceQuery defines the supported device ledger filters.
type DeviceQuery struct {
	Scope           string
	Page            int
	PageSize        int
	Search          string
	TypeID          *uuid.UUID
	LifecycleStatus string
	RackID          *uuid.UUID
}

// RackDiagramImportStore extends device persistence with room-level import lookups.
type RackDiagramImportStore interface {
	DeviceStore
	GetRoom(context.Context, uuid.UUID) (model.Room, error)
	ListRacksByRoom(context.Context, uuid.UUID) ([]model.Rack, error)
	ListDevicesBySerialNumbers(context.Context, []string) ([]model.Device, error)
}

// DeviceStore defines persistence required by device ledger and U-position use cases.
type DeviceStore interface {
	WithinDeviceTransaction(context.Context, func(DeviceStore) error) error
	ListDeviceTypes(context.Context) ([]model.DeviceType, error)
	GetDeviceType(context.Context, uuid.UUID) (model.DeviceType, error)
	CountDevicesByType(context.Context, uuid.UUID) (int64, error)
	CreateDeviceType(context.Context, *model.DeviceType) error
	UpdateDeviceType(context.Context, *model.DeviceType, uint) error
	DeleteDeviceType(context.Context, uuid.UUID, uint) error
	ListDevices(context.Context, DeviceQuery) ([]model.Device, int64, error)
	GetDevice(context.Context, uuid.UUID) (model.Device, error)
	GetDeviceForUpdate(context.Context, uuid.UUID) (model.Device, error)
	CountDevices(context.Context) (int64, error)
	CreateDevice(context.Context, *model.Device) error
	UpdateDevice(context.Context, *model.Device, uint) error
	UpdateDeviceLifecycle(context.Context, uuid.UUID, uint, string) error
	DeleteDevice(context.Context, uuid.UUID, uint) error
	GetRackForUpdate(context.Context, uuid.UUID) (model.Rack, error)
	UpdateRackOperationalStatus(context.Context, uuid.UUID, string) error
	GetPositionByDevice(context.Context, uuid.UUID) (model.RackDevicePosition, error)
	ListPositionsByRack(context.Context, uuid.UUID) ([]model.RackDevicePosition, error)
	ListOccupanciesByRack(context.Context, uuid.UUID) ([]model.RackUOccupancy, error)
	CountPositionsByRack(context.Context, uuid.UUID) (int64, error)
	CreatePosition(context.Context, *model.RackDevicePosition) error
	DeletePosition(context.Context, uuid.UUID) error
	CreateOccupancies(context.Context, []model.RackUOccupancy) error
	DeleteOccupanciesByPosition(context.Context, uuid.UUID) error
	CreatePositionHistory(context.Context, *model.DevicePositionHistory) error
	ListPositionHistory(context.Context, uuid.UUID) ([]model.DevicePositionHistory, error)
	GetOperationPolicy(context.Context) (model.OperationPolicy, error)
	UpdateOperationPolicy(context.Context, *model.OperationPolicy, uint) error
	CreateApproval(context.Context, *model.ApprovalRecord) error
	ListApprovals(context.Context, string) ([]model.ApprovalRecord, error)
	GetApprovalForUpdate(context.Context, uuid.UUID) (model.ApprovalRecord, error)
	UpdateApprovalDecision(context.Context, *model.ApprovalRecord, uint) error
	CreateAuditLog(context.Context, *model.AuditLog) error
}

// GormDeviceStore implements DeviceStore with GORM and PostgreSQL row locking.
type GormDeviceStore struct{ db *gorm.DB }

func NewDeviceStore(db *gorm.DB) *GormDeviceStore { return &GormDeviceStore{db: db} }

func (s *GormDeviceStore) WithinDeviceTransaction(ctx context.Context, fn func(DeviceStore) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&GormDeviceStore{db: tx}) })
}

func (s *GormDeviceStore) ListDeviceTypes(ctx context.Context) ([]model.DeviceType, error) {
	var items []model.DeviceType
	err := s.db.WithContext(ctx).Order("sort_order ASC, name ASC").Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) GetDeviceType(ctx context.Context, id uuid.UUID) (model.DeviceType, error) {
	var item model.DeviceType
	err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormDeviceStore) GetRoom(ctx context.Context, id uuid.UUID) (model.Room, error) {
	var item model.Room
	err := s.db.WithContext(ctx).First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormDeviceStore) ListRacksByRoom(ctx context.Context, roomID uuid.UUID) ([]model.Rack, error) {
	var items []model.Rack
	err := s.db.WithContext(ctx).Where("room_id = ?", roomID).Order("sort_order ASC, code ASC").Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) ListDevicesBySerialNumbers(ctx context.Context, serialNumbers []string) ([]model.Device, error) {
	if len(serialNumbers) == 0 {
		return []model.Device{}, nil
	}
	normalized := make([]string, 0, len(serialNumbers))
	for _, value := range serialNumbers {
		if value = strings.ToLower(strings.TrimSpace(value)); value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return []model.Device{}, nil
	}
	var items []model.Device
	err := s.db.WithContext(ctx).Preload("Type").Preload("CurrentPosition", "deleted_at IS NULL").Where("LOWER(serial_number) IN ?", normalized).Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) CountDevicesByType(ctx context.Context, id uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Device{}).Where("type_id = ?", id).Count(&count).Error
	return count, err
}

func (s *GormDeviceStore) CreateDeviceType(ctx context.Context, item *model.DeviceType) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormDeviceStore) UpdateDeviceType(ctx context.Context, item *model.DeviceType, expectedVersion uint) error {
	updates := map[string]interface{}{
		"code": item.Code, "name": item.Name, "category": item.Category, "status": item.Status,
		"default_height_u": item.DefaultHeightU, "default_weight_kg": item.DefaultWeightKg,
		"default_rated_power_w": item.DefaultRatedPowerW, "default_peak_power_w": item.DefaultPeakPowerW,
		"default_dual_power": item.DefaultDualPower, "description": item.Description, "sort_order": item.SortOrder,
		"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC(),
	}
	result := s.db.WithContext(ctx).Model(&model.DeviceType{}).
		Where("id = ? AND version = ?", item.ID, expectedVersion).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	item.Version = expectedVersion + 1
	return nil
}

func (s *GormDeviceStore) DeleteDeviceType(ctx context.Context, id uuid.UUID, expectedVersion uint) error {
	result := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, expectedVersion).Delete(&model.DeviceType{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (s *GormDeviceStore) ListDevices(ctx context.Context, query DeviceQuery) ([]model.Device, int64, error) {
	db := s.db.WithContext(ctx).Model(&model.Device{})
	if query.Scope == "PC" {
		db = db.Where("type_id IN (SELECT id FROM device_types WHERE category = ? AND deleted_at IS NULL)", model.DeviceCategoryPC)
	} else if query.Scope == "ROOM" {
		db = db.Where("type_id IN (SELECT id FROM device_types WHERE category <> ? AND deleted_at IS NULL)", model.DeviceCategoryPC)
	}
	if query.Search != "" {
		term := "%" + strings.ToLower(query.Search) + "%"
		db = db.Where("LOWER(name) LIKE ? OR LOWER(code) LIKE ? OR LOWER(asset_number) LIKE ? OR LOWER(serial_number) LIKE ? OR LOWER(management_ip) LIKE ? OR LOWER(business_ip) LIKE ?", term, term, term, term, term, term)
	}
	if query.TypeID != nil {
		db = db.Where("type_id = ?", *query.TypeID)
	}
	if query.LifecycleStatus != "" {
		db = db.Where("lifecycle_status = ?", query.LifecycleStatus)
	}
	if query.RackID != nil {
		db = db.Where("EXISTS (SELECT 1 FROM rack_device_positions p WHERE p.device_id = devices.id AND p.rack_id = ? AND p.deleted_at IS NULL)", *query.RackID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.Device
	err := db.Preload("Type").Preload("CurrentPosition", "deleted_at IS NULL").Preload("CurrentPosition.Rack").
		Order("created_at DESC").Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize).Find(&items).Error
	return items, total, err
}

func (s *GormDeviceStore) GetDevice(ctx context.Context, id uuid.UUID) (model.Device, error) {
	var item model.Device
	err := s.db.WithContext(ctx).Preload("Type").Preload("CurrentPosition", "deleted_at IS NULL").Preload("CurrentPosition.Rack").First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormDeviceStore) GetDeviceForUpdate(ctx context.Context, id uuid.UUID) (model.Device, error) {
	var item model.Device
	err := s.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormDeviceStore) CountDevices(ctx context.Context) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.Device{}).Count(&count).Error
	return count, err
}

func (s *GormDeviceStore) CreateDevice(ctx context.Context, item *model.Device) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormDeviceStore) UpdateDevice(ctx context.Context, item *model.Device, expectedVersion uint) error {
	updates := map[string]interface{}{
		"type_id": item.TypeID, "code": item.Code, "name": item.Name, "asset_number": item.AssetNumber,
		"serial_number": item.SerialNumber, "manufacturer": item.Manufacturer, "model_number": item.ModelNumber,
		"specification": item.Specification, "firmware_version": item.FirmwareVersion, "purchase_batch": item.PurchaseBatch,
		"warranty_expires_at": item.WarrantyExpiresAt, "organization": item.Organization, "manager": item.Manager,
		"contact": item.Contact, "business_system": item.BusinessSystem, "application_name": item.ApplicationName,
		"lifecycle_status": item.LifecycleStatus, "height_u": item.HeightU, "width_mm": item.WidthMm,
		"depth_mm": item.DepthMm, "height_mm": item.HeightMm, "weight_kg": item.WeightKg,
		"rated_power_w": item.RatedPowerW, "peak_power_w": item.PeakPowerW, "input_voltage": item.InputVoltage,
		"dual_power_required": item.DualPowerRequired, "management_ip": item.ManagementIP, "business_ip": item.BusinessIP,
		"mac_address": item.MACAddress, "management_protocol": item.ManagementProtocol,
		"monitoring_status": item.MonitoringStatus, "external_qr_code": item.ExternalQRCode,
		"external_qr_code_url": item.ExternalQRCodeURL, "tags": item.Tags, "remarks": item.Remarks,
		"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC(),
	}
	result := s.db.WithContext(ctx).Model(&model.Device{}).Where("id = ? AND version = ?", item.ID, expectedVersion).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	item.Version = expectedVersion + 1
	return nil
}

func (s *GormDeviceStore) UpdateDeviceLifecycle(ctx context.Context, id uuid.UUID, expectedVersion uint, status string) error {
	result := s.db.WithContext(ctx).Model(&model.Device{}).Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]interface{}{"lifecycle_status": status, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (s *GormDeviceStore) DeleteDevice(ctx context.Context, id uuid.UUID, expectedVersion uint) error {
	result := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, expectedVersion).Delete(&model.Device{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (s *GormDeviceStore) GetRackForUpdate(ctx context.Context, id uuid.UUID) (model.Rack, error) {
	var item model.Rack
	err := s.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormDeviceStore) UpdateRackOperationalStatus(ctx context.Context, id uuid.UUID, status string) error {
	return s.db.WithContext(ctx).Model(&model.Rack{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": status, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}).Error
}

func (s *GormDeviceStore) GetPositionByDevice(ctx context.Context, deviceID uuid.UUID) (model.RackDevicePosition, error) {
	var item model.RackDevicePosition
	err := s.db.WithContext(ctx).Preload("Device").Preload("Rack").Where("device_id = ?", deviceID).First(&item).Error
	return item, err
}

func (s *GormDeviceStore) ListPositionsByRack(ctx context.Context, rackID uuid.UUID) ([]model.RackDevicePosition, error) {
	var items []model.RackDevicePosition
	err := s.db.WithContext(ctx).Preload("Device").Preload("Device.Type").Where("rack_id = ?", rackID).Order("start_u DESC").Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) ListOccupanciesByRack(ctx context.Context, rackID uuid.UUID) ([]model.RackUOccupancy, error) {
	var items []model.RackUOccupancy
	err := s.db.WithContext(ctx).Where("rack_id = ?", rackID).Order("start_u ASC").Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) CountPositionsByRack(ctx context.Context, rackID uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.RackDevicePosition{}).Where("rack_id = ?", rackID).Count(&count).Error
	return count, err
}

func (s *GormDeviceStore) CreatePosition(ctx context.Context, item *model.RackDevicePosition) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormDeviceStore) DeletePosition(ctx context.Context, id uuid.UUID) error {
	return s.db.WithContext(ctx).Delete(&model.RackDevicePosition{}, "id = ?", id).Error
}

func (s *GormDeviceStore) CreateOccupancies(ctx context.Context, items []model.RackUOccupancy) error {
	if len(items) == 0 {
		return nil
	}
	err := s.db.WithContext(ctx).Create(&items).Error
	if isExclusionViolation(err) {
		return ErrUPositionConflict
	}
	return err
}

func (s *GormDeviceStore) DeleteOccupanciesByPosition(ctx context.Context, positionID uuid.UUID) error {
	return s.db.WithContext(ctx).Where("position_id = ?", positionID).Delete(&model.RackUOccupancy{}).Error
}

func (s *GormDeviceStore) CreatePositionHistory(ctx context.Context, item *model.DevicePositionHistory) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormDeviceStore) ListPositionHistory(ctx context.Context, deviceID uuid.UUID) ([]model.DevicePositionHistory, error) {
	var items []model.DevicePositionHistory
	err := s.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("created_at DESC").Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) GetOperationPolicy(ctx context.Context) (model.OperationPolicy, error) {
	var item model.OperationPolicy
	err := s.db.WithContext(ctx).Order("created_at ASC").First(&item).Error
	return item, err
}

func (s *GormDeviceStore) UpdateOperationPolicy(ctx context.Context, item *model.OperationPolicy, expectedVersion uint) error {
	result := s.db.WithContext(ctx).Model(&model.OperationPolicy{}).Where("id = ? AND version = ?", item.ID, expectedVersion).
		Updates(map[string]interface{}{
			"decommission_approval_enabled": item.DecommissionApprovalEnabled,
			"assign_approval_enabled":       item.AssignApprovalEnabled, "move_approval_enabled": item.MoveApprovalEnabled,
			"approval_mode": item.ApprovalMode, "default_approver_role": item.DefaultApproverRole,
			"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	item.Version = expectedVersion + 1
	return nil
}

func (s *GormDeviceStore) CreateApproval(ctx context.Context, item *model.ApprovalRecord) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormDeviceStore) ListApprovals(ctx context.Context, status string) ([]model.ApprovalRecord, error) {
	var items []model.ApprovalRecord
	db := s.db.WithContext(ctx).Preload("Device").Order("requested_at DESC")
	if status != "" {
		db = db.Where("status = ?", status)
	}
	err := db.Find(&items).Error
	return items, err
}

func (s *GormDeviceStore) GetApprovalForUpdate(ctx context.Context, id uuid.UUID) (model.ApprovalRecord, error) {
	var item model.ApprovalRecord
	err := s.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Device").First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormDeviceStore) UpdateApprovalDecision(ctx context.Context, item *model.ApprovalRecord, expectedVersion uint) error {
	result := s.db.WithContext(ctx).Model(&model.ApprovalRecord{}).Where("id = ? AND version = ?", item.ID, expectedVersion).
		Updates(map[string]interface{}{
			"status": item.Status, "decided_by": item.DecidedBy, "decided_at": item.DecidedAt,
			"decision_comment": item.DecisionComment, "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	item.Version = expectedVersion + 1
	return nil
}

func (s *GormDeviceStore) CreateAuditLog(ctx context.Context, item *model.AuditLog) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func isExclusionViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23P01"
}
