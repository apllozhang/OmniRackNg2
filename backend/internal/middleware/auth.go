package middleware

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"omnirack-replica/backend/internal/response"
)

const (
	UserIDKey = "userID"
	RoleKey   = "roles"
)

type Claims struct {
	UserID      string   `json:"userId"`
	Roles       []string `json:"roles"`
	UserVersion uint     `json:"userVersion,omitempty"`
	jwt.RegisteredClaims
}

// IdentityResolver checks live account state after signature validation and
// returns the current roles. Production routes must supply a resolver so that
// deleted/disabled accounts and revoked sessions stop working immediately.
type IdentityResolver func(context.Context, *Claims) ([]string, error)

var ErrIdentityUnavailable = errors.New("identity backend unavailable")

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = "req_" + uuid.NewString()
		}
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > 100 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || strings.ContainsRune("._:-", char)) {
			return false
		}
	}
	return true
}

func Auth(secret string, resolvers ...IdentityResolver) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") || len(header) > 8192 {
			response.Error(c, 401, "UNAUTHORIZED", "未登录或登录已过期", nil)
			c.Abort()
			return
		}
		token, err := jwt.ParseWithClaims(strings.TrimPrefix(header, "Bearer "), &Claims{}, func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if err != nil || !token.Valid {
			response.Error(c, 401, "UNAUTHORIZED", "登录凭证无效", nil)
			c.Abort()
			return
		}
		claims, ok := token.Claims.(*Claims)
		if !ok || claims.UserID == "" || claims.Subject != claims.UserID || claims.IssuedAt == nil {
			response.Error(c, 401, "UNAUTHORIZED", "登录凭证无效", nil)
			c.Abort()
			return
		}
		roles := claims.Roles
		if len(resolvers) > 0 {
			if resolvers[0] == nil {
				response.Error(c, 401, "UNAUTHORIZED", "登录凭证无效", nil)
				c.Abort()
				return
			}
			roles, err = resolvers[0](c.Request.Context(), claims)
			if err != nil {
				if errors.Is(err, ErrIdentityUnavailable) {
					response.Error(c, 503, "AUTH_UNAVAILABLE", "认证服务暂时不可用，请稍后重试", nil)
					c.Abort()
					return
				}
				response.Error(c, 401, "UNAUTHORIZED", "登录凭证已失效，请重新登录", nil)
				c.Abort()
				return
			}
		}
		c.Set(UserIDKey, claims.UserID)
		c.Set(RoleKey, roles)
		c.Next()
	}
}

func NewToken(secret, userID string, roles []string, expiresIn time.Duration) (string, error) {
	return NewTokenWithVersion(secret, userID, roles, 1, expiresIn)
}

func NewTokenWithVersion(secret, userID string, roles []string, version uint, expiresIn time.Duration) (string, error) {
	if secret == "" || userID == "" || version == 0 || expiresIn <= 0 {
		return "", errors.New("invalid token parameters")
	}
	now := time.Now()
	claims := Claims{
		UserID:      userID,
		Roles:       roles,
		UserVersion: version,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiresIn)),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}
