package service

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"omnirack-replica/backend/internal/model"
)

func TestNotificationSecretsEncryptedRedactedAndAuthenticated(t *testing.T) {
	s := NewNotificationService(nil, "test-key")
	password := "private-test-credential"
	one, err := s.encrypt(password)
	if err != nil {
		t.Fatal(err)
	}
	two, _ := s.encrypt(password)
	if one == two || strings.Contains(one, password) {
		t.Fatal("encryption must use a unique nonce")
	}
	if plain, err := s.decrypt(one); err != nil || plain != password {
		t.Fatal("roundtrip failed", err)
	}
	if _, err := NewNotificationService(nil, "different-key").decrypt(one); err == nil {
		t.Fatal("wrong key accepted")
	}
	data, _ := base64.StdEncoding.DecodeString(one)
	data[len(data)-1] ^= 1
	if _, err := s.decrypt(base64.StdEncoding.EncodeToString(data)); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	x := model.DefaultNotificationSettings()
	x.SMTPPasswordCipher = one
	x.WeComWebhookCipher = two
	view := notificationView(x)
	payload, _ := json.Marshal(view)
	if !view.HasSMTPPassword || !view.HasWeComWebhook || strings.Contains(string(payload), one) || strings.Contains(string(payload), password) || strings.Contains(string(payload), "Cipher") {
		t.Fatal("safe view leaks credentials", string(payload))
	}
}

func TestNotificationValidation(t *testing.T) {
	base := model.DefaultNotificationSettings()
	base.EmailEnabled = true
	base.SMTPHost = "smtp.example.test"
	base.EmailFrom = "assets@example.test"
	base.EmailRecipients = []string{"ops@example.test"}
	if err := validateNotificationSettings(base); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*model.NotificationSettings){
		func(x *model.NotificationSettings) { x.RackWarningPercent = 90 },
		func(x *model.NotificationSettings) { x.RepeatHours = 0 },
		func(x *model.NotificationSettings) { x.SMTPPort = 0 },
		func(x *model.NotificationSettings) { x.SMTPHost = "smtp.example.test\r\nInjected" },
		func(x *model.NotificationSettings) {
			x.EmailRecipients = []string{"test@example.test\r\nBcc: other@example.test"}
		},
		func(x *model.NotificationSettings) { x.SMTPUsername = "user" },
		func(x *model.NotificationSettings) {
			x.SMTPSecurity = "NONE"
			x.SMTPUsername = "user"
			x.SMTPPasswordCipher = "secret"
		},
		func(x *model.NotificationSettings) { x.WeComEnabled = true },
	} {
		x := base
		change(&x)
		if validateNotificationSettings(x) == nil {
			t.Fatal("invalid settings accepted", x)
		}
	}
	if !validWeComWebhook("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=12345678") {
		t.Fatal("valid webhook rejected")
	}
	for _, raw := range []string{"http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=12345678", "https://localhost/cgi-bin/webhook/send?key=12345678", "https://qyapi.weixin.qq.com.evil.test/cgi-bin/webhook/send?key=12345678", "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=12345678&other=x", "https://user@qyapi.weixin.qq.com/cgi-bin/webhook/send?key=12345678"} {
		if validWeComWebhook(raw) {
			t.Fatal("unsafe webhook accepted", raw)
		}
	}
}

func TestRackUnitsAndExactThresholdBoundaries(t *testing.T) {
	positions := []model.RackUOccupancy{{StartU: 1, EndU: 4}, {StartU: 3, EndU: 6}, {StartU: 9, EndU: 20}, {StartU: -3, EndU: 0}}
	if got := usedRackUnits(10, positions); got != 8 {
		t.Fatalf("got %d, want merged and bounded count 8", got)
	}
	policy := model.DefaultNotificationSettings()
	for _, tt := range []struct {
		used, capacity int
		severity       string
	}{{69, 100, ""}, {70, 100, "WARNING"}, {89, 100, "WARNING"}, {90, 100, "DANGER"}, {29, 42, ""}, {30, 42, "WARNING"}, {38, 42, "DANGER"}, {0, 0, ""}} {
		if got := rackUSeverity(tt.used, tt.capacity, policy); got != tt.severity {
			t.Fatalf("%d/%d: got %s want %s", tt.used, tt.capacity, got, tt.severity)
		}
	}
	policy.RackWarningPercent = 80
	policy.RackCriticalPercent = 95
	if rackUSeverity(70, 100, policy) != "" {
		t.Fatal("custom thresholds ignored")
	}
}

