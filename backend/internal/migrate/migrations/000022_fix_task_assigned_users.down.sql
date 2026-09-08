DROP VIEW IF EXISTS task_user_status;
DROP VIEW IF EXISTS task_assigned_users;

CREATE VIEW task_assigned_users AS
    SELECT
        r.task_id,
        r.target_id AS user_id,
        r.rule_type
    FROM task_assignment_rules r
    WHERE r.rule_type = 'user' AND r.target_id IS NOT NULL

    UNION

    SELECT
        r.task_id,
        us.user_id,
        r.rule_type
    FROM task_assignment_rules r
    JOIN user_suborganizations us ON us.suborganization_id = r.target_id
    WHERE r.rule_type = 'sub_org' AND r.target_id IS NOT NULL

    UNION

    SELECT
        r.task_id,
        u.id AS user_id,
        r.rule_type
    FROM task_assignment_rules r
    JOIN tasks t ON t.id = r.task_id
    JOIN users u ON u.tenant_id = t.tenant_id
    WHERE r.rule_type = 'org';

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
