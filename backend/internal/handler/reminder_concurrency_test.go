package handler

import (
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/service"
)

func TestAcknowledgeCannotResurrectReminderClearedByRefresh(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	id, userID := uuid.New(), uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .*asset_reminders.*FOR UPDATE`).WillReturnRows(sqlmock.NewRows([]string{"id", "active", "version"}).AddRow(id, false, 3))
	mock.ExpectRollback()
	router := gin.New()
	router.POST("/reminders/:id/acknowledge", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, userID.String())
		c.Next()
	}, NewAssetOperationsHandler(service.NewAssetOperations(db)).Acknowledge)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("POST", "/reminders/"+id.String()+"/acknowledge", strings.NewReader(`{"note":"已核对"}`)))
	if response.Code < 400 {
		t.Fatalf("acknowledged an alert that no longer exists: %d %s", response.Code, response.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAuditPaginationRejectsHugeOffsetBeforeDatabaseAccess(t *testing.T) {
	for _, page := range []int{10001, int(^uint(0) >> 1)} {
		router := gin.New()
		router.GET("/audit", NewAssetOperationsHandler(service.NewAssetOperations(nil)).Audit)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/audit?page="+strconv.Itoa(page), nil))
		if response.Code != 400 || !strings.Contains(response.Body.String(), "INVALID_RESOURCE") {
			t.Fatalf("unsafe page %d reached database: %d %s", page, response.Code, response.Body.String())
		}
	}
}
