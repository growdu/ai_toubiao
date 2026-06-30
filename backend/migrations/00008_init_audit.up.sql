-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS audit_reports (
    id UUID PRIMARY KEY,
    bid_job_id UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    critical INT NOT NULL DEFAULT 0,
    major INT NOT NULL DEFAULT 0,
    minor INT NOT NULL DEFAULT 0,
    total_issues INT NOT NULL DEFAULT 0,
    passed BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_reports_bid_job_id ON audit_reports(bid_job_id);
CREATE INDEX idx_audit_reports_tenant_id ON audit_reports(tenant_id);

CREATE TABLE IF NOT EXISTS audit_issues (
    id UUID PRIMARY KEY,
    report_id UUID NOT NULL REFERENCES audit_reports(id) ON DELETE CASCADE,
    bid_job_id UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    chapter_id UUID REFERENCES chapter_specs(id) ON DELETE SET NULL,
    chapter_title VARCHAR(500) NOT NULL,
    severity VARCHAR(20) NOT NULL,
    dimension VARCHAR(50) NOT NULL,
    issue TEXT NOT NULL,
    suggestion TEXT,
    evidence TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    resolved_by UUID,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_issues_report_id ON audit_issues(report_id);
CREATE INDEX idx_audit_issues_bid_job_id ON audit_issues(bid_job_id);
CREATE INDEX idx_audit_issues_tenant_id ON audit_issues(tenant_id);
CREATE INDEX idx_audit_issues_severity ON audit_issues(severity);
CREATE INDEX idx_audit_issues_status ON audit_issues(status);

COMMENT ON TABLE audit_reports IS 'Audit reports for bid jobs';
COMMENT ON TABLE audit_issues IS 'Individual issues found during audit';
COMMENT ON COLUMN audit_issues.severity IS 'critical, major, minor';
COMMENT ON COLUMN audit_issues.dimension IS 'compliance, consistency, completeness, format, accuracy';
COMMENT ON COLUMN audit_issues.status IS 'open, accepted, rejected, resolved';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_issues;
DROP TABLE IF EXISTS audit_reports;
-- +goose StatementEnd
