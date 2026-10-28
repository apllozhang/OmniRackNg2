package service

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func NormalizeScanBaseURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	invalid := func() (string, error) {
		return "", opsInvalid("请填写完整的 HTTP/HTTPS IP 或域名，可含端口；不包含账号密码、路径、查询参数或片段")
	}
	if len(value) > 512 || strings.ContainsAny(value, "\\") || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return invalid()
	}
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Opaque != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(value, "#") {
		return invalid()
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return invalid()
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return invalid()
	}
	if net.ParseIP(u.Hostname()) == nil {
		for _, label := range strings.Split(u.Hostname(), ".") {
			if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
				return invalid()
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
					return invalid()
				}
			}
		}
	}
	return u.Scheme + "://" + strings.ToLower(u.Host), nil
}
func (s *AssetOperations) ScanAccess(ctx context.Context) (model.ScanAccessSettings, error) {
	var out model.ScanAccessSettings
	err := s.DB.WithContext(ctx).First(&out, "id = ?", 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.ScanAccessSettings{}, nil
	}
	return out, err
}
func (s *AssetOperations) SaveScanAccess(ctx context.Context, actor Actor, input model.ScanAccessSettings) (model.ScanAccessSettings, error) {
	var out model.ScanAccessSettings
	address, err := NormalizeScanBaseURL(input.BaseURL)
	if err != nil {
		return out, err
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 338743, 3).Error; e != nil {
			return e
		}
		old, e := NewAssetOperations(tx).ScanAccess(ctx)
		if e != nil {
			return e
		}
		if old.Version != input.Version {
			return ErrResourceVersion
		}
		out = model.ScanAccessSettings{ID: 1, Version: old.Version + 1, BaseURL: address}
		if old.Version == 0 {
			e = tx.Create(&out).Error
		} else {
			e = tx.Save(&out).Error
		}
		if e != nil {
			return e
		}
		return s.Audit(tx, actor, "UPDATE", "scan_access_settings", uuid.Nil, old, out)
	})
	return out, err
}
