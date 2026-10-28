package middleware

import (
	"strings"
	"time"

	"omnirack-replica/backend/internal/response"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	UserIDKey = "userID"
	RoleKey   = "roles"
)

type Claims struct {
	UserID string   `json:"userId"`
	Roles  []string `json:"roles"`
	jwt.RegisteredClaims
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = "req_" + uuid.NewString()
		}
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			response.Error(c, 401, "UNAUTHORIZED", "未登录或登录已过期", nil)
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			response.Error(c, 401, "UNAUTHORIZED", "登录凭证无效", nil)
			c.Abort()
			return
		}
		claims, ok := token.Claims.(*Claims)
		if !ok {
			response.Error(c, 401, "UNAUTHORIZED", "登录凭证无效", nil)
			c.Abort()
			return
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(RoleKey, claims.Roles)
		c.Next()
	}
}

func NewToken(secret, userID string, roles []string, expiresIn time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Roles:  roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
