package service

import (
	"context"
	"errors"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type TerminalRegistration struct {
	Status     string     `json:"status"`
	Assignee   string     `json:"assignee"`
	Department string     `json:"department"`
	SiteID     *uuid.UUID `json:"siteId"`
	Position   string     `json:"position"`
}

// The custody profile is authoritative. Old rack flags are only a compatibility fallback.
func terminalUsage(d model.Device, p model.AssetProfile) string {
	if d.LifecycleStatus == model.DeviceStatusScrapped {
		return "RETIRED"
	}
	if d.LifecycleStatus == model.DeviceStatusMaintenance && p.UsageStatus != "IN_USE" {
		return "REPAIR"
	}
	if p.UsageStatus == "" {
		return "IDLE"
	}
	return p.UsageStatus
}

func validateTerminalRegistration(in TerminalRegistration) error {
	switch in.Status {
	case "IDLE", "IN_USE", "REPAIR", "RETIRED":
	default:
		return opsInvalid("请选择终端使用状态：在库、使用中、维修中、已报废")
	}
	if len(in.Assignee) > 120 || len(in.Department) > 120 || len(in.Position) > 160 {
		return opsInvalid("终端使用人、部门或工位字段过长")
	}
	if in.Status == "IN_USE" && (strings.TrimSpace(in.Assignee) == "" || strings.TrimSpace(in.Department) == "" || in.SiteID == nil || *in.SiteID == uuid.Nil) {
		return opsInvalid("登记使用中的终端必须填写使用人、部门和位置")
	}
	if in.Status != "IN_USE" && (strings.TrimSpace(in.Assignee) != "" || strings.TrimSpace(in.Department) != "") {
		return opsInvalid("非使用中终端不能登记当前使用人和使用部门")
	}
	return nil
}

func (s *AssetOperations) RegisterTerminal(ctx context.Context, actor Actor, input DeviceCreateInput, in TerminalRegistration) (model.Device, error) {
	var out model.Device
	if err := validateTerminalRegistration(in); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var typ model.DeviceType
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&typ, "id = ?", input.Device.TypeID).Error; err != nil {
			return mapStoreError(err)
		}
		if typ.Category != model.DeviceCategoryPC {
			return opsInvalid("此入口只登记终端，请在机房设备入口登记其他设备")
		}
		var site *model.AssetSite
		if in.SiteID != nil {
			var x model.AssetSite
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&x, "id = ?", *in.SiteID).Error; err != nil {
				return mapStoreError(err)
			}
			site = &x
		}
		input.Device.LifecycleStatus = model.DeviceStatusWaitingRack
		input.Device.HeightU = 1
		input.Device.DualPowerRequired = false
		if in.Status == "RETIRED" {
			input.Device.LifecycleStatus = model.DeviceStatusScrapped
		}
		var err error
		out, err = NewDeviceService(repository.NewDeviceStore(tx)).CreateDevice(ctx, actor, input)
		if err != nil {
			return err
		}
		p := model.AssetProfile{DeviceID: out.ID, UsageStatus: in.Status, Assignee: strings.TrimSpace(in.Assignee), Department: strings.TrimSpace(in.Department), SiteID: in.SiteID, Site: site, Position: strings.TrimSpace(in.Position), Environment: "OFFICE"}
		if err := tx.Omit("Device", "Site").Create(&p).Error; err != nil {
			return err
		}
		name := ""
		if actor.UserID != nil {
			var u model.User
			if err := tx.First(&u, "id = ?", *actor.UserID).Error; err != nil {
				return err
			}
			name = u.DisplayName
		}
		history := model.PCHandover{DeviceID: out.ID, DeviceCode: out.Code, DeviceName: out.Name, SerialNumber: out.SerialNumber, Operation: "REGISTER", After: pcCustody(p), Condition: "UNRECORDED", Note: "初始状态登记，不补造历史领用时间", ConfirmedBy: actor.UserID, ConfirmedByName: name, ConfirmedAt: time.Now().UTC()}
		if err := tx.Omit("Device").Create(&history).Error; err != nil {
			return err
		}
		return s.Audit(tx, actor, "REGISTER_TERMINAL", "device", out.ID, nil, history)
	})
	return out, err
}

func (s *AssetOperations) TerminalState(ctx context.Context, id uuid.UUID) (TerminalRegistration, error) {
	var out TerminalRegistration
	var d model.Device
	if err := s.DB.WithContext(ctx).Preload("Type").First(&d, "id = ?", id).Error; err != nil {
		return out, mapStoreError(err)
	}
	if d.Type.Category != model.DeviceCategoryPC {
		return out, opsInvalid("该设备不是终端")
	}
	var p model.AssetProfile
	err := s.DB.WithContext(ctx).Where("device_id = ?", id).First(&p).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, err
	}
	return TerminalRegistration{Status: terminalUsage(d, p), Assignee: p.Assignee, Department: p.Department, SiteID: p.SiteID, Position: p.Position}, nil
}
