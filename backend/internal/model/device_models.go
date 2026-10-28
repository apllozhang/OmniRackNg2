package model

import (
	"time"

	"github.com/google/uuid"
)

const (
	DeviceTypeStatusActive   = "ACTIVE"
	DeviceTypeStatusDisabled = "DISABLED"

	DeviceCategoryServer    = "SERVER"
	DeviceCategoryPC        = "PC"
	DeviceCategoryNetwork   = "NETWORK"
	DeviceCategoryStorage   = "STORAGE"
	DeviceCategorySecurity  = "SECURITY"
	DeviceCategoryPowerEnv  = "POWER_ENVIRONMENT"
	DeviceCategoryAccessory = "ACCESSORY"
	DeviceCategoryOther     = "OTHER"

	DeviceStatusWaitingRack    = "WAITING_RACK"
	DeviceStatusRunning        = "RUNNING"
	DeviceStatusMaintenance    = "MAINTENANCE"
	DeviceStatusPendingRemoval = "PENDING_REMOVAL"
	DeviceStatusOffRack        = "OFF_RACK"
	DeviceStatusScrapped       = "SCRAPPED"

	DeviceOrientationNormal  = "NORMAL"
	DeviceOrientationReverse = "REVERSE"

	PositionOperationAssign       = "ASSIGN"
	PositionOperationMove         = "MOVE"
	PositionOperationDecommission = "DECOMMISSION"

	ApprovalStatusPending   = "PENDING"
	ApprovalStatusApproved  = "APPROVED"
	ApprovalStatusRejected  = "REJECTED"
	ApprovalStatusCancelled = "CANCELLED"
)

// DeviceType describes a reusable category and default installation parameters.
type DeviceType struct {
	BaseModel
	Code               string   `gorm:"size:50;not null;uniqueIndex:idx_device_types_code_active,where:deleted_at IS NULL" json:"code"`
	Name               string   `gorm:"size:150;not null;index" json:"name"`
	Category           string   `gorm:"size:40;not null;index" json:"category"`
	Status             string   `gorm:"size:20;not null;default:ACTIVE;index" json:"status"`
	DefaultHeightU     int      `gorm:"not null;default:1" json:"defaultHeightU"`
	DefaultWeightKg    *float64 `json:"defaultWeightKg,omitempty"`
	DefaultRatedPowerW *float64 `json:"defaultRatedPowerW,omitempty"`
	DefaultPeakPowerW  *float64 `json:"defaultPeakPowerW,omitempty"`
	DefaultDualPower   bool     `gorm:"not null;default:false" json:"defaultDualPower"`
	Description        string   `gorm:"type:text" json:"description,omitempty"`
	SortOrder          int      `gorm:"not null;default:0;index" json:"sortOrder"`
}

