# ai_toubiao · AI 标书自动生成系统

> 本仓库承载 **AI 标书自动生成系统** 的产品调研、需求分析、架构设计与后端/前端实现代码。

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
    API["API 网关<br/>Gin + JWT + 限流"]

    %% 核心服务
    subgraph SERVICES["核心服务"]
        ORCH["编排服务<br/>状态机 + 调度 + HIL"]
        KB["知识库服务<br/>检索 + 证据链 + pgvector"]
        DOC["文档服务<br/>Word 模板 + 渲染"]
        AUDIT["审计服务<br/>合规 + 一致性 + 废标项"]
        TEMPLATE["模板服务<br/>Word 模板管理"]
        BILLING["账单服务<br/>预算 + 交易记录"]
        NOTIFY["通知服务<br/>邮件/钉钉/企微"]
    end

    %% 任务队列
    subgraph QUEUES["任务队列（Asynq + Redis）"]
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
        MT["go-echarts"]
        TB["自实现表格"]
    end

    %% 存储
    subgraph STORE["存储"]
        PG[("PostgreSQL<br/>元数据 + 向量")]
        RD[("Redis<br/>队列 + 锁 + 缓存")]
        S3[("S3 / MinIO<br/>章节 + 图表 + Word/PDF")]
    end

    %% 主流程
    UP --> API
    HIL --> API
    API --> ORCH
    API --> DOC
    API --> KB
    API --> AUDIT
    API --> TEMPLATE
    API --> BILLING
    API --> NOTIFY

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

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.22 + Gin |
| 前端 | React 18 + TypeScript + Vite + Tailwind CSS |
| 数据库 | PostgreSQL + pgvector |
| 任务队列 | Asynq + Redis |
| AI 路由 | Anthropic / OpenAI / DeepSeek 多 Provider 路由 |
| 文档生成 | unioffice (Word) |
| 图表 | Mermaid + go-echarts |
| 部署 | Docker Compose |

## 项目结构

```
ai_toubiao/
├── backend/                    # Go 后端（微服务）
│   ├── services/
│   │   ├── api-gateway/       # API 网关（认证、限流、路由）
│   │   ├── project-svc/       # 项目管理
│   │   ├── router-svc/        # AI 路由（多 Provider）
│   │   ├── workflow-svc/     # 工作流编排（状态机）
│   │   ├── document-svc/     # 文档解析与导出
│   │   ├── knowledge-svc/     # 知识库（RAG + pgvector）
│   │   ├── audit-svc/       # 合规审查
│   │   ├── template-svc/     # Word 模板管理
│   │   ├── billing-svc/      # 账单与预算
│   │   └── notify-svc/       # 通知服务
│   ├── shared/               # 共享包（db、logger、tenant、httperr）
│   └── migrations/          # 数据库迁移（goose）
├── web/                      # React 前端
│   ├── src/
│   │   ├── api/             # API 客户端
│   │   ├── components/      # 通用组件
│   │   ├── pages/          # 页面
│   │   │   ├── auth/       # 登录
│   │   │   ├── bids/        # 标书管理（列表、详情、大纲、章节、审计、导出）
│   │   │   └── knowledge/  # 知识库
│   │   ├── lib/             # 状态管理（Zustand）
│   │   └── hooks/           # 自定义 Hooks
│   └── package.json
└── docs/                    # 设计文档
```

## 服务状态

| 服务 | 状态 | 说明 |
|------|------|------|
| api-gateway | 完整 | JWT 认证、限流、路由代理 |
| project-svc | 完整 | Project CRUD、多租户隔离 |
| router-svc | 完整 | AI Provider 路由、LRU 缓存 |
| workflow-svc | 完整 | 状态机、事件溯源 |
| document-svc | 部分 | 解析完成，存储待实现 S3/MinIO |
| knowledge-svc | 部分 | 向量搜索待完善 |
| audit-svc | 完整 | 合规审查、一致性检查、废标项扫描 |
| template-svc | 完整 | Word 模板 CRUD |
| billing-svc | 完整 | 预算管理、交易记录 |
| notify-svc | 完整 | 多渠道通知偏好 |

