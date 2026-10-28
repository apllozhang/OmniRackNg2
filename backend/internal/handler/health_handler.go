package handler

import (
	"net/http"

	"omnirack-replica/backend/internal/response"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct{ db *gorm.DB }

func NewHealthHandler(db *gorm.DB) *HealthHandler { return &HealthHandler{db: db} }

func (h *HealthHandler) Live(c *gin.Context) {
	response.Success(c, gin.H{"status": "alive"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
		response.Error(c, http.StatusServiceUnavailable, "NOT_READY", "数据库尚未就绪", nil)
		return
	}
	response.Success(c, gin.H{"status": "ready"})
}
