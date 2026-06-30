# 数据库设计文档

> 本文档定义 AI 标书自动生成系统的 PostgreSQL 数据库结构设计。

---

# 一、设计原则

| 原则 | 说明 |
|---|---|
| **事务一致性** | 核心业务表使用 InnoDB 引擎，关键操作使用事务 |
| **范式与反范式平衡** | 元数据高度范式化，正文/内容使用 JSONB 存储 |
| **分区策略** | 审计日志等大表按时间分区 |
| **索引策略** | 复合索引覆盖常见查询，避免过多索引 |

---

# 二、表结构

## 2.1 用户与会话表

### users

```sql
CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email           VARCHAR(255) NOT NULL UNIQUE,
    password_hash   VARCHAR(255) NOT NULL,
    name            VARCHAR(128),
    company         VARCHAR(255),
    roles           VARCHAR(32)[] DEFAULT '{}',
    is_active       BOOLEAN DEFAULT TRUE,
    last_login_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_company ON users(company);
```

### user_sessions

```sql
CREATE TABLE user_sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  VARCHAR(64) NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    ip_address  INET,
    user_agent  TEXT
);

CREATE INDEX idx_sessions_user ON user_sessions(user_id);
CREATE INDEX idx_sessions_token ON user_sessions(token_hash);
CREATE INDEX idx_sessions_expires ON user_sessions(expires_at);
```

## 2.2 标书任务表

### bid_jobs

```sql
CREATE TABLE bid_jobs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rfp_file_path       TEXT,
    rfp_file_name       VARCHAR(255),
    status              VARCHAR(32) NOT NULL DEFAULT 'pending',
    current_step        VARCHAR(64),
    config              JSONB DEFAULT '{}',
    word_template_id    UUID,

    -- 解析结果
    parse_result        JSONB,

    -- 项目信息
    project_name        VARCHAR(255),
    industry            VARCHAR(32),
    issuer              VARCHAR(255),
    bid_deadline        TIMESTAMPTZ,
    budget              NUMERIC(18, 2),

    -- 统计
    total_chapters      INTEGER DEFAULT 0,
    completed_chapters  INTEGER DEFAULT 0,
    total_illustrations INTEGER DEFAULT 0,
    completed_illustrations INTEGER DEFAULT 0,

    -- 时间戳
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    updated_at          TIMESTAMPTZ DEFAULT NOW(),
    completed_at        TIMESTAMPTZ,
    failed_at           TIMESTAMPTZ,

    -- 乐观锁
    version             INTEGER DEFAULT 1
);

CREATE INDEX idx_bid_jobs_user_status ON bid_jobs(user_id, status);
CREATE INDEX idx_bid_jobs_created ON bid_jobs(created_at DESC);
CREATE INDEX idx_bid_jobs_status ON bid_jobs(status);
CREATE INDEX idx_bid_jobs_industry ON bid_jobs(industry);
```

### bid_job_events

```sql
CREATE TABLE bid_job_events (
    id          BIGSERIAL PRIMARY KEY,
    bid_job_id  UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    event       VARCHAR(64) NOT NULL,
    detail      JSONB,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_bid_job_events_job ON bid_job_events(bid_job_id);
CREATE INDEX idx_bid_job_events_created ON bid_job_events(created_at DESC);
```

## 2.3 章节相关表

### chapter_specs

```sql
CREATE TABLE chapter_specs (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_job_id              UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    parent_id               UUID REFERENCES chapter_specs(id),
    title                   TEXT NOT NULL,
    level                   SMALLINT NOT NULL CHECK (level BETWEEN 1 AND 3),
    order_index             INTEGER NOT NULL,
    chapter_type            VARCHAR(32) NOT NULL,
    target_word_count       INTEGER NOT NULL DEFAULT 1500,
    min_word_count          INTEGER NOT NULL DEFAULT 800,
    writing_style           VARCHAR(32) NOT NULL DEFAULT 'formal',
    required_elements       JSONB DEFAULT '[]',
    illustration_requirements JSONB DEFAULT '[]',
    evidence_requirements   JSONB DEFAULT '[]',
    priority                VARCHAR(16) NOT NULL DEFAULT 'normal',
    dependencies           UUID[] DEFAULT '{}',
    estimated_llm_tokens    INTEGER,
    status                 VARCHAR(32) NOT NULL DEFAULT 'planned',
    error_message          TEXT,
    retry_count            INTEGER DEFAULT 0,
    created_at             TIMESTAMPTZ DEFAULT NOW(),
    updated_at             TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_chapter_specs_bid_order ON chapter_specs(bid_job_id, order_index);
CREATE INDEX idx_chapter_specs_status ON chapter_specs(bid_job_id, status);
CREATE INDEX idx_chapter_specs_priority ON chapter_specs(bid_job_id, priority);
CREATE INDEX idx_chapter_specs_parent ON chapter_specs(parent_id);
```

