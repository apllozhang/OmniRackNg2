package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"omnirack-replica/backend/internal/model"
)

func Open(databaseURL string) (*gorm.DB, error) {
	return gorm.Open(postgres.Open(databaseURL), &gorm.Config{TranslateError: true})
}

func Migrate(db *gorm.DB) error {
	// PostgreSQL DDL is transactional. Serialize starting replicas and roll back
	// the entire migration if any constraint cannot be installed.
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(?, ?)", 20261028, 1).Error; err != nil {
			return err
		}
		return migrateSchema(tx)
	})
}

func migrateSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&model.User{}, &model.Role{}, &model.Permission{}, &model.AuditLog{}, &model.LDAPConfig{},
		&model.RackTemplate{}, &model.RackTemplateVersion{},
		&model.DataCenter{}, &model.Room{}, &model.Rack{},
		&model.DeviceType{}, &model.Device{}, &model.RackDevicePosition{}, &model.RackUOccupancy{},
		&model.DevicePositionHistory{}, &model.PDU{}, &model.PDUSocket{}, &model.PDUConnection{}, &model.OperationPolicy{}, &model.ApprovalRecord{},
		&model.AssetSite{}, &model.AssetProfile{}, &model.AssetCoverage{}, &model.SpareSpec{}, &model.SpareStock{},
		&model.PCHandover{},
		&model.SpareMovement{}, &model.SpareMinimum{}, &model.AssetCheckTask{}, &model.AssetCheckLine{}, &model.AssetCheckDiscovery{}, &model.AssetReminder{},
		&model.ScanAccessSettings{}, &model.NotificationSettings{}, &model.NotificationDelivery{},
	); err != nil {
		return err
	}
	return ensureDevicePositionConstraints(db)
}

func ensureDevicePositionConstraints(db *gorm.DB) error {
	statements := []string{
		`CREATE EXTENSION IF NOT EXISTS btree_gist`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_asset_active ON devices (asset_number) WHERE deleted_at IS NULL AND asset_number <> ''`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_device_positions_device_active ON rack_device_positions (device_id) WHERE deleted_at IS NULL`,
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'rack_u_occupancies_no_overlap' AND conrelid = 'rack_u_occupancies'::regclass) THEN
				ALTER TABLE rack_u_occupancies ADD CONSTRAINT rack_u_occupancies_no_overlap
				EXCLUDE USING gist (rack_id WITH =, int8range(start_u, end_u, '[]') WITH &&)
				WHERE (deleted_at IS NULL);
			END IF;
		END $$`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

func SeedRoles(ctx context.Context, db *gorm.DB) error {
	roles := []model.Role{{Code: "system_admin", Name: "系统管理员"}, {Code: "user", Name: "普通用户"}, {Code: "asset_manager", Name: "资产管理员"}, {Code: "asset_auditor", Name: "资产审计员"}}
	for i := range roles {
		if err := db.WithContext(ctx).Where("code = ?", roles[i].Code).FirstOrCreate(&roles[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

func SeedAdmin(ctx context.Context, db *gorm.DB, username, password string) error {
	var user model.User
	result := db.WithContext(ctx).Preload("Roles").Where("username = ?", username).First(&user)
	if result.Error == nil {
		return nil
	}
	if result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	var role model.Role
	if err := db.WithContext(ctx).Where("code = ?", "system_admin").First(&role).Error; err != nil {
		return err
	}
	user = model.User{Username: username, DisplayName: "系统管理员", AuthSource: "local", Enabled: true, Roles: []model.Role{role}}
	if err := user.SetPassword(password); err != nil {
		return err
	}
	return db.WithContext(ctx).Create(&user).Error
}

func FindUserByUsername(ctx context.Context, db *gorm.DB, username string) (model.User, error) {
	var user model.User
	err := db.WithContext(ctx).Preload("Roles").Where("username = ?", username).First(&user).Error
	return user, err
}

func FindUserByID(ctx context.Context, db *gorm.DB, id uuid.UUID) (model.User, error) {
	var user model.User
	err := db.WithContext(ctx).Preload("Roles").First(&user, "id = ?", id).Error
	return user, err
}

// SeedDeviceCatalog creates baseline device types and the default disabled approval policy.
func SeedDeviceCatalog(ctx context.Context, db *gorm.DB) error {
	defaults := []model.DeviceType{
		{Code: "TERMINAL-MONITOR", Name: "显示器", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 1},
		{Code: "TERMINAL-DESKTOP", Name: "主机", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 2},
		{Code: "TERMINAL-PRINTER", Name: "打印机", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 3},
		{Code: "TERMINAL-LAPTOP", Name: "笔记本", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 4},
		{Code: "TERMINAL-MOUSE", Name: "鼠标", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 5},
		{Code: "TERMINAL-KEYBOARD", Name: "键盘", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 6},
		{Code: "TERMINAL-OTHER", Name: "其他终端", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 7},
		{Code: "PC", Name: "终端（待细分）", Category: model.DeviceCategoryPC, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 8},
		{Code: "SERVER", Name: "服务器", Category: model.DeviceCategoryServer, Status: model.DeviceTypeStatusActive, DefaultHeightU: 2, SortOrder: 10},
		{Code: "NETWORK", Name: "网络设备", Category: model.DeviceCategoryNetwork, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 20},
		{Code: "STORAGE", Name: "存储设备", Category: model.DeviceCategoryStorage, Status: model.DeviceTypeStatusActive, DefaultHeightU: 2, SortOrder: 30},
		{Code: "SECURITY", Name: "安全设备", Category: model.DeviceCategorySecurity, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 40},
		{Code: "POWER-ENV", Name: "配电与环境设备", Category: model.DeviceCategoryPowerEnv, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 50},
		{Code: "ACCESSORY", Name: "机柜配件", Category: model.DeviceCategoryAccessory, Status: model.DeviceTypeStatusActive, DefaultHeightU: 1, SortOrder: 60},
	}
	for i := range defaults {
		if err := db.WithContext(ctx).Unscoped().Where("code = ?", defaults[i].Code).FirstOrCreate(&defaults[i]).Error; err != nil {
			return err
		}
	}
	// Only rename the original system default. Keep IDs, associations and custom names.
	if err := db.WithContext(ctx).Model(&model.DeviceType{}).Where("code = ? AND category = ? AND name = ?", "PC", model.DeviceCategoryPC, "PC 终端").Updates(map[string]interface{}{"name": "终端（待细分）", "version": gorm.Expr("version + 1"), "updated_at": time.Now().UTC()}).Error; err != nil {
		return err
	}
	var count int64
	if err := db.WithContext(ctx).Model(&model.OperationPolicy{}).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		policy := model.OperationPolicy{DecommissionApprovalEnabled: true, ApprovalMode: "SINGLE", DefaultApproverRole: "system_admin"}
		return db.WithContext(ctx).Create(&policy).Error
	}
	return nil
}
