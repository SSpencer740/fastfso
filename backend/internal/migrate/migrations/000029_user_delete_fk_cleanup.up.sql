-- Allow tenant administrators to remove a member without first hand-deleting
-- everything they ever touched. Several user_id foreign keys default to
-- RESTRICT, so a single completed task review or assigned action item blocks
-- the DELETE outright. Switch them to a sensible default per relationship:
--
--   sessions                — CASCADE (a deleted user's sessions are dead)
--   tasks.created_by        — SET NULL + drop NOT NULL (task survives the
--                             creator; other ICs may still be working on it)
--   *.reviewed_by           — SET NULL (preserve the record, just lose the
--                             reviewer attribution)
--   action_items.assigned_to — SET NULL (item survives, becomes unassigned)

-- sessions
ALTER TABLE sessions DROP CONSTRAINT sessions_user_id_fkey;
ALTER TABLE sessions
    ADD CONSTRAINT sessions_user_id_fkey
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- tasks
ALTER TABLE tasks ALTER COLUMN created_by_user_id DROP NOT NULL;
ALTER TABLE tasks DROP CONSTRAINT tasks_created_by_user_id_fkey;
ALTER TABLE tasks
    ADD CONSTRAINT tasks_created_by_user_id_fkey
    FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL;

-- task_completions.reviewed_by
ALTER TABLE task_completions DROP CONSTRAINT task_completions_reviewed_by_fkey;
ALTER TABLE task_completions
    ADD CONSTRAINT task_completions_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL;

-- action_items.assigned_to
ALTER TABLE action_items DROP CONSTRAINT action_items_assigned_to_fkey;
ALTER TABLE action_items
    ADD CONSTRAINT action_items_assigned_to_fkey
    FOREIGN KEY (assigned_to) REFERENCES users(id) ON DELETE SET NULL;

-- travel_reports.reviewed_by
ALTER TABLE travel_reports DROP CONSTRAINT travel_reports_reviewed_by_fkey;
ALTER TABLE travel_reports
    ADD CONSTRAINT travel_reports_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL;

-- visit_requests.reviewed_by
ALTER TABLE visit_requests DROP CONSTRAINT visit_requests_reviewed_by_fkey;
ALTER TABLE visit_requests
    ADD CONSTRAINT visit_requests_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL;

-- reports.reviewed_by
ALTER TABLE reports DROP CONSTRAINT reports_reviewed_by_fkey;
ALTER TABLE reports
    ADD CONSTRAINT reports_reviewed_by_fkey
    FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL;
