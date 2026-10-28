package service

import (
	"context"
	"fmt"
	"strings"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
)

func (s *ResourceService) CopyDataCenter(ctx context.Context, actor Actor, id uuid.UUID, input ResourceCopyInput) (ResourceCopyResult, error) {
	if input.Version == 0 {
		return ResourceCopyResult{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	var result ResourceCopyResult
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		tree, err := store.ListResourceTree(ctx)
		if err != nil {
			return err
		}
		source, ok := findDataCenter(tree, id)
		if !ok {
			return ErrResourceNotFound
		}
		if source.Version != input.Version {
			return ErrResourceVersion
		}
		if source.Status == model.ResourceStatusArchived {
			return fmt.Errorf("%w: 归档的数据中心不能复制", ErrInvalidResource)
		}
		copied := cloneDataCenter(source)
		copied.Code = copyCode(input, "DC")
		copied.Name = copyName(input.Name, source.Name)
		normalizeDataCenter(&copied)
		if err := validateDataCenter(copied); err != nil {
			return err
		}
		if err := store.CreateDataCenter(ctx, &copied); err != nil {
			return mapStoreError(err)
		}
		for _, sourceRoom := range source.Rooms {
			room := cloneRoom(sourceRoom)
			room.DataCenterID = copied.ID
			room.Code = generateAutoCode("ROOM")
			room.Name = sourceRoom.Name
			normalizeRoom(&room)
			if err := validateRoom(room); err != nil {
				return err
			}
			if err := store.CreateRoom(ctx, &room); err != nil {
				return mapStoreError(err)
			}
			result.CopiedRooms++
			for _, sourceRack := range sourceRoom.Racks {
				rack := cloneRack(sourceRack)
				rack.DataCenterID, rack.RoomID = copied.ID, room.ID
				rack.Code = generateAutoCode("RACK")
				normalizeRack(&rack)
				if err := validateRack(rack); err != nil {
					return err
				}
				if err := store.CreateRack(ctx, &rack); err != nil {
					return mapStoreError(err)
				}
				result.CopiedRacks++
			}
		}
		result.Resource = copied
		return createAudit(ctx, store, actor, "COPY", "data_center", copied.ID, source, result)
	})
	return result, err
}

func (s *ResourceService) CopyRoom(ctx context.Context, actor Actor, id uuid.UUID, input ResourceCopyInput) (ResourceCopyResult, error) {
	if input.Version == 0 || input.TargetDataCenterID == uuid.Nil {
		return ResourceCopyResult{}, fmt.Errorf("%w: version 和目标数据中心不能为空", ErrInvalidResource)
	}
	var result ResourceCopyResult
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		tree, err := store.ListResourceTree(ctx)
		if err != nil {
			return err
		}
		source, ok := findRoom(tree, id)
		if !ok {
			return ErrResourceNotFound
		}
		if source.Version != input.Version {
			return ErrResourceVersion
		}
		if source.Status == model.ResourceStatusArchived {
			return fmt.Errorf("%w: 归档的机房不能复制", ErrInvalidResource)
		}
		target, err := store.GetDataCenter(ctx, input.TargetDataCenterID)
		if err != nil {
			return mapStoreError(err)
		}
		if isDisabled(target.Status) {
			return fmt.Errorf("%w: 停用或归档的数据中心不能接收复制资源", ErrParentDisabled)
		}
		room := cloneRoom(source)
		room.DataCenterID = target.ID
		room.Code = copyCode(input, "ROOM")
		room.Name = copyName(input.Name, source.Name)
		normalizeRoom(&room)
		if err := validateRoom(room); err != nil {
			return err
		}
		if err := store.CreateRoom(ctx, &room); err != nil {
			return mapStoreError(err)
		}
		result.CopiedRooms = 1
		for _, sourceRack := range source.Racks {
			rack := cloneRack(sourceRack)
			rack.DataCenterID, rack.RoomID = target.ID, room.ID
			rack.Code = generateAutoCode("RACK")
			normalizeRack(&rack)
			if err := validateRack(rack); err != nil {
				return err
			}
			if err := store.CreateRack(ctx, &rack); err != nil {
				return mapStoreError(err)
			}
			result.CopiedRacks++
		}
		result.Resource = room
		return createAudit(ctx, store, actor, "COPY", "room", room.ID, source, result)
	})
	return result, err
}

func (s *ResourceService) CopyRack(ctx context.Context, actor Actor, id uuid.UUID, input ResourceCopyInput) (ResourceCopyResult, error) {
	if input.Version == 0 || input.TargetRoomID == uuid.Nil {
		return ResourceCopyResult{}, fmt.Errorf("%w: version 和目标机房不能为空", ErrInvalidResource)
	}
	var result ResourceCopyResult
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		source, err := store.GetRack(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if source.Version != input.Version {
			return ErrResourceVersion
		}
		target, err := store.GetRoom(ctx, input.TargetRoomID)
		if err != nil {
			return mapStoreError(err)
		}
		if isDisabled(target.Status) {
			return fmt.Errorf("%w: 停用或归档的机房不能接收复制资源", ErrParentDisabled)
		}
		rack := cloneRack(source)
		rack.DataCenterID, rack.RoomID = target.DataCenterID, target.ID
		rack.Code = copyCode(input, "RACK")
		rack.Name = copyName(input.Name, source.Name)
		normalizeRack(&rack)
		if err := validateRack(rack); err != nil {
			return err
		}
		if err := store.CreateRack(ctx, &rack); err != nil {
			return mapStoreError(err)
		}
		result.Resource, result.CopiedRacks = rack, 1
		return createAudit(ctx, store, actor, "COPY", "rack", rack.ID, source, result)
	})
	return result, err
}

