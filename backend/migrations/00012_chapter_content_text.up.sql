-- +goose Up
-- +goose StatementBegin

-- Add inline content_text column to chapter_contents so workers can store
-- generated chapter content directly without requiring S3/MinIO round-trips.
-- content_path remains for the object-store path when used, but content_text
-- is the primary store for AI-generated inline content.

ALTER TABLE chapter_contents
    ADD COLUMN IF NOT EXISTS content_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS llm_task TEXT;

-- Index for quick lookup of latest content per spec.
CREATE INDEX IF NOT EXISTS idx_chapter_contents_spec_latest
    ON chapter_contents(chapter_spec_id, version DESC);

-- +goose StatementEnd
