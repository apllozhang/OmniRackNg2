package model

import (
	"github.com/google/uuid"
	"time"
)

// AssetSite also represents office locations; RoomID optionally links the original room tree.
type AssetSite struct {
	BaseModel
	Name    string     `gorm:"size:160;not null;uniqueIndex" json:"name"`
	Region  string     `gorm:"size:120;not null" json:"region"`
	Kind    string     `gorm:"size:20;not null" json:"kind"`
	RoomID  *uuid.UUID `gorm:"type:uuid;index" json:"roomId,omitempty"`
	Room    *Room      `gorm:"foreignKey:RoomID" json:"-"`
	Address string     `gorm:"size:300" json:"address"`
}

// AssetProfile extends, rather than duplicates, the existing device identity and rack position.
type AssetProfile struct {
	BaseModel
	DeviceID        uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"deviceId"`
	Device          Device     `gorm:"foreignKey:DeviceID" json:"-"`
	SiteID          *uuid.UUID `gorm:"type:uuid;index" json:"siteId,omitempty"`
	Site            *AssetSite `gorm:"foreignKey:SiteID" json:"site,omitempty"`
	Position        string     `gorm:"size:160" json:"position"`
	UsageStatus     string     `gorm:"size:20;not null;default:IDLE" json:"usageStatus"`
	Assignee        string     `gorm:"size:120" json:"assignee"`
	Department      string     `gorm:"size:120" json:"department"`
	CPU             string     `gorm:"size:200" json:"cpu"`
	MemoryGB        int        `gorm:"not null;default:0" json:"memoryGb"`
	Disk            string     `gorm:"size:500" json:"disk"`
	OperatingSystem string     `gorm:"size:160" json:"operatingSystem"`
	GPU             string     `gorm:"size:160" json:"gpu"`
	RAID            string     `gorm:"size:160" json:"raid"`
	Ports           string     `gorm:"size:200" json:"ports"`
	Environment     string     `gorm:"size:30" json:"environment"`
	Importance      string     `gorm:"size:30" json:"importance"`
	NoWarranty      bool       `gorm:"not null;default:false" json:"noWarranty"`
}

type AssetCoverage struct {
	BaseModel
	DeviceID     uuid.UUID `gorm:"type:uuid;not null;index" json:"deviceId"`
	Device       Device    `gorm:"foreignKey:DeviceID" json:"-"`
	Kind         string    `gorm:"size:30;not null" json:"kind"`
	Provider     string    `gorm:"size:160;not null" json:"provider"`
	StartsOn     time.Time `gorm:"type:date;not null" json:"startsOn"`
	EndsOn       time.Time `gorm:"type:date;not null" json:"endsOn"`
	Contact      string    `gorm:"size:200" json:"contact"`
	ServiceLevel string    `gorm:"size:300" json:"serviceLevel"`
}

type SpareSpec struct {
	BaseModel
	Name          string `gorm:"size:160;not null" json:"name"`
	Category      string `gorm:"size:40;not null" json:"category"`
	Manufacturer  string `gorm:"size:120" json:"manufacturer"`
	ModelNumber   string `gorm:"size:160" json:"modelNumber"`
	PartNumber    string `gorm:"size:120;not null;uniqueIndex" json:"partNumber"`
	Specification string `gorm:"size:500;not null" json:"specification"`
	Tracking      string `gorm:"size:20;not null" json:"tracking"`
	Unit          string `gorm:"size:20;not null;default:件" json:"unit"`
	Compatibility string `gorm:"size:500" json:"compatibility"`
}

type SpareStock struct {
	BaseModel
	SpecID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"specId"`
	Spec              SpareSpec  `gorm:"foreignKey:SpecID" json:"spec"`
	SiteID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"siteId"`
	Site              AssetSite  `gorm:"foreignKey:SiteID" json:"site"`
	Bin               string     `gorm:"size:160" json:"bin"`
	SerialNumber      string     `gorm:"size:120;uniqueIndex:idx_spare_sn,where:deleted_at IS NULL AND serial_number <> ''" json:"serialNumber"`
	Batch             string     `gorm:"size:120" json:"batch"`
	Notes             string     `gorm:"size:1000" json:"notes"`
	Quantity          int        `gorm:"not null;default:0;check:spare_quantity_nonnegative,quantity >= 0" json:"quantity"`
	State             string     `gorm:"size:20;not null;default:AVAILABLE" json:"state"`
	InstalledDeviceID *uuid.UUID `gorm:"type:uuid;index" json:"installedDeviceId,omitempty"`
	InstalledDevice   *Device    `gorm:"foreignKey:InstalledDeviceID" json:"installedDevice,omitempty"`
	DestinationSiteID *uuid.UUID `gorm:"type:uuid" json:"destinationSiteId,omitempty"`
	WarrantyExpiresAt *time.Time `gorm:"type:date" json:"warrantyExpiresAt,omitempty"`
}

type SpareMovement struct {
	BaseModel
	SubmissionID       uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"submissionId"`
	StockID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"stockId"`
	Stock              SpareStock `gorm:"foreignKey:StockID" json:"stock,omitempty"`
	DestinationStockID *uuid.UUID `gorm:"type:uuid" json:"destinationStockId,omitempty"`
	Operation          string     `gorm:"size:30;not null" json:"operation"`
	Quantity           int        `gorm:"not null" json:"quantity"`
	BeforeQuantity     int        `gorm:"not null" json:"beforeQuantity"`
	AfterQuantity      int        `gorm:"not null" json:"afterQuantity"`
	BeforeState        string     `gorm:"size:20" json:"beforeState"`
	AfterState         string     `gorm:"size:20" json:"afterState"`
	DeviceID           *uuid.UUID `gorm:"type:uuid" json:"deviceId,omitempty"`
	SourceSiteID       uuid.UUID  `gorm:"type:uuid;not null" json:"sourceSiteId"`
	DestinationSiteID  *uuid.UUID `gorm:"type:uuid" json:"destinationSiteId,omitempty"`
	ActorID            uuid.UUID  `gorm:"type:uuid;not null" json:"actorId"`
	Reason             string     `gorm:"type:text;not null" json:"reason"`
}

type SpareMinimum struct {
	BaseModel
	SpecID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_spare_minimum" json:"specId"`
	Spec    SpareSpec `gorm:"foreignKey:SpecID" json:"spec"`
	SiteID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_spare_minimum" json:"siteId"`
	Site    AssetSite `gorm:"foreignKey:SiteID" json:"site"`
	Minimum int       `gorm:"not null;check:spare_minimum_nonnegative,minimum >= 0" json:"minimum"`
}

