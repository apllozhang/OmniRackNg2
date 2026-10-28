package repository

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"omnirack-replica/backend/internal/model"
)

var ErrAssetRelations = errors.New("device has asset profile, coverage, inventory or installed parts")

type AssetDeletionGuard interface {
	AssertCanDeleteAsset(context.Context, uuid.UUID) error
}

// CountActivePowerConnections is checked while the device row is locked. PDU connection
// creation takes the same lock, so a device cannot leave its rack between the
// connection check and the committed position change.
func (s *GormDeviceStore) CountActivePowerConnections(ctx context.Context, id uuid.UUID) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&model.PDUConnection{}).Where("device_id = ?", id).Count(&count).Error
	return count, err
}

func (s *GormDeviceStore) AssertCanDeleteAsset(ctx context.Context, id uuid.UUID) error {
	for _, query := range []struct {
		entity    interface{}
		condition string
	}{
		{&model.AssetProfile{}, "device_id = ?"}, {&model.AssetCoverage{}, "device_id = ?"},
		{&model.SpareStock{}, "installed_device_id = ? AND state = 'INSTALLED'"},
		{&model.AssetCheckLine{}, "target_id = ? AND kind = 'DEVICE'"},
		{&model.PDUConnection{}, "device_id = ?"},
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
