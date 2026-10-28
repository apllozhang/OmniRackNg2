package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
)

var ErrLastEnabledAdmin = errors.New("cannot remove the last enabled administrator")
var ErrLDAPConfigVersion = errors.New("LDAP configuration version conflict")

// Every administrative account mutation takes the same transaction-scoped lock
// before checking the invariant. A service-level count alone is vulnerable to
// two administrators concurrently demoting or deleting each other.
const userAdministrationLockKey int64 = 0x4f4d4e4955534552
const ldapConfigurationLockKey int64 = userAdministrationLockKey + 1

func lockUserAdministration(tx *gorm.DB) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(?)", userAdministrationLockKey).Error
}

func requireAnotherEnabledAdmin(ctx context.Context, tx *gorm.DB, id uuid.UUID, expectedVersion uint) error {
	var count int64
	err := tx.Model(&model.User{}).
		Joins("JOIN user_roles ON user_roles.user_id = users.id").
		Joins("JOIN roles ON roles.id = user_roles.role_id").
		Where("users.id = ? AND users.version = ? AND users.enabled = ? AND roles.code = ? AND roles.deleted_at IS NULL", id, expectedVersion, true, "system_admin").Count(&count).Error
	if err != nil || count == 0 {
		return err
	}
	count, err = (&GormUserStore{db: tx}).CountEnabledAdmins(ctx, id)
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrLastEnabledAdmin
	}
	return nil
}

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
	enabled := item.Enabled
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		roles, err := (&GormUserStore{db: tx}).roleSet(ctx, roleCodes)
		if err != nil {
			return err
		}
		item.Roles = roles
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		// GORM fills default:true for a zero bool during Create. Persist an
		// explicitly disabled account before committing its creation.
		if !enabled {
			if err := tx.Model(&model.User{}).Where("id = ?", item.ID).UpdateColumn("enabled", false).Error; err != nil {
				return err
			}
			item.Enabled = false
		}
		return nil
	})
}
func (s *GormUserStore) UpdateUser(ctx context.Context, item *model.User, expectedVersion uint, roleCodes []string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUserAdministration(tx); err != nil {
			return err
		}
		keepsAdmin := false
		for _, code := range roleCodes {
			if code == "system_admin" {
				keepsAdmin = true
				break
			}
		}
		if !item.Enabled || !keepsAdmin {
			if err := requireAnotherEnabledAdmin(ctx, tx, item.ID, expectedVersion); err != nil {
				return err
			}
		}
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
	q := s.db.WithContext(ctx).Model(&model.User{}).Joins("JOIN user_roles ON user_roles.user_id = users.id").Joins("JOIN roles ON roles.id = user_roles.role_id").Where("users.enabled = ? AND roles.code = ? AND roles.deleted_at IS NULL", true, "system_admin")
	if except != uuid.Nil {
		q = q.Where("users.id <> ?", except)
	}
	return count, q.Count(&count).Error
}
func (s *GormUserStore) DeleteUser(ctx context.Context, id uuid.UUID, expectedVersion uint) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockUserAdministration(tx); err != nil {
			return err
		}
		if err := requireAnotherEnabledAdmin(ctx, tx, id, expectedVersion); err != nil {
			return err
		}
		result := tx.Where("id = ? AND version = ?", id, expectedVersion).Delete(&model.User{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
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
	err := s.db.WithContext(ctx).Order("created_at ASC, id ASC").First(&item).Error
	return item, err
}
func (s *GormUserStore) SaveLDAPConfig(ctx context.Context, item *model.LDAPConfig) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?)", ldapConfigurationLockKey).Error; err != nil {
			return err
		}
		var current model.LDAPConfig
		err := tx.Order("created_at ASC, id ASC").First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if item.ID != uuid.Nil || item.Version != 0 {
				return ErrLDAPConfigVersion
			}
			return tx.Create(item).Error
		}
		if err != nil {
			return err
		}
		if item.ID != current.ID || item.Version != current.Version {
			return ErrLDAPConfigVersion
		}
		candidate := *item
		candidate.Version = current.Version + 1
		// Unlike Save, a guarded Updates cannot silently insert a new singleton
		// if an existing configuration was removed or changed concurrently.
		result := tx.Model(&model.LDAPConfig{}).Where("id = ? AND version = ?", current.ID, current.Version).
			Select("*").Omit("id", "created_at", "deleted_at").Updates(&candidate)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLDAPConfigVersion
		}
		*item = candidate
		return nil
	})
}
