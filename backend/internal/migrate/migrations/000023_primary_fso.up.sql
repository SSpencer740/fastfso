ALTER TABLE suborganizations
    ADD COLUMN primary_fso_user_id uuid REFERENCES users(id) ON DELETE SET NULL;
