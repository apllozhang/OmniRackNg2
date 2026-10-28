package handler

import (
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type NotificationHandler struct{ service *service.NotificationService }

func (h *NotificationHandler) ApprovalSettings(c *gin.Context) {
	x, err := h.service.ApprovalSettings(c.Request.Context())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}

func (h *NotificationHandler) SaveApprovalSettings(c *gin.Context) {
	var input service.ApprovalEmailSettings
	if c.ShouldBindJSON(&input) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.service.SaveApprovalSettings(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}

func NewNotificationHandler(s *service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: s}
}

func (h *NotificationHandler) Settings(c *gin.Context) {
	x, err := h.service.View(c.Request.Context())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *NotificationHandler) Save(c *gin.Context) {
	var input service.NotificationInput
	if c.ShouldBindJSON(&input) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.service.Save(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *NotificationHandler) Test(c *gin.Context) {
	var input struct {
		Channel string `json:"channel"`
	}
	if c.ShouldBindJSON(&input) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.service.Test(c.Request.Context(), actorFromContext(c), input.Channel)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	// Keep failed attempts in delivery history, but never present them as success.
	if x.Status != "SUCCESS" {
		response.Error(c, 502, "NOTIFICATION_FAILED", x.ErrorMessage, nil)
		return
	}
	response.Success(c, x)
}
func (h *NotificationHandler) Deliveries(c *gin.Context) {
	page := queryInt(c, "page", 1)
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		page = 10000
	}
	q := h.service.DB.WithContext(c.Request.Context()).Model(&model.NotificationDelivery{})
	var total int64
	if err := q.Count(&total).Error; err != nil {
		writeResourceError(c, err)
		return
	}
	items := []model.NotificationDelivery{}
	if err := q.Order("created_at DESC").Limit(20).Offset((page - 1) * 20).Find(&items).Error; err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "pageSize": 20})
}

func (h *NotificationHandler) TestApproval(c *gin.Context) {
	var in struct {
		Channel string `json:"channel"`
	}
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.service.TestApproval(c.Request.Context(), actorFromContext(c), in.Channel)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	if x.Status != "SUCCESS" {
		response.Error(c, 502, "NOTIFICATION_FAILED", x.ErrorMessage, nil)
		return
	}
	response.Success(c, x)
}
