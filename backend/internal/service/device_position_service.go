package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *DeviceService) Assign(ctx context.Context, actor Actor, deviceID uuid.UUID, input PositionChangeInput) (OperationResult, error) {
	return s.requestPositionChange(ctx, actor, deviceID, model.PositionOperationAssign, input)
}

func (s *DeviceService) Move(ctx context.Context, actor Actor, deviceID uuid.UUID, input PositionChangeInput) (OperationResult, error) {
	return s.requestPositionChange(ctx, actor, deviceID, model.PositionOperationMove, input)
}

func (s *DeviceService) requestPositionChange(ctx context.Context, actor Actor, deviceID uuid.UUID, operation string, input PositionChangeInput) (OperationResult, error) {
	normalizePositionInput(&input)
	if operation == model.PositionOperationDecommission {
		if input.Version == 0 {
			return OperationResult{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
		}
	} else if err := validatePositionInput(input); err != nil {
		return OperationResult{}, err
	}
	policy, err := s.store.GetOperationPolicy(ctx)
	if err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	approvalRequired := operation == model.PositionOperationAssign && policy.AssignApprovalEnabled || operation == model.PositionOperationMove && policy.MoveApprovalEnabled || operation == model.PositionOperationDecommission && policy.DecommissionApprovalEnabled
	if !approvalRequired {
		var result OperationResult
		err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
			var executeErr error
			result, executeErr = executePositionChange(ctx, store, actor, deviceID, operation, input)
			return executeErr
		})
		return result, err
	}
	var approval model.ApprovalRecord
	err = s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		device, err := store.GetDeviceForUpdate(ctx, deviceID)
		if err != nil {
			return mapStoreError(err)
		}
		if device.Version != input.Version {
			return ErrResourceVersion
		}
		current, currentErr := store.GetPositionByDevice(ctx, deviceID)
		if operation == model.PositionOperationAssign && currentErr == nil {
			return fmt.Errorf("%w: 设备已经上架", ErrDevicePositioned)
		}
		if (operation == model.PositionOperationMove || operation == model.PositionOperationDecommission) && errors.Is(currentErr, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: 设备尚未上架", ErrDeviceNotPositioned)
		}
		if currentErr != nil && !errors.Is(currentErr, gorm.ErrRecordNotFound) {
			return currentErr
		}
		if operation == model.PositionOperationDecommission {
			if _, err := store.GetRackForUpdate(ctx, current.RackID); err != nil {
				return mapStoreError(err)
			}
			input.RackID, input.StartU, input.HeightU, input.Orientation = current.RackID, current.StartU, current.HeightU, current.Orientation
		} else if err := previewPositionChange(ctx, store, device, current, operation, input); err != nil {
			return err
		}
		now := time.Now().UTC()
		approval = model.ApprovalRecord{
			DeviceID: deviceID, Operation: operation, Status: model.ApprovalStatusPending,
			RequestedBy: actor.UserID, RequestedAt: now, RequestedDeviceVersion: input.Version,
			TargetRackID: input.RackID, TargetStartU: input.StartU,
			TargetHeightU: input.HeightU, TargetOrientation: input.Orientation, Reason: input.Reason,
		}
		if currentErr == nil {
			snapshot := positionSnapshot(current)
			approval.SourcePosition = &snapshot
		}
		if err := store.CreateApproval(ctx, &approval); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "REQUEST_"+operation, "approval", approval.ID, nil, approval)
	})
	if err != nil {
		return OperationResult{}, err
	}
	return OperationResult{Executed: false, Approval: &approval}, nil
}

func (s *DeviceService) Decommission(ctx context.Context, actor Actor, deviceID uuid.UUID, input DecommissionInput) (OperationResult, error) {
	return s.requestPositionChange(ctx, actor, deviceID, model.PositionOperationDecommission, PositionChangeInput{Version: input.Version, Reason: input.Reason})
}

