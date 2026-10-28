package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"omnirack-replica/backend/internal/config"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrDisabledUser = errors.New("disabled user")

type AuthService struct {
	db    *gorm.DB
	cfg   config.Config
	ldap  *LDAPService
	users repository.UserStore
}

func NewAuthService(db *gorm.DB, cfg config.Config, ldapServices ...*LDAPService) *AuthService {
	var ldapService *LDAPService
	if len(ldapServices) > 0 {
		ldapService = ldapServices[0]
	}
	return &AuthService{db: db, cfg: cfg, ldap: ldapService, users: repository.NewUserStore(db)}
}

type LoginResult struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

func (s *AuthService) issueToken(ctx context.Context, user model.User) (LoginResult, error) {
	roles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, role.Code)
	}
	token, err := middleware.NewToken(s.cfg.JWTSecret, user.ID.String(), roles, s.cfg.JWTExpiresIn)
	if err != nil {
		return LoginResult{}, err
	}
	now := time.Now().UTC()
	if err := s.db.WithContext(ctx).Model(&user).Updates(map[string]interface{}{"last_login_at": now, "failed_logins": 0}).Error; err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Token: token, User: user}, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (LoginResult, error) {
	user, findErr := repository.FindUserByUsername(ctx, s.db, username)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return LoginResult{}, ErrInvalidCredentials
	}
	if findErr == nil && user.AuthSource == "local" {
		if !user.Enabled {
			return LoginResult{}, ErrDisabledUser
		}
		// Local accounts never fall through to LDAP after a bad password. This
		// prevents an LDAP account with the same name from bypassing local auth.
		cfg, _ := s.users.GetLDAPConfig(ctx)
		if cfg.Enabled && !cfg.AllowLocalFallback && !hasAdminRole(user) {
			return LoginResult{}, ErrInvalidCredentials
		}
		if err := user.CheckPassword(password); err != nil {
			return LoginResult{}, ErrInvalidCredentials
		}
		return s.issueToken(ctx, user)
	}
	if findErr == nil && user.AuthSource == "ldap" && !user.Enabled {
		return LoginResult{}, ErrDisabledUser
	}
	if s.ldap == nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	identity, err := s.ldap.Authenticate(ctx, username, password)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if identity.Username == "" {
		identity.Username = username
	}
	cfg, cfgErr := s.users.GetLDAPConfig(ctx)
	if cfgErr != nil && !errors.Is(cfgErr, gorm.ErrRecordNotFound) {
		return LoginResult{}, ErrInvalidCredentials
	}
	roles, roleErr := s.ldapRoles(ctx, cfg, identity.Groups)
	if roleErr != nil {
		// A broken group-to-role mapping must never silently grant a more
		// privileged fallback role. Treat it as an authentication failure.
		return LoginResult{}, ErrInvalidCredentials
	}
	if findErr != nil {
		user = model.User{Username: identity.Username, DisplayName: identity.DisplayName, Email: identity.Email, AuthSource: "ldap", Enabled: true}
		if err := user.SetPassword(uuid.NewString()); err != nil {
			return LoginResult{}, err
		}
		user.Roles = roles
		if err := s.db.WithContext(ctx).Create(&user).Error; err != nil {
			return LoginResult{}, err
		}
	} else {
		user.DisplayName = identity.DisplayName
		user.Email = identity.Email
		if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&user).Updates(map[string]interface{}{"display_name": user.DisplayName, "email": user.Email}).Error; err != nil {
				return err
			}
			if err := tx.Model(&user).Association("Roles").Replace(roles); err != nil {
				return err
			}
			return nil
		}); err != nil {
			return LoginResult{}, err
		}
		user.Roles = roles
	}
	return s.issueToken(ctx, user)
}

func (s *AuthService) ldapRoles(ctx context.Context, cfg model.LDAPConfig, groups []string) ([]model.Role, error) {
	codes := LDAPRoleCodes(cfg, groups)
	if len(codes) == 0 {
		return nil, fmt.Errorf("ldap role mapping produced no role")
	}
	roles := make([]model.Role, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		role, err := s.users.GetRoleByCode(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("ldap role %q is not available: %w", code, err)
		}
		seen[code] = struct{}{}
		roles = append(roles, role)
	}
	if len(roles) == 0 {
		return nil, fmt.Errorf("ldap role mapping produced no valid role")
	}
	return roles, nil
}

func hasAdminRole(user model.User) bool {
	for _, role := range user.Roles {
		if role.Code == "system_admin" {
			return true
		}
	}
	return false
}

func (s *AuthService) CurrentUser(ctx context.Context, userID string) (model.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return model.User{}, err
	}
	return repository.FindUserByID(ctx, s.db, id)
}
