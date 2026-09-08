-- Identity layer (decoupled from tenant)
CREATE TABLE identities (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           TEXT NOT NULL,
    name            TEXT NOT NULL,
    password_hash   TEXT,
    is_super_admin  BOOLEAN NOT NULL DEFAULT FALSE,
    email_verified  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_identities_email ON identities (lower(email));

-- Link users to identities
ALTER TABLE users ADD COLUMN identity_id UUID NOT NULL REFERENCES identities(id);
CREATE UNIQUE INDEX idx_users_identity_tenant ON users (identity_id, tenant_id);

-- WebAuthn passkeys
CREATE TABLE passkeys (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id   UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    credential_id BYTEA NOT NULL UNIQUE,
    public_key    BYTEA NOT NULL,
    aaguid        BYTEA,
    sign_count    BIGINT NOT NULL DEFAULT 0,
    transports    TEXT[],
    friendly_name TEXT,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_passkeys_identity_id ON passkeys(identity_id);

-- TOTP secrets (one per identity)
CREATE TABLE totp_secrets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id UUID NOT NULL UNIQUE REFERENCES identities(id) ON DELETE CASCADE,
    secret      TEXT NOT NULL,
    verified    BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Email verification codes
CREATE TABLE email_codes (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    code_hash   TEXT NOT NULL,
    purpose     TEXT NOT NULL CHECK (purpose IN ('2fa', 'email_verification', 'password_reset')),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_email_codes_identity_id ON email_codes(identity_id);

-- SSO configuration (one per tenant)
CREATE TABLE sso_configurations (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID NOT NULL UNIQUE REFERENCES tenants(id) ON DELETE CASCADE,
    protocol       TEXT NOT NULL CHECK (protocol IN ('saml', 'oidc')),
    -- SAML fields
    entity_id      TEXT,
    sso_url        TEXT,
    certificate    TEXT,
    -- OIDC fields
    client_id      TEXT,
    client_secret  TEXT,
    issuer_url     TEXT,
    -- Common fields
    enabled        BOOLEAN NOT NULL DEFAULT FALSE,
    auto_provision BOOLEAN NOT NULL DEFAULT FALSE,
    default_role   TEXT NOT NULL DEFAULT 'individual_contributor' CHECK (default_role IN ('administrator', 'fso', 'read_only_fso', 'individual_contributor')),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Sessions (source of truth)
CREATE TABLE sessions (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id    UUID NOT NULL REFERENCES identities(id) ON DELETE CASCADE,
    tenant_id      UUID REFERENCES tenants(id),
    user_id        UUID REFERENCES users(id),
    state          TEXT NOT NULL CHECK (state IN ('pre_auth', 'pre_tenant', 'authenticated')),
    ip_address     INET,
    user_agent     TEXT,
    auth_method    TEXT NOT NULL CHECK (auth_method IN ('password', 'passkey', 'saml', 'oidc')),
    second_factor  TEXT CHECK (second_factor IN ('totp', 'email_code', 'passkey')),
    is_super_admin BOOLEAN NOT NULL DEFAULT FALSE,
    csrf_token     TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    revoked_at     TIMESTAMPTZ,
    revoked_by     UUID REFERENCES identities(id),
    revoke_reason  TEXT
);

CREATE INDEX idx_sessions_identity_id ON sessions(identity_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at) WHERE revoked_at IS NULL;

-- Audit log
CREATE TABLE auth_audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_id UUID REFERENCES identities(id),
    session_id  UUID REFERENCES sessions(id),
    tenant_id   UUID REFERENCES tenants(id),
    action      TEXT NOT NULL,
    ip_address  INET,
    user_agent  TEXT,
    metadata    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_auth_audit_log_identity_id ON auth_audit_log(identity_id);
CREATE INDEX idx_auth_audit_log_created_at ON auth_audit_log(created_at);
