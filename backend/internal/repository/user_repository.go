package repository

import (
	"context"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserQuery struct {
	Search     string
	Enabled    *bool
	AuthSource string
}

type UserStore interface {
	ListUsers(context.Context, UserQuery) ([]model.User, error)
	GetUser(context.Context, uuid.UUID) (model.User, error)
	CreateUser(context.Context, *model.User, []string) error
	UpdateUser(context.Context, *model.User, uint, []string) error
	CountEnabledAdmins(context.Context, uuid.UUID) (int64, error)
	DeleteUser(context.Context, uuid.UUID, uint) error
	ListRoles(context.Context) ([]model.Role, error)
	GetRoleByCode(context.Context, string) (model.Role, error)
	GetLDAPConfig(context.Context) (model.LDAPConfig, error)
	SaveLDAPConfig(context.Context, *model.LDAPConfig) error
}

type GormUserStore struct{ db *gorm.DB }

func NewUserStore(db *gorm.DB) *GormUserStore { return &GormUserStore{db: db} }

func (s *GormUserStore) ListUsers(ctx context.Context, q UserQuery) ([]model.User, error) {
	var items []model.User
	query := s.db.WithContext(ctx).Preload("Roles").Order("username ASC")
	if q.Search != "" {
		like := "%" + q.Search + "%"
		query = query.Where("username ILIKE ? OR display_name ILIKE ?", like, like)
	}
	if q.Enabled != nil {
		query = query.Where("enabled = ?", *q.Enabled)
	}
	if q.AuthSource != "" {
		query = query.Where("auth_source = ?", q.AuthSource)
	}
	return items, query.Find(&items).Error
}
func (s *GormUserStore) GetUser(ctx context.Context, id uuid.UUID) (model.User, error) {
	var item model.User
	err := s.db.WithContext(ctx).Preload("Roles").First(&item, "id = ?", id).Error
	return item, err
}
func (s *GormUserStore) roleSet(ctx context.Context, codes []string) ([]model.Role, error) {
	roles := make([]model.Role, 0, len(codes))
	for _, code := range codes {
		var role model.Role
		if err := s.db.WithContext(ctx).Where("code = ?", code).First(&role).Error; err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, nil
}
func (s *GormUserStore) CreateUser(ctx context.Context, item *model.User, roleCodes []string) error {
	roles, err := s.roleSet(ctx, roleCodes)
	if err != nil {
		return err
	}
	item.Roles = roles
	return s.db.WithContext(ctx).Create(item).Error
}
func (s *GormUserStore) UpdateUser(ctx context.Context, item *model.User, expectedVersion uint, roleCodes []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.User{}).Where("id = ? AND version = ?", item.ID, expectedVersion).Updates(map[string]interface{}{
			"display_name": item.DisplayName, "email": item.Email, "auth_source": item.AuthSource, "enabled": item.Enabled, "password_hash": item.PasswordHash, "version": gorm.Expr("version + 1"),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		roles, err := (&GormUserStore{db: tx}).roleSet(ctx, roleCodes)
		if err != nil {
			return err
		}
		if err := tx.Model(item).Association("Roles").Replace(roles); err != nil {
			return err
		}
		return nil
	})
}
func (s *GormUserStore) CountEnabledAdmins(ctx context.Context, except uuid.UUID) (int64, error) {
	var count int64
	q := s.db.WithContext(ctx).Model(&model.User{}).Joins("JOIN user_roles ON user_roles.user_id = users.id").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("users.enabled = ? AND roles.code = ?", true, "system_admin")
	if except != uuid.Nil {
		q = q.Where("users.id <> ?", except)
	}
	return count, q.Count(&count).Error
}
func (s *GormUserStore) DeleteUser(ctx context.Context, id uuid.UUID, expectedVersion uint) error {
	result := s.db.WithContext(ctx).Where("id = ? AND version = ?", id, expectedVersion).Delete(&model.User{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s *GormUserStore) ListRoles(ctx context.Context) ([]model.Role, error) {
	var items []model.Role
	err := s.db.WithContext(ctx).Order("name ASC").Find(&items).Error
	return items, err
}
func (s *GormUserStore) GetRoleByCode(ctx context.Context, code string) (model.Role, error) {
	var item model.Role
	err := s.db.WithContext(ctx).Where("code = ?", code).First(&item).Error
	return item, err
}
func (s *GormUserStore) GetLDAPConfig(ctx context.Context) (model.LDAPConfig, error) {
	var item model.LDAPConfig
	err := s.db.WithContext(ctx).Order("created_at ASC").First(&item).Error
	return item, err
}
func (s *GormUserStore) SaveLDAPConfig(ctx context.Context, item *model.LDAPConfig) error {
	if item.ID == uuid.Nil {
		return s.db.WithContext(ctx).Create(item).Error
	}
	return s.db.WithContext(ctx).Save(item).Error
}
