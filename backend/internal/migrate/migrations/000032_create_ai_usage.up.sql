-- Tracks AI invocations across features (chat, future summarization, etc.).
-- One row per AI call. Token counts may be 0 if the provider did not return
-- usage metadata. Used for per-user rate enforcement and per-tenant billing
-- rollups.
CREATE TABLE ai_usage (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feature         TEXT NOT NULL,
    prompt_tokens   INTEGER NOT NULL DEFAULT 0,
    response_tokens INTEGER NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Supports per-user/per-feature enforcement queries: COUNT WHERE user_id = ?
-- AND feature = ? AND created_at >= ?
CREATE INDEX idx_ai_usage_user_feature_created
    ON ai_usage (user_id, feature, created_at DESC);

-- Supports per-tenant billing rollups: SUM(tokens) WHERE tenant_id = ? AND
-- created_at BETWEEN ?
CREATE INDEX idx_ai_usage_tenant_created
    ON ai_usage (tenant_id, created_at DESC);
