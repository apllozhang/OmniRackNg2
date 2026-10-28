-- 000002_resource_hierarchy.sql
-- Data center -> room -> rack baseline tables.
CREATE TABLE IF NOT EXISTS data_centers (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL DEFAULT 1,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(150) NOT NULL,
    address VARCHAR(500) NOT NULL DEFAULT '',
    longitude DOUBLE PRECISION NULL,
    latitude DOUBLE PRECISION NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'OPERATING',
    manager VARCHAR(100) NOT NULL DEFAULT '',
    contact VARCHAR(100) NOT NULL DEFAULT '',
    service_provider VARCHAR(150) NOT NULL DEFAULT '',
    commissioned_at TIMESTAMPTZ NULL,
    remarks TEXT NOT NULL DEFAULT '',
    sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_data_centers_code_active ON data_centers (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_data_centers_status ON data_centers (status);
CREATE INDEX IF NOT EXISTS idx_data_centers_sort_order ON data_centers (sort_order);

CREATE TABLE IF NOT EXISTS rooms (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    data_center_id UUID NOT NULL REFERENCES data_centers(id), code VARCHAR(50) NOT NULL, name VARCHAR(150) NOT NULL,
    building VARCHAR(100) NOT NULL DEFAULT '', floor VARCHAR(50) NOT NULL DEFAULT '', room_number VARCHAR(50) NOT NULL DEFAULT '',
    area_square_meters DOUBLE PRECISION NULL, clear_height_meters DOUBLE PRECISION NULL, purpose VARCHAR(200) NOT NULL DEFAULT '',
    status VARCHAR(30) NOT NULL DEFAULT 'OPERATING', environment_level VARCHAR(50) NOT NULL DEFAULT '', max_load_kg DOUBLE PRECISION NULL,
    cooling_capacity_kw DOUBLE PRECISION NULL, design_power_kw DOUBLE PRECISION NULL, available_power_kw DOUBLE PRECISION NULL, used_power_kw DOUBLE PRECISION NULL,
    redundancy_policy VARCHAR(100) NOT NULL DEFAULT '', manager VARCHAR(100) NOT NULL DEFAULT '', contact VARCHAR(100) NOT NULL DEFAULT '',
    open_hours VARCHAR(100) NOT NULL DEFAULT '', access_notes TEXT NOT NULL DEFAULT '', floor_plan_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    floor_plan_format VARCHAR(20) NOT NULL DEFAULT 'SVG', remarks TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rooms_data_center_code ON rooms (data_center_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rooms_data_center_id ON rooms (data_center_id);
CREATE INDEX IF NOT EXISTS idx_rooms_status ON rooms (status);

CREATE TABLE IF NOT EXISTS racks (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), deleted_at TIMESTAMPTZ NULL, version BIGINT NOT NULL DEFAULT 1,
    data_center_id UUID NOT NULL REFERENCES data_centers(id), room_id UUID NOT NULL REFERENCES rooms(id), code VARCHAR(50) NOT NULL, name VARCHAR(150) NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'STANDARD', manufacturer VARCHAR(100) NOT NULL DEFAULT '', model_number VARCHAR(100) NOT NULL DEFAULT '',
    serial_number VARCHAR(100) NOT NULL DEFAULT '', asset_number VARCHAR(100) NOT NULL DEFAULT '', u_height INTEGER NOT NULL DEFAULT 42,
    width_mm INTEGER NOT NULL DEFAULT 600, depth_mm INTEGER NOT NULL DEFAULT 1200, height_mm INTEGER NOT NULL DEFAULT 2000, load_capacity_kg DOUBLE PRECISION NULL,
    zone VARCHAR(100) NOT NULL DEFAULT '', rack_row VARCHAR(50) NOT NULL DEFAULT '', rack_column VARCHAR(50) NOT NULL DEFAULT '', aisle VARCHAR(100) NOT NULL DEFAULT '',
    x_coordinate DOUBLE PRECISION NULL, y_coordinate DOUBLE PRECISION NULL, rotation INTEGER NOT NULL DEFAULT 0, status VARCHAR(30) NOT NULL DEFAULT 'AVAILABLE',
    manager VARCHAR(100) NOT NULL DEFAULT '', department VARCHAR(100) NOT NULL DEFAULT '', purpose VARCHAR(200) NOT NULL DEFAULT '', dual_power BOOLEAN NOT NULL DEFAULT FALSE,
    input_circuits INTEGER NOT NULL DEFAULT 0, rated_voltage DOUBLE PRECISION NULL, rated_current DOUBLE PRECISION NULL, rated_power_kw DOUBLE PRECISION NULL,
    peak_power_kw DOUBLE PRECISION NULL, pdu_count INTEGER NOT NULL DEFAULT 0, remarks TEXT NOT NULL DEFAULT '', sort_order INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_racks_room_code ON racks (room_id, code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_racks_data_center_id ON racks (data_center_id);
CREATE INDEX IF NOT EXISTS idx_racks_room_id ON racks (room_id);
CREATE INDEX IF NOT EXISTS idx_racks_status ON racks (status);
