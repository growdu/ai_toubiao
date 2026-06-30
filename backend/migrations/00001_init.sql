-- +goose Up
-- +goose StatementBegin

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- Tenants (organizations using BidWriter)
-- ============================================================================
CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    plan        TEXT NOT NULL DEFAULT 'free'
                    CHECK (plan IN ('free','pro','enterprise')),
    status      TEXT NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','suspended','deleted')),
    settings    JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================================
-- Users
-- ============================================================================
CREATE TABLE users (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id      UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email          TEXT NOT NULL,
    password_hash  TEXT NOT NULL,
    display_name   TEXT NOT NULL,
    role           TEXT NOT NULL DEFAULT 'member'
                       CHECK (role IN ('owner','admin','member','viewer')),
    status         TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active','invited','suspended','deleted')),
    last_login_at  TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, email)
);

CREATE INDEX idx_users_tenant ON users(tenant_id) WHERE status != 'deleted';

-- ============================================================================
-- Projects (the project-svc root aggregate)
-- ============================================================================
CREATE TABLE projects (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name            TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 256),
    description     TEXT CHECK (description IS NULL OR char_length(description) <= 2000),
    industry        TEXT CHECK (industry IS NULL OR char_length(industry) <= 64),
    template_id     UUID,                       -- FK to templates (added later)
    status          TEXT NOT NULL DEFAULT 'draft'
                        CHECK (status IN ('draft','active','completed','archived')),
    estimated_value NUMERIC(18,2) CHECK (estimated_value IS NULL OR estimated_value >= 0),
    currency        CHAR(3) NOT NULL DEFAULT 'CNY',
    deadline        TIMESTAMPTZ,
    owner_id        UUID NOT NULL REFERENCES users(id),
    version         INTEGER NOT NULL DEFAULT 1 CHECK (version > 0),
    deleted_at      TIMESTAMPTZ,                -- soft delete
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- All queries MUST filter by tenant_id. Index makes this fast.
CREATE INDEX idx_projects_tenant_status ON projects(tenant_id, status)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_projects_tenant_owner  ON projects(tenant_id, owner_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_projects_tenant_id_desc ON projects(tenant_id, id DESC)
    WHERE deleted_at IS NULL;  -- supports cursor pagination

-- ============================================================================
-- Audit log (minimal — full audit is in audit-svc)
-- ============================================================================
CREATE TABLE audit_events (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   UUID NOT NULL,
    actor_id    UUID,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    resource_id UUID,
    payload     JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_tenant_time ON audit_events(tenant_id, occurred_at DESC);

-- ============================================================================
-- updated_at trigger
-- ============================================================================
CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tenants_updated  BEFORE UPDATE ON tenants
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_users_updated    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();
CREATE TRIGGER trg_projects_updated BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TRIGGER IF EXISTS trg_projects_updated ON projects;
DROP TRIGGER IF EXISTS trg_users_updated ON users;
DROP TRIGGER IF EXISTS trg_tenants_updated ON tenants;
DROP FUNCTION IF EXISTS touch_updated_at();
DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;
-- +goose StatementEnd
