package model

import (
	"github.com/google/uuid"
	"time"
)

type PCCustody struct {
	UsageStatus string     `json:"usageStatus"`
	Assignee    string     `json:"assignee"`
	Department  string     `json:"department"`
	SiteID      *uuid.UUID `json:"siteId"`
	SiteName    string     `json:"siteName"`
	Position    string     `json:"position"`
}

// Immutable business history; no update/delete endpoint is provided.
type PCHandover struct {
	BaseModel
	DeviceID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"deviceId"`
	Device          Device     `gorm:"foreignKey:DeviceID" json:"-"`
	DeviceCode      string     `gorm:"size:80" json:"deviceCode"`
	DeviceName      string     `gorm:"size:150" json:"deviceName"`
	SerialNumber    string     `gorm:"size:120" json:"serialNumber"`
	Operation       string     `gorm:"size:20;not null" json:"operation"`
	Before          PCCustody  `gorm:"type:jsonb;serializer:json" json:"before"`
	After           PCCustody  `gorm:"type:jsonb;serializer:json" json:"after"`
	Condition       string     `gorm:"size:20" json:"condition"`
	Note            string     `gorm:"size:500" json:"note"`
	ConfirmedBy     *uuid.UUID `gorm:"type:uuid" json:"confirmedBy"`
	ConfirmedByName string     `gorm:"size:160" json:"confirmedByName"`
	ConfirmedAt     time.Time  `json:"confirmedAt"`
}
