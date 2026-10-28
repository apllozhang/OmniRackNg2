package service

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationView struct {
	model.NotificationSettings
	HasSMTPPassword bool `json:"hasSmtpPassword"`
	HasWeComWebhook bool `json:"hasWeComWebhook"`
}

type NotificationInput struct {
	model.NotificationSettings
	SMTPPassword      string `json:"smtpPassword"`
	WeComWebhook      string `json:"weComWebhook"`
	ClearSMTPPassword bool   `json:"clearSmtpPassword"`
	ClearWeComWebhook bool   `json:"clearWeComWebhook"`
}

type NotificationService struct {
	DB                  *gorm.DB
	key                 [32]byte
	HTTP                *http.Client
	approvalWeComSender func(context.Context, model.NotificationSettings, string) error
	approvalSender      func(context.Context, model.NotificationSettings, string, string) error
}

func NewNotificationService(db *gorm.DB, secret string) *NotificationService {
	return &NotificationService{DB: db, key: sha256.Sum256([]byte("asset-notifications/v1:" + secret)), HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

func notificationView(x model.NotificationSettings) NotificationView {
	return NotificationView{NotificationSettings: x, HasSMTPPassword: x.SMTPPasswordCipher != "", HasWeComWebhook: x.WeComWebhookCipher != ""}
}

func ReadNotificationSettings(db *gorm.DB) (model.NotificationSettings, error) {
	x := model.DefaultNotificationSettings()
	err := db.First(&x, "id = ?", 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.DefaultNotificationSettings(), nil
	}
	return x, err
}

func (s *NotificationService) View(ctx context.Context) (NotificationView, error) {
	x, err := ReadNotificationSettings(s.DB.WithContext(ctx))
	return notificationView(x), err
}

func (s *NotificationService) encrypt(value string) (string, error) {
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, []byte(value), nil)), nil
}

func (s *NotificationService) decrypt(value string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", errors.New("通知凭据无法解密，请重新保存凭据")
	}
	block, _ := aes.NewCipher(s.key[:])
	gcm, _ := cipher.NewGCM(block)
	if len(data) < gcm.NonceSize() {
		return "", errors.New("通知凭据无法解密，请重新保存凭据")
	}
	plain, err := gcm.Open(nil, data[:gcm.NonceSize()], data[gcm.NonceSize():], nil)
	if err != nil {
		return "", errors.New("通知凭据无法解密，请重新保存凭据")
	}
	return string(plain), nil
}

func validMailbox(value string) bool {
	if strings.ContainsAny(value, "\r\n") || len(value) > 254 {
		return false
	}
	x, err := mail.ParseAddress(value)
	return err == nil && x.Address == value && strings.Contains(value, "@")
}

func validWeComWebhook(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && len(raw) < 2048 && u.Scheme == "https" && u.Host == "qyapi.weixin.qq.com" && u.User == nil && u.Path == "/cgi-bin/webhook/send" && u.Fragment == "" && len(u.Query()) == 1 && len(u.Query()["key"]) == 1 && len(u.Query().Get("key")) >= 8
}

func validateNotificationSettings(x model.NotificationSettings) error {
	if err := validateApprovalEmail(x); err != nil {
		return err
	}
	if x.RackWarningPercent < 1 || x.RackCriticalPercent > 100 || x.RackWarningPercent >= x.RackCriticalPercent {
		return opsInvalid("U 位阈值须满足 1 ≤ 预警 < 严重 ≤ 100")
	}
	if x.RepeatHours < 1 || x.RepeatHours > 168 {
		return opsInvalid("重复提醒间隔应为 1–168 小时")
	}
	if x.EmailEnabled {
		if x.SMTPHost == "" || len(x.SMTPHost) > 253 || strings.ContainsAny(x.SMTPHost, " /\\\r\n\t:@") || x.SMTPPort < 1 || x.SMTPPort > 65535 {
			return opsInvalid("请填写有效 SMTP 主机名和端口")
		}
		if x.SMTPSecurity != "TLS" && x.SMTPSecurity != "STARTTLS" && x.SMTPSecurity != "NONE" {
			return opsInvalid("请选择 SMTP 加密方式")
		}
		if x.SMTPSecurity == "NONE" && (x.SMTPUsername != "" || x.SMTPPasswordCipher != "") {
			return opsInvalid("未加密 SMTP 仅支持无认证内网中继，请清除账号密码或使用 TLS")
		}
		if len(x.SMTPUsername) > 255 || strings.ContainsAny(x.SMTPUsername, "\r\n") || (x.SMTPUsername != "" && x.SMTPPasswordCipher == "") {
			return opsInvalid("SMTP 认证账号需配置密码或授权码")
		}
		if !validMailbox(x.EmailFrom) || len(x.EmailRecipients) < 1 || len(x.EmailRecipients) > 20 {
			return opsInvalid("请填写发件邮箱和 1–20 个收件邮箱")
		}
		for _, address := range x.EmailRecipients {
			if !validMailbox(address) {
				return opsInvalid("收件邮箱格式无效")
			}
		}
	}
	if x.WeComEnabled && x.WeComWebhookCipher == "" {
		return opsInvalid("请配置企业微信群机器人 Webhook")
	}
	return nil
}