func executeDecommission(ctx context.Context, store repository.DeviceStore, actor Actor, deviceID uuid.UUID, input PositionChangeInput) (OperationResult, error) {
	device, err := store.GetDeviceForUpdate(ctx, deviceID)
	if err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	if device.Version != input.Version {
		return OperationResult{}, ErrResourceVersion
	}
	position, err := store.GetPositionByDevice(ctx, deviceID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return OperationResult{}, fmt.Errorf("%w: 设备尚未上架", ErrDeviceNotPositioned)
	}
	if err != nil {
		return OperationResult{}, err
	}
	if _, err := store.GetRackForUpdate(ctx, position.RackID); err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	before := positionSnapshot(position)
	if input.expectedSource != nil && before != *input.expectedSource {
		return OperationResult{}, fmt.Errorf("%w: 设备位置已变化，请重新提交下架申请", ErrApprovalState)
	}
	if err := store.DeleteOccupanciesByPosition(ctx, position.ID); err != nil {
		return OperationResult{}, err
	}
	if err := store.DeletePosition(ctx, position.ID); err != nil {
		return OperationResult{}, err
	}
	if err := store.UpdateDeviceLifecycle(ctx, device.ID, device.Version, model.DeviceStatusOffRack); err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	device.Version++
	device.LifecycleStatus = model.DeviceStatusOffRack
	history := model.DevicePositionHistory{DeviceID: device.ID, Operation: model.PositionOperationDecommission, FromPosition: &before, Reason: input.Reason, ActorID: actor.UserID, RequestID: actor.RequestID}
	if err := store.CreatePositionHistory(ctx, &history); err != nil {
		return OperationResult{}, err
	}
	if err := refreshRackStatus(ctx, store, position.RackID); err != nil {
		return OperationResult{}, err
	}
	if err := createAudit(ctx, store, actor, model.PositionOperationDecommission, "device", device.ID, before, nil); err != nil {
		return OperationResult{}, err
	}
	return OperationResult{Executed: true, Device: &device}, nil
}

func (s *DeviceService) GetRackLayout(ctx context.Context, rackID uuid.UUID) (RackULayout, error) {
	rack, err := s.store.GetRackForUpdate(ctx, rackID)
	if err != nil {
		return RackULayout{}, mapStoreError(err)
	}
	positions, err := s.store.ListPositionsByRack(ctx, rackID)
	if err != nil {
		return RackULayout{}, err
	}
	occupancies, err := s.store.ListOccupanciesByRack(ctx, rackID)
	if err != nil {
		return RackULayout{}, err
	}
	return RackULayout{
		Rack: rack, Positions: positions,
		Free: availableRanges(rack.UHeight, occupancies),
	}, nil
}

func (s *DeviceService) ListHistory(ctx context.Context, deviceID uuid.UUID) ([]model.DevicePositionHistory, error) {
	if _, err := s.store.GetDevice(ctx, deviceID); err != nil {
		return nil, mapStoreError(err)
	}
	return s.store.ListPositionHistory(ctx, deviceID)
}

func (s *DeviceService) GetPolicy(ctx context.Context) (model.OperationPolicy, error) {
	item, err := s.store.GetOperationPolicy(ctx)
	if err != nil {
		return model.OperationPolicy{}, mapStoreError(err)
	}
	return item, nil
}

func (s *DeviceService) UpdatePolicy(ctx context.Context, actor Actor, input PolicyUpdateInput) (model.OperationPolicy, error) {
	if input.Version == 0 {
		return model.OperationPolicy{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	input.DefaultApproverRole = strings.TrimSpace(input.DefaultApproverRole)
	if input.DefaultApproverRole == "" {
		input.DefaultApproverRole = "system_admin"
	}
	var updated model.OperationPolicy
	err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetOperationPolicy(ctx)
		if err != nil {
			return mapStoreError(err)
		}
		updated = current
		if input.DecommissionApprovalEnabled != nil {
			updated.DecommissionApprovalEnabled = *input.DecommissionApprovalEnabled
		}
		updated.AssignApprovalEnabled = input.AssignApprovalEnabled
		updated.MoveApprovalEnabled = input.MoveApprovalEnabled
		updated.ApprovalMode = "SINGLE"
		updated.DefaultApproverRole = input.DefaultApproverRole
		if err := store.UpdateOperationPolicy(ctx, &updated, input.Version); err != nil {
			return mapStoreError(err)
		}
		return createAudit(ctx, store, actor, "UPDATE", "operation_policy", updated.ID, current, updated)
	})
	return updated, err
}

