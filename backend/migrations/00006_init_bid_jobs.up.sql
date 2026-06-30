-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- bid_jobs — 标书任务（关联 project + workflow）
-- ============================================================================
CREATE TABLE bid_jobs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    workflow_id     UUID REFERENCES workflows(id) ON DELETE SET NULL,
    rfp_document_id UUID REFERENCES documents(id) ON DELETE SET NULL,

    -- 状态
    status          TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending','parsing','outlining','generating','auditing','exporting','done','failed','cancelled','paused')),
    current_step    TEXT,
    error_message   TEXT,

    -- 解析结果（JSONB 存储结构化解析结果）
    parse_result    JSONB DEFAULT '{}',

    -- 项目信息（冗余存储，加速查询）
    project_name    TEXT,
    industry        TEXT,
    issuer          TEXT,
    bid_deadline    TIMESTAMPTZ,
    budget          NUMERIC(18, 2),

    -- 进度统计
    total_chapters  INT NOT NULL DEFAULT 0,
    done_chapters   INT NOT NULL DEFAULT 0,
    total_figures   INT NOT NULL DEFAULT 0,
    done_figures    INT NOT NULL DEFAULT 0,

    -- 版本（乐观锁）
    version         INT NOT NULL DEFAULT 1,

    -- 软删除
    deleted_at      TIMESTAMPTZ,

    -- 时间戳
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bid_jobs_tenant       ON bid_jobs(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_bid_jobs_project      ON bid_jobs(project_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_bid_jobs_workflow     ON bid_jobs(workflow_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_bid_jobs_status       ON bid_jobs(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_bid_jobs_created_desc ON bid_jobs(tenant_id, created_at DESC) WHERE deleted_at IS NULL;

-- 每个 project 同时只能有一个活跃的 bid_job
CREATE UNIQUE INDEX uq_bid_jobs_active_per_project
    ON bid_jobs(project_id)
    WHERE deleted_at IS NULL AND status NOT IN ('done','cancelled','failed');

-- ============================================================================
-- chapter_specs — 章节规格定义（树形结构，支持 3 级）
-- ============================================================================
CREATE TABLE chapter_specs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_job_id          UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    parent_id           UUID REFERENCES chapter_specs(id) ON DELETE CASCADE,

    -- 基本信息
    title               TEXT NOT NULL,
    level               SMALLINT NOT NULL CHECK (level BETWEEN 1 AND 3),
    order_index         INT NOT NULL,
    chapter_type        TEXT NOT NULL DEFAULT 'normal'
                           CHECK (chapter_type IN ('cover','toc','normal','summary','appendix')),

    -- 字数控制
    target_word_count   INT NOT NULL DEFAULT 1500,
    min_word_count      INT NOT NULL DEFAULT 800,

    -- 写作风格
    writing_style       TEXT NOT NULL DEFAULT 'formal'
                           CHECK (writing_style IN ('formal','concise','detailed')),

    -- 优先级
    priority            TEXT NOT NULL DEFAULT 'normal'
                           CHECK (priority IN ('critical','high','normal','low')),

    -- 依赖关系（章节依赖图）
    dependencies        UUID[] NOT NULL DEFAULT '{}',

    -- 状态
    status              TEXT NOT NULL DEFAULT 'planned'
                           CHECK (status IN ('planned','pending','running','succeeded','failed','skipped')),

    -- 错误信息
    error_message       TEXT,
    retry_count         INT NOT NULL DEFAULT 0,

    -- 乐观锁
    version             INT NOT NULL DEFAULT 1,

    -- 时间戳
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_chapter_specs_bid         ON chapter_specs(bid_job_id);
CREATE INDEX idx_chapter_specs_bid_order   ON chapter_specs(bid_job_id, order_index);
CREATE INDEX idx_chapter_specs_bid_status  ON chapter_specs(bid_job_id, status);
CREATE INDEX idx_chapter_specs_parent      ON chapter_specs(parent_id);

-- ============================================================================
-- chapter_contents — 章节内容版本化存储
-- ============================================================================
CREATE TABLE chapter_contents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_spec_id UUID NOT NULL REFERENCES chapter_specs(id) ON DELETE CASCADE,
    version         INT NOT NULL DEFAULT 1,

    -- 内容存储（文件路径，内容在 S3/MinIO）
    content_path    TEXT NOT NULL,
    content_hash    CHAR(64),            -- SHA-256，用于去重

    -- 统计
    word_count      INT NOT NULL DEFAULT 0,
    min_word_met    BOOLEAN NOT NULL DEFAULT FALSE,  -- 是否满足最低字数要求

    -- 生成信息
    generated_by    TEXT NOT NULL DEFAULT 'ai' CHECK (generated_by IN ('ai','human','hybrid')),
    llm_model       TEXT,
    prompt_tokens   INT NOT NULL DEFAULT 0,
    completion_tokens INT NOT NULL DEFAULT 0,
    generation_duration_ms INT,

    -- 时间戳
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (chapter_spec_id, version)
);

CREATE INDEX idx_chapter_contents_spec      ON chapter_contents(chapter_spec_id);
CREATE INDEX idx_chapter_contents_spec_ver  ON chapter_contents(chapter_spec_id, version DESC);

-- ============================================================================
-- illustrations — 图表定义与渲染状态
-- ============================================================================
CREATE TABLE illustrations (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id      UUID NOT NULL REFERENCES chapter_specs(id) ON DELETE CASCADE,
    bid_job_id      UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,

    -- 图表定义（JSONB 存储图表规格）
    figure_spec     JSONB NOT NULL DEFAULT '{}',
    figure_type     TEXT NOT NULL CHECK (figure_type IN ('flowchart','sequence','class','state','mermaid','bar','line','pie','table','image','custom')),

    -- 顺序
    order_in_chapter INT NOT NULL DEFAULT 0,

    -- 基本信息
    title           TEXT NOT NULL,
    caption         TEXT,

    -- 来源
    source          TEXT NOT NULL DEFAULT 'auto'
                       CHECK (source IN ('auto','manual','template','ai')),

    -- 渲染结果
    source_path     TEXT,                -- 原始文件（如 mermaid 源码）
    rendered_path   TEXT,                -- 渲染后图片路径
    rendered_format TEXT CHECK (rendered_format IN ('png','svg','jpg')),

    -- 数据引用（关联 evidence）
    data_refs       UUID[] NOT NULL DEFAULT '{}',
    cited_in_chapters UUID[] NOT NULL DEFAULT '{}',

    -- 回退链
    fallback_chain  JSONB NOT NULL DEFAULT '[]',

    -- 状态
    status          TEXT NOT NULL DEFAULT 'draft'
                       CHECK (status IN ('draft','pending','rendering','succeeded','failed','fallback')),

    -- 质量
    quality_score   REAL,
    validator_notes JSONB NOT NULL DEFAULT '[]',
    placeholder_reason TEXT,

    -- 重试
    retry_count     INT NOT NULL DEFAULT 0,
    version         INT NOT NULL DEFAULT 1,

    -- 时间戳
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    rendered_at     TIMESTAMPTZ,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_illustrations_chapter   ON illustrations(chapter_id, order_in_chapter);
CREATE INDEX idx_illustrations_bid       ON illustrations(bid_job_id, status);
CREATE INDEX idx_illustrations_bid_type  ON illustrations(bid_job_id, figure_type);
CREATE INDEX idx_illustrations_source    ON illustrations(source);

-- ============================================================================
-- evidence — 证据/素材（知识库证据链）
-- ============================================================================
CREATE TABLE evidence (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    bid_job_id      UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,

    -- 来源
    source_type     TEXT NOT NULL CHECK (source_type IN ('rfp','kb_material','user_upload','generated')),
    source_ref      TEXT NOT NULL,          -- 原始文档 ID 或引用

    -- 内容
    content_path    TEXT NOT NULL,          -- 内容文件路径（S3）
    content_hash    CHAR(64),               -- SHA-256
    chunk_index     INT,                    -- 如果是分段存储的文档，这是第几块

    -- 向量（pgvector）
    embedding       VECTOR(1536),

    -- 引用统计
    used_in_chapters    UUID[] NOT NULL DEFAULT '{}',
    used_in_figures     UUID[] NOT NULL DEFAULT '{}',

    -- 可靠性
    reliability_score   REAL NOT NULL DEFAULT 1.0 CHECK (reliability_score BETWEEN 0 AND 1),

    -- 元数据
    metadata           JSONB NOT NULL DEFAULT '{}',

    -- 时间戳
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_evidence_tenant         ON evidence(tenant_id);
CREATE INDEX idx_evidence_bid            ON evidence(bid_job_id);
CREATE INDEX idx_evidence_source_type    ON evidence(source_type);
CREATE INDEX idx_evidence_chapters       ON evidence USING gin(used_in_chapters);

-- 向量索引（IVFFlat，适合百万级）
CREATE INDEX idx_evidence_embedding ON evidence
    USING ivfflat (embedding vector_cosine_ops)
    WITH (lists = 100)
    WHERE embedding IS NOT NULL;

-- ============================================================================
-- updated_at 触发器
-- ============================================================================
CREATE TRIGGER trg_bid_jobs_updated
    BEFORE UPDATE ON bid_jobs
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TRIGGER trg_chapter_specs_updated
    BEFORE UPDATE ON chapter_specs
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

CREATE TRIGGER trg_illustrations_updated
    BEFORE UPDATE ON illustrations
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_illustrations_updated ON illustrations;
DROP TRIGGER IF EXISTS trg_chapter_specs_updated ON chapter_specs;
DROP TRIGGER IF EXISTS trg_bid_jobs_updated ON bid_jobs;

DROP INDEX IF EXISTS idx_evidence_embedding;
DROP INDEX IF EXISTS idx_evidence_chapters;
DROP INDEX IF EXISTS idx_evidence_source_type;
DROP INDEX IF EXISTS idx_evidence_bid;
DROP INDEX IF EXISTS idx_evidence_tenant;
DROP TABLE IF EXISTS evidence;

DROP INDEX IF EXISTS idx_illustrations_source;
DROP INDEX IF EXISTS idx_illustrations_bid_type;
DROP INDEX IF EXISTS idx_illustrations_bid;
DROP INDEX IF EXISTS idx_illustrations_chapter;
DROP TABLE IF EXISTS illustrations;

DROP INDEX IF EXISTS idx_chapter_contents_spec_ver;
DROP INDEX IF EXISTS idx_chapter_contents_spec;
DROP TABLE IF EXISTS chapter_contents;

DROP INDEX IF EXISTS idx_chapter_specs_parent;
DROP INDEX IF EXISTS idx_chapter_specs_bid_status;
DROP INDEX IF EXISTS idx_chapter_specs_bid_order;
DROP INDEX IF EXISTS idx_chapter_specs_bid;
DROP TABLE IF EXISTS chapter_specs;

DROP INDEX IF EXISTS uq_bid_jobs_active_per_project;
DROP INDEX IF EXISTS idx_bid_jobs_created_desc;
DROP INDEX IF EXISTS idx_bid_jobs_status;
DROP INDEX IF EXISTS idx_bid_jobs_workflow;
DROP INDEX IF EXISTS idx_bid_jobs_project;
DROP INDEX IF EXISTS idx_bid_jobs_tenant;
DROP TABLE IF EXISTS bid_jobs;

-- +goose StatementEnd