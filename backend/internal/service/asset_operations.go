package service

import (
	"context"
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

type AssetOperations struct{ DB *gorm.DB }

func NewAssetOperations(db *gorm.DB) *AssetOperations { return &AssetOperations{DB: db} }

func opsInvalid(message string) error { return fmt.Errorf("%w: %s", ErrInvalidResource, message) }
func (s *AssetOperations) Audit(tx *gorm.DB, actor Actor, action, kind string, id uuid.UUID, before, after interface{}) error {
	old, _ := json.Marshal(before)
	next, _ := json.Marshal(after)
	return tx.Create(&model.AuditLog{UserID: actor.UserID, Action: action, ResourceType: kind, ResourceID: &id,
		RequestID: actor.RequestID, Source: "asset_operations", Result: "SUCCESS", BeforeJSON: string(old), AfterJSON: string(next)}).Error
}

func (s *AssetOperations) CreateSite(ctx context.Context, actor Actor, x model.AssetSite) (model.AssetSite, error) {
	if e := normalizeSite(&x); e != nil {
		return x, e
	}
	if x.RoomID != nil {
		var room model.Room
		if e := s.DB.WithContext(ctx).First(&room, "id = ?", *x.RoomID).Error; e != nil {
			return x, mapStoreError(e)
		}
	}
	x.BaseModel = model.BaseModel{}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Omit("Room").Create(&x).Error; e != nil {
			return mapStoreError(e)
		}
		return s.Audit(tx, actor, "CREATE", "asset_site", x.ID, nil, x)
	})
	return x, e
}

func (s *AssetOperations) SaveProfile(ctx context.Context, actor Actor, deviceID uuid.UUID, x model.AssetProfile) (model.AssetProfile, error) {
	if x.MemoryGB < 0 || x.MemoryGB > 1048576 {
		return x, opsInvalid("内存容量不合法")
	}
	switch x.UsageStatus {
	case "IDLE", "IN_USE", "REPAIR", "RETIRED":
	default:
		return x, opsInvalid("资产使用状态无效")
	}
	if x.UsageStatus == "IN_USE" && strings.TrimSpace(x.Assignee) == "" {
		return x, opsInvalid("在用资产必须填写使用人或运维责任人")
	}
	if len(x.CPU) > 200 || len(x.Disk) > 500 || len(x.Ports) > 200 || len(x.Position) > 160 {
		return x, opsInvalid("配置字段过长")
	}
	var output model.AssetProfile
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var device model.Device
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&device, "id = ?", deviceID).Error; e != nil {
			return mapStoreError(e)
		}
		if x.SiteID != nil {
			var site model.AssetSite
			if e := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&site, "id = ?", *x.SiteID).Error; e != nil {
				return mapStoreError(e)
			}
			var pos model.RackDevicePosition
			e := tx.Where("device_id = ?", deviceID).First(&pos).Error
			if e == nil && (site.RoomID == nil || *site.RoomID != pos.RoomID) {
				return opsInvalid("已上架设备的资产位置必须关联当前机房；迁移请使用原机柜流程")
			}
			if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
				return e
			}
		}
		if x.UsageStatus == "RETIRED" && device.LifecycleStatus != model.DeviceStatusScrapped {
			return opsInvalid("报废请先在原设备流程处理；本页仅登记在用、闲置及维修状态")
		}
		var old model.AssetProfile
		e := tx.Where("device_id = ?", deviceID).First(&old).Error
		if e != nil && !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		if e == nil && old.Version != x.Version {
			return ErrResourceVersion
		}
		if errors.Is(e, gorm.ErrRecordNotFound) && x.Version != 0 {
			return ErrResourceVersion
		}
		// Terminal state and custody changes must create a business history record.
		if terminalUsage(device, old) != x.UsageStatus || old.Assignee != x.Assignee || old.Department != x.Department {
			var typ model.DeviceType
			if err := tx.First(&typ, "id = ?", device.TypeID).Error; err != nil {
				return mapStoreError(err)
			}
			if typ.Category == model.DeviceCategoryPC {
				return opsInvalid("终端状态、使用人和部门请通过终端管理的领用、归还、维修或报废操作变更")
			}
		}
		x.DeviceID = deviceID
		x.Device = model.Device{}
		x.Site = nil
		if old.ID == uuid.Nil {
			x.BaseModel = model.BaseModel{}
			if e := tx.Omit("Device", "Site").Create(&x).Error; e != nil {
				return e
			}
		} else {
			x.BaseModel = old.BaseModel
			x.Version++
			if e := tx.Omit("Device", "Site").Save(&x).Error; e != nil {
				return e
			}
		}
		if e := tx.Model(&model.Device{}).Where("id = ?", deviceID).Updates(map[string]interface{}{"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}).Error; e != nil {
			return e
		}
		output = x
		return s.Audit(tx, actor, "UPDATE_PROFILE", "device", deviceID, old, x)
	})
	return output, e
}

