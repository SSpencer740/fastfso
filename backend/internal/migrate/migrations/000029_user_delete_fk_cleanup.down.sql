-- Restore the original FK behavior. We don't restore NOT NULL on
-- tasks.created_by_user_id because rows may already have NULL by the time
-- this rolls back; the down migration would fail. Operators rolling back
-- need to backfill those rows manually first if they want NOT NULL back.

ALTER TABLE sessions DROP CONSTRAINT sessions_user_id_fkey;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id);

ALTER TABLE tasks DROP CONSTRAINT tasks_created_by_user_id_fkey;
ALTER TABLE tasks
    ADD CONSTRAINT tasks_created_by_user_id_fkey
    FOREIGN KEY (created_by_user_id) REFERENCES users(id);

ALTER TABLE task_completions DROP CONSTRAINT task_completions_reviewed_by_fkey;
ALTER TABLE task_completions
    ADD CONSTRAINT task_completions_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id);

ALTER TABLE action_items DROP CONSTRAINT action_items_assigned_to_fkey;
ALTER TABLE action_items
    ADD CONSTRAINT action_items_assigned_to_fkey
    FOREIGN KEY (assigned_to) REFERENCES users(id);

ALTER TABLE travel_reports DROP CONSTRAINT travel_reports_reviewed_by_fkey;
ALTER TABLE travel_reports
    ADD CONSTRAINT travel_reports_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id);

ALTER TABLE visit_requests DROP CONSTRAINT visit_requests_reviewed_by_fkey;
ALTER TABLE visit_requests
    ADD CONSTRAINT visit_requests_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id);

ALTER TABLE reports DROP CONSTRAINT reports_reviewed_by_fkey;
ALTER TABLE reports
    ADD CONSTRAINT reports_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id);
