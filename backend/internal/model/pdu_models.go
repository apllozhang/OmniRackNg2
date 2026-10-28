package model

import (
	"github.com/google/uuid"
	"time"
)

const (
	PDUStatusActive          = "ACTIVE"
	PDUStatusDisabled        = "DISABLED"
	PDUSocketStatusAvailable = "AVAILABLE"
	PDUSocketStatusConnected = "CONNECTED"
	PDUSocketStatusDisabled  = "DISABLED"
	PDUSocketStandardCN      = "CN"
	PDUSocketStandardEU      = "EU"
	PDUConnectionPrimary     = "PRIMARY"
	PDUConnectionRedundant   = "REDUNDANT"
)

// PDU represents an optional power distribution unit installed in a rack.
type PDU struct {
	BaseModel
	RackID        uuid.UUID   `gorm:"type:uuid;not null;index" json:"rackId"`
	Code          string      `gorm:"size:80;not null;uniqueIndex:idx_pdus_code_active,where:deleted_at IS NULL" json:"code"`
	Name          string      `gorm:"size:150;not null" json:"name"`
	Manufacturer  string      `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber   string      `gorm:"size:100" json:"modelNumber,omitempty"`
	SerialNumber  string      `gorm:"size:120;index" json:"serialNumber,omitempty"`
	InputVoltage  *float64    `json:"inputVoltage,omitempty"`
	RatedPowerW   *float64    `json:"ratedPowerW,omitempty"`
	RatedCurrentA *float64    `json:"ratedCurrentA,omitempty"`
	Status        string      `gorm:"size:20;not null;default:ACTIVE;index" json:"status"`
	Remarks       string      `gorm:"type:text" json:"remarks,omitempty"`
	Rack          Rack        `gorm:"foreignKey:RackID" json:"rack,omitempty"`
	Sockets       []PDUSocket `gorm:"foreignKey:PDUID" json:"sockets,omitempty"`
}

// PDUSocket is a separately addressable outlet. Standard distinguishes Chinese
// national sockets from European sockets; amperage is intentionally optional at
// the PDU level but explicit when an outlet is configured.
type PDUSocket struct {
	BaseModel
	PDUID      uuid.UUID      `gorm:"type:uuid;not null;index;uniqueIndex:idx_pdu_sockets_number_active,where:deleted_at IS NULL" json:"pduId"`
	SocketNo   int            `gorm:"not null;uniqueIndex:idx_pdu_sockets_number_active,where:deleted_at IS NULL" json:"socketNo"`
	Standard   string         `gorm:"size:10;not null" json:"standard"`
	AmperageA  int            `gorm:"not null" json:"amperageA"`
	Label      string         `gorm:"size:100" json:"label,omitempty"`
	Status     string         `gorm:"size:20;not null;default:AVAILABLE" json:"status"`
	PDU        PDU            `gorm:"-" json:"pdu,omitempty"`
	Connection *PDUConnection `gorm:"-" json:"connection,omitempty"`
}

// PDUConnection links one device power input to one PDU socket. A device may
// have two connections for dual-power operation.
type PDUConnection struct {
	BaseModel
	SocketID       uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_pdu_connections_socket_active,where:deleted_at IS NULL" json:"socketId"`
	DeviceID       uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_pdu_connections_device_role_active,where:deleted_at IS NULL" json:"deviceId"`
	PowerW         *float64   `json:"powerW,omitempty"`
	Circuit        string     `gorm:"size:100" json:"circuit,omitempty"`
	RedundancyRole string     `gorm:"size:20;not null;default:PRIMARY;uniqueIndex:idx_pdu_connections_device_role_active,where:deleted_at IS NULL" json:"redundancyRole"`
	ConnectedAt    time.Time  `gorm:"not null" json:"connectedAt"`
	ConnectedBy    *uuid.UUID `gorm:"type:uuid;index" json:"connectedBy,omitempty"`
	Socket         PDUSocket  `gorm:"-" json:"socket,omitempty"`
	Device         Device     `gorm:"-" json:"device,omitempty"`
}
