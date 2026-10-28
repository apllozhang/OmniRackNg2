package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
)

// DeviceHandler exposes device ledger, U-position, history, and approval APIs.
type DeviceHandler struct {
	service *service.DeviceService
}

func NewDeviceHandler(svc *service.DeviceService) *DeviceHandler {
	return &DeviceHandler{service: svc}
}

func (h *DeviceHandler) ListDeviceTypes(c *gin.Context) {
	items, err := h.service.ListDeviceTypes(c.Request.Context())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DeviceHandler) CreateDeviceType(c *gin.Context) {
	var item model.DeviceType
	if err := c.ShouldBindJSON(&item); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.CreateDeviceType(c.Request.Context(), actorFromContext(c), item)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *DeviceHandler) UpdateDeviceType(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var item model.DeviceType
	if err := c.ShouldBindJSON(&item); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.UpdateDeviceType(c.Request.Context(), actorFromContext(c), id, item.Version, item)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *DeviceHandler) DeleteDeviceType(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteDeviceType(c.Request.Context(), actorFromContext(c), id, version); err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}

func (h *DeviceHandler) ListDevices(c *gin.Context) {
	query := repository.DeviceQuery{
		SortBy:          c.Query("sortBy"),
		SortOrder:       c.Query("sortOrder"),
		Scope:           c.Query("scope"),
		Page:            queryInt(c, "page", 1),
		PageSize:        queryInt(c, "pageSize", 20),
		Search:          c.Query("search"),
		LifecycleStatus: c.Query("lifecycleStatus"),
	}
	if raw := strings.TrimSpace(c.Query("typeId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_DEVICE_TYPE_ID", "设备类型 ID 格式无效", nil)
			return
		}
		query.TypeID = &id
	}
	if raw := strings.TrimSpace(c.Query("rackId")); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "INVALID_RACK_ID", "机柜 ID 格式无效", nil)
			return
		}
		query.RackID = &id
	}
	result, err := h.service.ListDevices(c.Request.Context(), query)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DeviceHandler) GetDevice(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.GetDevice(c.Request.Context(), id)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, item)
}

type deviceCreateRequest struct {
	model.Device
	AutoGenerateCode bool `json:"autoGenerateCode"`
}

func (h *DeviceHandler) CreateDevice(c *gin.Context) {
	var request deviceCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.CreateDevice(c.Request.Context(), actorFromContext(c), service.DeviceCreateInput{
		Device: request.Device, AutoGenerateCode: request.AutoGenerateCode,
	})
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *DeviceHandler) UpdateDevice(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var item model.Device
	if err := c.ShouldBindJSON(&item); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.UpdateDevice(c.Request.Context(), actorFromContext(c), id, item.Version, item)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *DeviceHandler) DeleteDevice(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteDevice(c.Request.Context(), actorFromContext(c), id, version); err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}

func (h *DeviceHandler) Assign(c *gin.Context) {
	h.changePosition(c, model.PositionOperationAssign)
}

func (h *DeviceHandler) Move(c *gin.Context) {
	h.changePosition(c, model.PositionOperationMove)
}

func (h *DeviceHandler) changePosition(c *gin.Context, operation string) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input service.PositionChangeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	var result service.OperationResult
	var err error
	if operation == model.PositionOperationAssign {
		result, err = h.service.Assign(c.Request.Context(), actorFromContext(c), id, input)
	} else {
		result, err = h.service.Move(c.Request.Context(), actorFromContext(c), id, input)
	}
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DeviceHandler) Decommission(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input service.DecommissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	result, err := h.service.Decommission(c.Request.Context(), actorFromContext(c), id, input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DeviceHandler) History(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	items, err := h.service.ListHistory(c.Request.Context(), id)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DeviceHandler) RackLayout(c *gin.Context) {
	id, ok := parseID(c, "rackId")
	if !ok {
		return
	}
	layout, err := h.service.GetRackLayout(c.Request.Context(), id)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, layout)
}

func (h *DeviceHandler) ValidateRackDiagramImport(c *gin.Context) {
	roomID, ok := parseID(c, "roomId")
	if !ok {
		return
	}
	var input service.RackDiagramImportValidateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	input.RoomID = roomID
	result, err := h.service.ValidateRackDiagramImport(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DeviceHandler) CommitRackDiagramImport(c *gin.Context) {
	roomID, ok := parseID(c, "roomId")
	if !ok {
		return
	}
	var input service.RackDiagramImportCommitInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	input.RoomID = roomID
	result, err := h.service.CommitRackDiagramImport(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *DeviceHandler) GetPolicy(c *gin.Context) {
	item, err := h.service.GetPolicy(c.Request.Context())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, item)
}

func (h *DeviceHandler) UpdatePolicy(c *gin.Context) {
	var input service.PolicyUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.UpdatePolicy(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *DeviceHandler) ListApprovals(c *gin.Context) {
	items, err := h.service.ListApprovals(c.Request.Context(), c.Query("status"))
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *DeviceHandler) Approve(c *gin.Context) {
	h.decide(c, true)
}

func (h *DeviceHandler) Reject(c *gin.Context) {
	h.decide(c, false)
}

func (h *DeviceHandler) decide(c *gin.Context, approve bool) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input service.ApprovalDecisionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	var item model.ApprovalRecord
	var err error
	if approve {
		item, err = h.service.Approve(c.Request.Context(), actorFromContext(c), id, input)
	} else {
		item, err = h.service.Reject(c.Request.Context(), actorFromContext(c), id, input)
	}
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, item)
}

func queryInt(c *gin.Context, name string, fallback int) int {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
