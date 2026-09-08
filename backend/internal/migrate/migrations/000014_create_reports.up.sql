CREATE TABLE reports (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id            UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id   UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reporting_for        TEXT NOT NULL CHECK (reporting_for IN ('self', 'other', 'fcl')),
    subject_name         TEXT NOT NULL DEFAULT '',
    report_type          TEXT NOT NULL,
    details              TEXT NOT NULL,
    status               TEXT NOT NULL DEFAULT 'unreviewed'
                         CHECK (status IN ('unreviewed', 'under_review', 'processed')),
    reviewed_by          UUID REFERENCES users(id),
    reviewed_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_reports_tenant_id        ON reports(tenant_id);
CREATE INDEX idx_reports_created_by       ON reports(created_by_user_id);
CREATE INDEX idx_reports_status           ON reports(status);
