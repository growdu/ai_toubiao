# BidWriter 后端（backend）

> BidWriter 系统的后端工程 —— 一套基于 Go 1.25 的多服务（microservices）实现，
> 负责 AI 标书自动撰写系统的核心业务、状态机、模型路由、知识库与审计能力。

[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.25%2B-blue.svg)](https://go.dev/)
[![Services](https://img.shields.io/badge/services-12-7B68EE)](#服务清单)
[![Postgres](https://img.shields.io/badge/postgres-16%20%2B%20pgvector-336791)](#技术栈)
[![Redis](https://img.shields.io/badge/redis-7-DC382D)](#技术栈)
[![Docs](https://img.shields.io/badge/docs-mkdocs%20material-blue)](docs/index.md)

本目录（`backend/`）是 BidWriter 的**后端 monorepo**。前端（`web/`）、桌面
（`desktop/`）和 Helm Chart（`helm/`）位于同级仓库的其它子目录中。

---

## 目录

- [这是什么](#这是什么)
- [架构总览](#架构总览)
- [服务清单](#服务清单)
- [设计要点](#设计要点)
- [技术栈](#技术栈)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [日常开发](#日常开发)
- [测试与基准](#测试与基准)
- [部署与运维](#部署与运维)
- [API 文档](#api-文档)
- [强规则](#强规则)
- [许可](#许可)

---

## 这是什么

BidWriter 把一份 100–1000 页的非结构化标书，拆解为"结构化任务 + 证据驱动 +
多模型路由"的可执行工作流。后端是这个工作流的承载体，负责：

1. **项目与文档生命周期**：项目/标段 CRUD、文档上传与解析（`.docx`/`.pdf`
   → Markdown）。
2. **五步法工作流（Step01–05）**：解析招标 → 拆解任务 → 生成大纲 →
   全局事实 → 正文生成，由 `workflow-svc` 编排 + Asynq 队列执行。
3. **多模型路由**：`router-svc` 统一封装 OpenAI / Anthropic / DeepSeek /
   Ollama 等 Provider，按成本、质量、可用性自动选型并降级。
4. **三层知识库**：`knowledge-svc` 提供精确匹配 + 弱 RAG + 全局事实，支撑
   章节写作的"有据可查"。
5. **四层一致性审计**：`audit-svc` 跑废标项 / 错别字 / 逻辑 / 查重，必要时
   由 agent 模式自动修复。
6. **计费与通知**：`billing-svc` 计量项目订阅 + Token 用量（硬限保护），
   `notify-svc` 推送结果（邮件 / Webhook / IM）。
7. **统一入口**：`api-gateway` 负责认证、路由、限流、计量、审计日志。

---

## 架构总览

```mermaid
graph LR
    Client[Web / Desktop / CLI] --> GW[api-gateway :8080]
    GW --> PS[project-svc :8081]
    GW --> DS[document-svc :8082]
    GW --> WS[workflow-svc :8083]
    GW --> KS[knowledge-svc :8084]
    GW --> RS[router-svc :8085]
    GW --> TS[template-svc :8086]
    GW --> BS[billing-svc :8087]
    GW --> NS[notify-svc :8088]
    GW --> AS[audit-svc :8089]

    WS --> Asynq[(Asynq 队列<br/>Redis 7)]
    WS --> RS
    AS --> RS
    RS --> Providers[(OpenAI · Anthropic<br/>DeepSeek · Ollama)]

    PS --> PG[(PostgreSQL 16 + pgvector)]
    DS --> PG
    WS --> PG
    KS --> PG
    TS --> PG
    BS --> PG
    AS --> PG
    PS --> R[(Redis 7)]
    WS --> R
    RS --> R
    BS --> R
    NS --> R
    DS --> S3[(S3 / R2 / MinIO)]
    TS --> S3
    NS --> S3
    AS --> S3
```

关键链路（创建一份标书）：

1. `api-gateway` 鉴权后，将请求路由到 `project-svc` / `document-svc`。
2. 用户触发"开始工作流"，`workflow-svc` 写入状态机记录，并向 Asynq
   投递 Step01–05 的任务。
3. Worker 拉起任务后，统一通过 `router-svc` 调用大模型，得到结构化结果。
4. 章节内容写入 PostgreSQL `chapter_content`，原始文档存到 S3。
5. `audit-svc` 在全文生成完成后跑一致性审计，必要时回写修订建议。
6. 用户在 Web 端通过 SSE（`/workflows/{id}/events`）订阅实时进度。

更详细的图与序列图：[docs/architecture/overview.md](docs/architecture/overview.md)、
[docs/architecture/state-machine.md](docs/architecture/state-machine.md)。

---

## 服务清单

| 服务 | 职责 | 端口 | 主要依赖 | 实现状态 |
|---|---|---|---|---|
| `api-gateway` | 认证 / 路由 / 限流 / 计量 / 审计日志 | 8080 | 全部下游、Redis | ✅ 完整 |
| `project-svc` | 项目、标段、成员 CRUD | 8081 | PostgreSQL | ✅ 完整 |
| `document-svc` | 招标 / 素材上传与解析（docx/pdf → Markdown） | 8082 | PostgreSQL、S3 | ✅ 完整 |
| `workflow-svc` | Step01–05 编排、状态机、Asynq 任务 | 8083 | PostgreSQL、Redis、Asynq、router-svc | ✅ 完整 |
| `knowledge-svc` | 知识库 CRUD、pgvector 检索、chunk 管理 | 8084 | PostgreSQL + pgvector | ✅ 完整 |
| `router-svc` | 多模型路由、降级、Prompt 缓存、用量计量 | 8085 | Redis、多 Provider | ✅ 完整 |
| `template-svc` | 行业模板 / 模板市集 | 8086 | PostgreSQL、S3 | ✅ 完整 |
| `billing-svc` | 订阅、Token 计费、硬限保护 | 8087 | PostgreSQL、Redis | ✅ 完整 |
| `notify-svc` | 邮件 / Webhook / IM 推送 | 8088 | Redis、S3 | ✅ 完整 |
| `audit-svc` | 四层一致性审计 + agent 修复 | 8089 | PostgreSQL、S3、router-svc | ✅ 完整 |
| `doc-gen` | 独立的 docx 导出引擎（CLI + service） | 9090 | SQLite / S3 | ✅ 完整 |
| `shared` | 跨服务共享库（不是服务） | — | — | n/a |

每个服务都遵循统一的目录约定：

```text
services/<name>/
├── cmd/<name>/main.go         # 入口
├── internal/                  # 业务代码（api / config / store / workers …）
├── go.mod / go.sum            # 独立 Go module（被 go.work 引用）
├── Dockerfile
└── README.md                  # 服务级说明
```

完整模块设计：[docs/architecture/modules.md](docs/architecture/modules.md)。
每个服务的细节请进入对应子目录查看 `README.md`。

---

## 设计要点

### 1. 多服务 + Go workspace

仓库使用 [`go.work`](go.work) 把 12 个 module 串成一个 workspace：

- `services/*` 各自是独立的 Go module（独立的 `go.mod`），便于按服务拆
  团队 / 拆 PR。
- `shared/` 提供跨服务复用的基础库：`tenant` 上下文、`httperr` 统一错误
  响应、`validator` 输入校验、`logger`（基于 `slog`）、`db`（pgx 连接池）。

```bash
# 编译某一个服务
(cd services/workflow-svc && go build ./...)

# 编译全部（Makefile 会跳过未实现的占位 service）
make build
```

### 2. 五步法工作流与状态机

状态机定义在 [`docs/architecture/state-machine.md`](docs/architecture/state-machine.md)，
由 `workflow-svc` 实现。每个 `bid_job` 在 PG 中有一行 `state` 字段，
合法迁移：

```text
pending → parsing → outlining → fact_filling → drafting
        → auditing → ready → exporting → done
        ↘ paused / failed / restoring
```

迁移通过 Asynq 任务 + DB 行级 advisory lock 实现幂等，避免重复扣费与重复
生成。Step01–05 的具体行为见
[`docs/architecture/modules.md`](docs/architecture/modules.md)。

### 3. 多模型路由

`router-svc` 对外暴露统一的 `POST /route`，内部按"任务类型 → Provider
策略表"选择模型，并支持：

- **多 Provider**：OpenAI / Anthropic / DeepSeek / Ollama，可通过环境变量
  启用 / 禁用。
- **降级**：当首选 Provider 失败或超时，自动切到备选，并写审计事件。
- **Prompt 缓存**：对相同 `(model, prompt_hash)` 的请求命中 Redis 缓存，
  节省 Token。
- **用量计量**：每次调用都写入 `token_usage`，由 `billing-svc` 聚合对账。

详见 [`docs/architecture/ai-router.md`](docs/architecture/ai-router.md)。

### 4. 知识库与一致性审计

- `knowledge-svc` 用 `pgvector` 存储 chunk embedding，支持
  `vector <-> vector` 相似度检索与全文 `tsvector` 过滤（hybrid）。
  chunk 来源可以是历史标书、企业资料、招标附件。
- `audit-svc` 提供"普通模式"（规则引擎 + 文本精确替换）与"agent 模式"
  （基于 router-svc 的多轮修复）两种工作模式，可在任务级配置切换。

### 5. 鉴权与多租户

所有写请求必须带 `Authorization: Bearer <JWT>`，`api-gateway` 验证后把
`tenant_id` 注入 `context.Context`。**任何对 `projects` / `bid_jobs` /
`kb_*` 等表的查询都必须带 `tenant_id` 过滤**（ADR-0001），由 `shared/pkg/tenant`
提供 helper。

---

## 技术栈

| 层 | 技术 |
|---|---|
| 语言 | Go 1.25+（`go.work`） |
| HTTP | [`chi`](https://github.com/go-chi/chi)（服务内）+ [`httprouter`](https://github.com/julienschmidt/httprouter)（网关） |
| 数据库 | PostgreSQL 16 + pgvector；迁移用 [goose](https://github.com/pressly/goose) |
| DB 驱动 | [`pgx/v5`](https://github.com/jackc/pgx)（不用 `database/sql` ORM） |
| 缓存 / 队列 | Redis 7；任务队列 [Asynq](https://github.com/hibiken/asynq) |
| 对象存储 | S3 兼容：AWS S3 / Cloudflare R2 / MinIO / 阿里云 OSS |
| 配置 | 环境变量（`viper` 可选）+ `.env`（`shared/pkg/config`） |
| 日志 | [`slog`](https://pkg.go.dev/log/slog)（结构化） |
| 鉴权 | JWT（`github.com/golang-jwt/jwt`）+ Refresh Token |
| AI Provider | OpenAI · Anthropic · DeepSeek · Ollama |
| 监控 | OpenTelemetry → Prometheus + Grafana（[docker-compose.yml](docker-compose.yml) 自带） |
| 文档 | MkDocs Material + Mermaid（[docs/](docs/index.md)） |
| Lint / 工具 | golangci-lint、goose、air（热重载）、mkdocs |

---

## 目录结构

```text
backend/
├── services/                # 11 个 Go 微服务 + doc-gen 引擎
│   ├── api-gateway/         # cmd/api-gateway + internal/{auth,proxy,ratelimit}
│   ├── project-svc/         # 项目 / 标段
│   ├── document-svc/        # 文档上传与解析
│   ├── workflow-svc/        # 编排 / 状态机 / workers
│   ├── knowledge-svc/       # pgvector 知识库
│   ├── router-svc/          # AI 模型路由（provider / router / cache）
│   ├── template-svc/        # 行业模板
│   ├── billing-svc/         # 订阅 / Token 计费
│   ├── notify-svc/          # 邮件 / Webhook
│   ├── audit-svc/           # 四层一致性审计
│   └── doc-gen/             # docx 导出引擎（CLI + service）
├── shared/                  # 跨服务共享库
│   └── pkg/                 # tenant / httperr / validator / logger / db
├── migrations/              # goose 风格的 SQL 迁移（00001_init.sql …）
├── docs/                    # MkDocs 文档站源（架构 / 计划 / 决策 / 开发 / 运维 / API）
├── docker/                  # 数据库 / 中间件镜像构建脚本
├── helm/                    # K8s 部署（Prometheus / Grafana provisioning）
├── scripts/                 # CI / bench-guard / 校验脚本
├── bench/                   # Go 基准（baseline / current / report）
├── docs-site/               # mkdocs 渲染产物（构建生成）
├── docker-compose.yml       # 本地 PG / Redis / MinIO / Ollama / Prometheus / Grafana
├── go.work / go.work.sum    # 多 module workspace
├── Makefile                 # 统一入口（make help 查看全部 target）
├── mkdocs.yml               # 文档站配置
├── AGENTS.md                # 给 AI Agent 的硬规则
├── CONTRIBUTING.md
├── CHANGELOG.md
├── LICENSE                  # AGPL-3.0
└── README.md                # 本文件
```

---

## 快速开始

### 0. 前置

- Go **1.25+**
- Docker + Docker Compose（启动 PG / Redis / MinIO / Ollama / Prometheus / Grafana）
- 可选：`golangci-lint`、`goose`、`air`、`mkdocs`

### 1. 启动基础设施

```bash
# 仅起数据库（PG + Redis + MinIO）；也可加 ollama / prometheus / grafana
make db-up
# 或全量：
docker compose up -d postgres redis minio
```

> 🔒 **安全提示**：仓库根的 `docker-compose.yml` 把 PG 绑在 `127.0.0.1`
> （默认账号 `postgres:postgres`）。如果你的开发机可直接被公网访问，
> 请把 Redis（6379）、MinIO（9000/9001）也改成 `127.0.0.1` 绑定，避免被
> 自动化扫描器改密 / 锁库。

### 2. 跑数据库迁移

```bash
export DB_DSN='postgres://postgres:postgres@127.0.0.1:5432/bidwriter?sslmode=disable'
make migrate              # 等价于 goose -dir migrations postgres "$DB_DSN" up
make migrate-status
```

迁移文件位于 [`migrations/`](migrations/)，使用 goose 的
`-- +goose Up` / `-- +goose StatementBegin` 标记。

### 3. 启动某个服务

```bash
# 拉依赖（首次）
make deps

# 直接跑
(cd services/api-gateway && go run ./cmd/api-gateway)

# 或用 air 热重载
make run-svc SVC=workflow-svc
```

各服务的环境变量约定（详见 `services/<name>/internal/config/`）：

| 变量 | 含义 | 默认 |
|---|---|---|
| `HTTP_ADDR` | 服务监听地址 | `:8080`–`:8089`（按服务） |
| `DB_DSN` | PostgreSQL 连接串 | — |
| `REDIS_ADDR` | Redis 地址 | `127.0.0.1:6379` |
| `S3_ENDPOINT` / `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` | 对象存储 | MinIO 默认 |
| `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` / `OLLAMA_HOST` | 模型 Provider | 缺省即禁用 |
| `JWT_SECRET` | 网关 JWT 验签密钥 | dev 默认 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | OpenTelemetry 导出 | 关闭 |

### 4. 跑前端（同级仓库的 `web/`）

后端只暴露 HTTP API。Web 前端位于仓库根的 `web/` 目录：

```bash
cd ../web
pnpm install
pnpm dev          # http://localhost:3000
```

### 5. 一键跑全栈

```bash
make dev          # docker compose up（PG / Redis / MinIO / …）
# 另开终端：make run-svc SVC=api-gateway …
```

---

## 日常开发

| 任务 | 命令 |
|---|---|
| 查看全部 Makefile 目标 | `make help` |
| 拉全部依赖 | `make deps` |
| 编译全部服务 | `make build` |
| 跑全部测试 | `make test` |
| 跑某个服务的测试 | `make test-svc SVC=workflow-svc` |
| 覆盖率报告 | `make coverage`（输出到 `coverage/`） |
| Lint（golangci-lint + markdownlint + CI 校验） | `make lint` |
| 格式化 | `make fmt` |
| 起本地文档站 | `make docs-serve`（`http://localhost:8000`） |
| 构建文档站（strict） | `make docs` |
| 安装 pre-commit hook | `make hook-install` |

开发流程细节：[`docs/development/workflow.md`](docs/development/workflow.md)、
[`docs/development/git-workflow.md`](docs/development/git-workflow.md)、
[`docs/development/coding-standards.md`](docs/development/coding-standards.md)。

---

## 测试与基准

- 单元测试：`testify/assert` + 表格驱动，**目标覆盖率 ≥ 80%**。
  服务级 fixture 与 golden file 放在 `services/<name>/testdata/`。
- 集成测试需要本地 PG / Redis / MinIO，通过 `docker-compose.yml` 启动。
- 基准测试：仓库带 `bench/` 目录与 `bench-guard` 脚本，可在 CI 中对
  `api-gateway`、`workflow-svc` 与 `shared` 做性能回归（默认阈值 20%）。

  ```bash
  make bench                  # 写 bench/current.txt
  make bench-guard            # 对比 bench/baseline/baseline.txt
  make bench-baseline         # 更新基线
  ```

---

## 部署与运维

- **本地**：`docker-compose.yml` + `make dev`。
- **K8s**：[`docs/operations/deployment.md`](docs/operations/deployment.md) 与
  `helm/` 目录提供 Prometheus / Grafana provisioning 与 chart 骨架。
- **监控**：`docker-compose.yml` 自带 Prometheus（9090）+ Grafana（3000），
  服务通过 OpenTelemetry OTLP 上报，参见 [`docs/operations/monitoring.md`](docs/operations/monitoring.md)。
- **故障排查**：[`docs/operations/troubleshooting.md`](docs/operations/troubleshooting.md)、
  [`docs/operations/security.md`](docs/operations/security.md)。
- **CI**：`.github/workflows/`（lint / test / build / docs）。

---

## API 文档

- 总览：[`docs/api/overview.md`](docs/api/overview.md)
- REST：[`docs/api/rest.md`](docs/api/rest.md) + [`docs/api/openapi.yaml`](docs/api/openapi.yaml)
- 鉴权：[`docs/api/authentication.md`](docs/api/authentication.md)
- 错误码：[`docs/api/errors.md`](docs/api/errors.md)

统一入口：`http://<host>:8080`，所有下游服务挂在网关后面。

---

## 强规则

本项目对 AI Agent 与贡献者有强约定，**写在 [`AGENTS.md`](AGENTS.md)**：

1. **先文档后代码** —— 任何代码改动必须先有对应的文档改动（或同步 PR）。
2. **代码规范** —— `gofmt` + `goimports` + `golangci-lint`；错误必须显式
   处理（不允许 `_ = err`）；日志用 `slog`；DB 用 `pgx` / sqlc，不用 ORM。
3. **PR 自检**：跑 `make lint && make test`，单 PR < 500 行，必须附文档
   改动清单与测试覆盖说明。
4. **多租户**：所有查询必须带 `tenant_id`，详见
   [`docs/decisions/`](docs/decisions/) 中 ADR-0001。

---

## 许可

[AGPL-3.0](LICENSE)

---

## 致谢

本项目在架构设计上借鉴了 [OpenBidKit_Yibiao](https://github.com/FB208/OpenBidKit_Yibiao)
（易标投标工具箱）的工程经验，特别是：

- Step01–05 工作流抽象
- 任务状态机设计（paused / restoring / auditing）
- 全文一致性审计（普通 + agent 双模式）
- Prompt 缓存与 JSON 修复链路
- 文本精确替换（多策略 fallback）

感谢 Mark / FB208 在标书 AI 领域的开源贡献。

---

## 联系我们

- Issue / Discussion：本仓库 GitHub
- 邮件：team@bidwriter.app
