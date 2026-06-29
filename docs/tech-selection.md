# 技术选型文档

> 本文档基于 `docs/framework.md` 的设计纲要，对每个技术领域给出推荐选型与候选对比。
> 选型原则：MVP 阶段优先成熟度与生态丰富度；后续按性能、成本、可维护性逐项替换。

---

# 〇、选型原则

1. **生态优先**：AI/LLM 生态最丰富的语言是 Python，业务核心用 Python
2. **可替换**：每个组件都要有清晰的抽象边界，便于后续替换
3. **可降级**：AI 调用、图表生成、文档转换都要支持 fallback
4. **可观测**：从第一天起就接日志/指标/追踪
5. **本地优先**：MVP 阶段单机可跑，避免过早分布式

---

# 一、整体架构选型

## 1.1 三种候选

| 方案 | 优点 | 缺点 | 适合场景 |
|---|---|---|---|
| **单体（Python FastAPI）** | 部署简单、AI 生态强、异步支持好 | 单机性能上限 | **MVP 推荐** |
| 微服务（多语言） | 各组件独立扩展 | 复杂度高、AI 生态分裂 | 后期规模化 |
| Serverless | 弹性好、按量付费 | 冷启动、AI 长任务不友好 | 突发场景 |

**推荐：单体 Python FastAPI**，内置异步任务队列。

## 1.2 技术栈全景

```
┌──────────────────────────────────────────────────────────┐
│  客户端（可选）                                           │
│  - Web: React 19 + TypeScript + Vite 7                  │
│  - Desktop: Electron（参考 openbidkit-yibiao）           │
└──────────────────────────────────────────────────────────┘
                          ↓ HTTPS / IPC
┌──────────────────────────────────────────────────────────┐
│  服务端（Python FastAPI）                                │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ API 层     │  │ 任务层     │  │ AI 编排层  │         │
│  │ FastAPI    │  │ Celery     │  │ LangChain  │         │
│  └────────────┘  └────────────┘  └────────────┘         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ 存储层     │  │ 图表层     │  │ 文档层     │         │
│  │ SQLAlchemy │  │ Mermaid/SD │  │ python-docx│         │
│  └────────────┘  └────────────┘  └────────────┘         │
└──────────────────────────────────────────────────────────┘
                          ↓
┌──────────────────────────────────────────────────────────┐
│  基础设施                                                │
│  - PostgreSQL / SQLite  - Redis（broker/cache）          │
│  - S3 兼容对象存储        - Prometheus + Grafana         │
└──────────────────────────────────────────────────────────┘
```

---

# 二、后端语言与框架

## 2.1 候选对比

| 语言 | AI 生态 | 并发模型 | 性能 | 类型系统 |
|---|---|---|---|---|
| **Python** | ⭐⭐⭐⭐⭐ | asyncio + 多进程 | 中 | 动态 + type hints |
| Node.js | ⭐⭐⭐ | 事件循环 | 中高 | TypeScript |
| Go | ⭐⭐ | goroutine | 高 | 静态 |
| Rust | ⭐⭐ | async/await | 极高 | 静态 |

**推荐：Python 3.11+**

理由：
- AI/LLM SDK 最完整（Anthropic / OpenAI / Cohere / DeepSeek 官方 SDK 都是 Python 优先）
- LangChain / LlamaIndex / DSPy 等编排框架是 Python 原生
- 数据处理生态（pandas / pydantic / numpy）成熟
- 异步 IO 已经成熟（asyncio + httpx）

## 2.2 Web 框架

| 框架 | 优点 | 缺点 |
|---|---|---|
| **FastAPI** | 异步原生、自动 OpenAPI、Pydantic 集成 | 生态相对新 |
| Flask | 极简、成熟 | 同步为主，需手动集成 async |
| Django | 全栈、ORM 强 | 太重、AI 集成不便 |

**推荐：FastAPI**

理由：异步原生契合章节级并发的 IO 密集型场景；Pydantic v2 适合章节规格/响应矩阵等结构化数据。

---

# 三、任务调度与并发

## 3.1 任务队列