type AssetCheckTask struct {
	BaseModel
	Name       string     `gorm:"size:160;not null" json:"name"`
	SiteID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"siteId"`
	Site       AssetSite  `gorm:"foreignKey:SiteID" json:"site"`
	AssigneeID uuid.UUID  `gorm:"type:uuid;not null" json:"assigneeId"`
	State      string     `gorm:"size:20;not null;default:ACTIVE" json:"state"`
	CreatedBy  uuid.UUID  `gorm:"type:uuid;not null" json:"createdBy"`
	ClosedAt   *time.Time `json:"closedAt,omitempty"`
}

type AssetCheckLine struct {
	BaseModel
	TaskID                uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_asset_check_target" json:"taskId"`
	Task                  AssetCheckTask `gorm:"foreignKey:TaskID" json:"-"`
	Kind                  string         `gorm:"size:20;not null;uniqueIndex:idx_asset_check_target" json:"kind"`
	TargetID              uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:idx_asset_check_target" json:"targetId"`
	Code                  string         `gorm:"size:120" json:"code"`
	Name                  string         `gorm:"size:160" json:"name"`
	SerialNumber          string         `gorm:"size:120" json:"serialNumber"`
	TargetVersion         uint           `gorm:"not null" json:"targetVersion"`
	Expected              int            `gorm:"not null" json:"expected"`
	Actual                *int           `json:"actual,omitempty"`
	Snapshot              string         `gorm:"type:jsonb;not null" json:"snapshot"`
	Result                string         `gorm:"size:30;not null;default:UNSCANNED" json:"result"`
	ConfigurationVerified bool           `gorm:"not null;default:false" json:"configurationVerified"`
	Note                  string         `gorm:"type:text" json:"note"`
	ScannedBy             *uuid.UUID     `gorm:"type:uuid" json:"scannedBy,omitempty"`
	ScannedAt             *time.Time     `json:"scannedAt,omitempty"`
	ResolvedBy            *uuid.UUID     `gorm:"type:uuid" json:"resolvedBy,omitempty"`
	Resolution            string         `gorm:"type:text" json:"resolution"`
}

type AssetCheckDiscovery struct {
	BaseModel
	TaskID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"taskId"`
	Code       string     `gorm:"size:500;not null" json:"code"`
	Note       string     `gorm:"type:text;not null" json:"note"`
	ActorID    uuid.UUID  `gorm:"type:uuid;not null" json:"actorId"`
	ResolvedBy *uuid.UUID `gorm:"type:uuid" json:"resolvedBy,omitempty"`
	Resolution string     `gorm:"type:text" json:"resolution"`
}

type AssetReminder struct {
	BaseModel
	OccurrenceAt    time.Time  `json:"occurrenceAt"`
	Key             string     `gorm:"size:160;not null;uniqueIndex" json:"key"`
	Kind            string     `gorm:"size:30;not null" json:"kind"`
	DeviceID        *uuid.UUID `gorm:"type:uuid;index" json:"deviceId,omitempty"`
	StockID         *uuid.UUID `gorm:"type:uuid;index" json:"stockId,omitempty"`
	Title           string     `gorm:"size:240;not null" json:"title"`
	Detail          string     `gorm:"type:text" json:"detail"`
	Severity        string     `gorm:"size:20;not null" json:"severity"`
	Active          bool       `gorm:"not null;default:true" json:"active"`
	AcknowledgedBy  *uuid.UUID `gorm:"type:uuid" json:"acknowledgedBy,omitempty"`
	Acknowledgement string     `gorm:"type:text" json:"acknowledgement"`
}
