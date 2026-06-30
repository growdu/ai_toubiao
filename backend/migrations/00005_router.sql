-- 00005_router.sql — AI router call logs and quality scores
-- See docs/architecture/ai-router.md and docs/architecture/data-model.md

BEGIN;

-- ============================================================================
-- router_call_logs — every AI provider invocation
-- ============================================================================
CREATE TABLE router_call_logs (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID NOT NULL,
    workflow_id       UUID,
    step_id           UUID,
    task              VARCHAR(64)  NOT NULL,
    provider          VARCHAR(32)  NOT NULL,
    model             VARCHAR(64)  NOT NULL,
    prompt_tokens     INTEGER NOT NULL DEFAULT 0,
    completion_tokens INTEGER NOT NULL DEFAULT 0,
    latency_ms        INTEGER NOT NULL DEFAULT 0,
    cost_usd          NUMERIC(10, 6) NOT NULL DEFAULT 0,
    cache_hit         BOOLEAN NOT NULL DEFAULT FALSE,
    fallback_used     BOOLEAN NOT NULL DEFAULT FALSE,
    attempt           SMALLINT NOT NULL DEFAULT 1,
    error             TEXT,
    metadata          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_router_call_logs_tenant_time
    ON router_call_logs(tenant_id, created_at DESC);

CREATE INDEX idx_router_call_logs_task
    ON router_call_logs(tenant_id, task, created_at DESC);

CREATE INDEX idx_router_call_logs_workflow
    ON router_call_logs(workflow_id, created_at DESC)
    WHERE workflow_id IS NOT NULL;

-- ============================================================================
-- router_route_configs — active routing rules (override-able per tenant later)
-- ============================================================================
CREATE TABLE router_route_configs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID,                -- NULL = global default
    task        VARCHAR(64) NOT NULL,
    config      JSONB NOT NULL,      -- RouteConfig JSON
    version     INTEGER NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, task)
);

CREATE INDEX idx_router_route_configs_tenant
    ON router_route_configs(tenant_id)
    WHERE tenant_id IS NOT NULL;

-- ============================================================================
-- router_tenant_budgets — monthly spend cap per tenant per task
-- ============================================================================
CREATE TABLE router_tenant_budgets (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID NOT NULL,
    task             VARCHAR(64) NOT NULL,        -- or '*' for global cap
    monthly_cap_usd  NUMERIC(10, 4) NOT NULL DEFAULT 100.0,
    period_start     DATE NOT NULL DEFAULT DATE_TRUNC('month', NOW()),
    spent_usd        NUMERIC(10, 6) NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, task)
);

CREATE INDEX idx_router_tenant_budgets_tenant
    ON router_tenant_budgets(tenant_id);

-- ============================================================================
-- router_quality_scores — daily aggregate per (provider, model, task)
-- ============================================================================
CREATE TABLE router_quality_scores (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider         VARCHAR(32) NOT NULL,
    model            VARCHAR(64) NOT NULL,
    task             VARCHAR(64) NOT NULL,
    date             DATE NOT NULL,
    sample_count     INTEGER NOT NULL DEFAULT 0,
    success_count    INTEGER NOT NULL DEFAULT 0,
    cache_hits       INTEGER NOT NULL DEFAULT 0,
    avg_latency_ms   INTEGER NOT NULL DEFAULT 0,
    total_cost_usd   NUMERIC(10, 6) NOT NULL DEFAULT 0,
    error_count      INTEGER NOT NULL DEFAULT 0,
    quality_score    NUMERIC(5, 4),                 -- 0..1, NULL = unrated
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, model, task, date)
);

CREATE INDEX idx_router_quality_scores_lookup
    ON router_quality_scores(task, date DESC);

COMMIT;