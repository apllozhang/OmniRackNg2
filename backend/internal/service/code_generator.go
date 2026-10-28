package service

import (
	"strings"

	"github.com/google/uuid"
)

const autoCodeRandomLength = 12

func generateAutoCode(prefix string) string {
	compact := strings.ReplaceAll(uuid.NewString(), "-", "")
	return strings.ToUpper(prefix + "-" + compact[:autoCodeRandomLength])
}
