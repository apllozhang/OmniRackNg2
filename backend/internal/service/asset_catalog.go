package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func normalizeSite(x *model.AssetSite) error {
	x.Name = strings.TrimSpace(x.Name)
	x.Region = strings.TrimSpace(x.Region)
	x.Address = strings.TrimSpace(x.Address)
	if x.Kind == "OFFICE" {
		x.RoomID = nil
	}
	if x.Name == "" || x.Region == "" || len(x.Name) > 160 || len(x.Region) > 120 || len(x.Address) > 300 || (x.Kind != "OFFICE" && x.Kind != "ROOM" && x.Kind != "STORE") {
		return opsInvalid("请填写有效的区域、位置名称、类型及地址")
	}
	return nil
}
func normalizeSpec(x *model.SpareSpec) error {
	x.Name = strings.TrimSpace(x.Name)
	x.PartNumber = strings.ToUpper(strings.TrimSpace(x.PartNumber))
	x.Specification = strings.TrimSpace(x.Specification)
	x.Category = strings.TrimSpace(x.Category)
	x.Unit = strings.TrimSpace(x.Unit)
	if x.Unit == "" {
		x.Unit = "件"
	}
	if x.Name == "" || x.PartNumber == "" || x.Specification == "" || x.Category == "" || (x.Tracking != "SERIAL" && x.Tracking != "QUANTITY") {
		return opsInvalid("请填写名称、类别、料号、规格及管理方式")
	}
	if len(x.Name) > 160 || len(x.Category) > 40 || len(x.PartNumber) > 120 || len(x.Specification) > 500 || len(x.Manufacturer) > 120 || len(x.ModelNumber) > 160 || len(x.Unit) > 20 || len(x.Compatibility) > 500 {
		return opsInvalid("备件规格字段超过允许长度")
	}
	return nil
}

type catalogReference struct {
	entity    interface{}
	condition string
}

func catalogReferenced(tx *gorm.DB, id uuid.UUID, refs ...catalogReference) (bool, error) {
	for _, ref := range refs {
		var count int64
		args := []interface{}{}
		for i := 0; i < strings.Count(ref.condition, "?"); i++ {
			args = append(args, id)
		}
		if err := tx.Unscoped().Model(ref.entity).Where(ref.condition, args...).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
}
func siteReferenced(tx *gorm.DB, id uuid.UUID) (bool, error) {
	return catalogReferenced(tx, id, catalogReference{&model.AssetProfile{}, "site_id = ?"}, catalogReference{&model.SpareStock{}, "site_id = ? OR destination_site_id = ?"}, catalogReference{&model.SpareMinimum{}, "site_id = ?"}, catalogReference{&model.AssetCheckTask{}, "site_id = ?"}, catalogReference{&model.SpareMovement{}, "source_site_id = ? OR destination_site_id = ?"})
}
func stockHasHistory(tx *gorm.DB, id uuid.UUID) (bool, error) {
	return catalogReferenced(tx, id, catalogReference{&model.SpareMovement{}, "stock_id = ? OR destination_stock_id = ?"}, catalogReference{&model.AssetCheckLine{}, "kind = 'SPARE' AND target_id = ?"})
}
func sameOptionalID(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func catalogWriteError(err error) error {
	if errors.Is(err, gorm.ErrForeignKeyViolated) {
		return opsInvalid("记录仍被其他业务引用，请刷新并处理关联记录后重试")
	}
	return mapStoreError(err)
}

func (s *AssetOperations) UpdateSite(ctx context.Context, actor Actor, id uuid.UUID, in model.AssetSite) (model.AssetSite, error) {
	var out model.AssetSite
	if err := normalizeSite(&in); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.AssetSite
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, "id = ?", id).Error; err != nil {
			return mapStoreError(err)
		}
		if old.Version != in.Version {
			return ErrResourceVersion
		}
		if old.Kind != in.Kind || !sameOptionalID(old.RoomID, in.RoomID) {
			used, err := siteReferenced(tx, id)
			if err != nil {
				return err
			}
			if used {
				return opsInvalid("位置已有关联业务，仅可修改区域、名称及地址；迁移请使用资产或库存业务流程")
			}
		}
		if in.RoomID != nil {
			var room model.Room
			if err := tx.First(&room, "id = ?", *in.RoomID).Error; err != nil {
				return mapStoreError(err)
			}
		}
		out = old
		out.Name = in.Name
		out.Region = in.Region
		out.Kind = in.Kind
		out.RoomID = in.RoomID
		out.Address = in.Address
		out.Version++
		if err := tx.Omit("Room").Save(&out).Error; err != nil {
			return catalogWriteError(err)
		}
		return s.Audit(tx, actor, "UPDATE", "asset_site", id, old, out)
	})
	return out, err
}