func (s *ResourceService) MoveRoom(ctx context.Context, actor Actor, id uuid.UUID, input ResourceMoveInput) (model.Room, error) {
	if input.Version == 0 || input.TargetDataCenterID == uuid.Nil {
		return model.Room{}, fmt.Errorf("%w: version 和目标数据中心不能为空", ErrInvalidResource)
	}
	var moved model.Room
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetRoom(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if current.Version != input.Version {
			return ErrResourceVersion
		}
		if current.Status == model.ResourceStatusArchived {
			return fmt.Errorf("%w: 归档的机房不能移动", ErrInvalidResource)
		}
		if current.DataCenterID == input.TargetDataCenterID {
			return fmt.Errorf("%w: 机房已经位于目标数据中心", ErrInvalidResource)
		}
		target, err := store.GetDataCenter(ctx, input.TargetDataCenterID)
		if err != nil {
			return mapStoreError(err)
		}
		if isDisabled(target.Status) {
			return fmt.Errorf("%w: 停用或归档的数据中心不能接收移动资源", ErrParentDisabled)
		}
		moved = current
		moved.DataCenterID = target.ID
		moved.Code = moveCode(input, current.Code, "ROOM")
		normalizeRoom(&moved)
		if err := validateRoom(moved); err != nil {
			return err
		}
		if err := store.MoveRoomHierarchy(ctx, &moved, input.Version); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "MOVE", "room", id, current, moved)
	})
	return moved, err
}

func (s *ResourceService) MoveRack(ctx context.Context, actor Actor, id uuid.UUID, input ResourceMoveInput) (model.Rack, error) {
	if input.Version == 0 || input.TargetRoomID == uuid.Nil {
		return model.Rack{}, fmt.Errorf("%w: version 和目标机房不能为空", ErrInvalidResource)
	}
	var moved model.Rack
	err := s.store.WithinTransaction(ctx, func(store repository.ResourceStore) error {
		current, err := store.GetRack(ctx, id)
		if err != nil {
			return mapStoreError(err)
		}
		if current.Version != input.Version {
			return ErrResourceVersion
		}
		if current.RoomID == input.TargetRoomID {
			return fmt.Errorf("%w: 机柜已经位于目标机房", ErrInvalidResource)
		}
		target, err := store.GetRoom(ctx, input.TargetRoomID)
		if err != nil {
			return mapStoreError(err)
		}
		if isDisabled(target.Status) {
			return fmt.Errorf("%w: 停用或归档的机房不能接收移动资源", ErrParentDisabled)
		}
		moved = current
		moved.RoomID, moved.DataCenterID = target.ID, target.DataCenterID
		moved.Code = moveCode(input, current.Code, "RACK")
		moved.XCoordinate, moved.YCoordinate = nil, nil
		moved.SortOrder = 0
		normalizeRack(&moved)
		if err := validateRack(moved); err != nil {
			return err
		}
		if err := store.MoveRackHierarchy(ctx, &moved, input.Version); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "MOVE", "rack", id, current, moved)
	})
	return moved, err
}

func cloneDataCenter(source model.DataCenter) model.DataCenter {
	source.BaseModel = model.BaseModel{}
	source.Rooms = nil
	source.Status = model.ResourceStatusPlanning
	source.SortOrder = 0
	return source
}
func cloneRoom(source model.Room) model.Room {
	source.BaseModel = model.BaseModel{}
	source.Racks = nil
	source.Status = model.ResourceStatusPlanning
	source.SortOrder = 0
	return source
}
func cloneRack(source model.Rack) model.Rack {
	source.BaseModel = model.BaseModel{}
	source.Status = model.RackStatusPlanning
	source.SerialNumber = ""
	source.AssetNumber = ""
	source.XCoordinate, source.YCoordinate = nil, nil
	source.SortOrder = 0
	return source
}
func copyCode(input ResourceCopyInput, prefix string) string {
	if input.AutoGenerateCode {
		return generateAutoCode(prefix)
	}
	return strings.TrimSpace(input.Code)
}
func moveCode(input ResourceMoveInput, current, prefix string) string {
	if input.AutoGenerateCode {
		return generateAutoCode(prefix)
	}
	if strings.TrimSpace(input.Code) == "" {
		return current
	}
	return strings.TrimSpace(input.Code)
}
func copyName(name, source string) string {
	if strings.TrimSpace(name) != "" {
		return strings.TrimSpace(name)
	}
	return source + "-副本"
}
func findDataCenter(tree []model.DataCenter, id uuid.UUID) (model.DataCenter, bool) {
	for _, item := range tree {
		if item.ID == id {
			return item, true
		}
	}
	return model.DataCenter{}, false
}
func findRoom(tree []model.DataCenter, id uuid.UUID) (model.Room, bool) {
	for _, dc := range tree {
		for _, room := range dc.Rooms {
			if room.ID == id {
				return room, true
			}
		}
	}
	return model.Room{}, false
}
