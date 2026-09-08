-- Rename email_verified → activated
ALTER TABLE identities RENAME COLUMN email_verified TO activated;

-- Update email_codes.purpose CHECK: drop 'email_verification', add 'invite'
ALTER TABLE email_codes DROP CONSTRAINT email_codes_purpose_check;
ALTER TABLE email_codes ADD CONSTRAINT email_codes_purpose_check
    CHECK (purpose IN ('2fa', 'password_reset', 'invite'));

-- Add 'invite' to sessions.auth_method CHECK
ALTER TABLE sessions DROP CONSTRAINT sessions_auth_method_check;
ALTER TABLE sessions ADD CONSTRAINT sessions_auth_method_check
    CHECK (auth_method IN ('password', 'passkey', 'saml', 'oidc', 'invite'));

-- Data fixup: mark existing identities with passwords as activated
UPDATE identities SET activated = TRUE WHERE password_hash IS NOT NULL;
