package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewToken(t *testing.T) {
	tokenString, err := NewToken("test-secret", "user-1", []string{"system_admin"}, time.Hour)
	if err != nil {
		t.Fatalf("NewToken returned error: %v", err)
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) { return []byte("test-secret"), nil })
	if err != nil || !token.Valid {
		t.Fatalf("token should be valid: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Fatalf("unexpected user id: %s", claims.UserID)
	}
}