### chapter_contents

```sql
CREATE TABLE chapter_contents (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_spec_id     UUID NOT NULL REFERENCES chapter_specs(id) ON DELETE CASCADE,
    version             INTEGER NOT NULL DEFAULT 1,
    content_path        TEXT NOT NULL,
    content_hash        CHAR(64) NOT NULL,
    word_count          INTEGER NOT NULL,
    min_word_met        BOOLEAN NOT NULL DEFAULT FALSE,
    generated_by        VARCHAR(16) NOT NULL DEFAULT 'ai',
    generation_duration INTEGER,
    created_at          TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(chapter_spec_id, version)
);

CREATE INDEX idx_chapter_contents_spec ON chapter_contents(chapter_spec_id);
CREATE INDEX idx_chapter_contents_version ON chapter_contents(chapter_spec_id, version DESC);
```

## 2.4 图表相关表

### illustrations

```sql
CREATE TABLE illustrations (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chapter_id              UUID NOT NULL REFERENCES chapter_specs(id) ON DELETE CASCADE,
    bid_job_id              UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    figure_spec             JSONB NOT NULL,
    type                    VARCHAR(32) NOT NULL,
    order_in_chapter        INTEGER NOT NULL,
    title                   TEXT NOT NULL,
    caption                 TEXT,
    source                  VARCHAR(32) NOT NULL,
    source_path             TEXT NOT NULL,
    rendered_path           TEXT,
    rendered_format         VARCHAR(8),
    data_refs               UUID[] DEFAULT '{}',
    cited_in_chapters       UUID[] DEFAULT '{}',
    fallback_chain          JSONB DEFAULT '[]',
    status                  VARCHAR(32) NOT NULL DEFAULT 'draft',
    quality_score           REAL,
    validator_notes         JSONB DEFAULT '[]',
    placeholder_reason      TEXT,
    retry_count             INTEGER DEFAULT 0,
    version                 INTEGER NOT NULL DEFAULT 1,
    created_at              TIMESTAMPTZ DEFAULT NOW(),
    rendered_at             TIMESTAMPTZ,
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_illustrations_chapter_order ON illustrations(chapter_id, order_in_chapter);
CREATE INDEX idx_illustrations_bid_status ON illustrations(bid_job_id, status);
CREATE INDEX idx_illustrations_type ON illustrations(bid_job_id, type);
CREATE INDEX idx_illustrations_source ON illustrations(source);
```

## 2.5 审计相关表

### audit_issues

```sql
CREATE TABLE audit_issues (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_job_id      UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    chapter_id      UUID REFERENCES chapter_specs(id),
    dimension       VARCHAR(32) NOT NULL,
    severity        VARCHAR(16) NOT NULL,
    location        JSONB,
    issue           TEXT NOT NULL,
    suggestion      TEXT,
    status          VARCHAR(16) NOT NULL DEFAULT 'open',
    resolved_by     UUID REFERENCES users(id),
    resolved_at     TIMESTAMPTZ,
    resolve_action  VARCHAR(32),
    resolve_payload JSONB,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_audit_issues_bid ON audit_issues(bid_job_id);
CREATE INDEX idx_audit_issues_severity ON audit_issues(bid_job_id, severity);
CREATE INDEX idx_audit_issues_status ON audit_issues(status);
CREATE INDEX idx_audit_issues_dimension ON audit_issues(dimension);
```

### response_items

```sql
CREATE TABLE response_items (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_job_id              UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    requirement_id          VARCHAR(64) NOT NULL,
    requirement_text        TEXT NOT NULL,
    response_chapter_id     UUID REFERENCES chapter_specs(id),
    response_summary        TEXT,
    compliance_status       VARCHAR(32) NOT NULL DEFAULT 'unaddressed',
    evidence_refs          UUID[] DEFAULT '{}',
    auditor_notes          TEXT,
    score_estimate         REAL,
    created_at              TIMESTAMPTZ DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_response_items_bid ON response_items(bid_job_id);
CREATE INDEX idx_response_items_chapter ON response_items(response_chapter_id);
CREATE INDEX idx_response_items_compliance ON response_items(compliance_status);
```

