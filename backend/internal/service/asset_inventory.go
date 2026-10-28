package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"omnirack-replica/backend/internal/model"
	"strings"
	"time"
)

type CheckCreateInput struct {
	Name       string    `json:"name"`
	SiteID     uuid.UUID `json:"siteId"`
	AssigneeID uuid.UUID `json:"assigneeId"`
}

func (s *AssetOperations) CreateCheck(ctx context.Context, actor Actor, in CheckCreateInput) (model.AssetCheckTask, error) {
	var task model.AssetCheckTask
	if actor.UserID == nil || strings.TrimSpace(in.Name) == "" || len(in.Name) > 160 {
		return task, opsInvalid("盘点名称和操作身份必填")
	}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var site model.AssetSite
		var user model.User
		if e := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&site, "id = ?", in.SiteID).Error; e != nil {
			return mapStoreError(e)
		}
		if e := tx.Preload("Roles").First(&user, "id = ? AND enabled = true", in.AssigneeID).Error; e != nil {
			return mapStoreError(e)
		}
		permitted := false
		for _, r := range user.Roles {
			if r.Code == "system_admin" || r.Code == "asset_manager" {
				permitted = true
			}
		}
		if !permitted {
			return opsInvalid("盘点人必须为启用的资产管理员或系统管理员")
		}
		task = model.AssetCheckTask{Name: strings.TrimSpace(in.Name), SiteID: site.ID, AssigneeID: user.ID, CreatedBy: *actor.UserID, State: "ACTIVE"}
		if e := tx.Create(&task).Error; e != nil {
			return e
		}
		ops := NewAssetOperations(tx)
		items, e := ops.Overview(ctx)
		if e != nil {
			return e
		}
		for _, a := range items {
			if a.Device.LifecycleStatus == model.DeviceStatusScrapped {
				continue
			}
			inScope := a.Device.CurrentPosition != nil && site.RoomID != nil && a.Device.CurrentPosition.RoomID == *site.RoomID
			if a.Device.CurrentPosition == nil && a.Profile.SiteID != nil && *a.Profile.SiteID == site.ID {
				inScope = true
			}
			if !inScope {
				continue
			}
			snapshot, _ := json.Marshal(a)
			line := model.AssetCheckLine{TaskID: task.ID, Kind: "DEVICE", TargetID: a.Device.ID, Code: a.Device.Code, Name: a.Device.Name, SerialNumber: a.Device.SerialNumber,
				TargetVersion: a.Device.Version, Expected: 1, Snapshot: string(snapshot), Result: "UNSCANNED"}
			if e := tx.Create(&line).Error; e != nil {
				return e
			}
		}
		stocks := []model.SpareStock{}
		if e := tx.Preload("Spec").Where("site_id = ? AND quantity > 0 AND state IN ?", site.ID, []string{"AVAILABLE", "RESERVED", "TESTING", "FAULTY"}).Find(&stocks).Error; e != nil {
			return e
		}
		for _, x := range stocks {
			snapshot, _ := json.Marshal(x)
			line := model.AssetCheckLine{TaskID: task.ID, Kind: "SPARE", TargetID: x.ID, Code: x.ID.String(), Name: x.Spec.Name, SerialNumber: x.SerialNumber,
				TargetVersion: x.Version, Expected: x.Quantity, Snapshot: string(snapshot), Result: "UNSCANNED"}
			if e := tx.Create(&line).Error; e != nil {
				return e
			}
		}
		return s.Audit(tx, actor, "CREATE", "asset_check", task.ID, nil, task)
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	return task, e
}

type CheckScanInput struct {
	LineID                uuid.UUID `json:"lineId"`
	Actual                int       `json:"actual"`
	Difference            bool      `json:"difference"`
	ConfigurationVerified bool      `json:"configurationVerified"`
	Note                  string    `json:"note"`
}

