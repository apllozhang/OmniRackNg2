package service

import (
	"fmt"
	"net"
	"strings"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
)

func normalizeDeviceType(item *model.DeviceType) {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Category = strings.ToUpper(strings.TrimSpace(item.Category))
	item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = model.DeviceTypeStatusActive
	}
	if item.DefaultHeightU == 0 {
		item.DefaultHeightU = 1
	}
	item.Description = strings.TrimSpace(item.Description)
}

func validateDeviceType(item model.DeviceType) error {
	if item.Code == "" || len(item.Code) > 50 {
		return fmt.Errorf("%w: 设备类型编码不能为空且不能超过 50 个字符", ErrInvalidResource)
	}
	if item.Name == "" || len(item.Name) > 150 {
		return fmt.Errorf("%w: 设备类型名称不能为空且不能超过 150 个字符", ErrInvalidResource)
	}
	if !validDeviceCategory(item.Category) {
		return fmt.Errorf("%w: 设备分类无效", ErrInvalidResource)
	}
	if item.Status != model.DeviceTypeStatusActive && item.Status != model.DeviceTypeStatusDisabled {
		return fmt.Errorf("%w: 设备类型状态无效", ErrInvalidResource)
	}
	if item.DefaultHeightU < 1 || item.DefaultHeightU > 100 {
		return fmt.Errorf("%w: 默认 U 高度必须是 1 到 100 的整数", ErrInvalidResource)
	}
	if negative(item.DefaultWeightKg, item.DefaultRatedPowerW, item.DefaultPeakPowerW) {
		return fmt.Errorf("%w: 重量和功率不能为负数", ErrInvalidResource)
	}
	return nil
}

func applyDeviceTypeDefaults(item *model.Device, typeItem model.DeviceType) {
	if item.HeightU == 0 {
		item.HeightU = typeItem.DefaultHeightU
	}
	if item.WeightKg == nil {
		item.WeightKg = typeItem.DefaultWeightKg
	}
	if item.RatedPowerW == nil {
		item.RatedPowerW = typeItem.DefaultRatedPowerW
	}
	if item.PeakPowerW == nil {
		item.PeakPowerW = typeItem.DefaultPeakPowerW
	}
	if !item.DualPowerRequired {
		item.DualPowerRequired = typeItem.DefaultDualPower
	}
}

func normalizeDevice(item *model.Device) {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.AssetNumber = strings.TrimSpace(item.AssetNumber)
	item.SerialNumber = strings.TrimSpace(item.SerialNumber)
	item.Manufacturer = strings.TrimSpace(item.Manufacturer)
	item.ModelNumber = strings.TrimSpace(item.ModelNumber)
	item.Specification = strings.TrimSpace(item.Specification)
	item.FirmwareVersion = strings.TrimSpace(item.FirmwareVersion)
	item.PurchaseBatch = strings.TrimSpace(item.PurchaseBatch)
	item.Organization = strings.TrimSpace(item.Organization)
	item.Manager = strings.TrimSpace(item.Manager)
	item.Contact = strings.TrimSpace(item.Contact)
	item.BusinessSystem = strings.TrimSpace(item.BusinessSystem)
	item.ApplicationName = strings.TrimSpace(item.ApplicationName)
	item.LifecycleStatus = strings.ToUpper(strings.TrimSpace(item.LifecycleStatus))
	if item.LifecycleStatus == "" {
		item.LifecycleStatus = model.DeviceStatusWaitingRack
	}
	item.ManagementIP = strings.TrimSpace(item.ManagementIP)
	item.BusinessIP = strings.TrimSpace(item.BusinessIP)
	item.MACAddress = strings.ToUpper(strings.TrimSpace(item.MACAddress))
	item.ManagementProtocol = strings.TrimSpace(item.ManagementProtocol)
	item.MonitoringStatus = strings.TrimSpace(item.MonitoringStatus)
	item.ExternalQRCode = strings.TrimSpace(item.ExternalQRCode)
	item.ExternalQRCodeURL = strings.TrimSpace(item.ExternalQRCodeURL)
	item.Tags = strings.TrimSpace(item.Tags)
	item.Remarks = strings.TrimSpace(item.Remarks)
}

