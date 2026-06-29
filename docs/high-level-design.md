# 概要设计文档（HLD）

> 本文档基于 `docs/framework.md` 的设计纲要和 `docs/tech-selection.md` 的技术选型，给出系统的具体架构设计。
> 目标读者：开发工程师、架构师、测试工程师。

---

# 〇、文档约定

- **接口**：用 TypeScript-style 伪代码表示
- **数据流**：用文字 + 流程图描述
- **状态机**：用 ASCII 图表示
- **错误码**：统一前缀 `BID_` 开头

---

# 一、系统目标与边界

## 1.1 系统目标

实现"输入 RFP + 背景材料 → 自动生成完整标书"的端到端系统，支撑：

| 目标 | 量化指标 |
|---|---|
| 端到端延迟 | 50 章节 / 10 并发，≤ 10 分钟（不含人在回路） |
| 章节质量 | 最小字数 800/小节，合规条款响应率 100% |
| 图表质量 | 渲染成功率 ≥ 95%（含 fallback） |
| 跨章节一致性 | 术语统一率 ≥ 98%，数据矛盾 ≤ 0 |
| 失败恢复 | 单章节失败不阻塞其他，可重试可恢复 |

## 1.2 系统边界

**在范围内**：
- RFP 解析（PDF/Word/Markdown）
- 章节规划、撰写、配图、审计、汇总
- 人在回路点（章节大纲确认、审计问题处理、最终样式微调）
- 知识库管理（公司资质、历史标书、技术文档）
- Word/PDF 输出

**不在范围内（MVP）**：
- 实时多人协作
- 移动端
- 跨语言（先支持中文）
- 自动投标（只生成标书，不参与投标流程）

## 1.3 非功能性需求

| 维度 | 指标 |
|---|---|
| 可用性 | ≥ 99.5%（单实例） |
| 并发 | 至少 10 章节并行 + 5 人在回路点 |
| 数据持久性 | 零丢失（异常关闭后可恢复） |
| 可观测 | 端到端追踪、日志结构化、指标全覆盖 |
| 安全 | 敏感数据加密、API Key 不落地前端 |

---

# 二、组件架构

## 2.1 顶层组件图

```
┌────────────────────────────────────────────────────────────────┐
│                       客户端（Web / Desktop）                    │
│  - 上传 RFP + 背景材料                                          │
│  - 进度可视化                                                   │
│  - 人在回路点交互                                                │
└────────────────────────────────────────────────────────────────┘
                            │ HTTPS / IPC
                            ↓
┌────────────────────────────────────────────────────────────────┐
│                       API 网关（FastAPI）                       │
│  - 鉴权 / 限流 / 路由 / OpenAPI                                 │
└────────────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        ↓                   ↓                   ↓
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│  编排服务     │   │  知识库服务   │   │  文档服务     │
│ Orchestrator │   │  KB Service  │   │  Doc Service │
│  - 端到端流程 │   │  - 文档索引   │   │  - 文件存储   │
│  - 状态机     │   │  - 全文检索   │   │  - 模板管理   │
└──────────────┘   │  - 证据链     │   │  - 导出 Word  │
        │          └──────────────┘   └──────────────┘
        │                  │                   │
        ↓                  ↓                   ↓
┌────────────────────────────────────────────────────────────────┐
│                       任务队列（Celery + Redis）                 │
│  - 章节级任务组                                                  │
│  - 重试 / 优先级 / 并发控制                                       │
└────────────────────────────────────────────────────────────────┘
        │
        ├──→ 章节规划任务（Planner Task）
        ├──→ 章节撰写任务（Writer Task）
        ├──→ 图表生成任务（Illustrator Task）
        ├──→ 章节审计任务（Auditor Task）
        ├──→ 跨章节审计任务（Cross-Auditor Task）
        └──→ 标书汇总任务（Assembler Task）
                            │
                            ↓
┌────────────────────────────────────────────────────────────────┐
│                  AI 路由层（LLM Router）                        │
│  - 多 provider 路由                                              │
│  - Prompt 缓存                                                  │
│  - 重试 / 降级 / 熔断                                            │
└────────────────────────────────────────────────────────────────┘
        │              │              │              │
        ↓              ↓              ↓              ↓
   ┌────────┐    ┌────────┐    ┌────────┐    ┌────────┐
   │Claude  │    │DeepSeek│    │ GPT-4o │    │ 本地模型│
   │ Sonnet │    │  V3    │    │  mini  │    │(可选)  │
   └────────┘    └────────┘    └────────┘    └────────┘
                            │
                            ↓
┌────────────────────────────────────────────────────────────────┐
│                  存储层（PostgreSQL + S3）                       │
│  - 元数据（章节规格、章节正文索引、响应矩阵、证据链）              │
│  - 大文件（章节正文、图表源码、渲染产物、模板）                    │
│  - 缓存（Redis：任务锁、Prompt 缓存、检索缓存）                   │
└────────────────────────────────────────────────────────────────┘
```

