// route-inventory lists the routes registered by the application without opening
// a database, starting workers, or listening on a socket.
package main

import (
	"encoding/json"
	"os"
	"sort"

	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/router"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	assets := &handler.AssetOperationsHandler{}
	r := router.New(&handler.AuthHandler{}, &handler.HealthHandler{},
		&handler.ResourceHandler{}, &handler.RackTemplateHandler{},
		&handler.DeviceHandler{}, &handler.PDUHandler{}, &handler.AdminHandler{},
		"inventory-only", assets.LegacyAccess())
	router.RegisterAssetOperations(r, assets, "inventory-only")
	router.RegisterNotifications(r, &handler.NotificationHandler{}, assets, "inventory-only")
	type route struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	routes := make([]route, 0, len(r.Routes()))
	for _, item := range r.Routes() {
		routes = append(routes, route{item.Method, item.Path})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(routes); err != nil {
		panic(err)
	}
}