func (s *AssetOperations) AddCoverage(ctx context.Context, actor Actor, x model.AssetCoverage) (model.AssetCoverage, error) {
	if x.DeviceID == uuid.Nil || strings.TrimSpace(x.Provider) == "" || (x.Kind != "FACTORY" && x.Kind != "THIRD_PARTY") || x.StartsOn.IsZero() || x.EndsOn.IsZero() || x.EndsOn.Before(x.StartsOn) {
		return x, opsInvalid("服务类型、服务商和起止日期不合法")
	}
	x.StartsOn = dateOnly(x.StartsOn)
	x.EndsOn = dateOnly(x.EndsOn)
	x.BaseModel = model.BaseModel{}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var device model.Device
		if e := tx.First(&device, "id = ?", x.DeviceID).Error; e != nil {
			return mapStoreError(e)
		}
		if e := tx.Omit("Device").Create(&x).Error; e != nil {
			return e
		}
		if e := tx.Model(&model.AssetProfile{}).Where("device_id = ?", x.DeviceID).Updates(map[string]interface{}{"no_warranty": false, "version": gorm.Expr("version + 1")}).Error; e != nil {
			return e
		}
		return s.Audit(tx, actor, "ADD_COVERAGE", "device", x.DeviceID, nil, x)
	})
	return x, e
}

var chinaZone = time.FixedZone("Asia/Shanghai", 8*60*60)

func dateOnly(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, chinaZone)
}

type WarrantyView struct {
	Status string     `json:"status"`
	Label  string     `json:"label"`
	EndsOn *time.Time `json:"endsOn,omitempty"`
}

func EvaluateWarranty(device model.Device, profile model.AssetProfile, coverages []model.AssetCoverage, now time.Time) WarrantyView {
	if profile.NoWarranty {
		return WarrantyView{Status: "NONE", Label: "无保修"}
	}
	today := dateOnly(now.In(chinaZone))
	var end *time.Time
	var latest *time.Time
	future := false
	if device.WarrantyExpiresAt != nil {
		d := dateOnly(*device.WarrantyExpiresAt)
		latest = &d
		if !d.Before(today) {
			end = &d
		}
	}
	for _, c := range coverages {
		start, last := dateOnly(c.StartsOn), dateOnly(c.EndsOn)
		if latest == nil || last.After(*latest) {
			v := last
			latest = &v
		}
		if start.After(today) {
			future = true
		}
		if !start.After(today) && !last.Before(today) && (end == nil || last.After(*end)) {
			v := last
			end = &v
		}
	}
	if end != nil {
		if end.Sub(today) <= 30*24*time.Hour {
			return WarrantyView{Status: "SOON", Label: "即将过保", EndsOn: end}
		}
		return WarrantyView{Status: "VALID", Label: "在保", EndsOn: end}
	}
	if future {
		return WarrantyView{Status: "GAP", Label: "当前保障空档"}
	}
	if latest != nil {
		return WarrantyView{Status: "EXPIRED", Label: "已过保", EndsOn: latest}
	}
	return WarrantyView{Status: "UNKNOWN", Label: "信息待补全"}
}

type AssetOverview struct {
	Device     model.Device          `json:"device"`
	Profile    model.AssetProfile    `json:"profile"`
	Warranty   WarrantyView          `json:"warranty"`
	Coverages  []model.AssetCoverage `json:"coverages"`
	Components []model.SpareStock    `json:"components"`
}

