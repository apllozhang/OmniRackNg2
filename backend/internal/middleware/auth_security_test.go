package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestAuthRequiresExpirySubjectIssuedAtAndHS256(t *testing.T) {
	gin.SetMode(gin.TestMode)
	now := time.Now()
	base := Claims{UserID: "user-1", Roles: []string{"user"}, RegisteredClaims: jwt.RegisteredClaims{
		Subject: "user-1", IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	for _, name := range []string{"missing-exp", "missing-iat", "wrong-subject", "future-iat", "hs512", "valid"} {
		t.Run(name, func(t *testing.T) {
			claims := base
			method := jwt.SigningMethodHS256
			switch name {
			case "missing-exp":
				claims.ExpiresAt = nil
			case "missing-iat":
				claims.IssuedAt = nil
			case "wrong-subject":
				claims.Subject = "another-user"
			case "future-iat":
				claims.IssuedAt = jwt.NewNumericDate(now.Add(time.Hour))
			case "hs512":
				method = jwt.SigningMethodHS512
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString([]byte("secret"))
			if err != nil {
				t.Fatal(err)
			}
			r := gin.New()
			r.GET("/", Auth("secret"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res := httptest.NewRecorder()
			r.ServeHTTP(res, req)
			want := http.StatusUnauthorized
			if name == "valid" {
				want = http.StatusNoContent
			}
			if res.Code != want {
				t.Fatalf("status = %d, want %d: %s", res.Code, want, res.Body.String())
			}
		})
	}
}

func TestAuthUsesLiveRolesAndFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := NewTokenWithVersion("secret", "user-1", []string{"system_admin"}, 7, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	for _, reject := range []bool{false, true} {
		r := gin.New()
		resolver := func(_ context.Context, claims *Claims) ([]string, error) {
			if claims.UserVersion != 7 {
				t.Fatal("account version missing from token")
			}
			if reject {
				return nil, errors.New("disabled, deleted, or revoked")
			}
			return []string{"user"}, nil
		}
		r.GET("/", Auth("secret", resolver), RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		want := http.StatusForbidden
		if reject {
			want = http.StatusUnauthorized
		}
		if res.Code != want {
			t.Fatalf("status = %d, want %d", res.Code, want)
		}
	}
}

func TestAuthDatabaseOutageFailsClosedWithRetryableStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token, err := NewToken("secret", "user-1", []string{"user"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/", Auth("secret", func(context.Context, *Claims) ([]string, error) { return nil, ErrIdentityUnavailable }), func(c *gin.Context) { t.Fatal("unverified request reached application") })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)
	if res.Code != http.StatusServiceUnavailable || !strings.Contains(res.Body.String(), "AUTH_UNAVAILABLE") {
		t.Fatalf("outage status: %d, %s", res.Code, res.Body.String())
	}
}

func TestRequestIDRejectsUnboundedAndUntrustedValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, value := range []string{strings.Repeat("x", 101), "contains spaces", "审计", "good-request_123:456"} {
		r := gin.New()
		r.GET("/", RequestID(), func(c *gin.Context) { c.Status(http.StatusNoContent) })
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-Request-ID", value)
		res := httptest.NewRecorder()
		r.ServeHTTP(res, req)
		actual := res.Header().Get("X-Request-ID")
		if !validRequestID(actual) {
			t.Fatalf("invalid generated request ID %q", actual)
		}
		if validRequestID(value) && actual != value {
			t.Fatal("valid correlation ID was replaced")
		}
	}
}

func TestAttemptLimiterRecoversAndBoundsMemory(t *testing.T) {
	limiter := NewAttemptLimiter(2, time.Minute, 2)
	now := time.Now()
	limiter.now = func() time.Time { return now }
	if !limiter.Allow("first") || !limiter.Allow("first") || limiter.Allow("first") {
		t.Fatal("attempt limit not enforced")
	}
	if !limiter.Allow("second") || limiter.Allow("third") || len(limiter.entries) != 2 {
		t.Fatal("entry cap not enforced")
	}
	now = now.Add(time.Minute)
	if !limiter.Allow("third") {
		t.Fatal("expired attempts did not release capacity")
	}
	limiter.Reset("third")
	if !limiter.Allow("third") {
		t.Fatal("successful login did not release account limit")
	}
}

func TestAttemptLimiterConcurrentRequestsShareOneBudget(t *testing.T) {
	limiter := NewAttemptLimiter(10, time.Minute, 10)
	var accepted atomic.Int32
	var requests sync.WaitGroup
	for i := 0; i < 100; i++ {
		requests.Add(1)
		go func() {
			defer requests.Done()
			if limiter.Allow("same-account") {
				accepted.Add(1)
			}
		}()
	}
	requests.Wait()
	if accepted.Load() != 10 {
		t.Fatalf("parallel requests admitted %d attempts, want10", accepted.Load())
	}
}
