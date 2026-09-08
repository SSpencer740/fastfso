CREATE TABLE user_settings (
    user_id                UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    notification_frequency TEXT NOT NULL DEFAULT 'every_task'
                           CHECK (notification_frequency IN ('every_task', 'daily_summary')),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);
