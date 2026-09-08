CREATE TABLE visit_requests (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id  UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    destination_name    TEXT NOT NULL,
    diss_smo_code       TEXT NOT NULL,
    visit_address       TEXT NOT NULL,
    visit_start_date    DATE NOT NULL,
    visit_end_date      DATE NOT NULL,
    access_level        TEXT NOT NULL
                        CHECK (access_level IN ('confidential', 'secret', 'top_secret', 'top_secret_sci')),
    visit_description   TEXT NOT NULL,
    poc_name            TEXT NOT NULL,
    poc_email           TEXT NOT NULL,
    poc_phone           TEXT NOT NULL,
    security_poc_name   TEXT NOT NULL,
    security_poc_email  TEXT NOT NULL,
    security_poc_phone  TEXT NOT NULL,
    status              TEXT NOT NULL DEFAULT 'submitted'
                        CHECK (status IN ('submitted', 'under_review', 'approved', 'rejected', 'cancelled')),
    cloned_from_id      UUID REFERENCES visit_requests(id),
    reviewed_by         UUID REFERENCES users(id),
    reviewed_at         TIMESTAMPTZ,
    reviewer_notes      TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_visit_requests_tenant_id ON visit_requests(tenant_id);
CREATE INDEX idx_visit_requests_status ON visit_requests(status);
CREATE INDEX idx_visit_requests_created_by ON visit_requests(created_by_user_id);
