-- 000004_devices_and_u_positions.sql
-- Device ledger, rack U-position occupancy, history, and configurable single-level approval.
-- Development uses GORM AutoMigrate; production should apply reviewed SQL migrations.

CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE IF NOT EXISTS device_types (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    code VARCHAR(50) NOT NULL, name VARCHAR(150) NOT NULL, category VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE', default_height_u INTEGER NOT NULL DEFAULT 1,
    default_weight_kg DOUBLE PRECISION NULL, default_rated_power_w DOUBLE PRECISION NULL,
    default_peak_power_w DOUBLE PRECISION NULL, default_dual_power BOOLEAN NOT NULL DEFAULT FALSE,
    description TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_device_types_code_active ON device_types (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_device_types_category ON device_types (category);
CREATE INDEX IF NOT EXISTS idx_device_types_status ON device_types (status);

CREATE TABLE IF NOT EXISTS devices (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    type_id UUID NOT NULL REFERENCES device_types(id), code VARCHAR(80) NOT NULL, name VARCHAR(150) NOT NULL,
    asset_number VARCHAR(100) NOT NULL DEFAULT '', serial_number VARCHAR(120) NOT NULL DEFAULT '',
    manufacturer VARCHAR(100) NOT NULL DEFAULT '', model_number VARCHAR(100) NOT NULL DEFAULT '',
    specification VARCHAR(500) NOT NULL DEFAULT '', firmware_version VARCHAR(100) NOT NULL DEFAULT '',
    purchase_batch VARCHAR(100) NOT NULL DEFAULT '', warranty_expires_at TIMESTAMPTZ NULL,
    organization VARCHAR(150) NOT NULL DEFAULT '', manager VARCHAR(100) NOT NULL DEFAULT '', contact VARCHAR(100) NOT NULL DEFAULT '',
    business_system VARCHAR(150) NOT NULL DEFAULT '', application_name VARCHAR(150) NOT NULL DEFAULT '',
    lifecycle_status VARCHAR(30) NOT NULL DEFAULT 'WAITING_RACK', height_u INTEGER NOT NULL DEFAULT 1,
    width_mm INTEGER NULL, depth_mm INTEGER NULL, height_mm INTEGER NULL, weight_kg DOUBLE PRECISION NULL,
    rated_power_w DOUBLE PRECISION NULL, peak_power_w DOUBLE PRECISION NULL, input_voltage DOUBLE PRECISION NULL,
    dual_power_required BOOLEAN NOT NULL DEFAULT FALSE, management_ip VARCHAR(64) NOT NULL DEFAULT '',
    business_ip VARCHAR(64) NOT NULL DEFAULT '', mac_address VARCHAR(64) NOT NULL DEFAULT '',
    management_protocol VARCHAR(100) NOT NULL DEFAULT '', monitoring_status VARCHAR(50) NOT NULL DEFAULT '',
    external_qr_code VARCHAR(500) NOT NULL DEFAULT '', external_qr_code_url VARCHAR(1000) NOT NULL DEFAULT '',
    tags VARCHAR(500) NOT NULL DEFAULT '', remarks TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_code_active ON devices (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_asset_active ON devices (asset_number) WHERE deleted_at IS NULL AND asset_number <> '';
CREATE INDEX IF NOT EXISTS idx_devices_type_id ON devices (type_id);
CREATE INDEX IF NOT EXISTS idx_devices_lifecycle_status ON devices (lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_devices_name ON devices (name);

CREATE TABLE IF NOT EXISTS rack_device_positions (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    device_id UUID NOT NULL REFERENCES devices(id), rack_id UUID NOT NULL REFERENCES racks(id),
    room_id UUID NOT NULL REFERENCES rooms(id), data_center_id UUID NOT NULL REFERENCES data_centers(id),
    face VARCHAR(10) NULL, start_u INTEGER NOT NULL, height_u INTEGER NOT NULL, end_u INTEGER NOT NULL,
    orientation VARCHAR(20) NOT NULL DEFAULT 'NORMAL', installed_at TIMESTAMPTZ NOT NULL,
    installed_by UUID NULL, reason VARCHAR(500) NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_device_positions_device_active ON rack_device_positions (device_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_rack_id ON rack_device_positions (rack_id);
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_room_id ON rack_device_positions (room_id);
CREATE INDEX IF NOT EXISTS idx_rack_device_positions_data_center_id ON rack_device_positions (data_center_id);

CREATE TABLE IF NOT EXISTS rack_u_occupancies (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    position_id UUID NOT NULL REFERENCES rack_device_positions(id), device_id UUID NOT NULL REFERENCES devices(id),
    rack_id UUID NOT NULL REFERENCES racks(id), face VARCHAR(10) NULL,
    start_u INTEGER NOT NULL, end_u INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_position_id ON rack_u_occupancies (position_id);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_device_id ON rack_u_occupancies (device_id);
CREATE INDEX IF NOT EXISTS idx_rack_u_occupancies_rack_id ON rack_u_occupancies (rack_id);

DO $$ BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'rack_u_occupancies_no_overlap') THEN
        ALTER TABLE rack_u_occupancies ADD CONSTRAINT rack_u_occupancies_no_overlap
        EXCLUDE USING gist (
            rack_id WITH =,
            int8range(start_u, end_u, '[]') WITH &&
        ) WHERE (deleted_at IS NULL);
    END IF;
END $$;

CREATE TABLE IF NOT EXISTS device_position_histories (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    device_id UUID NOT NULL REFERENCES devices(id), operation VARCHAR(30) NOT NULL,
    from_position JSONB NULL, to_position JSONB NULL, reason VARCHAR(500) NOT NULL DEFAULT '',
    actor_id UUID NULL, request_id VARCHAR(100) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_device_position_histories_device_id ON device_position_histories (device_id);
CREATE INDEX IF NOT EXISTS idx_device_position_histories_operation ON device_position_histories (operation);

CREATE TABLE IF NOT EXISTS operation_policies (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    assign_approval_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    move_approval_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    approval_mode VARCHAR(20) NOT NULL DEFAULT 'SINGLE',
    default_approver_role VARCHAR(100) NOT NULL DEFAULT 'system_admin'
);

CREATE TABLE IF NOT EXISTS approval_records (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    device_id UUID NOT NULL REFERENCES devices(id), operation VARCHAR(30) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING', requested_by UUID NULL, requested_at TIMESTAMPTZ NOT NULL,
    requested_device_version BIGINT NOT NULL, source_position JSONB NULL,
    target_rack_id UUID NOT NULL REFERENCES racks(id), target_face VARCHAR(10) NULL,
    target_start_u INTEGER NOT NULL, target_height_u INTEGER NOT NULL,
    target_orientation VARCHAR(20) NOT NULL DEFAULT 'NORMAL', reason VARCHAR(500) NOT NULL DEFAULT '',
    decided_by UUID NULL, decided_at TIMESTAMPTZ NULL, decision_comment VARCHAR(500) NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_approval_records_device_id ON approval_records (device_id);
CREATE INDEX IF NOT EXISTS idx_approval_records_status ON approval_records (status);
CREATE INDEX IF NOT EXISTS idx_approval_records_target_rack_id ON approval_records (target_rack_id);

-- Seed rows are created idempotently by repository.SeedDeviceCatalog so IDs are generated consistently by the application.

