-- +goose Up
-- +goose StatementBegin

-- ============================================================================
-- kb_materials — 知识库素材（企业资质、案例、证书等）
-- ============================================================================
CREATE TABLE kb_materials (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- 分类
    category    TEXT NOT NULL CHECK (category IN ('certificate','case','patent','team','equipment','qualification','other')),
    title       TEXT NOT NULL,
    summary     TEXT,

    -- 内容（文件或文本）
    content     TEXT,                              -- 直接存储短文本
    file_path   TEXT,                              -- 长文本/大文件存 S3
    file_size   BIGINT,
    mime_type   VARCHAR(64),

    -- 元数据
    metadata    JSONB NOT NULL DEFAULT '{}',

    -- 状态
    status      TEXT NOT NULL DEFAULT 'active'
                   CHECK (status IN ('active','archived','deleted')),

    -- 统计
    chunk_count     INT NOT NULL DEFAULT 0,
    indexed_at      TIMESTAMPTZ,                   -- 向量化完成时间

    -- 软删除
    deleted_at  TIMESTAMPTZ,

    -- 时间戳
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kb_materials_tenant      ON kb_materials(tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_kb_materials_category    ON kb_materials(tenant_id, category) WHERE deleted_at IS NULL;
CREATE INDEX idx_kb_materials_status      ON kb_materials(status) WHERE deleted_at IS NULL;
CREATE INDEX idx_kb_materials_title_fts   ON kb_materials USING gin(to_tsvector('simple', title));

-- ============================================================================
-- kb_chunks — 知识库文本分块（向量化存储）
-- ============================================================================
CREATE TABLE kb_chunks (
    id          BIGSERIAL PRIMARY KEY,
    material_id UUID NOT NULL REFERENCES kb_materials(id) ON DELETE CASCADE,
    tenant_id   UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,

    -- 内容
    content     TEXT NOT NULL,
    content_tsv TSVECTOR,                          -- 全文检索

    -- 向量
    content_vec VECTOR(1536),                      -- 与 evidence embedding 同维度

    -- 位置
    chunk_index     INT NOT NULL,                  -- 在文档中的顺序
    char_start      INT,                           -- 在原文中的字符偏移
    char_end        INT,

    -- 来源追溯
    source_location TEXT,                          -- 页码、章节名等

    -- 引用统计
    hit_count       INT NOT NULL DEFAULT 0,        -- 被检索命中次数
    used_count      INT NOT NULL DEFAULT 0,        -- 被实际引用的次数

    -- 元数据
    metadata    JSONB NOT NULL DEFAULT '{}',

    -- 时间戳
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_kb_chunks_material   ON kb_chunks(material_id);
CREATE INDEX idx_kb_chunks_tenant     ON kb_chunks(tenant_id);
CREATE INDEX idx_kb_chunks_tsv        ON kb_chunks USING GIN(content_tsv);

-- 向量索引
CREATE INDEX idx_kb_chunks_vec ON kb_chunks
    USING ivfflat (content_vec vector_cosine_ops)
    WITH (lists = 100)
    WHERE content_vec IS NOT NULL;

-- ============================================================================
-- kb_evidence_links — 证据关联关系（章节 ↔ 证据）
-- ============================================================================
CREATE TABLE kb_evidence_links (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    bid_job_id      UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,

    -- 关联的两端
    chapter_id      UUID REFERENCES chapter_specs(id) ON DELETE CASCADE,
    evidence_id     UUID NOT NULL REFERENCES evidence(id) ON DELETE CASCADE,

    -- 关联类型
    link_type       TEXT NOT NULL DEFAULT 'citation'
                       CHECK (link_type IN ('citation','support','example','definition')),

    -- 关联强度（0-1）
    relevance_score REAL NOT NULL DEFAULT 0.5
                       CHECK (relevance_score BETWEEN 0 AND 1),

    -- 引用的原文摘录
    quoted_text     TEXT,

    -- 状态
    status          TEXT NOT NULL DEFAULT 'active'
                       CHECK (status IN ('active','rejected','pending')),

    -- 时间戳
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (chapter_id, evidence_id, link_type)
);

CREATE INDEX idx_kb_evidence_links_bid     ON kb_evidence_links(bid_job_id);
CREATE INDEX idx_kb_evidence_links_chapter ON kb_evidence_links(chapter_id);
CREATE INDEX idx_kb_evidence_links_evidence ON kb_evidence_links(evidence_id);

-- ============================================================================
-- updated_at 触发器
-- ============================================================================
CREATE TRIGGER trg_kb_materials_updated
    BEFORE UPDATE ON kb_materials
    FOR EACH ROW EXECUTE FUNCTION touch_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_kb_materials_updated ON kb_materials;

DROP INDEX IF EXISTS idx_kb_evidence_links_evidence;
DROP INDEX IF EXISTS idx_kb_evidence_links_chapter;
DROP INDEX IF EXISTS idx_kb_evidence_links_bid;
DROP TABLE IF EXISTS kb_evidence_links;

DROP INDEX IF EXISTS idx_kb_chunks_vec;
DROP INDEX IF EXISTS idx_kb_chunks_tsv;
DROP INDEX IF EXISTS idx_kb_chunks_tenant;
DROP INDEX IF EXISTS idx_kb_chunks_material;
DROP TABLE IF EXISTS kb_chunks;

DROP INDEX IF EXISTS idx_kb_materials_title_fts;
DROP INDEX IF EXISTS idx_kb_materials_status;
DROP INDEX IF EXISTS idx_kb_materials_category;
DROP INDEX IF EXISTS idx_kb_materials_tenant;
DROP TABLE IF EXISTS kb_materials;

-- +goose StatementEnd