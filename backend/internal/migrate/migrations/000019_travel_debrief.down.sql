DROP TABLE travel_debriefs;

ALTER TABLE travel_reports DROP COLUMN debrief_created_at;

ALTER TABLE action_items DROP CONSTRAINT action_items_source_type_check;
ALTER TABLE action_items ADD CONSTRAINT action_items_source_type_check
    CHECK (source_type IN (
        'task_submission', 'visit_request', 'travel_report',
        'incident_report', 'clearance_renewal', 'sf86_submission'
    ));
