-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_chapter_contents_spec_latest;
ALTER TABLE chapter_contents
    DROP COLUMN IF EXISTS content_text,
    DROP COLUMN IF EXISTS llm_task;

-- +goose StatementEnd
