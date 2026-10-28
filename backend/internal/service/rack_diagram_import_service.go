package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RackDiagramImportUnchanged     = "UNCHANGED"
	RackDiagramImportCreateNew     = "CREATE_NEW"
	RackDiagramImportMoveExisting  = "MOVE_EXISTING"
	RackDiagramImportNeedsDecision = "NEEDS_DECISION"
	RackDiagramImportRemoveMissing = "REMOVE_MISSING"
	RackDiagramImportError         = "ERROR"

	RackDiagramDecisionUpdateExisting = "UPDATE_EXISTING"
	RackDiagramDecisionCreateNew      = "CREATE_NEW"
	RackDiagramDecisionDecommission   = "DECOMMISSION"
	RackDiagramDecisionIgnore         = "IGNORE"
)

var (
	ErrRackDiagramImportDraft    = errors.New("rack diagram import draft not found")
	ErrRackDiagramImportStale    = errors.New("rack diagram import data is stale")
	ErrRackDiagramImportDecision = errors.New("rack diagram import decision is incomplete")
)

type RackDiagramImportDeviceInput struct {
	ClientID            string     `json:"clientId"`
	RackID              uuid.UUID  `json:"rackId"`
	RackCode            string     `json:"rackCode"`
	StartU              int        `json:"startU"`
	EndU                int        `json:"endU"`
	Name                string     `json:"name"`
	SerialNumber        string     `json:"serialNumber"`
	RatedPowerW         *float64   `json:"ratedPowerW"`
	TypeID              *uuid.UUID `json:"typeId"`
	TypeCategory        string     `json:"typeCategory"`
	SourceDeviceID      *uuid.UUID `json:"sourceDeviceId"`
	SourceDeviceVersion uint       `json:"sourceDeviceVersion"`
	SourceDeviceCode    string     `json:"sourceDeviceCode"`
	SourceRackID        *uuid.UUID `json:"sourceRackId"`
	SourceStartU        int        `json:"sourceStartU"`
	SourceEndU          int        `json:"sourceEndU"`
}

type RackDiagramImportValidateInput struct {
	FormatVersion  string                         `json:"formatVersion"`
	DataCenterID   uuid.UUID                      `json:"dataCenterId"`
	RoomID         uuid.UUID                      `json:"roomId"`
	ExportedAt     string                         `json:"exportedAt"`
	DefaultTypeID  uuid.UUID                      `json:"defaultTypeId"`
	CoveredRackIDs []uuid.UUID                    `json:"coveredRackIds"`
	Devices        []RackDiagramImportDeviceInput `json:"devices"`
}

type RackDiagramImportDecisionInput struct {
	ItemID string `json:"itemId"`
	Action string `json:"action"`
}

type RackDiagramImportCommitInput struct {
	RoomID    uuid.UUID                        `json:"-"`
	Token     string                           `json:"token"`
	Decisions []RackDiagramImportDecisionInput `json:"decisions"`
}

type RackDiagramImportItem struct {
	ID                 string     `json:"id"`
	Kind               string     `json:"kind"`
	Severity           string     `json:"severity"`
	Message            string     `json:"message"`
	RackID             *uuid.UUID `json:"rackId,omitempty"`
	RackCode           string     `json:"rackCode,omitempty"`
	StartU             int        `json:"startU,omitempty"`
	EndU               int        `json:"endU,omitempty"`
	Name               string     `json:"name"`
	SerialNumber       string     `json:"serialNumber,omitempty"`
	RatedPowerW        *float64   `json:"ratedPowerW,omitempty"`
	SourceDeviceID     *uuid.UUID `json:"sourceDeviceId,omitempty"`
	ExistingDeviceID   *uuid.UUID `json:"existingDeviceId,omitempty"`
	ExistingDeviceCode string     `json:"existingDeviceCode,omitempty"`
	ExistingDeviceName string     `json:"existingDeviceName,omitempty"`
	RequiresDecision   bool       `json:"requiresDecision"`
	AllowedDecisions   []string   `json:"allowedDecisions,omitempty"`
	DefaultDecision    string     `json:"defaultDecision,omitempty"`
}

type RackDiagramImportSummary struct {
	Total     int `json:"total"`
	Unchanged int `json:"unchanged"`
	Create    int `json:"create"`
	Move      int `json:"move"`
	Decisions int `json:"decisions"`
	Removals  int `json:"removals"`
	Errors    int `json:"errors"`
}

type RackDiagramImportValidation struct {
	Token     string                   `json:"token"`
	ExpiresAt time.Time                `json:"expiresAt"`
	Summary   RackDiagramImportSummary `json:"summary"`
	Items     []RackDiagramImportItem  `json:"items"`
}

