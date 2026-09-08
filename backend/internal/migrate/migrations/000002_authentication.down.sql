DROP TABLE IF EXISTS auth_audit_log;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS sso_configurations;
DROP TABLE IF EXISTS email_codes;
DROP TABLE IF EXISTS totp_secrets;
DROP TABLE IF EXISTS passkeys;
ALTER TABLE users DROP COLUMN IF EXISTS identity_id;
DROP TABLE IF EXISTS identities;