func (s *NotificationService) Save(ctx context.Context, actor Actor, in NotificationInput) (NotificationView, error) {
	var out model.NotificationSettings
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 20261019, 2).Error; err != nil {
			return err
		}
		old, err := ReadNotificationSettings(tx)
		if err != nil {
			return err
		}
		if old.Version != in.Version {
			return ErrResourceVersion
		}
		x := in.NotificationSettings
		// Approval settings have their own endpoint; stale alert forms must not overwrite them.
		x.ApprovalEmailEnabled = old.ApprovalEmailEnabled
		x.ApprovalWeComEnabled = old.ApprovalWeComEnabled
		x.ApprovalEmailRecipients = old.ApprovalEmailRecipients
		x.ApprovalBaseURL = old.ApprovalBaseURL
		x.ID = 1
		x.Version = old.Version + 1
		x.SMTPHost = strings.TrimSpace(x.SMTPHost)
		x.SMTPUsername = strings.TrimSpace(x.SMTPUsername)
		x.EmailFrom = strings.TrimSpace(x.EmailFrom)
		x.SMTPPasswordCipher = old.SMTPPasswordCipher
		x.WeComWebhookCipher = old.WeComWebhookCipher
		if in.ClearSMTPPassword {
			x.SMTPPasswordCipher = ""
		}
		if in.ClearWeComWebhook {
			x.WeComWebhookCipher = ""
		}
		if len(in.SMTPPassword) > 2048 {
			return opsInvalid("SMTP 密码过长")
		}
		if in.SMTPPassword != "" {
			x.SMTPPasswordCipher, err = s.encrypt(in.SMTPPassword)
			if err != nil {
				return err
			}
		}
		if in.WeComWebhook != "" {
			if !validWeComWebhook(strings.TrimSpace(in.WeComWebhook)) {
				return opsInvalid("请填写官方企业微信群机器人 HTTPS Webhook 地址")
			}
			x.WeComWebhookCipher, err = s.encrypt(strings.TrimSpace(in.WeComWebhook))
			if err != nil {
				return err
			}
		}
		seen := map[string]bool{}
		recipients := []string{}
		for _, address := range x.EmailRecipients {
			address = strings.TrimSpace(address)
			if !seen[address] {
				recipients = append(recipients, address)
				seen[address] = true
			}
		}
		x.EmailRecipients = recipients
		if err = validateNotificationSettings(x); err != nil {
			return err
		}
		if err = tx.Save(&x).Error; err != nil {
			return err
		}
		// Only safe views enter the audit log; encrypted/plaintext credentials are excluded.
		if err = NewAssetOperations(tx).Audit(tx, actor, "UPDATE", "notification_settings", uuid.Nil, notificationView(old), notificationView(x)); err != nil {
			return err
		}
		out = x
		return nil
	})
	return notificationView(out), err
}

func newEmailMessageID(from string) string {
	parts := strings.Split(from, "@")
	return "<" + uuid.NewString() + "@" + parts[len(parts)-1] + ">"
}

func (s *NotificationService) sendEmail(ctx context.Context, x model.NotificationSettings, subject, body string) error {
	return s.sendEmailWithID(ctx, x, subject, body, newEmailMessageID(x.EmailFrom))
}