func CheckResult(line model.AssetCheckLine, in CheckScanInput, currentVersion uint) (string, error) {
	if in.Actual < 0 || in.Actual > 1000000 || ((line.Kind == "DEVICE" || line.SerialNumber != "") && in.Actual > 1) {
		return "", opsInvalid("实盘数量无效")
	}
	if currentVersion != line.TargetVersion {
		return "CONFLICT", nil
	}
	if in.Actual != line.Expected || in.Difference {
		if strings.TrimSpace(in.Note) == "" {
			return "", opsInvalid("差异必须填写现场说明")
		}
		return "DIFFERENCE", nil
	}
	return "NORMAL", nil
}

func targetVersion(tx *gorm.DB, line model.AssetCheckLine) (uint, error) {
	if line.Kind == "DEVICE" {
		var d model.Device
		e := tx.First(&d, "id = ?", line.TargetID).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return d.Version, e
	}
	var x model.SpareStock
	e := tx.First(&x, "id = ?", line.TargetID).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return x.Version, e
}

func (s *AssetOperations) ScanCheck(ctx context.Context, actor Actor, taskID uuid.UUID, in CheckScanInput) (model.AssetCheckLine, bool, error) {
	var line model.AssetCheckLine
	duplicate := false
	if actor.UserID == nil {
		return line, false, opsInvalid("缺少操作身份")
	}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.AssetCheckTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ?", taskID).Error; e != nil {
			return mapStoreError(e)
		}
		if task.State != "ACTIVE" || task.AssigneeID != *actor.UserID {
			return opsInvalid("仅指定盘点人可执行进行中的任务")
		}
		if e := tx.Where("task_id = ? AND id = ?", taskID, in.LineID).First(&line).Error; e != nil {
			return mapStoreError(e)
		}
		if line.Result != "UNSCANNED" {
			duplicate = true
			return nil
		}
		version, e := targetVersion(tx, line)
		if e != nil {
			return e
		}
		result, e := CheckResult(line, in, version)
		if e != nil {
			return e
		}
		now := time.Now().UTC()
		line.Actual = &in.Actual
		line.Note = strings.TrimSpace(in.Note)
		line.ConfigurationVerified = in.ConfigurationVerified
		line.Result = result
		line.ScannedBy = actor.UserID
		line.ScannedAt = &now
		line.Version++
		if e := tx.Save(&line).Error; e != nil {
			return e
		}
		return s.Audit(tx, actor, "SCAN", "asset_check_line", line.ID, nil, line)
	})
	return line, duplicate, e
}

type CheckResolveInput struct {
	Resolution     string `json:"resolution"`
	AdjustQuantity bool   `json:"adjustQuantity"`
}

