ALTER TABLE action_items
    DROP CONSTRAINT action_items_source_type_check,
    ADD CONSTRAINT action_items_source_type_check CHECK (source_type IN (
        'task_submission', 'visit_request', 'travel_report',
        'incident_report', 'clearance_renewal', 'sf86_submission', 'report'
    ));
