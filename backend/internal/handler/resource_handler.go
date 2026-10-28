package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// resourceService captures the data center, room, and rack use cases required by this handler.
// Keeping the handler dependent on this small contract allows the HTTP layer to be tested
// independently from the database-backed service implementation.
type resourceService interface {
	ListTree(context.Context) ([]model.DataCenter, error)
	CreateDataCenter(context.Context, service.Actor, service.DataCenterCreateInput) (model.DataCenter, error)
	UpdateDataCenter(context.Context, service.Actor, uuid.UUID, uint, model.DataCenter) (model.DataCenter, error)
	DeleteDataCenter(context.Context, service.Actor, uuid.UUID, uint) error
	CopyDataCenter(context.Context, service.Actor, uuid.UUID, service.ResourceCopyInput) (service.ResourceCopyResult, error)
	CreateRoom(context.Context, service.Actor, uuid.UUID, service.RoomWriteInput) (model.Room, error)
	UpdateRoom(context.Context, service.Actor, uuid.UUID, uint, service.RoomWriteInput) (model.Room, error)
	DeleteRoom(context.Context, service.Actor, uuid.UUID, uint) error
	CopyRoom(context.Context, service.Actor, uuid.UUID, service.ResourceCopyInput) (service.ResourceCopyResult, error)
	MoveRoom(context.Context, service.Actor, uuid.UUID, service.ResourceMoveInput) (model.Room, error)
	CreateRack(context.Context, service.Actor, uuid.UUID, service.RackCreateInput) (model.Rack, error)
	UpdateRack(context.Context, service.Actor, uuid.UUID, uint, model.Rack) (model.Rack, error)
	DeleteRack(context.Context, service.Actor, uuid.UUID, uint) error
	CopyRack(context.Context, service.Actor, uuid.UUID, service.ResourceCopyInput) (service.ResourceCopyResult, error)
	MoveRack(context.Context, service.Actor, uuid.UUID, service.ResourceMoveInput) (model.Rack, error)
}

// ResourceHandler exposes data center, room and rack APIs.
type ResourceHandler struct {
	service resourceService
}

func NewResourceHandler(svc resourceService) *ResourceHandler {
	return &ResourceHandler{service: svc}
}

type optionalInt struct {
	value int
	set   bool
}

func (item *optionalInt) UnmarshalJSON(data []byte) error {
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return errors.New("integer value cannot be null")
	}
	if err := json.Unmarshal(data, &item.value); err != nil {
		return err
	}
	item.set = true
	return nil
}

type dataCenterCreateRequest struct {
	model.DataCenter
	AutoGenerateCode bool `json:"autoGenerateCode"`
}

type roomWriteRequest struct {
	model.Room
	RacksPerRow      optionalInt `json:"racksPerRow"`
	AutoGenerateCode bool        `json:"autoGenerateCode"`
}

type rackCreateRequest struct {
	model.Rack
	AutoGenerateCode     bool `json:"autoGenerateCode"`
	OverrideTemplateSpec bool `json:"overrideTemplateSpec"`
}

type resourceCopyRequest struct {
	Version            uint      `json:"version"`
	Code               string    `json:"code"`
	Name               string    `json:"name"`
	AutoGenerateCode   bool      `json:"autoGenerateCode"`
	TargetDataCenterID uuid.UUID `json:"targetDataCenterId"`
	TargetRoomID       uuid.UUID `json:"targetRoomId"`
}

func (request resourceCopyRequest) serviceInput() service.ResourceCopyInput {
	return service.ResourceCopyInput{Version: request.Version, Code: request.Code, Name: request.Name, AutoGenerateCode: request.AutoGenerateCode, TargetDataCenterID: request.TargetDataCenterID, TargetRoomID: request.TargetRoomID}
}

type resourceMoveRequest struct {
	Version            uint      `json:"version"`
	Code               string    `json:"code"`
	AutoGenerateCode   bool      `json:"autoGenerateCode"`
	TargetDataCenterID uuid.UUID `json:"targetDataCenterId"`
	TargetRoomID       uuid.UUID `json:"targetRoomId"`
}

func (request resourceMoveRequest) serviceInput() service.ResourceMoveInput {
	return service.ResourceMoveInput{Version: request.Version, Code: request.Code, AutoGenerateCode: request.AutoGenerateCode, TargetDataCenterID: request.TargetDataCenterID, TargetRoomID: request.TargetRoomID}
}

