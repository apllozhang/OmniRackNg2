package model

import "time"

// Shared link generation policy; it does not change listeners or authentication.
type ScanAccessSettings struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	Version   uint      `gorm:"not null" json:"version"`
	BaseURL   string    `gorm:"size:512" json:"baseUrl"`
	UpdatedAt time.Time `json:"updatedAt"`
}
