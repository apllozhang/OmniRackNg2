package repository

import (
	"errors"
	"strings"
)

var ErrInvalidDeviceSort = errors.New("invalid device sort")

// DeviceSortExpressions is the complete allowlist. SQL never comes from a caller.
func DeviceSortExpressions() map[string]string {
	return map[string]string{
		"code": "NULLIF(devices.code, '')", "name": "NULLIF(devices.name, '')",
		"typeName":    "(SELECT NULLIF(t.name, '') FROM device_types t WHERE t.id=devices.type_id AND t.deleted_at IS NULL)",
		"assetNumber": "NULLIF(devices.asset_number, '')", "heightU": "devices.height_u",
		"managementIp": "NULLIF(devices.management_ip, '')", "createdAt": "devices.created_at",
		"lifecycleStatus": "CASE devices.lifecycle_status WHEN 'WAITING_RACK' THEN '待上架' WHEN 'RUNNING' THEN '运行中' WHEN 'MAINTENANCE' THEN '维护中' WHEN 'PENDING_REMOVAL' THEN '待下架' WHEN 'OFF_RACK' THEN '已下架' WHEN 'SCRAPPED' THEN '已报废' ELSE devices.lifecycle_status END",
		"location":        "(SELECT ROW(r.name,r.code,p.start_u,p.end_u) FROM rack_device_positions p JOIN racks r ON r.id=p.rack_id AND r.deleted_at IS NULL WHERE p.device_id=devices.id AND p.deleted_at IS NULL)",
	}
}

func DeviceOrder(query DeviceQuery) (string, error) {
	if query.SortBy == "" && query.SortOrder == "" {
		return "devices.created_at DESC, devices.id ASC", nil
	}
	expression, ok := DeviceSortExpressions()[query.SortBy]
	if !ok || (query.SortOrder != "asc" && query.SortOrder != "desc") {
		return "", ErrInvalidDeviceSort
	}
	return expression + " " + strings.ToUpper(query.SortOrder) + " NULLS LAST, devices.id ASC", nil
}
