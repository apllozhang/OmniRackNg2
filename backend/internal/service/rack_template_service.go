package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
)

var (
	ErrSystemTemplate   = errors.New("system template cannot be deleted or disabled")
	ErrTemplateDisabled = errors.New("rack template is disabled")
)

type RackTemplateCreateInput struct {
	Code             string                    `json:"code"`
	AutoGenerateCode bool                      `json:"autoGenerateCode"`
	Name             string                    `json:"name"`
	Description      string                    `json:"description"`
	Status           string                    `json:"status"`
	Remarks          string                    `json:"remarks"`
	Spec             model.RackTemplateVersion `json:"spec"`
}

type RackTemplateUpdateInput struct {
	Version     uint   `json:"version"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Remarks     string `json:"remarks"`
}

type RackTemplateService struct{ store repository.RackTemplateStore }

func NewRackTemplateService(store repository.RackTemplateStore) *RackTemplateService {
	return &RackTemplateService{store: store}
}

func (s *RackTemplateService) List(ctx context.Context) ([]model.RackTemplate, error) {
	return s.store.ListRackTemplates(ctx)
}

func (s *RackTemplateService) Create(ctx context.Context, actor Actor, input RackTemplateCreateInput) (model.RackTemplate, error) {
	if input.AutoGenerateCode {
		input.Code = generateAutoCode("TPL")
	}
	template := model.RackTemplate{
		Code: input.Code, Name: input.Name, Description: input.Description,
		Status: input.Status, Remarks: input.Remarks, CurrentRevision: 1,
	}
	normalizeTemplate(&template)
	if err := validateTemplate(template); err != nil {
		return model.RackTemplate{}, err
	}
	version := input.Spec
	normalizeTemplateVersion(&version)
	if err := validateTemplateVersion(version); err != nil {
		return model.RackTemplate{}, err
	}
	version.Revision = 1
	version.CreatedBy = actor.UserID

	err := s.store.WithinTemplateTransaction(ctx, func(store repository.RackTemplateStore) error {
		if err := store.CreateRackTemplate(ctx, &template); err != nil {
			return mapStoreError(err)
		}
		version.TemplateID = template.ID
		if err := store.CreateRackTemplateVersion(ctx, &version); err != nil {
			return mapStoreError(err)
		}
		template.Versions = []model.RackTemplateVersion{version}
		return createAudit(ctx, store, actor, "CREATE", "rack_template", template.ID, nil, template)
	})
	return template, err
}

func (s *RackTemplateService) Update(ctx context.Context, actor Actor, id uuid.UUID, input RackTemplateUpdateInput) (model.RackTemplate, error) {
	if input.Version == 0 {
		return model.RackTemplate{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	var updated model.RackTemplate
	err := s.store.WithinTemplateTransaction(ctx, func(store repository.RackTemplateStore) error {
		current, err := store.GetRackTemplate(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		item := current
		item.Code = input.Code
		item.Name = input.Name
		item.Description = input.Description
		item.Status = input.Status
		item.Remarks = input.Remarks
		normalizeTemplate(&item)
		if item.IsSystem && item.Status == model.RackTemplateStatusDisabled {
			return fmt.Errorf("%w: 系统内置 42U 模板不能停用", ErrSystemTemplate)
		}
		if err := validateTemplate(item); err != nil {
			return err
		}
		if err := store.UpdateRackTemplate(ctx, &item, input.Version); err != nil {
			return mapStoreError(err)
		}
		updated = item
		return createAudit(ctx, store, actor, "UPDATE", "rack_template", id, current, updated)
	})
	return updated, err
}

func (s *RackTemplateService) CreateVersion(ctx context.Context, actor Actor, templateID uuid.UUID, expectedVersion uint, item model.RackTemplateVersion) (model.RackTemplateVersion, error) {
	if expectedVersion == 0 {
		return model.RackTemplateVersion{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	normalizeTemplateVersion(&item)
	if err := validateTemplateVersion(item); err != nil {
		return model.RackTemplateVersion{}, err
	}
	item.CreatedBy = actor.UserID
	err := s.store.WithinTemplateTransaction(ctx, func(store repository.RackTemplateStore) error {
		template, err := store.GetRackTemplate(ctx, templateID)
		if err != nil {
			return mapStoreError(err)
		}
		if template.Status != model.RackTemplateStatusActive {
			return fmt.Errorf("%w: 停用模板不能创建新版本", ErrTemplateDisabled)
		}
		item.TemplateID = template.ID
		item.Revision = template.CurrentRevision + 1
		if err := store.CreateRackTemplateVersion(ctx, &item); err != nil {
			return mapStoreError(err)
		}
		if err := store.AdvanceRackTemplateRevision(ctx, template.ID, expectedVersion, item.Revision); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "CREATE_VERSION", "rack_template", template.ID, template, item)
	})
	return item, err
}

func (s *RackTemplateService) Delete(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint) error {
	if expectedVersion == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	return s.store.WithinTemplateTransaction(ctx, func(store repository.RackTemplateStore) error {
		current, err := store.GetRackTemplate(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if current.IsSystem {
			return fmt.Errorf("%w: 系统内置 42U 模板不能删除", ErrSystemTemplate)
		}
		if err := store.DeleteRackTemplate(ctx, id, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "DELETE", "rack_template", id, current, nil)
	})
}

func normalizeTemplate(item *model.RackTemplate) {
	item.Code = strings.ToUpper(strings.TrimSpace(item.Code))
	item.Name = strings.TrimSpace(item.Name)
	item.Description = strings.TrimSpace(item.Description)
	item.Status = strings.ToUpper(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = model.RackTemplateStatusActive
	}
	item.Remarks = strings.TrimSpace(item.Remarks)
}

func validateTemplate(item model.RackTemplate) error {
	if item.Code == "" || len(item.Code) > 50 {
		return fmt.Errorf("%w: 模板编码不能为空且不能超过 50 个字符", ErrInvalidResource)
	}
	if item.Name == "" || len(item.Name) > 150 {
		return fmt.Errorf("%w: 模板名称不能为空且不能超过 150 个字符", ErrInvalidResource)
	}
	if item.Status != model.RackTemplateStatusActive && item.Status != model.RackTemplateStatusDisabled {
		return fmt.Errorf("%w: 模板状态仅支持 ACTIVE 或 DISABLED", ErrInvalidResource)
	}
	return nil
}

func normalizeTemplateVersion(item *model.RackTemplateVersion) {
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
	item.ChangeNote = strings.TrimSpace(item.ChangeNote)
}

func validateTemplateVersion(item model.RackTemplateVersion) error {
	if item.UHeight < 1 || item.UHeight > 100 {
		return fmt.Errorf("%w: 模板 U 位高度必须是 1 到 100 的整数", ErrInvalidResource)
	}
	if item.WidthMm <= 0 || item.DepthMm <= 0 || item.HeightMm <= 0 {
		return fmt.Errorf("%w: 模板尺寸必须大于 0", ErrInvalidResource)
	}
	if item.InputCircuits < 0 || item.PDUCount < 0 || negative(item.LoadCapacityKg, item.RatedVoltage, item.RatedCurrent, item.RatedPowerKw, item.PeakPowerKw) {
		return fmt.Errorf("%w: 模板电源、PDU 和承重参数不能为负数", ErrInvalidResource)
	}
	return nil
}
