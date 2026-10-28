-- 000003_rack_templates.sql
-- Rack template metadata, immutable versions, and rack instance snapshots.
CREATE TABLE IF NOT EXISTS rack_templates (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL DEFAULT 1,
    code VARCHAR(50) NOT NULL,
    name VARCHAR(150) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    current_revision INTEGER NOT NULL DEFAULT 1,
    remarks TEXT NOT NULL DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_templates_code_active ON rack_templates (code) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rack_templates_status ON rack_templates (status);

CREATE TABLE IF NOT EXISTS rack_template_versions (
    id UUID PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL DEFAULT 1,
    template_id UUID NOT NULL REFERENCES rack_templates(id),
    revision INTEGER NOT NULL,
    type VARCHAR(50) NOT NULL DEFAULT 'STANDARD',
    manufacturer VARCHAR(100) NOT NULL DEFAULT '',
    model_number VARCHAR(100) NOT NULL DEFAULT '',
    u_height INTEGER NOT NULL DEFAULT 42,
    width_mm INTEGER NOT NULL DEFAULT 600,
    depth_mm INTEGER NOT NULL DEFAULT 1200,
    height_mm INTEGER NOT NULL DEFAULT 2000,
    load_capacity_kg DOUBLE PRECISION NULL,
    dual_power BOOLEAN NOT NULL DEFAULT FALSE,
    input_circuits INTEGER NOT NULL DEFAULT 0,
    rated_voltage DOUBLE PRECISION NULL,
    rated_current DOUBLE PRECISION NULL,
    rated_power_kw DOUBLE PRECISION NULL,
    peak_power_kw DOUBLE PRECISION NULL,
    pdu_count INTEGER NOT NULL DEFAULT 0,
    change_note VARCHAR(500) NOT NULL DEFAULT '',
    created_by UUID NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rack_template_revision_active ON rack_template_versions (template_id, revision) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_rack_template_versions_template_id ON rack_template_versions (template_id);

ALTER TABLE racks ADD COLUMN IF NOT EXISTS template_id UUID NULL;
ALTER TABLE racks ADD COLUMN IF NOT EXISTS template_version_id UUID NULL;
ALTER TABLE racks ADD COLUMN IF NOT EXISTS template_code VARCHAR(50) NOT NULL DEFAULT '';
ALTER TABLE racks ADD COLUMN IF NOT EXISTS template_name VARCHAR(150) NOT NULL DEFAULT '';
ALTER TABLE racks ADD COLUMN IF NOT EXISTS template_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE racks ADD COLUMN IF NOT EXISTS template_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_racks_template_id ON racks (template_id);
CREATE INDEX IF NOT EXISTS idx_racks_template_version_id ON racks (template_version_id);