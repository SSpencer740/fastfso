-- Recreate task_assigned_users to restrict sub_org and org assignments to
-- individual_contributor role only (FSO/admin roles should not receive tasks).
DROP VIEW IF EXISTS task_user_status;
DROP VIEW IF EXISTS task_assigned_users;

CREATE VIEW task_assigned_users AS
    -- Direct user assignment
    SELECT
        r.task_id,
        r.target_id AS user_id,
        r.rule_type
    FROM task_assignment_rules r
    WHERE r.rule_type = 'user' AND r.target_id IS NOT NULL

    UNION

    -- Sub-org assignment: individual_contributor users in the suborganization
    SELECT
        r.task_id,
        us.user_id,
        r.rule_type
    FROM task_assignment_rules r
    JOIN user_suborganizations us ON us.suborganization_id = r.target_id
    JOIN users u ON u.id = us.user_id
    WHERE r.rule_type = 'sub_org' AND r.target_id IS NOT NULL
      AND u.role = 'individual_contributor'

    UNION

    -- Org-wide assignment: individual_contributor users in the task's tenant
    SELECT
        r.task_id,
        u.id AS user_id,
        r.rule_type
    FROM task_assignment_rules r
    JOIN tasks t ON t.id = r.task_id
    JOIN users u ON u.tenant_id = t.tenant_id
    WHERE r.rule_type = 'org'
      AND u.role = 'individual_contributor';

CREATE VIEW task_user_status AS
    SELECT
        tau.task_id,
        tau.user_id,
        tau.rule_type,
        COALESCE(tc.id, '00000000-0000-0000-0000-000000000000') AS completion_id,
        COALESCE(tc.status, 'to_do') AS status,
        tc.viewed_at,
        tc.submitted_at,
        tc.reviewed_at,
        tc.reviewed_by
    FROM task_assigned_users tau
    LEFT JOIN task_completions tc ON tc.task_id = tau.task_id AND tc.user_id = tau.user_id;
