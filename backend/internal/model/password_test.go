package model

import (
	"strings"
	"testing"
)

func TestPasswordByteBoundsAndOverlongLogin(t *testing.T) {
	for _, password := range []string{"", "short", strings.Repeat("x", 73), strings.Repeat("密", 25)} {
		if err := ValidatePassword(password); err == nil {
			t.Fatalf("invalid password length %d accepted", len(password))
		}
	}
	var user User
	password := strings.Repeat("a", MaxPasswordBytes)
	if err := user.SetPassword(password); err != nil {
		t.Fatal(err)
	}
	if err := user.CheckPassword(password); err != nil {
		t.Fatal(err)
	}
	if err := user.CheckPassword(password + "different-suffix"); err == nil {
		t.Fatal("overlong password authenticated by its 72-byte prefix")
	}
}
