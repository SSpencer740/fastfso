-- 000029 was supposed to switch sessions.user_id's FK to ON DELETE CASCADE,
-- but on staging the constraint is still rejecting deletes with the original
-- restrict behavior. Cause is murky (constraint name mismatch, golang-migrate
-- dirty state, or the original DROP no-op'd) — rather than diagnose, this
-- migration is defensive: it loops over every FK from sessions to users and
-- drops them, then recreates exactly one with the cascade behavior we want.
-- Idempotent and safe to run on databases that already have the right state.

DO $$
DECLARE
    c text;
BEGIN
    FOR c IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = 'sessions'::regclass
          AND contype = 'f'
          AND confrelid = 'users'::regclass
    LOOP
        EXECUTE format('ALTER TABLE sessions DROP CONSTRAINT %I', c);
    END LOOP;
END $$;

ALTER TABLE sessions
    ADD CONSTRAINT sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