func (s *AssetOperations) Overview(ctx context.Context) ([]AssetOverview, error) {
	db := s.DB.WithContext(ctx)
	devices := []model.Device{}
	profiles := []model.AssetProfile{}
	coverages := []model.AssetCoverage{}
	components := []model.SpareStock{}
	if e := db.Preload("Type").Preload("CurrentPosition").Preload("CurrentPosition.Rack").Order("code").Find(&devices).Error; e != nil {
		return nil, e
	}
	if e := db.Preload("Site").Find(&profiles).Error; e != nil {
		return nil, e
	}
	if e := db.Find(&coverages).Error; e != nil {
		return nil, e
	}
	if e := db.Preload("Spec").Preload("Site").Where("state = ?", "INSTALLED").Find(&components).Error; e != nil {
		return nil, e
	}
	pmap := map[uuid.UUID]model.AssetProfile{}
	cmap := map[uuid.UUID][]model.AssetCoverage{}
	partmap := map[uuid.UUID][]model.SpareStock{}
	for _, p := range profiles {
		pmap[p.DeviceID] = p
	}
	for _, c := range coverages {
		cmap[c.DeviceID] = append(cmap[c.DeviceID], c)
	}
	for _, c := range components {
		if c.InstalledDeviceID != nil {
			partmap[*c.InstalledDeviceID] = append(partmap[*c.InstalledDeviceID], c)
		}
	}
	out := make([]AssetOverview, 0, len(devices))
	for _, d := range devices {
		p := pmap[d.ID]
		if _, exists := pmap[d.ID]; !exists {
			p = model.AssetProfile{DeviceID: d.ID, UsageStatus: "IDLE"}
		}
		if d.Type.Category == model.DeviceCategoryPC {
			p.UsageStatus = terminalUsage(d, p)
		}
		cs := cmap[d.ID]
		if cs == nil {
			cs = []model.AssetCoverage{}
		}
		parts := partmap[d.ID]
		if parts == nil {
			parts = []model.SpareStock{}
		}
		out = append(out, AssetOverview{d, p, EvaluateWarranty(d, p, cs, time.Now()), cs, parts})
	}
	return out, nil
}

func (s *AssetOperations) CreateSpec(ctx context.Context, actor Actor, x model.SpareSpec) (model.SpareSpec, error) {
	if e := normalizeSpec(&x); e != nil {
		return x, e
	}
	x.BaseModel = model.BaseModel{}
	e := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&x).Error; e != nil {
			return mapStoreError(e)
		}
		return s.Audit(tx, actor, "CREATE", "spare_spec", x.ID, nil, x)
	})
	return x, e
}
func (s *AssetOperations) CreateStock(ctx context.Context, actor Actor, x model.SpareStock) (model.SpareStock, error) {
	x.SerialNumber = strings.ToUpper(strings.TrimSpace(x.SerialNumber))
	x.Notes = strings.TrimSpace(x.Notes)
	if x.Quantity != 0 || x.InstalledDeviceID != nil || (x.State != "" && x.State != "AVAILABLE") {
		return x, opsInvalid("先建立零库存档案，再通过入库单增加数量")
	}
	db := s.DB.WithContext(ctx)
	if len(x.Notes) > 1000 || len(x.SerialNumber) > 120 || len(x.Bin) > 160 || len(x.Batch) > 120 {
		return x, opsInvalid("SN、库位、批次或备注超过允许长度")
	}
	x.BaseModel = model.BaseModel{}
	x.State = "AVAILABLE"
	x.Spec = model.SpareSpec{}
	x.Site = model.AssetSite{}
	x.DestinationSiteID = nil
	e := db.Transaction(func(tx *gorm.DB) error {
		var spec model.SpareSpec
		var site model.AssetSite
		if e := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&spec, "id = ?", x.SpecID).Error; e != nil {
			return mapStoreError(e)
		}
		if e := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&site, "id = ?", x.SiteID).Error; e != nil {
			return mapStoreError(e)
		}
		if (spec.Tracking == "SERIAL" && x.SerialNumber == "") || (spec.Tracking == "QUANTITY" && x.SerialNumber != "") {
			return opsInvalid("按件备件必须填写 SN，按数量备件不能填写单件 SN")
		}
		if e := tx.Omit("Spec", "Site", "InstalledDevice").Create(&x).Error; e != nil {
			return mapStoreError(e)
		}
		return s.Audit(tx, actor, "CREATE", "spare_stock", x.ID, nil, x)
	})
	return x, e
}

