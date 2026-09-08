DO $$
BEGIN
  ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_state_check;
END $$;

ALTER TABLE sessions
  ADD CONSTRAINT sessions_state_check
  CHECK (state IN ('pre_auth', 'pre_tenant', 'authenticated', 'setup_2fa'));
