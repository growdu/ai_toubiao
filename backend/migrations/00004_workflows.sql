-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- Workflows (bid-state-machine.md)
-- ============================================================================

CREATE TABLE workflows (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status       TEXT NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','parsing','outlining','facts',
                                     'generating','auditing','exporting','done',
                                     'failed','cancelled','paused')),
    current_step TEXT,
    error        TEXT,
    metadata     JSONB NOT NULL DEFAULT '{}',
    started_at   TIMESTAMPTZ,
    finished_at  TIMESTAMPTZ,
    created_by   UUID NOT NULL REFERENCES users(id),
    version      INT NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ
);

CREATE INDEX idx_workflows_tenant      ON workflows(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_workflows_project     ON workflows(project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_workflows_status      ON workflows(status) WHERE deleted_at IS NULL;

-- A project can have multiple workflows (re-runs) but only one non-terminal
CREATE UNIQUE INDEX uq_workflows_active_per_project
    ON workflows(project_id)
    WHERE status NOT IN ('done','cancelled','failed') AND deleted_at IS NULL;

-- ============================================================================
-- Workflow Steps — individual Step02-05 progress records
-- ============================================================================

CREATE TABLE workflow_steps (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id   UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name          TEXT NOT NULL CHECK (name IN ('parsing','outlining','facts','generating','auditing','exporting')),
    status        TEXT NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','running','succeeded','failed','skipped')),
    progress      INT NOT NULL DEFAULT 0 CHECK (progress BETWEEN 0 AND 100),
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    error         TEXT,
    artifacts     JSONB NOT NULL DEFAULT '{}',  -- output of the step
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workflow_id, name)
);

CREATE INDEX idx_workflow_steps_wf  ON workflow_steps(workflow_id);

-- ============================================================================
-- Workflow Events — append-only audit log of transitions
-- ============================================================================

CREATE TABLE workflow_events (
    id          BIGSERIAL PRIMARY KEY,
    workflow_id UUID NOT NULL REFERENCES workflows(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    from_state  TEXT,
    to_state    TEXT NOT NULL,
    actor_id    UUID NOT NULL REFERENCES users(id),
    reason      TEXT,
    payload     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_events_wf ON workflow_events(workflow_id, created_at DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workflow_events;
DROP TABLE IF EXISTS workflow_steps;
DROP TABLE IF EXISTS workflows;
-- +goose StatementEnd