| 选项 | 优点 | 缺点 |
|---|---|---|
| **Celery** | 成熟、Redis/RabbitMQ broker、丰富的 retry/canvas | 较重、文档分散 |
| Dramatiq | 轻量、Actor 模型 | 生态小 |
| RQ | 极简 | 功能单薄 |
| APScheduler | 定时任务为主 | 不适合长任务链 |
| Temporal | 工作流引擎、状态机友好 | 学习曲线陡、运维重 |
| **arq + Redis** | 异步、轻量、原生 asyncio | 文档少 |

**推荐：Celery**（MVP）/ **arq**（如果纯异步需求）

理由：
- 章节任务有重试/超时/优先级需求，Celery 原生支持
- canvas（chain/group/chord）正好契合章节级并行 + 章节内串行的模式
- broker 用 Redis 起步（同时充当缓存）

## 3.2 任务模式实现

章节级并行 + 章节内串行：

```python
# 章节规划完成后
chapters = [ch1, ch2, ch3, ...]  # 50 个章节
job = celery_group(
    chapter_pipeline.s(ch.id) for ch in chapters  # 每章节独立
)
job.apply_async()

@celery_app.task(bind=True, max_retries=3)
def chapter_pipeline(self, chapter_id):
    # 章节内串行：检索 → 撰写 → 图表 → 内审
    materials = retrieve_materials(chapter_id)            # 串行 1
    content = write_chapter(chapter_id, materials)        # 串行 2
    illustrations = generate_illustrations(chapter_id)    # 串行 3
    audit = chapter_audit(chapter_id, content, illustrations)  # 串行 4
    return {"chapter_id": chapter_id, "audit": audit}
```

## 3.3 任务组锁

Celery 不直接提供任务组锁，可用以下方式实现：
- **Redis SETNX + EX**：每个 chapter_id 一个锁 key
- **PostgreSQL advisory lock**：`pg_advisory_lock(hashtext(chapter_id))`
- **etcd / ZooKeeper**：分布式锁，但太重

**推荐：Redis SETNX**（MVP）→ **PostgreSQL advisory lock**（规模化）

---

# 四、数据库与存储

## 4.1 元数据库

| 选项 | 优点 | 缺点 |
|---|---|---|
| **PostgreSQL** | JSONB、advisory lock、全文检索 | 部署稍重 |
| SQLite | 零运维、单文件 | 并发写弱 |
| MySQL | 成熟 | JSON 支持弱 |

**推荐：PostgreSQL 16+**

理由：
- JSONB 完美契合 chapter_specs / chapter_contents 的结构化字段
- advisory lock 实现任务组锁
- 全文检索（tsvector）可用于知识库章节检索
- LISTEN/NOTIFY 可用于章节状态实时推送

## 4.2 ORM

| 选项 | 优点 | 缺点 |
|---|---|---|
| **SQLAlchemy 2.0** | 异步支持、类型友好、生态最广 | 学习曲线 |
| SQLModel | SQLAlchemy + Pydantic 结合 | 较新、生态小 |
| Tortoise ORM | Django-like、异步 | 生态小 |
| Piccolo | 异步、类型友好 | 太小众 |

**推荐：SQLAlchemy 2.0（async）**

理由：成熟度最高，章节规格等复杂模型用 declarative + relationship 表达清晰。

## 4.3 文件存储

| 选项 | 场景 |
|---|---|
| **本地文件系统** | MVP 单机部署 |
| **S3 兼容**（MinIO / 阿里云 OSS / AWS S3） | 规模化、多副本 |
| 分布式文件系统（cephfs / juicefs） | 大规模、跨节点 |

**目录结构**：
```
storage/
 ├── chapters/{chapter_id}/content_v{n}.md
 ├── illustrations/{illustration_id}/source.{ext}
 ├── illustrations/{illustration_id}/rendered.{png|svg}
 ├── tenders/{tender_id}/rfp.pdf
 └── evidence/{evidence_id}/source.{ext}
```

**推荐：MVP 用本地文件系统 + boto3 抽象**（便于后续切到 S3）

---

# 五、AI / LLM 选型

## 5.1 主力模型