## 2.6 知识库表

### kb_documents

```sql
CREATE TABLE kb_documents (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    doc_type        VARCHAR(32) NOT NULL,
    file_path       TEXT NOT NULL,
    file_size       BIGINT,
    mime_type       VARCHAR(64),
    metadata        JSONB DEFAULT '{}',
    status          VARCHAR(16) NOT NULL DEFAULT 'processing',
    error_message   TEXT,
    indexed_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_kb_documents_user ON kb_documents(user_id);
CREATE INDEX idx_kb_documents_type ON kb_documents(doc_type);
CREATE INDEX idx_kb_documents_status ON kb_documents(status);
CREATE INDEX idx_kb_documents_name ON kb_documents USING gin(to_tsvector('simple', name));
```

### kb_chunks

```sql
CREATE TABLE kb_chunks (
    id              BIGSERIAL PRIMARY KEY,
    document_id     UUID NOT NULL REFERENCES kb_documents(id) ON DELETE CASCADE,
    content         TEXT NOT NULL,
    content_tsv     TSVECTOR,
    content_vec     VECTOR(1024),
    chunk_index     INTEGER NOT NULL,
    metadata        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_kb_chunks_document ON kb_chunks(document_id);
CREATE INDEX idx_kb_chunks_tsv ON kb_chunks USING GIN(content_tsv);
```

## 2.7 证据链表

### evidences

```sql
CREATE TABLE evidences (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bid_job_id          UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    source_type         VARCHAR(32) NOT NULL,
    source_ref          TEXT NOT NULL,
    content_path        TEXT NOT NULL,
    content_hash        CHAR(64),
    used_in_chapters    UUID[] DEFAULT '{}',
    used_in_illustrations UUID[] DEFAULT '{}',
    reliability_score   REAL DEFAULT 1.0,
    created_at          TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_evidences_bid ON evidences(bid_job_id);
CREATE INDEX idx_evidences_source_type ON evidences(source_type);
CREATE INDEX idx_evidences_chapters ON evidences USING gin(used_in_chapters);
```

## 2.8 模板表

### word_templates

```sql
CREATE TABLE word_templates (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID REFERENCES users(id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    template_path   TEXT NOT NULL,
    styles          JSONB,
    is_default      BOOLEAN DEFAULT FALSE,
    is_system       BOOLEAN DEFAULT FALSE,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_word_templates_user ON word_templates(user_id);
CREATE INDEX idx_word_templates_default ON word_templates(is_default);
```

## 2.9 审计日志表（分区表）

```sql
CREATE TABLE audit_logs (
    id              BIGSERIAL,
    user_id         UUID,
    action          VARCHAR(64) NOT NULL,
    resource_type   VARCHAR(32),
    resource_id     VARCHAR(64),
    details         JSONB,
    ip_address     INET,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- 创建月度分区
CREATE TABLE audit_logs_2026_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE audit_logs_2026_02 PARTITION OF audit_logs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
-- ... 其他月份分区

CREATE INDEX idx_audit_logs_user ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
```

---

# 三、全文检索与向量索引

## 3.1 全文检索配置

```sql
-- 创建全文检索配置（中文分词）
CREATE TEXT SEARCH CONFIGURATION IF NOT EXISTS chinese_zh (COPY = simple);

-- 添加映射（需要安装 zhparser 扩展）
-- ALTER TEXT SEARCH CONFIGURATION chinese_zh
--     ADD MAPPING FOR hword_with_num WITH english, simple;
```

## 3.2 向量检索（pgvector）

```sql
-- 启用 pgvector 扩展
CREATE EXTENSION IF NOT EXISTS vector;

-- 知识库文本块向量索引
ALTER TABLE kb_chunks ADD COLUMN IF NOT EXISTS content_vec VECTOR(1024);

CREATE INDEX idx_kb_chunks_vec ON kb_chunks
    USING ivfflat (content_vec vector_cosine_ops)
    WITH (lists = 100);

-- 图表语义索引（用于双向语义匹配）
CREATE TABLE figure_embeddings (
    figure_id      UUID PRIMARY KEY,
    caption_vec   VECTOR(1024),
    description_vec VECTOR(1024),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_figure_caption_vec ON figure_embeddings
    USING ivfflat (caption_vec vector_cosine_ops)
    WITH (lists = 50);
```