type SpareMoveInput struct {
	receiptState      string
	Version           uint       `json:"version"`
	SubmissionID      uuid.UUID  `json:"submissionId"`
	Operation         string     `json:"operation"`
	Quantity          int        `json:"quantity"`
	DeviceID          *uuid.UUID `json:"deviceId"`
	DestinationSiteID *uuid.UUID `json:"destinationSiteId"`
	Reason            string     `json:"reason"`
	DisposalEvidence  string     `json:"disposalEvidence"`
}

// ValidateSpareMove is deliberately pure so transition/quantity boundaries can be tested independently.
func ValidateSpareMove(x model.SpareStock, in SpareMoveInput) error {
	if in.receiptState != "" && (in.Operation != "IN" || x.Quantity != 0 || !validSpareReceiptState(in.receiptState)) {
		return opsInvalid("仅首次入库可登记初始库存状态")
	}
	if in.SubmissionID == uuid.Nil || in.Quantity < 1 || in.Quantity > 1000000 || strings.TrimSpace(in.Reason) == "" {
		return opsInvalid("提交标识、正整数数量及操作依据必填")
	}
	if len(in.Reason) > 2000 || len(in.DisposalEvidence) > 2000 {
		return opsInvalid("操作依据或数据处置依据过长")
	}
	if in.Version != x.Version {
		return ErrResourceVersion
	}
	if x.Spec.Tracking == "SERIAL" && in.Quantity != 1 {
		return opsInvalid("按件备件每次只能操作一件")
	}
	switch in.Operation {
	case "IN":
		if x.State != "AVAILABLE" || x.InstalledDeviceID != nil || (x.Spec.Tracking == "SERIAL" && x.Quantity != 0) {
			return opsInvalid("当前记录不可入库或 SN 已在库")
		}
	case "OUT":
		if x.Spec.Tracking != "QUANTITY" || x.State != "AVAILABLE" || in.Quantity > x.Quantity {
			return opsInvalid("数量库存不足；按件备件需记录安装或调拨去向")
		}
	case "INSTALL":
		if x.Spec.Tracking != "SERIAL" || x.Quantity != 1 || (x.State != "AVAILABLE" && x.State != "RESERVED") || in.DeviceID == nil {
			return opsInvalid("仅在库按件备件可以安装，且必须指定设备")
		}
	case "REMOVE":
		if x.State != "INSTALLED" || x.InstalledDeviceID == nil {
			return opsInvalid("该备件尚未安装")
		}
	case "TRANSFER":
		if x.State != "AVAILABLE" || x.Quantity < in.Quantity || in.DestinationSiteID == nil || *in.DestinationSiteID == x.SiteID {
			return opsInvalid("调拨数量、状态或目标位置无效")
		}
	case "RECEIVE":
		if x.State != "TRANSIT" || x.DestinationSiteID == nil || in.Quantity != x.Quantity {
			return opsInvalid("仅可整批确认在途记录收货")
		}
	case "RESERVE":
		if x.State != "AVAILABLE" || x.Quantity != in.Quantity {
			return opsInvalid("预留须覆盖当前库存行全部数量，请按批次建档")
		}
	case "RELEASE":
		if x.State != "RESERVED" || x.Quantity != in.Quantity {
			return opsInvalid("释放数量与预留记录不符")
		}
	case "PASS", "FAULT":
		if (x.State != "TESTING" && x.State != "REPAIR" && !(in.Operation == "FAULT" && x.State == "AVAILABLE")) || x.Quantity != in.Quantity {
			return opsInvalid("检测或故障登记须覆盖本条记录全部数量，且状态须允许此操作")
		}
	case "REPAIR":
		if x.State != "FAULTY" || x.Quantity != in.Quantity {
			return opsInvalid("仅故障库存可送修，数量需一致")
		}
	case "RETIRE":
		if (x.State != "TESTING" && x.State != "FAULTY") || x.Quantity != in.Quantity || strings.TrimSpace(in.DisposalEvidence) == "" {
			return opsInvalid("报废需故障或待检测状态、全批数量及数据处置依据")
		}
	default:
		return opsInvalid("不支持的库存操作")
	}
	return nil
}

