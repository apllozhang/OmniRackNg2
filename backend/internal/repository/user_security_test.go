package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
)

func userSecurityMock(t *testing.T) (*GormUserStore, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, PreferSimpleProtocol: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return NewUserStore(db), mock
}

func expectUserAdministrationLock(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WithArgs(userAdministrationLockKey).WillReturnResult(sqlmock.NewResult(0, 1))
}

func TestUserMutationChecksLastAdministratorUnderSharedTransactionLock(t *testing.T) {
	for _, operation := range []string{"demote", "disable", "delete"} {
		t.Run(operation, func(t *testing.T) {
			store, mock := userSecurityMock(t)
			id := uuid.New()
			expectUserAdministrationLock(mock)
			mock.ExpectQuery(`SELECT count\(\*\) FROM "users" JOIN user_roles.*users.id = .*users.version =`).
				WithArgs(id.String(), 3, true, "system_admin").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery(`SELECT count\(\*\) FROM "users" JOIN user_roles.*roles.deleted_at IS NULL.*users.id <>`).
				WithArgs(true, "system_admin", id.String()).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectRollback()
			var err error
			if operation == "delete" {
				err = store.DeleteUser(context.Background(), id, 3)
			} else {
				roles := []string{"user"}
				if operation == "disable" {
					roles = []string{"system_admin"}
				}
				err = store.UpdateUser(context.Background(), &model.User{BaseModel: model.BaseModel{ID: id}, Enabled: operation != "disable"}, 3, roles)
			}
			if !errors.Is(err, ErrLastEnabledAdmin) {
				t.Fatalf("last administrator mutation accepted: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDeleteAllowsAdministratorWhenAnotherEnabledAdministratorRemains(t *testing.T) {
	store, mock := userSecurityMock(t)
	id := uuid.New()
	expectUserAdministrationLock(mock)
	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec(`UPDATE "users" SET "deleted_at"=`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := store.DeleteUser(context.Background(), id, 3); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateUserPreservesExplicitDisabledStateAtomically(t *testing.T) {
	store, mock := userSecurityMock(t)
	id := uuid.New()
	roleID := uuid.New()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT .* FROM "roles"`).WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(roleID.String(), "user"))
	mock.ExpectExec(`INSERT INTO "users"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "roles"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO "user_roles"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "users" SET "enabled"=`).WithArgs(false, id.String()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	item := model.User{BaseModel: model.BaseModel{ID: id}, Username: "disabled-user", AuthSource: "local", Enabled: false}
	if err := store.CreateUser(context.Background(), &item, []string{"user"}); err != nil {
		t.Fatal(err)
	}
	if item.Enabled {
		t.Fatal("explicitly disabled account was enabled by ORM defaults")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
