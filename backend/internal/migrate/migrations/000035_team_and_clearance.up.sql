-- Clearance history table. Every clearance change (initial, upgrade,
-- downgrade, revocation) inserts a new row; the prior current row is
-- marked superseded. users.current_clearance_id is a denormalized
-- pointer to the latest non-superseded row for fast list queries.
--
-- Modeled as history (rather than columns on users) so an auditor can
-- reconstruct "what was their clearance when visit X was approved on
-- date Y" — required for defensible review records.

CREATE TYPE clearance_level AS ENUM (
    'none',
    'confidential',
    'secret',
    'top_secret',
    'ts_sci'
);

CREATE TYPE investigation_type AS ENUM (
    'T3',
    'T3R',
    'T5',
    'T5R'
);

CREATE TABLE user_clearance_records (
    id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    clearance                clearance_level NOT NULL,
    investigation_type       investigation_type,
    eligibility_date         DATE,
    last_investigation_date  DATE,
    next_investigation_date  DATE,
    notes                    TEXT NOT NULL DEFAULT '',
    recorded_by              UUID NOT NULL REFERENCES users(id),
    recorded_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    superseded_at            TIMESTAMPTZ
);

CREATE INDEX idx_clearance_user_current
    ON user_clearance_records(user_id)
    WHERE superseded_at IS NULL;

CREATE INDEX idx_clearance_user_history
    ON user_clearance_records(user_id, recorded_at DESC);

ALTER TABLE users
    ADD COLUMN current_clearance_id UUID REFERENCES user_clearance_records(id);
