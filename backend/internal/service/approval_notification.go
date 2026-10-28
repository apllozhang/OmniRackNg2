package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ApprovalEmailSettings struct {
	Version      uint     `json:"version"`
	Enabled      bool     `json:"enabled"`
	Recipients   []string `json:"recipients"`
	BaseURL      string   `json:"baseUrl"`
	EmailEnabled bool     `json:"emailEnabled"`
	WeComEnabled bool     `json:"weComEnabled"`
	WeComReady   bool     `json:"weComReady"`
}

func approvalEmailView(x model.NotificationSettings) ApprovalEmailSettings {
	recipients := append([]string{}, x.ApprovalEmailRecipients...)
	return ApprovalEmailSettings{x.Version, x.ApprovalEmailEnabled, recipients, x.ApprovalBaseURL, x.EmailEnabled, x.ApprovalWeComEnabled, x.WeComEnabled}
}

func validateApprovalEmail(x model.NotificationSettings) error {
	if !x.ApprovalEmailEnabled && !x.ApprovalWeComEnabled {
		return nil
	}
	if x.ApprovalEmailEnabled && !x.EmailEnabled {
		return opsInvalid("请先在告警通知配置中启用 SMTP 邮件；停用 SMTP 前请先关闭审批邮件通知")
	}
	if x.ApprovalWeComEnabled && !x.WeComEnabled {
		return opsInvalid("请先在告警通知中启用企业微信，再开启审批企微通知")
	}
	base, err := NormalizeScanBaseURL(x.ApprovalBaseURL)
	if err != nil || base == "" {
		return opsInvalid("请填写管理员可访问的系统 HTTP/HTTPS 根地址，不包含路径或参数")
	}
	if x.ApprovalEmailEnabled && (len(x.ApprovalEmailRecipients) < 1 || len(x.ApprovalEmailRecipients) > 20) {
		return opsInvalid("请填写 1–20 个管理员收件邮箱")
	}
	for _, address := range x.ApprovalEmailRecipients {
		if !validMailbox(address) {
			return opsInvalid("管理员收件邮箱格式无效")
		}
	}
	return nil
}

func (s *NotificationService) ApprovalSettings(ctx context.Context) (ApprovalEmailSettings, error) {
	x, err := ReadNotificationSettings(s.DB.WithContext(ctx))
	return approvalEmailView(x), err
}

func (s *NotificationService) SaveApprovalSettings(ctx context.Context, actor Actor, in ApprovalEmailSettings) (ApprovalEmailSettings, error) {
	var out ApprovalEmailSettings
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 20261019, 2).Error; err != nil {
			return err
		}
		x, err := ReadNotificationSettings(tx)
		if err != nil {
			return err
		}
		if x.Version != in.Version {
			return ErrResourceVersion
		}
		before := approvalEmailView(x)
		base, err := NormalizeScanBaseURL(in.BaseURL)
		if err != nil {
			return err
		}
		recipients, seen := []string{}, map[string]bool{}
		for _, address := range in.Recipients {
			address = strings.TrimSpace(address)
			key := strings.ToLower(address)
			if address != "" && !seen[key] {
				recipients = append(recipients, address)
				seen[key] = true
			}
		}
		x.ApprovalEmailEnabled, x.ApprovalEmailRecipients, x.ApprovalBaseURL = in.Enabled, recipients, base
		x.ApprovalWeComEnabled = in.WeComEnabled
		if err := validateNotificationSettings(x); err != nil {
			return err
		}
		x.ID, x.Version = 1, x.Version+1
		if err := tx.Save(&x).Error; err != nil {
			return err
		}
		out = approvalEmailView(x)
		return NewAssetOperations(tx).Audit(tx, actor, "UPDATE", "approval_email_settings", uuid.Nil, before, out)
	})
	return out, err
}

