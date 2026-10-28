package service

import (
	"regexp"
	"testing"
)

func TestGenerateAutoCode(t *testing.T) {
	pattern := regexp.MustCompile(`^[A-Z]+-[0-9A-F]{12}$`)
	for _, prefix := range []string{"DC", "ROOM", "RACK", "DEV"} {
		first := generateAutoCode(prefix)
		second := generateAutoCode(prefix)
		if !pattern.MatchString(first) {
			t.Fatalf("generated code %q does not match expected format", first)
		}
		if first == second {
			t.Fatalf("two generated codes unexpectedly matched: %q", first)
		}
	}
}
