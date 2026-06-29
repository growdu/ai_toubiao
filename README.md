# ai_toubiao · AI 标书自动生成系统

> 本仓库承载 **AI 标书自动生成系统** 的产品调研、需求分析与设计文档。
> 不包含代码实现 —— 代码位于 `OpenBidKit_Yibiao`（Electron 客户端）与 `bidwriter`（Go 后端）。

📘 **在线文档（GitHub Pages）**：<https://growdu.github.io/ai_toubiao/>

## 系统架构

```mermaid
flowchart TB
    %% 客户端
    subgraph CLIENT["客户端（Web / Desktop）"]
        UP["上传 RFP + 材料"]
        HIL["人在回路交互<br/>3 个暂停点"]
        DL["下载 Word / PDF"]
    end

    %% API
    API["API 网关<br/>FastAPI + OpenAPI"]

    %% 核心服务
    subgraph SERVICES["核心服务"]
        ORCH["编排服务<br/>状态机 + 调度 + HIL"]
        KB["知识库服务<br/>检索 + 证据链"]
        DOC["文档服务<br/>Word 模板 + 渲染"]
    end

    %% 任务队列
    subgraph QUEUES["任务队列（Celery + Redis）"]
        QP["planner-q"]
        QC["chapter-q<br/>默认 10 并发"]
        QA["auditor-q"]
        QX["export-q"]
    end

    %% AI
    subgraph AI["AI 路由层"]
        LLM["LLM Router<br/>多 provider + Prompt 缓存"]
    end

    %% 图表
    subgraph ILL["图表渲染层"]
        M["Mermaid"]
        IAI["DALL-E 3"]
        MT["matplotlib"]
        TB["自实现表格"]
    end

    %% 存储
    subgraph STORE["存储"]
        PG[("PostgreSQL<br/>元数据 + 索引")]
        RD[("Redis<br/>队列 + 锁 + 缓存")]
        S3[("S3 / MinIO<br/>章节 + 图表 + Word/PDF")]
    end

    %% 主流程
    UP --> API
    HIL --> API
    API --> ORCH
    API --> DOC
    API --> KB

    ORCH --> QP
    ORCH --> QC
    ORCH --> QA
    ORCH --> QX

    QC --> KB
    QC --> LLM
    QC --> ILL
    QA --> LLM
    QX --> DOC

    LLM -.->|"HTTPS"| LLMEXT[("Claude / DeepSeek / GPT")]

    ORCH --> PG
    ORCH --> RD
    KB --> PG
    DOC --> S3
    ILL --> S3

    DOC --> DL

    classDef external fill:#fff,stroke:#999,stroke-dasharray: 5 5
    class LLMEXT external
```

> 主输出格式：**Word（.docx）**，PDF 为衍生品（LibreOffice headless 异步生成）。
> 完整 ASCII 架构图与组件职责详见 [docs/high-level-design.md §2](docs/high-level-design.md)。

## 文档索引

### 需求基线

| 文档 | 内容 |
|---|---|
| [docs/requirements-spec.md](docs/requirements-spec.md) | **需求规格说明书 SRS**（9 节）：术语表 / 痛点 / 8 大功能模块 / 非功能 / 9 大技术难点 / 验收 / 风险 / MVP 优先级。研发需求基线 |
| [docs/diaoyan.md](docs/diaoyan.md) | 调研：行业现状、痛点、机会、目标用户 |

### 设计与架构

| 文档 | 内容 |
|---|---|
| [docs/framework.md](docs/framework.md) | 设计纲要：系统目标、核心三要素（AI/章节任务/图表）、状态机、人在回路点 |
| [docs/tech-selection.md](docs/tech-selection.md) | 技术选型（13 节）：后端 / 队列 / LLM 路由 / 图表 / Word / KB / 编排 / 存储 / 可观测 / 部署 / 成本 / 风险 / 决策 |
| [docs/high-level-design.md](docs/high-level-design.md) | 概要设计 HLD（15 节）：组件架构、核心流程、**章节划分与调度 ★**、**图表设计与实现 ★**、**Word 输出流水线 ★**、数据模型、接口、算法、可观测、安全、部署 |

### HLD 重点章节速查

| 章节 | 解决的问题 |
|---|---|
| §4 章节划分与调度 | 章节怎么分？优先级？依赖？并发？防饿死？ |
| §5 图表设计与实现 | 图表分几类？怎么定义？怎么渲染？怎么校验？失败怎么办？ |
| §6 Word 输出流水线 | 为什么 Word 为主？模板怎么用？Markdown 怎么变 docx？图表怎么嵌？ |

### 需求-设计追溯关系

```
需求书（产品）→ requirements-spec.md（需求基线）
    → framework.md（设计纲要）
        → tech-selection.md（技术选型）
            → high-level-design.md（概要设计）
                → 详细设计 + 实现
```

下游文档变更必须反向检查上游；上游变更需评估对所有下游的影响面。

## 关键决策

- **主输出格式**：Word（.docx），PDF 为衍生品（LibreOffice headless 异步生成）
- **章节任务并发度**：默认 10（章节间并行，章节内串行）
- **人在回路点**：3 个（章节大纲确认 / 审计问题处理 / 样式微调）
- **Prompt 缓存**：Anthropic cache_control（系统前缀强缓存，章节规格章节内复用）

## 关联仓库

- **OpenBidKit_Yibiao**（Electron 客户端）：`/work/ai/OpenBidKit_Yibiao`
- **bidwriter**（Go 后端）：`/work/ai/bidwriter`

## CI

| 检查 | 工具 | 严格度 |
|---|---|---|
| 必需文件存在且非空 | shell | 严格（CI 红） |
| Markdown 风格 | markdownlint-cli2 | 严格（CI 红） |
| Mermaid 块渲染 | mermaid.js + Chrome | 严格（CI 红） |
| 链接检查 | lychee | 宽松（仅 Job Summary） |

工作流：`.github/workflows/ci.yml`，对 `push` 到 `main` 与所有 `pull_request` 触发。

## 本地开发

### 校验 Mermaid 图

CI 会自动渲染 README 与 `docs/` 下所有 `mermaid` 代码块；本地开发可在提交前自检：

```bash
npm install                    # 首次：安装 mermaid + puppeteer-core
npm run lint:mermaid           # 校验默认 docs/**/*.md + README.md
MERMAID_LINT_VERBOSE=1 npm run lint:mermaid   # 打印每个块的行号
node tools/mermaid-lint.mjs README.md         # 只校验某个文件
```

需本机已安装 Chrome / Chromium；非默认路径可用 `MERMAID_LINT_CHROME` 环境变量指定。

## License

Private · 仅供内部使用
