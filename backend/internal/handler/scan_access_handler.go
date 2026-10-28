package handler

import (
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/response"
	"github.com/gin-gonic/gin"
)

func (h *AssetOperationsHandler) ScanAccess(c *gin.Context) {
	x, err := h.ops.ScanAccess(c.Request.Context())
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
func (h *AssetOperationsHandler) SaveScanAccess(c *gin.Context) {
	var input model.ScanAccessSettings
	if c.ShouldBindJSON(&input) != nil {
		invalidJSON(c)
		return
	}
	x, err := h.ops.SaveScanAccess(c.Request.Context(), actorFromContext(c), input)
	if err != nil {
		writeResourceError(c, err)
		return
	}
	response.Success(c, x)
}
