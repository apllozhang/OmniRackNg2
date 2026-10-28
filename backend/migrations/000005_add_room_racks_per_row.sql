-- Add configurable rack wrapping for the room visualization screen.
ALTER TABLE rooms
    ADD COLUMN IF NOT EXISTS racks_per_row INTEGER NOT NULL DEFAULT 8;

ALTER TABLE rooms
    DROP CONSTRAINT IF EXISTS chk_rooms_racks_per_row;

ALTER TABLE rooms
    ADD CONSTRAINT chk_rooms_racks_per_row CHECK (racks_per_row BETWEEN 1 AND 20);
