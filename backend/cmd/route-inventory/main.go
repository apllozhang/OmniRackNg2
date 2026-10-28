// route-inventory lists the routes registered by the application without opening
// a database, starting workers, or listening on a socket.
package main

import (
	"encoding/json"
	"flag"
	"os"
	"sort"

	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/handler"
	"omnirack-replica/backend/internal/router"
)

func main() {
	sourceContract := flag.Bool("source-contract", false, "include handler/source and JSON binding metadata (run from backend)")
	flag.Parse()
	gin.SetMode(gin.ReleaseMode)
	assets := &handler.AssetOperationsHandler{}
	r := router.New(&handler.AuthHandler{}, &handler.HealthHandler{},
		&handler.ResourceHandler{}, &handler.RackTemplateHandler{},
		&handler.DeviceHandler{}, &handler.PDUHandler{}, &handler.AdminHandler{},
		"inventory-only", assets.LegacyAccess())
	router.RegisterAssetOperations(r, assets, "inventory-only")
	router.RegisterNotifications(r, &handler.NotificationHandler{}, assets, "inventory-only")
	type route struct {
		Method  string `json:"method"`
		Path    string `json:"path"`
		Handler string `json:"handler,omitempty"`
	}
	routes := make([]route, 0, len(r.Routes()))
	for _, item := range r.Routes() {
		entry := route{Method: item.Method, Path: item.Path}
		if *sourceContract {
			entry.Handler = item.Handler
		}
		routes = append(routes, entry)
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Path == routes[j].Path {
			return routes[i].Method < routes[j].Method
		}
		return routes[i].Path < routes[j].Path
	})
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	var output interface{} = routes
	if *sourceContract {
		metadata, err := readSourceContract("internal")
		if err != nil {
			panic(err)
		}
		metadata["routes"] = routes
		output = metadata
	}
	if err := encoder.Encode(output); err != nil {
		panic(err)
	}
}
