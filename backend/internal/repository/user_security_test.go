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

func expectLDAPLockedIdentity(mock sqlmock.Sqlmock, id uuid.UUID, version uint, enabled bool, source string, roleID uuid.UUID, roleCode string) {
	expectUserAdministrationLock(mock)
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version", "enabled", "auth_source", "display_name", "email"}).
			AddRow(id.String(), version, enabled, source, "old name", "old@example.invalid"))
	mock.ExpectQuery(`SELECT .* FROM "user_roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}).AddRow(id.String(), roleID.String()))
	mock.ExpectQuery(`SELECT .* FROM "roles"`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(roleID.String(), roleCode))
}

func TestLDAPIdentitySyncRejectsConcurrentAccountChanges(t *testing.T) {
	for _, test := range []struct {
		name    string
		version uint
		enabled bool
		source  string
	}{
		{"changed-version", 5, true, "ldap"},
		{"disabled", 4, false, "ldap"},
		{"became-local", 4, true, "local"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store, mock := userSecurityMock(t)
			id, roleID := uuid.New(), uuid.New()
			expectLDAPLockedIdentity(mock, id, test.version, test.enabled, test.source, roleID, "user")
			mock.ExpectRollback()
			_, err := SyncLDAPIdentity(context.Background(), store.db, id, 4, "LDAP name", "ldap@example.invalid", []string{"system_admin"})
			if !errors.Is(err, ErrLDAPIdentityChanged) {
				t.Fatalf("concurrently changed LDAP identity accepted: %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLDAPProfileRefreshPreservesVersionAndCannotSaveStaleAssociations(t *testing.T) {
	store, mock := userSecurityMock(t)
	id, roleID := uuid.New(), uuid.New()
	expectLDAPLockedIdentity(mock, id, 4, true, "ldap", roleID, "user")
	mock.ExpectQuery(`SELECT .* FROM "roles"`).WithArgs("user", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(roleID.String(), "user"))
	// The exact SET list rejects a version increment, password/auth-source write,
	// or GORM association insert during a routine LDAP profile refresh.
	mock.ExpectExec(`UPDATE "users" SET "display_name"=\$1,"email"=\$2,"updated_at"=\$3 WHERE \(id = \$4 AND version = \$5 AND enabled = \$6 AND auth_source = \$7\) AND "users"\."deleted_at" IS NULL`).
		WithArgs("LDAP name", "ldap@example.invalid", sqlmock.AnyArg(), id.String(), 4, true, "ldap").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	user, err := SyncLDAPIdentity(context.Background(), store.db, id, 4, "LDAP name", "ldap@example.invalid", []string{"user", "user"})
	if err != nil || user.Version != 4 || user.DisplayName != "LDAP name" || len(user.Roles) != 1 {
		t.Fatalf("profile refresh unexpectedly changed sessions or roles: %+v, %v", user, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLDAPRoleMappingCannotDemoteLastEnabledAdministrator(t *testing.T) {
	store, mock := userSecurityMock(t)
	id, oldRoleID, nextRoleID := uuid.New(), uuid.New(), uuid.New()
	expectLDAPLockedIdentity(mock, id, 4, true, "ldap", oldRoleID, "system_admin")
	mock.ExpectQuery(`SELECT .* FROM "roles"`).WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(nextRoleID.String(), "user"))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery(`SELECT count\(\*\) FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()
	_, err := SyncLDAPIdentity(context.Background(), store.db, id, 4, "LDAP name", "ldap@example.invalid", []string{"user"})
	if !errors.Is(err, ErrLastEnabledAdmin) {
		t.Fatalf("LDAP role mapping removed last administrator: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLDAPRoleChangeAtomicallyAdvancesVersionAndReplacesOnlyJoinRows(t *testing.T) {
	store, mock := userSecurityMock(t)
	id, oldRoleID, nextRoleID := uuid.New(), uuid.New(), uuid.New()
	expectLDAPLockedIdentity(mock, id, 4, true, "ldap", oldRoleID, "user")
	mock.ExpectQuery(`SELECT .* FROM "roles"`).WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(nextRoleID.String(), "system_admin"))
	mock.ExpectExec(`UPDATE "users" SET "display_name"=\$1,"email"=\$2,"version"=version \+ 1,"updated_at"=\$3 WHERE \(id = \$4 AND version = \$5 AND enabled = \$6 AND auth_source = \$7\) AND "users"\."deleted_at" IS NULL`).
		WithArgs("LDAP name", "ldap@example.invalid", sqlmock.AnyArg(), id.String(), 4, true, "ldap").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`DELETE FROM user_roles WHERE user_id =`).WithArgs(id.String()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO user_roles \(user_id, role_id\) VALUES`).WithArgs(id.String(), nextRoleID.String()).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	user, err := SyncLDAPIdentity(context.Background(), store.db, id, 4, "LDAP name", "ldap@example.invalid", []string{"system_admin"})
	if err != nil || user.Version != 5 || len(user.Roles) != 1 || user.Roles[0].Code != "system_admin" {
		t.Fatalf("role change did not produce the new account version: %+v, %v", user, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