type RackDiagramImportCommitResult struct {
	Created        int `json:"created"`
	Updated        int `json:"updated"`
	Moved          int `json:"moved"`
	Decommissioned int `json:"decommissioned"`
	Unchanged      int `json:"unchanged"`
}

type rackDiagramDraft struct {
	Token         string
	RoomID        uuid.UUID
	DataCenterID  uuid.UUID
	DefaultTypeID uuid.UUID
	ActorID       string
	ExpiresAt     time.Time
	Snapshot      string
	Items         []rackDiagramDraftItem
	HasErrors     bool
}

type rackDiagramDraftItem struct {
	Public   RackDiagramImportItem
	Input    RackDiagramImportDeviceInput
	Existing *model.Device
	Position *model.RackDevicePosition
}

type rackDiagramDraftStore struct {
	mu    sync.Mutex
	items map[string]rackDiagramDraft
	now   func() time.Time
}

func newRackDiagramDraftStore() *rackDiagramDraftStore {
	return &rackDiagramDraftStore{items: map[string]rackDiagramDraft{}, now: func() time.Time { return time.Now().UTC() }}
}

func (s *rackDiagramDraftStore) put(item rackDiagramDraft) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for key, draft := range s.items {
		if !draft.ExpiresAt.After(now) {
			delete(s.items, key)
		}
	}
	s.items[item.Token] = item
}

func (s *rackDiagramDraftStore) get(token string) (rackDiagramDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[token]
	if !ok || !item.ExpiresAt.After(s.now()) {
		delete(s.items, token)
		return rackDiagramDraft{}, false
	}
	return item, true
}

func (s *rackDiagramDraftStore) take(token string) (rackDiagramDraft, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[token]
	if !ok || !item.ExpiresAt.After(s.now()) {
		delete(s.items, token)
		return rackDiagramDraft{}, false
	}
	delete(s.items, token)
	return item, true
}

type rackDiagramState struct {
	Room      model.Room
	Racks     []model.Rack
	Positions []model.RackDevicePosition
	ByDevice  map[uuid.UUID]model.RackDevicePosition
	ByRack    map[uuid.UUID][]model.RackDevicePosition
}

func actorKey(actor Actor) string {
	if actor.UserID == nil {
		return ""
	}
	return actor.UserID.String()
}