## 2.2 核心服务职责

### 2.2.1 编排服务（Orchestrator）

**职责**：
- 端到端流程的状态机管理
- 章节任务的派发与依赖管理
- 人在回路点的暂停/恢复
- 跨服务的协调调用

**关键接口**：

```typescript
interface Orchestrator {
    // 启动端到端流程
    async startBidGeneration(input: BidGenerationInput): Promise<BidJob>
    
    // 暂停（保留状态）
    async pauseBidJob(jobId: string): Promise<void>
    
    // 恢复
    async resumeBidJob(jobId: string): Promise<void>
    
    // 重做单个章节
    async redoChapter(jobId: string, chapterId: string): Promise<void>
    
    // 查询进度
    async getBidJobStatus(jobId: string): Promise<BidJobStatus>
}
```

### 2.2.2 知识库服务（KB Service）

**职责**：
- 背景材料入库（PDF/Word/Markdown）
- 文档索引化（章节切分、关键词抽取）
- 全文检索（tsvector）
- 证据链管理

**关键接口**：

```typescript
interface KBService {
    async ingestDocument(file: UploadedFile): Promise<Document>
    async search(query: string, filters: SearchFilters): Promise<SearchHit[]>
    async getEvidence(evidenceId: string): Promise<Evidence>
    async linkEvidenceToChapter(evidenceId: string, chapterId: string): Promise<void>
}
```

### 2.2.3 文档服务（Doc Service）

**职责**：
- 文件存储（S3 兼容）
- Word 模板管理
- Word/PDF 导出
- 文件下载

**关键接口**：

```typescript
interface DocService {
    async storeChapterContent(chapterId: string, content: string, version: number): Promise<Path>
    async loadChapterContent(chapterId: string, version: number): Promise<string>
    async renderWordTemplate(templateId: string, data: TemplateData): Promise<Buffer>
    async exportToWord(bidJobId: string): Promise<Buffer>
    async exportToPDF(bidJobId: string): Promise<Buffer>
}
```

### 2.2.4 AI 路由层（LLM Router）

**职责**：
- 按 task 名路由到合适 provider
- Prompt 缓存管理
- 重试、降级、熔断
- 用量统计与成本核算

**关键接口**：

```typescript
interface LLMRouter {
    async chat(taskName: string, request: ChatRequest): Promise<ChatResponse>
    async chatJson<T>(taskName: string, request: ChatRequest, schema: Type<T>): Promise<T>
    async embed(text: string): Promise<number[]>
}
```

---

# 三、核心流程设计

## 3.1 端到端流程

```
用户提交
   ↓
[API] POST /api/v1/bids
   ↓
[Orchestrator] 创建 BidJob（状态：pending）
   ↓
[Orchestrator] 调度 Planner Task
   ↓
[Planner Task]
   ├─ 解析 RFP（PDF/Word → 结构化）
   ├─ 索引化背景材料
   ├─ LLM 生成章节大纲 + 规格清单
   └─ 写库 + 触发人在回路点 1
   ↓
[人在回路点 1] 用户确认/调整章节大纲
   ↓
[Orchestrator] 派发章节任务（celery_group）
   ↓
   ┌──────────────────────────┐
   │ 章节级并行（每章节独立）  │
   │ ┌────┐ ┌────┐ ┌────┐    │
   │ │Ch1 │ │Ch2 │ │Ch3 │... │  并发度 = 10
   │ └────┘ └────┘ └────┘    │
   │   ↓      ↓      ↓       │
   │   撰+图+审 串行执行       │
   └──────────────────────────┘
   ↓
[Orchestrator] 收集所有章节完成信号
   ↓
[Cross-Auditor Task] 跨章节一致性审计
   ↓
[Rejection-Check Task] 废标项扫描
   ↓
[Duplicate-Check Task] 标书查重
   ↓
[Orchestrator] 触发人在回路点 2
   ↓
[人在回路点 2] 用户处理审计问题
   ↓
[Assembler Task] 标书汇总
   ├─ 章节排序、编号
   ├─ 图表编号统一
   ├─ 交叉引用解析
   ├─ 目录自动生成
   └─ 输出 docx
   ↓
[Orchestrator] 触发人在回路点 3
   ↓
[人在回路点 3] 用户微调样式
   ↓
[Doc Service] 输出 docx + pdf
   ↓
用户下载
```

