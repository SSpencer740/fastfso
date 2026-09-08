-- Add sub_org_id to all data tables so records can be scoped to a sub-org.
-- NULL means tenant-wide (visible to all FSOs in the tenant + admins).

ALTER TABLE tasks
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL;

ALTER TABLE action_items
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL;

ALTER TABLE travel_reports
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL;

ALTER TABLE visit_requests
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL;

ALTER TABLE wiki_posts
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL;

ALTER TABLE reports
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL;

-- Cache sub-org and role on sessions to avoid per-request DB lookups.
ALTER TABLE sessions
    ADD COLUMN sub_org_id UUID REFERENCES suborganizations(id) ON DELETE SET NULL,
    ADD COLUMN user_role  TEXT;

-- Backfill IC-owned records from the owner's current sub-org.
UPDATE travel_reports tr
SET sub_org_id = (
    SELECT us.suborganization_id
    FROM user_suborganizations us
    WHERE us.user_id = tr.user_id
    LIMIT 1
)
WHERE tr.sub_org_id IS NULL;

UPDATE visit_requests vr
SET sub_org_id = (
    SELECT us.suborganization_id
    FROM user_suborganizations us
    WHERE us.user_id = vr.created_by_user_id
    LIMIT 1
)
WHERE vr.sub_org_id IS NULL;

UPDATE reports r
SET sub_org_id = (
    SELECT us.suborganization_id
    FROM user_suborganizations us
    WHERE us.user_id = r.created_by_user_id
    LIMIT 1
)
WHERE r.sub_org_id IS NULL;

-- Backfill action_items from their source records.
UPDATE action_items ai
SET sub_org_id = tr.sub_org_id
FROM travel_reports tr
WHERE ai.source_type = 'travel_report'
  AND ai.source_id = tr.id
  AND ai.sub_org_id IS NULL;

UPDATE action_items ai
SET sub_org_id = vr.sub_org_id
FROM visit_requests vr
WHERE ai.source_type = 'visit_request'
  AND ai.source_id = vr.id
  AND ai.sub_org_id IS NULL;

UPDATE action_items ai
SET sub_org_id = r.sub_org_id
FROM reports r
WHERE ai.source_type = 'report'
  AND ai.source_id = r.id
  AND ai.sub_org_id IS NULL;

UPDATE action_items ai
SET sub_org_id = tr.sub_org_id
FROM travel_debriefs td
JOIN travel_reports tr ON tr.id = td.report_id
WHERE ai.source_type = 'travel_debrief'
  AND ai.source_id = td.id
  AND ai.sub_org_id IS NULL;