func (s *NotificationService) sendEmailWithID(ctx context.Context, x model.NotificationSettings, subject, body, messageID string) error {
	dialer := net.Dialer{Timeout: 12 * time.Second}
	address := net.JoinHostPort(x.SMTPHost, strconv.Itoa(x.SMTPPort))
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return errors.New("SMTP 连接失败，请检查地址、端口及网络")
	}
	defer conn.Close()
	deadline := time.Now().Add(20 * time.Second)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	tlsConfig := &tls.Config{ServerName: x.SMTPHost, MinVersion: tls.VersionTLS12}
	if x.SMTPSecurity == "TLS" {
		tlsConn := tls.Client(conn, tlsConfig)
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return errors.New("SMTP TLS 握手失败，请检查服务器证书")
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, x.SMTPHost)
	if err != nil {
		return errors.New("SMTP 服务响应失败")
	}
	defer client.Close()
	if x.SMTPSecurity == "STARTTLS" {
		if err = client.StartTLS(tlsConfig); err != nil {
			return errors.New("SMTP STARTTLS 失败，请检查加密方式及证书")
		}
	}
	if x.SMTPUsername != "" {
		password, e := s.decrypt(x.SMTPPasswordCipher)
		if e != nil {
			return e
		}
		if err = client.Auth(smtp.PlainAuth("", x.SMTPUsername, password, x.SMTPHost)); err != nil {
			return errors.New("SMTP 认证失败，请检查账号或授权码")
		}
	}
	if err = client.Mail(x.EmailFrom); err != nil {
		return errors.New("SMTP 拒绝发件邮箱")
	}
	for _, to := range x.EmailRecipients {
		if err = client.Rcpt(to); err != nil {
			return errors.New("SMTP 拒绝收件邮箱")
		}
	}
	w, err := client.Data()
	if err != nil {
		return errors.New("SMTP 无法接收消息")
	}
	message := "Message-ID: " + messageID + "\r\nFrom: " + x.EmailFrom + "\r\nTo: " + strings.Join(x.EmailRecipients, ", ") + "\r\nSubject: " + mime.QEncoding.Encode("UTF-8", subject) + "\r\nDate: " + time.Now().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: base64\r\n\r\n"
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	for len(encoded) > 76 {
		message += encoded[:76] + "\r\n"
		encoded = encoded[76:]
	}
	message += encoded + "\r\n"
	if _, err = io.WriteString(w, message); err != nil {
		return errors.New("SMTP 消息传输失败，发送状态未知")
	}
	if err = w.Close(); err != nil {
		return errors.New("SMTP 未确认接收消息，发送状态未知")
	}
	// DATA completion confirms server acceptance; QUIT failure must not trigger a duplicate.
	_ = client.Quit()
	return nil
}

