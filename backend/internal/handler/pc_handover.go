package handler

import (
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AssetOperationsHandler) RegisterTerminal(c *gin.Context) {
	var in struct {
		model.Device
		AutoGenerateCode bool                         `json:"autoGenerateCode"`
		Terminal         service.TerminalRegistration `json:"terminal"`
	}
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.RegisterTerminal(c.Request.Context(), actorFromContext(c), service.DeviceCreateInput{Device: in.Device, AutoGenerateCode: in.AutoGenerateCode}, in.Terminal)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, x)
}

func (h *AssetOperationsHandler) TerminalState(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	x, err := h.ops.TerminalState(c.Request.Context(), id)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}

func (h *AssetOperationsHandler) ConfirmPCHandover(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in service.PCHandoverInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.ConfirmPCHandover(c.Request.Context(), actorFromContext(c), id, in)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, x)
}

func (h *AssetOperationsHandler) PCHistory(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	page := queryInt(c, "page", 1)
	if page < 1 || page > 10000 {
		page = 1
	}
	q := h.ops.DB.WithContext(c.Request.Context()).Model(&model.PCHandover{}).Where("device_id = ?", id)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		writeResourceError(c, err)
		return
	}
	items := []model.PCHandover{}
	if err := q.Order("confirmed_at DESC, id DESC").Limit(50).Offset((page - 1) * 50).Find(&items).Error; err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total, "page": page, "pageSize": 50})
}