func (s *AssetOperations) ResolveCheck(ctx context.Context, actor Actor, taskID, lineID uuid.UUID, in CheckResolveInput) (model.AssetCheckLine, error) {
	var line model.AssetCheckLine
	if actor.UserID == nil || strings.TrimSpace(in.Resolution) == "" {
		return line, opsInvalid("处理依据必填")
	}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.AssetCheckTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ?", taskID).Error; e != nil {
			return mapStoreError(e)
		}
		if task.State != "ACTIVE" {
			return opsInvalid("任务已结案")
		}
		if e := tx.Where("id = ? AND task_id = ?", lineID, taskID).First(&line).Error; e != nil {
			return mapStoreError(e)
		}
		if line.Result == "NORMAL" || line.ResolvedBy != nil {
			return opsInvalid("此项无需重复处理")
		}
		if task.AssigneeID == *actor.UserID || (line.ScannedBy != nil && *line.ScannedBy == *actor.UserID) {
			return opsInvalid("差异必须由另一名授权人员复核")
		}
		if in.AdjustQuantity {
			if line.Kind != "SPARE" || line.Actual == nil {
				return opsInvalid("仅已实盘的数量备件可调整")
			}
			var x model.SpareStock
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, "id = ?", line.TargetID).Error; e != nil {
				return e
			}
			if e := tx.First(&x.Spec, "id = ?", x.SpecID).Error; e != nil {
				return e
			}
			if x.Version != line.TargetVersion {
				return ErrResourceVersion
			}
			if x.Spec.Tracking != "QUANTITY" || x.State != "AVAILABLE" {
				return opsInvalid("仅可调整可用的数量管理备件")
			}
			before := x.Quantity
			x.Quantity = *line.Actual
			x.Version++
			if e := tx.Omit("Spec", "Site", "InstalledDevice").Save(&x).Error; e != nil {
				return e
			}
			delta := x.Quantity - before
			if delta < 0 {
				delta = -delta
			}
			movement := model.SpareMovement{SubmissionID: uuid.New(), StockID: x.ID, Operation: "ADJUST", Quantity: delta,
				BeforeQuantity: before, AfterQuantity: x.Quantity, BeforeState: x.State, AfterState: x.State, SourceSiteID: x.SiteID, ActorID: *actor.UserID, Reason: in.Resolution}
			if e := tx.Create(&movement).Error; e != nil {
				return e
			}
		}
		line.ResolvedBy = actor.UserID
		line.Resolution = strings.TrimSpace(in.Resolution)
		line.Version++
		if e := tx.Save(&line).Error; e != nil {
			return e
		}
		return s.Audit(tx, actor, "RESOLVE", "asset_check_line", line.ID, nil, line)
	})
	return line, e
}

func (s *AssetOperations) CloseCheck(ctx context.Context, actor Actor, id uuid.UUID) error {
	changed := false
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task model.AssetCheckTask
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ?", id).Error; e != nil {
			return mapStoreError(e)
		}
		if task.State != "ACTIVE" {
			return opsInvalid("任务已结案")
		}
		lines := []model.AssetCheckLine{}
		if e := tx.Where("task_id = ?", id).Find(&lines).Error; e != nil {
			return e
		}
		for _, l := range lines {
			if l.Result == "NORMAL" {
				v, e := targetVersion(tx, l)
				if e != nil {
					return e
				}
				if v != l.TargetVersion {
					l.Result = "CONFLICT"
					l.Note += "\n结案复查：台账在扫码后发生变化"
					l.Version++
					if e := tx.Save(&l).Error; e != nil {
						return e
					}
					changed = true
				}
			}
		}
		if changed {
			return nil
		}
		for _, l := range lines {
			if l.Result != "NORMAL" && l.ResolvedBy == nil {
				return opsInvalid("仍有未盘或未复核差异")
			}
		}
		var discoveries int64
		if e := tx.Model(&model.AssetCheckDiscovery{}).Where("task_id = ? AND resolved_by IS NULL", id).Count(&discoveries).Error; e != nil {
			return e
		}
		if discoveries > 0 {
			return opsInvalid("仍有未处理的发现记录")
		}
		now := time.Now().UTC()
		task.State = "CLOSED"
		task.ClosedAt = &now
		task.Version++
		if e := tx.Save(&task).Error; e != nil {
			return e
		}
		return s.Audit(tx, actor, "CLOSE", "asset_check", id, nil, task)
	})
	if e == nil && changed {
		return opsInvalid("扫码后台账发生变化，已标记冲突，请重新核查后结案")
	}
	return e
}