func approvalEmailBody(a model.ApprovalRecord, base string) string {
	operation := "迁移"
	if a.Operation == "ASSIGN" {
		operation = "上架"
	}
	locationLabel := "目标"
	if a.Operation == model.PositionOperationDecommission {
		operation, locationLabel = "下架", "原"
	}
	requester := "—"
	if a.RequestedBy != nil {
		requester = a.RequestedBy.String()
	}
	return fmt.Sprintf("IT 基础设施管理平台 · 待审批\n\n申请编号：%s\n操作：%s\n设备：%s\n设备编码：%s\nSN：%s\n申请账号 ID：%s\n%s机柜 ID：%s\n%s U 位：%d–%dU\n申请时间：%s\n申请原因：%s\n\n请使用管理员账号登录后审批：\n%s/approvals\n\n通知仅作提醒，最终状态以系统为准。", a.ID, operation, a.Device.Name, a.Device.Code, a.Device.SerialNumber, requester, locationLabel, a.TargetRackID, locationLabel, a.TargetStartU, a.TargetStartU+a.TargetHeightU-1, a.RequestedAt.Format(time.RFC3339), a.Reason, base)
}

// Each delivery commits separately. Remote acceptance followed by a process/DB failure
// can still cause a retry; email links never perform a decision themselves.
// Channels use separate locks and workers: an SMTP timeout must not block WeCom.
func (s *NotificationService) DispatchApprovals(ctx context.Context) error {
	x, err := ReadNotificationSettings(s.DB.WithContext(ctx))
	if err != nil {
		return err
	}
	var workers sync.WaitGroup
	failures := make(chan error, 2)
	for _, channel := range []string{"EMAIL", "WECOM"} {
		if (channel == "EMAIL" && !x.ApprovalEmailEnabled) || (channel == "WECOM" && !x.ApprovalWeComEnabled) {
			continue
		}
		workers.Add(1)
		go func(channel string) {
			defer workers.Done()
			if e := s.dispatchApprovalChannel(ctx, channel); e != nil {
				failures <- e
			}
		}(channel)
	}
	workers.Wait()
	close(failures)
	var all []error
	for e := range failures {
		all = append(all, e)
	}
	return errors.Join(all...)
}

func approvalChannelSettings(x model.NotificationSettings, channel string) (model.NotificationSettings, error) {
	switch channel {
	case "EMAIL":
		if !x.ApprovalEmailEnabled {
			return x, opsInvalid("请先保存并启用审批邮件通知")
		}
		x.ApprovalWeComEnabled, x.WeComEnabled = false, false
		x.EmailRecipients = append([]string{}, x.ApprovalEmailRecipients...)
	case "WECOM":
		if !x.ApprovalWeComEnabled {
			return x, opsInvalid("请先保存并启用审批企业微信通知")
		}
		x.ApprovalEmailEnabled, x.EmailEnabled = false, false
	default:
		return x, opsInvalid("通知渠道无效")
	}
	return x, validateNotificationSettings(x)
}

func (s *NotificationService) sendApproval(ctx context.Context, x model.NotificationSettings, channel, body, messageID string) error {
	if channel == "WECOM" {
		if s.approvalWeComSender != nil {
			return s.approvalWeComSender(ctx, x, body)
		}
		return s.sendWeCom(ctx, x, body)
	}
	if s.approvalSender != nil {
		return s.approvalSender(ctx, x, "IT 基础设施管理平台 · 待审批通知", body)
	}
	return s.sendEmailWithID(ctx, x, "IT 基础设施管理平台 · 待审批通知", body, messageID)
}

