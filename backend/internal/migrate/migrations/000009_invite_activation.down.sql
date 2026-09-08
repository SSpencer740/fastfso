-- Reverse sessions.auth_method CHECK
ALTER TABLE sessions DROP CONSTRAINT sessions_auth_method_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_auth_method_check
    CHECK (auth_method IN ('password', 'passkey', 'saml', 'oidc'));

-- Reverse email_codes.purpose CHECK
ALTER TABLE email_codes DROP CONSTRAINT email_codes_purpose_check;
ALTER TABLE email_codes ADD CONSTRAINT email_codes_purpose_check
    CHECK (purpose IN ('2fa', 'email_verification', 'password_reset'));

-- Rename activated → email_verified
ALTER TABLE identities RENAME COLUMN activated TO email_verified;
