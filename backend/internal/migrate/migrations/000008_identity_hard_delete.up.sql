-- Allow hard-deleting identities by adjusting FK constraints.

-- auth_audit_log.identity_id → SET NULL (preserve audit trail)
ALTER TABLE auth_audit_log
    DROP CONSTRAINT auth_audit_log_identity_id_fkey,
    ADD CONSTRAINT auth_audit_log_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities(id) ON DELETE SET NULL;

-- sessions.revoked_by → SET NULL (already nullable)
ALTER TABLE sessions
    DROP CONSTRAINT sessions_revoked_by_fkey,
    ADD CONSTRAINT sessions_revoked_by_fkey
        FOREIGN KEY (revoked_by) REFERENCES identities(id) ON DELETE SET NULL;

-- users.identity_id → CASCADE (remove user rows when identity is deleted)
ALTER TABLE users
    DROP CONSTRAINT users_identity_id_fkey,
    ADD CONSTRAINT users_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities(id) ON DELETE CASCADE;
