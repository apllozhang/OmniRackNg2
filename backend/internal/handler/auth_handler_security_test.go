package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/config"
	"omnirack-replica/backend/internal/service"
)

func TestLoginRateLimitsAccountBeforeMoreAuthenticationWork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(service.NewAuthService(db, config.Config{}))
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.POST("/login", h.Login)
	for i := 0; i < 11; i++ {
		if i < 10 {
			mock.ExpectQuery(`SELECT .* FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"target","password":"wrong123"}`))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		want := http.StatusUnauthorized
		if i == 10 {
			want = http.StatusTooManyRequests
		}
		if res.Code != want {
			t.Fatalf("attempt %d = %d, want %d: %s", i+1, res.Code, want, res.Body.String())
		}
		if i == 10 && res.Header().Get("Retry-After") == "" {
			t.Fatal("missing retry hint")
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLoginBoundsBodyAndIPWork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(service.NewAuthService(nil, config.Config{}))
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	r.POST("/login", h.Login)
	for i := 0; i < 121; i++ {
		body := "invalid-json"
		if i == 0 {
			body = `{"username":"` + strings.Repeat("x", 17<<10) + `","password":"wrong123"}`
		}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("X-Forwarded-For", "198.51.100."+strings.Repeat("1", i%3+1))
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		want := http.StatusBadRequest
		if i == 0 {
			want = http.StatusRequestEntityTooLarge
		}
		if i == 120 {
			want = http.StatusTooManyRequests
		}
		if res.Code != want {
			t.Fatalf("attempt %d = %d, want %d: %s", i+1, res.Code, want, res.Body.String())
		}
	}
}

func TestNilAuthHandlerFailsClosedWithoutPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var h *AuthHandler
	r := gin.New()
	r.GET("/", h.AuthMiddleware(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", res.Code)
	}
}
