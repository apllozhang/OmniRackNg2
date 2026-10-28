package router

import (
	"omnirack-replica/backend/internal/handler"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestAssetExtensionRequiresAuthenticationBeforeAccessingData(t *testing.T) {
	r := gin.New()
	RegisterAssetOperations(r, handler.NewAssetOperationsHandler(nil), "test-extension-secret")
	for _, path := range []string{"/api/v1/asset-ops/sites", "/api/v1/asset-ops/devices", "/api/v1/asset-ops/stocks", "/api/v1/asset-ops/checks"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 401 {
			t.Fatalf("%s status %d", path, w.Code)
		}
	}
}

func TestApprovalEmailSettingsRequireAuthentication(t *testing.T) {
	r := gin.New()
	RegisterNotifications(r, handler.NewNotificationHandler(nil), handler.NewAssetOperationsHandler(nil), "test-extension-secret")
	for _, method := range []string{"GET", "PUT"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/v1/asset-ops/notifications/approval-settings", nil))
		if w.Code != 401 {
			t.Fatalf("%s approval settings status %d", method, w.Code)
		}
	}
}
