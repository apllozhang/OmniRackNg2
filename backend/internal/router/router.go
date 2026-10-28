package router

import (
	"net/http"

	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/response"
	"github.com/gin-gonic/gin"
)

func New(authHandler *handler.AuthHandler, healthHandler *handler.HealthHandler, resourceHandler *handler.ResourceHandler, rackTemplateHandler *handler.RackTemplateHandler, deviceHandler *handler.DeviceHandler, pduHandler *handler.PDUHandler, adminHandler *handler.AdminHandler, jwtSecret string, currentAccess ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	// 默认不信任任何反向代理，避免客户端伪造 X-Forwarded-For。
	// 部署在受控代理后时，再按实际代理网段配置 SetTrustedProxies。
	_ = r.SetTrustedProxies(nil)
	r.Use(gin.Recovery(), middleware.RequestID(), middleware.RequestLog())
	r.GET("/health/live", healthHandler.Live)
	r.GET("/health/ready", healthHandler.Ready)
	v1 := r.Group("/api/v1")
	v1.POST("/auth/login", authHandler.Login)
	protected := v1.Group("")
	protected.Use(middleware.Auth(jwtSecret))
	if len(currentAccess) > 0 {
		protected.Use(currentAccess...)
	}
	protected.GET("/auth/me", authHandler.Me)
	protected.POST("/auth/logout", authHandler.Logout)

	protected.GET("/resource-tree", resourceHandler.Tree)
	protected.POST("/data-centers", resourceHandler.CreateDataCenter)
	protected.POST("/data-centers/:dataCenterId/copy", resourceHandler.CopyDataCenter)
	protected.PUT("/data-centers/:id", resourceHandler.UpdateDataCenter)
	protected.DELETE("/data-centers/:id", resourceHandler.DeleteDataCenter)
	protected.POST("/data-centers/:dataCenterId/rooms", resourceHandler.CreateRoom)
	protected.PUT("/rooms/:id", resourceHandler.UpdateRoom)
	protected.POST("/rooms/:roomId/copy", resourceHandler.CopyRoom)
	protected.POST("/rooms/:roomId/move", resourceHandler.MoveRoom)
	protected.DELETE("/rooms/:id", resourceHandler.DeleteRoom)
	protected.POST("/rooms/:roomId/racks", resourceHandler.CreateRack)
	protected.PUT("/racks/:id", resourceHandler.UpdateRack)
	protected.POST("/racks/:rackId/copy", resourceHandler.CopyRack)
	protected.POST("/racks/:rackId/move", resourceHandler.MoveRack)
	protected.DELETE("/racks/:id", resourceHandler.DeleteRack)
	protected.GET("/racks/:rackId/u-layout", deviceHandler.RackLayout)
	protected.GET("/racks/:rackId/pdus", pduHandler.List)
	protected.POST("/racks/:rackId/pdus", pduHandler.Create)
	protected.GET("/racks/:rackId/pdu-connections", pduHandler.ListConnections)
	protected.GET("/pdus/:id", pduHandler.Get)
	protected.PUT("/pdus/:id", pduHandler.Update)
	protected.DELETE("/pdus/:id", pduHandler.Delete)
	protected.GET("/pdus/:id/sockets", pduHandler.ListSockets)
	protected.POST("/pdus/:id/sockets", pduHandler.CreateSocket)
	protected.PUT("/pdu-sockets/:id", pduHandler.UpdateSocket)
	protected.DELETE("/pdu-sockets/:id", pduHandler.DeleteSocket)
	protected.POST("/pdu-sockets/:id/connection", pduHandler.Connect)
	protected.DELETE("/pdu-connections/:id", pduHandler.Disconnect)
	protected.POST("/rooms/:roomId/rack-diagram-import/validate", deviceHandler.ValidateRackDiagramImport)
	protected.POST("/rooms/:roomId/rack-diagram-import/commit", deviceHandler.CommitRackDiagramImport)

	protected.GET("/rack-templates", rackTemplateHandler.List)
	protected.POST("/rack-templates", rackTemplateHandler.Create)
	protected.PUT("/rack-templates/:id", rackTemplateHandler.Update)
	protected.DELETE("/rack-templates/:id", rackTemplateHandler.Delete)
	protected.POST("/rack-templates/:id/versions", rackTemplateHandler.CreateVersion)

	protected.GET("/device-types", deviceHandler.ListDeviceTypes)
	protected.POST("/device-types", deviceHandler.CreateDeviceType)
	protected.PUT("/device-types/:id", deviceHandler.UpdateDeviceType)
	protected.DELETE("/device-types/:id", deviceHandler.DeleteDeviceType)
	protected.GET("/devices", deviceHandler.ListDevices)
	protected.GET("/devices/:id", deviceHandler.GetDevice)
	protected.POST("/devices", deviceHandler.CreateDevice)
	protected.PUT("/devices/:id", deviceHandler.UpdateDevice)
	protected.DELETE("/devices/:id", deviceHandler.DeleteDevice)
	protected.POST("/devices/:id/assign", deviceHandler.Assign)
	protected.POST("/devices/:id/move", deviceHandler.Move)
	protected.POST("/devices/:id/decommission", deviceHandler.Decommission)
	protected.GET("/devices/:id/history", deviceHandler.History)

	// System administration is a separate, server-enforced capability.
	admin := protected.Group("/admin")
	admin.Use(middleware.RequireAdmin())
	admin.GET("/users", adminHandler.ListUsers)
	admin.POST("/users", adminHandler.CreateUser)
	admin.PUT("/users/:id", adminHandler.UpdateUser)
	admin.POST("/users/:id/reset-password", adminHandler.ResetPassword)
	admin.DELETE("/users/:id", adminHandler.DeleteUser)
	admin.GET("/roles", adminHandler.ListRoles)
	admin.GET("/approval-policy", adminHandler.GetPolicy)
	admin.PUT("/approval-policy", adminHandler.UpdatePolicy)
	admin.GET("/approvals", adminHandler.ListApprovals)
	admin.POST("/approvals/:id/approve", adminHandler.Approve)
	admin.POST("/approvals/:id/reject", adminHandler.Reject)
	admin.GET("/ldap", adminHandler.GetLDAP)
	admin.PUT("/ldap", adminHandler.UpdateLDAP)
	admin.POST("/ldap/test", adminHandler.TestLDAP)

	// Keep the former approval URLs as protected compatibility aliases. They
	// deliberately sit behind RequireAdmin so ordinary users receive 403 rather
	// than silently retaining access through an old bookmarked endpoint.
	protected.GET("/settings/operation-policy", middleware.RequireAdmin(), adminHandler.GetPolicy)
	protected.PUT("/settings/operation-policy", middleware.RequireAdmin(), adminHandler.UpdatePolicy)
	protected.GET("/approvals", middleware.RequireAdmin(), adminHandler.ListApprovals)
	protected.POST("/approvals/:id/approve", middleware.RequireAdmin(), adminHandler.Approve)
	protected.POST("/approvals/:id/reject", middleware.RequireAdmin(), adminHandler.Reject)

	r.NoRoute(func(c *gin.Context) { response.Error(c, http.StatusNotFound, "NOT_FOUND", "接口不存在", nil) })
	return r
}
