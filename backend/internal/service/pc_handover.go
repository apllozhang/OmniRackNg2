package service

import (
	"context"
	"errors"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type PCHandoverInput struct {
	Version    uint      `json:"version"` // Device version protects both profile and core changes.
	Operation  string    `json:"operation"`
	Assignee   string    `json:"assignee"`
	Department string    `json:"department"`
	SiteID     uuid.UUID `json:"siteId"`
	Position   string    `json:"position"`
	Condition  string    `json:"condition"`
	Note       string    `json:"note"`
}

func pcCustody(p model.AssetProfile) model.PCCustody {
	x := model.PCCustody{UsageStatus: p.UsageStatus, Assignee: p.Assignee, Department: p.Department, SiteID: p.SiteID, Position: p.Position}
	if p.Site != nil {
		x.SiteName = p.Site.Region + " / " + p.Site.Name
	}
	return x
}

func nextPCCustody(p model.AssetProfile, in PCHandoverInput) (model.AssetProfile, error) {
	if len(in.Assignee) > 120 || len(in.Department) > 120 || len(in.Position) > 160 || len(in.Note) > 500 || strings.TrimSpace(in.Note) == "" || in.SiteID == uuid.Nil {
		return p, opsInvalid("请选择位置并填写交接说明；使用人/部门最多 120 字节，工位 160 字节，说明 500 字节")
	}
	switch in.Operation {
	case "ISSUE":
		if p.UsageStatus != "IDLE" {
			return p, opsInvalid("仅闲置终端可以领用；在用设备请先归还或交接")
		}
	case "RETURN", "TRANSFER":
		if p.UsageStatus != "IN_USE" || strings.TrimSpace(p.Assignee) == "" {
			return p, opsInvalid("仅有使用人的在用终端可以归还或交接")
		}
	case "REPAIR", "RESTORE", "RETIRE":
		if in.Operation == "REPAIR" && (p.UsageStatus != "IDLE" || in.Condition != "FAULTY") {
			return p, opsInvalid("在库终端可以登记维修；使用中设备请先归还")
		}
		if in.Operation == "RESTORE" && (p.UsageStatus != "REPAIR" || in.Condition != "NORMAL") {
			return p, opsInvalid("仅维修中终端可确认维修完成并恢复在库")
		}
		if in.Operation == "RETIRE" && (p.UsageStatus != "IDLE" && p.UsageStatus != "REPAIR") {
			return p, opsInvalid("仅在库或维修中终端可报废；使用中设备请先归还")
		}
		if in.Condition != "NORMAL" && in.Condition != "FAULTY" {
			return p, opsInvalid("请选择设备状况")
		}
		p.Assignee, p.Department = "", ""
		p.UsageStatus = map[string]string{"REPAIR": "REPAIR", "RESTORE": "IDLE", "RETIRE": "RETIRED"}[in.Operation]
		p.SiteID, p.Position = &in.SiteID, strings.TrimSpace(in.Position)
		return p, nil
	default:
		return p, opsInvalid("终端交接操作无效")
	}
	if in.Operation == "RETURN" {
		if in.Condition != "NORMAL" && in.Condition != "FAULTY" {
			return p, opsInvalid("请选择归还设备状况")
		}
		p.UsageStatus, p.Assignee, p.Department = "IDLE", "", ""
		if in.Condition == "FAULTY" {
			p.UsageStatus = "REPAIR"
		}
	} else {
		if strings.TrimSpace(in.Assignee) == "" || strings.TrimSpace(in.Department) == "" {
			return p, opsInvalid("请填写接收人和部门")
		}
		if in.Condition != "NORMAL" {
			return p, opsInvalid("领用及交接须确认设备正常")
		}
		if in.Operation == "TRANSFER" && strings.EqualFold(strings.TrimSpace(in.Assignee), strings.TrimSpace(p.Assignee)) {
			return p, opsInvalid("交接接收人须与当前使用人不同")
		}
		p.UsageStatus, p.Assignee, p.Department = "IN_USE", strings.TrimSpace(in.Assignee), strings.TrimSpace(in.Department)
	}
	p.SiteID, p.Position = &in.SiteID, strings.TrimSpace(in.Position)
	return p, nil
}

func (s *AssetOperations) ConfirmPCHandover(ctx context.Context, actor Actor, id uuid.UUID, in PCHandoverInput) (model.PCHandover, error) {
	var out model.PCHandover
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var d model.Device
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&d, "id = ?", id).Error; err != nil {
			return mapStoreError(err)
		}
		if d.Version != in.Version {
			return ErrResourceVersion
		}
		var typ model.DeviceType
		if err := tx.First(&typ, "id = ?", d.TypeID).Error; err != nil {
			return mapStoreError(err)
		}
		if typ.Category != model.DeviceCategoryPC || d.LifecycleStatus == model.DeviceStatusScrapped {
			return opsInvalid("仅未报废的 终端可以办理领用归还")
		}
		if d.LifecycleStatus == model.DeviceStatusMaintenance && (in.Operation == "ISSUE" || in.Operation == "TRANSFER") {
			return opsInvalid("维修中的设备不能领用或交接，请先确认维修完成")
		}
		var positioned int64
		if err := tx.Model(&model.RackDevicePosition{}).Where("device_id = ?", id).Count(&positioned).Error; err != nil {
			return err
		}
		if positioned > 0 {
			return opsInvalid("该终端已上架，请先核实并通过机柜流程下架")
		}
		p := model.AssetProfile{DeviceID: id, UsageStatus: "IDLE"}
		err := tx.Where("device_id = ?", id).Preload("Site").First(&p).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		p.UsageStatus = terminalUsage(d, p)
		before := pcCustody(p)
		next, err := nextPCCustody(p, in)
		if err != nil {
			return err
		}
		var site model.AssetSite
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&site, "id = ?", in.SiteID).Error; err != nil {
			return mapStoreError(err)
		}
		next.Site = &site
		if next.ID == uuid.Nil {
			if err := tx.Omit("Device", "Site").Create(&next).Error; err != nil {
				return err
			}
		} else {
			next.Version++
			if err := tx.Omit("Device", "Site").Save(&next).Error; err != nil {
				return err
			}
		}
		updates := map[string]interface{}{"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}
		if in.Operation == "RETIRE" {
			updates["lifecycle_status"] = model.DeviceStatusScrapped
		}
		if in.Operation == "RESTORE" || in.Operation == "RETURN" {
			updates["lifecycle_status"] = model.DeviceStatusWaitingRack
		}
		if err := tx.Model(&model.Device{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}
		name := ""
		if actor.UserID != nil {
			var user model.User
			if err := tx.First(&user, "id = ?", *actor.UserID).Error; err != nil {
				return mapStoreError(err)
			}
			name = user.DisplayName
		}
		out = model.PCHandover{DeviceID: id, DeviceCode: d.Code, DeviceName: d.Name, SerialNumber: d.SerialNumber, Operation: in.Operation, Before: before, After: pcCustody(next), Condition: in.Condition, Note: strings.TrimSpace(in.Note), ConfirmedBy: actor.UserID, ConfirmedByName: name, ConfirmedAt: time.Now().UTC()}
		if err := tx.Omit("Device").Create(&out).Error; err != nil {
			return err
		}
		return s.Audit(tx, actor, "PC_"+in.Operation, "device", id, before, out)
	})
	return out, err
}