| 模型 | 上下文 | Prompt 缓存 | 中文 | 成本（输入/输出 $/M token） |
|---|---|---|---|---|
| **Claude Sonnet 4.6** | 200K | ✅ cache_control | 优 | 3 / 15 |
| GPT-4o | 128K | ✅ 自动 | 良 | 2.5 / 10 |
| GPT-4o mini | 128K | ✅ 自动 | 良 | 0.15 / 0.6 |
| DeepSeek V3 | 64K | ✅ | 优 | 0.27 / 1.1 |
| Qwen Max | 128K | ✅ | 优 | 0.34 / 1.4 |

**推荐主力：Claude Sonnet 4.6**

理由：
- 200K 上下文可容纳章节规格 + 全部章节素材 + global_facts
- `cache_control` 主动控制缓存前缀，命中率高，**可降本 5-10 倍**
- 写作质量稳定，结构化输出（JSON）容错好
- Tool use 能力强

**推荐备选**：
- 成本敏感场景：DeepSeek V3（中文场景强，价格 1/10）
- 多模态/简单任务：GPT-4o mini（速度最快）

## 5.2 双层模型策略

```
粗稿/规划阶段：便宜模型（DeepSeek V3 / GPT-4o mini）
  → 章节规格、章节大纲、章节素材筛选

精稿/写作阶段：主力模型（Claude Sonnet 4.6）
  → 章节正文、图表描述、响应矩阵

校对阶段：主力模型 + agent 模式
  → 跨章节一致性审计
```

## 5.3 Prompt 缓存实现

Anthropic 的 `cache_control` 关键点：

```python
response = client.messages.create(
    model="claude-sonnet-4-6",
    system=[
        {
            "type": "text",
            "text": GLOBAL_FACTS +  # 系统级共享，放最前
        },
        {
            "type": "text",
            "text": CHAPTER_SPEC,
            "cache_control": {"type": "ephemeral"}  # 标记可缓存
        }
    ],
    messages=[{"role": "user", "content": ...}]
)
```

缓存位置策略：
1. `system` 段 1：global_facts + 术语表（所有章节共享）→ **强缓存**
2. `system` 段 2：章节规格（本章节唯一）→ 章节内复用
3. `messages`：上一轮对话（同章节多轮扩写时复用前缀）

## 5.4 AI 调用抽象

```python
class LLMProvider(Protocol):
    async def chat(self, messages, *, system, max_tokens, temperature,
                   cache_control=None) -> ChatResponse: ...

class AnthropicProvider:
    async def chat(self, ...): ...

class OpenAIProvider:
    async def chat(self, ...): ...

class DeepSeekProvider:
    async def chat(self, ...): ...

# Router：按 task 名选择 provider，支持 fallback
class LLMRouter:
    async def route(self, task_name: str, messages, **kw) -> ChatResponse:
        for provider in self.providers_for(task_name):
            try:
                return await provider.chat(messages, **kw)
            except RetryableError:
                continue  # 下一个 provider
        raise AllProvidersFailed(...)
```

## 5.5 JSON 修复链路

```python
async def parse_json_response(content: str, schema: Type[T]) -> T:
    # 1. 直接解析
    try: return schema.model_validate_json(content)
    except ValidationError: pass

    # 2. 抽取第一个 {...} 块
    extracted = extract_first_json_block(content)
    try: return schema.model_validate_json(extracted)
    except ValidationError: pass

    # 3. 局部修补（不重发）
    repaired = repair_json(extracted, schema)
    try: return schema.model_validate_json(repaired)
    except ValidationError:
        # 4. 重发一次（仅发送错误信息 + 原内容，不发整个 prompt）
        return await retry_with_repair(...)
```

---

# 六、图表生成选型

## 6.1 Mermaid（流程/架构/时序图）

| 渲染方式 | 优点 | 缺点 |
|---|---|---|
| **mermaid.ink（在线 API）** | 零部署 | 依赖外网、有频率限制 |
| **mermaid-cli（本地）** | 无外网依赖 | 需要 Node.js + puppeteer |
| **kroki（统一 API）** | 多图表统一 | 部署重 |

**推荐：mermaid.ink 主用 + mermaid-cli 本地 fallback**

实现：

```python
async def render_mermaid(source: str) -> bytes:
    # 1. 校验语法
    validate_mermaid_syntax(source)

    # 2. 主用：mermaid.ink
    try:
        return await fetch_mermaid_ink(source, timeout=15)
    except (Timeout, NetworkError):
        pass  # 降级

    # 3. 降级：本地 mermaid-cli
    try:
        return await render_mermaid_local(source)
    except MermaidRenderError:
        pass

    # 4. 占位图
    return generate_placeholder_image("Mermaid 渲染失败")
```

