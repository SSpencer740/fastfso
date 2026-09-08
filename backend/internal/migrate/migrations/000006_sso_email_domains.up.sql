CREATE TABLE sso_email_domains (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    domain TEXT NOT NULL UNIQUE,
    sso_configuration_id UUID NOT NULL REFERENCES sso_configurations(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_sso_email_domains_domain ON sso_email_domains(domain);