func validateDevice(item model.Device) error {
	if item.TypeID == uuid.Nil {
		return fmt.Errorf("%w: 必须选择设备类型", ErrInvalidResource)
	}
	if item.Code == "" || len(item.Code) > 80 {
		return fmt.Errorf("%w: 设备编码不能为空且不能超过 80 个字符", ErrInvalidResource)
	}
	if item.Name == "" || len(item.Name) > 150 {
		return fmt.Errorf("%w: 设备名称不能为空且不能超过 150 个字符", ErrInvalidResource)
	}
	if !validDeviceStatus(item.LifecycleStatus) {
		return fmt.Errorf("%w: 设备生命周期状态无效", ErrInvalidResource)
	}
	if item.HeightU < 1 || item.HeightU > 100 {
		return fmt.Errorf("%w: 设备 U 高度必须是 1 到 100 的整数", ErrInvalidResource)
	}
	if item.WidthMm != nil && *item.WidthMm <= 0 || item.DepthMm != nil && *item.DepthMm <= 0 || item.HeightMm != nil && *item.HeightMm <= 0 {
		return fmt.Errorf("%w: 设备尺寸必须大于 0", ErrInvalidResource)
	}
	if negative(item.WeightKg, item.RatedPowerW, item.PeakPowerW, item.InputVoltage) {
		return fmt.Errorf("%w: 重量、电力参数不能为负数", ErrInvalidResource)
	}
	for _, address := range []string{item.ManagementIP, item.BusinessIP} {
		if address != "" && net.ParseIP(address) == nil {
			return fmt.Errorf("%w: IP 地址格式无效", ErrInvalidResource)
		}
	}
	return nil
}

func normalizePositionInput(input *PositionChangeInput) {
	input.Orientation = strings.ToUpper(strings.TrimSpace(input.Orientation))
	if input.Orientation == "" {
		input.Orientation = model.DeviceOrientationNormal
	}
	input.Reason = strings.TrimSpace(input.Reason)
}

func validatePositionInput(input PositionChangeInput) error {
	if input.Version == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	if input.RackID == uuid.Nil {
		return fmt.Errorf("%w: 必须选择目标机柜", ErrInvalidResource)
	}
	if input.StartU < 1 || input.HeightU < 1 {
		return fmt.Errorf("%w: 起始 U 和占用 U 必须是正整数", ErrInvalidResource)
	}
	if input.Orientation != model.DeviceOrientationNormal && input.Orientation != model.DeviceOrientationReverse {
		return fmt.Errorf("%w: 设备方向无效", ErrInvalidResource)
	}
	return nil
}

func occupancyRows(position model.RackDevicePosition) []model.RackUOccupancy {
	return []model.RackUOccupancy{{PositionID: position.ID, DeviceID: position.DeviceID, RackID: position.RackID, StartU: position.StartU, EndU: position.EndU}}
}

func findConflicts(positions []model.RackDevicePosition, deviceID uuid.UUID, input PositionChangeInput) []model.RackDevicePosition {
	endU := input.StartU + input.HeightU - 1
	items := make([]model.RackDevicePosition, 0)
	for _, position := range positions {
		if position.DeviceID == deviceID {
			continue
		}
		if position.StartU <= endU && input.StartU <= position.EndU {
			items = append(items, position)
		}
	}
	return items
}

func positionSnapshot(position model.RackDevicePosition) model.DevicePositionSnapshot {
	return model.DevicePositionSnapshot{PositionID: position.ID, RackID: position.RackID, DataCenterID: position.DataCenterID, RoomID: position.RoomID, StartU: position.StartU, HeightU: position.HeightU, EndU: position.EndU, Orientation: position.Orientation}
}

func availableRanges(height int, occupancies []model.RackUOccupancy) []RackFreeRange {
	used := make([]bool, height+1)
	for _, item := range occupancies {
		start, end := item.StartU, item.EndU
		if start < 1 {
			start = 1
		}
		if end > height {
			end = height
		}
		for u := start; u <= end; u++ {
			used[u] = true
		}
	}
	items := make([]RackFreeRange, 0)
	for u := 1; u <= height; {
		if used[u] {
			u++
			continue
		}
		start := u
		for u <= height && !used[u] {
			u++
		}
		items = append(items, RackFreeRange{StartU: start, EndU: u - 1, HeightU: u - start})
	}
	return items
}

func validDeviceCategory(category string) bool {
	switch category {
	case model.DeviceCategoryPC, model.DeviceCategoryServer, model.DeviceCategoryNetwork, model.DeviceCategoryStorage, model.DeviceCategorySecurity, model.DeviceCategoryPowerEnv, model.DeviceCategoryAccessory, model.DeviceCategoryOther:
		return true
	default:
		return false
	}
}

func validDeviceStatus(status string) bool {
	switch status {
	case model.DeviceStatusWaitingRack, model.DeviceStatusRunning, model.DeviceStatusMaintenance, model.DeviceStatusPendingRemoval, model.DeviceStatusOffRack, model.DeviceStatusScrapped:
		return true
	default:
		return false
	}
}
