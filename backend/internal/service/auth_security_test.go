package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/config"
	"omnirack-replica/backend/internal/middleware"
	"omnirack-replica/backend/internal/model"
)

func authMock(t *testing.T) (*AuthService, sqlmock.Sqlmock) {
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
	return NewAuthService(db, config.Config{JWTSecret: "auth-security-test-secret", JWTExpiresIn: time.Hour}), mock
}

func expectAuthUser(mock sqlmock.Sqlmock, id uuid.UUID, version uint, enabled bool, roleCode string) {
	mock.ExpectQuery(`SELECT .* FROM "users"`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "enabled", "username", "auth_source"}).AddRow(id.String(), version, enabled, "test-user", "local"))
	if roleCode == "" {
		mock.ExpectQuery(`SELECT .* FROM "user_roles"`).WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
		return
	}
	roleID := uuid.New()
	mock.ExpectQuery(`SELECT .* FROM "user_roles"`).WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}).AddRow(id.String(), roleID.String()))
	mock.ExpectQuery(`SELECT .* FROM "roles"`).WillReturnRows(sqlmock.NewRows([]string{"id", "code"}).AddRow(roleID.String(), roleCode))
}

func TestResolveIdentityRejectsDisabledAndRevokedUsers(t *testing.T) {
	for _, test := range []struct {
		name         string
		enabled      bool
		tokenVersion uint
	}{
		{"disabled", false, 4}, {"password-changed", true, 3}, {"legacy-token", true, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, mock := authMock(t)
			id := uuid.New()
			expectAuthUser(mock, id, 4, test.enabled, "user")
			_, err := svc.ResolveIdentity(context.Background(), &middleware.Claims{UserID: id.String(), UserVersion: test.tokenVersion, Roles: []string{"system_admin"}})
			if err == nil {
				t.Fatal("disabled or revoked identity accepted")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResolveIdentityUsesDatabaseRoles(t *testing.T) {
	svc, mock := authMock(t)
	id := uuid.New()
	expectAuthUser(mock, id, 4, true, "user")
	roles, err := svc.ResolveIdentity(context.Background(), &middleware.Claims{UserID: id.String(), UserVersion: 4, Roles: []string{"system_admin"}})
	if err != nil || len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("stale token roles retained: %v, %v", roles, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIdentityAndLoginDistinguishDatabaseOutageFromBadCredentials(t *testing.T) {
	svc, mock := authMock(t)
	mock.ExpectQuery(`SELECT .* FROM "users"`).WillReturnError(errors.New("database temporarily offline"))
	_, err := svc.ResolveIdentity(context.Background(), &middleware.Claims{UserID: uuid.NewString(), UserVersion: 1})
	if !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("database outage not retryable: %v", err)
	}
	mock.ExpectQuery(`SELECT .* FROM "users"`).WillReturnError(errors.New("database temporarily offline"))
	_, err = svc.Login(context.Background(), "test-user", "password123")
	if !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("database outage treated as wrong password: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogoutPersistentlyRevokesPreviouslyIssuedToken(t *testing.T) {
	svc, mock := authMock(t)
	id := uuid.New()
	claims := &middleware.Claims{UserID: id.String(), UserVersion: 4}
	expectAuthUser(mock, id, 4, true, "user")
	if _, err := svc.ResolveIdentity(context.Background(), claims); err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "users" SET "version"=version \+ 1 WHERE`).WithArgs(id.String(), true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := svc.Logout(context.Background(), id.String()); err != nil {
		t.Fatal(err)
	}
	expectAuthUser(mock, id, 5, true, "user")
	if _, err := svc.ResolveIdentity(context.Background(), claims); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("logged-out token still accepted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestIssueTokenCannotRestoreStaleRolesOrIgnoreConcurrentAccountChange(t *testing.T) {
	for _, affected := range []int64{0, 1} {
		svc, mock := authMock(t)
		user := model.User{BaseModel: model.BaseModel{ID: uuid.New(), Version: 4}, Enabled: true,
			Roles: []model.Role{{BaseModel: model.BaseModel{ID: uuid.New()}, Code: "system_admin"}}}
		mock.ExpectBegin()
		// Any INSERT into roles/user_roles would fail this expectation.
		mock.ExpectExec(`UPDATE "users" SET .* WHERE .*id = .* AND version = .* AND enabled =`).
			WithArgs(0, sqlmock.AnyArg(), sqlmock.AnyArg(), user.ID.String(), 4, true).WillReturnResult(sqlmock.NewResult(0, affected))
		mock.ExpectCommit()
		result, err := svc.issueToken(context.Background(), user)
		if affected == 0 && !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("concurrent account update accepted: %v", err)
		}
		if affected == 1 && (err != nil || result.Token == "") {
			t.Fatalf("unchanged account login failed: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLDAPSyncMapsConcurrentMutationToInvalidCredentials(t *testing.T) {
	svc, mock := authMock(t)
	id := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT .* FROM "users".*FOR UPDATE`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "version", "enabled", "auth_source"}).AddRow(id.String(), 5, true, "ldap"))
	mock.ExpectQuery(`SELECT .* FROM "user_roles"`).WillReturnRows(sqlmock.NewRows([]string{"user_id", "role_id"}))
	mock.ExpectRollback()
	_, err := svc.syncLDAPIdentity(context.Background(), model.User{BaseModel: model.BaseModel{ID: id, Version: 4}},
		LDAPIdentity{DisplayName: "directory name", Email: "test@example.invalid"}, []model.Role{{Code: "user"}})
	if !errors.Is(err, ErrInvalidCredentials) || errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("changed LDAP identity should be rejected without overwriting admin changes: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLDAPSyncKeepsDatabaseOutageRetryable(t *testing.T) {
	svc, mock := authMock(t)
	id := uuid.New()
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnError(errors.New("database temporarily offline"))
	mock.ExpectRollback()
	_, err := svc.syncLDAPIdentity(context.Background(), model.User{BaseModel: model.BaseModel{ID: id, Version: 4}},
		LDAPIdentity{DisplayName: "directory name"}, []model.Role{{Code: "user"}})
	if !errors.Is(err, ErrAuthUnavailable) {
		t.Fatalf("LDAP synchronization database outage should remain retryable: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
