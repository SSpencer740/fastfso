ALTER TABLE users DROP COLUMN IF EXISTS current_clearance_id;

DROP TABLE IF EXISTS user_clearance_records;

DROP TYPE IF EXISTS investigation_type;
DROP TYPE IF EXISTS clearance_level;
