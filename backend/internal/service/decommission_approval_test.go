package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
)

type decommissionStore struct {
	repository.DeviceStore
	policy      model.OperationPolicy
	device      model.Device
	position    model.RackDevicePosition
	rack        model.Rack
	approval    model.ApprovalRecord
	occupied    bool
	history     []model.DevicePositionHistory
	failHistory bool
}

func newDecommissionStore() *decommissionStore {
	id, rackID := uuid.New(), uuid.New()
	return &decommissionStore{policy: model.OperationPolicy{DecommissionApprovalEnabled: true}, device: model.Device{BaseModel: model.BaseModel{ID: id, Version: 3}, LifecycleStatus: model.DeviceStatusRunning}, position: model.RackDevicePosition{BaseModel: model.BaseModel{ID: uuid.New()}, DeviceID: id, RackID: rackID, StartU: 5, HeightU: 2, EndU: 6}, rack: model.Rack{BaseModel: model.BaseModel{ID: rackID}, Status: model.RackStatusPartial}, occupied: true}
}
func (f *decommissionStore) WithinDeviceTransaction(_ context.Context, fn func(repository.DeviceStore) error) error {
	before := *f
	err := fn(f)
	if err != nil {
		*f = before
	}
	return err
}
func (f *decommissionStore) GetOperationPolicy(context.Context) (model.OperationPolicy, error) {
	return f.policy, nil
}
func (f *decommissionStore) GetDeviceForUpdate(context.Context, uuid.UUID) (model.Device, error) {
	return f.device, nil
}
func (f *decommissionStore) CountActivePowerConnections(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}
func (f *decommissionStore) GetPositionByDevice(context.Context, uuid.UUID) (model.RackDevicePosition, error) {
	if f.position.ID == uuid.Nil {
		return model.RackDevicePosition{}, gorm.ErrRecordNotFound
	}
	return f.position, nil
}
func (f *decommissionStore) GetRackForUpdate(context.Context, uuid.UUID) (model.Rack, error) {
	return f.rack, nil
}
func (f *decommissionStore) CreateApproval(_ context.Context, a *model.ApprovalRecord) error {
	a.ID = uuid.New()
	a.Version = 1
	f.approval = *a
	return nil
}
func (f *decommissionStore) CreateAuditLog(context.Context, *model.AuditLog) error { return nil }
func (f *decommissionStore) DeleteOccupanciesByPosition(context.Context, uuid.UUID) error {
	f.occupied = false
	return nil
}
func (f *decommissionStore) DeletePosition(context.Context, uuid.UUID) error {
	f.position = model.RackDevicePosition{}
	return nil
}
func (f *decommissionStore) UpdateDeviceLifecycle(_ context.Context, _ uuid.UUID, _ uint, status string) error {
	f.device.LifecycleStatus = status
	f.device.Version++
	return nil
}
func (f *decommissionStore) CreatePositionHistory(_ context.Context, h *model.DevicePositionHistory) error {
	if f.failHistory {
		return errors.New("history unavailable")
	}
	f.history = append(f.history, *h)
	return nil
}
func (f *decommissionStore) CountPositionsByRack(context.Context, uuid.UUID) (int64, error) {
	if f.occupied {
		return 1, nil
	}
	return 0, nil
}
func (f *decommissionStore) UpdateRackOperationalStatus(_ context.Context, _ uuid.UUID, status string) error {
	f.rack.Status = status
	return nil
}
func (f *decommissionStore) GetApprovalForUpdate(context.Context, uuid.UUID) (model.ApprovalRecord, error) {
	return f.approval, nil
}
func (f *decommissionStore) UpdateApprovalDecision(_ context.Context, a *model.ApprovalRecord, _ uint) error {
	a.Version++
	f.approval = *a
	return nil
}
func (f *decommissionStore) UpdateOperationPolicy(_ context.Context, p *model.OperationPolicy, _ uint) error {
	f.policy = *p
	return nil
}

