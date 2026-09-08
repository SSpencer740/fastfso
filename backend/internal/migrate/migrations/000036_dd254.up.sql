-- DD254: contract security classification specifications.
--
-- A DD254 is a per-contract document issued by a Government Contracting
-- Activity (or prime) specifying the access a contractor is authorized to.
-- One contractor (tenant) may carry many DD254s. Users are "read-on" to
-- specific DD254s via dd254_user_access.
--
-- CUI / CMMC L1 boundary
-- =====================
-- fastFSO is CMMC L1 self-attested (FAR 52.204-21, FCI scope). It is NOT
-- authorized to store CUI, FOUO, or PROPIN. The markings column is enforced
-- to 'unclassified' via a CHECK constraint. Belt-and-suspenders with the
-- upload-time attestation in the UI. If we ever move into an Assured
-- Workloads boundary, the CHECK can be relaxed.

CREATE TYPE dd254_status AS ENUM ('active', 'expired', 'superseded');

CREATE TABLE dd254_forms (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sub_org_id          UUID REFERENCES suborganizations(id) ON DELETE SET NULL,
    contract_number     TEXT NOT NULL,
    prime_contractor    TEXT NOT NULL DEFAULT '',
    classification_max  clearance_level NOT NULL,
    period_start        DATE,
    period_end          DATE,
    storage_key         TEXT NOT NULL,
    filename            TEXT NOT NULL,
    content_type        TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL,
    markings            TEXT NOT NULL DEFAULT 'unclassified'
                        CHECK (markings = 'unclassified'),
    cui_attestation_by  UUID NOT NULL REFERENCES users(id),
    cui_attestation_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status              dd254_status NOT NULL DEFAULT 'active',
    supersedes_id       UUID REFERENCES dd254_forms(id),
    uploaded_by         UUID NOT NULL REFERENCES users(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_dd254_tenant_status ON dd254_forms(tenant_id, status);
CREATE INDEX idx_dd254_tenant_suborg ON dd254_forms(tenant_id, sub_org_id) WHERE status = 'active';

CREATE TABLE dd254_user_access (
    dd254_id     UUID NOT NULL REFERENCES dd254_forms(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    briefed_at   DATE,
    debriefed_at DATE,
    added_by     UUID NOT NULL REFERENCES users(id),
    added_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (dd254_id, user_id)
);

CREATE INDEX idx_dd254_user_access_user ON dd254_user_access(user_id);

-- Visit requests gain an optional DD254 link, populated by the FSO during
-- review. SET NULL on delete so a DD254 cleanup never breaks visit history.
ALTER TABLE visit_requests
    ADD COLUMN dd_254_id UUID REFERENCES dd254_forms(id) ON DELETE SET NULL;