func TestNotificationDigestStableBoundedAndDetectsRecurrence(t *testing.T) {
	items := []model.AssetReminder{}
	for i := 0; i < 100; i++ {
		items = append(items, model.AssetReminder{Key: fmt.Sprint(i), Kind: "WARRANTY", Title: strings.Repeat("设备", 100), Detail: strings.Repeat("过保", 100), Severity: "WARNING"})
	}
	one, body := notificationDigest(items)
	if !utf8.ValidString(body) || len(body) > 2048 || !strings.Contains(body, "另有") {
		t.Fatal("invalid or oversized WeCom text", len(body))
	}
	items[0], items[99] = items[99], items[0]
	two, _ := notificationDigest(items)
	if one != two {
		t.Fatal("ordering changed digest")
	}
	items[0].OccurrenceAt = time.Now()
	two, _ = notificationDigest(items)
	if one == two {
		t.Fatal("reactivated alert must notify again")
	}
	items[0].Severity = "DANGER"
	_, body = notificationDigest(items)
	if !strings.Contains(body, "IT 基础设施管理平台 · 告警") {
		t.Fatal("missing summary")
	}
	now := time.Now()
	last := &model.NotificationDelivery{BaseModel: model.BaseModel{CreatedAt: now.Add(-23 * time.Hour)}, Status: "SUCCESS"}
	if notificationDue(last, now, 24) {
		t.Fatal("duplicate before repeat interval")
	}
	last.CreatedAt = now.Add(-24 * time.Hour)
	if !notificationDue(last, now, 24) {
		t.Fatal("repeat due at boundary")
	}
	last.Status = "FAILED"
	last.CreatedAt = now.Add(-14 * time.Minute)
	if notificationDue(last, now, 24) {
		t.Fatal("retry too early")
	}
	last.CreatedAt = now.Add(-15 * time.Minute)
	if !notificationDue(last, now, 24) {
		t.Fatal("failed channel should retry")
	}
}

type notificationRoundTripper func(*http.Request) (*http.Response, error)

func (f notificationRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWeComChecksBusinessResultWithoutLeakingKey(t *testing.T) {
	s := NewNotificationService(nil, "test-key")
	x := model.DefaultNotificationSettings()
	x.WeComEnabled = true
	x.WeComWebhookCipher, _ = s.encrypt("https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=private-test-key")
	for _, tt := range []struct {
		body    string
		status  int
		success bool
	}{{"{\"errcode\":0}", 200, true}, {"{\"errcode\":40001,\"errmsg\":\"private-test-key\"}", 200, false}, {"{}", 200, false}, {"bad json", 200, false}, {"{\"errcode\":0}", 500, false}} {
		s.HTTP.Transport = notificationRoundTripper(func(r *http.Request) (*http.Response, error) {
			if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
				t.Fatal("incorrect request")
			}
			payload, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(payload), "text") {
				t.Fatal("incorrect message payload")
			}
			return &http.Response{StatusCode: tt.status, Body: io.NopCloser(strings.NewReader(tt.body)), Header: make(http.Header)}, nil
		})
		err := s.sendWeCom(context.Background(), x, "渠道测试")
		if (err == nil) != tt.success {
			t.Fatalf("body=%s err=%v", tt.body, err)
		}
		if err != nil && strings.Contains(err.Error(), "private-test-key") {
			t.Fatal("error leaks webhook key")
		}
	}
}

func TestSMTPRelaySendsUTF8AndRequiresStartTLS(t *testing.T) {
	for _, security := range []string{"NONE", "STARTTLS"} {
		t.Run(security, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			captured := make(chan string, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				reader := bufio.NewReader(conn)
				_, _ = io.WriteString(conn, "220 local-test ESMTP\r\n")
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					switch {
					case strings.HasPrefix(line, "EHLO"):
						_, _ = io.WriteString(conn, "250-local-test\r\n250 HELP\r\n")
					case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
						_, _ = io.WriteString(conn, "250 OK\r\n")
					case strings.HasPrefix(line, "DATA"):
						_, _ = io.WriteString(conn, "354 End with dot\r\n")
						var body strings.Builder
						for {
							line, e := reader.ReadString('\n')
							if e != nil {
								return
							}
							if line == ".\r\n" {
								break
							}
							body.WriteString(line)
						}
						captured <- body.String()
						_, _ = io.WriteString(conn, "250 accepted\r\n")
						return // QUIT fails after acceptance; do not report a failed send.
					default:
						_, _ = io.WriteString(conn, "500 unsupported\r\n")
					}
				}
			}()
			x := model.DefaultNotificationSettings()
			x.EmailEnabled = true
			x.SMTPHost = "127.0.0.1"
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			x.SMTPPort, _ = strconv.Atoi(port)
			x.SMTPSecurity = security
			x.EmailFrom = "assets@example.test"
			x.EmailRecipients = []string{"ops@example.test"}
			err = NewNotificationService(nil, "test").sendEmail(context.Background(), x, "测试告警", "中文资产过保提醒")
			if security == "STARTTLS" {
				if err == nil {
					t.Fatal("silently downgraded to plaintext")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			select {
			case message := <-captured:
				if !strings.Contains(message, "Message-ID: <") {
					t.Fatal("missing SMTP Message-ID")
				}
				parts := strings.SplitN(message, "\r\n\r\n", 2)
				if len(parts) != 2 {
					t.Fatal("invalid MIME")
				}
				decoded, e := base64.StdEncoding.DecodeString(strings.ReplaceAll(parts[1], "\r\n", ""))
				if e != nil || string(decoded) != "中文资产过保提醒" {
					t.Fatal("body corrupted", e)
				}
			case <-time.After(time.Second):
				t.Fatal("SMTP message not received")
			}
		})
	}
}