## 3.2 章节任务内部流程

```python
@celery_app.task(base=ChapterTask, bind=True, max_retries=3)
def chapter_pipeline(self, chapter_spec: ChapterSpec, materials: List[Evidence]) -> ChapterResult:
    # 1. 任务组锁（防止并发）
    with redis_lock(f"chapter:{chapter_spec.id}", ttl=600):
        
        # 2. 检索章节素材（如未提供）
        if not materials:
            materials = kb_service.search_by_spec(chapter_spec)
        
        # 3. 生成章节正文
        content = llm_router.chat(
            task_name="chapter_write",
            system=[
                {"type": "text", "text": GLOBAL_FACTS_AND_GLOSSARY},
                {"type": "text", "text": chapter_spec.to_prompt(),
                 "cache_control": {"type": "ephemeral"}}
            ],
            messages=[{"role": "user", "content": build_user_prompt(materials)}],
            max_tokens=4000,
            temperature=0.3
        )
        
        # 4. 解析章节正文（Markdown）
        chapter_content = parse_markdown(content.text)
        
        # 5. 提取图表占位符
        illustration_specs = extract_illustrations(chapter_content)
        
        # 6. 串行生成图表
        illustrations = []
        for illust_spec in illustration_specs:
            illust = illustrator_service.generate(illust_spec, materials)
            illustrations.append(illust)
            if illust.status == "failed":
                # 占位图，不阻塞
                illustrations.append(create_placeholder(illust_spec))
        
        # 7. 章节内一致性审计
        audit_report = chapter_auditor.audit(chapter_content, illustrations, chapter_spec)
        
        # 8. 写入存储
        doc_service.store_chapter_content(chapter_spec.id, chapter_content, version=1)
        
        # 9. 更新状态
        update_chapter_status(chapter_spec.id, "done", audit_report)
        
        return ChapterResult(
            chapter_id=chapter_spec.id,
            content_path=...,
            illustrations=illustrations,
            audit=audit_report
        )
```

## 3.3 状态机

### 3.3.1 BidJob 状态机

```
pending
  ↓
planning          ← 章节规划中
  ↓
awaiting_review   ← 人在回路点 1（章节大纲确认）
  ↓
writing           ← 章节撰写中（含配图、内审）
  ↓
auditing          ← 跨章节审计中
  ↓
awaiting_fixup    ← 人在回路点 2（审计问题处理）
  ↓
assembling        ← 汇总中
  ↓
awaiting_style    ← 人在回路点 3（样式微调）
  ↓
completed
  ↓
failed
```

### 3.3.2 Chapter 状态机

```
planned
  ↓
writing           ← 撰写
  ↓
illustrating      ← 图表生成
  ↓
chapter_auditing  ← 章节内审计
  ↓
done

任何阶段可 → paused → restoring → 原状态
任何阶段可 → failed → writing（重做）
```

---

# 四、数据模型（核心 ER）

## 4.1 实体关系

```
┌──────────┐ 1     N ┌──────────┐ 1     N ┌──────────┐
│  BidJob  │────────→│ Chapter  │────────→│  Content │
└──────────┘         └──────────┘         └──────────┘
     │ 1                                       │ N
     │                                         ↓
     │ N                                  ┌──────────┐
     ↓                                    │Illustrat │
┌──────────┐                              └──────────┘
│ Material │                                   │ N
└──────────┘                                   ↓
     │ N                                  ┌──────────┐
     ↓                                    │ Evidence │
┌──────────┐                              └──────────┘
│Evidence  │──────────────────────────────── ↑
└──────────┘ N                            │ 1
     │ 1
     ↓
┌──────────┐
│Response  │
│ Matrix   │
└──────────┘
```

## 4.2 关键表

### bid_jobs

```sql
CREATE TABLE bid_jobs (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    rfp_file_path   TEXT NOT NULL,
    status          VARCHAR(32) NOT NULL,
    current_step    VARCHAR(64),
    config          JSONB,              -- 用户配置（粒度、偏好、模板等）
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    updated_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
CREATE INDEX idx_bid_jobs_user_status ON bid_jobs(user_id, status);
```