func (s *DeviceService) ListApprovals(ctx context.Context, status string) ([]model.ApprovalRecord, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	return s.store.ListApprovals(ctx, status)
}

func (s *DeviceService) Approve(ctx context.Context, actor Actor, approvalID uuid.UUID, input ApprovalDecisionInput) (model.ApprovalRecord, error) {
	if input.Version == 0 {
		return model.ApprovalRecord{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	input.Comment = strings.TrimSpace(input.Comment)
	var approved model.ApprovalRecord
	err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetApprovalForUpdate(ctx, approvalID)
		if err != nil {
			return mapStoreError(err)
		}
		if current.Version != input.Version {
			return ErrResourceVersion
		}
		if current.Status != model.ApprovalStatusPending {
			return fmt.Errorf("%w: 仅待审批记录可批准", ErrApprovalState)
		}
		change := PositionChangeInput{Version: current.RequestedDeviceVersion, RackID: current.TargetRackID, StartU: current.TargetStartU, HeightU: current.TargetHeightU, Orientation: current.TargetOrientation, Reason: current.Reason}
		if current.Operation == model.PositionOperationDecommission {
			if current.SourcePosition == nil {
				return fmt.Errorf("%w: 下架申请缺少原位置快照", ErrApprovalState)
			}
			change.expectedSource = current.SourcePosition
		}
		if _, err := executePositionChange(ctx, store, actor, current.DeviceID, current.Operation, change); err != nil {
			return err
		}
		now := time.Now().UTC()
		current.Status, current.DecidedBy, current.DecidedAt, current.DecisionComment = model.ApprovalStatusApproved, actor.UserID, &now, input.Comment
		if err := store.UpdateApprovalDecision(ctx, &current, input.Version); err != nil {
			return mapStoreError(err)
		}
		approved = current
		return createAudit(ctx, store, actor, "APPROVE", "approval", current.ID, nil, approved)
	})
	return approved, err
}

func (s *DeviceService) Reject(ctx context.Context, actor Actor, approvalID uuid.UUID, input ApprovalDecisionInput) (model.ApprovalRecord, error) {
	if input.Version == 0 {
		return model.ApprovalRecord{}, fmt.Errorf("%w: version 必须大于 0", ErrInvalidResource)
	}
	input.Comment = strings.TrimSpace(input.Comment)
	var rejected model.ApprovalRecord
	err := s.store.WithinDeviceTransaction(ctx, func(store repository.DeviceStore) error {
		current, err := store.GetApprovalForUpdate(ctx, approvalID)
		if err != nil {
			return mapStoreError(err)
		}
		if current.Version != input.Version {
			return ErrResourceVersion
		}
		if current.Status != model.ApprovalStatusPending {
			return fmt.Errorf("%w: 仅待审批记录可驳回", ErrApprovalState)
		}
		now := time.Now().UTC()
		current.Status, current.DecidedBy, current.DecidedAt, current.DecisionComment = model.ApprovalStatusRejected, actor.UserID, &now, input.Comment
		if err := store.UpdateApprovalDecision(ctx, &current, input.Version); err != nil {
			return mapStoreError(err)
		}
		rejected = current
		return createAudit(ctx, store, actor, "REJECT", "approval", current.ID, nil, rejected)
	})
	return rejected, err
}

