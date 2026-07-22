-- +goose Up
-- +goose StatementBegin

-- 00014: 章节人工审核（HIL approve/reject）+ 工作流 awaiting_review 暂停点
-- 对应前端 4 步向导"审核"环节与 docs/architecture/bid-workflow.md

-- chapter_specs: 人工审核字段
ALTER TABLE chapter_specs ADD COLUMN IF NOT EXISTS approved_at   TIMESTAMPTZ;
ALTER TABLE chapter_specs ADD COLUMN IF NOT EXISTS approved_by   UUID;
ALTER TABLE chapter_specs ADD COLUMN IF NOT EXISTS rejection_reason TEXT;

-- 扩展 chapter_specs.status 约束以包含 'approved'
ALTER TABLE chapter_specs DROP CONSTRAINT IF EXISTS chapter_specs_status_check;
ALTER TABLE chapter_specs ADD CONSTRAINT chapter_specs_status_check
    CHECK (status IN ('planned','pending','running','succeeded','failed','skipped','approved'));

-- workflows: 增加 awaiting_review 审核暂停点（generating -> awaiting_review -> auditing）
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS workflows_status_check;
ALTER TABLE workflows ADD CONSTRAINT workflows_status_check
    CHECK (status IN ('pending','parsing','outlining','facts','generating','awaiting_review','auditing','exporting','done','failed','cancelled','paused'));

-- bid_jobs: 对齐状态枚举（补 facts / awaiting_review）
ALTER TABLE bid_jobs DROP CONSTRAINT IF EXISTS bid_jobs_status_check;
ALTER TABLE bid_jobs ADD CONSTRAINT bid_jobs_status_check
    CHECK (status IN ('pending','parsing','outlining','facts','generating','awaiting_review','auditing','exporting','done','failed','cancelled','paused'));

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE bid_jobs DROP CONSTRAINT IF EXISTS bid_jobs_status_check;
ALTER TABLE bid_jobs ADD CONSTRAINT bid_jobs_status_check
    CHECK (status IN ('pending','parsing','outlining','generating','auditing','exporting','done','failed','cancelled','paused'));
ALTER TABLE workflows DROP CONSTRAINT IF EXISTS workflows_status_check;
ALTER TABLE workflows ADD CONSTRAINT workflows_status_check
    CHECK (status IN ('pending','parsing','outlining','facts','generating','auditing','exporting','done','failed','cancelled','paused'));
ALTER TABLE chapter_specs DROP CONSTRAINT IF EXISTS chapter_specs_status_check;
ALTER TABLE chapter_specs ADD CONSTRAINT chapter_specs_status_check
    CHECK (status IN ('planned','pending','running','succeeded','failed','skipped'));
ALTER TABLE chapter_specs DROP COLUMN IF EXISTS rejection_reason;
ALTER TABLE chapter_specs DROP COLUMN IF EXISTS approved_by;
ALTER TABLE chapter_specs DROP COLUMN IF EXISTS approved_at;
-- +goose StatementEnd
