-- Fix: deleting an identity cascades to sessions, but auth_audit_log.session_id
-- blocked session deletion. SET NULL preserves the audit trail.

ALTER TABLE auth_audit_log
    DROP CONSTRAINT auth_audit_log_session_id_fkey,
    ADD CONSTRAINT auth_audit_log_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE SET NULL;
