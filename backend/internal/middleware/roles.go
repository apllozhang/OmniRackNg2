package middleware

import (
	"strings"

	"omnirack-replica/backend/internal/response"
	"github.com/gin-gonic/gin"
)

// RequireRole protects routes using the role codes embedded in the signed JWT.
// Role checks are deliberately enforced on the API, not only in the frontend.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get(RoleKey)
		if !exists {
			response.Error(c, 403, "FORBIDDEN", "当前用户没有访问权限", nil)
			c.Abort()
			return
		}
		roles, ok := raw.([]string)
		if !ok {
			response.Error(c, 403, "FORBIDDEN", "当前用户没有访问权限", nil)
			c.Abort()
			return
		}
		for _, item := range roles {
			if strings.EqualFold(strings.TrimSpace(item), role) {
				c.Next()
				return
			}
		}
		response.Error(c, 403, "FORBIDDEN", "当前用户没有访问权限", nil)
		c.Abort()
	}
}

func RequireAdmin() gin.HandlerFunc { return RequireRole("system_admin") }

func HasRole(roles []string, role string) bool {
	for _, item := range roles {
		if strings.EqualFold(strings.TrimSpace(item), role) {
			return true
		}
	}
	return false
}
