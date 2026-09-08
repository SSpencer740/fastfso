-- Tenant-scoped audit log queries (audit.QueryByTenant) filter by tenant_id
-- and ORDER BY created_at DESC with LIMIT/OFFSET. A composite index on
-- (tenant_id, created_at DESC) supports both the filter and an index-order
-- scan for the sort, avoiding a sort step on the result set.
CREATE INDEX idx_auth_audit_log_tenant_created
    ON auth_audit_log(tenant_id, created_at DESC);
