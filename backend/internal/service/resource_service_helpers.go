package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func applyRackTemplate(item *model.Rack, version model.RackTemplateVersion) {
	templateID := version.TemplateID
	versionID := version.ID
	item.TemplateID = &templateID
	item.TemplateVersionID = &versionID
	item.TemplateCode = version.Template.Code
	item.TemplateName = version.Template.Name
	item.TemplateRevision = version.Revision
	item.Type = version.Type
	item.Manufacturer = version.Manufacturer
	item.ModelNumber = version.ModelNumber
	item.UHeight = version.UHeight
	item.WidthMm = version.WidthMm
	item.DepthMm = version.DepthMm
	item.HeightMm = version.HeightMm
	item.LoadCapacityKg = version.LoadCapacityKg
	item.DualPower = version.DualPower
	item.InputCircuits = version.InputCircuits
	item.RatedVoltage = version.RatedVoltage
	item.RatedCurrent = version.RatedCurrent
	item.RatedPowerKw = version.RatedPowerKw
	item.PeakPowerKw = version.PeakPowerKw
	item.PDUCount = version.PDUCount
	item.TemplateSnapshot = model.RackTemplateSnapshot{
		TemplateID: version.TemplateID, VersionID: version.ID,
		Code: version.Template.Code, Name: version.Template.Name, Revision: version.Revision,
		Type: version.Type, Manufacturer: version.Manufacturer, ModelNumber: version.ModelNumber,
		UHeight: version.UHeight, WidthMm: version.WidthMm, DepthMm: version.DepthMm, HeightMm: version.HeightMm,
		LoadCapacityKg: version.LoadCapacityKg, DualPower: version.DualPower, InputCircuits: version.InputCircuits,
		RatedVoltage: version.RatedVoltage, RatedCurrent: version.RatedCurrent,
		RatedPowerKw: version.RatedPowerKw, PeakPowerKw: version.PeakPowerKw, PDUCount: version.PDUCount,
	}
}

// applyRackSpecOverride keeps the immutable template source/snapshot while applying
// the instance values the user confirmed in the create form. This allows a template
// to seed defaults without silently discarding width, depth, height or power edits.
func applyRackSpecOverride(item *model.Rack, requested model.Rack) {
	item.Type = requested.Type
	item.Manufacturer = requested.Manufacturer
	item.ModelNumber = requested.ModelNumber
	item.UHeight = requested.UHeight
	item.WidthMm = requested.WidthMm
	item.DepthMm = requested.DepthMm
	item.HeightMm = requested.HeightMm
	item.LoadCapacityKg = requested.LoadCapacityKg
	item.DualPower = requested.DualPower
	item.InputCircuits = requested.InputCircuits
	item.RatedVoltage = requested.RatedVoltage
	item.RatedCurrent = requested.RatedCurrent
	item.RatedPowerKw = requested.RatedPowerKw
	item.PeakPowerKw = requested.PeakPowerKw
	item.PDUCount = requested.PDUCount
}

func normalizeDataCenter(item *model.DataCenter) {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = model.ResourceStatusOperating
	}
	item.Address = strings.TrimSpace(item.Address)
	item.Manager = strings.TrimSpace(item.Manager)
	item.Contact = strings.TrimSpace(item.Contact)
	item.ServiceProvider = strings.TrimSpace(item.ServiceProvider)
	item.Remarks = strings.TrimSpace(item.Remarks)
}

func normalizeRoom(item *model.Room) {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = model.ResourceStatusOperating
	}
	if item.FloorPlanFormat == "" {
		item.FloorPlanFormat = "SVG"
	}
	item.FloorPlanFormat = strings.ToUpper(strings.TrimSpace(item.FloorPlanFormat))
	item.Building = strings.TrimSpace(item.Building)
	item.Floor = strings.TrimSpace(item.Floor)
	item.RoomNumber = strings.TrimSpace(item.RoomNumber)
	item.Purpose = strings.TrimSpace(item.Purpose)
	item.Manager = strings.TrimSpace(item.Manager)
	item.Contact = strings.TrimSpace(item.Contact)
	item.Remarks = strings.TrimSpace(item.Remarks)
}

func normalizeRack(item *model.Rack) {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = model.DefaultRackStatus
	}
	item.Type = strings.ToUpper(strings.TrimSpace(item.Type))
	if item.Type == "" {
		item.Type = model.DefaultRackType
	}
	if item.UHeight == 0 {
		item.UHeight = model.DefaultRackUHeight
	}
	if item.WidthMm == 0 {
		item.WidthMm = model.DefaultRackWidthMm
	}
	if item.DepthMm == 0 {
		item.DepthMm = model.DefaultRackDepthMm
	}
	if item.HeightMm == 0 {
		item.HeightMm = model.DefaultRackHeightMm
	}
	item.Manufacturer = strings.TrimSpace(item.Manufacturer)
	item.ModelNumber = strings.TrimSpace(item.ModelNumber)
	item.SerialNumber = strings.TrimSpace(item.SerialNumber)
	item.AssetNumber = strings.TrimSpace(item.AssetNumber)
	item.Zone = strings.TrimSpace(item.Zone)
	item.RackRow = strings.TrimSpace(item.RackRow)
	item.RackColumn = strings.TrimSpace(item.RackColumn)
	item.Aisle = strings.TrimSpace(item.Aisle)
	item.Manager = strings.TrimSpace(item.Manager)
	item.Department = strings.TrimSpace(item.Department)
	item.Purpose = strings.TrimSpace(item.Purpose)
	item.Remarks = strings.TrimSpace(item.Remarks)
}

