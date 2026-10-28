package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
)

type AuthHandler struct {
	service         *service.AuthService
	ipAttempts      *middleware.AttemptLimiter
	accountAttempts *middleware.AttemptLimiter
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{service: svc,
		ipAttempts:      middleware.NewAttemptLimiter(120, time.Minute, 4096),
		accountAttempts: middleware.NewAttemptLimiter(10, 5*time.Minute, 4096),
	}
}

func (h *AuthHandler) AuthMiddleware() gin.HandlerFunc {
	if h == nil || h.service == nil {
		return middleware.Auth("", nil)
	}
	return h.service.AuthMiddleware()
}

type loginRequest struct {
	Username string `json:"username" binding:"required,max=100"`
	Password string `json:"password" binding:"required,max=128"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	if h == nil || h.service == nil {
		response.Error(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "认证服务不可用", nil)
		return
	}
	if !h.ipAttempts.Allow(c.ClientIP()) {
		h.loginLimited(c)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			response.Error(c, http.StatusRequestEntityTooLarge, "REQUEST_TOO_LARGE", "登录请求过大", nil)
			return
		}
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "用户名和密码不能为空", nil)
		return
	}
	accountKey := strings.ToLower(strings.TrimSpace(req.Username))
	if !h.accountAttempts.Allow(accountKey) {
		h.loginLimited(c)
		return
	}
	result, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDisabledUser):
			response.Error(c, http.StatusForbidden, "USER_DISABLED", "用户已停用", nil)
		case errors.Is(err, service.ErrAuthUnavailable):
			response.Error(c, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "认证服务暂时不可用，请稍后重试", nil)
		default:
			response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误", nil)
		}
		return
	}
	h.accountAttempts.Reset(accountKey)
	response.Success(c, result)
}

func (h *AuthHandler) loginLimited(c *gin.Context) {
	c.Header("Retry-After", "300")
	response.Error(c, http.StatusTooManyRequests, "LOGIN_RATE_LIMITED", "登录尝试过于频繁，请稍后重试", nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString(middleware.UserIDKey)
	user, err := h.service.CurrentUser(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrAuthUnavailable) {
			response.Error(c, http.StatusServiceUnavailable, "AUTH_UNAVAILABLE", "认证服务暂时不可用，请稍后重试", nil)
			return
		}
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "用户身份无效", nil)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if err := h.service.Logout(c.Request.Context(), c.GetString(middleware.UserIDKey)); err != nil {
		response.Error(c, http.StatusServiceUnavailable, "LOGOUT_FAILED", "注销失败，请重试", nil)
		return
	}
	response.Success(c, gin.H{"loggedOut": true})
}
