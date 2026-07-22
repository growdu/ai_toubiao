# Backend CLI 与运维脚本

> 后端的"接口"有两层：
>
> 1. **HTTP API** — 详见 [`openapi.generated.yaml`](openapi.generated.yaml) 与 [`overview.md`](overview.md)。
> 2. **运维 / 内部 CLI** — 服务启动、迁移、烟雾测试等命令行工具，主要面向后端工程师与 CI。
>
> 本文档列出第二层的全部入口、参数与典型用法。

---

## 1. 速查表

| 命令 | 何时用 | 入口 |
|---|---|---|
| `make db-up` / `docker compose up -d postgres redis minio` | 起本地基础设施 | [`docker-compose.yml`](../docker-compose.yml) |
| `make migrate` | 跑所有 goose 迁移 | [`migrations/`](../migrations/) |
| `make migrate-status` | 看迁移状态 | 同上 |
| `make run-svc SVC=<name>` | 启某个服务（带 air 热重载） | [`services/<name>/cmd/<name>`](../services/) |
| `make build` / `make build-svc SVC=<name>` | 编译全部 / 单个 | `Makefile` |
| `make test` / `make test-svc SVC=<name>` | 跑全部 / 单个测试 | `Makefile` |
| `make lint` | golangci-lint + markdownlint + CI 校验 | `Makefile` |
| `make bench` / `make bench-guard` | 基准 + 回归检测 | [`bench/`](../bench/) |
| `python3 scripts/gen-api.py` | 从 Go 代码重新生成 OpenAPI 规范 | [`scripts/gen-api.py`](../scripts/gen-api.py) |
| `./scripts/build-services.sh [svc]` | 用 Docker 镜像批量/单服务编译 | [`scripts/build-services.sh`](../scripts/build-services.sh) |
| `./scripts/apply-migrations.sh` | CI 友好的纯 SQL 迁移（无需 goose） | [`scripts/apply-migrations.sh`](../scripts/apply-migrations.sh) |
| `./scripts/start-stack.sh` | 一键起基础设施 + 全部服务（开发机） | [`scripts/start-stack.sh`](../scripts/start-stack.sh) |
| `./scripts/start-services.sh` | 仅起全部服务（假设基础设施已在） | [`scripts/start-services.sh`](../scripts/start-services.sh) |
| `./scripts/start-pg.sh` | 只起 PostgreSQL | [`scripts/start-pg.sh`](../scripts/start-pg.sh) |
| `./scripts/stack-entrypoint.sh` | 容器化部署的入口脚本 | [`scripts/stack-entrypoint.sh`](../scripts/stack-entrypoint.sh) |
| `./scripts/smoke-test.sh` | 跨服务冒烟测试（HTTP 链路） | [`scripts/smoke-test.sh`](../scripts/smoke-test.sh) |
| `./scripts/bench-guard/main.go` | 基准回归检测（独立小工具） | [`scripts/bench-guard/`](../scripts/bench-guard/) |
| `python3 scripts/validate-ci.py` | MkDocs / docs / CI 配置校验 | [`scripts/validate-ci.py`](../scripts/validate-ci.py) |
| `bidgen`（doc-gen 二进制） | 离线 docx 导出 CLI | [`services/doc-gen/cmd/bidgen`](../services/doc-gen/cmd/bidgen/) |
| `docgen-svc`（doc-gen 服务） | 启动 docx 导出 HTTP 服务（端口 9090） | [`services/doc-gen/cmd/docgen-svc`](../services/doc-gen/cmd/docgen-svc/) |
| `mkdocs serve` / `mkdocs build --strict` | 文档站本地预览 / 构建 | [`mkdocs.yml`](../mkdocs.yml) |

---

## 2. 各服务 Go 二进制（`cmd/<name>/main.go`）

每个服务都是一个独立的 `cmd/<name>/main.go`，参数完全由标准 `flag` 解析（环境变量优先，flag 覆盖）。

### 2.1 `api-gateway`（端口 8080）

唯一入口；JWT 鉴权 + 反向代理到下游。

| 环境变量 / Flag | 含义 | 默认 |
|---|---|---|
| `HTTP_ADDR` / `-addr` | 监听地址 | `:8080` |
| `PROJECT_SVC_URL` / `-project` | `project-svc` 上游 | `http://localhost:8081` |
| `DOCUMENT_SVC_URL` / `-document` | `document-svc` 上游 | `http://localhost:8082` |
| `WORKFLOW_SVC_URL` / `-workflow` | `workflow-svc` 上游 | `http://localhost:8083` |
| `KNOWLEDGE_SVC_URL` / `-knowledge` | `knowledge-svc` 上游 | `http://localhost:8084` |
| `ROUTER_SVC_URL` / `-router` | `router-svc` 上游 | `http://localhost:8085` |
| `TEMPLATE_SVC_URL` / `-template` | `template-svc` 上游 | `http://localhost:8086` |
| `BILLING_SVC_URL` / `-billing` | `billing-svc` 上游 | `http://localhost:8087` |
| `NOTIFY_SVC_URL` / `-notify` | `notify-svc` 上游 | `http://localhost:8088` |
| `AUDIT_SVC_URL` / `-audit` | `audit-svc` 上游 | `http://localhost:8089` |
| `DOCGEN_SVC_URL` / `-docgen` | `docgen-svc` 上游 | `http://localhost:9090` |
| `DB_DSN` / `-dsn` | Postgres DSN（auth/login 用） | — |
| `JWT_SECRET` / `-jwt-secret` | 签名密钥 | dev 默认（请勿生产用） |
| `ACCESS_TTL` / `-access-ttl` | access JWT TTL | `1h` |
| `REFRESH_TTL` / `-refresh-ttl` | refresh JWT TTL | `720h` |

