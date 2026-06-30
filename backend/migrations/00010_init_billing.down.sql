-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS billing_transactions;
DROP TABLE IF EXISTS billing_budgets;
-- +goose StatementEnd
