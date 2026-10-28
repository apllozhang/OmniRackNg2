package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
)

type AdminHandler struct {
	users   *service.UserService
	ldap    *service.LDAPService
	devices *service.DeviceService
}

func NewAdminHandler(users *service.UserService, ldap *service.LDAPService, devices *service.DeviceService) *AdminHandler {
	return &AdminHandler{users: users, ldap: ldap, devices: devices}
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var enabled *bool
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			response.Error(c, 400, "INVALID_REQUEST", "enabled 参数格式无效", nil)
			return
		}
		enabled = &value
	}
	items, err := h.users.List(c.Request.Context(), repository.UserQuery{Search: strings.TrimSpace(c.Query("search")), AuthSource: strings.TrimSpace(c.Query("authSource")), Enabled: enabled})
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AdminHandler) ListRoles(c *gin.Context) {
	items, err := h.users.Roles(c.Request.Context())
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

type userRequest struct {
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	AuthSource  string   `json:"authSource"`
	Enabled     *bool    `json:"enabled"`
	RoleCodes   []string `json:"roleCodes"`
}

func (r userRequest) input() service.UserWriteInput {
	return service.UserWriteInput{Username: r.Username, DisplayName: r.DisplayName, Email: r.Email, Password: r.Password, AuthSource: r.AuthSource, Enabled: r.Enabled, RoleCodes: r.RoleCodes}
}
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req userRequest
	if c.ShouldBindJSON(&req) != nil {
		invalidJSON(c)
		return
	}
	item, err := h.users.Create(c.Request.Context(), req.input())
	if err != nil {
		adminError(c, err)
		return
	}
	response.Created(c, item)
}
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	var req userRequest
	if c.ShouldBindJSON(&req) != nil {
		invalidJSON(c)
		return
	}
	item, err := h.users.Update(c.Request.Context(), id, version, req.input())
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, item)
}
func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if c.ShouldBindJSON(&req) != nil {
		invalidJSON(c)
		return
	}
	item, err := h.users.ResetPassword(c.Request.Context(), id, version, req.Password)
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, item)
}
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.users.Delete(c.Request.Context(), id, version); err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}

func (h *AdminHandler) GetPolicy(c *gin.Context) {
	item, err := h.devices.GetPolicy(c.Request.Context())
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, item)
}
func (h *AdminHandler) UpdatePolicy(c *gin.Context) {
	var input service.PolicyUpdateInput
	if c.ShouldBindJSON(&input) != nil {
		invalidJSON(c)
		return
	}
	item, err := h.devices.UpdatePolicy(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, item)
}
func (h *AdminHandler) ListApprovals(c *gin.Context) {
	items, err := h.devices.ListApprovals(c.Request.Context(), c.Query("status"))
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}
func (h *AdminHandler) Approve(c *gin.Context) { h.decide(c, true) }
func (h *AdminHandler) Reject(c *gin.Context)  { h.decide(c, false) }
func (h *AdminHandler) decide(c *gin.Context, approve bool) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input service.ApprovalDecisionInput
	if c.ShouldBindJSON(&input) != nil {
		invalidJSON(c)
		return
	}
	var item model.ApprovalRecord
	var err error
	if approve {
		item, err = h.devices.Approve(c.Request.Context(), actorFromContext(c), id, input)
	} else {
		item, err = h.devices.Reject(c.Request.Context(), actorFromContext(c), id, input)
	}
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, item)
}

type ldapRequest struct {
	Version              *uint                        `json:"version"`
	Enabled              bool                         `json:"enabled"`
	URL                  string                       `json:"url"`
	UseTLS               bool                         `json:"useTls"`
	SkipTLSVerify        bool                         `json:"skipTlsVerify"`
	BindDN               string                       `json:"bindDn"`
	BindPassword         string                       `json:"bindPassword"`
	BaseDN               string                       `json:"baseDn"`
	UserFilter           string                       `json:"userFilter"`
	UsernameAttribute    string                       `json:"usernameAttribute"`
	DisplayNameAttribute string                       `json:"displayNameAttribute"`
	EmailAttribute       string                       `json:"emailAttribute"`
	DefaultRole          string                       `json:"defaultRole"`
	AllowLocalFallback   bool                         `json:"allowLocalFallback"`
	GroupSearchBaseDN    string                       `json:"groupSearchBaseDn"`
	GroupFilter          string                       `json:"groupFilter"`
	GroupAttribute       string                       `json:"groupAttribute"`
	GroupRoleMappings    []model.LDAPGroupRoleMapping `json:"groupRoleMappings"`
}