func (s *NotificationService) sendWeCom(ctx context.Context, x model.NotificationSettings, body string) error {
	webhook, err := s.decrypt(x.WeComWebhookCipher)
	if err != nil {
		return err
	}
	if !validWeComWebhook(webhook) {
		return errors.New("企微 Webhook 无效，请重新配置")
	}
	data, _ := json.Marshal(map[string]interface{}{"msgtype": "text", "text": map[string]string{"content": body}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook, bytes.NewReader(data))
	if err != nil {
		return errors.New("企微请求创建失败")
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := s.HTTP.Do(req)
	if err != nil {
		return errors.New("企微连接失败，请检查网络、代理及证书")
	}
	defer res.Body.Close()
	var result struct {
		ErrCode *int `json:"errcode"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&result) != nil || result.ErrCode == nil {
		return errors.New("企微响应异常，消息未确认送达")
	}
	if *result.ErrCode != 0 {
		return fmt.Errorf("企微拒绝消息（错误码 %d）", *result.ErrCode)
	}
	return nil
}

func (s *NotificationService) send(ctx context.Context, x model.NotificationSettings, channel, body string) error {
	if err := validateNotificationSettings(x); err != nil {
		return errors.New("通知配置不完整，请重新保存配置")
	}
	switch channel {
	case "EMAIL":
		if !x.EmailEnabled {
			return opsInvalid("请先保存并启用邮件渠道")
		}
		return s.sendEmail(ctx, x, "IT 基础设施管理平台 · 告警", body)
	case "WECOM":
		if !x.WeComEnabled {
			return opsInvalid("请先保存并启用企微渠道")
		}
		return s.sendWeCom(ctx, x, body)
	default:
		return opsInvalid("通知渠道无效")
	}
}

func clipUTF8(value string, maximum int) string {
	if len(value) <= maximum {
		return value
	}
	value = value[:maximum]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value + "…"
}

func notificationDigest(items []model.AssetReminder) (string, string) {
	ordered := append([]model.AssetReminder(nil), items...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })
	h := sha256.New()
	for _, x := range ordered {
		data, _ := json.Marshal([]string{x.Key, x.Kind, x.Title, x.Detail, x.Severity, x.OccurrenceAt.UTC().Format(time.RFC3339Nano)})
		h.Write(data)
	}
	body := fmt.Sprintf("IT 基础设施管理平台 · 告警 · 共 %d 项\n", len(items))
	// The full digest includes every alert; payload is bounded for WeCom's text limit.
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Severity == "DANGER" && ordered[j].Severity != "DANGER" })
	shown := 0
	for _, x := range ordered {
		line := "\n• " + clipUTF8(x.Title, 100) + "\n  " + clipUTF8(x.Detail, 120) + "\n"
		if len(body)+len(line) > 1700 {
			break
		}
		body += line
		shown++
	}
	if shown < len(items) {
		body += fmt.Sprintf("\n另有 %d 项，请登录系统查看。", len(items)-shown)
	}
	body += "\n请登录系统「工作台 → 提醒中心」处理。"
	return hex.EncodeToString(h.Sum(nil)), body
}

func notificationDue(last *model.NotificationDelivery, now time.Time, repeatHours int) bool {
	if last == nil {
		return true
	}
	interval := time.Duration(repeatHours) * time.Hour
	if last.Status != "SUCCESS" {
		interval = 15 * time.Minute
	}
	return !now.Before(last.CreatedAt.Add(interval))
}

// A short transaction lock prevents concurrent workers from sending the same digest.
// Delivery is at-least-once: a process/database failure after remote acceptance may retry.
func (s *NotificationService) Dispatch(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked bool
		if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?, ?)", 20261019, 3).Scan(&locked).Error; err != nil {
			return err
		}
		if !locked {
			return nil
		}
		x, err := ReadNotificationSettings(tx)
		if err != nil {
			return err
		}
		if !x.EmailEnabled && !x.WeComEnabled {
			return nil
		}
		kinds := []string{}
		if x.WarrantyEnabled {
			kinds = append(kinds, "WARRANTY", "PART_WARRANTY")
		}
		if x.RackUEnabled {
			kinds = append(kinds, "RACK_U")
		}
		if len(kinds) == 0 {
			return nil
		}
		items := []model.AssetReminder{}
		if err = tx.Where("active = true AND acknowledged_by IS NULL AND kind IN ?", kinds).Order("key").Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		fingerprint, body := notificationDigest(items)
		for _, channel := range []string{"EMAIL", "WECOM"} {
			if (channel == "EMAIL" && !x.EmailEnabled) || (channel == "WECOM" && !x.WeComEnabled) {
				continue
			}
			var previous model.NotificationDelivery
			err = tx.Where("channel = ? AND fingerprint = ? AND test = false", channel, fingerprint).Order("created_at DESC").First(&previous).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err == nil && !notificationDue(&previous, time.Now(), x.RepeatHours) {
				continue
			}
			delivery := model.NotificationDelivery{Channel: channel, Fingerprint: fingerprint, AlertCount: len(items), Summary: fmt.Sprintf("%d 项资产保修 / U 位告警", len(items)), Status: "SUCCESS"}
			if err = s.send(ctx, x, channel, body); err != nil {
				delivery.Status = "FAILED"
				delivery.ErrorMessage = clipUTF8(err.Error(), 240)
			}
			if err = tx.Create(&delivery).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *NotificationService) Test(ctx context.Context, actor Actor, channel string) (model.NotificationDelivery, error) {
	var result model.NotificationDelivery
	if channel != "EMAIL" && channel != "WECOM" {
		return result, opsInvalid("通知渠道无效")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 20261019, 3).Error; err != nil {
			return err
		}
		x, err := ReadNotificationSettings(tx)
		if err != nil {
			return err
		}
		var recent int64
		if err = tx.Model(&model.NotificationDelivery{}).Where("test = true AND channel = ? AND created_at > ?", channel, time.Now().Add(-time.Minute)).Count(&recent).Error; err != nil {
			return err
		}
		if recent > 0 {
			return opsInvalid("测试通知每个渠道每分钟最多一次，请稍后再试")
		}
		delivery := model.NotificationDelivery{Channel: channel, Test: true, Status: "SUCCESS", Summary: "管理员发送的渠道测试"}
		if err = s.send(ctx, x, channel, "IT 基础设施管理平台 · 渠道测试\n管理员发起了此测试，通知通道已连接。"); err != nil {
			delivery.Status = "FAILED"
			delivery.ErrorMessage = clipUTF8(err.Error(), 240)
		}
		if e := tx.Create(&delivery).Error; e != nil {
			return e
		}
		if e := NewAssetOperations(tx).Audit(tx, actor, "TEST", "notification_settings", uuid.Nil, nil, map[string]string{"channel": channel, "status": delivery.Status}); e != nil {
			return e
		}
		result = delivery
		return nil
	})
	return result, err
}
