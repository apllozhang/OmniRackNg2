package handler

import (
	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
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
	pageSize := queryInt(c, "pageSize", 20)
	if page < 1 || page > 10000 || pageSize < 1 || pageSize > 100 {
		response.Error(c, 400, "INVALID_RESOURCE", "page 需为 1–10000，pageSize 需为 1–100", nil)
		return
	}
	order, ok := tableOrder(c, c.Query("sortBy"), c.Query("sortOrder"), map[string]string{
		"createdAt": "created_at", "channel": "CASE WHEN channel='EMAIL' THEN '邮件' ELSE '企业微信' END",
		"source":  "CASE WHEN test THEN '渠道测试' WHEN summary LIKE '待审批%' THEN '审批通知' ELSE '告警通知' END",
		"summary": "summary", "status": "CASE WHEN status='SUCCESS' THEN '通道已接收' ELSE '发送失败' END", "errorMessage": "error_message",
	}, "created_at DESC")
	if !ok {
		return
	}
	search, ok := tableSearch(c, c.Query("search"))
	if !ok {
		return
	}
	q := h.service.DB.WithContext(c.Request.Context()).Model(&model.NotificationDelivery{})
	if search != "%%" {
		q = q.Where("summary ILIKE ? OR error_message ILIKE ? OR message_id ILIKE ? OR recipients::text ILIKE ?", search, search, search, search)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		writeResourceError(c, err)
		return
	}
	items := []model.NotificationDelivery{}
	if err := q.Order(order).Limit(pageSize).Offset((page - 1) * pageSize).Find(&items).Error; err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
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
