package repository

import (
	"context"
	"errors"
	"time"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RackTemplateReader is the minimal dependency required when a rack is
// instantiated from an immutable template version.
type RackTemplateReader interface {
	GetRackTemplateVersion(context.Context, uuid.UUID) (model.RackTemplateVersion, error)
}

// RackTemplateStore defines persistence operations for template metadata and
// immutable template versions.
type RackTemplateStore interface {
	WithinTemplateTransaction(context.Context, func(RackTemplateStore) error) error
	ListRackTemplates(context.Context) ([]model.RackTemplate, error)
	GetRackTemplate(context.Context, uuid.UUID) (model.RackTemplate, error)
	GetRackTemplateVersion(context.Context, uuid.UUID) (model.RackTemplateVersion, error)
	CreateRackTemplate(context.Context, *model.RackTemplate) error
	UpdateRackTemplate(context.Context, *model.RackTemplate, uint) error
	AdvanceRackTemplateRevision(context.Context, uuid.UUID, uint, int) error
	DeleteRackTemplate(context.Context, uuid.UUID, uint) error
	CreateRackTemplateVersion(context.Context, *model.RackTemplateVersion) error
	CreateAuditLog(context.Context, *model.AuditLog) error
}

// GormRackTemplateStore implements RackTemplateStore using GORM.
type GormRackTemplateStore struct{ db *gorm.DB }

func NewRackTemplateStore(db *gorm.DB) *GormRackTemplateStore {
	return &GormRackTemplateStore{db: db}
}

func (s *GormRackTemplateStore) WithinTemplateTransaction(ctx context.Context, fn func(RackTemplateStore) error) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(&GormRackTemplateStore{db: tx})
	})
}

func (s *GormRackTemplateStore) ListRackTemplates(ctx context.Context) ([]model.RackTemplate, error) {
	var items []model.RackTemplate
	err := s.db.WithContext(ctx).
		Order("is_system DESC, code ASC").
		Preload("Versions", func(db *gorm.DB) *gorm.DB { return db.Order("revision DESC") }).
		Find(&items).Error
	return items, err
}

func (s *GormRackTemplateStore) GetRackTemplate(ctx context.Context, id uuid.UUID) (model.RackTemplate, error) {
	var item model.RackTemplate
	err := s.db.WithContext(ctx).
		Preload("Versions", func(db *gorm.DB) *gorm.DB { return db.Order("revision DESC") }).
		First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormRackTemplateStore) GetRackTemplateVersion(ctx context.Context, id uuid.UUID) (model.RackTemplateVersion, error) {
	var item model.RackTemplateVersion
	err := s.db.WithContext(ctx).Preload("Template").First(&item, "id = ?", id).Error
	return item, err
}

func (s *GormRackTemplateStore) CreateRackTemplate(ctx context.Context, item *model.RackTemplate) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormRackTemplateStore) UpdateRackTemplate(ctx context.Context, item *model.RackTemplate, expectedVersion uint) error {
	updates := map[string]interface{}{
		"code": item.Code, "name": item.Name, "description": item.Description,
		"status": item.Status, "remarks": item.Remarks,
		"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC(),
	}
	result := s.db.WithContext(ctx).Model(&model.RackTemplate{}).
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

func (s *GormRackTemplateStore) AdvanceRackTemplateRevision(ctx context.Context, id uuid.UUID, expectedVersion uint, revision int) error {
	result := s.db.WithContext(ctx).Model(&model.RackTemplate{}).
		Where("id = ? AND version = ?", id, expectedVersion).
		Updates(map[string]interface{}{
			"current_revision": revision,
			"version":          gorm.Expr("version + 1"),
			"updated_at":       time.Now().UTC(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (s *GormRackTemplateStore) DeleteRackTemplate(ctx context.Context, id uuid.UUID, expectedVersion uint) error {
	result := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, expectedVersion).Delete(&model.RackTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrVersionConflict
	}
	return nil
}

func (s *GormRackTemplateStore) CreateRackTemplateVersion(ctx context.Context, item *model.RackTemplateVersion) error {
	return s.db.WithContext(ctx).Create(item).Error
}

func (s *GormRackTemplateStore) CreateAuditLog(ctx context.Context, item *model.AuditLog) error {
	return s.db.WithContext(ctx).Create(item).Error
}

// SeedRackTemplates guarantees that every installation has a stable built-in
// 42U template. It is idempotent and does not overwrite administrator changes.
func SeedRackTemplates(ctx context.Context, db *gorm.DB) error {
	var existing model.RackTemplate
	err := db.WithContext(ctx).Where("code = ?", "STANDARD-42U").First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		template := model.RackTemplate{
			Code: model.DefaultRackTemplateCode, Name: "标准 42U 机柜", Description: "系统内置的 42U 标准机柜模板",
			Status: model.RackTemplateStatusActive, IsSystem: true, CurrentRevision: 1,
		}
		if err := tx.Create(&template).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return nil
			}
			return err
		}
		version := model.RackTemplateVersion{
			TemplateID: template.ID, Revision: 1, Type: model.DefaultRackType, UHeight: model.DefaultRackUHeight,
			WidthMm: model.DefaultRackWidthMm, DepthMm: model.DefaultRackDepthMm, HeightMm: model.DefaultRackHeightMm,
			DualPower: false, InputCircuits: 0, PDUCount: 0,
			ChangeNote: "系统初始化版本",
		}
		return tx.Create(&version).Error
	})
}
