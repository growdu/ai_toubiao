# Plan: AI 标书自动撰写系统 v1 设计（借鉴 yibiao、面向云原生）

> 模式：plan-only。本文件只做设计，不动代码。
> 关联：/work/ai/framework.md（v1, 1056 行，已废弃 yibiao 锚定模式，转为通用设计纲要）
> 参考：/work/ai/OpenBidKit_Yibiao/yibiao.md（经验来源，不是实现标准）
> 写入时间：2026-06-27 23:04
> 工作目录：/work/ai
> 保存路径：/work/ai/.hermes/plans/2026-06-27_230405-ai-bid-system-v1.md

---

## 0. 与 yibiao 的根本差异（决策摘要）

> 借鉴的是"工程经验"和"领域抽象"，不是"技术选型"和"产品形态"。

  ┌────────────────┬─────────────────────────┬────────────────────────────┐
  │ 维度           │ yibiao (参考)            │ 新系统 v1 (本次设计)        │
  ├────────────────┼─────────────────────────┼────────────────────────────┤
  │ 产品形态       │ 桌面 Electron 单机       │ Web SaaS 为主 + 桌面备      │
  │ 用户模型       │ 单用户本地               │ 中小企业多租户              │
  │ 后端           │ 无（仅 Electron main）   │ Go 微服务                   │
  │ 数据库         │ 本地 SQLite              │ PostgreSQL + pgvector       │
  │ 任务调度       │ 进程内 taskService       │ Asynq (Redis)               │
  │ 对象存储       │ 本地 fs                  │ S3 兼容 (R2/MinIO)          │
  │ 实时通信       │ Electron IPC            │ WebSocket + SSE            │
  │ AI 调用        │ 单模型用户自配           │ 多模型路由（成本/质量自适应）│
  │ 计费           │ 无（开源免费）           │ 按项目订阅 + Token 用量     │
  │ 协作           │ 无                       │ 团队空间 + 角色权限         │
  │ 知识库         │ "不 RAG" 本地目录树      │ 混合策略（路由 + 弱 RAG）   │
  │ 部署           │ 用户本地                 │ 云原生 + 私有化双模         │
  └────────────────┴─────────────────────────┴────────────────────────────┘

借鉴的"领域抽象"（保留）：
- Step01-05 工作流（解析→拆解→大纲→事实→生成）
- 任务状态机（paused/restoring/auditing 可恢复语义）
- 全文一致性审计（普通 + agent 双模式）
- Prompt 缓存策略（大段共享上下文前置）
- JSON 修复链路（流式主请求后局部修补）
- 文本精确替换（多策略 fallback + 块级 anchor）
- 章节级并行 + 段级串行（缓存命中）

明确不照搬的：
- Electron + SQLite + CommonJS 旧式架构
- AI 配置单模型、用户自己填 API Key
- 本地文件存储 + 文件 hash
- "agent 不直接动业务数据"的隔离约定（云端用版本化 + 审计日志替代）

---

## 1. 产品定位

### 1.1 一句话定义

> "让中小型投标团队 4 小时内交付一份 80% 完成度的高合规标书"。

### 1.2 用户画像

- **核心用户**：5-50 人规模的系统集成 / 信息化 / 工程咨询公司
- **使用场景**：每周 1-3 个标，1-3 人临时组队编写
- **痛点**：
  - 标书模板反复改、内容雷同易废标
  - 老员工经验沉淀在个人电脑里，新人难复用
  - 不同 AI 模型各有所长，但切换成本高
  - 多人协作靠微信传文件，版本混乱
- **付费意愿**：每标 500-2000 元 或 月 2000-5000 元订阅

### 1.3 与 yibiao 的产品边界

| 场景 | yibiao | 新系统 v1 |
|---|---|---|
| 团队 3+ 人协作 | ❌ | ✅ 实时协作 |
| 私有化部署 | ✅ 但需自己编译 | ✅ 标准化 helm chart |
| 跨设备 | ❌ 数据在单机 | ✅ Web 端跨设备 |
| 历史标书沉淀 | 本地 | 云端企业知识库 |
| 行业模板 | 手工维护 | 模板市集 |
| 智能模型选择 | 用户手填 | 系统按成本/质量自动路由 |
| 计费 | 免费 | 按项目 / Token |

---

## 2. 核心架构

### 2.1 总体架构图