func (s *AssetOperations) UpdateSpec(ctx context.Context, actor Actor, id uuid.UUID, in model.SpareSpec) (model.SpareSpec, error) {
	var out model.SpareSpec
	if err := normalizeSpec(&in); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.SpareSpec
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, "id = ?", id).Error; err != nil {
			return mapStoreError(err)
		}
		if old.Version != in.Version {
			return ErrResourceVersion
		}
		if old.Tracking != in.Tracking || old.Unit != in.Unit {
			used, err := catalogReferenced(tx, id, catalogReference{&model.SpareStock{}, "spec_id = ?"})
			if err != nil {
				return err
			}
			if used {
				return opsInvalid("该规格已有备件档案，不能改变按件/数量管理方式或单位")
			}
		}
		out = in
		out.BaseModel = old.BaseModel
		out.Version++
		if err := tx.Save(&out).Error; err != nil {
			return catalogWriteError(err)
		}
		return s.Audit(tx, actor, "UPDATE", "spare_spec", id, old, out)
	})
	return out, err
}

type StockUpdateInput struct {
	Notes             *string    `json:"notes"`
	Version           uint       `json:"version"`
	SpecID            uuid.UUID  `json:"specId"`
	SiteID            uuid.UUID  `json:"siteId"`
	Bin               string     `json:"bin"`
	SerialNumber      string     `json:"serialNumber"`
	Batch             string     `json:"batch"`
	WarrantyExpiresAt *time.Time `json:"warrantyExpiresAt"`
}

func stockDraft(x model.SpareStock) bool {
	return x.Quantity == 0 && x.State == "AVAILABLE" && x.InstalledDeviceID == nil && x.DestinationSiteID == nil
}
func (s *AssetOperations) UpdateStock(ctx context.Context, actor Actor, id uuid.UUID, in StockUpdateInput) (model.SpareStock, error) {
	var out model.SpareStock
	in.SerialNumber = strings.ToUpper(strings.TrimSpace(in.SerialNumber))
	in.Bin = strings.TrimSpace(in.Bin)
	in.Batch = strings.TrimSpace(in.Batch)
	if in.SpecID == uuid.Nil || in.SiteID == uuid.Nil || len(in.SerialNumber) > 120 || len(in.Bin) > 160 || len(in.Batch) > 120 {
		return out, opsInvalid("请填写有效规格、位置及未超长的备件信息")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old model.SpareStock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, "id = ?", id).Error; err != nil {
			return mapStoreError(err)
		}
		if old.Version != in.Version {
			return ErrResourceVersion
		}
		if old.SpecID != in.SpecID || old.SiteID != in.SiteID || old.SerialNumber != in.SerialNumber {
			history, err := stockHasHistory(tx, id)
			if err != nil {
				return err
			}
			if !stockDraft(old) || history {
				return opsInvalid("已有库存或历史的备件不能直接修改规格、所在位置或 SN；位置变更请办理调拨")
			}
		}
		var spec model.SpareSpec
		var site model.AssetSite
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&spec, "id = ?", in.SpecID).Error; err != nil {
			return mapStoreError(err)
		}
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&site, "id = ?", in.SiteID).Error; err != nil {
			return mapStoreError(err)
		}
		if (spec.Tracking == "SERIAL" && in.SerialNumber == "") || (spec.Tracking == "QUANTITY" && in.SerialNumber != "") {
			return opsInvalid("按件备件必须填写 SN，按数量备件不能填写单件 SN")
		}
		out = old
		out.SpecID = in.SpecID
		out.SiteID = in.SiteID
		out.SerialNumber = in.SerialNumber
		out.Bin = in.Bin
		if in.Notes != nil {
			notes := strings.TrimSpace(*in.Notes)
			if len(notes) > 1000 {
				return opsInvalid("备注不能超过 1000 字节")
			}
			out.Notes = notes
		}
		out.Batch = in.Batch
		out.WarrantyExpiresAt = in.WarrantyExpiresAt
		out.Version++
		if out.WarrantyExpiresAt != nil {
			date := dateOnly(*out.WarrantyExpiresAt)
			out.WarrantyExpiresAt = &date
		}
		if err := tx.Omit("Spec", "Site", "InstalledDevice").Save(&out).Error; err != nil {
			return catalogWriteError(err)
		}
		return s.Audit(tx, actor, "UPDATE", "spare_stock", id, old, out)
	})
	return out, err
}

