package model

import (
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Version   uint           `gorm:"not null;default:1" json:"version"`
}

func (m *BaseModel) BeforeCreate(*gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	if m.Version == 0 {
		m.Version = 1
	}
	return nil
}

type User struct {
	BaseModel
	Username     string     `gorm:"size:100;not null;uniqueIndex" json:"username"`
	DisplayName  string     `gorm:"size:100;not null" json:"displayName"`
	Email        string     `gorm:"size:255" json:"email,omitempty"`
	PasswordHash string     `gorm:"size:255;not null" json:"-"`
	AuthSource   string     `gorm:"size:20;not null;default:local" json:"authSource"`
	Enabled      bool       `gorm:"not null;default:true" json:"enabled"`
	FailedLogins int        `gorm:"not null;default:0" json:"-"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	Roles        []Role     `gorm:"many2many:user_roles;" json:"roles,omitempty"`
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
}

type Role struct {
	BaseModel
	Code        string       `gorm:"size:50;not null;uniqueIndex" json:"code"`
	Name        string       `gorm:"size:100;not null" json:"name"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

type Permission struct {
	BaseModel
	Code string `gorm:"size:100;not null;uniqueIndex" json:"code"`
	Name string `gorm:"size:100;not null" json:"name"`
}

type AuditLog struct {
	BaseModel
	UserID       *uuid.UUID `gorm:"type:uuid;index" json:"userId,omitempty"`
	Action       string     `gorm:"size:100;not null;index" json:"action"`
	ResourceType string     `gorm:"size:100;not null;index" json:"resourceType"`
	ResourceID   *uuid.UUID `gorm:"type:uuid;index" json:"resourceId,omitempty"`
	RequestID    string     `gorm:"size:100;index" json:"requestId"`
	BeforeJSON   string     `gorm:"type:jsonb" json:"before,omitempty"`
	AfterJSON    string     `gorm:"type:jsonb" json:"after,omitempty"`
	Result       string     `gorm:"size:20;not null" json:"result"`
	ErrorCode    string     `gorm:"size:100" json:"errorCode,omitempty"`
	Source       string     `gorm:"size:50" json:"source"`
}
