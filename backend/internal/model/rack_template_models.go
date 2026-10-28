package model

import "github.com/google/uuid"

const (
	RackTemplateStatusActive   = "ACTIVE"
	RackTemplateStatusDisabled = "DISABLED"
)

// RackTemplate is the mutable template identity and metadata. Physical parameters
// live in immutable RackTemplateVersion records.
type RackTemplate struct {
	BaseModel
	Code            string                `gorm:"size:50;not null;uniqueIndex:idx_rack_templates_code_active,where:deleted_at IS NULL" json:"code"`
	Name            string                `gorm:"size:150;not null;index" json:"name"`
	Description     string                `gorm:"type:text" json:"description,omitempty"`
	Status          string                `gorm:"size:20;not null;default:ACTIVE;index" json:"status"`
	IsSystem        bool                  `gorm:"not null;default:false" json:"isSystem"`
	CurrentRevision int                   `gorm:"not null;default:1" json:"currentRevision"`
	Remarks         string                `gorm:"type:text" json:"remarks,omitempty"`
	Versions        []RackTemplateVersion `gorm:"foreignKey:TemplateID" json:"versions,omitempty"`
}

// RackTemplateVersion is immutable after creation. Revision numbers increase
// independently for each template.
type RackTemplateVersion struct {
	BaseModel
	TemplateID     uuid.UUID    `gorm:"type:uuid;not null;index;uniqueIndex:idx_rack_template_revision_active,where:deleted_at IS NULL" json:"templateId"`
	Revision       int          `gorm:"not null;uniqueIndex:idx_rack_template_revision_active,where:deleted_at IS NULL" json:"revision"`
	Type           string       `gorm:"size:50;not null;default:STANDARD" json:"type"`
	Manufacturer   string       `gorm:"size:100" json:"manufacturer,omitempty"`
	ModelNumber    string       `gorm:"size:100" json:"modelNumber,omitempty"`
	UHeight        int          `gorm:"not null;default:42" json:"uHeight"`
	WidthMm        int          `gorm:"not null;default:600" json:"widthMm"`
	DepthMm        int          `gorm:"not null;default:1200" json:"depthMm"`
	HeightMm       int          `gorm:"not null;default:2000" json:"heightMm"`
	LoadCapacityKg *float64     `json:"loadCapacityKg,omitempty"`
	DualPower      bool         `gorm:"not null;default:false" json:"dualPower"`
	InputCircuits  int          `gorm:"not null;default:0" json:"inputCircuits"`
	RatedVoltage   *float64     `json:"ratedVoltage,omitempty"`
	RatedCurrent   *float64     `json:"ratedCurrent,omitempty"`
	RatedPowerKw   *float64     `json:"ratedPowerKw,omitempty"`
	PeakPowerKw    *float64     `json:"peakPowerKw,omitempty"`
	PDUCount       int          `gorm:"not null;default:0" json:"pduCount"`
	ChangeNote     string       `gorm:"size:500" json:"changeNote,omitempty"`
	CreatedBy      *uuid.UUID   `gorm:"type:uuid;index" json:"createdBy,omitempty"`
	Template       RackTemplate `gorm:"foreignKey:TemplateID" json:"-"`
}

// RackTemplateSnapshot is copied to a rack when it is created from a template.
// Later template revisions do not change existing rack instances.
type RackTemplateSnapshot struct {
	TemplateID     uuid.UUID `json:"templateId"`
	VersionID      uuid.UUID `json:"versionId"`
	Code           string    `json:"code"`
	Name           string    `json:"name"`
	Revision       int       `json:"revision"`
	Type           string    `json:"type"`
	Manufacturer   string    `json:"manufacturer,omitempty"`
	ModelNumber    string    `json:"modelNumber,omitempty"`
	UHeight        int       `json:"uHeight"`
	WidthMm        int       `json:"widthMm"`
	DepthMm        int       `json:"depthMm"`
	HeightMm       int       `json:"heightMm"`
	LoadCapacityKg *float64  `json:"loadCapacityKg,omitempty"`
	DualPower      bool      `json:"dualPower"`
	InputCircuits  int       `json:"inputCircuits"`
	RatedVoltage   *float64  `json:"ratedVoltage,omitempty"`
	RatedCurrent   *float64  `json:"ratedCurrent,omitempty"`
	RatedPowerKw   *float64  `json:"ratedPowerKw,omitempty"`
	PeakPowerKw    *float64  `json:"peakPowerKw,omitempty"`
	PDUCount       int       `json:"pduCount"`
}
