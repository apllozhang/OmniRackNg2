package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

const (
	MaxDevices        = 3000
	MaxDevicesPerRack = 20
	MaxListPage       = 10000
)

var (
	ErrDeviceTypeInUse     = errors.New("device type is in use")
	ErrDevicePositioned    = errors.New("device has active rack position")
	ErrDeviceNotPositioned = errors.New("device has no active rack position")
	ErrRackUnavailable     = errors.New("rack is unavailable")
	ErrApprovalState       = errors.New("approval state conflict")
)

// UConflictError contains actionable conflict details for API clients.
type UConflictError struct {
	RackID    uuid.UUID
	StartU    int
	HeightU   int
	Conflicts []model.RackDevicePosition
}

func (e *UConflictError) Error() string { return "设备 U 位与已有设备冲突" }

// DeviceListResult is a paginated ledger response.
type DeviceListResult struct {
	Items    []model.Device `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

// PositionChangeInput describes assign and move targets.
type PositionChangeInput struct {
	expectedSource *model.DevicePositionSnapshot
	Version        uint      `json:"version"`
	RackID         uuid.UUID `json:"rackId"`
	StartU         int       `json:"startU"`
	HeightU        int       `json:"heightU"`
	Orientation    string    `json:"orientation"`
	Reason         string    `json:"reason"`
}

// DecommissionInput removes a device from its active rack position.
type DecommissionInput struct {
	Version uint   `json:"version"`
	Reason  string `json:"reason"`
}

// OperationResult tells the client whether a position change executed or awaits approval.
type OperationResult struct {
	Executed bool                      `json:"executed"`
	Device   *model.Device             `json:"device,omitempty"`
	Position *model.RackDevicePosition `json:"position,omitempty"`
	Approval *model.ApprovalRecord     `json:"approval,omitempty"`
}

// RackFreeRange is an inclusive free U interval.
type RackFreeRange struct {
	StartU  int `json:"startU"`
	EndU    int `json:"endU"`
	HeightU int `json:"heightU"`
}

// RackULayout provides the unified U layout and available ranges for visualization.
type RackULayout struct {
	Rack      model.Rack                 `json:"rack"`
	Positions []model.RackDevicePosition `json:"positions"`
	Free      []RackFreeRange            `json:"free"`
}

// PolicyUpdateInput updates the configurable single-level approval switches.
type PolicyUpdateInput struct {
	DecommissionApprovalEnabled *bool  `json:"decommissionApprovalEnabled"`
	Version                     uint   `json:"version"`
	AssignApprovalEnabled       bool   `json:"assignApprovalEnabled"`
	MoveApprovalEnabled         bool   `json:"moveApprovalEnabled"`
	DefaultApproverRole         string `json:"defaultApproverRole"`
}

// ApprovalDecisionInput is used for approve/reject actions.
type ApprovalDecisionInput struct {
	Version uint   `json:"version"`
	Comment string `json:"comment"`
}

// DeviceCreateInput contains create-only options that are not persisted.
type DeviceCreateInput struct {
	Device           model.Device
	AutoGenerateCode bool
}

// DeviceService implements device ledger, U-position, history, and approval use cases.
type DeviceService struct {
	store  repository.DeviceStore
	drafts *rackDiagramDraftStore
}

func NewDeviceService(store repository.DeviceStore) *DeviceService {
	return &DeviceService{store: store, drafts: newRackDiagramDraftStore()}
}

func (s *DeviceService) ListDeviceTypes(ctx context.Context) ([]model.DeviceType, error) {
	return s.store.ListDeviceTypes(ctx)
}

func (s *DeviceService) CreateDeviceType(ctx context.Context, actor Actor, item model.DeviceType) (model.DeviceType, error) {
	normalizeDeviceType(&item)
	if err := validateDeviceType(item); err != nil {
		return model.DeviceType{}, err
	}
	err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		if err := store.CreateDeviceType(ctx, &item); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "CREATE", "device_type", item.ID, nil, item)
	})
	return item, err
}

func (s *DeviceService) UpdateDeviceType(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint, item model.DeviceType) (model.DeviceType, error) {
	if expectedVersion == 0 {
		return model.DeviceType{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	normalizeDeviceType(&item)
	if err := validateDeviceType(item); err != nil {
		return model.DeviceType{}, err
	}
	var updated model.DeviceType
	err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetDeviceType(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		item.ID, item.CreatedAt = current.ID, current.CreatedAt
		if err := store.UpdateDeviceType(ctx, &item, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		updated = item
		return createAudit(ctx, store, actor, "UPDATE", "device_type", id, current, updated)
	})
	return updated, err
}

func (s *DeviceService) DeleteDeviceType(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint) error {
	if expectedVersion == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	return s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetDeviceType(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		count, err := store.CountDevicesByType(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("%w: 请先调整引用此类型的设备", ErrDeviceTypeInUse)
		}
		if err := store.DeleteDeviceType(ctx, id, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "DELETE", "device_type", id, current, nil)
	})
}

func (s *DeviceService) ListDevices(ctx context.Context, query repository.DeviceQuery) (DeviceListResult, error) {
	if query.Scope != "" && query.Scope != "PC" && query.Scope != "ROOM" {
		return DeviceListResult{}, opsInvalid("设备范围无效")
	}
	if query.Page <= 0 {
		query.Page = 1
	}
	if query.Page > MaxListPage {
		return DeviceListResult{}, opsInvalid("page 不能超过 10000")
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}
	if query.PageSize > 100 {
		query.PageSize = 100
	}
	query.Search = strings.TrimSpace(query.Search)
	query.LifecycleStatus = strings.ToUpper(strings.TrimSpace(query.LifecycleStatus))
	items, total, err := s.store.ListDevices(ctx, query)
	return DeviceListResult{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, err
}

func (s *DeviceService) GetDevice(ctx context.Context, id uuid.UUID) (model.Device, error) {
	item, err := s.store.GetDevice(ctx, id)
	if err != nil {
		return model.Device{}, mapStoreError(err)
	}
	return item, nil
}

func (s *DeviceService) CreateDevice(ctx context.Context, actor Actor, input DeviceCreateInput) (model.Device, error) {
	item := input.Device
	if input.AutoGenerateCode {
		item.Code = generateAutoCode("DEV")
	}
	typeItem, err := s.store.GetDeviceType(ctx, item.TypeID)
	if err != nil {
		return model.Device{}, mapStoreError(err)
	}
	if typeItem.Status != model.DeviceTypeStatusActive {
		return model.Device{}, fmt.Errorf("%w: 停用设备类型不能创建设备", ErrParentDisabled)
	}
	applyDeviceTypeDefaults(&item, typeItem)
	normalizeDevice(&item)
	if err := validateDevice(item); err != nil {
		return model.Device{}, err
	}
	err = s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		count, err := store.CountDevices(ctx)
		if err != nil {
			return err
		}
		if count >= MaxDevices {
			return fmt.Errorf("%w: 设备总数最多允许 %d 台", ErrResourceLimit, MaxDevices)
		}
		if err := store.CreateDevice(ctx, &item); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "CREATE", "device", item.ID, nil, item)
	})
	item.Type = typeItem
	return item, err
}

func (s *DeviceService) UpdateDevice(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint, item model.Device) (model.Device, error) {
	if expectedVersion == 0 {
		return model.Device{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	normalizeDevice(&item)
	if err := validateDevice(item); err != nil {
		return model.Device{}, err
	}
	var updated model.Device
	err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetDeviceForUpdate(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if current.Version != expectedVersion {
			return ErrResourceVersion
		}
		typeItem, err := store.GetDeviceType(ctx, item.TypeID)
		if err != nil {
			return mapStoreError(err)
		}
		if typeItem.Status != model.DeviceTypeStatusActive && typeItem.ID != current.TypeID {
			return fmt.Errorf("%w: 停用设备类型不能用于设备", ErrParentDisabled)
		}
		if typeItem.Category == model.DeviceCategoryPC && item.LifecycleStatus != current.LifecycleStatus {
			return opsInvalid("终端使用状态请在终端管理通过领用、归还、维修或报废操作变更")
		}
		position, positionErr := store.GetPositionByDevice(ctx, id)
		positioned := positionErr == nil
		if positionErr != nil && !errors.Is(positionErr, gorm.ErrRecordNotFound) {
			return positionErr
		}
		if positioned && item.HeightU != current.HeightU {
			return fmt.Errorf("%w: 已上架设备变更 U 高度时请通过迁移操作重新校验位置", ErrDevicePositioned)
		}
		if positioned && (item.LifecycleStatus == model.DeviceStatusWaitingRack || item.LifecycleStatus == model.DeviceStatusOffRack || item.LifecycleStatus == model.DeviceStatusScrapped) {
			return fmt.Errorf("%w: 已上架设备不能设置为待上架、已下架或报废", ErrInvalidResource)
		}
		if !positioned && item.LifecycleStatus == model.DeviceStatusRunning && typeItem.Category != model.DeviceCategoryPC {
			return fmt.Errorf("%w: 未上架设备不能设置为运行中", ErrInvalidResource)
		}
		_ = position
		item.ID, item.CreatedAt = current.ID, current.CreatedAt
		if err := store.UpdateDevice(ctx, &item, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		updated = item
		return createAudit(ctx, store, actor, "UPDATE", "device", id, current, updated)
	})
	return updated, err
}

func (s *DeviceService) DeleteDevice(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint) error {
	if expectedVersion == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	return s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetDeviceForUpdate(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if current.Version != expectedVersion {
			return ErrResourceVersion
		}
		if _, err := store.GetPositionByDevice(ctx, id); err == nil {
			return fmt.Errorf("%w: 请先下架设备", ErrDevicePositioned)
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if guard, ok := store.(repository.AssetDeletionGuard); ok {
			if err := guard.AssertCanDeleteAsset(ctx, id); err != nil {
				if errors.Is(err, repository.ErrAssetRelations) {
					return fmt.Errorf("%w: 已有资产扩展、服务、盘点或部件关联，请保留历史并使用报废状态", ErrHasChildren)
				}
				return err
			}
		}
		if err := store.DeleteDevice(ctx, id, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "DELETE", "device", id, current, nil)
	})
}
