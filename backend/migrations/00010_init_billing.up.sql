-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS billing_budgets (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    month VARCHAR(7) NOT NULL,
    limit_cents BIGINT NOT NULL DEFAULT 0,
    spent_cents BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (tenant_id, month)
);

CREATE INDEX idx_billing_budgets_tenant_id ON billing_budgets(tenant_id);
CREATE INDEX idx_billing_budgets_month ON billing_budgets(month);

CREATE TABLE IF NOT EXISTS billing_transactions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    budget_id UUID REFERENCES billing_budgets(id) ON DELETE SET NULL,
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(100) NOT NULL,
    task_type VARCHAR(50) NOT NULL,
    input_tokens INT NOT NULL DEFAULT 0,
    output_tokens INT NOT NULL DEFAULT 0,
    cost_cents BIGINT NOT NULL DEFAULT 0,
    call_id VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_billing_transactions_tenant_id ON billing_transactions(tenant_id);
CREATE INDEX idx_billing_transactions_created_at ON billing_transactions(created_at);
CREATE INDEX idx_billing_transactions_provider ON billing_transactions(provider);
CREATE INDEX idx_billing_transactions_task_type ON billing_transactions(task_type);

COMMENT ON TABLE billing_budgets IS 'Monthly spending budgets per tenant';
COMMENT ON TABLE billing_transactions IS 'AI API call transaction records';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS billing_transactions;
DROP TABLE IF EXISTS billing_budgets;
-- +goose StatementEnd