func (r ldapRequest) apply(item *model.LDAPConfig) {
	item.Enabled = r.Enabled
	item.URL = strings.TrimSpace(r.URL)
	item.UseTLS = r.UseTLS
	item.SkipTLSVerify = r.SkipTLSVerify
	item.BindDN = strings.TrimSpace(r.BindDN)
	item.BaseDN = strings.TrimSpace(r.BaseDN)
	item.UserFilter = strings.TrimSpace(r.UserFilter)
	item.UsernameAttribute = strings.TrimSpace(r.UsernameAttribute)
	item.DisplayNameAttribute = strings.TrimSpace(r.DisplayNameAttribute)
	item.EmailAttribute = strings.TrimSpace(r.EmailAttribute)
	item.DefaultRole = strings.TrimSpace(r.DefaultRole)
	item.AllowLocalFallback = r.AllowLocalFallback
	item.GroupSearchBaseDN = strings.TrimSpace(r.GroupSearchBaseDN)
	item.GroupFilter = strings.TrimSpace(r.GroupFilter)
	item.GroupAttribute = strings.TrimSpace(r.GroupAttribute)
	if r.GroupRoleMappings != nil {
		item.GroupRoleMappings = r.GroupRoleMappings
	}
	if strings.TrimSpace(r.BindPassword) != "" {
		item.BindPassword = r.BindPassword
	}
}
func (h *AdminHandler) GetLDAP(c *gin.Context) {
	item, err := h.users.GetLDAPConfig(c.Request.Context())
	if err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, ldapView(item))
}
func ldapView(item model.LDAPConfig) model.LDAPConfigView {
	return model.LDAPConfigView{LDAPConfig: item, HasBindPassword: strings.TrimSpace(item.BindPassword) != ""}
}
func (h *AdminHandler) UpdateLDAP(c *gin.Context) {
	var req ldapRequest
	if c.ShouldBindJSON(&req) != nil {
		invalidJSON(c)
		return
	}
	item, err := h.users.GetLDAPConfig(c.Request.Context())
	if err != nil {
		adminError(c, err)
		return
	}
	if req.Version != nil {
		item.Version = *req.Version
	}
	req.apply(&item)
	if err := h.users.SaveLDAPConfig(c.Request.Context(), &item); err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, ldapView(item))
}
func (h *AdminHandler) TestLDAP(c *gin.Context) {
	var req ldapRequest
	if c.ShouldBindJSON(&req) != nil {
		invalidJSON(c)
		return
	}
	item, err := h.users.GetLDAPConfig(c.Request.Context())
	if err != nil {
		adminError(c, err)
		return
	}
	req.apply(&item)
	if err := h.ldap.Test(c.Request.Context(), item); err != nil {
		adminError(c, err)
		return
	}
	response.Success(c, gin.H{"connected": true, "message": "LDAP 连接和服务账号验证成功"})
}

func adminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrUserNotFound):
		response.Error(c, http.StatusNotFound, "USER_NOT_FOUND", "用户不存在", nil)
	case errors.Is(err, service.ErrUserConflict):
		response.Error(c, http.StatusConflict, "USER_CONFLICT", "用户名已存在", nil)
	case errors.Is(err, service.ErrLastAdmin):
		response.Error(c, http.StatusConflict, "LAST_ADMIN_PROTECTED", "系统至少需要保留一个启用的管理员", nil)
	case errors.Is(err, service.ErrInvalidUser):
		response.Error(c, http.StatusBadRequest, "INVALID_USER", err.Error(), nil)
	case errors.Is(err, service.ErrRoleNotFound):
		response.Error(c, http.StatusBadRequest, "ROLE_NOT_FOUND", "选择的角色不存在", nil)
	case errors.Is(err, service.ErrInvalidLDAPConfig):
		response.Error(c, http.StatusBadRequest, "INVALID_LDAP_CONFIG", err.Error(), nil)
	case errors.Is(err, service.ErrLDAPConfigVersion):
		response.Error(c, http.StatusConflict, "LDAP_VERSION_CONFLICT", "LDAP 配置已被其他管理员修改，请刷新后重试", nil)
	case errors.Is(err, service.ErrLDAPCredentials):
		response.Error(c, http.StatusBadRequest, "LDAP_AUTH_FAILED", "LDAP 服务账号或用户认证失败", nil)
	case errors.Is(err, service.ErrLDAPUnavailable):
		response.Error(c, http.StatusBadRequest, "LDAP_UNAVAILABLE", err.Error(), nil)
	default:
		writeResourceError(c, err)
	}
}
