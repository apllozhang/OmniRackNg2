package config

import (
	"strings"
	"testing"
	"time"
)

func TestProductionConfigurationRejectsWeakOrAmbiguousValues(t *testing.T) {
	for _, test := range []struct {
		name, env, secret, password, ttl string
		valid                            bool
	}{
		{"production-valid", "production", strings.Repeat("x", 32), "admin123", "120", true},
		{"case-normalized", "Production", "weak-secret", "admin123", "120", false},
		{"whitespace-normalized", " production ", "weak-secret", "admin123", "120", false},
		{"unknown-env", "prod", strings.Repeat("x", 32), "admin123", "120", false},
		{"short-secret", "production", "custom-but-short", "admin123", "120", false},
		{"blank-secret", "production", strings.Repeat(" ", 32), "admin123", "120", false},
		{"default-admin", "production", strings.Repeat("x", 32), "ChangeMe123!", "120", false},
		{"overlong-admin", "production", strings.Repeat("x", 32), strings.Repeat("x", 73), "120", false},
		{"overflow-ttl", "production", strings.Repeat("x", 32), "admin123", "9223372036854775807", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("APP_ENV", test.env)
			t.Setenv("JWT_SECRET", test.secret)
			t.Setenv("ADMIN_PASSWORD", test.password)
			t.Setenv("JWT_EXPIRES_MINUTES", test.ttl)
			t.Setenv("ADMIN_USERNAME", "admin")
			cfg, err := Load()
			if test.valid && (err != nil || cfg.JWTExpiresIn != 120*time.Minute) {
				t.Fatalf("valid config rejected: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
}