func spareMoveReason(in SpareMoveInput) string {
	if in.Operation == "RETIRE" {
		return strings.TrimSpace(in.Reason + "\n处置依据：" + in.DisposalEvidence)
	}
	return strings.TrimSpace(in.Reason)
}

// A retry must identify the same business command, not merely the same actor and
// stock row. Version is deliberately excluded: the first execution increments it.
// REMOVE and RECEIVE derive their device/destination from locked stock, so their
// callers may omit those fields when replaying the original request.
func sameSpareMove(prior model.SpareMovement, in SpareMoveInput) bool {
	if prior.Operation != in.Operation || prior.Quantity != in.Quantity || prior.Reason != spareMoveReason(in) {
		return false
	}
	if !(in.Operation == "REMOVE" && in.DeviceID == nil) && !sameOptionalID(prior.DeviceID, in.DeviceID) {
		return false
	}
	if !(in.Operation == "RECEIVE" && in.DestinationSiteID == nil) && !sameOptionalID(prior.DestinationSiteID, in.DestinationSiteID) {
		return false
	}
	return in.receiptState == "" || prior.AfterState == in.receiptState
}

func (s *AssetOperations) MoveStock(ctx context.Context, actor Actor, id uuid.UUID, in SpareMoveInput) (model.SpareMovement, error) {
	var output model.SpareMovement
	if actor.UserID == nil || in.SubmissionID == uuid.Nil {
		return output, opsInvalid("缺少操作身份或提交标识")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", in.SubmissionID.String()).Error; e != nil {
			return e
		}
		var prior model.SpareMovement
		e := tx.Where("submission_id = ?", in.SubmissionID).First(&prior).Error
		if e == nil {
			if prior.StockID != id || prior.ActorID != *actor.UserID || !sameSpareMove(prior, in) {
				return opsInvalid("提交标识冲突")
			}
			output = prior
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return e
		}
		var x model.SpareStock
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, "id = ?", id).Error; e != nil {
			return mapStoreError(e)
		}
		if e := tx.First(&x.Spec, "id = ?", x.SpecID).Error; e != nil {
			return e
		}
		if e := ValidateSpareMove(x, in); e != nil {
			return e
		}
		before := x
		var destinationStock *uuid.UUID
		sourceSite := x.SiteID
		switch in.Operation {
		case "IN":
			x.Quantity += in.Quantity
			if in.receiptState != "" {
				x.State = in.receiptState
			}
		case "OUT":
			x.Quantity -= in.Quantity
		case "INSTALL":
			var d model.Device
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&d, "id = ?", *in.DeviceID).Error; e != nil {
				return mapStoreError(e)
			}
			if d.LifecycleStatus == model.DeviceStatusScrapped {
				return opsInvalid("不能安装到已报废设备")
			}
			var site model.AssetSite
			if e := tx.First(&site, "id = ?", x.SiteID).Error; e != nil {
				return e
			}
			var pos model.RackDevicePosition
			e := tx.Where("device_id = ?", d.ID).First(&pos).Error
			if e == nil {
				if site.RoomID == nil || *site.RoomID != pos.RoomID {
					return opsInvalid("请先将备件调拨至设备所在机房")
				}
			} else if errors.Is(e, gorm.ErrRecordNotFound) {
				var p model.AssetProfile
				if e := tx.Where("device_id = ?", d.ID).First(&p).Error; e != nil || p.SiteID == nil || *p.SiteID != x.SiteID {
					return opsInvalid("未上架设备需要先登记同一资产位置")
				}
			} else {
				return e
			}
			x.State = "INSTALLED"
			x.Quantity = 0
			x.InstalledDeviceID = in.DeviceID
		case "REMOVE":
			in.DeviceID = x.InstalledDeviceID
			var installedDevice model.Device
			if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&installedDevice, "id = ?", *in.DeviceID).Error; e != nil {
				return mapStoreError(e)
			}
			// A rack move may have happened since installation. Resolve the current room,
			// rather than silently returning the part to its historical source store.
			var position model.RackDevicePosition
			positionErr := tx.Where("device_id = ?", *in.DeviceID).First(&position).Error
			if positionErr == nil {
				var sites []model.AssetSite
				if e := tx.Where("room_id = ?", position.RoomID).Find(&sites).Error; e != nil {
					return e
				}
				if len(sites) != 1 {
					return opsInvalid("请为当前机房保留一个明确的资产位置，再拆回备件")
				}
				x.SiteID = sites[0].ID
			} else if errors.Is(positionErr, gorm.ErrRecordNotFound) {
				var profile model.AssetProfile
				if e := tx.Where("device_id = ?", *in.DeviceID).First(&profile).Error; e != nil || profile.SiteID == nil {
					return opsInvalid("请先确认未上架设备的实际资产位置，再拆回备件")
				}
				x.SiteID = *profile.SiteID
			} else {
				return positionErr
			}
			x.InstalledDeviceID = nil
			x.Quantity = 1
			x.State = "TESTING"
		case "TRANSFER":
			var site model.AssetSite
			if e := tx.First(&site, "id = ?", *in.DestinationSiteID).Error; e != nil {
				return mapStoreError(e)
			}
			if x.Spec.Tracking == "SERIAL" {
				x.State = "TRANSIT"
				x.DestinationSiteID = in.DestinationSiteID
			} else {
				target := model.SpareStock{SpecID: x.SpecID, SiteID: *in.DestinationSiteID, Quantity: in.Quantity, State: "TRANSIT", DestinationSiteID: in.DestinationSiteID, Notes: x.Notes, Batch: x.Batch, WarrantyExpiresAt: x.WarrantyExpiresAt}
				if e := tx.Create(&target).Error; e != nil {
					return e
				}
				destinationStock = &target.ID
				x.Quantity -= in.Quantity
			}
		case "RECEIVE":
			in.DestinationSiteID = x.DestinationSiteID
			x.SiteID = *x.DestinationSiteID
			x.DestinationSiteID = nil
			x.State = "AVAILABLE"
		case "RESERVE":
			x.State = "RESERVED"
		case "RELEASE", "PASS":
			x.State = "AVAILABLE"
		case "FAULT":
			x.State = "FAULTY"
		case "REPAIR":
			x.State = "REPAIR"
		case "RETIRE":
			x.State = "RETIRED"
			x.Quantity = 0
		}
		if x.Quantity < 0 {
			return opsInvalid("库存不能为负数")
		}
		if x.Spec.Tracking == "SERIAL" && x.Quantity > 1 {
			return opsInvalid("单 SN 库存不能大于 1")
		}
		x.Version++
		if e := tx.Omit("Spec", "Site", "InstalledDevice").Save(&x).Error; e != nil {
			return e
		}
		if in.Operation == "INSTALL" || in.Operation == "REMOVE" {
			if e := tx.Model(&model.Device{}).Where("id = ?", *in.DeviceID).Updates(map[string]interface{}{"version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
		}
		output = model.SpareMovement{SubmissionID: in.SubmissionID, StockID: x.ID, DestinationStockID: destinationStock, Operation: in.Operation, Quantity: in.Quantity,
			BeforeQuantity: before.Quantity, AfterQuantity: x.Quantity, BeforeState: before.State, AfterState: x.State, DeviceID: in.DeviceID, SourceSiteID: sourceSite, DestinationSiteID: in.DestinationSiteID, ActorID: *actor.UserID, Reason: spareMoveReason(in)}
		if e := tx.Create(&output).Error; e != nil {
			return e
		}
		return s.Audit(tx, actor, in.Operation, "spare_stock", x.ID, before, x)
	})
	return output, err
}
