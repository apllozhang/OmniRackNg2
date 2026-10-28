package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func approvalSettingsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "version", "email_enabled", "smtp_host", "smtp_port", "smtp_security", "email_from", "email_recipients", "approval_email_enabled", "approval_email_recipients", "approval_base_url"}).AddRow(1, 4, true, "smtp.example.test", 25, "NONE", "it@example.test", `["alerts@example.test"]`, true, `["admin@example.test"]`, "https://it.example.test")
}

func TestApprovalSettingsPreserveSMTPAndDeduplicateRecipients(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(approvalSettingsRows())
	m.ExpectExec("UPDATE .*notification_settings").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	var saved model.NotificationSettings
	ops.DB.Callback().Update().Before("gorm:update").Register("approval_capture", func(tx *gorm.DB) {
		if x, ok := tx.Statement.Dest.(*model.NotificationSettings); ok {
			saved = *x
		}
	})
	out, err := NewNotificationService(ops.DB, "key").SaveApprovalSettings(context.Background(), Actor{}, ApprovalEmailSettings{Version: 4, Enabled: true, Recipients: []string{" admin@example.test ", "ADMIN@example.test"}, BaseURL: "https://IT.example.test/", EmailEnabled: false})
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != 5 || len(out.Recipients) != 1 || out.BaseURL != "https://it.example.test" || !out.EmailEnabled || saved.SMTPHost != "smtp.example.test" || saved.EmailRecipients[0] != "alerts@example.test" {
		t.Fatalf("settings not isolated: %+v / %+v", out, saved)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAlertSettingsCannotOverwriteApprovalSettings(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(dualApprovalSettingsRows())
	m.ExpectExec("UPDATE .*notification_settings").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	var saved model.NotificationSettings
	ops.DB.Callback().Update().Before("gorm:update").Register("approval_capture", func(tx *gorm.DB) {
		if x, ok := tx.Statement.Dest.(*model.NotificationSettings); ok {
			saved = *x
		}
	})
	x := model.DefaultNotificationSettings()
	x.WeComEnabled = true
	x.Version, x.EmailEnabled, x.SMTPHost, x.SMTPPort, x.SMTPSecurity, x.EmailFrom, x.EmailRecipients = 4, true, "smtp.example.test", 25, "NONE", "it@example.test", []string{"alerts@example.test"}
	if _, err := NewNotificationService(ops.DB, "key").Save(context.Background(), Actor{}, NotificationInput{NotificationSettings: x}); err != nil {
		t.Fatal(err)
	}
	if !saved.ApprovalWeComEnabled || !saved.ApprovalEmailEnabled || saved.ApprovalBaseURL != "https://it.example.test" || saved.ApprovalEmailRecipients[0] != "admin@example.test" {
		t.Fatal("alert save destroyed approval configuration")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalEmailValidation(t *testing.T) {
	x := model.DefaultNotificationSettings()
	x.ApprovalEmailEnabled = true
	x.ApprovalBaseURL, x.ApprovalEmailRecipients = "https://it.example.test", []string{"admin@example.test"}
	if validateApprovalEmail(x) == nil {
		t.Fatal("enabled without SMTP")
	}
	x.EmailEnabled = true
	if err := validateApprovalEmail(x); err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{"", "javascript:alert(1)", "https://it.example.test/admin", "https://user:pass@it.example.test"} {
		x.ApprovalBaseURL = base
		if validateApprovalEmail(x) == nil {
			t.Fatalf("accepted unsafe/incomplete address: %q", base)
		}
	}
	x.ApprovalBaseURL = "https://it.example.test"
	x.ApprovalEmailRecipients = []string{"admin@example.test\r\nBcc:other@example.test"}
	if validateApprovalEmail(x) == nil {
		t.Fatal("accepted header injection")
	}
}

func TestApprovalSettingsRejectStaleVersion(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(approvalSettingsRows())
	m.ExpectRollback()
	_, err := NewNotificationService(ops.DB, "key").SaveApprovalSettings(context.Background(), Actor{}, ApprovalEmailSettings{Version: 3})
	if !errors.Is(err, ErrResourceVersion) {
		t.Fatal("stale settings were accepted", err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalDispatchCommitsSuccessAndFailureWithoutSendingRealMail(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			ops, m, closeDB := opsMock(t)
			defer closeDB()
			s := NewNotificationService(ops.DB, "key")
			id, deviceID, rackID := uuid.New(), uuid.New(), uuid.New()
			calls := 0
			m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(approvalSettingsRows())
			s.approvalSender = func(_ context.Context, x model.NotificationSettings, subject, body string) error {
				calls++
				if len(x.EmailRecipients) != 1 || x.EmailRecipients[0] != "admin@example.test" || !strings.Contains(subject, "待审批") || !strings.Contains(body, "SN-001") || !strings.Contains(body, "https://it.example.test/approvals") || !strings.Contains(body, "12–13U") {
					t.Fatalf("wrong email: %v %s", x.EmailRecipients, body)
				}
				if failed {
					return errors.New("private-password")
				}
				return nil
			}
			var delivery model.NotificationDelivery
			ops.DB.Callback().Create().Before("gorm:create").Register("approval_delivery_capture", func(tx *gorm.DB) {
				if x, ok := tx.Statement.Dest.(*model.NotificationDelivery); ok {
					delivery = *x
				}
			})
			for i := 0; i < 2; i++ {
				m.ExpectBegin()
				m.ExpectQuery("SELECT pg_try_advisory_xact_lock").WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
				m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(approvalSettingsRows())
				rows := sqlmock.NewRows([]string{"id", "device_id", "operation", "status", "requested_at", "target_rack_id", "target_start_u", "target_height_u"})
				if i == 0 {
					rows.AddRow(id, deviceID, "ASSIGN", "PENDING", time.Now(), rackID, 12, 2)
				}
				m.ExpectQuery(`SELECT .*approval_records.*NOT EXISTS .*nd.status = 'SUCCESS' OR nd.created_at >`).WithArgs("PENDING", "EMAIL", sqlmock.AnyArg(), 1).WillReturnRows(rows)
				if i == 0 {
					m.ExpectQuery("SELECT .*devices").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "code", "serial_number"}).AddRow(deviceID, "服务器", "DEV-001", "SN-001"))
					m.ExpectExec(`INSERT INTO "notification_deliveries"`).WillReturnResult(sqlmock.NewResult(0, 1))
				}
				m.ExpectCommit()
			}
			if err := s.DispatchApprovals(context.Background()); err != nil {
				t.Fatal(err)
			}
			want := "SUCCESS"
			if failed {
				want = "FAILED"
			}
			if calls != 1 || delivery.Status != want || delivery.Fingerprint != "approval:"+id.String() || strings.Contains(delivery.ErrorMessage, "private-password") {
				t.Fatalf("wrong delivery: %+v", delivery)
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestApprovalEmailDisabledDoesNotQueryOrSendApprovals(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(sqlmock.NewRows([]string{"id", "approval_email_enabled"}).AddRow(1, false))
	s := NewNotificationService(ops.DB, "key")
	s.approvalSender = func(context.Context, model.NotificationSettings, string, string) error {
		t.Fatal("disabled notification sent")
		return nil
	}
	if err := s.DispatchApprovals(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