// Device is the asset ledger record. Its active rack location is stored separately.
type Device struct {
	BaseModel
	TypeID             uuid.UUID           `gorm:"type:uuid;not null;index" json:"typeId"`
	Type               DeviceType          `gorm:"foreignKey:TypeID" json:"type,omitempty"`
	Code               string              `gorm:"size:80;not null;uniqueIndex:idx_devices_code_active,where:deleted_at IS NULL" json:"code"`
	Name               string              `gorm:"size:150;not null;index" json:"name"`
	AssetNumber        string              `gorm:"size:100;uniqueIndex:idx_devices_asset_active,where:deleted_at IS NULL AND asset_number <> ''" json:"assetNumber,omitempty"`
	SerialNumber       string              `gorm:"size:120;index" json:"serialNumber,omitempty"`
	Manufacturer       string              `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber        string              `gorm:"size:100" json:"modelNumber,omitempty"`
	Specification      string              `gorm:"size:500" json:"specification,omitempty"`
	FirmwareVersion    string              `gorm:"size:100" json:"firmwareVersion,omitempty"`
	PurchaseBatch      string              `gorm:"size:100" json:"purchaseBatch,omitempty"`
	WarrantyExpiresAt  *time.Time          `json:"warrantyExpiresAt,omitempty"`
	Organization       string              `gorm:"size:150" json:"organization,omitempty"`
	Manager            string              `gorm:"size:100" json:"manager,omitempty"`
	Contact            string              `gorm:"size:100" json:"contact,omitempty"`
	BusinessSystem     string              `gorm:"size:150;index" json:"businessSystem,omitempty"`
	ApplicationName    string              `gorm:"size:150" json:"applicationName,omitempty"`
	LifecycleStatus    string              `gorm:"size:30;not null;default:WAITING_RACK;index" json:"lifecycleStatus"`
	HeightU            int                 `gorm:"not null;default:1" json:"heightU"`
	WidthMm            *int                `json:"widthMm,omitempty"`
	DepthMm            *int                `json:"depthMm,omitempty"`
	HeightMm           *int                `json:"heightMm,omitempty"`
	WeightKg           *float64            `json:"weightKg,omitempty"`
	RatedPowerW        *float64            `json:"ratedPowerW,omitempty"`
	PeakPowerW         *float64            `json:"peakPowerW,omitempty"`
	InputVoltage       *float64            `json:"inputVoltage,omitempty"`
	DualPowerRequired  bool                `gorm:"not null;default:false" json:"dualPowerRequired"`
	ManagementIP       string              `gorm:"size:64;index" json:"managementIp,omitempty"`
	BusinessIP         string              `gorm:"size:64;index" json:"businessIp,omitempty"`
	MACAddress         string              `gorm:"size:64;index" json:"macAddress,omitempty"`
	ManagementProtocol string              `gorm:"size:100" json:"managementProtocol,omitempty"`
	MonitoringStatus   string              `gorm:"size:50" json:"monitoringStatus,omitempty"`
	ExternalQRCode     string              `gorm:"size:255;index" json:"externalQrCode,omitempty"`
	ExternalQRCodeURL  string              `gorm:"size:500" json:"externalQrCodeUrl,omitempty"`
	Tags               string              `gorm:"size:500" json:"tags,omitempty"`
	Remarks            string              `gorm:"type:text" json:"remarks,omitempty"`
	CurrentPosition    *RackDevicePosition `gorm:"foreignKey:DeviceID" json:"currentPosition,omitempty"`
}

// RackDevicePosition is the single active physical position of a device.
type RackDevicePosition struct {
	BaseModel
	DeviceID     uuid.UUID  `gorm:"type:uuid;not null;index;uniqueIndex:idx_rack_device_positions_device_active,where:deleted_at IS NULL" json:"deviceId"`
	RackID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"rackId"`
	RoomID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"roomId"`
	DataCenterID uuid.UUID  `gorm:"type:uuid;not null;index" json:"dataCenterId"`
	StartU       int        `gorm:"not null" json:"startU"`
	HeightU      int        `gorm:"not null" json:"heightU"`
	EndU         int        `gorm:"not null" json:"endU"`
	Orientation  string     `gorm:"size:20;not null;default:NORMAL" json:"orientation"`
	InstalledAt  time.Time  `gorm:"not null" json:"installedAt"`
	InstalledBy  *uuid.UUID `gorm:"type:uuid;index" json:"installedBy,omitempty"`
	Reason       string     `gorm:"size:500" json:"reason,omitempty"`
	Device       Device     `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	Rack         Rack       `gorm:"foreignKey:RackID" json:"rack,omitempty"`
}

// RackUOccupancy materializes unified U ranges so PostgreSQL can enforce non-overlap.
type RackUOccupancy struct {
	BaseModel
	PositionID uuid.UUID `gorm:"type:uuid;not null;index" json:"positionId"`
	DeviceID   uuid.UUID `gorm:"type:uuid;not null;index" json:"deviceId"`
	RackID     uuid.UUID `gorm:"type:uuid;not null;index" json:"rackId"`
	StartU     int       `gorm:"not null" json:"startU"`
	EndU       int       `gorm:"not null" json:"endU"`
}