func (s *DeviceService) ValidateRackDiagramImport(ctx context.Context, actor Actor, input RackDiagramImportValidateInput) (RackDiagramImportValidation, error) {
	store, ok := s.store.(repository.RackDiagramImportStore)
	if !ok {
		return RackDiagramImportValidation{}, errors.New("rack diagram import store is unavailable")
	}
	if input.FormatVersion != "1" || input.RoomID == uuid.Nil || input.DataCenterID == uuid.Nil || input.DefaultTypeID == uuid.Nil {
		return RackDiagramImportValidation{}, fmt.Errorf("%w: 机柜图导入参数不完整", ErrInvalidResource)
	}
	state, err := loadRackDiagramState(ctx, store, input.RoomID)
	if err != nil {
		return RackDiagramImportValidation{}, mapStoreError(err)
	}
	if state.Room.DataCenterID != input.DataCenterID {
		return RackDiagramImportValidation{}, fmt.Errorf("%w: 机柜图不属于当前数据中心", ErrInvalidResource)
	}
	typeItem, err := store.GetDeviceType(ctx, input.DefaultTypeID)
	if err != nil {
		return RackDiagramImportValidation{}, mapStoreError(err)
	}
	if typeItem.Status != model.DeviceTypeStatusActive {
		return RackDiagramImportValidation{}, fmt.Errorf("%w: 新增设备默认类型已停用", ErrParentDisabled)
	}

	rackByID := map[uuid.UUID]model.Rack{}
	for _, rack := range state.Racks {
		rackByID[rack.ID] = rack
	}
	covered := map[uuid.UUID]bool{}
	for _, id := range input.CoveredRackIDs {
		if _, exists := rackByID[id]; !exists {
			return RackDiagramImportValidation{}, fmt.Errorf("%w: 导入文件包含不属于当前机房的机柜", ErrInvalidResource)
		}
		covered[id] = true
	}

	serialSet := map[string]string{}
	for i := range input.Devices {
		normalizeRackDiagramDeviceInput(&input.Devices[i])
		if input.Devices[i].SerialNumber != "" {
			serialSet[strings.ToLower(input.Devices[i].SerialNumber)] = input.Devices[i].SerialNumber
		}
	}
	serials := make([]string, 0, len(serialSet))
	for _, value := range serialSet {
		serials = append(serials, value)
	}
	databaseSerialDevices, err := store.ListDevicesBySerialNumbers(ctx, serials)
	if err != nil {
		return RackDiagramImportValidation{}, err
	}
	bySerial := map[string][]model.Device{}
	for _, device := range databaseSerialDevices {
		bySerial[strings.ToLower(strings.TrimSpace(device.SerialNumber))] = append(bySerial[strings.ToLower(strings.TrimSpace(device.SerialNumber))], device)
	}

	currentByID := map[uuid.UUID]model.Device{}
	currentByExact := map[string]model.RackDevicePosition{}
	for _, position := range state.Positions {
		currentByID[position.DeviceID] = position.Device
		currentByExact[positionKey(position.RackID, position.StartU, position.EndU)] = position
	}

	items := make([]rackDiagramDraftItem, 0, len(input.Devices)+len(state.Positions))
	matched := map[uuid.UUID]bool{}
	occupied := map[uuid.UUID]map[int]string{}
	importSerialOwner := map[string]string{}
	for index, deviceInput := range input.Devices {
		itemID := strings.TrimSpace(deviceInput.ClientID)
		if itemID == "" {
			itemID = fmt.Sprintf("row-%d", index+1)
		}
		public := RackDiagramImportItem{ID: itemID, RackID: &deviceInput.RackID, RackCode: deviceInput.RackCode, StartU: deviceInput.StartU, EndU: deviceInput.EndU, Name: deviceInput.Name, SerialNumber: deviceInput.SerialNumber, RatedPowerW: deviceInput.RatedPowerW, SourceDeviceID: deviceInput.SourceDeviceID}
		draftItem := rackDiagramDraftItem{Public: public, Input: deviceInput}
		rack, rackExists := rackByID[deviceInput.RackID]
		if !rackExists || !covered[deviceInput.RackID] {
			setImportError(&draftItem, "目标机柜不存在或未包含在导入范围内")
		} else if deviceInput.Name == "" || len(deviceInput.Name) > 150 {
			setImportError(&draftItem, "设备名称不能为空且不能超过 150 个字符")
		} else if deviceInput.StartU < 1 || deviceInput.EndU < deviceInput.StartU || deviceInput.EndU > rack.UHeight {
			setImportError(&draftItem, fmt.Sprintf("U 位范围 %d-%d 超出机柜 %dU", deviceInput.StartU, deviceInput.EndU, rack.UHeight))
		} else if len(deviceInput.SerialNumber) > 120 {
			setImportError(&draftItem, "设备序列号不能超过 120 个字符")
		} else if deviceInput.RatedPowerW != nil && *deviceInput.RatedPowerW < 0 {
			setImportError(&draftItem, "设备功耗不能为负数")
		} else {
			if occupied[deviceInput.RackID] == nil {
				occupied[deviceInput.RackID] = map[int]string{}
			}
			for u := deviceInput.StartU; u <= deviceInput.EndU; u++ {
				if previous := occupied[deviceInput.RackID][u]; previous != "" {
					setImportError(&draftItem, fmt.Sprintf("与导入项 %s 在 %dU 重叠", previous, u))
					break
				}
				occupied[deviceInput.RackID][u] = itemID
			}
		}
		if draftItem.Public.Kind == RackDiagramImportError {
			items = append(items, draftItem)
			continue
		}

		if deviceInput.SerialNumber != "" {
			serialKey := strings.ToLower(deviceInput.SerialNumber)
			if owner := importSerialOwner[serialKey]; owner != "" {
				setImportError(&draftItem, fmt.Sprintf("序列号与导入项 %s 重复", owner))
				items = append(items, draftItem)
				continue
			}
			importSerialOwner[serialKey] = itemID
		}

		var existing *model.Device
		var position *model.RackDevicePosition
		if deviceInput.SourceDeviceID != nil {
			if current, exists := currentByID[*deviceInput.SourceDeviceID]; exists {
				currentCopy := current
				existing = &currentCopy
				posCopy := state.ByDevice[current.ID]
				position = &posCopy
			} else {
				setImportError(&draftItem, "导出时的原设备已不存在或已不在当前机房")
			}
			if existing != nil && deviceInput.SourceDeviceVersion > 0 && existing.Version != deviceInput.SourceDeviceVersion {
				setImportError(&draftItem, "原设备在导出后已被修改，请重新导出后再导入")
			}
			if existing != nil && deviceInput.SourceDeviceCode != "" && !strings.EqualFold(existing.Code, deviceInput.SourceDeviceCode) {
				setImportError(&draftItem, "设备编号在导出后已变化，无法安全判断设备身份，请重新导出")
			}
			if position != nil && deviceInput.SourceRackID != nil && (position.RackID != *deviceInput.SourceRackID || position.StartU != deviceInput.SourceStartU || position.EndU != deviceInput.SourceEndU) {
				setImportError(&draftItem, "原设备位置在导出后已变化，请重新导出后再导入")
			}
		} else if exact, exists := currentByExact[positionKey(deviceInput.RackID, deviceInput.StartU, deviceInput.EndU)]; exists {
			copyDevice, copyPosition := exact.Device, exact
			existing, position = &copyDevice, &copyPosition
		} else if deviceInput.SerialNumber != "" && len(bySerial[strings.ToLower(deviceInput.SerialNumber)]) == 1 {
			copyDevice := bySerial[strings.ToLower(deviceInput.SerialNumber)][0]
			existing = &copyDevice
			if currentPosition, exists := state.ByDevice[copyDevice.ID]; exists {
				copyPosition := currentPosition
				position = &copyPosition
			}
		}

		if draftItem.Public.Kind == RackDiagramImportError {
			items = append(items, draftItem)
			continue
		}
		if deviceInput.SerialNumber != "" {
			matches := bySerial[strings.ToLower(deviceInput.SerialNumber)]
			if len(matches) > 1 {
				setImportError(&draftItem, "数据库中存在多个相同序列号，必须先清理重复数据")
				items = append(items, draftItem)
				continue
			}
			if len(matches) == 1 && existing != nil && matches[0].ID != existing.ID {
				setImportError(&draftItem, fmt.Sprintf("序列号已属于设备 %s（%s）", matches[0].Name, matches[0].Code))
				items = append(items, draftItem)
				continue
			}
		}

		if existing != nil && matched[existing.ID] {
			setImportError(&draftItem, fmt.Sprintf("原设备 %s（%s）在导入文件中被匹配到多个位置", existing.Name, existing.Code))
			items = append(items, draftItem)
			continue
		}

		draftItem.Existing, draftItem.Position = existing, position
		if existing == nil {
			setImportKind(&draftItem, RackDiagramImportCreateNew, "info", "将创建新设备，设备编号由系统自动生成")
		} else {
			matched[existing.ID] = true
			draftItem.Public.ExistingDeviceID = &existing.ID
			draftItem.Public.ExistingDeviceCode = existing.Code
			draftItem.Public.ExistingDeviceName = existing.Name
			sameLocation := position != nil && position.RackID == deviceInput.RackID && position.StartU == deviceInput.StartU && position.EndU == deviceInput.EndU
			sameData := sameRackDiagramDeviceData(*existing, deviceInput)
			switch {
			case sameLocation && sameData:
				setImportKind(&draftItem, RackDiagramImportUnchanged, "info", "原设备位置和设备信息未变化，将保留原设备编号")
			case sameData && position != nil:
				setImportKind(&draftItem, RackDiagramImportMoveExisting, "warning", fmt.Sprintf("将保留设备编号 %s 并迁移位置", existing.Code))
			default:
				allowed := []string{RackDiagramDecisionUpdateExisting, RackDiagramDecisionCreateNew}
				if deviceInput.SerialNumber != "" && strings.EqualFold(deviceInput.SerialNumber, existing.SerialNumber) {
					allowed = []string{RackDiagramDecisionUpdateExisting}
				}
				setImportDecision(&draftItem, fmt.Sprintf("当前位置或序列号匹配到原设备 %s（%s），请确认是修改原设备还是创建新设备", existing.Name, existing.Code), allowed)
			}
		}
		items = append(items, draftItem)
	}

	for _, position := range state.Positions {
		if !covered[position.RackID] || matched[position.DeviceID] {
			continue
		}
		deviceID, rackID := position.DeviceID, position.RackID
		public := RackDiagramImportItem{ID: "remove:" + deviceID.String(), Kind: RackDiagramImportRemoveMissing, Severity: "warning", Message: "导入机柜图中已删除该设备，请确认是否下架；选择忽略将保留当前位置", RackID: &rackID, RackCode: rackByID[position.RackID].Code, StartU: position.StartU, EndU: position.EndU, Name: position.Device.Name, SerialNumber: position.Device.SerialNumber, RatedPowerW: position.Device.RatedPowerW, ExistingDeviceID: &deviceID, ExistingDeviceCode: position.Device.Code, ExistingDeviceName: position.Device.Name, RequiresDecision: true, AllowedDecisions: []string{RackDiagramDecisionDecommission, RackDiagramDecisionIgnore}, DefaultDecision: RackDiagramDecisionIgnore}
		deviceCopy, positionCopy := position.Device, position
		items = append(items, rackDiagramDraftItem{Public: public, Existing: &deviceCopy, Position: &positionCopy})
	}

	publicItems := make([]RackDiagramImportItem, 0, len(items))
	summary := RackDiagramImportSummary{Total: len(items)}
	hasErrors := false
	for _, item := range items {
		publicItems = append(publicItems, item.Public)
		switch item.Public.Kind {
		case RackDiagramImportUnchanged:
			summary.Unchanged++
		case RackDiagramImportCreateNew:
			summary.Create++
		case RackDiagramImportMoveExisting:
			summary.Move++
		case RackDiagramImportNeedsDecision:
			summary.Decisions++
		case RackDiagramImportRemoveMissing:
			summary.Removals++
		case RackDiagramImportError:
			summary.Errors++
			hasErrors = true
		}
	}
	token := uuid.NewString()
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	draft := rackDiagramDraft{Token: token, RoomID: input.RoomID, DataCenterID: input.DataCenterID, DefaultTypeID: input.DefaultTypeID, ActorID: actorKey(actor), ExpiresAt: expiresAt, Snapshot: rackDiagramStateSnapshot(state), Items: items, HasErrors: hasErrors}
	s.drafts.put(draft)
	return RackDiagramImportValidation{Token: token, ExpiresAt: expiresAt, Summary: summary, Items: publicItems}, nil
}

