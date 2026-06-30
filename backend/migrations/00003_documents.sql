-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- Documents (the document-svc root aggregate)
-- ============================================================================
CREATE TABLE documents (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id      UUID NOT NULL,                       -- FK to projects in project-svc (no cross-service FK)
    name            TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 512),
    kind            TEXT NOT NULL
                        CHECK (kind IN ('tender','proposal','spec','attachment','reference')),
    mime_type       TEXT NOT NULL CHECK (mime_type ~ '^[a-z]+/[a-z0-9.+-]+$'),
    size_bytes      BIGINT NOT NULL CHECK (size_bytes >= 0),
    storage_key     TEXT NOT NULL,                       -- S3/MinIO object key
    checksum_sha256 CHAR(64) NOT NULL CHECK (checksum_sha256 ~ '^[0-9a-fA-F]{64}$'),
    status          TEXT NOT NULL DEFAULT 'uploading'
                        CHECK (status IN ('uploading','ready','parsing','parsed','failed','deleted')),
    parse_status    JSONB,                                -- {progress, error, page_count, ...}
    metadata        JSONB NOT NULL DEFAULT '{}',
    uploaded_by     UUID NOT NULL REFERENCES users(id),
    version         INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cross-tenant access is the #1 thing we want to prevent.
CREATE INDEX idx_documents_tenant_project ON documents(tenant_id, project_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_tenant_status ON documents(tenant_id, status)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_documents_tenant_id_desc ON documents(tenant_id, id DESC)
    WHERE deleted_at IS NULL;  -- cursor pagination
CREATE INDEX idx_documents_checksum ON documents(checksum_sha256)
    WHERE deleted_at IS NULL;  -- dedup

CREATE TRIGGER trg_documents_updated BEFORE UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_documents_updated ON documents;
DROP TABLE IF EXISTS documents;
-- +goose StatementEnd