package handler

import (
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAssetAccessUsesCurrentDatabaseRoleInsteadOfOldToken(t *testing.T) {
	tests := []struct {
		name, role, method, path string
		status                   int
	}{
		{"manager can read scan entry", "asset_manager", "GET", "/api/v1/asset-ops/access/settings", 200},
		{"auditor can read scan entry", "asset_auditor", "GET", "/api/v1/asset-ops/access/settings", 200},
		{"manager cannot change scan entry", "asset_manager", "PUT", "/api/v1/asset-ops/access/settings", 403},
		{"admin can change scan entry", "system_admin", "PUT", "/api/v1/asset-ops/access/settings", 200},
		{"ordinary user denied", "user", "GET", "/api/v1/asset-ops/devices", 403},
		{"auditor can read", "asset_auditor", "GET", "/api/v1/asset-ops/devices", 200},
		{"auditor cannot mutate", "asset_auditor", "POST", "/api/v1/asset-ops/stocks", 403},
		{"auditor can export", "asset_auditor", "POST", "/api/v1/asset-ops/export", 200},
		{"auditor cannot confirm PC", "asset_auditor", "POST", "/api/v1/asset-ops/devices/test/pc-handover", 403},
		{"manager can confirm PC", "asset_manager", "POST", "/api/v1/asset-ops/devices/test/pc-handover", 200},
		{"auditor can inspect PC history", "asset_auditor", "GET", "/api/v1/asset-ops/devices/test/pc-history", 200},
		{"ordinary user cannot inspect PC history", "user", "GET", "/api/v1/asset-ops/devices/test/pc-history", 403},
		{"auditor cannot register terminal", "asset_auditor", "POST", "/api/v1/asset-ops/terminal-devices", 403},
		{"manager can register terminal", "asset_manager", "POST", "/api/v1/asset-ops/terminal-devices", 200},
		{"ordinary user cannot inspect terminal state", "user", "GET", "/api/v1/asset-ops/devices/test/terminal-state", 403},
		{"auditor can inspect terminal state", "asset_auditor", "GET", "/api/v1/asset-ops/devices/test/terminal-state", 200},
		{"manager can mutate", "asset_manager", "POST", "/api/v1/asset-ops/stocks", 200},
		{"auditor cannot delete site", "asset_auditor", "DELETE", "/api/v1/asset-ops/sites/test", 403},
		{"auditor cannot edit stock", "asset_auditor", "PUT", "/api/v1/asset-ops/stocks/test", 403},
		{"manager can delete catalog", "asset_manager", "DELETE", "/api/v1/asset-ops/specs/test", 200},
		{"auditor cannot mutate legacy", "asset_auditor", "POST", "/api/v1/devices", 403},
		{"auditor can read legacy", "asset_auditor", "GET", "/api/v1/devices", 200},
		{"auditor can log out", "asset_auditor", "POST", "/api/v1/auth/logout", 200},
		{"manager cannot read notification credentials", "asset_manager", "GET", "/api/v1/asset-ops/notifications/settings", 403},
		{"auditor cannot configure notifications", "asset_auditor", "PUT", "/api/v1/asset-ops/notifications/settings", 403},
		{"current admin can configure notifications", "system_admin", "PUT", "/api/v1/asset-ops/notifications/settings", 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sqlDB, m, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer sqlDB.Close()
			db, e := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			if e != nil {
				t.Fatal(e)
			}
			uid, rid := uuid.New(), uuid.New()
			m.ExpectQuery(`SELECT .*users`).WillReturnRows(sqlmock.NewRows([]string{"id", "username", "enabled"}).AddRow(uid, "test", true))
			m.ExpectQuery(`SELECT .*user_roles`).WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}).AddRow(uid, rid))
			m.ExpectQuery(`SELECT .*roles`).WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(rid, tt.role))
			h := NewAssetOperationsHandler(service.NewAssetOperations(db))
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(middleware.UserIDKey, uid.String())
				c.Set(middleware.RoleKey, []string{"system_admin"})
				c.Next()
			})
			guard := h.CurrentAccess()
			if !strings.HasPrefix(tt.path, "/api/v1/asset-ops/") {
				guard = h.LegacyAccess()
			}
			if strings.Contains(tt.path, "/notifications/") || (strings.Contains(tt.path, "/access/") && tt.method == "PUT") {
				r.Handle(tt.method, tt.path, guard, middleware.RequireAdmin(), func(c *gin.Context) { c.Status(200) })
			} else {
				r.Handle(tt.method, tt.path, guard, func(c *gin.Context) { c.Status(200) })
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
			if w.Code != tt.status {
				t.Fatalf("got %d want %d: %s", w.Code, tt.status, w.Body.String())
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
