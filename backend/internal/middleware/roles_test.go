package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRequireAdminRejectsOrdinaryUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(RoleKey, []string{"user"}); c.Next() })
	r.GET("/admin-only", RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("ordinary user status = %d, want 403", res.Code)
	}
}

func TestRequireAdminAllowsSystemAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(RoleKey, []string{"system_admin"}); c.Next() })
	r.GET("/admin-only", RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodGet, "/admin-only", nil)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusNoContent {
		t.Fatalf("admin status = %d, want 204", res.Code)
	}
}