func (s *DeviceService) CommitRackDiagramImport(ctx context.Context, actor Actor, input RackDiagramImportCommitInput) (RackDiagramImportCommitResult, error) {
	draft, ok := s.drafts.get(strings.TrimSpace(input.Token))
	if !ok || draft.ActorID != actorKey(actor) || input.RoomID == uuid.Nil || draft.RoomID != input.RoomID {
		return RackDiagramImportCommitResult{}, ErrRackDiagramImportDraft
	}
	if draft.HasErrors {
		return RackDiagramImportCommitResult{}, fmt.Errorf("%w: 校验结果仍包含错误", ErrRackDiagramImportDecision)
	}
	decisions := map[string]string{}
	for _, decision := range input.Decisions {
		decisions[decision.ItemID] = strings.ToUpper(strings.TrimSpace(decision.Action))
	}
	for _, item := range draft.Items {
		if item.Public.RequiresDecision && !containsString(item.Public.AllowedDecisions, decisions[item.Public.ID]) {
			return RackDiagramImportCommitResult{}, fmt.Errorf("%w: 导入项 %s 尚未完成确认", ErrRackDiagramImportDecision, item.Public.ID)
		}
	}
	reservedDraft, reservedOK := s.drafts.take(draft.Token)
	if !reservedOK || reservedDraft.ActorID != actorKey(actor) || reservedDraft.RoomID != input.RoomID {
		return RackDiagramImportCommitResult{}, ErrRackDiagramImportDraft
	}
	draft = reservedDraft

	var result RackDiagramImportCommitResult
	err := s.store.WithinDeviceTransaction(ctx, func(baseStore repository.DeviceStore) error {
		store, ok := baseStore.(repository.RackDiagramImportStore)
		if !ok {
			return errors.New("rack diagram import store is unavailable")
		}
		state, err := loadRackDiagramState(ctx, store, draft.RoomID)
		if err != nil {
			return mapStoreError(err)
		}
		if rackDiagramStateSnapshot(state) != draft.Snapshot {
			return ErrRackDiagramImportStale
		}
		rackByID := map[uuid.UUID]model.Rack{}
		for _, rack := range state.Racks {
			rackByID[rack.ID] = rack
		}

		typeCache := map[uuid.UUID]model.DeviceType{}
		loadType := func(id uuid.UUID) (model.DeviceType, error) {
			if item, exists := typeCache[id]; exists {
				return item, nil
			}
			item, err := store.GetDeviceType(ctx, id)
			if err != nil {
				return model.DeviceType{}, mapStoreError(err)
			}
			if item.Status != model.DeviceTypeStatusActive {
				return model.DeviceType{}, fmt.Errorf("%w: 新增设备类型已停用", ErrParentDisabled)
			}
			typeCache[id] = item
			return item, nil
		}

		type targetOperation struct {
			item           rackDiagramDraftItem
			existingID     *uuid.UUID
			createNew      bool
			updateExisting bool
		}
		targets := []targetOperation{}
		removeOnly := map[uuid.UUID]rackDiagramDraftItem{}
		positionChanged := map[uuid.UUID]bool{}
		for _, item := range draft.Items {
			switch item.Public.Kind {
			case RackDiagramImportUnchanged:
				result.Unchanged++
			case RackDiagramImportCreateNew:
				targets = append(targets, targetOperation{item: item, createNew: true})
			case RackDiagramImportMoveExisting:
				targets = append(targets, targetOperation{item: item, existingID: item.Public.ExistingDeviceID, updateExisting: true})
				if item.Public.ExistingDeviceID != nil {
					positionChanged[*item.Public.ExistingDeviceID] = true
				}
			case RackDiagramImportNeedsDecision:
				if decisions[item.Public.ID] == RackDiagramDecisionCreateNew {
					if item.Public.ExistingDeviceID != nil {
						removeOnly[*item.Public.ExistingDeviceID] = item
					}
					targets = append(targets, targetOperation{item: item, createNew: true})
				} else {
					targets = append(targets, targetOperation{item: item, existingID: item.Public.ExistingDeviceID, updateExisting: true})
					if item.Public.ExistingDeviceID != nil && (item.Position == nil || item.Position.RackID != item.Input.RackID || item.Position.StartU != item.Input.StartU || item.Position.EndU != item.Input.EndU) {
						positionChanged[*item.Public.ExistingDeviceID] = true
					}
				}
			case RackDiagramImportRemoveMissing:
				if decisions[item.Public.ID] == RackDiagramDecisionDecommission && item.Public.ExistingDeviceID != nil {
					removeOnly[*item.Public.ExistingDeviceID] = item
				}
			}
		}

		if len(removeOnly) > 0 {
			policy, err := baseStore.GetOperationPolicy(ctx)
			if err != nil {
				return mapStoreError(err)
			}
			if policy.DecommissionApprovalEnabled {
				return fmt.Errorf("%w: 下架审批已启用，请通过设备台账提交下架申请；机柜图导入不能直接下架或替换移除原设备", ErrInvalidResource)
			}
		}

		finalRanges := map[uuid.UUID]map[int]string{}
		finalCounts := map[uuid.UUID]int{}
		for _, position := range state.Positions {
			if _, removing := removeOnly[position.DeviceID]; removing || positionChanged[position.DeviceID] {
				continue
			}
			if finalRanges[position.RackID] == nil {
				finalRanges[position.RackID] = map[int]string{}
			}
			for u := position.StartU; u <= position.EndU; u++ {
				finalRanges[position.RackID][u] = position.DeviceID.String()
			}
			finalCounts[position.RackID]++
		}
		for _, target := range targets {
			if !target.createNew && target.existingID != nil && !positionChanged[*target.existingID] {
				continue
			}
			rack, exists := rackByID[target.item.Input.RackID]
			if !exists {
				return fmt.Errorf("%w: 目标机柜不存在", ErrResourceNotFound)
			}
			positionInput := PositionChangeInput{RackID: rack.ID, StartU: target.item.Input.StartU, HeightU: target.item.Input.EndU - target.item.Input.StartU + 1, Orientation: model.DeviceOrientationNormal}
			if err := validateTargetRack(rack, positionInput); err != nil {
				return err
			}
			if finalRanges[rack.ID] == nil {
				finalRanges[rack.ID] = map[int]string{}
			}
			for u := target.item.Input.StartU; u <= target.item.Input.EndU; u++ {
				if owner := finalRanges[rack.ID][u]; owner != "" {
					return &UConflictError{RackID: rack.ID, StartU: target.item.Input.StartU, HeightU: positionInput.HeightU}
				}
				finalRanges[rack.ID][u] = target.item.Public.ID
			}
			finalCounts[rack.ID]++
			if finalCounts[rack.ID] > MaxDevicesPerRack {
				return fmt.Errorf("%w: 每个机柜最多允许 %d 台设备", ErrResourceLimit, MaxDevicesPerRack)
			}
		}

		newCount := 0
		for _, target := range targets {
			if target.createNew {
				newCount++
			}
		}
		deviceCount, err := store.CountDevices(ctx)
		if err != nil {
			return err
		}
		if deviceCount+int64(newCount) > MaxDevices {
			return fmt.Errorf("%w: 设备总数最多允许 %d 台", ErrResourceLimit, MaxDevices)
		}

		touchedRacks := map[uuid.UUID]bool{}
		removeIDs := make([]string, 0, len(removeOnly)+len(positionChanged))
		for id := range removeOnly {
			removeIDs = append(removeIDs, id.String())
		}
		for id := range positionChanged {
			if _, exists := removeOnly[id]; !exists {
				removeIDs = append(removeIDs, id.String())
			}
		}
		sort.Strings(removeIDs)
		lockedDevices := map[uuid.UUID]model.Device{}
		for _, rawID := range removeIDs {
			id := uuid.MustParse(rawID)
			device, err := store.GetDeviceForUpdate(ctx, id)
			if err != nil {
				return mapStoreError(err)
			}
			lockedDevices[id] = device
			position, exists := state.ByDevice[id]
			if !exists {
				return ErrRackDiagramImportStale
			}
			if err := store.DeleteOccupanciesByPosition(ctx, position.ID); err != nil {
				return err
			}
			if err := store.DeletePosition(ctx, position.ID); err != nil {
				return err
			}
			touchedRacks[position.RackID] = true
		}

		for id, item := range removeOnly {
			device := lockedDevices[id]
			if err := store.UpdateDeviceLifecycle(ctx, id, device.Version, model.DeviceStatusOffRack); err != nil {
				return mapStoreError(err)
			}
			device.Version++
			position := state.ByDevice[id]
			before := positionSnapshot(position)
			history := model.DevicePositionHistory{DeviceID: id, Operation: model.PositionOperationDecommission, FromPosition: &before, Reason: "机柜图导入确认下架", ActorID: actor.UserID, RequestID: actor.RequestID}
			if err := store.CreatePositionHistory(ctx, &history); err != nil {
				return err
			}
			if err := createAudit(ctx, store, actor, "IMPORT_DECOMMISSION", "device", id, item.Existing, device); err != nil {
				return err
			}
			result.Decommissioned++
		}

		for _, target := range targets {
			inputItem := target.item.Input
			height := inputItem.EndU - inputItem.StartU + 1
			var device model.Device
			var beforePosition *model.DevicePositionSnapshot
			if target.createNew {
				typeID := draft.DefaultTypeID
				if inputItem.TypeID != nil {
					typeID = *inputItem.TypeID
				}
				typeItem, err := loadType(typeID)
				if err != nil {
					return err
				}
				device = model.Device{TypeID: typeID, Code: generateAutoCode("DEV"), Name: inputItem.Name, SerialNumber: inputItem.SerialNumber, RatedPowerW: inputItem.RatedPowerW, HeightU: height, LifecycleStatus: model.DeviceStatusRunning}
				applyDeviceTypeDefaults(&device, typeItem)
				device.HeightU = height
				normalizeDevice(&device)
				if err := validateDevice(device); err != nil {
					return err
				}
				if err := store.CreateDevice(ctx, &device); err != nil {
					return mapStoreError(err)
				}
				if err := createAudit(ctx, store, actor, "IMPORT_CREATE", "device", device.ID, nil, device); err != nil {
					return err
				}
				result.Created++
			} else {
				if target.existingID == nil {
					return ErrRackDiagramImportStale
				}
				current, exists := lockedDevices[*target.existingID]
				if !exists {
					var err error
					current, err = store.GetDeviceForUpdate(ctx, *target.existingID)
					if err != nil {
						return mapStoreError(err)
					}
				}
				beforeDevice := current
				device = current
				device.Name, device.SerialNumber, device.RatedPowerW, device.HeightU, device.LifecycleStatus = inputItem.Name, inputItem.SerialNumber, inputItem.RatedPowerW, height, model.DeviceStatusRunning
				normalizeDevice(&device)
				if err := validateDevice(device); err != nil {
					return err
				}
				if err := store.UpdateDevice(ctx, &device, current.Version); err != nil {
					return mapStoreError(err)
				}
				if err := createAudit(ctx, store, actor, "IMPORT_UPDATE", "device", device.ID, beforeDevice, device); err != nil {
					return err
				}
				result.Updated++
				if oldPosition, exists := state.ByDevice[device.ID]; exists {
					snapshot := positionSnapshot(oldPosition)
					beforePosition = &snapshot
				}
			}

			needsPosition := target.createNew || (target.existingID != nil && positionChanged[*target.existingID])
			if needsPosition {
				now := time.Now().UTC()
				position := model.RackDevicePosition{DeviceID: device.ID, RackID: inputItem.RackID, RoomID: draft.RoomID, DataCenterID: draft.DataCenterID, StartU: inputItem.StartU, HeightU: height, EndU: inputItem.EndU, Orientation: model.DeviceOrientationNormal, InstalledAt: now, InstalledBy: actor.UserID, Reason: "机柜图导入"}
				if err := store.CreatePosition(ctx, &position); err != nil {
					return mapStoreError(err)
				}
				if err := store.CreateOccupancies(ctx, occupancyRows(position)); err != nil {
					return mapStoreError(err)
				}
				after := positionSnapshot(position)
				operation := model.PositionOperationAssign
				if beforePosition != nil {
					operation = model.PositionOperationMove
					result.Moved++
				}
				history := model.DevicePositionHistory{DeviceID: device.ID, Operation: operation, FromPosition: beforePosition, ToPosition: &after, Reason: "机柜图导入", ActorID: actor.UserID, RequestID: actor.RequestID}
				if err := store.CreatePositionHistory(ctx, &history); err != nil {
					return err
				}
				touchedRacks[inputItem.RackID] = true
			}
		}
		for rackID := range touchedRacks {
			if err := refreshRackStatus(ctx, store, rackID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return RackDiagramImportCommitResult{}, err
	}
	return result, nil
}

func loadRackDiagramState(ctx context.Context, store repository.RackDiagramImportStore, roomID uuid.UUID) (rackDiagramState, error) {
	room, err := store.GetRoom(ctx, roomID)
	if err != nil {
		return rackDiagramState{}, err
	}
	racks, err := store.ListRacksByRoom(ctx, roomID)
	if err != nil {
		return rackDiagramState{}, err
	}
	state := rackDiagramState{Room: room, Racks: racks, ByDevice: map[uuid.UUID]model.RackDevicePosition{}, ByRack: map[uuid.UUID][]model.RackDevicePosition{}}
	for _, rack := range racks {
		positions, err := store.ListPositionsByRack(ctx, rack.ID)
		if err != nil {
			return rackDiagramState{}, err
		}
		state.Positions = append(state.Positions, positions...)
		state.ByRack[rack.ID] = positions
		for _, position := range positions {
			state.ByDevice[position.DeviceID] = position
		}
	}
	return state, nil
}

func rackDiagramStateSnapshot(state rackDiagramState) string {
	parts := make([]string, 0, len(state.Racks)+len(state.Positions))
	for _, rack := range state.Racks {
		parts = append(parts, fmt.Sprintf("r:%s:%d:%s:%d", rack.ID, rack.Version, rack.Status, rack.UHeight))
	}
	for _, position := range state.Positions {
		parts = append(parts, fmt.Sprintf("p:%s:%d:%s:%d:%d:%s:%d", position.ID, position.Version, position.DeviceID, position.StartU, position.EndU, position.Device.Code, position.Device.Version))
	}
	sort.Strings(parts)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(parts, "|"))))
}