func (request roomWriteRequest) serviceInput() service.RoomWriteInput {
	var racksPerRow *int
	if request.RacksPerRow.set {
		value := request.RacksPerRow.value
		racksPerRow = &value
	}
	return service.RoomWriteInput{Room: request.Room, RacksPerRow: racksPerRow, AutoGenerateCode: request.AutoGenerateCode}
}

func (h *ResourceHandler) Tree(c *gin.Context) {
	items, err := h.service.ListTree(c.Request.Context())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *ResourceHandler) CreateDataCenter(c *gin.Context) {
	var request dataCenterCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.CreateDataCenter(c.Request.Context(), actorFromContext(c), service.DataCenterCreateInput{
		DataCenter: request.DataCenter, AutoGenerateCode: request.AutoGenerateCode,
	})
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *ResourceHandler) UpdateDataCenter(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var item model.DataCenter
	if err := c.ShouldBindJSON(&item); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.UpdateDataCenter(c.Request.Context(), actorFromContext(c), id, item.Version, item)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ResourceHandler) DeleteDataCenter(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteDataCenter(c.Request.Context(), actorFromContext(c), id, version); err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}

func (h *ResourceHandler) CreateRoom(c *gin.Context) {
	dataCenterID, ok := parseID(c, "dataCenterId")
	if !ok {
		return
	}
	var request roomWriteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.CreateRoom(c.Request.Context(), actorFromContext(c), dataCenterID, request.serviceInput())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *ResourceHandler) UpdateRoom(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var request roomWriteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.UpdateRoom(c.Request.Context(), actorFromContext(c), id, request.Room.Version, request.serviceInput())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ResourceHandler) DeleteRoom(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteRoom(c.Request.Context(), actorFromContext(c), id, version); err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}

func (h *ResourceHandler) CreateRack(c *gin.Context) {
	roomID, ok := parseID(c, "roomId")
	if !ok {
		return
	}
	var request rackCreateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.CreateRack(c.Request.Context(), actorFromContext(c), roomID, service.RackCreateInput{
		Rack: request.Rack, AutoGenerateCode: request.AutoGenerateCode, OverrideTemplateSpec: request.OverrideTemplateSpec,
	})
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *ResourceHandler) UpdateRack(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var item model.Rack
	if err := c.ShouldBindJSON(&item); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.UpdateRack(c.Request.Context(), actorFromContext(c), id, item.Version, item)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ResourceHandler) DeleteRack(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.service.DeleteRack(c.Request.Context(), actorFromContext(c), id, version); err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}

func (h *ResourceHandler) CopyDataCenter(c *gin.Context) { h.copyResource(c, "data-center") }
func (h *ResourceHandler) CopyRoom(c *gin.Context)       { h.copyResource(c, "room") }
func (h *ResourceHandler) CopyRack(c *gin.Context)       { h.copyResource(c, "rack") }

func (h *ResourceHandler) copyResource(c *gin.Context, kind string) {
	id, ok := parseID(c, actionIDParam(kind))
	if !ok {
		return
	}
	var request resourceCopyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	var result service.ResourceCopyResult
	var err error
	switch kind {
	case "data-center":
		result, err = h.service.CopyDataCenter(c.Request.Context(), actorFromContext(c), id, request.serviceInput())
	case "room":
		result, err = h.service.CopyRoom(c.Request.Context(), actorFromContext(c), id, request.serviceInput())
	default:
		result, err = h.service.CopyRack(c.Request.Context(), actorFromContext(c), id, request.serviceInput())
	}
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, result)
}

func (h *ResourceHandler) MoveRoom(c *gin.Context) { h.moveResource(c, "room") }
func (h *ResourceHandler) MoveRack(c *gin.Context) { h.moveResource(c, "rack") }

func (h *ResourceHandler) moveResource(c *gin.Context, kind string) {
	id, ok := parseID(c, actionIDParam(kind))
	if !ok {
		return
	}
	var request resourceMoveRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		invalidJSON(c)
		return
	}
	if kind == "room" {
		result, err := h.service.MoveRoom(c.Request.Context(), actorFromContext(c), id, request.serviceInput())
		if err != nil {
			writeResourceError(c, err)
			return
		}
		response.Success(c, result)
		return
	}
	result, err := h.service.MoveRack(c.Request.Context(), actorFromContext(c), id, request.serviceInput())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, result)
}

func actionIDParam(kind string) string {
	switch kind {
	case "data-center":
		return "dataCenterId"
	case "room":
		return "roomId"
	default:
		return "rackId"
	}
}
func parseID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_RESOURCE_ID", "资源 ID 格式无效", nil)
		return uuid.Nil, false
	}
	return id, true
}

