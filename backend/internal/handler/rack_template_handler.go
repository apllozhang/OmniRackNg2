package handler

import (
	"net/http"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type RackTemplateHandler struct{ service *service.RackTemplateService }

func NewRackTemplateHandler(templateService *service.RackTemplateService) *RackTemplateHandler {
	return &RackTemplateHandler{service: templateService}
}

func (h *RackTemplateHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "系统内部错误", nil)
		return
	}
	response.Success(c, gin.H{"items": items})
}

func (h *RackTemplateHandler) Create(c *gin.Context) {
	var input service.RackTemplateCreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.Create(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *RackTemplateHandler) Update(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var input service.RackTemplateUpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	updated, err := h.service.Update(c.Request.Context(), actorFromContext(c), id, input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *RackTemplateHandler) CreateVersion(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	var input model.RackTemplateVersion
	if err := c.ShouldBindJSON(&input); err != nil {
		invalidJSON(c)
		return
	}
	created, err := h.service.CreateVersion(c.Request.Context(), actorFromContext(c), id, version, input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Created(c, created)
}

func (h *RackTemplateHandler) Delete(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	version, ok := parseVersion(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.Request.Context(), actorFromContext(c), id, version); err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true, "id": id})
}