func normalizeRackDiagramDeviceInput(item *RackDiagramImportDeviceInput) {
	item.ClientID = strings.TrimSpace(item.ClientID)
	item.RackCode = strings.TrimSpace(item.RackCode)
	item.Name = strings.TrimSpace(item.Name)
	item.SerialNumber = strings.TrimSpace(item.SerialNumber)
	item.SourceDeviceCode = strings.TrimSpace(item.SourceDeviceCode)
	item.TypeCategory = strings.ToUpper(strings.TrimSpace(item.TypeCategory))
}

func sameRackDiagramDeviceData(device model.Device, input RackDiagramImportDeviceInput) bool {
	if device.Name != input.Name || !strings.EqualFold(strings.TrimSpace(device.SerialNumber), strings.TrimSpace(input.SerialNumber)) {
		return false
	}
	if device.RatedPowerW == nil && input.RatedPowerW == nil {
		return true
	}
	if device.RatedPowerW == nil || input.RatedPowerW == nil {
		return false
	}
	delta := *device.RatedPowerW - *input.RatedPowerW
	if delta < 0 {
		delta = -delta
	}
	return delta < 0.001
}

func positionKey(rackID uuid.UUID, startU, endU int) string {
	return fmt.Sprintf("%s:%d:%d", rackID, startU, endU)
}

func setImportKind(item *rackDiagramDraftItem, kind, severity, message string) {
	item.Public.Kind, item.Public.Severity, item.Public.Message = kind, severity, message
}

func setImportError(item *rackDiagramDraftItem, message string) {
	setImportKind(item, RackDiagramImportError, "error", message)
	item.Public.RequiresDecision = false
}

func setImportDecision(item *rackDiagramDraftItem, message string, allowed []string) {
	setImportKind(item, RackDiagramImportNeedsDecision, "warning", message)
	item.Public.RequiresDecision = true
	item.Public.AllowedDecisions = allowed
	if len(allowed) > 0 {
		item.Public.DefaultDecision = allowed[0]
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

var _ = gorm.ErrRecordNotFound