func (s *NotificationService) dispatchApprovalChannel(ctx context.Context, channel string) error {
	lockID := 4
	if channel == "WECOM" {
		lockID = 5
	}
	for i := 0; i < 20; i++ {
		didWork := false
		err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var locked bool
			if err := tx.Raw("SELECT pg_try_advisory_xact_lock(?, ?)", 20261019, lockID).Scan(&locked).Error; err != nil {
				return err
			}
			if !locked {
				return nil
			}
			x, err := ReadNotificationSettings(tx)
			if err != nil {
				return err
			}
			if (channel == "EMAIL" && !x.ApprovalEmailEnabled) || (channel == "WECOM" && !x.ApprovalWeComEnabled) {
				return nil
			}
			x, err = approvalChannelSettings(x, channel)
			if err != nil {
				return err
			}
			var a model.ApprovalRecord
			err = tx.Where("status = ?", "PENDING").Where(`NOT EXISTS (SELECT 1 FROM notification_deliveries nd WHERE nd.fingerprint = 'approval:' || approval_records.id::text AND nd.channel = ? AND nd.test = false AND nd.deleted_at IS NULL AND (nd.status = 'SUCCESS' OR nd.created_at > ?))`, channel, time.Now().Add(-15*time.Minute)).Order("requested_at ASC, id ASC").First(&a).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			if err != nil {
				return err
			}
			if err := tx.First(&a.Device, "id = ?", a.DeviceID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			delivery := model.NotificationDelivery{Channel: channel, Fingerprint: "approval:" + a.ID.String(), AlertCount: 1, Summary: "待审批 · " + a.ID.String(), Status: "SUCCESS"}
			if channel == "EMAIL" {
				delivery.Recipients = append([]string{}, x.EmailRecipients...)
				delivery.MessageID = newEmailMessageID(x.EmailFrom)
			}
			if err := s.sendApproval(ctx, x, channel, approvalEmailBody(a, x.ApprovalBaseURL), delivery.MessageID); err != nil {
				delivery.Status, delivery.ErrorMessage = "FAILED", "SMTP 发送失败，请检查邮件配置及网络；系统将在 15 分钟后重试"
				if channel == "WECOM" {
					delivery.ErrorMessage = "企业微信发送失败，请检查机器人配置及网络；系统将在 15 分钟后重试"
				}
			}
			if err := tx.Create(&delivery).Error; err != nil {
				return err
			}
			didWork = true
			return nil
		})
		if err != nil {
			return err
		}
		if !didWork {
			break
		}
	}
	return nil
}

func (s *NotificationService) TestApproval(ctx context.Context, actor Actor, channel string) (model.NotificationDelivery, error) {
	var out model.NotificationDelivery
	if channel != "EMAIL" && channel != "WECOM" {
		return out, opsInvalid("通知渠道无效")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Share the channel test rate limit, and do not mark any real approval delivered.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 20261019, 3).Error; err != nil {
			return err
		}
		x, err := ReadNotificationSettings(tx)
		if err != nil {
			return err
		}
		x, err = approvalChannelSettings(x, channel)
		if err != nil {
			return err
		}
		var recent int64
		if err := tx.Model(&model.NotificationDelivery{}).Where("test = true AND channel = ? AND created_at > ?", channel, time.Now().Add(-time.Minute)).Count(&recent).Error; err != nil {
			return err
		}
		if recent > 0 {
			return opsInvalid("测试通知每个渠道每分钟最多一次，请稍后再试")
		}
		out = model.NotificationDelivery{Channel: channel, Test: true, Status: "SUCCESS", Summary: "管理员发送的审批通知测试"}
		if channel == "EMAIL" {
			out.Recipients = append([]string{}, x.EmailRecipients...)
			out.MessageID = newEmailMessageID(x.EmailFrom)
		}
		body := "IT 基础设施管理平台 · 审批通知测试\n\n这是审批渠道配置测试，不对应真实审批申请。\n请登录审批中心：" + x.ApprovalBaseURL + "/approvals"
		if err := s.sendApproval(ctx, x, channel, body, out.MessageID); err != nil {
			out.Status = "FAILED"
			out.ErrorMessage = "审批通知测试失败，请检查对应渠道配置及网络"
		}
		if err := tx.Create(&out).Error; err != nil {
			return err
		}
		return NewAssetOperations(tx).Audit(tx, actor, "TEST", "approval_notification", uuid.Nil, nil, map[string]string{"channel": channel, "status": out.Status})
	})
	return out, err
}