```
┌──────────────────────────────────────────────────────────────────────────┐
│                          客户端层                                        │
│  ┌──────────────────────┐    ┌──────────────────────┐                  │
│  │ Web App (Next.js)    │    │ Desktop (Tauri)      │                  │
│  │ - SSR/SSG            │    │ - 离线工作            │                  │
│  │ - 实时协作 UI        │    │ - 本地缓存            │                  │
│  │ - 模板市集           │    │ - 大文件处理          │                  │
│  └──────────┬───────────┘    └──────────┬───────────┘                  │
│             │                           │                              │
│             └─────────────┬─────────────┘                              │
│                           │ HTTPS / WSS                                │
└───────────────────────────┼──────────────────────────────────────────┘
                            ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       API Gateway (Go)                                   │
│  - 认证 (JWT / OIDC)                                                     │
│  - 租户路由                                                              │
│  - 限流 (per-tenant QPS)                                                 │
│  - 计量埋点 (每个 AI 请求的 token/成本)                                  │
└───────────────────────────┬──────────────────────────────────────────┘
                            ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       业务服务 (Go 微服务)                                │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐           │
│  │ project    │ │ document   │ │ workflow   │ │ knowledge  │           │
│  │ (项目/标段)│ │ (文档解析) │ │ (Step01-05)│ │ (知识库)   │           │
│  └────────────┘ └────────────┘ └────────────┘ └────────────┘           │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐           │
│  │ router     │ │ template   │ │ billing    │ │ notify     │           │
│  │ (模型路由) │ │ (模板市集) │ │ (计费)     │ │ (通知)     │           │
│  └────────────┘ └────────────┘ └────────────┘ └────────────┘           │
│         │                                                            │
│         └────────────┬───────────────────────────────────────────────┘
└──────────────────────┼──────────────────────────────────────────────┘
                       ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       任务调度层                                          │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ Asynq (Redis-based)                                            │    │
│  │ - 工作流任务队列 (按 Step 分队列)                                │    │
│  │ - 优先级 (合规 > 必答 > 加分)                                    │    │
│  │ - 失败重试 + 死信                                               │    │
│  │ - 状态可恢复 (UI 可重连看到进度)                                │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└──────────────────────┬──────────────────────────────────────────────┘
                       ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       AI 路由层 (核心创新)                                │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │ Model Router                                                    │    │
│  │ - 多 Provider 适配 (OpenAI/DeepSeek/Claude/Ollama/...)         │    │
│  │ - 任务画像 (解析/大纲/正文/审计/配图)                            │    │
│  │ - 成本预估 + 质量历史                                           │    │
│  │ - 降级策略 (主模型失败→备选)                                    │    │
│  │ - 缓存 (Prompt 缓存 + 响应缓存)                                 │    │
│  │ - 用量计量 (每次请求的 token + 成本)                            │    │
│  └─────────────────────────────────────────────────────────────────┘    │
└──────────────────────┬──────────────────────────────────────────────┘
                       ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       数据层                                              │
│  - PostgreSQL 16 (主数据 + pgvector 向量)                                │
│  - Redis 7 (队列/缓存/Session)                                          │
│  - S3 / R2 / MinIO (文档、图片、导出物)                                  │
│  - ClickHouse (可选，埋点分析)                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

### 2.2 服务拆分（Go）

| 服务 | 职责 | 端口 | 依赖 |
|---|---|---|---|
| api-gateway | 认证/路由/限流 | 8080 | 所有下游 |
| project-svc | 项目/标段 CRUD | 8081 | PG |
| document-svc | 文档上传/解析/Markdown 化 | 8082 | PG, S3 |
| workflow-svc | Step01-05 编排 + 状态机 | 8083 | PG, Redis, Asynq |
| knowledge-svc | 知识库/向量检索 | 8084 | PG, pgvector |
| router-svc | AI 模型路由 | 8085 | Redis, 多 Provider |
| template-svc | 行业模板/市集 | 8086 | PG, S3 |
| billing-svc | 订阅/Token 计费 | 8087 | PG, Redis |
| notify-svc | 邮件/Webhook/IM 通知 | 8088 | Redis, S3 |
| audit-svc | 一致性审计/agent 修复 | 8089 | PG, S3, router-svc |

微服务治理：
- 服务发现：Consul（私有化）或 k8s Service（云端）
- 链路追踪：OpenTelemetry → Jaeger / 阿里云 SLS
- 日志：结构化 JSON → Loki / 阿里云 SLS
- 配置中心：Consul KV / Nacos

---

## 3. 多模型路由（核心创新点）

> 这是 yibiao 没有的能力。新系统把"AI 调用"抽象为"路由问题"。

### 3.1 路由决策维度

每次 AI 请求需要回答 4 个问题：
1. **任务画像**：解析？大纲？正文？审计？配图？
2. **质量要求**：精确 JSON？长文本？推理？多语言？
3. **成本预算**：用户配额还剩多少？任务 SLA 多少？
4. **降级链**：主失败用什么？备失败用什么？

### 3.2 路由表设计

```yaml
# 路由配置（管理员可改、用户可覆盖）
routes:
  - task: rfp_parse
    quality: high_precision_json
    primary: { provider: anthropic, model: claude-sonnet-4, max_tokens: 8192 }
    fallback: [{ provider: openai, model: gpt-4o }, { provider: deepseek, model: deepseek-coder }]
    cost_per_call: 0.03  # 美元

  - task: outline_generate
    quality: balanced
    primary: { provider: deepseek, model: deepseek-chat, max_tokens: 4096 }
    fallback: [{ provider: openai, model: gpt-4o-mini }]
    cost_per_call: 0.002

  - task: content_generate
    quality: high_quality_long
    primary: { provider: anthropic, model: claude-sonnet-4, max_tokens: 16000 }
    fallback: [{ provider: openai, model: gpt-4o }]
    cost_per_call: 0.12

  - task: consistency_audit
    quality: precise_diff
    primary: { provider: anthropic, model: claude-sonnet-4, max_tokens: 8000 }
    fallback: [{ provider: openai, model: gpt-4o }]
    cost_per_call: 0.08

  - task: image_generate
    quality: balanced
    primary: { provider: openai, model: dall-e-3 }
    fallback: [{ provider: stability, model: sd-xl }]
    cost_per_call: 0.04
