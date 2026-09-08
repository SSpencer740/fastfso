CREATE TABLE wiki_post_files (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id      UUID NOT NULL REFERENCES wiki_posts(id) ON DELETE CASCADE,
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    file_name    TEXT NOT NULL,
    storage_key  TEXT NOT NULL,
    file_size    BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_post_files_post_id ON wiki_post_files(post_id);