### chapter_specs

```sql
CREATE TABLE chapter_specs (
    id                      UUID PRIMARY KEY,
    bid_job_id              UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    parent_id               UUID REFERENCES chapter_specs(id),
    title                   TEXT NOT NULL,
    level                   SMALLINT NOT NULL,        -- 1/2/3
    order_index             INTEGER NOT NULL,
    chapter_type            VARCHAR(32) NOT NULL,     -- technical/business/...
    target_word_count       INTEGER NOT NULL,
    min_word_count          INTEGER NOT NULL DEFAULT 800,
    writing_style           VARCHAR(32) NOT NULL,
    required_elements       JSONB DEFAULT '[]',
    illustration_requirements JSONB DEFAULT '[]',
    evidence_requirements   JSONB DEFAULT '[]',
    dependencies            UUID[] DEFAULT '{}',
    status                  VARCHAR(32) NOT NULL DEFAULT 'planned',
    created_at              TIMESTAMPTZ DEFAULT NOW(),
    updated_at              TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_chapter_specs_bid_order ON chapter_specs(bid_job_id, order_index);
CREATE INDEX idx_chapter_specs_status ON chapter_specs(bid_job_id, status);
```

### chapter_contents

```sql
CREATE TABLE chapter_contents (
    id                      UUID PRIMARY KEY,
    chapter_spec_id         UUID NOT NULL REFERENCES chapter_specs(id) ON DELETE CASCADE,
    version                 INTEGER NOT NULL DEFAULT 1,
    content_path            TEXT NOT NULL,           -- 大文本路径
    content_hash            CHAR(64) NOT NULL,
    word_count              INTEGER NOT NULL,
    min_word_met            BOOLEAN NOT NULL,
    generated_by            VARCHAR(16) NOT NULL,    -- ai/human/hybrid
    generated_at            TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(chapter_spec_id, version)
);
```

### illustrations

```sql
CREATE TABLE illustrations (
    id                      UUID PRIMARY KEY,
    chapter_id              UUID NOT NULL REFERENCES chapter_specs(id) ON DELETE CASCADE,
    type                    VARCHAR(32) NOT NULL,
    position                VARCHAR(16) NOT NULL DEFAULT 'inline',
    order_in_chapter        INTEGER NOT NULL,
    title                   TEXT NOT NULL,
    caption                 TEXT,
    source                  VARCHAR(32) NOT NULL,    -- mermaid/ai/data/table
    source_path             TEXT NOT NULL,           -- 源码/prompt/HTML/JSON
    rendered_path           TEXT,
    data_refs               UUID[] DEFAULT '{}',
    version                 INTEGER NOT NULL DEFAULT 1,
    status                  VARCHAR(32) NOT NULL DEFAULT 'draft'
);
CREATE INDEX idx_illustrations_chapter_order ON illustrations(chapter_id, order_in_chapter);
```

### response_matrix

```sql
CREATE TABLE response_items (
    id                      UUID PRIMARY KEY,
    bid_job_id              UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    requirement_id          VARCHAR(64) NOT NULL,
    requirement_text        TEXT NOT NULL,
    response_chapter_id     UUID REFERENCES chapter_specs(id),
    response_summary        TEXT,
    compliance_status       VARCHAR(32) NOT NULL,    -- compliant/partial/...
    evidence_refs           UUID[] DEFAULT '{}',
    auditor_notes           TEXT
);
CREATE INDEX idx_response_items_bid ON response_items(bid_job_id);
```

### evidence

```sql
CREATE TABLE evidences (
    id                      UUID PRIMARY KEY,
    bid_job_id              UUID NOT NULL REFERENCES bid_jobs(id) ON DELETE CASCADE,
    source_type             VARCHAR(32) NOT NULL,
    source_ref              TEXT NOT NULL,
    content_path            TEXT NOT NULL,
    used_in_chapters        UUID[] DEFAULT '{}',
    used_in_illustrations   UUID[] DEFAULT '{}',
    reliability_score       REAL DEFAULT 1.0
);
```

## 4.3 任务队列表（Celery 标准）

```sql
-- Celery 自带，不需自定义
-- celery_taskmeta: 任务结果
-- 通过 Redis broker 调度
```

---

# 五、接口设计

## 5.1 REST API（OpenAPI 3.1）

