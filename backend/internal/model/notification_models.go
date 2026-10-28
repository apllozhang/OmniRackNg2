package model

import "time"

// One global policy; secrets are ciphertext and never serialized to API clients.
type NotificationSettings struct {
	ID                      uint      `gorm:"primaryKey" json:"-"`
	Version                 uint      `gorm:"not null" json:"version"`
	EmailEnabled            bool      `json:"emailEnabled"`
	SMTPHost                string    `gorm:"size:255" json:"smtpHost"`
	SMTPPort                int       `json:"smtpPort"`
	SMTPSecurity            string    `gorm:"size:20" json:"smtpSecurity"`
	SMTPUsername            string    `gorm:"size:255" json:"smtpUsername"`
	SMTPPasswordCipher      string    `gorm:"type:text" json:"-"`
	EmailFrom               string    `gorm:"size:255" json:"emailFrom"`
	EmailRecipients         []string  `gorm:"type:jsonb;serializer:json" json:"emailRecipients"`
	ApprovalEmailEnabled    bool      `json:"-"`
	ApprovalWeComEnabled    bool      `json:"-"`
	ApprovalEmailRecipients []string  `gorm:"type:jsonb;serializer:json" json:"-"`
	ApprovalBaseURL         string    `gorm:"size:512" json:"-"`
	WeComEnabled            bool      `json:"weComEnabled"`
	WeComWebhookCipher      string    `gorm:"type:text" json:"-"`
	WarrantyEnabled         bool      `json:"warrantyEnabled"`
	RackUEnabled            bool      `json:"rackUEnabled"`
	RackWarningPercent      int       `json:"rackWarningPercent"`
	RackCriticalPercent     int       `json:"rackCriticalPercent"`
	RepeatHours             int       `json:"repeatHours"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

func DefaultNotificationSettings() NotificationSettings {
	return NotificationSettings{SMTPPort: 587, SMTPSecurity: "STARTTLS", EmailRecipients: []string{}, WarrantyEnabled: true, RackUEnabled: true, RackWarningPercent: 70, RackCriticalPercent: 90, RepeatHours: 24}
}

type NotificationDelivery struct {
	BaseModel
	Channel      string   `gorm:"size:20;index" json:"channel"`
	Fingerprint  string   `gorm:"size:64;index" json:"-"`
	Status       string   `gorm:"size:20" json:"status"`
	AlertCount   int      `json:"alertCount"`
	Summary      string   `gorm:"size:255" json:"summary"`
	ErrorMessage string   `gorm:"size:255" json:"errorMessage"`
	Test         bool     `json:"test"`
	Recipients   []string `gorm:"type:jsonb;serializer:json" json:"recipients,omitempty"`
	MessageID    string   `gorm:"size:255" json:"messageId,omitempty"`
}
