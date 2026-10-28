package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound      = errors.New("user not found")
	ErrUserConflict      = errors.New("user conflict")
	ErrLastAdmin         = errors.New("cannot remove the last administrator")
	ErrInvalidUser       = errors.New("invalid user")
	ErrRoleNotFound      = errors.New("role not found")
	ErrInvalidLDAPConfig = errors.New("invalid ldap config")
)

type UserWriteInput struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	AuthSource  string   `json:"authSource"`
	Enabled     *bool    `json:"enabled"`
	RoleCodes   []string `json:"roleCodes"`
}

type UserService struct{ store repository.UserStore }

func NewUserService(store repository.UserStore) *UserService { return &UserService{store: store} }

func (s *UserService) List(ctx context.Context, query repository.UserQuery) ([]model.User, error) {
	return s.store.ListUsers(ctx, query)
}
func (s *UserService) Roles(ctx context.Context) ([]model.Role, error) { return s.store.ListRoles(ctx) }
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (model.User, error) {
	item, err := s.store.GetUser(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, ErrUserNotFound
	}
	return item, err
}

func normalizeUserInput(input UserWriteInput, creating bool) (UserWriteInput, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	input.Email = strings.TrimSpace(input.Email)
	input.AuthSource = strings.ToLower(strings.TrimSpace(input.AuthSource))
	if input.AuthSource == "" {
		input.AuthSource = "local"
	}
	if input.AuthSource != "local" && input.AuthSource != "ldap" {
		return input, fmt.Errorf("%w: authSource 必须是 local 或 ldap", ErrInvalidUser)
	}
	if creating && input.Username == "" {
		return input, fmt.Errorf("%w: 用户名不能为空", ErrInvalidUser)
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Username
	}
	if creating && input.AuthSource == "local" && len(input.Password) < 8 {
		return input, fmt.Errorf("%w: 本地用户密码至少 8 位", ErrInvalidUser)
	}
	if input.Password != "" && len(input.Password) < 8 {
		return input, fmt.Errorf("%w: 密码至少 8 位", ErrInvalidUser)
	}
	if len(input.RoleCodes) == 0 {
		input.RoleCodes = []string{"user"}
	}
	seen := map[string]bool{}
	roles := input.RoleCodes[:0]
	for _, code := range input.RoleCodes {
		code = strings.TrimSpace(code)
		if code != "" && !seen[code] {
			seen[code] = true
			roles = append(roles, code)
		}
	}
	if len(roles) == 0 {
		return input, fmt.Errorf("%w: 至少选择一个角色", ErrInvalidUser)
	}
	input.RoleCodes = roles
	return input, nil
}

func (s *UserService) Create(ctx context.Context, input UserWriteInput) (model.User, error) {
	input, err := normalizeUserInput(input, true)
	if err != nil {
		return model.User{}, err
	}
	item := model.User{Username: input.Username, DisplayName: input.DisplayName, Email: input.Email, AuthSource: input.AuthSource, Enabled: true}
	if input.Enabled != nil {
		item.Enabled = *input.Enabled
	}
	if input.AuthSource == "local" {
		if err := item.SetPassword(input.Password); err != nil {
			return model.User{}, err
		}
	}
	if err := s.store.CreateUser(ctx, &item, input.RoleCodes); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.User{}, ErrUserConflict
		}
		if strings.Contains(strings.ToLower(err.Error()), "record not found") {
			return model.User{}, ErrRoleNotFound
		}
		return model.User{}, err
	}
	return s.store.GetUser(ctx, item.ID)
}