### 5.1.1 端到端流程

```yaml
POST /api/v1/bids
  description: 创建标书生成任务
  request:
    body:
      rfp_file: File
      materials: File[]
      config: BidConfig
  response:
    202:
      bid_job_id: UUID

GET /api/v1/bids/{bid_job_id}
  description: 查询标书状态
  response:
    200:
      status: string
      progress: { planned: int, writing: int, ... }
      chapters: [ChapterSummary]
      illustrations: [IllustrationSummary]
      audit_issues: [AuditIssue]

POST /api/v1/bids/{bid_job_id}/pause
POST /api/v1/bids/{bid_job_id}/resume
POST /api/v1/bids/{bid_job_id}/chapters/{chapter_id}/redo

GET /api/v1/bids/{bid_job_id}/chapters/{chapter_id}/content
  description: 获取章节正文（Markdown）
  response:
    200:
      content: string
      illustrations: [Illustration]
      citations: [Evidence]

PUT /api/v1/bids/{bid_job_id}/chapters/{chapter_id}/content
  description: 用户编辑章节正文
  request:
    body: { content: string }
  response:
    200: { content_hash: string, version: int }
```

### 5.1.2 章节大纲确认（人在回路点 1）

```yaml
GET /api/v1/bids/{bid_job_id}/outline
POST /api/v1/bids/{bid_job_id}/outline/confirm
  request: { outline: ChapterSpec[] }
  response: 204
```

### 5.1.3 审计问题处理（人在回路点 2）

```yaml
GET /api/v1/bids/{bid_job_id}/audit-issues
POST /api/v1/bids/{bid_job_id}/audit-issues/{issue_id}/resolve
  request: { action: "auto-fix" | "manual-edit" | "ignore", payload?: any }
POST /api/v1/bids/{bid_job_id}/confirm-audit
  response: 204
```

### 5.1.4 文档导出

```yaml
GET /api/v1/bids/{bid_job_id}/export/word
GET /api/v1/bids/{bid_job_id}/export/pdf
GET /api/v1/bids/{bid_job_id}/export/summary   # 一页纸摘要
```

## 5.2 内部接口（gRPC / 消息队列）

章节任务与编排服务之间用 Redis Pub/Sub 通信：

```python
# 章节完成事件
{
  "event": "chapter.completed",
  "bid_job_id": "...",
  "chapter_id": "...",
  "version": 1,
  "audit": { "issues": 0 },
  "timestamp": "..."
}

# 章节失败事件
{
  "event": "chapter.failed",
  "chapter_id": "...",
  "error_code": "BID_CHAPTER_RETRY_EXHAUSTED",
  "retry_count": 3
}
```

---

# 六、关键算法与策略

## 6.1 章节规划算法

```python
async def plan_chapters(rfp_struct: RFPStruct, user_config: UserConfig) -> List[ChapterSpec]:
    # 1. LLM 生成章节大纲
    outline = await llm_router.chat_json(
        task_name="chapter_planning",
        system=GLOBAL_FACTS_AND_GLOSSARY,
        messages=[{
            "role": "user",
            "content": f"""请将以下 RFP 拆分为章节大纲：
            
{rfp_struct.to_markdown()}

要求：
- 3 级标题结构
- 每章节 800-3000 字
- 输出 JSON 数组
"""
        }],
        schema=List[ChapterSpec]
    )
    
    # 2. 粒度自适应
    target_chapter_count = max(20, min(80, rfp_struct.estimated_word_count // 2000))
    outline = adjust_granularity(outline, target_chapter_count)
    
    # 3. 优先级标注
    outline = assign_priority(outline, rfp_struct.compliance_items)
    
    # 4. 依赖分析
    outline = analyze_dependencies(outline)
    
    # 5. 必含要素对齐
    outline = align_required_elements(outline, rfp_struct.requirements)
    
    return outline
```

## 6.2 Prompt 缓存策略

```python
def build_chapter_writing_prompt(spec: ChapterSpec, materials: List[Evidence]) -> ChatRequest:
    # 段 1：系统级共享（强缓存）
    system_prefix = GLOBAL_FACTS + GLOSSARY + WRITING_STYLE_GUIDE
    
    # 段 2：章节级规格（章节内复用）
    chapter_spec_text = spec.to_prompt()
    
    # 段 3：素材
    materials_text = format_materials(materials)
    
    return ChatRequest(
        system=[
            {"type": "text", "text": system_prefix},
            {"type": "text", "text": chapter_spec_text,
             "cache_control": {"type": "ephemeral"}},  # 标记缓存边界
            {"type": "text", "text": materials_text}
        ],
        messages=[{"role": "user", "content": "请按规格撰写本章节"}],
        max_tokens=4000,
        temperature=0.3
    )
```

