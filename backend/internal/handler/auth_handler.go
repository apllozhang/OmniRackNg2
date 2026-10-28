package handler

import (
	"errors"
	"net/http"

	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ service *service.AuthService }

func NewAuthHandler(svc *service.AuthService) *AuthHandler { return &AuthHandler{service: svc} }

type loginRequest struct {
	Username string `json:"username" binding:"required,max=100"`
	Password string `json:"password" binding:"required,max=128"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "用户名和密码不能为空", nil)
		return
	}
	result, err := h.service.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDisabledUser):
			response.Error(c, http.StatusForbidden, "USER_DISABLED", "用户已停用", nil)
		default:
			response.Error(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "用户名或密码错误", nil)
		}
		return
	}
	response.Success(c, result)
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetString(middleware.UserIDKey)
	user, err := h.service.CurrentUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "UNAUTHORIZED", "用户身份无效", nil)
		return
	}
	response.Success(c, user)
}

func (h *AuthHandler) Logout(c *gin.Context) { response.Success(c, gin.H{"loggedOut": true}) }
