package service

import (
	"context"
	"fmt"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
)

func (s *ResourceService) ListTree(ctx context.Context) ([]model.DataCenter, error) {
	return s.store.ListResourceTree(ctx)
}

func (s *ResourceService) CreateDataCenter(ctx context.Context, actor Actor, input DataCenterCreateInput) (model.DataCenter, error) {
	item := input.DataCenter
	if input.AutoGenerateCode {
		item.Code = generateAutoCode("DC")
	}
	normalizeDataCenter(&item)
	if err := validateDataCenter(item); err != nil {
		return model.DataCenter{}, err
	}
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		if err := store.CreateDataCenter(ctx, &item); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "CREATE", "data_center", item.ID, nil, item)
	})
	return item, err
}

func (s *ResourceService) UpdateDataCenter(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint, item model.DataCenter) (model.DataCenter, error) {
	if expectedVersion == 0 {
		return model.DataCenter{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	normalizeDataCenter(&item)
	if err := validateDataCenter(item); err != nil {
		return model.DataCenter{}, err
	}
	var updated model.DataCenter
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetDataCenter(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		item.ID = current.ID
		item.CreatedAt = current.CreatedAt
		item.Version = current.Version
		if err := store.UpdateDataCenter(ctx, &item, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		updated = item
		return createAudit(ctx, store, actor, "UPDATE", "data_center", id, current, updated)
	})
	return updated, err
}

func (s *ResourceService) DeleteDataCenter(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint) error {
	if expectedVersion == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	return s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetDataCenter(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		count, err := store.CountRoomsByDataCenter(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("%w: 请先迁移或删除下属机房", ErrHasChildren)
		}
		if err := store.DeleteDataCenter(ctx, id, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "DELETE", "data_center", id, current, nil)
	})
}

func (s *ResourceService) CreateRoom(ctx context.Context, actor Actor, dataCenterID uuid.UUID, input RoomWriteInput) (model.Room, error) {
	item := input.Room
	if input.AutoGenerateCode {
		item.Code = generateAutoCode("ROOM")
	}
	if input.RacksPerRow == nil {
		item.RacksPerRow = 8
	} else {
		item.RacksPerRow = *input.RacksPerRow
	}
	normalizeRoom(&item)
	if err := validateRoom(item); err != nil {
		return model.Room{}, err
	}
	item.DataCenterID = dataCenterID
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		parent, err := store.GetDataCenter(ctx, dataCenterID)
		if err != nil {
			return mapStoreError(err)
		}
		if isDisabled(parent.Status) {
			return fmt.Errorf("%w: 停用或归档的数据中心不能新增机房", ErrParentDisabled)
		}
		if err := store.CreateRoom(ctx, &item); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "CREATE", "room", item.ID, nil, item)
	})
	return item, err
}

func (s *ResourceService) UpdateRoom(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint, input RoomWriteInput) (model.Room, error) {
	if expectedVersion == 0 {
		return model.Room{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	item := input.Room
	var updated model.Room
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetRoom(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if input.RacksPerRow == nil {
			item.RacksPerRow = current.RacksPerRow
		} else {
			item.RacksPerRow = *input.RacksPerRow
		}
		normalizeRoom(&item)
		if err := validateRoom(item); err != nil {
			return err
		}
		item.ID = current.ID
		item.DataCenterID = current.DataCenterID
		item.CreatedAt = current.CreatedAt
		item.Version = current.Version
		if err := store.UpdateRoom(ctx, &item, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		updated = item
		return createAudit(ctx, store, actor, "UPDATE", "room", id, current, updated)
	})
	return updated, err
}

func (s *ResourceService) DeleteRoom(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint) error {
	if expectedVersion == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	return s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetRoom(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		count, err := store.CountRacksByRoom(ctx, id)
		if err != nil {
			return err
		}
		if count > 0 {
			return fmt.Errorf("%w: 请先迁移或删除下属机柜", ErrHasChildren)
		}
		if err := store.DeleteRoom(ctx, id, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "DELETE", "room", id, current, nil)
	})
}

func (s *ResourceService) CreateRack(ctx context.Context, actor Actor, roomID uuid.UUID, input RackCreateInput) (model.Rack, error) {
	item := input.Rack
	requestedSpec := item
	if input.AutoGenerateCode {
		item.Code = generateAutoCode("RACK")
	}
	if item.TemplateVersionID != nil {
		if s.templateReader == nil {
			return model.Rack{}, fmt.Errorf("%w: 当前服务未配置机柜模板读取器", ErrInvalidResource)
		}
		version, err := s.templateReader.GetRackTemplateVersion(ctx, *item.TemplateVersionID)
		if err != nil {
			return model.Rack{}, mapStoreError(err)
		}
		if version.Template.Status != model.RackTemplateStatusActive {
			return model.Rack{}, fmt.Errorf("%w: 停用模板不能用于创建机柜", ErrTemplateDisabled)
		}
		applyRackTemplate(&item, version)
		if input.OverrideTemplateSpec {
			applyRackSpecOverride(&item, requestedSpec)
		}
	}
	normalizeRack(&item)
	if err := validateRack(item); err != nil {
		return model.Rack{}, err
	}
	item.RoomID = roomID
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		parent, err := store.GetRoom(ctx, roomID)
		if err != nil {
			return mapStoreError(err)
		}
		if isDisabled(parent.Status) {
			return fmt.Errorf("%w: 停用或归档的机房不能新增机柜", ErrParentDisabled)
		}
		item.DataCenterID = parent.DataCenterID
		if err := store.CreateRack(ctx, &item); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "CREATE", "rack", item.ID, nil, item)
	})
	return item, err
}

func (s *ResourceService) UpdateRack(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint, item model.Rack) (model.Rack, error) {
	if expectedVersion == 0 {
		return model.Rack{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	normalizeRack(&item)
	if err := validateRack(item); err != nil {
		return model.Rack{}, err
	}
	var updated model.Rack
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetRack(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		item.ID = current.ID
		item.DataCenterID = current.DataCenterID
		item.RoomID = current.RoomID
		item.TemplateID = current.TemplateID
		item.TemplateVersionID = current.TemplateVersionID
		item.TemplateCode = current.TemplateCode
		item.TemplateName = current.TemplateName
		item.TemplateRevision = current.TemplateRevision
		item.TemplateSnapshot = current.TemplateSnapshot
		item.CreatedAt = current.CreatedAt
		item.Version = current.Version
		if err := store.UpdateRack(ctx, &item, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		updated = item
		return createAudit(ctx, store, actor, "UPDATE", "rack", id, current, updated)
	})
	return updated, err
}

func (s *ResourceService) DeleteRack(ctx context.Context, actor Actor, id uuid.UUID, expectedVersion uint) error {
	if expectedVersion == 0 {
		return fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	return s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetRack(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		positionCount, err := store.CountDevicePositionsByRack(ctx, id)
		if err != nil {
			return err
		}
		if positionCount > 0 {
			return fmt.Errorf("%w: 机柜内仍有设备，请先迁移或下架设备", ErrHasChildren)
		}
		if err := store.DeleteRack(ctx, id, expectedVersion); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "DELETE", "rack", id, current, nil)
	})
}