---

# 四、触发器与约束

## 4.1 自动更新时间戳

```sql
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为常用表创建触发器
CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_bid_jobs_updated_at
    BEFORE UPDATE ON bid_jobs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_chapter_specs_updated_at
    BEFORE UPDATE ON chapter_specs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_illustrations_updated_at
    BEFORE UPDATE ON illustrations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_audit_issues_updated_at
    BEFORE UPDATE ON audit_issues
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

## 4.2 乐观锁检查

```sql
CREATE OR REPLACE FUNCTION check_version()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.version + 1 != NEW.version THEN
        RAISE EXCEPTION 'Version conflict: expected % but got %', OLD.version + 1, NEW.version;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_bid_jobs_version
    BEFORE UPDATE ON bid_jobs
    FOR EACH ROW EXECUTE FUNCTION check_version();
```

## 4.3 级联删除章节内容

```sql
CREATE OR REPLACE FUNCTION cascade_delete_chapters()
RETURNS TRIGGER AS $$
BEGIN
    -- 删除章节内容
    DELETE FROM chapter_contents WHERE chapter_spec_id = OLD.id;
    -- 删除图表
    DELETE FROM illustrations WHERE chapter_id = OLD.id;
    -- 删除审计问题
    DELETE FROM audit_issues WHERE chapter_id = OLD.id;
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_chapter_specs_delete
    BEFORE DELETE ON chapter_specs
    FOR EACH ROW EXECUTE FUNCTION cascade_delete_chapters();
```

---

# 五、迁移策略

## 5.1 迁移工具

使用 [golang-migrate](https://github.com/golang-migrate/migrate) 进行数据库迁移：

```bash
# 创建迁移文件
migrate create -ext sql -dir migrations -seq initial_schema

# 应用迁移
migrate -path migrations -database $DATABASE_URL up

# 回滚
migrate -path migrations -database $DATABASE_URL down 1
```

## 5.2 迁移文件命名

```
migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_add_vector_index.up.sql
├── 000002_add_vector_index.down.sql
└── ...
```

## 5.3 数据迁移检查

```sql
-- 迁移前后数据一致性检查
SELECT
    COUNT(*) as total,
    COUNT(DISTINCT user_id) as unique_users,
    COUNT(DISTINCT bid_job_id) as unique_bids
FROM chapter_specs;

-- 检查孤立数据
SELECT COUNT(*) FROM chapter_specs cs
WHERE NOT EXISTS (SELECT 1 FROM bid_jobs bj WHERE bj.id = cs.bid_job_id);
```

---

# 六、性能优化

## 6.1 常用查询优化

```sql
-- 1. 获取标书进度（利用复合索引）
EXPLAIN ANALYZE
SELECT status, COUNT(*)
FROM bid_jobs
WHERE user_id = $1 AND status IN ('writing', 'auditing')
GROUP BY status;

-- 2. 获取章节状态统计（利用索引）
EXPLAIN ANALYZE
SELECT status, COUNT(*)
FROM chapter_specs
WHERE bid_job_id = $1
GROUP BY status;

-- 3. 向量相似度搜索
EXPLAIN ANALYZE
SELECT d.id, d.name,
       1 - (c.content_vec <=> $1) AS similarity
FROM kb_chunks c
JOIN kb_documents d ON d.id = c.document_id
WHERE c.content_vec <=> $1 < 0.3
ORDER BY c.content_vec <=> $1
LIMIT 10;
```

## 6.2 连接池配置

PostgreSQL 连接池建议：

| 参数 | 开发环境 | 生产环境 |
|---|---|---|
| max_connections | 50 | 200 |
| shared_buffers | 128MB | 2GB |
| effective_cache_size | 256MB | 6GB |
| work_mem | 4MB | 64MB |
| maintenance_work_mem | 64MB | 512MB |

## 6.3 定期维护

```sql
-- 定期 ANALYZE 更新统计信息
ANALYZE;

-- 定期 VACUUM 清理 dead tuples
VACUUM ANALYZE;

-- 检查索引使用情况
SELECT
    schemaname,
    tablename,
    indexname,
    idx_tup_read,
    idx_tup_fetch,
    idx_scan
FROM pg_stat_user_indexes
WHERE idx_scan = 0
ORDER BY pg_relation_size(indexrelid) DESC;
```