func executePositionChange(ctx context.Context, store repository.DeviceStore, actor Actor, deviceID uuid.UUID, operation string, input PositionChangeInput) (OperationResult, error) {
	if operation == model.PositionOperationDecommission {
		return executeDecommission(ctx, store, actor, deviceID, input)
	}
	device, err := store.GetDeviceForUpdate(ctx, deviceID)
	if err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	if device.Version != input.Version {
		return OperationResult{}, ErrResourceVersion
	}
	current, currentErr := store.GetPositionByDevice(ctx, deviceID)
	if operation == model.PositionOperationAssign {
		if currentErr == nil {
			return OperationResult{}, fmt.Errorf("%w: 设备已经上架", ErrDevicePositioned)
		}
		if currentErr != nil && !errors.Is(currentErr, gorm.ErrRecordNotFound) {
			return OperationResult{}, currentErr
		}
		if device.LifecycleStatus != model.DeviceStatusWaitingRack && device.LifecycleStatus != model.DeviceStatusOffRack {
			return OperationResult{}, fmt.Errorf("%w: 当前生命周期状态不允许上架", ErrInvalidResource)
		}
	} else if operation == model.PositionOperationMove {
		if errors.Is(currentErr, gorm.ErrRecordNotFound) {
			return OperationResult{}, fmt.Errorf("%w: 设备尚未上架", ErrDeviceNotPositioned)
		}
		if currentErr != nil {
			return OperationResult{}, currentErr
		}
		if device.LifecycleStatus == model.DeviceStatusScrapped {
			return OperationResult{}, fmt.Errorf("%w: 报废设备不能迁移", ErrInvalidResource)
		}
	} else {
		return OperationResult{}, fmt.Errorf("%w: 未知位置操作", ErrInvalidResource)
	}

	rackIDs := []uuid.UUID{input.RackID}
	if currentErr == nil && current.RackID != input.RackID {
		rackIDs = append(rackIDs, current.RackID)
	}
	locked, err := lockRacks(ctx, store, rackIDs)
	if err != nil {
		return OperationResult{}, err
	}
	targetRack := locked[input.RackID]
	if err := validateTargetRack(targetRack, input); err != nil {
		return OperationResult{}, err
	}
	count, err := store.CountPositionsByRack(ctx, input.RackID)
	if err != nil {
		return OperationResult{}, err
	}
	if (currentErr != nil || current.RackID != input.RackID) && count >= MaxDevicesPerRack {
		return OperationResult{}, fmt.Errorf("%w: 每个机柜最多允许 %d 台设备", ErrResourceLimit, MaxDevicesPerRack)
	}
	positions, err := store.ListPositionsByRack(ctx, input.RackID)
	if err != nil {
		return OperationResult{}, err
	}
	conflicts := findConflicts(positions, deviceID, input)
	if len(conflicts) > 0 {
		return OperationResult{}, &UConflictError{RackID: input.RackID, StartU: input.StartU, HeightU: input.HeightU, Conflicts: conflicts}
	}

	var before *model.DevicePositionSnapshot
	if currentErr == nil {
		snapshot := positionSnapshot(current)
		before = &snapshot
		if err := store.DeleteOccupanciesByPosition(ctx, current.ID); err != nil {
			return OperationResult{}, err
		}
		if err := store.DeletePosition(ctx, current.ID); err != nil {
			return OperationResult{}, err
		}
	}
	now := time.Now().UTC()
	position := model.RackDevicePosition{
		DeviceID: device.ID, RackID: targetRack.ID, RoomID: targetRack.RoomID, DataCenterID: targetRack.DataCenterID,
		StartU: input.StartU, HeightU: input.HeightU, EndU: input.StartU + input.HeightU - 1,
		Orientation: input.Orientation, InstalledAt: now, InstalledBy: actor.UserID, Reason: input.Reason,
	}
	if err := store.CreatePosition(ctx, &position); err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	if err := store.CreateOccupancies(ctx, occupancyRows(position)); err != nil {
		if errors.Is(err, repository.ErrUPositionConflict) {
			return OperationResult{}, &UConflictError{RackID: input.RackID, StartU: input.StartU, HeightU: input.HeightU}
		}
		return OperationResult{}, err
	}
	if err := store.UpdateDeviceLifecycle(ctx, device.ID, device.Version, model.DeviceStatusRunning); err != nil {
		return OperationResult{}, mapStoreError(err)
	}
	device.Version++
	device.LifecycleStatus = model.DeviceStatusRunning
	after := positionSnapshot(position)
	history := model.DevicePositionHistory{DeviceID: device.ID, Operation: operation, FromPosition: before, ToPosition: &after, Reason: input.Reason, ActorID: actor.UserID, RequestID: actor.RequestID}
	if err := store.CreatePositionHistory(ctx, &history); err != nil {
		return OperationResult{}, err
	}
	for rackID := range locked {
		if err := refreshRackStatus(ctx, store, rackID); err != nil {
			return OperationResult{}, err
		}
	}
	if err := createAudit(ctx, store, actor, operation, "device", device.ID, before, after); err != nil {
		return OperationResult{}, err
	}
	return OperationResult{Executed: true, Device: &device, Position: &position}, nil
}