## 6.2 AI 图（示意图/封面图/组织架构图）

| 提供商 | 强项 | 成本 |
|---|---|---|
| **DALL-E 3**（OpenAI） | 通用、风格可控 | $0.04/张 |
| **Stable Diffusion 3** | 可本地部署 | GPU 成本 |
| **Midjourney** | 质量最高 | $0.05+/张 |
| **Replicate** | 多模型聚合 | 按模型定价 |
| 国产（通义万相 / 文心一格） | 中文友好 | ¥0.1-0.3/张 |

**推荐：DALL-E 3 主用 + Replicate（备选）**

理由：
- 同一份标书需要风格统一，DALL-E 3 风格可控性最强
- 失败时降级到 Replicate 上的其他模型
- 国产模型作为中文场景补充

## 6.3 表格 / 响应矩阵

直接生成 HTML / JSON，自实现 HTML → docx 表格，无需第三方。

## 6.4 数据图表

| 选项 | 优点 | 缺点 |
|---|---|---|
| **matplotlib** | Python 原生、可控 | 样式老 |
| plotly | 交互式、样式现代 | 输出体积大 |
| echarts | 样式丰富 | 需要前端渲染 |

**推荐：matplotlib（生成静态图）+ plotly（需要交互时）**

数据来源走 evidence chain，无数据不生成。

---

# 七、文档导出选型

## 7.1 Word（.docx）

| 库 | 优点 | 缺点 |
|---|---|---|
| **python-docx** | 主流、API 稳定 | 表格/样式处理繁琐 |
| docxtpl | 模板渲染 | 复杂逻辑不便 |
| pandoc | Markdown → docx 强 | 样式定制弱 |
| LibreOffice headless | 完美兼容 Word | 重、启动慢 |

**推荐：python-docx + 自实现 HTML → docx**

理由：
- 标书样式复杂（页眉页脚、目录、序号、图表编号）
- 走 Markdown → AST → docx，绕过 headless browser
- 模板套用：解析用户提供的 docx 模板，提取样式后套用

## 7.2 PDF

| 选项 | 优点 | 缺点 |
|---|---|---|
| **LibreOffice headless** | docx → pdf 一致性最好 | 重 |
| weasyprint | HTML → pdf 灵活 | 与 docx 样式不同 |
| pdfkit（wkhtmltopdf） | 成熟 | 维护停滞 |

**推荐：LibreOffice headless**

理由：docx → pdf 样式一致性最关键（标书格式不能变），其他方案都会有偏差。

## 7.3 Mermaid → docx 内嵌

```python
def embed_mermaid_in_docx(doc, illustration):
    rendered_path = illustration.rendered_path  # PNG/SVG
    # 1. 插入图片
    doc.add_picture(rendered_path, width=Cm(14))
    # 2. 编号（图 3-2）
    last_paragraph = doc.paragraphs[-1]
    last_paragraph.alignment = WD_ALIGN_PARAGRAPH.CENTER
    # 3. 图注
    caption = doc.add_paragraph(f"图 {illustration.chapter_number}-{illustration.order} {illustration.title}")
    caption.alignment = WD_ALIGN_PARAGRAPH.CENTER
```

---

# 八、知识库与检索

## 8.1 检索方案

framework.md 提出"你不需要 RAG"哲学，技术实现：

| 方案 | 适用 |
|---|---|
| **目录树 + 关键词**（PostgreSQL 全文检索） | MVP 推荐 |
| BM25 + 关键词 | 简单场景 |
| 向量检索（pgvector / Qdrant） | 复杂语义检索 |
| 混合检索（关键词 + 向量） | 规模化 |

**推荐：MVP 用 PostgreSQL tsvector 全文检索；后续加 pgvector 做混合检索**

理由：标书场景召回率 > 相似度，目录树 + 关键词足够。

## 8.2 文档预处理

```python
# 文档统一走 doc2markdown 中间态
async def ingest_document(path: str) -> Document:
    if path.endswith('.pdf'):
        content = await pdf_to_markdown(path)
    elif path.endswith('.docx'):
        content = await docx_to_markdown(path)
    else:
        content = read_file(path)
    return Document(
        path=path,
        content=content,
        chunks=split_into_chunks(content),
        metadata=extract_metadata(path)
    )
```

