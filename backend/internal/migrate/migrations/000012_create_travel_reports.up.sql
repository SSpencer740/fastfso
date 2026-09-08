CREATE TABLE travel_reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    trip_name       TEXT NOT NULL,
    multi_country   BOOLEAN NOT NULL DEFAULT false,
    passport_number TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'submitted', 'under_review', 'approved', 'rejected')),
    emergency_first_name TEXT NOT NULL DEFAULT '',
    emergency_last_name  TEXT NOT NULL DEFAULT '',
    emergency_phone      TEXT NOT NULL DEFAULT '',
    additional_comments  TEXT NOT NULL DEFAULT '',
    submitted_at    TIMESTAMPTZ,
    reviewed_at     TIMESTAMPTZ,
    reviewed_by     UUID REFERENCES users(id),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_travel_reports_tenant_id ON travel_reports(tenant_id);
CREATE INDEX idx_travel_reports_user_id ON travel_reports(user_id);
CREATE INDEX idx_travel_reports_status ON travel_reports(status);

CREATE TABLE travel_report_countries (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id       UUID NOT NULL REFERENCES travel_reports(id) ON DELETE CASCADE,
    country_name    TEXT NOT NULL,
    sort_order      INT NOT NULL DEFAULT 0,
    start_date      DATE,
    end_date        DATE,
    reason          TEXT NOT NULL DEFAULT ''
                    CHECK (reason IN ('', 'ngo_missionary', 'official_non_dod', 'vacation_personal', 'other')),
    transportation  TEXT[] NOT NULL DEFAULT '{}',
    has_companions      BOOLEAN NOT NULL DEFAULT false,
    companions_detail   TEXT NOT NULL DEFAULT '',
    has_foreign_contacts BOOLEAN NOT NULL DEFAULT false,
    contacts_detail     TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_travel_report_countries_report_id ON travel_report_countries(report_id);

CREATE TABLE travel_report_uploads (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id       UUID NOT NULL REFERENCES travel_reports(id) ON DELETE CASCADE,
    file_name       TEXT NOT NULL,
    file_size       BIGINT NOT NULL,
    content_type    TEXT NOT NULL,
    storage_key     TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_travel_report_uploads_report_id ON travel_report_uploads(report_id);