## 6.3 章节内串行流程

```python
# 章节内严格串行，确保 Prompt 缓存前缀复用
async def execute_chapter_pipeline(spec: ChapterSpec, materials: List[Evidence]) -> ChapterResult:
    # Step 1: 撰写（缓存命中 → 章节内复用）
    content = await write_chapter(spec, materials)
    
    # Step 2: 提取图表占位
    illustration_specs = extract_illustration_placeholders(content)
    
    # Step 3: 串行生成图表（每图独立调用，无依赖）
    illustrations = []
    for illust_spec in illustration_specs:
        try:
            illust = await generate_illustration(illust_spec, materials)
        except IllustrationError as e:
            illust = create_placeholder(illust_spec, error=str(e))
        illustrations.append(illust)
    
    # Step 4: 章节内审计
    audit = await chapter_audit(content, illustrations, spec)
    
    # Step 5: 最小字数校验
    if not audit.min_word_met:
        # 触发扩写
        content = await expand_chapter(spec, content, materials, audit.short_sections)
        audit = await chapter_audit(content, illustrations, spec)
    
    return ChapterResult(content=content, illustrations=illustrations, audit=audit)
```

## 6.4 跨章节一致性算法

```python
async def cross_chapter_audit(bid_job_id: str) -> CrossAuditReport:
    chapters = await load_all_chapters(bid_job_id)
    response_matrix = await load_response_matrix(bid_job_id)
    
    # 1. 抽取术语表
    glossary = extract_glossary_from_chapters(chapters)
    
    # 2. 响应矩阵交叉
    response_audit = cross_check_response_matrix(response_matrix, chapters)
    
    # 3. 数据一致性（关键参数）
    data_audit = check_data_consistency(chapters, response_matrix)
    
    # 4. 术语一致性
    term_audit = check_term_consistency(chapters, glossary)
    
    # 5. 章节引用完整性
    ref_audit = check_references(chapters, illustrations)
    
    return CrossAuditReport(
        response=response_audit,
        data=data_audit,
        terms=term_audit,
        references=ref_audit
    )
```

---

# 七、可观测性设计

## 7.1 指标（Prometheus）

```
# 端到端
bid_job_duration_seconds{status="completed|failed"}
bid_job_total{status="..."}

# 章节
chapter_duration_seconds{type="technical|business|..."}
chapter_total{status="done|failed|retried"}
chapter_word_count{type="..."}

# 图表
illustration_total{type="mermaid|ai|data|table", status="success|failed"}
illustration_render_duration_seconds{type="..."}

# AI
llm_request_total{provider, model, task}
llm_token_total{provider, model, direction="input|output"}
llm_cache_hit_total{provider, model}
llm_cost_usd_total{provider, model}

# 任务
celery_task_total{name, status}
celery_task_duration_seconds{name}
celery_queue_length{queue}
```

## 7.2 日志（结构化 JSON）

```python
import structlog

logger = structlog.get_logger()

logger.info(
    "chapter.completed",
    bid_job_id=bid_job_id,
    chapter_id=chapter_id,
    duration_seconds=42.3,
    word_count=2050,
    illustration_count=3,
    audit_issues=0,
    llm_usage={
        "input_tokens": 28500,
        "output_tokens": 4200,
        "cache_hit": True,
        "cost_usd": 0.18
    }
)
```

## 7.3 追踪（OpenTelemetry）

```python
from opentelemetry import trace

tracer = trace.get_tracer("bid-orchestrator")

with tracer.start_as_current_span("chapter.write") as span:
    span.set_attribute("chapter.id", chapter_id)
    span.set_attribute("chapter.type", spec.chapter_type)
    
    with tracer.start_as_current_span("llm.call") as llm_span:
        llm_span.set_attribute("llm.provider", "anthropic")
        llm_span.set_attribute("llm.model", "claude-sonnet-4-6")
        response = await llm_router.chat(...)
        llm_span.set_attribute("llm.tokens.input", response.usage.input_tokens)
        llm_span.set_attribute("llm.tokens.output", response.usage.output_tokens)
```

---

# 八、安全设计