func (s *AssetOperations) UpdateMinimum(ctx context.Context, actor Actor, id uuid.UUID, version uint, minimum int) (model.SpareMinimum, error) {
	var out model.SpareMinimum
	if minimum < 0 || minimum > 1000000 {
		return out, opsInvalid("最低库存应为 0–1000000 的整数")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&out, "id = ?", id).Error; err != nil {
			return mapStoreError(err)
		}
		if out.Version != version {
			return ErrResourceVersion
		}
		before := out
		out.Minimum = minimum
		out.Version++
		if err := tx.Omit("Spec", "Site").Save(&out).Error; err != nil {
			return err
		}
		return s.Audit(tx, actor, "UPDATE", "spare_minimum", id, before, out)
	})
	return out, err
}

// Only unused catalog rows are physically removed. All business history remains,
// and PostgreSQL foreign keys reject a concurrent new reference at delete time.
func (s *AssetOperations) DeleteCatalog(ctx context.Context, actor Actor, kind string, id uuid.UUID, version uint) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var entity interface{}
		var before interface{}
		var actual uint
		var used bool
		var err error
		switch kind {
		case "asset_site":
			var x model.AssetSite
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, "id = ?", id).Error; err != nil {
				return mapStoreError(err)
			}
			entity = &x
			before = x
			actual = x.Version
			if actual != version {
				return ErrResourceVersion
			}
			used, err = siteReferenced(tx, id)
		case "spare_spec":
			var x model.SpareSpec
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, "id = ?", id).Error; err != nil {
				return mapStoreError(err)
			}
			entity = &x
			before = x
			actual = x.Version
			if actual != version {
				return ErrResourceVersion
			}
			used, err = catalogReferenced(tx, id, catalogReference{&model.SpareStock{}, "spec_id = ?"}, catalogReference{&model.SpareMinimum{}, "spec_id = ?"})
		case "spare_stock":
			var x model.SpareStock
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, "id = ?", id).Error; err != nil {
				return mapStoreError(err)
			}
			entity = &x
			before = x
			actual = x.Version
			if actual != version {
				return ErrResourceVersion
			}
			if !stockDraft(x) {
				return opsInvalid("仅可删除未入库的零库存空档；已有库存请通过办理、调拨或报废处理")
			}
			used, err = stockHasHistory(tx, id)
		case "spare_minimum":
			var x model.SpareMinimum
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&x, "id = ?", id).Error; err != nil {
				return mapStoreError(err)
			}
			entity = &x
			before = x
			actual = x.Version
		default:
			return opsInvalid("不支持删除此类记录")
		}
		if actual != version {
			return ErrResourceVersion
		}
		if err != nil {
			return err
		}
		if used {
			return opsInvalid("记录已被资产、库存、规则、流水或盘点引用，不能删除；请先处理关联业务，历史记录需保留")
		}
		if err = tx.Unscoped().Delete(entity).Error; err != nil {
			return catalogWriteError(err)
		}
		if kind == "spare_stock" || kind == "spare_minimum" {
			prefix := "partwarranty:"
			if kind == "spare_minimum" {
				prefix = "minimum:"
			}
			if err = tx.Model(&model.AssetReminder{}).Where("key = ?", prefix+id.String()).Update("active", false).Error; err != nil {
				return err
			}
		}
		return s.Audit(tx, actor, "DELETE", kind, id, before, nil)
	})
}
