-- One verification record per task_upload. Created in 'pending' state when the
-- upload handler enqueues the Cloud Task, transitioned to 'succeeded' or
-- 'failed' by the verify worker.
--
-- tenant_id is denormalized off task_uploads so admin queries don't have to
-- join through task_completions -> tasks.
--
-- criteria_snapshot preserves the criteria text used at verification time;
-- if the FSO edits the requirement later, this record still shows what was
-- actually checked.
CREATE TABLE upload_verifications (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    upload_id          UUID NOT NULL UNIQUE REFERENCES task_uploads(id) ON DELETE CASCADE,
    tenant_id          UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    status             TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'succeeded', 'failed')),
    flagged            BOOLEAN NOT NULL DEFAULT false,
    confidence         REAL,
    type_match         BOOLEAN,
    extracted_fields   JSONB NOT NULL DEFAULT '{}',
    discrepancies      JSONB NOT NULL DEFAULT '[]',
    reasoning          TEXT NOT NULL DEFAULT '',
    error_message      TEXT NOT NULL DEFAULT '',
    criteria_snapshot  TEXT NOT NULL DEFAULT '',
    model              TEXT NOT NULL DEFAULT '',
    admin_feedback     TEXT CHECK (admin_feedback IN ('correct', 'incorrect')),
    admin_feedback_at  TIMESTAMPTZ,
    admin_feedback_by  UUID REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Per-tenant rollup of flagged items needing admin attention.
CREATE INDEX idx_upload_verifications_tenant_status
    ON upload_verifications(tenant_id, status, flagged);
