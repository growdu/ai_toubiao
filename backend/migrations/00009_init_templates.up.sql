-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS word_templates (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    kind VARCHAR(50) NOT NULL DEFAULT 'standard',
    storage_key VARCHAR(500) NOT NULL,
    size_bytes BIGINT NOT NULL DEFAULT 0,
    checksum VARCHAR(64) NOT NULL,
    version INT NOT NULL DEFAULT 1,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_word_templates_tenant_id ON word_templates(tenant_id);
CREATE INDEX idx_word_templates_kind ON word_templates(kind);
CREATE INDEX idx_word_templates_is_default ON word_templates(is_default) WHERE is_default = true;

COMMENT ON TABLE word_templates IS 'Word document templates for bid exports';
COMMENT ON COLUMN word_templates.kind IS 'standard, technical, commercial';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS word_templates;
-- +goose StatementEnd