---

# 九、客户端（可选）

## 9.1 桌面客户端（参考 openbidkit-yibiao）

| 技术 | 用途 |
|---|---|
| **Electron 41+** | 跨平台桌面壳 |
| React 19 + TypeScript 5.9 | UI |
| Vite 7 | 构建 |
| Radix UI / shadcn/ui | 组件库 |
| Zustand / Redux Toolkit | 状态管理 |

## 9.2 Web 客户端

| 技术 | 用途 |
|---|---|
| **React 19 + Vite 7** | SPA |
| TanStack Query | 服务端状态 |
| React Router | 路由 |
| Radix UI | 组件 |

**推荐：MVP 用 Web 优先**（开发快），后续加桌面壳。

---

# 十、基础设施

## 10.1 部署

| 阶段 | 推荐 |
|---|---|
| MVP | Docker Compose 单机 |
| 中期 | K8s（minikube → 生产） |
| 大规模 | 多区域 K8s + CDN |

## 10.2 反向代理

**推荐：Nginx**（成熟）或 **Caddy**（自动 HTTPS）

## 10.3 监控与可观测性

| 维度 | 工具 |
|---|---|
| **指标** | Prometheus + Grafana |
| **日志** | Loki / ELK |
| **追踪** | OpenTelemetry + Jaeger |
| **错误** | Sentry |

## 10.4 CI/CD

**GitHub Actions**（推荐）：
- PR 检查：lint + type check + test
- main 合并：自动构建 Docker 镜像 + 部署

## 10.5 配置管理

| 选项 | 场景 |
|---|---|
| 环境变量 | 简单 |
| **Pydantic Settings** | Python 项目推荐 |
| Consul / etcd | 分布式 |

---

# 十一、成本预估

按 50 章节、每章节 2000 字、每章节 3 张图、Claude Sonnet 4.6 计算：

| 项 | 单价 | 用量 | 小计 |
|---|---|---|---|
| 章节规划 | $3 / 1M tok in | 50K input + 5K output | $0.15 |
| 章节撰写 | $3 / 1M tok in + $15 / 1M tok out | 50 × (30K in + 4K out) = 1.5M in + 200K out | $4.5 + $3 = $7.5 |
| 图表描述 | 同上 | 50 × (10K in + 1K out) | $1.5 + $0.75 |
| AI 图生成 | $0.04/张 | 150 张 | $6 |
| Mermaid 渲染 | 免费（在线 API） | 100 张 | $0 |
| 跨章节审计 | 同章节撰写 | 1 × (200K in + 20K out) | $0.6 + $0.3 |
| **合计** | | | **~$16 / 份标书** |

使用 Prompt 缓存 + DeepSeek 粗稿：可降至 **~$5-8 / 份标书**。

---

# 十二、风险与备选

| 风险 | 备选方案 |
|---|---|
| Claude API 不可用 | DeepSeek → GPT-4o → 本地模型 |
| mermaid.ink 不可用 | 本地 mermaid-cli |
| AI 图风格不一致 | 缓存相似图、固定 prompt 模板 |
| PostgreSQL 单点 | 主从 + PgBouncer |
| Celery broker 单点 | Redis Sentinel / Cluster |

---

# 十三、关键决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 后端语言 | Python 3.11+ | AI 生态最强 |
| Web 框架 | FastAPI | 异步原生、Pydantic 集成 |
| 任务队列 | Celery | 成熟、retry/canvas 完整 |
| 数据库 | PostgreSQL 16 | JSONB + advisory lock + tsvector |
| 主力模型 | Claude Sonnet 4.6 | 200K 上下文 + cache_control |
| 成本模型 | DeepSeek 粗稿 + Claude 精稿 | 双层降本 |
| 文档导出 | python-docx + LibreOffice | 样式一致性 |
| 部署 | Docker Compose MVP → K8s 后期 | 渐进式 |
| CI/CD | GitHub Actions | 与仓库集成最好 |

---

文档完成。下游 `docs/high-level-design.md` 将基于本选型给出组件、数据流、接口的具体设计。