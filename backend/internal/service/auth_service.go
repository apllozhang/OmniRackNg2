package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/config"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrDisabledUser = errors.New("disabled user")
var ErrAuthUnavailable = middleware.ErrIdentityUnavailable

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
	token, err := middleware.NewTokenWithVersion(s.cfg.JWTSecret, user.ID.String(), roles, user.Version, s.cfg.JWTExpiresIn)
	if err != nil {
		return LoginResult{}, err
	}
	now := time.Now().UTC()
	// A preloaded User contains role associations. Saving it here could restore
	// a role concurrently removed by an administrator. Update only scalar login
	// metadata, and reject an account changed since its credentials were read.
	result := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ? AND version = ? AND enabled = ?", user.ID, user.Version, true).Updates(map[string]interface{}{"last_login_at": now, "failed_logins": 0})
	if result.Error != nil {
		return LoginResult{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, result.Error)
	}
	if result.RowsAffected != 1 {
		return LoginResult{}, ErrInvalidCredentials
	}
	user.LastLoginAt = &now
	user.FailedLogins = 0
	return LoginResult{Token: token, User: user}, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || utf8.RuneCountInString(username) > 100 || len(password) == 0 || len(password) > 512 {
		return LoginResult{}, ErrInvalidCredentials
	}
	user, findErr := repository.FindUserByUsername(ctx, s.db, username)
	if findErr != nil && !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return LoginResult{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, findErr)
	}
	if findErr == nil && user.AuthSource == "local" {
		if !user.Enabled {
			return LoginResult{}, ErrDisabledUser
		}
		// Local accounts never fall through to LDAP after a bad password. This
		// prevents an LDAP account with the same name from bypassing local auth.
		cfg, cfgErr := s.users.GetLDAPConfig(ctx)
		if cfgErr != nil && !errors.Is(cfgErr, gorm.ErrRecordNotFound) {
			return LoginResult{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, cfgErr)
		}
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
	cfg, cfgErr := s.users.GetLDAPConfig(ctx)
	if errors.Is(cfgErr, gorm.ErrRecordNotFound) || cfgErr == nil && !cfg.Enabled {
		return LoginResult{}, ErrInvalidCredentials
	}
	if cfgErr != nil {
		return LoginResult{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, cfgErr)
	}
	identity, err := s.ldap.Authenticate(ctx, username, password)
	if err != nil {
		if errors.Is(err, ErrLDAPUnavailable) {
			return LoginResult{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
		}
		return LoginResult{}, ErrInvalidCredentials
	}
	if identity.Username == "" {
		identity.Username = username
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
			return LoginResult{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
		}
	} else {
		user, err = s.syncLDAPIdentity(ctx, user, identity, roles)
		if err != nil {
			return LoginResult{}, err
		}
	}
	return s.issueToken(ctx, user)
}

func (s *AuthService) syncLDAPIdentity(ctx context.Context, user model.User, identity LDAPIdentity, roles []model.Role) (model.User, error) {
	roleCodes := make([]string, 0, len(roles))
	for _, role := range roles {
		roleCodes = append(roleCodes, role.Code)
	}
	updated, err := repository.SyncLDAPIdentity(ctx, s.db, user.ID, user.Version, identity.DisplayName, identity.Email, roleCodes)
	if err != nil {
		if errors.Is(err, repository.ErrLDAPIdentityChanged) || errors.Is(err, repository.ErrLastEnabledAdmin) || errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrInvalidCredentials
		}
		return model.User{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	return updated, nil
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
	user, err := repository.FindUserByID(ctx, s.db, id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return model.User{}, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	if err == nil && !user.Enabled {
		return model.User{}, ErrDisabledUser
	}
	return user, err
}

// ResolveIdentity deliberately reads the database on every authenticated
// request. JWT roles are a client snapshot, never an authorization authority.
func (s *AuthService) ResolveIdentity(ctx context.Context, claims *middleware.Claims) ([]string, error) {
	if _, err := uuid.Parse(claims.UserID); err != nil {
		return nil, ErrInvalidCredentials
	}
	user, err := s.CurrentUser(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, ErrDisabledUser) || errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrAuthUnavailable, err)
	}
	if claims.UserVersion == 0 || claims.UserVersion != user.Version {
		return nil, ErrInvalidCredentials
	}
	roles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, role.Code)
	}
	return roles, nil
}

func (s *AuthService) AuthMiddleware() gin.HandlerFunc {
	return middleware.Auth(s.cfg.JWTSecret, s.ResolveIdentity)
}

// Logout revokes every session for this account, including on other backend
// processes. The same version is advanced by password and account updates.
func (s *AuthService) Logout(ctx context.Context, userID string) error {
	id, err := uuid.Parse(userID)
	if err != nil {
		return ErrInvalidCredentials
	}
	result := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ? AND enabled = ?", id, true).UpdateColumn("version", gorm.Expr("version + 1"))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInvalidCredentials
	}
	return nil
}