func requestDecommission(t *testing.T, f *decommissionStore) OperationResult {
	t.Helper()
	out, err := NewDeviceService(f).Decommission(context.Background(), Actor{}, f.device.ID, DecommissionInput{Version: 3, Reason: "  退役更换  "})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestDecommissionApprovalPreservesPositionUntilApproved(t *testing.T) {
	f := newDecommissionStore()
	before := f.position
	out := requestDecommission(t, f)
	if out.Executed || out.Approval == nil || f.approval.Operation != model.PositionOperationDecommission || f.approval.SourcePosition == nil || f.approval.SourcePosition.PositionID != before.ID || f.approval.Reason != "退役更换" || !f.occupied || positionSnapshot(f.position) != positionSnapshot(before) || len(f.history) != 0 {
		t.Fatalf("request mutated occupancy or lost snapshot: %+v", out)
	}
	approved, err := NewDeviceService(f).Approve(context.Background(), Actor{}, f.approval.ID, ApprovalDecisionInput{Version: 1, Comment: "允许下架"})
	if err != nil {
		t.Fatal(err)
	}
	if approved.Status != model.ApprovalStatusApproved || f.occupied || f.position.ID != uuid.Nil || f.device.LifecycleStatus != model.DeviceStatusOffRack || len(f.history) != 1 || f.history[0].Operation != model.PositionOperationDecommission {
		t.Fatalf("approval did not execute decommission: %+v", f)
	}
	_, err = NewDeviceService(f).Approve(context.Background(), Actor{}, f.approval.ID, ApprovalDecisionInput{Version: f.approval.Version})
	if !errors.Is(err, ErrApprovalState) || len(f.history) != 1 {
		t.Fatalf("repeat approval: %v", err)
	}
}
func TestDecommissionWithoutApprovalAndReject(t *testing.T) {
	t.Run("disabled executes", func(t *testing.T) {
		f := newDecommissionStore()
		f.policy.DecommissionApprovalEnabled = false
		out := requestDecommission(t, f)
		if !out.Executed || f.occupied || len(f.history) != 1 {
			t.Fatal("disabled policy did not execute")
		}
	})
	t.Run("reject leaves position", func(t *testing.T) {
		f := newDecommissionStore()
		requestDecommission(t, f)
		_, err := NewDeviceService(f).Reject(context.Background(), Actor{}, f.approval.ID, ApprovalDecisionInput{Version: 1})
		if err != nil || !f.occupied || f.position.ID == uuid.Nil || f.approval.Status != model.ApprovalStatusRejected {
			t.Fatalf("reject changed position: %v", err)
		}
	})
}
func TestDecommissionApprovalRevalidatesAndRollsBack(t *testing.T) {
	for _, kind := range []string{"device version", "source position", "history failure", "missing snapshot"} {
		t.Run(kind, func(t *testing.T) {
			f := newDecommissionStore()
			requestDecommission(t, f)
			switch kind {
			case "device version":
				f.device.Version++
			case "source position":
				f.position.StartU = 8
			case "history failure":
				f.failHistory = true
			case "missing snapshot":
				f.approval.SourcePosition = nil
			}
			_, err := NewDeviceService(f).Approve(context.Background(), Actor{}, f.approval.ID, ApprovalDecisionInput{Version: 1})
			if err == nil || !f.occupied || f.position.ID == uuid.Nil || f.approval.Status != model.ApprovalStatusPending || len(f.history) != 0 {
				t.Fatalf("invalid approval or rollback failed: %v %+v", err, f)
			}
		})
	}
}
func TestDecommissionRejectsInvalidRequest(t *testing.T) {
	for _, kind := range []string{"stale", "missing position", "zero version"} {
		t.Run(kind, func(t *testing.T) {
			f := newDecommissionStore()
			version := uint(3)
			if kind == "stale" {
				version = 2
			}
			if kind == "zero version" {
				version = 0
			}
			if kind == "missing position" {
				f.position = model.RackDevicePosition{}
			}
			_, err := NewDeviceService(f).Decommission(context.Background(), Actor{}, f.device.ID, DecommissionInput{Version: version})
			if err == nil || f.approval.ID != uuid.Nil || !f.occupied {
				t.Fatal("invalid request accepted")
			}
		})
	}
}
func TestPolicyUpdatePreservesOmittedDecommissionSwitch(t *testing.T) {
	f := newDecommissionStore()
	_, err := NewDeviceService(f).UpdatePolicy(context.Background(), Actor{}, PolicyUpdateInput{Version: 1})
	if err != nil || !f.policy.DecommissionApprovalEnabled {
		t.Fatalf("old client disabled new policy: %v", err)
	}
	disabled := false
	_, err = NewDeviceService(f).UpdatePolicy(context.Background(), Actor{}, PolicyUpdateInput{Version: 1, DecommissionApprovalEnabled: &disabled})
	if err != nil || f.policy.DecommissionApprovalEnabled {
		t.Fatalf("explicit false not saved: %v", err)
	}
}

type approvalImportStore struct{ *rackDiagramImportFakeStore }

func (f *approvalImportStore) WithinDeviceTransaction(_ context.Context, fn func(repository.DeviceStore) error) error {
	return fn(f)
}
func (f *approvalImportStore) GetOperationPolicy(context.Context) (model.OperationPolicy, error) {
	return model.OperationPolicy{DecommissionApprovalEnabled: true}, nil
}
func TestRackDiagramCannotBypassDecommissionApproval(t *testing.T) {
	f := &approvalImportStore{newRackDiagramImportFakeStore(true)}
	s := NewDeviceService(f)
	validation, err := s.ValidateRackDiagramImport(context.Background(), Actor{}, rackDiagramValidateInput(f.rackDiagramImportFakeStore, nil))
	if err != nil {
		t.Fatal(err)
	}
	item := validation.Items[0]
	_, err = s.CommitRackDiagramImport(context.Background(), Actor{}, RackDiagramImportCommitInput{RoomID: f.room.ID, Token: validation.Token, Decisions: []RackDiagramImportDecisionInput{{ItemID: item.ID, Action: RackDiagramDecisionDecommission}}})
	if err == nil || !strings.Contains(err.Error(), "下架审批已启用") || len(f.positions[f.racks[0].ID]) != 1 || len(f.occupancies[f.racks[0].ID]) != 1 {
		t.Fatalf("import bypassed approval: %v", err)
	}
}
func TestDecommissionApprovalEmailUsesSourceAndNewInbox(t *testing.T) {
	body := approvalEmailBody(model.ApprovalRecord{Operation: model.PositionOperationDecommission, TargetStartU: 5, TargetHeightU: 2}, "https://it.example.test")
	if !strings.Contains(body, "操作：下架") || !strings.Contains(body, "原 U 位：5–6U") || !strings.Contains(body, "/approvals") || strings.Contains(body, "目标 U 位") {
		t.Fatal(body)
	}
}
