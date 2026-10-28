package service

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func dualApprovalSettingsRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "version", "email_enabled", "smtp_host", "smtp_port", "smtp_security", "email_from", "email_recipients", "approval_email_enabled", "approval_email_recipients", "approval_base_url", "approval_we_com_enabled", "we_com_enabled", "we_com_webhook_cipher"}).AddRow(1, 4, true, "smtp.example.test", 25, "NONE", "it@example.test", `["alerts@example.test"]`, true, `["admin@example.test"]`, "https://it.example.test", true, true, "encrypted")
}

func TestDualApprovalChannelsRunIndependentlyAndUseSeparateDeduplication(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	m.MatchExpectationsInOrder(false)
	s := NewNotificationService(ops.DB, "key")
	id, deviceID, rackID := uuid.New(), uuid.New(), uuid.New()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	wecomSent := make(chan struct{})
	var once sync.Once
	s.approvalSender = func(ctx context.Context, x model.NotificationSettings, _, _ string) error {
		if len(x.EmailRecipients) != 1 || x.EmailRecipients[0] != "admin@example.test" {
			t.Error("wrong approval recipient")
		}
		select {
		case <-wecomSent:
			return errors.New("private-secret")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.approvalWeComSender = func(_ context.Context, x model.NotificationSettings, body string) error {
		if !x.WeComEnabled || !strings.Contains(body, "/approvals") {
			t.Error("wrong WeCom notification")
		}
		once.Do(func() { close(wecomSent) })
		return nil
	}
	var mu sync.Mutex
	deliveries := map[string]model.NotificationDelivery{}
	ops.DB.Callback().Create().Before("gorm:create").Register("capture_dual", func(tx *gorm.DB) {
		if x, ok := tx.Statement.Dest.(*model.NotificationDelivery); ok {
			mu.Lock()
			deliveries[x.Channel] = *x
			mu.Unlock()
		}
	})
	m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(dualApprovalSettingsRows())
	for _, channel := range []string{"EMAIL", "WECOM"} {
		for i := 0; i < 2; i++ {
			lockID := 4
			if channel == "WECOM" {
				lockID = 5
			}
			m.ExpectBegin()
			m.ExpectQuery("SELECT pg_try_advisory_xact_lock").WithArgs(20261019, lockID).WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
			m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(dualApprovalSettingsRows())
			rows := sqlmock.NewRows([]string{"id", "device_id", "operation", "status", "requested_at", "target_rack_id", "target_start_u", "target_height_u"})
			if i == 0 {
				rows.AddRow(id, deviceID, "DECOMMISSION", "PENDING", time.Now(), rackID, 5, 2)
			}
			m.ExpectQuery(`SELECT .*approval_records.*nd.channel = .*nd.status = 'SUCCESS' OR nd.created_at >`).WithArgs("PENDING", channel, sqlmock.AnyArg(), 1).WillReturnRows(rows)
			if i == 0 {
				m.ExpectQuery("SELECT .*devices").WillReturnRows(sqlmock.NewRows([]string{"id", "name"}).AddRow(deviceID, "服务器"))
				m.ExpectExec(`INSERT INTO "notification_deliveries"`).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			m.ExpectCommit()
		}
	}
	if err := s.DispatchApprovals(ctx); err != nil {
		t.Fatal(err)
	}
	if deliveries["EMAIL"].Status != "FAILED" || deliveries["WECOM"].Status != "SUCCESS" {
		t.Fatalf("one channel blocked the other: %+v", deliveries)
	}
	if strings.Contains(deliveries["EMAIL"].ErrorMessage, "private-secret") || deliveries["EMAIL"].MessageID == "" || len(deliveries["EMAIL"].Recipients) != 1 {
		t.Fatal("unsafe or incomplete receipt metadata")
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalWeComCanWorkWithoutEmailButRequiresSavedRobot(t *testing.T) {
	x := model.DefaultNotificationSettings()
	x.ApprovalWeComEnabled = true
	x.ApprovalBaseURL = "https://it.example.test"
	if validateApprovalEmail(x) == nil {
		t.Fatal("accepted disabled WeCom master channel")
	}
	x.WeComEnabled = true
	x.WeComWebhookCipher = "encrypted"
	if _, err := approvalChannelSettings(x, "WECOM"); err != nil {
		t.Fatal(err)
	}
	if _, err := approvalChannelSettings(x, "EMAIL"); err == nil {
		t.Fatal("accepted disabled approval email")
	}
	x.ApprovalBaseURL = "https://it.example.test/admin"
	if validateApprovalEmail(x) == nil {
		t.Fatal("accepted non-root access address")
	}
}

func TestApprovalChannelTestUsesIndependentRecipientsAndDoesNotConsumeApproval(t *testing.T) {
	ops, m, closeDB := opsMock(t)
	defer closeDB()
	s := NewNotificationService(ops.DB, "key")
	s.approvalSender = func(_ context.Context, x model.NotificationSettings, _, body string) error {
		if x.EmailRecipients[0] != "admin@example.test" || !strings.Contains(body, "不对应真实审批") {
			t.Fatal("not an approval channel test")
		}
		return nil
	}
	m.ExpectBegin()
	m.ExpectExec("SELECT pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT .*notification_settings").WillReturnRows(dualApprovalSettingsRows())
	m.ExpectQuery("SELECT count.*notification_deliveries").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.ExpectExec(`INSERT INTO "notification_deliveries"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec(`INSERT INTO "audit_logs"`).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectCommit()
	out, err := s.TestApproval(context.Background(), Actor{}, "EMAIL")
	if err != nil {
		t.Fatal(err)
	}
	if !out.Test || out.Fingerprint != "" || out.MessageID == "" || len(out.Recipients) != 1 {
		t.Fatalf("test altered normal dedup or lost recipients: %+v", out)
	}
	if err := m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