## 数据库迁移

| 迁移 | 内容 |
|------|------|
| 00006 | bid_jobs、chapter_specs、chapter_contents、illustrations、evidence |
| 00007 | kb_materials、kb_chunks、kb_evidence_links（知识库） |
| 00008 | audit_reports、audit_issues（审计） |
| 00009 | word_templates（模板） |
| 00010 | billing_budgets、billing_transactions（账单） |
| 00011 | notification_preferences、notification_logs（通知） |

## API 概览

| 端点 | 说明 |
|------|------|
| `POST /api/v1/auth/login` | 用户登录 |
| `GET/POST /api/v1/projects` | 项目管理 |
| `GET/POST /api/v1/bids` | 标书管理 |
| `GET /api/v1/bids/:id/outline` | 章节大纲 |
| `GET /api/v1/bids/:id/chapters` | 章节内容 |
| `POST/GET /api/v1/audit/bidjobs/:id/report` | 审计报告 |
| `POST/GET /api/v1/templates` | 模板管理 |
| `GET/POST /api/v1/billing/budget` | 预算管理 |
| `POST /api/v1/notifications/send` | 发送通知 |

## 文档索引

### 需求基线

| 文档 | 内容 |
|---|---|
| [docs/requirements-spec.md](docs/requirements-spec.md) | **需求规格说明书 SRS**（9 节） |
| [docs/diaoyan.md](docs/diaoyan.md) | 调研：行业现状、痛点、机会 |

### 设计与架构

| 文档 | 内容 |
|---|---|
| [docs/framework.md](docs/framework.md) | 设计纲要：系统目标、核心三要素 |
| [docs/tech-selection.md](docs/tech-selection.md) | 技术选型（13 节） |
| [docs/high-level-design.md](docs/high-level-design.md) | 概要设计 HLD（15 节） |

## CI

| 检查 | 工具 | 严格度 |
|---|---|---|
| 必需文件存在且非空 | shell | 严格 |
| Markdown 风格 | markdownlint-cli2 | 严格 |
| Mermaid 块渲染 | mermaid.js + Chrome | 严格 |

## 本地开发

### 后端

```bash
cd backend

# 运行所有服务（Docker Compose）
docker-compose up

# 或本地运行单个服务
go run ./services/api-gvc/cmd/api-gateway

# 数据库迁移
goose -dir migrations postgres "postgres://user:pass@localhost:5432/db" up
```

### 前端

```bash
cd web
npm install
npm run dev
```

### 校验 Mermaid 图

```bash
npm install
npm run lint:mermaid
```

## License

Private · 仅供内部使用

## 测试与基准

| 类别 | 数量 | 说明 |
|---|---|---|
| 后端 Test 函数 | 232 | 33 个 `_test.go`,覆盖 25 个包,0 失败 |
| 前端 vitest | 23 | 4 个 `*.test.{ts,tsx}`(LoginPage, Layout, auth store, bids api) |

性能基准 (Go `testing.B`):

| 包 | 基准 | 当前基线 (i5-8400) |
|---|---|---|
| `shared/pkg/httperr` | `BenchmarkWrite` / `BenchmarkWriteNoDetails` | 863 ns · 456 ns |
| `shared/pkg/validator` | `Validate` / `hex64` / `mime` | 1.1 µs · 747 ns · 486 ns |
| `api-gateway/.../ratelimit` | `AllowSingleKey` / `ManyKeys` / `Concurrent` | 75 ns · 89 ns · 170 ns |
| `workflow-svc/.../ooxml` | `DefaultOutline` / `LargeBid` | 219 µs · 233 µs |

运行方式见 `CHANGELOG.md` 末尾。