## 8.1 数据加密

| 数据 | 加密方式 |
|---|---|
| 传输 | TLS 1.3 |
| API Key（环境变量） | 不入数据库、不入日志 |
| 章节正文 | 静态加密（文件系统层） |
| 用户上传文件 | 同上 |

## 8.2 访问控制

```
用户 → 自己的 BidJob  ←  越权拒绝
用户 → 共享的 Material  ←  通过 ACL 控制
```

RBAC：
- `user`：创建/查看自己的 bid
- `admin`：管理知识库、用户

## 8.3 输入校验

- 文件大小限制（≤ 100MB/文件）
- 文件类型白名单（PDF/Word/Markdown）
- RFP 解析前 sandbox 扫描
- LLM 输出 JSON schema 校验
- 文件路径校验（防目录穿越）

## 8.4 限流

```python
# FastAPI 限流
from slowapi import Limiter

@limiter.limit("10/minute")
async def create_bid(request: Request, ...): ...

# LLM 调用限流
@llm_limiter.limit("60/minute", key="user_id")
async def chat(...): ...
```

---

# 九、部署架构

## 9.1 MVP 部署（Docker Compose）

```yaml
# docker-compose.yml
services:
  api:
    image: bid-system/api:latest
    ports: ["8000:8000"]
    environment:
      DATABASE_URL: postgresql://...
      REDIS_URL: redis://...
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
    depends_on: [postgres, redis, minio]
  
  worker:
    image: bid-system/api:latest
    command: celery -A bid_system.worker worker -l info -Q default,chapters
    environment:
      DATABASE_URL: postgresql://...
      REDIS_URL: redis://...
      ANTHROPIC_API_KEY: ${ANTHROPIC_API_KEY}
    depends_on: [postgres, redis, minio]
    deploy:
      replicas: 3  # 章节并行
  
  beat:
    image: bid-system/api:latest
    command: celery -A bid-system.worker beat -l info
    depends_on: [postgres, redis]
  
  postgres:
    image: postgres:16
    volumes: ["pgdata:/var/lib/postgresql/data"]
  
  redis:
    image: redis:7-alpine
  
  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    volumes: ["miniodata:/data"]
  
  prometheus:
    image: prom/prometheus
    volumes: ["./monitoring/prometheus.yml:/etc/prometheus/prometheus.yml"]
  
  grafana:
    image: grafana/grafana
    ports: ["3000:3000"]
```

## 9.2 资源需求估算

| 服务 | CPU | 内存 | 磁盘 | 备注 |
|---|---|---|---|---|
| API | 2 | 4GB | - | |
| Worker | 4 | 8GB | - | 章节并行 |
| PostgreSQL | 2 | 8GB | 100GB | 索引 + 元数据 |
| Redis | 1 | 2GB | 10GB | 任务队列 + 缓存 |
| MinIO | 1 | 2GB | 500GB | 章节正文、图表 |
| Prometheus | 1 | 2GB | 50GB | 7 天指标 |
| Grafana | 0.5 | 1GB | 5GB | |

按 50 章节/天的吞吐量，单实例足够。

---

# 十、MVP 实施路径

## 10.1 阶段划分

| 阶段 | 周 | 目标 | 关键交付 |
|---|---|---|---|
| **P0** | 1-2 | 章节规划器原型 | FastAPI + Planner + 单章节端到端 |
| **P1** | 3-4 | 章节级并发 | Celery + 10 并发 + Mermaid |
| **P2** | 5-6 | 端到端 MVP | 跨章节审计 + 人在回路 + 标书汇总 |
| **P3** | 7-8 | 行业化 | 多类型图表 + 多 provider + Word 模板 |
| **P4** | 9+ | 规模化 | K8s + 多租户 + 协作 |

## 10.2 P0 阶段详细拆解

**P0.1（1 周）**：
- FastAPI 骨架 + PostgreSQL + Redis
- 用户上传 RFP
- Planner Task：LLM 生成章节大纲
- 人在回路点 1：用户确认大纲

**P0.2（1 周）**：
- Writer Task：单章节撰写（不并发）
- 章节 Markdown → docx
- 简单 Word 模板

**P0 验收**：从一份 RFP + 1 个章节素材，生成 1 章节 docx。

---

# 十一、风险与缓解

