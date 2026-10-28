package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"omnirack-replica/backend/internal/model"
	"omnirack-replica/backend/internal/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRackTemplateStore struct {
	templates      []model.RackTemplate
	current        model.RackTemplate
	createdVersion model.RackTemplateVersion
	auditCount     int
	updateError    error
}

func (f *fakeRackTemplateStore) WithinTemplateTransaction(_ context.Context, fn func(repository.RackTemplateStore) error) error {
	return fn(f)
}
func (f *fakeRackTemplateStore) ListRackTemplates(context.Context) ([]model.RackTemplate, error) {
	return f.templates, nil
}
func (f *fakeRackTemplateStore) GetRackTemplate(context.Context, uuid.UUID) (model.RackTemplate, error) {
	if f.current.ID == uuid.Nil {
		return model.RackTemplate{}, gorm.ErrRecordNotFound
	}
	return f.current, nil
}
func (f *fakeRackTemplateStore) GetRackTemplateVersion(context.Context, uuid.UUID) (model.RackTemplateVersion, error) {
	if f.createdVersion.ID == uuid.Nil {
		return model.RackTemplateVersion{}, gorm.ErrRecordNotFound
	}
	return f.createdVersion, nil
}
func (f *fakeRackTemplateStore) CreateRackTemplate(_ context.Context, item *model.RackTemplate) error {
	item.ID = uuid.New()
	item.Version = 1
	f.current = *item
	return nil
}
func (f *fakeRackTemplateStore) UpdateRackTemplate(context.Context, *model.RackTemplate, uint) error {
	return f.updateError
}
func (f *fakeRackTemplateStore) AdvanceRackTemplateRevision(_ context.Context, _ uuid.UUID, expectedVersion uint, revision int) error {
	if f.updateError != nil {
		return f.updateError
	}
	f.current.CurrentRevision = revision
	f.current.Version = expectedVersion + 1
	return nil
}
func (f *fakeRackTemplateStore) DeleteRackTemplate(context.Context, uuid.UUID, uint) error {
	return f.updateError
}
func (f *fakeRackTemplateStore) CreateRackTemplateVersion(_ context.Context, item *model.RackTemplateVersion) error {
	item.ID = uuid.New()
	item.Version = 1
	f.createdVersion = *item
	return nil
}
func (f *fakeRackTemplateStore) CreateAuditLog(context.Context, *model.AuditLog) error {
	f.auditCount++
	return nil
}

func TestCreateRackTemplateCreatesInitialVersion(t *testing.T) {
	store := &fakeRackTemplateStore{}
	svc := NewRackTemplateService(store)
	created, err := svc.Create(context.Background(), Actor{}, RackTemplateCreateInput{
		Code: " standard-48u ", Name: " 48U 模板 ",
		Spec: model.RackTemplateVersion{UHeight: 48},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.Code != "STANDARD-48U" || created.CurrentRevision != 1 {
		t.Fatalf("unexpected template: %+v", created)
	}
	if store.createdVersion.Revision != 1 || store.createdVersion.UHeight != 48 || store.createdVersion.WidthMm != 600 {
		t.Fatalf("unexpected initial version: %+v", store.createdVersion)
	}
	if store.auditCount != 1 {
		t.Fatalf("expected one audit log, got %d", store.auditCount)
	}
}

func TestCreateRackTemplateVersionIncrementsRevision(t *testing.T) {
	id := uuid.New()
	store := &fakeRackTemplateStore{current: model.RackTemplate{
		BaseModel: model.BaseModel{ID: id, Version: 3}, Code: "T42", Name: "模板",
		Status: model.RackTemplateStatusActive, CurrentRevision: 2,
	}}
	svc := NewRackTemplateService(store)
	created, err := svc.CreateVersion(context.Background(), Actor{}, id, 3, model.RackTemplateVersion{UHeight: 45})
	if err != nil {
		t.Fatalf("CreateVersion returned error: %v", err)
	}
	if created.Revision != 3 || store.current.CurrentRevision != 3 || store.current.Version != 4 {
		t.Fatalf("revision was not advanced: created=%+v current=%+v", created, store.current)
	}
}

func TestDeleteSystemRackTemplateRejected(t *testing.T) {
	id := uuid.New()
	store := &fakeRackTemplateStore{current: model.RackTemplate{
		BaseModel: model.BaseModel{ID: id, Version: 1}, Code: "STANDARD-42U", Name: "标准 42U",
		Status: model.RackTemplateStatusActive, IsSystem: true, CurrentRevision: 1,
	}}
	svc := NewRackTemplateService(store)
	err := svc.Delete(context.Background(), Actor{}, id, 1)
	if !errors.Is(err, ErrSystemTemplate) {
		t.Fatalf("expected ErrSystemTemplate, got %v", err)
	}
}

func TestCreateRackTemplateCanAutoGenerateCode(t *testing.T) {
	store := &fakeRackTemplateStore{}
	svc := NewRackTemplateService(store)
	created, err := svc.Create(context.Background(), Actor{}, RackTemplateCreateInput{
		AutoGenerateCode: true,
		Name:             "自动编号模板",
		Spec:             model.RackTemplateVersion{UHeight: 42},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !strings.HasPrefix(created.Code, "TPL-") || len(created.Code) != 16 {
		t.Fatalf("unexpected generated template code: %q", created.Code)
	}
}