func parseVersion(c *gin.Context) (uint, bool) {
	raw := c.Query("version")
	parsed, err := strconv.ParseUint(raw, 10, 32)
	if err != nil || parsed == 0 {
		response.Error(c, http.StatusBadRequest, "INVALID_VERSION", "version 必须是大于 0 的整数", nil)
		return 0, false
	}
	return uint(parsed), true
}

func actorFromContext(c *gin.Context) service.Actor {
	var userID *uuid.UUID
	if parsed, err := uuid.Parse(c.GetString(middleware.UserIDKey)); err == nil {
		userID = &parsed
	}
	return service.Actor{UserID: userID, RequestID: c.GetString("requestId"), Source: "web"}
}

func invalidJSON(c *gin.Context) {
	response.Error(c, http.StatusBadRequest, "INVALID_REQUEST", "请求参数格式无效", nil)
}

func writeResourceError(c *gin.Context, err error) {
	var conflict *service.UConflictError
	switch {
	case errors.Is(err, service.ErrResourceNotFound):
		response.Error(c, http.StatusNotFound, "RESOURCE_NOT_FOUND", "资源不存在或已删除", nil)
	case errors.Is(err, service.ErrDuplicateCode):
		response.Error(c, http.StatusConflict, "RESOURCE_CODE_DUPLICATE", "同级资源编码已存在", nil)
	case errors.Is(err, service.ErrResourceVersion):
		response.Error(c, http.StatusConflict, "RESOURCE_VERSION_CONFLICT", "数据已被其他用户修改，请刷新后重试", nil)
	case errors.Is(err, service.ErrSystemTemplate):
		response.Error(c, http.StatusConflict, "SYSTEM_TEMPLATE_PROTECTED", err.Error(), nil)
	case errors.Is(err, service.ErrTemplateDisabled):
		response.Error(c, http.StatusConflict, "RACK_TEMPLATE_DISABLED", err.Error(), nil)
	case errors.Is(err, service.ErrParentDisabled):
		response.Error(c, http.StatusConflict, "PARENT_RESOURCE_DISABLED", err.Error(), nil)
	case errors.Is(err, service.ErrResourceLimit):
		response.Error(c, http.StatusConflict, "RESOURCE_LIMIT_EXCEEDED", err.Error(), nil)
	case errors.Is(err, service.ErrHasChildren):
		response.Error(c, http.StatusConflict, "RESOURCE_HAS_CHILDREN", err.Error(), nil)
	case errors.Is(err, service.ErrInvalidResource):
		response.Error(c, http.StatusBadRequest, "INVALID_RESOURCE", err.Error(), nil)
	case errors.Is(err, service.ErrDeviceTypeInUse):
		response.Error(c, http.StatusConflict, "DEVICE_TYPE_IN_USE", err.Error(), nil)
	case errors.Is(err, service.ErrDevicePositioned):
		response.Error(c, http.StatusConflict, "DEVICE_POSITIONED", err.Error(), nil)
	case errors.Is(err, service.ErrDeviceNotPositioned):
		response.Error(c, http.StatusConflict, "DEVICE_NOT_POSITIONED", err.Error(), nil)
	case errors.Is(err, service.ErrRackUnavailable):
		response.Error(c, http.StatusConflict, "RACK_UNAVAILABLE", err.Error(), nil)
	case errors.Is(err, service.ErrApprovalState):
		response.Error(c, http.StatusConflict, "APPROVAL_STATE_CONFLICT", err.Error(), nil)
	case errors.Is(err, service.ErrRackDiagramImportDraft):
		response.Error(c, http.StatusGone, "RACK_DIAGRAM_IMPORT_DRAFT_EXPIRED", "导入校验已过期，请重新选择文件校验", nil)
	case errors.Is(err, service.ErrRackDiagramImportStale):
		response.Error(c, http.StatusConflict, "RACK_DIAGRAM_IMPORT_STALE", "校验后机柜或设备数据发生变化，请重新校验", nil)
	case errors.Is(err, service.ErrRackDiagramImportDecision):
		response.Error(c, http.StatusBadRequest, "RACK_DIAGRAM_IMPORT_DECISION_REQUIRED", err.Error(), nil)
	case errors.As(err, &conflict):
		response.Error(c, http.StatusConflict, "RACK_U_CONFLICT", conflict.Error(), gin.H{
			"rackId": conflict.RackID, "startU": conflict.StartU,
			"heightU": conflict.HeightU, "conflictDevices": conflict.Conflicts,
		})
	default:
		_ = c.Error(err)
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "系统内部错误", nil)
	}
}
