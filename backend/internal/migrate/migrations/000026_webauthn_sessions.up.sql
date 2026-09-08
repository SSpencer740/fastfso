CREATE TABLE webauthn_sessions (
    id         TEXT        PRIMARY KEY,
    data       BYTEA       NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_webauthn_sessions_expires_at ON webauthn_sessions (expires_at);
