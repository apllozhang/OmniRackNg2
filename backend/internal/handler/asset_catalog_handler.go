package handler

import (
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *AssetOperationsHandler) UpdateSite(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in model.AssetSite
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.UpdateSite(c.Request.Context(), actorFromContext(c), id, in)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) UpdateSpec(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in model.SpareSpec
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.UpdateSpec(c.Request.Context(), actorFromContext(c), id, in)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) UpdateStock(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in service.StockUpdateInput
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.UpdateStock(c.Request.Context(), actorFromContext(c), id, in)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) UpdateMinimum(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Version uint `json:"version"`
		Minimum int  `json:"minimum"`
	}
	if c.ShouldBindJSON(&in) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.UpdateMinimum(c.Request.Context(), actorFromContext(c), id, in.Version, in.Minimum)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) DeleteCatalog(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, ok := parseID(c, "id")
		if !ok {
			return
		}
		version, ok := parseVersion(c)
		if !ok {
			return
		}
		if err := h.ops.DeleteCatalog(c.Request.Context(), actorFromContext(c), kind, id, version); err != nil {
			writeResourceError(c, err)
			return
		}
		response.Success(c, gin.H{"deleted": true, "id": id})
	}
}
