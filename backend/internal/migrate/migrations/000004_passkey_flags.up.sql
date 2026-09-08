ALTER TABLE passkeys
    ADD COLUMN backup_eligible BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN backup_state    BOOLEAN NOT NULL DEFAULT false;
