package repository

import (
	"context"
	"errors"
	"omnirack-replica/backend/internal/model"
	"github.com/google/uuid"
)

var ErrAssetRelations = errors.New("device has asset profile, coverage, inventory or installed parts")

type AssetDeletionGuard interface {
	AssertCanDeleteAsset(context.Context, uuid.UUID) error
}

func (s *GormDeviceStore) AssertCanDeleteAsset(ctx context.Context, id uuid.UUID) error {
	for _, query := range []struct {
		entity    interface{}
		condition string
	}{
		{&model.AssetProfile{}, "device_id = ?"}, {&model.AssetCoverage{}, "device_id = ?"},
		{&model.SpareStock{}, "installed_device_id = ? AND state = 'INSTALLED'"},
		{&model.AssetCheckLine{}, "target_id = ? AND kind = 'DEVICE'"},
	} {
		var n int64
		if e := s.db.WithContext(ctx).Model(query.entity).Where(query.condition, id).Count(&n).Error; e != nil {
			return e
		}
		if n > 0 {
			return ErrAssetRelations
		}
	}
	return nil
}
