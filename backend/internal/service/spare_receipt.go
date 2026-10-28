package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

type SpareReceiptInput struct {
	SubmissionID uuid.UUID  `json:"submissionId"`
	SpecID       *uuid.UUID `json:"specId"`
	SiteID       uuid.UUID  `json:"siteId"`
	Category     string     `json:"category"`
	Name         string     `json:"name"`
	ModelNumber  string     `json:"modelNumber"`
	Quantity     int        `json:"quantity"`
	SerialNumber string     `json:"serialNumber"`
	Notes        string     `json:"notes"`
	State        string     `json:"state"`
}

// The simple receipt is one transaction over the existing spec, stock and IN movement.
func validSpareReceiptState(state string) bool {
	switch state {
	case "AVAILABLE", "TESTING", "FAULTY", "REPAIR":
		return true
	}
	return false
}

func (s *AssetOperations) ReceiveSpare(ctx context.Context, actor Actor, in SpareReceiptInput) (model.SpareStock, error) {
	var out model.SpareStock
	in.Category, in.Name, in.ModelNumber = strings.TrimSpace(in.Category), strings.TrimSpace(in.Name), strings.TrimSpace(in.ModelNumber)
	in.SerialNumber, in.Notes = strings.ToUpper(strings.TrimSpace(in.SerialNumber)), strings.TrimSpace(in.Notes)
	in.State = strings.ToUpper(strings.TrimSpace(in.State))
	if in.State == "" {
		in.State = "AVAILABLE"
	}
	if actor.UserID == nil || in.SubmissionID == uuid.Nil || in.SiteID == uuid.Nil || in.Quantity < 1 || in.Quantity > 1000000 || len(in.SerialNumber) > 120 || len(in.Notes) > 1000 || (in.SerialNumber != "" && in.Quantity != 1) || !validSpareReceiptState(in.State) {
		return out, opsInvalid("请填写位置、有效数量及初始状态；单个序列号只能登记 1 件")
	}
	if in.SpecID == nil && (in.Name == "" || in.Category == "" || len(in.Name) > 160 || len(in.Category) > 40 || len(in.ModelNumber) > 160) {
		return out, opsInvalid("首次登记请填写类型、名称及未超长的型号")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", in.SubmissionID.String()).Error; err != nil {
			return err
		}
		var prior model.SpareMovement
		err := tx.Where("submission_id = ?", in.SubmissionID).First(&prior).Error
		if err == nil {
			state := prior.AfterState
			if state == "" {
				state = "AVAILABLE"
			}
			if state != in.State {
				return opsInvalid("重复提交的初始状态不一致，请刷新后重试")
			}
			if prior.ActorID != *actor.UserID || prior.Operation != "IN" || prior.Quantity != in.Quantity || !strings.HasPrefix(prior.Reason, "备件入库登记") {
				return opsInvalid("提交标识冲突，请刷新后重试")
			}
			if err := tx.Preload("Spec").Preload("Site").First(&out, "id = ?", prior.StockID).Error; err != nil {
				return mapStoreError(err)
			}
			if out.SiteID != in.SiteID || out.SerialNumber != in.SerialNumber || out.Notes != in.Notes || (in.SpecID != nil && out.SpecID != *in.SpecID) || (in.SpecID == nil && (out.Spec.Name != in.Name || out.Spec.Category != in.Category || out.Spec.ModelNumber != in.ModelNumber)) {
				return opsInvalid("重复提交内容不一致，请刷新后重试")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var spec model.SpareSpec
		if in.SpecID != nil {
			if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).First(&spec, "id = ?", *in.SpecID).Error; err != nil {
				return mapStoreError(err)
			}
		} else {
			tracking := "QUANTITY"
			if in.SerialNumber != "" {
				tracking = "SERIAL"
			}
			key := fmt.Sprintf("%x", sha256.Sum256([]byte(in.Category+"\x00"+in.Name+"\x00"+in.ModelNumber+"\x00"+tracking)))
			code := "AUTO-SPARE-" + key[:32]
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "spare-spec:"+code).Error; err != nil {
				return err
			}
			err := tx.Where("part_number = ?", code).First(&spec).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				spec = model.SpareSpec{Name: in.Name, Category: in.Category, ModelNumber: in.ModelNumber, PartNumber: code, Specification: in.ModelNumber, Tracking: tracking, Unit: "件"}
				if spec.Specification == "" {
					spec.Specification = "未填写详细规格"
				}
				if err := tx.Create(&spec).Error; err != nil {
					return catalogWriteError(err)
				}
				if err := s.Audit(tx, actor, "CREATE", "spare_spec", spec.ID, nil, spec); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
		}
		ops := NewAssetOperations(tx)
		stock, err := ops.CreateStock(ctx, actor, model.SpareStock{SpecID: spec.ID, SiteID: in.SiteID, SerialNumber: in.SerialNumber, Notes: in.Notes})
		if err != nil {
			return err
		}
		reason := "备件入库登记"
		if in.Notes != "" {
			reason += "\n" + in.Notes
		}
		_, err = ops.MoveStock(ctx, actor, stock.ID, SpareMoveInput{Version: stock.Version, SubmissionID: in.SubmissionID, Operation: "IN", Quantity: in.Quantity, Reason: reason, receiptState: in.State})
		if err != nil {
			return err
		}
		stock.Quantity = in.Quantity
		stock.State = in.State
		stock.Version++
		stock.Spec = spec
		out = stock
		return nil
	})
	return out, err
}