func validateDataCenter(item model.DataCenter) error {
	if item.Code == "" || len(item.Code) > 50 {
		return fmt.Errorf("%w: 数据中心编码不能为空且不能超过 50 个字符", ErrInvalidResource)
	}
	if item.Name == "" || len(item.Name) > 150 {
		return fmt.Errorf("%w: 数据中心名称不能为空且不能超过 150 个字符", ErrInvalidResource)
	}
	if !validResourceStatus(item.Status) {
		return fmt.Errorf("%w: 数据中心状态无效", ErrInvalidResource)
	}
	if item.Longitude != nil && (*item.Longitude < -180 || *item.Longitude > 180) {
		return fmt.Errorf("%w: 经度范围应为 -180 到 180", ErrInvalidResource)
	}
	if item.Latitude != nil && (*item.Latitude < -90 || *item.Latitude > 90) {
		return fmt.Errorf("%w: 纬度范围应为 -90 到 90", ErrInvalidResource)
	}
	return nil
}

func validateRoom(item model.Room) error {
	if item.Code == "" || len(item.Code) > 50 {
		return fmt.Errorf("%w: 机房编码不能为空且不能超过 50 个字符", ErrInvalidResource)
	}
	if item.Name == "" || len(item.Name) > 150 {
		return fmt.Errorf("%w: 机房名称不能为空且不能超过 150 个字符", ErrInvalidResource)
	}
	if !validResourceStatus(item.Status) {
		return fmt.Errorf("%w: 机房状态无效", ErrInvalidResource)
	}
	if item.FloorPlanFormat != "SVG" && item.FloorPlanFormat != "PNG" && item.FloorPlanFormat != "JPG" {
		return fmt.Errorf("%w: 平面图格式仅支持 SVG、PNG 或 JPG", ErrInvalidResource)
	}
	if item.RacksPerRow < 1 || item.RacksPerRow > 20 {
		return fmt.Errorf("%w: 大屏每排机柜数必须是 1 到 20 的整数", ErrInvalidResource)
	}
	if negative(item.AreaSquareMeters, item.ClearHeightMeters, item.MaxLoadKg, item.CoolingCapacityKw, item.DesignPowerKw, item.AvailablePowerKw, item.UsedPowerKw) {
		return fmt.Errorf("%w: 面积、高度、电力和承重不能为负数", ErrInvalidResource)
	}
	return nil
}

func validateRack(item model.Rack) error {
	if item.Code == "" || len(item.Code) > 50 {
		return fmt.Errorf("%w: 机柜编码不能为空且不能超过 50 个字符", ErrInvalidResource)
	}
	if item.Name == "" || len(item.Name) > 150 {
		return fmt.Errorf("%w: 机柜名称不能为空且不能超过 150 个字符", ErrInvalidResource)
	}
	if !validRackStatus(item.Status) {
		return fmt.Errorf("%w: 机柜状态无效", ErrInvalidResource)
	}
	if item.UHeight < 1 || item.UHeight > 100 {
		return fmt.Errorf("%w: U 位高度必须是 1 到 100 的整数", ErrInvalidResource)
	}
	if item.WidthMm <= 0 || item.DepthMm <= 0 || item.HeightMm <= 0 {
		return fmt.Errorf("%w: 机柜尺寸必须大于 0", ErrInvalidResource)
	}
	if item.Rotation != 0 && item.Rotation != 90 && item.Rotation != 180 && item.Rotation != 270 {
		return fmt.Errorf("%w: 旋转角度仅支持 0、90、180、270", ErrInvalidResource)
	}
	if item.InputCircuits < 0 || item.PDUCount < 0 || negative(item.LoadCapacityKg, item.RatedVoltage, item.RatedCurrent, item.RatedPowerKw, item.PeakPowerKw) {
		return fmt.Errorf("%w: 电源、PDU 和承重参数不能为负数", ErrInvalidResource)
	}
	return nil
}

func validResourceStatus(status string) bool {
	switch status {
	case model.ResourceStatusPlanning, model.ResourceStatusOperating, model.ResourceStatusMaintenance, model.ResourceStatusDisabled, model.ResourceStatusArchived:
		return true
	default:
		return false
	}
}

func validRackStatus(status string) bool {
	switch status {
	case model.RackStatusPlanning, model.RackStatusAvailable, model.RackStatusPartial, model.RackStatusFull, model.RackStatusMaintenance, model.RackStatusDisabled:
		return true
	default:
		return false
	}
}

func isDisabled(status string) bool {
	return status == model.ResourceStatusDisabled || status == model.ResourceStatusArchived
}

func negative(values ...*float64) bool {
	for _, value := range values {
		if value != nil && *value < 0 {
			return true
		}
	}
	return false
}

func mapStoreError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return ErrResourceNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return ErrDuplicateCode
	case errors.Is(err, repository.ErrVersionConflict):
		return ErrResourceVersion
	default:
		return err
	}
}

type auditWriter interface {
	CreateAuditLog(context.Context, *model.AuditLog) error
}

func createAudit(ctx context.Context, store auditWriter, actor Actor, action, resourceType string, resourceID uuid.UUID, before, after interface{}) error {
	beforeJSON := "{}"
	afterJSON := "{}"
	if before != nil {
		data, err := json.Marshal(before)
		if err != nil {
			return err
		}
		beforeJSON = string(data)
	}
	if after != nil {
		data, err := json.Marshal(after)
		if err != nil {
			return err
		}
		afterJSON = string(data)
	}
	source := actor.Source
	if source == "" {
		source = "web"
	}
	log := model.AuditLog{
		UserID: actor.UserID, Action: action, ResourceType: resourceType,
		ResourceID: &resourceID, RequestID: actor.RequestID,
		BeforeJSON: beforeJSON, AfterJSON: afterJSON, Result: "SUCCESS", Source: source,
	}
	return store.CreateAuditLog(ctx, &log)
}
