-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_issues;
DROP TABLE IF EXISTS audit_reports;
-- +goose StatementEnd