### 2.2 `project-svc` / `document-svc` / `knowledge-svc` / `template-svc` / `billing-svc` / `notify-svc` / `audit-svc`

形态一致，参数如下：

| 环境变量 | 含义 |
|---|---|
| `HTTP_ADDR` | 监听地址（默认 `:8081` … `:8089`，按服务） |
| `DB_DSN` | Postgres 连接串 |
| `REDIS_ADDR` | Redis（仅 notify / billing 必填） |
| `S3_ENDPOINT` / `S3_BUCKET` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` / `S3_REGION` | 对象存储 |

### 2.3 `workflow-svc`（端口 8083）

| 环境变量 | 含义 |
|---|---|
| `HTTP_ADDR` | `:8083` |
| `DB_DSN` | Postgres |
| `REDIS_ADDR` | Redis |
| `ROUTER_SVC_URL` | 必填，所有 Step 任务通过 router 调 LLM |
| `DOCGEN_SVC_URL` | 导出时调用 |
| `AUDIT_SVC_URL` | 全文审计时调用 |
| `S3_*` | 原始文档 / 导出产物 |

### 2.4 `router-svc`（端口 8085）

| 环境变量 | 含义 |
|---|---|
| `HTTP_ADDR` | `:8085` |
| `REDIS_ADDR` | Redis（Prompt 缓存 + 用量统计） |
| `OPENAI_API_KEY` / `ANTHROPIC_API_KEY` / `DEEPSEEK_API_KEY` / `OLLAMA_HOST` | 至少 1 个，缺省即禁用该 Provider |
| `ROUTER_DEFAULT_BUDGET` | 默认 Token 预算 |

### 2.5 `doc-gen`

`doc-gen` 同时提供两个二进制：

- **`bidgen`** — 纯 CLI，输入招标文件 + 模板 + 数据 → 输出 `.docx`。
  无 HTTP 端口，纯本地运行。
- **`docgen-svc`**（端口 9090）— HTTP 服务，被网关以 `/api/v1/docgen` 反向代理。

| 环境变量 | 含义 |
|---|---|
| `HTTP_ADDR` | `:9090` |
| `DOCGEN_THEME_DIR` | 模板主题目录 |
| `DOCGEN_DB` | SQLite 路径（用于模板缓存） |
| `S3_*` | 导出产物上传 |

---

## 3. 运维脚本（`scripts/`）

### 3.1 `build-services.sh`

主机没有 Go 工具链时，用 `golang:1.25-alpine` 镜像批量编译。

```bash
# 编译全部 12 个二进制（输出到 /tmp/bidwriter-bin）
./scripts/build-services.sh

# 只编译一个
./scripts/build-services.sh api-gateway

# 自定义输出目录
OUT_DIR=/var/lib/bidwriter/bin ./scripts/build-services.sh
```

### 3.2 `apply-migrations.sh`

CI 友好：用 `psql` + awk 切分 `-- +goose Down` 标记，**无需安装 goose**。

```bash
DATABASE_URL='postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable' \
  ./scripts/apply-migrations.sh
```

### 3.3 `start-stack.sh`

开发机一键拉起：基础设施 + 全部服务 + Nginx。详细参数见脚本头部注释。

```bash
./scripts/start-stack.sh                  # 默认行为
GATEWAY_PORT=7080 ./scripts/start-stack.sh
```

### 3.4 `start-services.sh` / `start-pg.sh` / `stack-entrypoint.sh`

- `start-services.sh` — 只起全部服务，假设基础设施已就绪。
- `start-pg.sh` — 只起 PostgreSQL（用于本地调试）。
- `stack-entrypoint.sh` — 容器编排入口，监听信号优雅停机。

### 3.5 `smoke-test.sh`

跨服务冒烟测试。先 `start-services.sh`，再跑这个。

```bash
GATEWAY=http://localhost:7080 ./scripts/smoke-test.sh
```

返回 PASS / FAIL 计数；任何非 200 状态都计为 FAIL。

### 3.6 `validate-ci.py`

MkDocs / 文档 / CI 配置的语法校验。`make docs-validate` 内部调用。

```bash
python3 scripts/validate-ci.py
```

### 3.7 `gen-api.py`

从 Go 代码重新生成 OpenAPI 规范。每次 PR 改 `internal/api/` 都应重跑。

```bash
python3 scripts/gen-api.py
# 产物：docs/api/openapi.generated.yaml + docs/api/services/*.yaml
```

---

## 4. 开发者最常用的 6 个命令

```bash
# 1. 起基础设施（首次或重启）
make db-up

# 2. 跑迁移（首次或 schema 改了之后）
export DB_DSN='postgres://postgres:postgres@127.0.0.1:5432/bidwriter?sslmode=disable'
make migrate

# 3. 启想要改的服务（带热重载）
make run-svc SVC=workflow-svc

# 4. 改完跑测试
make test-svc SVC=workflow-svc

# 5. 改完 API 路由 → 重生 OpenAPI
python3 scripts/gen-api.py

# 6. 改完跑全栈烟雾
./scripts/smoke-test.sh
```
