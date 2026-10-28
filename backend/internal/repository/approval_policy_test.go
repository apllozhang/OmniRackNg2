package repository

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"testing"
)

func TestApprovalPolicyPersistsExplicitFalseWithVersionCheck(t *testing.T) {
	resource, m := newMockResourceStore(t)
	id := uuid.New()
	policy := model.OperationPolicy{BaseModel: model.BaseModel{ID: id, Version: 3}, ApprovalMode: "SINGLE", DefaultApproverRole: "system_admin", DecommissionApprovalEnabled: false}
	m.ExpectExec(updateSQLPattern("operation_policies")).WithArgs("SINGLE", false, false, "system_admin", false, sqlmock.AnyArg(), id, uint(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := NewDeviceStore(resource.db).UpdateOperationPolicy(context.Background(), &policy, 3); err != nil {
		t.Fatal(err)
	}
	if policy.Version != 4 || policy.DecommissionApprovalEnabled {
		t.Fatalf("incorrect policy: %+v", policy)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
