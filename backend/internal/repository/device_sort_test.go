package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDeviceSortingRejectsUntrustedQueriesBeforeDatabaseAccess(t *testing.T) {
	for _, query := range []DeviceQuery{{SortBy: "name;DROP TABLE devices", SortOrder: "asc"}, {SortBy: "name", SortOrder: "asc;SELECT 1"}, {SortBy: "name"}, {SortOrder: "asc"}} {
		if _, _, err := NewDeviceStore(nil).ListDevices(context.Background(), query); !errors.Is(err, ErrInvalidDeviceSort) {
			t.Fatalf("unsafe query reached DB: %v", err)
		}
	}
	for field := range DeviceSortExpressions() {
		for _, direction := range []string{"asc", "desc"} {
			order, err := DeviceOrder(DeviceQuery{SortBy: field, SortOrder: direction})
			if err != nil || !strings.HasSuffix(order, "NULLS LAST, devices.id ASC") {
				t.Fatalf("field %s order %s err %v", field, order, err)
			}
		}
	}
}
