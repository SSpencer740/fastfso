CREATE TABLE wiki_posts (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id          UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    created_by_user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title              TEXT NOT NULL,
    content            TEXT NOT NULL DEFAULT '',
    published          BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_wiki_posts_tenant_id ON wiki_posts(tenant_id);
CREATE INDEX idx_wiki_posts_published ON wiki_posts(tenant_id, published);