func (s *AssetOperations) RefreshReminders(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 20261019, 1).Error; e != nil {
			return e
		}
		ops := NewAssetOperations(tx)
		items, e := ops.Overview(ctx)
		if e != nil {
			return e
		}
		seen := []string{}
		put := func(x model.AssetReminder) error {
			x.OccurrenceAt = time.Now().UTC()
			seen = append(seen, x.Key)
			var old model.AssetReminder
			e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("key = ?", x.Key).First(&old).Error
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
			if old.ID != uuid.Nil {
				if old.Active && !old.OccurrenceAt.IsZero() {
					x.OccurrenceAt = old.OccurrenceAt
				}
				x.BaseModel = old.BaseModel
				x.Version++
				if old.Title == x.Title && old.Detail == x.Detail && old.Severity == x.Severity && old.Active {
					x.AcknowledgedBy = old.AcknowledgedBy
					x.Acknowledgement = old.Acknowledgement
				}
			}
			x.Active = true
			return tx.Save(&x).Error
		}
		for _, x := range items {
			if x.Device.LifecycleStatus == model.DeviceStatusScrapped {
				continue
			}
			w := x.Warranty
			if w.Status == "SOON" || w.Status == "EXPIRED" || w.Status == "GAP" {
				severity := "WARNING"
				if w.Status != "SOON" {
					severity = "DANGER"
				}
				detail := x.Device.Code + " / SN " + x.Device.SerialNumber
				if w.EndsOn != nil {
					detail += " / " + w.EndsOn.Format("2006-01-02")
				}
				if e := put(model.AssetReminder{Key: "warranty:" + x.Device.ID.String(), Kind: "WARRANTY", DeviceID: &x.Device.ID, Title: x.Device.Name + " · " + w.Label, Detail: detail, Severity: severity}); e != nil {
					return e
				}
			}
			if strings.TrimSpace(x.Device.SerialNumber) == "" {
				if e := put(model.AssetReminder{Key: "sn:" + x.Device.ID.String(), Kind: "DATA", DeviceID: &x.Device.ID, Title: x.Device.Name + " · SN 待补全", Detail: x.Device.Code, Severity: "WARNING"}); e != nil {
					return e
				}
			}
		}
		rules := []model.SpareMinimum{}
		if e := tx.Preload("Spec").Preload("Site").Find(&rules).Error; e != nil {
			return e
		}
		for _, r := range rules {
			var available int64
			if e := tx.Model(&model.SpareStock{}).Where("spec_id = ? AND site_id = ? AND state = 'AVAILABLE'", r.SpecID, r.SiteID).Select("COALESCE(SUM(quantity),0)").Scan(&available).Error; e != nil {
				return e
			}
			if available < int64(r.Minimum) {
				if e := put(model.AssetReminder{Key: "minimum:" + r.ID.String(), Kind: "STOCK", Title: r.Spec.Name + " · 库存不足", Detail: fmt.Sprintf("%s：可用 %d / 最低 %d %s", r.Site.Name, available, r.Minimum, r.Spec.Unit), Severity: "WARNING"}); e != nil {
					return e
				}
			}
		}
		stocks := []model.SpareStock{}
		if e := tx.Preload("Spec").Preload("Site").Where("warranty_expires_at IS NOT NULL AND state NOT IN ?", []string{"INSTALLED", "RETIRED"}).Find(&stocks).Error; e != nil {
			return e
		}
		today := dateOnly(time.Now().In(chinaZone))
		for _, x := range stocks {
			end := dateOnly(*x.WarrantyExpiresAt)
			if !end.After(today.AddDate(0, 0, 30)) {
				label, severity := "即将过保", "WARNING"
				if end.Before(today) {
					label, severity = "已过保", "DANGER"
				}
				if e := put(model.AssetReminder{Key: "partwarranty:" + x.ID.String(), Kind: "PART_WARRANTY", StockID: &x.ID, Title: x.Spec.Name + " · " + label, Detail: x.Site.Name + " / " + x.SerialNumber + " / " + end.Format("2006-01-02"), Severity: severity}); e != nil {
					return e
				}
			}
		}
		if e := refreshRackUReminders(tx, put); e != nil {
			return e
		}
		query := tx.Model(&model.AssetReminder{}).Where("active = true")
		if len(seen) > 0 {
			query = query.Where("key NOT IN ?", seen)
		}
		return query.Update("active", false).Error
	})
}
