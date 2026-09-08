-- Track the last time we sent a reminder for each clearance record and DD254
-- form. The reminders cron uses these to avoid re-firing the same bucket every
-- day — 90-day reminders go out once, 30-day once, overdue daily.

ALTER TABLE user_clearance_records
    ADD COLUMN last_due_reminder_at TIMESTAMPTZ;

ALTER TABLE dd254_forms
    ADD COLUMN last_expiration_reminder_at TIMESTAMPTZ;