func previewPositionChange(ctx context.Context, store repository.DeviceStore, device model.Device, current model.RackDevicePosition, operation string, input PositionChangeInput) error {
	rackIDs := []uuid.UUID{input.RackID}
	if operation == model.PositionOperationMove && current.RackID != input.RackID {
		rackIDs = append(rackIDs, current.RackID)
	}
	locked, err := lockRacks(ctx, store, rackIDs)
	if err != nil {
		return err
	}
	if err := validateTargetRack(locked[input.RackID], input); err != nil {
		return err
	}
	count, err := store.CountPositionsByRack(ctx, input.RackID)
	if err != nil {
		return err
	}
	if (operation == model.PositionOperationAssign || current.RackID != input.RackID) && count >= MaxDevicesPerRack {
		return fmt.Errorf("%w: 每个机柜最多允许 %d 台设备", ErrResourceLimit, MaxDevicesPerRack)
	}
	positions, err := store.ListPositionsByRack(ctx, input.RackID)
	if err != nil {
		return err
	}
	conflicts := findConflicts(positions, device.ID, input)
	if len(conflicts) > 0 {
		return &UConflictError{RackID: input.RackID, StartU: input.StartU, HeightU: input.HeightU, Conflicts: conflicts}
	}
	return nil
}

func lockRacks(ctx context.Context, store repository.DeviceStore, ids []uuid.UUID) (map[uuid.UUID]model.Rack, error) {
	unique := map[string]uuid.UUID{}
	for _, id := range ids {
		unique[id.String()] = id
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	items := make(map[uuid.UUID]model.Rack, len(keys))
	for _, key := range keys {
		item, err := store.GetRackForUpdate(ctx, unique[key])
		if err != nil {
			return nil, mapStoreError(err)
		}
		items[item.ID] = item
	}
	return items, nil
}

func validateTargetRack(rack model.Rack, input PositionChangeInput) error {
	if rack.Status == model.RackStatusDisabled || rack.Status == model.RackStatusMaintenance || rack.Status == model.RackStatusPlanning {
		return fmt.Errorf("%w: 规划、维护或停用机柜不能接收设备", ErrRackUnavailable)
	}
	endU := input.StartU + input.HeightU - 1
	if endU > rack.UHeight {
		return fmt.Errorf("%w: U 位范围 %d-%d 超出机柜 %dU", ErrInvalidResource, input.StartU, endU, rack.UHeight)
	}
	return nil
}

func refreshRackStatus(ctx context.Context, store repository.DeviceStore, rackID uuid.UUID) error {
	rack, err := store.GetRackForUpdate(ctx, rackID)
	if err != nil {
		return mapStoreError(err)
	}
	if rack.Status == model.RackStatusMaintenance || rack.Status == model.RackStatusDisabled || rack.Status == model.RackStatusPlanning {
		return nil
	}
	positions, err := store.CountPositionsByRack(ctx, rackID)
	if err != nil {
		return err
	}
	status := model.RackStatusAvailable
	if positions > 0 {
		occupancies, err := store.ListOccupanciesByRack(ctx, rackID)
		if err != nil {
			return err
		}
		used := make([]bool, rack.UHeight+1)
		for _, item := range occupancies {
			start, end := item.StartU, item.EndU
			if start < 1 {
				start = 1
			}
			if end > rack.UHeight {
				end = rack.UHeight
			}
			for u := start; u <= end; u++ {
				used[u] = true
			}
		}
		status = model.RackStatusPartial
		full := true
		for u := 1; u <= rack.UHeight; u++ {
			if !used[u] {
				full = false
				break
			}
		}
		if full {
			status = model.RackStatusFull
		}
	}
	if status == rack.Status {
		return nil
	}
	return store.UpdateRackOperationalStatus(ctx, rackID, status)
}
