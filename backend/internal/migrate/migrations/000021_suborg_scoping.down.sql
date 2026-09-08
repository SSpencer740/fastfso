ALTER TABLE sessions DROP COLUMN IF EXISTS sub_org_id, DROP COLUMN IF EXISTS user_role;
ALTER TABLE reports DROP COLUMN IF EXISTS sub_org_id;
ALTER TABLE wiki_posts DROP COLUMN IF EXISTS sub_org_id;
ALTER TABLE visit_requests DROP COLUMN IF EXISTS sub_org_id;
ALTER TABLE travel_reports DROP COLUMN IF EXISTS sub_org_id;
ALTER TABLE action_items DROP COLUMN IF EXISTS sub_org_id;
ALTER TABLE tasks DROP COLUMN IF EXISTS sub_org_id;
