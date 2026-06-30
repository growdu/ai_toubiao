-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS notification_logs;
DROP TABLE IF EXISTS notification_preferences;
-- +goose StatementEnd
