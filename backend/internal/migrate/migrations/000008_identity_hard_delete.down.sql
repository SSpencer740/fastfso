-- Revert FK constraints to their original behavior (NO ACTION).

ALTER TABLE auth_audit_log
    DROP CONSTRAINT auth_audit_log_identity_id_fkey,
    ADD CONSTRAINT auth_audit_log_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities(id);

ALTER TABLE sessions
    DROP CONSTRAINT sessions_revoked_by_fkey,
    ADD CONSTRAINT sessions_revoked_by_fkey
        FOREIGN KEY (revoked_by) REFERENCES identities(id);

ALTER TABLE users
    DROP CONSTRAINT users_identity_id_fkey,
    ADD CONSTRAINT users_identity_id_fkey
        FOREIGN KEY (identity_id) REFERENCES identities(id);
