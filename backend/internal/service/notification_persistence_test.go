package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestNotificationSavePreservesSecretsAndAuditsSafeView(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	s := NewNotificationService(ops.DB, "secret-key")
	password, _ := s.encrypt("private-smtp-test")
	webhook, _ := s.encrypt("private-webhook-test")
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*notification_settings`).WillReturnRows(sqlmock.NewRows([]string{"id", "version", "smtp_password_cipher", "we_com_webhook_cipher"}).AddRow(1, 4, password, webhook))
	m.ExpectExec(`UPDATE .*notification_settings`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	var saved *model.NotificationSettings
	var audited *model.AuditLog
	ops.DB.Callback().Update().Before("gorm:update").Register("test_settings_capture", func(tx *gorm.DB) {
		if x, ok := tx.Statement.Dest.(*model.NotificationSettings); ok {
			copy := *x
			saved = &copy
		}
	})
	ops.DB.Callback().Create().Before("gorm:create").Register("test_audit_capture", func(tx *gorm.DB) {
		if x, ok := tx.Statement.Dest.(*model.AuditLog); ok {
			copy := *x
			audited = &copy
		}
	})
	in := NotificationInput{NotificationSettings: model.DefaultNotificationSettings()}
	in.Version = 4
	uid := uuid.New()
	out, err := s.Save(context.Background(), Actor{UserID: &uid}, in)
	if err != nil {
		t.Fatal(err)
	}
	if saved == nil || saved.SMTPPasswordCipher != password || saved.WeComWebhookCipher != webhook || out.Version != 5 {
		t.Fatal("blank replacement destroyed credentials")
	}
	if audited == nil {
		t.Fatal("missing audit")
	}
	for _, value := range []string{password, webhook, "private-smtp-test", "private-webhook-test"} {
		if strings.Contains(audited.BeforeJSON+audited.AfterJSON, value) {
			t.Fatal("audit leaks credential")
		}
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationSaveRejectsStaleVersion(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery(`SELECT .*notification_settings`).WillReturnRows(sqlmock.NewRows([]string{"id", "version"}).AddRow(1, 5))
	m.ExpectRollback()
	_, err := NewNotificationService(ops.DB, "key").Save(context.Background(), Actor{}, NotificationInput{NotificationSettings: model.DefaultNotificationSettings()})
	if !errors.Is(err, ErrResourceVersion) {
		t.Fatal("stale configuration overwrote latest version", err)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationDispatchRecordsRemoteFailureInsteadOfSuccess(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	s := NewNotificationService(ops.DB, "key")
	webhook, _ := s.encrypt("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=private-key-test")
	s.HTTP.Transport = notificationRoundTripper(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"errcode":40001,"errmsg":"private-key-test"}`))}, nil
	})
	m.ExpectBegin()
	m.ExpectQuery("SELECT pg_try_advisory_xact_lock").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	m.ExpectQuery(`SELECT .*notification_settings`).WillReturnRows(sqlmock.NewRows([]string{"id", "we_com_enabled", "we_com_webhook_cipher"}).AddRow(1, true, webhook))
	m.ExpectQuery(`SELECT .*asset_reminders`).WillReturnRows(sqlmock.NewRows([]string{"id", "key", "kind", "title", "severity"}).AddRow(uuid.New(), "warranty:one", "WARRANTY", "设备已过保", "DANGER"))
	m.ExpectQuery(`SELECT .*notification_deliveries`).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	m.ExpectExec(`INSERT INTO "notification_deliveries"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	var captured *model.NotificationDelivery
	ops.DB.Callback().Create().Before("gorm:create").Register("test_delivery_capture", func(tx *gorm.DB) {
		if x, ok := tx.Statement.Dest.(*model.NotificationDelivery); ok {
			copy := *x
			captured = &copy
		}
	})
	if err := s.Dispatch(context.Background()); err != nil {
		t.Fatal(err)
	}
	if captured == nil || captured.Status != "FAILED" || !strings.Contains(captured.ErrorMessage, "40001") {
		t.Fatal("remote failure was not persisted", captured)
	}
	data, _ := json.Marshal(captured)
	if strings.Contains(string(data), "private-key-test") {
		t.Fatal("delivery history leaks secret")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