```

### 3.3 路由器实现

```go
// router-svc/internal/router/router.go
type Router interface {
    Route(ctx context.Context, req RouteRequest) (RouteDecision, error)
}

type RouteRequest struct {
    Task       TaskType    // rfp_parse, outline_generate, ...
    Quality    QualityHint // high_precision_json, balanced, ...
    TenantID   string
    UserQuota  *Quota
    Prompt     string
    EstimatedTokens int
}

type RouteDecision struct {
    Provider   string
    Model      string
    Params     map[string]any
    EstimatedCost float64
    Fallbacks  []Provider
    CacheKey   string  // Prompt 缓存命中
}
```

### 3.4 关键机制

**a) Prompt 缓存**
- 大段共享上下文（招标文件摘要、企业资质库、模板）放 system prompt 最前
- 跨 Step01-05 复用 system prefix（成本直降 10 倍，参考 yibiao 实测）
- 缓存键 = hash(provider + model + system_prefix + temperature)

**b) 响应缓存**
- 解析任务的结果缓存 24h（同一份招标复用）
- 大纲/正文不缓存（用户期待每次不同）

**c) 降级链**
- 主 Provider 失败 → 备 1（不同平台，避免同一故障域）
- 备 1 失败 → 备 2（本地 Ollama，永远可用）
- 三次失败 → 进死信 + 通知用户

**d) 成本计量**
- 每次 AI 调用立刻异步上报 token + 成本到 billing-svc
- 配额不足 → 任务降级为低质量模式或拒绝

**e) 用户可覆盖**
- 设置 → AI 配置 → 自定义某个任务用哪个模型
- 管理员可禁用某些 Provider

### 3.5 借鉴 yibiao 但升级

| yibiao 模式 | 新系统 v1 模式 |
|---|---|
| 用户在配置文件填 API Key | 管理员后台统一配，用户用配额 |
| 一次请求选一个 Provider | 路由自动选 + 降级链 |
| 失败本地重试 3 次 | 跨 Provider 降级 + 死信 |
| 无成本可见性 | 每次请求 token/成本入账单 |
| 单机单用户配额 | 租户配额 + 项目级预算 |

---

## 4. 工作流（借鉴 + 改进）

### 4.1 Step01-05（保留 yibiao 抽象）

```
Step01 准备          项目创建 + 标段划分
   ↓
Step02 解析          三档（本地/MinerU-agent/MinerU API）
   ↓
Step03 拆解          大纲生成（树形，≤3 轮扩写）
   ↓
Step04 全局事实      跨章节事实抽取（硬约束注入）
   ↓
Step05 生成          状态机：planning → generating → expanding → auditing → illustrating → done
```

### 4.2 与 yibiao 的差异

| 维度 | yibiao | 新系统 v1 |
|---|---|---|
| 任务执行 | 进程内 taskService | Asynq 分布式队列 |
| 状态持久化 | SQLite | PostgreSQL + Redis |
| 状态可恢复 | 同进程 | 跨进程、跨设备 |
| 失败处理 | warn 继续 | 死信 + 通知 + 人工重试 |
| 实时进度 | IPC 事件 | WebSocket / SSE |
| 多人协作 | 无 | 任务锁 + 操作日志 |
| 审计 | 进程内 | 不可变操作日志 + ClickHouse |

### 4.3 状态机定义（Go 伪码）

```go
type StepStatus string
const (
    StepPending     StepStatus = "pending"
    StepRunning     StepStatus = "running"
    StepPaused      StepStatus = "paused"        // 用户暂停
    StepRestoring   StepStatus = "restoring"     // 异常恢复中
    StepAuditing    StepStatus = "auditing"      // 一致性审计中
    StepDone        StepStatus = "done"
    StepFailed      StepStatus = "failed"
    StepDeadLetter  StepStatus = "dead_letter"   // 多次重试失败
)

