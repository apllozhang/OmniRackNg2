package handler

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"omnirack-replica/backend/internal/response"
)

// Expressions come only from the caller's static allowlist, never from the URL.
// The ID tie-breaker makes pages deterministic when primary values are equal.
func tableOrder(c *gin.Context, key, direction string, allowed map[string]string, fallback string) (string, bool) {
	if key == "" && direction == "" {
		return fallback + ", id ASC", true
	}
	expression, ok := allowed[key]
	if !ok || (direction != "asc" && direction != "desc") {
		response.Error(c, http.StatusBadRequest, "INVALID_RESOURCE", "sortBy 或 sortOrder 无效", nil)
		return "", false
	}
	return expression + " " + strings.ToUpper(direction) + " NULLS LAST, id ASC", true
}
func tableSearch(c *gin.Context, value string) (string, bool) {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > 200 {
		response.Error(c, http.StatusBadRequest, "INVALID_RESOURCE", "search 最多 200 字符", nil)
		return "", false
	}
	// Treat SQL wildcard characters as literal user text, rather than widening matches.
	value = strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(value)
	return "%" + value + "%", true
}
