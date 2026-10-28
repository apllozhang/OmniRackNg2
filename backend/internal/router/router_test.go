package router

import (
	"testing"

	"omnirack-replica/backend/internal/handler"
)

func TestNewRegistersAllRoutesWithoutWildcardConflict(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("router registration panicked: %v", recovered)
		}
	}()

	_ = New(
		(*handler.AuthHandler)(nil),
		(*handler.HealthHandler)(nil),
		(*handler.ResourceHandler)(nil),
		(*handler.RackTemplateHandler)(nil),
		(*handler.DeviceHandler)(nil),
		(*handler.PDUHandler)(nil),
		(*handler.AdminHandler)(nil),
		"router-test-jwt-secret",
	)
}