type Workflow struct {
    ID         uuid.UUID
    ProjectID  uuid.UUID
    Step       StepType
    Status     StepStatus
    Attempts   int
    Payload    json.RawMessage
    Result     json.RawMessage
    ErrorLog   []StepError
    CreatedAt  time.Time
    UpdatedAt  time.Time
    LockedBy   *string  // 当前操作者 user_id
    LockedAt   *time.Time
}
```

### 4.4 Asynq 任务定义

```go
// workflow-svc/internal/tasks/
const (
    TypeParseRFP      = "workflow:parse_rfp"
    TypeGenerateOutline = "workflow:generate_outline"
    TypeExtractFacts  = "workflow:extract_facts"
    TypeGenerateContent = "workflow:generate_content"
    TypeAuditConsistency = "workflow:audit_consistency"
    TypeCleanupTables = "workflow:cleanup_tables"
    TypeGenerateImages = "workflow:generate_images"
)

func ParseRFPHandler(ctx context.Context, t *asynq.Task) error {
    var p ParseRFPayload
    json.Unmarshal(t.Payload(), &p)

    workflow, err := repo.GetWorkflow(ctx, p.WorkflowID)
    if err != nil { return err }

    if !workflow.AcquireLock(ctx, "parse-rfp") {
        return asynq.Skip  // 已被其他 worker 锁定
    }
    defer workflow.ReleaseLock(ctx)

    workflow.Status = StepRunning
    repo.Update(ctx, workflow)

    // 通过 router-svc 调用
    result, err := routerClient.ParseRFP(ctx, router.RouteRequest{
        Task: router.RFPParse,
        Prompt: buildParsePrompt(p.DocumentURL),
    })
    if err != nil {
        return handleRouteError(err, workflow)  // 触发重试或死信
    }

    workflow.Result = result.ToJSON()
    workflow.Status = StepDone
    repo.Update(ctx, workflow)

    // 触发下一步
    if p.NextStep != "" {
        client.Enqueue(p.NextStep, ...)
    }

    return nil
}
```

---

## 5. 数据模型（PostgreSQL）

### 5.1 多租户设计

- 共享数据库 + tenant_id 字段（行级隔离）
- 中小企业规模足够，省去 schema-per-tenant 的运维成本
- 未来可平滑升级到 schema-per-tenant

### 5.2 核心表

```sql
-- 租户
CREATE TABLE tenants (
    id          UUID PRIMARY KEY,
    name        TEXT NOT NULL,
    plan        TEXT NOT NULL,           -- free/pro/enterprise
    quota       JSONB NOT NULL,          -- {tokens_per_month: 1000000, projects: 50, ...}
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

-- 用户
CREATE TABLE users (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    email         TEXT UNIQUE NOT NULL,
    name          TEXT,
    role          TEXT NOT NULL,         -- owner/admin/member/viewer
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 项目（一次投标）
CREATE TABLE projects (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    name          TEXT NOT NULL,
    owner_id      UUID NOT NULL REFERENCES users(id),
    status        TEXT NOT NULL,         -- draft/in_progress/submitted/won/lost
    deadline      TIMESTAMPTZ,
    metadata      JSONB,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_projects_tenant ON projects(tenant_id, status);

-- 标段（一个项目可多个标段）
CREATE TABLE bid_sections (
    id            UUID PRIMARY KEY,
    project_id    UUID NOT NULL REFERENCES projects(id),
    name          TEXT NOT NULL,
    tender_doc_id UUID REFERENCES documents(id),
    config        JSONB,                 -- 该标段专属配置
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 文档（招标文件、参考文档、模板）
CREATE TABLE documents (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    project_id    UUID REFERENCES projects(id),
    kind          TEXT NOT NULL,         -- tender/reference/template/export
    filename      TEXT NOT NULL,
    mime_type     TEXT,
    size_bytes    BIGINT,
    s3_key        TEXT NOT NULL,
    sha256        TEXT NOT NULL,
    parse_status  TEXT,                  -- pending/parsing/parsed/failed
    parsed_md_key TEXT,                  -- 解析后的 Markdown S3 key
    metadata      JSONB,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_documents_tenant_kind ON documents(tenant_id, kind);
CREATE INDEX idx_documents_project ON documents(project_id);

-- 工作流（Step01-05 实例）
CREATE TABLE workflows (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    project_id    UUID NOT NULL REFERENCES projects(id),
    bid_section_id UUID REFERENCES bid_sections(id),
    step          TEXT NOT NULL,         -- parse/outline/facts/content/audit/cleanup/illustrate
    status        TEXT NOT NULL,         -- pending/running/paused/.../done/failed/dead_letter
    attempts      INT DEFAULT 0,
    payload       JSONB,
    result        JSONB,
    error_log     JSONB,
    locked_by     UUID REFERENCES users(id),
    locked_at     TIMESTAMPTZ,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_workflows_status ON workflows(tenant_id, status, step);
CREATE INDEX idx_workflows_project ON workflows(project_id, step, created_at);

-- 章节大纲（树形，借鉴 yibiao technical_plan_outline_nodes）
CREATE TABLE outline_nodes (
    id            UUID PRIMARY KEY,
    workflow_id   UUID NOT NULL REFERENCES workflows(id),
    parent_id     UUID REFERENCES outline_nodes(id),
    level         INT NOT NULL,
    order_index   INT NOT NULL,
    title         TEXT NOT NULL,
    content       TEXT,
    status        TEXT,                  -- pending/generating/done/failed
    token_count   INT,
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ
);
CREATE INDEX idx_outline_parent ON outline_nodes(parent_id, order_index);
CREATE INDEX idx_outline_status ON outline_nodes(workflow_id, status);

-- 全局事实（跨章节共享约束）
CREATE TABLE global_facts (
    id            UUID PRIMARY KEY,
    workflow_id   UUID NOT NULL REFERENCES workflows(id),
    category      TEXT NOT NULL,         -- qualification/case/product/compliance
    title         TEXT NOT NULL,
    content       TEXT NOT NULL,
    source_doc_id UUID REFERENCES documents(id),
    citations     JSONB,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_facts_workflow ON global_facts(workflow_id, category);

-- 知识库
CREATE TABLE knowledge_folders (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    parent_id     UUID REFERENCES knowledge_folders(id),
    name          TEXT NOT NULL,
    sort_order    INT DEFAULT 0,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE knowledge_documents (
    id            UUID PRIMARY KEY,
    folder_id     UUID NOT NULL REFERENCES knowledge_folders(id),
    title         TEXT NOT NULL,
    s3_key        TEXT NOT NULL,
    sha256        TEXT NOT NULL,
    metadata      JSONB,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);

-- 知识库分块 + 向量（pgvector）
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE knowledge_chunks (
    id            UUID PRIMARY KEY,
    document_id   UUID NOT NULL REFERENCES knowledge_documents(id) ON DELETE CASCADE,
    chunk_index   INT NOT NULL,
    content       TEXT NOT NULL,
    embedding     VECTOR(1536),         -- 视 embedding 模型维度
    metadata      JSONB,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_chunks_embedding ON knowledge_chunks USING ivfflat (embedding vector_cosine_ops);

-- 计费
CREATE TABLE usage_records (
    id            UUID PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(id),
    project_id    UUID REFERENCES projects(id),
    workflow_id   UUID REFERENCES workflows(id),
    provider      TEXT NOT NULL,
    model         TEXT NOT NULL,
    task          TEXT NOT NULL,         -- parse/outline/content/...
    prompt_tokens INT NOT NULL,
    completion_tokens INT NOT NULL,
    cost_usd      NUMERIC(10,6) NOT NULL,
    cached        BOOLEAN DEFAULT FALSE,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_usage_tenant_time ON usage_records(tenant_id, created_at);
CREATE INDEX idx_usage_project ON usage_records(project_id, created_at);

-- 审计日志（不可变）
CREATE TABLE audit_logs (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     UUID NOT NULL,
    actor_id      UUID,
    action        TEXT NOT NULL,         -- workflow.start/document.upload/config.update/...
    target_type   TEXT,
    target_id     UUID,
    details       JSONB,
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_audit_tenant_time ON audit_logs(tenant_id, created_at);
-- 不可变：只 INSERT，禁 UPDATE/DELETE（用 trigger 或权限控制）

-- 操作锁（团队协作冲突控制）
CREATE TABLE resource_locks (
    resource_type TEXT NOT NULL,         -- outline_node/workflow/document
    resource_id   UUID NOT NULL,
    locked_by     UUID NOT NULL REFERENCES users(id),
    locked_at     TIMESTAMPTZ NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (resource_type, resource_id)
);
```

### 5.3 借鉴 yibiao 但升级

| yibiao 表 | 新系统 v1 | 升级点 |
|---|---|---|
| technical_plan_meta (50 列) | projects + bid_sections | 拆为多表，按字段类型归位 |
| technical_plan_outline_nodes | outline_nodes | + token_count（计费）+ 协作锁 |
| technical_plan_global_fact_groups | global_facts | + 引用计数 + 版本 |
| knowledge_* | knowledge_folders/documents/chunks | + pgvector 向量 |
| 无 | usage_records | 新增：每次 AI 调用精确计量 |
| 无 | audit_logs | 新增：合规审计 + 团队责任 |
| 无 | resource_locks | 新增：协作冲突控制 |

---

## 6. 知识库（混合策略）

> 既不照搬 yibiao 的"不 RAG"，也不迷信纯 RAG，而是"路由 + 弱 RAG + 人工分类"三件套。

### 6.1 三层检索

```
Layer 1: 精确匹配（最优先）
  - 用户标签订位
  - 目录树定位（手动维护的标签）
  - 关键词全文检索（PG tsvector）

Layer 2: 弱 RAG（pgvector 向量）
  - top-k 相似度检索
  - 仅作候选池，不直接喂给 LLM
  - 重排序：BM25 + 关键词命中加权

Layer 3: 全局事实（借鉴 yibiao）
  - 从历史标书抽取的事实
  - 强约束注入 Step05 Prompt
  - 防止幻觉
```

### 6.2 知识采集流程

```
用户上传企业资质文件
   ↓
doc2markdown 解析（与 yibiao 一致）
   ↓
按章节自动分类（用 LLM 提取 category: 资质/案例/产品/团队）
   ↓
切块 + embedding（pgvector）
   ↓
用户审核目录（标签 / 移动 / 删除）
   ↓
入库完成
```

### 6.3 检索 API

```go
type SearchRequest struct {
    Query       string
    TenantID    string
    TopK        int               // 默认 20
    Layers      []Layer           // 哪些层参与
    Filters     map[string]string // category, tag, year, ...
    Rerank      bool              // 是否重排序
}

type SearchResult struct {
    Chunk       KnowledgeChunk
    Score       float64
    Layer       Layer
    Citation    string
    Highlighted string
}
```

---

## 7. 一致性审计（升级版）

> 借鉴 yibiao 四层防护，但云端化、可协作、可追溯。

### 7.1 四层防护

| 层 | yibiao 模式 | 新系统 v1 模式 |
|---|---|---|
| L1 废标项 | rejection_check_* 三类并发 | rejection-svc，AI + 规则混合 |
| L2 一致性 | Step05 auditing | audit-svc，可指定 normal/agent 模式 |
| L3 精确替换 | textEdit.cjs 446 行 | Go 重写 + 规则库（可热更新） |
| L4 查重 | duplicate_check 14 表 | duplicate-svc + 行业历史库 |

### 7.2 审计 API

```go
type AuditRequest struct {
    ProjectID    string
    WorkflowID   string
    Mode         AuditMode  // normal / agent
    Trigger      AuditTrigger  // auto/manual/scheduled
    Priority     int
}

type AuditResult struct {
    Issues       []AuditIssue
    Suggestions  []AuditSuggestion
    Cost         float64
    Confidence   float64
}

type AuditIssue struct {
    Type         string  // hallucination/inconsistency/typo/logic
    Severity     string  // high/medium/low
    Location     OutlineNodeLocation
    Description  string
    Evidence     []Citation
    Suggestion   string
}
```

### 7.3 agent 模式（OpenCode 集成）

借鉴 yibiao 的"双进程 + 端口代理"，但升级为：
- agent 工作在云端 worker pod
- 输入：当前 outline + 一致性问题清单
- 输出：精确的 old_text/new_text 编辑列表
- 业务侧：UI 展示 diff，用户确认后应用

### 7.4 操作日志（不可变）

每次审计发现的 issue 写入 audit_logs，作为团队责任和合规追溯依据。

---

## 8. 计费与配额

### 8.1 计费维度

```
┌────────────────────────────────────────────────────┐
│ Plan          │ 项目数  │ 月 Token  │ 团队人数   │ 单价  │
├────────────────────────────────────────────────────┤
│ Free          │ 1       │ 50k       │ 1          │  ¥0   │
│ Pro           │ 10      │ 1M        │ 5          │  ¥499 │
│ Team          │ 50      │ 5M        │ 20         │  ¥1999│
│ Enterprise    │ 不限    │ 不限      │ 不限       │  面议 │
└────────────────────────────────────────────────────┘

超额：按 ¥0.04/1k tokens 计费（基于实际路由成本 + 30% 利润）
```

### 8.2 计量点

- router-svc 每次 AI 调用立即异步上报 usage_records
- billing-svc 实时聚合
- 配额预警：用量达 80% / 95% / 100% 触发通知
- 硬限：100% 后任务降级（用本地 Ollama）或拒绝

### 8.3 订阅生命周期

```
注册 → 邮箱验证 → 选 plan → 支付（Stripe / 微信）
   ↓
激活 tenant + 创建 owner 用户
   ↓
30 天试用（无需支付）→ 试用结束 → 续费 / 降级 / 停服
   ↓
停服：保留数据 90 天，过期清理
```

---

## 9. 团队协作

### 9.1 角色

| 角色 | 权限 |
|---|---|
| owner | 全部 + 计费 + 成员管理 |
| admin | 项目管理 + 配置 + 审计 |
| member | 编辑 + 评论 |
| viewer | 只读 + 评论 |

### 9.2 实时协作

- WebSocket 推送 outline 节点变更
- 软锁 + 硬锁（用户进入编辑模式持锁 5 分钟，自动续期）
- 冲突解决：Last-Write-Wins + 操作日志
- 评论：节点级别讨论

### 9.3 评审流（借鉴 yibiao 全文审计）

```
作者提交初稿
   ↓
系统自动审计（normal 模式）
   ↓
如果 issues > 0，弹窗提示修改
   ↓
如果 issues == 0，转交审核人
   ↓
审核人：评论 / 批准 / 退回
   ↓
退回 → 作者修改 → 重审
批准 → 终稿锁 → 触发导出
```

---

## 10. 部署架构

### 10.1 云端 SaaS（默认）

```yaml
# helm chart
- ingress (nginx + cert-manager)
- api-gateway (3 replicas, HPA)
- 各业务 svc (2 replicas each, HPA)
- asynq worker (5 replicas, 队列分组)
- postgres (RDS / 阿里云 RDS, 16 vCPU 64GB)
- redis (ElastiCache / 阿里云 Redis, 集群 6 节点)
- s3 (AWS S3 / 阿里云 OSS)
- observability: prometheus + grafana + jaeger + loki
```

### 10.2 私有化（企业版）

```yaml
# 一键 helm install
- 离线镜像仓库
- 内嵌 postgresql + redis (StatefulSet)
- MinIO 替代 S3
- 离线模型（Ollama + Qwen2.5-72B / DeepSeek-V3）
- 域名 + HTTPS 由客户提供
```

### 10.3 桌面备（Tauri）

- 仅核心功能：项目查看、文档上传、离线编辑
- 同步：在线时增量同步到云端
- 离线：本地 SQLite 缓存 + 队列，云端恢复时上传

---

## 11. 关键技术决策记录

| 决策 | 选项 | 选定 | 理由 |
|---|---|---|---|
| 后端语言 | Go / Python / Node | Go | 高并发 + 任务调度友好 + 部署简单 |
| 队列 | Asynq / Celery / Temporal | Asynq | 纯 Go + Redis，与语言栈一致 |
| 数据库 | PostgreSQL / MySQL | PostgreSQL | pgvector 一体化 + JSONB 灵活 |
| 实时通信 | WebSocket / SSE | 两者结合 | WS 双向（协作），SSE 单向（任务进度） |
| 微服务 vs 单体 | 拆分 / 单体 | 单体起步 + 内部模块化 | 10 人以下团队避免过早拆分 |
| ORM | GORM / sqlc / ent | sqlc | 类型安全 + 性能好 + 团队熟悉 |
| 配置 | Nacos / Consul / k8s ConfigMap | k8s ConfigMap（云）/ 文件（私有化） | 简化 |
| 监控 | Prometheus / OpenTelemetry | OpenTelemetry + Prometheus | 标准 + 多后端 |
| 前端框架 | Next.js / Nuxt / Remix | Next.js | 生态最大 |
| 桌面备 | Electron / Tauri | Tauri | 二进制小、内存低、安全 |
| 认证 | JWT / OIDC / 自建 | OIDC + JWT | 支持企业 SSO |
| 计费 | Stripe / 微信 / 自建 | 抽象 billing-svc，按地区适配 | 多市场 |

---

## 12. 迭代路线（4 个里程碑）

### M1 (4 周) - 单机 MVP
- 项目 + 标段 + 文档上传
- Step02 解析（仅本地模式）
- Step03 大纲（单模型）
- Step05 生成（单模型 + 状态机）
- Word 导出
- 单租户，无协作
- 验证：1 个真实标走通全流程

### M2 (4 周) - 多租户 + 路由
- 注册/登录/租户隔离
- 多模型路由（OpenAI + DeepSeek）
- Prompt 缓存
- 配额 + 计量
- Web 协作（评论 + 锁）
- 验证：3 个种子用户用一周

### M3 (4 周) - 知识库 + 审计
- 知识库三层检索
- 全文一致性审计（normal + agent）
- 废标项检查
- 查重
- 模板市集
- 验证：内部 5 个真实标试用

### M4 (4 周) - 协作 + 私有化
- 实时协作（WebSocket）
- 团队角色 + 审计日志
- 桌面备（Tauri）
- 私有化 helm chart
- 验证：1 个付费企业客户 + 1 个私有化客户

---

## 13. 风险与边界

| 风险 | 缓解 |
|---|---|
| AI 路由成本失控 | 用量硬限 + 路由成本预估 + 缓存命中率监控 |
| 私有化部署复杂 | helm chart + 一键脚本 + 远程协助 |
| 多模型质量不一致 | 质量历史统计 + 自动降级 + 人工标注反馈 |
| 协作冲突 | 软锁 + 操作日志 + Last-Write-Wins |
| 数据隔离 | tenant_id 强制 + 自动化测试覆盖 + 季度渗透测试 |
| 大文档解析超时 | 分片 + 断点续传 + 多档策略 |
| 弱模型 JSON 不稳定 | 借鉴 yibiao：流式主请求后局部 repair |
| 长上下文溢出 | 按字数分桶（参考 yibiao 30 万字桶）+ 摘要压缩 |

---

## 14. Open Questions 决策记录（2026-06-27 23:10 已全部决策）

详细决策文档：/work/ai/.hermes/plans/2026-06-27_230405-ai-bid-system-answers.md
（每项含：推荐答案、理由、备选、实施细节、退出条件）

  ┌────┬─────────────────────┬──────────────────────────────────────────┐
  │ # │ 问题                │ 决策                                    │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 1 │ 多租户隔离粒度      │ 行级 tenant_id（M1-M3 长期），500GB 切 │
  │    │                     │ schema-per-tenant                       │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 2 │ 质量历史积累        │ M1-M2 规则路由                          │
  │    │                     │ M3 引入隐式反馈（采纳/修改/重生成）       │
  │    │                     │ M4+ LLM-as-a-Judge（1% 抽样）          │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 3 │ 私有化模型          │ 默认不提供，客户自备                    │
  │    │                     │ 提供 Ollama 一键部署脚本                │
  │    │                     │ 最低配置：1xRTX 4090 (24GB)             │
  │    │                     │ 推荐模型：Qwen2.5-72B-AWQ              │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 4 │ Word 格式           │ v1 仅 .docx                             │
  │    │                     │ ODF/WPS 看 M3 反馈（> 10% 才做）       │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 5 │ 审计 agent 模式     │ 默认关闭                                │
  │    │                     │ UI 显式按钮"深度审计"开启              │
  │    │                     │ 单独计费（5-10x 成本保护）              │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 6 │ 模板市集冷启动      │ v1 预置 6 个行业模板                    │
  │    │                     │ 信息化集成/政府采购/工程EPC/咨询/        │
  │    │                     │ 设备采购/教育培训                        │
  │    │                     │ M3 开放 UGC，M4 付费模板               │
  ├────┼─────────────────────┼──────────────────────────────────────────┤
  │ 7 │ 数据同步冲突        │ 云端为权威源                            │
  │    │                     │ 桌面本地 SQLite 缓存                    │
  │    │                     │ 冲突时以 Web 为准 + 冲突列表            │
  │    │                     │ UI 提供三选项：放弃/覆盖/手动合并        │
  └────┴─────────────────────┴──────────────────────────────────────────┘

每项的"退出条件"和"切换备选"详见 answers.md，触达阈值时回到本节重评。

---

## 15. 文件 / 目录预规划

```
/work/ai/bidwriter/                  # 项目根（待创建）
├── docs/
│   ├── architecture.md              # 本文档的代码化版本
│   ├── api.md                       # OpenAPI 规范
│   ├── router.md                    # AI 路由配置说明
│   └── deployment.md                # 部署手册
├── helm/                            # Kubernetes Helm Chart
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
├── services/
│   ├── api-gateway/
│   ├── project-svc/
│   ├── document-svc/
│   ├── workflow-svc/
│   ├── knowledge-svc/
│   ├── router-svc/
│   ├── template-svc/
│   ├── billing-svc/
│   ├── notify-svc/
│   └── audit-svc/
├── web/                             # Next.js 前端
│   ├── app/
│   ├── components/
│   ├── lib/
│   └── package.json
├── desktop/                         # Tauri 桌面备
│   ├── src/
│   └── tauri.conf.json
├── shared/
│   ├── proto/                       # 跨服务 protobuf
│   ├── types/                       # 共享类型
│   └── prompts/                     # 共享 Prompt 模板
├── migrations/                      # SQL 迁移
├── scripts/
├── go.mod
├── go.sum
├── Makefile
├── docker-compose.yml               # 本地开发
└── README.md
```

---

## 16. 验证标准（M1 验收）

- [ ] 注册 → 创建项目 → 上传招标 → Step02 解析完成（≤5 分钟）
- [ ] Step03 大纲生成（≤3 分钟）
- [ ] Step05 正文生成（≤20 分钟，含扩展 + 审计）
- [ ] Word 导出（≤30 秒）
- [ ] 单标 Token 消耗 ≤ 500k（基于 DeepSeek-chat 路由）
- [ ] 单标端到端 ≤ 30 分钟（不含人工审核）
- [ ] 异常关闭后状态可恢复（重连看到进度）
- [ ] 并发 10 个标无明显降速
- [ ] p95 任务响应 < 1s（除 AI 调用）

---

## 17. 关联文档

- /work/ai/framework.md：通用设计纲要（已从 yibiao 锚定改为通用版）
- /work/ai/OpenBidKit_Yibiao/yibiao.md：经验来源
- /work/ai/.hermes/plans/：本计划目录

---

## 18. 下一步行动（出 plan 模式后）

如果用户批准本计划：

1. 创建 /work/ai/bidwriter/ 项目骨架
2. 初始化 Go module + sqlc + Asynq + Next.js
3. M1 第一周：项目 + 文档 + 解析（同步 + 异步）
4. 每周五演示进度，每月评估里程碑

需要用户决策：
- 命名空间（bidwriter / bidai / bidmaster / 其他？）
- 是否真的开始 M1，还是先做更详细的需求调研？
- 团队规模（开发人数 / 角色）？
- 启动资金 / 商业模式确认？

---

> 本 plan 完成。等待用户确认或调整。
