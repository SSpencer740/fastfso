-- Tasks: the main task entity
CREATE TABLE tasks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id),
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    priority        TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    due_date        DATE,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'active', 'archived')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_tasks_tenant_id ON tasks(tenant_id);
CREATE INDEX idx_tasks_status ON tasks(tenant_id, status);

-- Task requirements: defines what the assignee must fill out
CREATE TABLE task_requirements (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    kind        TEXT NOT NULL CHECK (kind IN ('text', 'textarea', 'file_upload', 'checkbox')),
    label       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    required    BOOLEAN NOT NULL DEFAULT true,
    sort_order  INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_requirements_task_id ON task_requirements(task_id);

-- Task assignment rules: intent-based assignment
-- rule_type 'user' → target_id is a user.id
-- rule_type 'sub_org' → target_id is a suborganization.id
-- rule_type 'org' → target_id is NULL (means all users in tenant)
-- rule_type 'group' → target_id is a group.id (future)
CREATE TABLE task_assignment_rules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    rule_type   TEXT NOT NULL CHECK (rule_type IN ('user', 'sub_org', 'org')),
    target_id   UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_assignment_rules_task_id ON task_assignment_rules(task_id);

-- Task completions: tracks each user's progress on a task
CREATE TABLE task_completions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'to_do' CHECK (status IN ('to_do', 'in_progress', 'submitted', 'approved', 'rejected')),
    viewed_at    TIMESTAMPTZ,
    submitted_at TIMESTAMPTZ,
    reviewed_at  TIMESTAMPTZ,
    reviewed_by  UUID REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_id, user_id)
);
CREATE INDEX idx_task_completions_task_id ON task_completions(task_id);
CREATE INDEX idx_task_completions_user_id ON task_completions(user_id);

-- Task responses: the data the user submits for each requirement
CREATE TABLE task_responses (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    completion_id  UUID NOT NULL REFERENCES task_completions(id) ON DELETE CASCADE,
    requirement_id UUID NOT NULL REFERENCES task_requirements(id) ON DELETE CASCADE,
    text_value     TEXT,
    bool_value     BOOLEAN,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (completion_id, requirement_id)
);
CREATE INDEX idx_task_responses_completion_id ON task_responses(completion_id);

-- Task file uploads
CREATE TABLE task_uploads (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    completion_id  UUID NOT NULL REFERENCES task_completions(id) ON DELETE CASCADE,
    requirement_id UUID NOT NULL REFERENCES task_requirements(id) ON DELETE CASCADE,
    file_name      TEXT NOT NULL,
    file_size      BIGINT NOT NULL,
    content_type   TEXT NOT NULL,
    storage_key    TEXT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_task_uploads_completion_id ON task_uploads(completion_id);

-- Action items: unified work queue for admins
CREATE TABLE action_items (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    source_type TEXT NOT NULL CHECK (source_type IN (
        'task_submission', 'visit_request', 'travel_report',
        'incident_report', 'clearance_renewal', 'sf86_submission'
    )),
    source_id   UUID,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority    TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'under_review', 'processed', 'rejected')),
    assigned_to UUID REFERENCES users(id),
    notes       TEXT NOT NULL DEFAULT '',
    due_date    DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_action_items_tenant_id ON action_items(tenant_id);
CREATE INDEX idx_action_items_status ON action_items(tenant_id, status);
CREATE INDEX idx_action_items_assigned_to ON action_items(assigned_to);

-- View: resolves assignment rules to actual user IDs
-- UNION of user/sub_org/org rules
CREATE VIEW task_assigned_users AS
    -- Direct user assignment
    SELECT
        r.task_id,
        r.target_id AS user_id,
        r.rule_type
    FROM task_assignment_rules r
    WHERE r.rule_type = 'user' AND r.target_id IS NOT NULL

    UNION

    -- Sub-org assignment: all users in the suborganization
    SELECT
        r.task_id,
        us.user_id,
        r.rule_type
    FROM task_assignment_rules r
    JOIN user_suborganizations us ON us.suborganization_id = r.target_id
    WHERE r.rule_type = 'sub_org' AND r.target_id IS NOT NULL

    UNION

    -- Org-wide assignment: all users in the task's tenant
    SELECT
        r.task_id,
        u.id AS user_id,
        r.rule_type
    FROM task_assignment_rules r
    JOIN tasks t ON t.id = r.task_id
    JOIN users u ON u.tenant_id = t.tenant_id
    WHERE r.rule_type = 'org';

-- View: combines assignment resolution with completion status
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
