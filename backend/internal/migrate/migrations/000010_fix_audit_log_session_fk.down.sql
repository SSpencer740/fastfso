ALTER TABLE auth_audit_log
    DROP CONSTRAINT auth_audit_log_session_id_fkey,
    ADD CONSTRAINT auth_audit_log_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES sessions(id);