| 风险 | 影响 | 缓解 |
|---|---|---|
| Claude API 限流 | 端到端阻塞 | 多 provider fallback + 队列缓冲 |
| Mermaid 渲染失败率高 | 图表质量 | 本地 fallback + 占位图 |
| 章节状态机卡死 | 任务挂起 | 心跳 + 超时 + watchdog |
| 文件系统满 | 写入失败 | 监控 + 归档策略 |
| 并发写入冲突 | 数据不一致 | Redis 锁 + PostgreSQL 事务 |
| LLM 成本失控 | 财务风险 | 预算限制 + Prompt 缓存 + 双层模型 |
| 人在回路点超时 | 任务永远卡住 | 24h 超时自动恢复（用默认值） |

---

# 十二、关键决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 章节任务并发度 | 默认 10 | 平衡 provider 限流与端到端延迟 |
| 章节内串行 | 强制串行 | 保证 Prompt 缓存复用 |
| 任务组锁 | Redis SETNX → PG advisory | MVP 简单、规模化升级 |
| 人在回路点 | 3 个（大纲/审计/样式） | 合规要求 + 工程纪律 |
| 文档存储 | S3 兼容（MinIO 起步） | 多副本 + 易于迁移 |
| 缓存策略 | Anthropic cache_control | 主动控制、命中率高 |
| 错误恢复 | 状态机 + 重做接口 | 用户资产安全 |
| 部署 | Docker Compose → K8s | 渐进式 |

---

# 附录 A：关键模块目录结构

```
bid-system/
├── apps/
│   ├── api/                    # FastAPI 应用
│   │   ├── main.py
│   │   ├── routers/
│   │   │   ├── bids.py
│   │   │   ├── chapters.py
│   │   │   ├── illustrations.py
│   │   │   └── audit.py
│   │   ├── deps.py            # 依赖注入
│   │   └── schemas/           # Pydantic 模型
│   └── worker/                # Celery worker
│       ├── celery_app.py
│       ├── tasks/
│       │   ├── planner.py
│       │   ├── writer.py
│       │   ├── illustrator.py
│       │   ├── chapter_auditor.py
│       │   ├── cross_auditor.py
│       │   └── assembler.py
│       └── chains/
│           └── chapter_pipeline.py
├── packages/
│   ├── llm/                   # LLM 路由
│   │   ├── router.py
│   │   ├── providers/
│   │   │   ├── anthropic.py
│   │   │   ├── openai.py
│   │   │   └── deepseek.py
│   │   ├── prompts/           # Prompt 模板
│   │   └── repair.py          # JSON 修复
│   ├── kb/                    # 知识库
│   │   ├── indexer.py
│   │   ├── searcher.py
│   │   └── evidence.py
│   ├── docs/                  # 文档处理
│   │   ├── docx_builder.py
│   │   ├── pdf_renderer.py
│   │   └── templates/
│   └── orchestrator/          # 编排器
│       ├── state_machine.py
│       └── human_loop.py
├── storage/                   # 文件存储（gitignored）
├── migrations/                # Alembic
├── tests/
│   ├── unit/
│   ├── integration/
│   └── e2e/
├── monitoring/
│   ├── prometheus.yml
│   └── grafana/
├── docker-compose.yml
├── Dockerfile
└── pyproject.toml
```

# 附录 B：错误码

| 错误码 | 含义 | 触发条件 |
|---|---|---|
| BID_RFP_PARSE_FAILED | RFP 解析失败 | 文件格式不支持 / 加密 |
| BID_PLANNER_FAILED | 章节规划失败 | LLM 多次重试仍失败 |
| BID_CHAPTER_LOCK_TIMEOUT | 章节锁获取超时 | 同章节被并发 |
| BID_CHAPTER_RETRY_EXHAUSTED | 章节重试耗尽 | max_retries 用完 |
| BID_LLM_ALL_PROVIDERS_FAILED | 所有 LLM provider 失败 | 全部重试 + 降级用尽 |
| BID_ILLUSTRATION_FAILED | 图表生成失败 | 全部 fallback 用尽 |
| BID_AUDIT_CRITICAL_ISSUES | 审计有严重问题 | 阻断汇总 |
| BID_HUMAN_LOOP_TIMEOUT | 人在回路点超时 | 24h 未处理 |
| BID_EXPORT_FAILED | 导出失败 | LibreOffice 异常 |
| BID_STORAGE_FULL | 存储满 | 磁盘满 |

---

文档完成。本概要设计可直接作为开发启动的依据；具体实现细节（单元函数、SQL 调优、UI 布局）将在详细设计阶段补充。