// DevicePositionSnapshot is retained in history and approval records.
type DevicePositionSnapshot struct {
	PositionID   uuid.UUID `json:"positionId,omitempty"`
	RackID       uuid.UUID `json:"rackId,omitempty"`
	DataCenterID uuid.UUID `json:"dataCenterId,omitempty"`
	RoomID       uuid.UUID `json:"roomId,omitempty"`
	StartU       int       `json:"startU,omitempty"`
	HeightU      int       `json:"heightU,omitempty"`
	EndU         int       `json:"endU,omitempty"`
	Orientation  string    `json:"orientation,omitempty"`
}

// DevicePositionHistory records complete assign, move, and decommission facts.
type DevicePositionHistory struct {
	BaseModel
	DeviceID     uuid.UUID               `gorm:"type:uuid;not null;index" json:"deviceId"`
	Operation    string                  `gorm:"size:30;not null;index" json:"operation"`
	FromPosition *DevicePositionSnapshot `gorm:"type:jsonb;serializer:json" json:"fromPosition,omitempty"`
	ToPosition   *DevicePositionSnapshot `gorm:"type:jsonb;serializer:json" json:"toPosition,omitempty"`
	Reason       string                  `gorm:"size:500" json:"reason,omitempty"`
	ActorID      *uuid.UUID              `gorm:"type:uuid;index" json:"actorId,omitempty"`
	RequestID    string                  `gorm:"size:100;index" json:"requestId,omitempty"`
}

// OperationPolicy stores the single-level approval feature switches.
type OperationPolicy struct {
	DecommissionApprovalEnabled bool `gorm:"not null;default:true" json:"decommissionApprovalEnabled"`
	BaseModel
	AssignApprovalEnabled bool   `gorm:"not null;default:false" json:"assignApprovalEnabled"`
	MoveApprovalEnabled   bool   `gorm:"not null;default:false" json:"moveApprovalEnabled"`
	ApprovalMode          string `gorm:"size:20;not null;default:SINGLE" json:"approvalMode"`
	DefaultApproverRole   string `gorm:"size:100;not null;default:system_admin" json:"defaultApproverRole"`
}

// ApprovalRecord stores a pending assign, move or decommission request. Final validation is repeated on approval.
type ApprovalRecord struct {
	BaseModel
	DeviceID               uuid.UUID               `gorm:"type:uuid;not null;index" json:"deviceId"`
	Operation              string                  `gorm:"size:30;not null;index" json:"operation"`
	Status                 string                  `gorm:"size:20;not null;default:PENDING;index" json:"status"`
	RequestedBy            *uuid.UUID              `gorm:"type:uuid;index" json:"requestedBy,omitempty"`
	RequestedAt            time.Time               `gorm:"not null" json:"requestedAt"`
	RequestedDeviceVersion uint                    `gorm:"not null" json:"requestedDeviceVersion"`
	SourcePosition         *DevicePositionSnapshot `gorm:"type:jsonb;serializer:json" json:"sourcePosition,omitempty"`
	TargetRackID           uuid.UUID               `gorm:"type:uuid;not null;index" json:"targetRackId"`
	TargetStartU           int                     `gorm:"not null" json:"targetStartU"`
	TargetHeightU          int                     `gorm:"not null" json:"targetHeightU"`
	TargetOrientation      string                  `gorm:"size:20;not null;default:NORMAL" json:"targetOrientation"`
	Reason                 string                  `gorm:"size:500" json:"reason,omitempty"`
	DecidedBy              *uuid.UUID              `gorm:"type:uuid;index" json:"decidedBy,omitempty"`
	DecidedAt              *time.Time              `json:"decidedAt,omitempty"`
	DecisionComment        string                  `gorm:"size:500" json:"decisionComment,omitempty"`
	Device                 Device                  `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}