func (s *UserService) Update(ctx context.Context, id uuid.UUID, expectedVersion uint, input UserWriteInput) (model.User, error) {
	if expectedVersion == 0 {
		return model.User{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidUser)
	}
	current, err := s.Get(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	input.Username = current.Username
	if input.DisplayName == "" {
		input.DisplayName = current.DisplayName
	}
	if input.AuthSource == "" {
		input.AuthSource = current.AuthSource
	}
	if input.Enabled == nil {
		value := current.Enabled
		input.Enabled = &value
	}
	input, err = normalizeUserInput(input, false)
	if err != nil {
		return model.User{}, err
	}
	if current.AuthSource != input.AuthSource && input.AuthSource == "local" && input.Password == "" {
		return model.User{}, fmt.Errorf("%w: LDAP 账号切换为本地账号时必须设置密码", ErrInvalidUser)
	}
	isAdmin := false
	for _, role := range current.Roles {
		if role.Code == "system_admin" {
			isAdmin = true
			break
		}
	}
	if isAdmin && !*input.Enabled {
		count, e := s.store.CountEnabledAdmins(ctx, id)
		if e != nil {
			return model.User{}, e
		}
		if count == 0 {
			return model.User{}, ErrLastAdmin
		}
	}
	current.DisplayName, current.Email, current.AuthSource, current.Enabled = input.DisplayName, input.Email, input.AuthSource, *input.Enabled
	removingAdmin := false
	if isAdmin {
		removingAdmin = true
		for _, code := range input.RoleCodes {
			if code == "system_admin" {
				removingAdmin = false
				break
			}
		}
	}
	if removingAdmin {
		count, e := s.store.CountEnabledAdmins(ctx, id)
		if e != nil {
			return model.User{}, e
		}
		if count == 0 {
			return model.User{}, ErrLastAdmin
		}
	}
	if input.Password != "" {
		if err := current.SetPassword(input.Password); err != nil {
			return model.User{}, err
		}
	}
	if err := s.store.UpdateUser(ctx, &current, expectedVersion, input.RoleCodes); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return model.User{}, ErrUserConflict
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, err
	}
	return s.store.GetUser(ctx, id)
}

func (s *UserService) ResetPassword(ctx context.Context, id uuid.UUID, expectedVersion uint, password string) (model.User, error) {
	current, err := s.Get(ctx, id)
	if err != nil {
		return model.User{}, err
	}
	if current.AuthSource == "ldap" {
		return model.User{}, fmt.Errorf("%w: LDAP 账号由目录服务验证，不能重置本地密码", ErrInvalidUser)
	}
	input := UserWriteInput{DisplayName: current.DisplayName, Email: current.Email, AuthSource: current.AuthSource, Enabled: &current.Enabled, Password: password}
	codes := make([]string, 0, len(current.Roles))
	for _, role := range current.Roles {
		codes = append(codes, role.Code)
	}
	input.RoleCodes = codes
	return s.Update(ctx, id, expectedVersion, input)
}

func (s *UserService) Delete(ctx context.Context, id uuid.UUID, expectedVersion uint) error {
	current, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	for _, role := range current.Roles {
		if role.Code == "system_admin" {
			count, e := s.store.CountEnabledAdmins(ctx, id)
			if e != nil {
				return e
			}
			if count == 0 {
				return ErrLastAdmin
			}
			break
		}
	}
	if err := s.store.DeleteUser(ctx, id, expectedVersion); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

func (s *UserService) GetLDAPConfig(ctx context.Context) (model.LDAPConfig, error) {
	item, err := s.store.GetLDAPConfig(ctx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.DefaultLDAPConfig(), nil
	}
	if err != nil {
		return model.LDAPConfig{}, err
	}
	item.ApplyDefaults()
	return item, nil
}
func (s *UserService) SaveLDAPConfig(ctx context.Context, item *model.LDAPConfig) error {
	if item == nil {
		return fmt.Errorf("%w: 配置不能为空", ErrInvalidLDAPConfig)
	}
	item.ApplyDefaults()
	if item.Enabled && strings.TrimSpace(item.URL) == "" {
		return fmt.Errorf("%w: 启用 LDAP 时地址不能为空", ErrInvalidLDAPConfig)
	}
	if item.Enabled && strings.TrimSpace(item.BaseDN) == "" {
		return fmt.Errorf("%w: 启用 LDAP 时 Base DN 不能为空", ErrInvalidLDAPConfig)
	}
	if _, err := s.store.GetRoleByCode(ctx, item.DefaultRole); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrRoleNotFound
		}
		return err
	}
	seenGroups := map[string]bool{}
	for _, mapping := range item.GroupRoleMappings {
		group := strings.TrimSpace(mapping.Group)
		roleCode := strings.TrimSpace(mapping.RoleCode)
		if group == "" || roleCode == "" {
			return fmt.Errorf("%w: LDAP 组映射的组名和角色不能为空", ErrInvalidLDAPConfig)
		}
		if seenGroups[strings.ToLower(group)] {
			return fmt.Errorf("%w: LDAP 组映射不能重复", ErrInvalidLDAPConfig)
		}
		seenGroups[strings.ToLower(group)] = true
		if _, err := s.store.GetRoleByCode(ctx, roleCode); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRoleNotFound
			}
			return err
		}
	}
	return s.store.SaveLDAPConfig(ctx, item)
}
