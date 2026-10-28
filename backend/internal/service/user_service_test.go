package service

import (
	"context"
	"testing"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type userServiceStoreStub struct {
	repository.UserStore
	user         model.User
	adminCount   int64
	updateCalled bool
	ldap         model.LDAPConfig
	role         model.Role
}

func (s *userServiceStoreStub) GetUser(context.Context, uuid.UUID) (model.User, error) {
	return s.user, nil
}
func (s *userServiceStoreStub) CountEnabledAdmins(context.Context, uuid.UUID) (int64, error) {
	return s.adminCount, nil
}
func (s *userServiceStoreStub) UpdateUser(context.Context, *model.User, uint, []string) error {
	s.updateCalled = true
	return nil
}
func (s *userServiceStoreStub) GetRoleByCode(context.Context, string) (model.Role, error) {
	return s.role, nil
}
func (s *userServiceStoreStub) GetLDAPConfig(context.Context) (model.LDAPConfig, error) {
	return s.ldap, gorm.ErrRecordNotFound
}
func (s *userServiceStoreStub) SaveLDAPConfig(_ context.Context, item *model.LDAPConfig) error {
	s.ldap = *item
	return nil
}

func TestUpdateCannotRemoveLastAdminByChangingRoles(t *testing.T) {
	id := uuid.New()
	store := &userServiceStoreStub{
		user:       model.User{BaseModel: model.BaseModel{ID: id, Version: 1}, Username: "admin", DisplayName: "管理员", AuthSource: "local", Enabled: true, Roles: []model.Role{{Code: "system_admin"}}},
		adminCount: 0,
	}
	enabled := true
	_, err := NewUserService(store).Update(context.Background(), id, 1, UserWriteInput{DisplayName: "管理员", AuthSource: "local", Enabled: &enabled, RoleCodes: []string{"user"}})
	if err != ErrLastAdmin {
		t.Fatalf("expected ErrLastAdmin, got %v", err)
	}
	if store.updateCalled {
		t.Fatal("last administrator role removal must not be persisted")
	}
}

func TestSaveLDAPConfigAppliesDefaultsAndRejectsIncompleteEnabledConfig(t *testing.T) {
	roleID := uuid.New()
	store := &userServiceStoreStub{role: model.Role{BaseModel: model.BaseModel{ID: roleID}, Code: "user", Name: "普通用户"}}
	svc := NewUserService(store)
	defaults, err := svc.GetLDAPConfig(context.Background())
	if err != nil || !defaults.AllowLocalFallback || defaults.UserFilter == "" {
		t.Fatalf("new LDAP defaults are invalid: %+v, err=%v", defaults, err)
	}
	cfg := &model.LDAPConfig{Enabled: true, URL: "ldap://ldap.example.com:389", AllowLocalFallback: true}
	if err := svc.SaveLDAPConfig(context.Background(), cfg); err == nil {
		t.Fatal("enabled LDAP without Base DN should be rejected")
	}
	cfg.BaseDN = "dc=example,dc=com"
	if err := svc.SaveLDAPConfig(context.Background(), cfg); err != nil {
		t.Fatalf("valid LDAP config rejected: %v", err)
	}
	if cfg.UserFilter == "" || cfg.UsernameAttribute == "" || cfg.DefaultRole != "user" || !cfg.AllowLocalFallback {
		t.Fatalf("LDAP defaults not applied: %+v", cfg)
	}
}

func TestSaveLDAPConfigRejectsInvalidGroupRoleMappings(t *testing.T) {
	roleID := uuid.New()
	store := &userServiceStoreStub{role: model.Role{BaseModel: model.BaseModel{ID: roleID}, Code: "user", Name: "普通用户"}}
	svc := NewUserService(store)
	base := &model.LDAPConfig{Enabled: false, DefaultRole: "user", GroupRoleMappings: []model.LDAPGroupRoleMapping{{Group: "", RoleCode: "user"}}}
	if err := svc.SaveLDAPConfig(context.Background(), base); err == nil {
		t.Fatal("empty LDAP group should be rejected")
	}
	base.GroupRoleMappings = []model.LDAPGroupRoleMapping{{Group: "cabinet-users", RoleCode: ""}}
	if err := svc.SaveLDAPConfig(context.Background(), base); err == nil {
		t.Fatal("empty role mapping should be rejected")
	}
	base.GroupRoleMappings = []model.LDAPGroupRoleMapping{{Group: "cabinet-users", RoleCode: "user"}, {Group: "CABINET-USERS", RoleCode: "user"}}
	if err := svc.SaveLDAPConfig(context.Background(), base); err == nil {
		t.Fatal("duplicate LDAP groups should be rejected case-insensitively")
	}
}
