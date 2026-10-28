package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/response"
)

type HealthHandler struct{ db *gorm.DB }

func NewHealthHandler(db *gorm.DB) *HealthHandler { return &HealthHandler{db: db} }

func (h *HealthHandler) Live(c *gin.Context) {
	response.Success(c, gin.H{"status": "alive"})
}

func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		response.Error(c, http.StatusServiceUnavailable, "NOT_READY", "数据库尚未就绪", nil)
		return
	}
	response.Success(c, gin.H{"status": "ready"})
}
