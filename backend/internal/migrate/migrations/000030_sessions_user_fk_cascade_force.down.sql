-- The down does the same defensive loop, then re-creates the FK without
-- CASCADE — matching what 000029.down would have left.
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
    FOREIGN KEY (user_id) REFERENCES users(id);
