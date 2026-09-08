-- Add debrief tracking to travel_reports
ALTER TABLE travel_reports ADD COLUMN debrief_created_at TIMESTAMPTZ;

-- Update action_items source_type constraint to include 'travel_debrief'
ALTER TABLE action_items DROP CONSTRAINT action_items_source_type_check;
ALTER TABLE action_items ADD CONSTRAINT action_items_source_type_check
    CHECK (source_type IN (
        'task_submission', 'visit_request', 'travel_report', 'travel_debrief',
        'incident_report', 'clearance_renewal', 'sf86_submission', 'report'
    ));

-- Store IC debrief records and responses
CREATE TABLE travel_debriefs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id       UUID NOT NULL REFERENCES travel_reports(id) ON DELETE CASCADE,
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status          TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'submitted')),
    due_date        DATE,
    q1_foreign_contact  BOOLEAN,
    q1_details          TEXT NOT NULL DEFAULT '',
    q2_surveillance     BOOLEAN,
    q2_details          TEXT NOT NULL DEFAULT '',
    q3_equipment_loss   BOOLEAN,
    q3_details          TEXT NOT NULL DEFAULT '',
    q4_unusual_requests BOOLEAN,
    q4_details          TEXT NOT NULL DEFAULT '',
    submitted_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_travel_debriefs_report_id ON travel_debriefs(report_id);
CREATE INDEX idx_travel_debriefs_user_id ON travel_debriefs(user_id);
CREATE INDEX idx_travel_debriefs_tenant_id ON travel_debriefs(tenant_id);
