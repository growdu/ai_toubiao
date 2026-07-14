# doc-gen 文档生成模块架构

> AI 标书系统的文档生产内核：输入招标文件 + 企业材料目录，输出可交付的 Word/PDF 标书。
> 遵循"CLI 优先、内核复用、服务化演进"三原则。

*最后更新：2026-07-14*

---

## 1. 模块定位

`doc-gen` 是对现有 `workflow-svc` 的 `generating + exporting` 阶段的**整体增强**，补齐三大短板：

| 短板 | 现状 | doc-gen 方案 |
|---|---|---|
| 图表渲染 | 无（仅文本占位符） | Illustrator：mermaid/data_chart/ai_image/table 四类 + 统一美化 |
| 图表美化 | 无 | Beautifier：主题驱动的引擎无关后处理 |
| 学习迭代 | 无 | Learner：模式库 + 检索增强 + Prompt Bandit + 反馈闭环 |

与现有 `document-svc`（仅做 RFP 解析 + 占位符导出）的区别：`doc-gen` 是**完整的生成内核**。

## 2. 设计原则

- **内核与入口分离**：`core` 包是纯逻辑，不含 IO 绑定；`cmd/bidgen`（CLI）与 `cmd/docgen-svc`（服务）只是两个入口壳。
- **图表一等公民**：图表不是附属占位符，而是独立 `FigureSpec → Illustration` 流水线。
- **学习是离线累积**：不做在线模型训练，靠"模式库 + 检索 + Prompt Bandit + 反馈评分"实现渐进提质。
- **本地优先**：CLI 模式所有状态落 SQLite，不依赖 PG/Redis。

## 3. 八步流水线

```
Ingest → Analyze → Plan → Generate → Illustrate → Audit → Assemble → Learn
 摄取     分析     规划    生成       图表渲染      审计     组装       学习
```

| 步骤 | 组件 | 输入 | 输出 | LLM 调用 |
|---|---|---|---|---|
| Ingest | `ingest.Ingester` | 材料目录 | []Chunk | embed 批量 |
| Analyze | `analyzer.Analyzer` | RFP 文本 | RFPProfile | 1 次 rfp_parse |
| Plan | `planner.Planner` | RFPProfile | Outline | 1 次 outline_generate |
| Generate | `generator.Generator` | Outline | []Chapter | C 次 content_generate |
| Illustrate | `illustrator.Illustrator` | []Chapter | []Illustration | F 次（类型选择 + 渲染） |
| Audit | `auditor.Auditor` | BidPackage | []AuditIssue | 0（纯规则） |
| Assemble | `assembler.Assembler` | BidPackage | .docx | 0（纯本地） |
| Learn | `learner.Learner` | BidPackage | Pattern入库 | 0（纯统计） |

## 4. 三大可插拔接口

| 接口 | CLI 实现（Phase 1） | 服务实现（Phase 2） |
|---|---|---|
| `Store` | SQLite + 内存 cosine 检索 | PostgreSQL + pgvector |
| `LLMClient` | 直连 Provider / Anthropic 兼容 | router-svc |
| `Renderer` | shell-out mmdc/python | 常驻 gRPC Python Worker |

切换入口零核心代码改动，仅替换依赖注入的实现。

## 5. 目录结构

```
backend/services/doc-gen/
├── cmd/bidgen/main.go           # CLI 入口（cobra）
├── internal/
│   ├── core/                     # 纯逻辑内核
│   │   ├── types.go              # 领域类型
│   │   ├── config.go             # Theme/Options/RunConfig
│   │   └── pipeline.go           # 8步编排 + 组件接口
│   ├── store/                    # 存储抽象
│   │   ├── store.go              # Store 接口
│   │   └── sqlite.go             # SQLite 实现（11 表）
│   ├── llm/                      # LLM 客户端
│   │   ├── client.go             # Client 接口
│   │   ├── direct.go             # OpenAI 兼容直连
│   │   ├── anthropic.go          # Anthropic 兼容
│   │   └── router_client.go      # router-svc 客户端
│   ├── ingest/                   # 材料摄取
│   ├── analyzer/                 # RFP 分析
│   ├── planner/                  # 大纲规划
│   ├── generator/                # 章节生成
│   ├── illustrator/              # 图表引擎
│   ├── auditor/                  # 内审
│   ├── assembler/                # 文档组装
│   └── learner/                  # 学习迭代
├── themes/default.yaml           # 默认主题
├── configs/bidgen.yaml           # CLI 配置
└── go.mod
```

## 6. CLI 命令

`bidgen` 是 doc-gen 的命令行入口（cobra 实现），与 `docgen-svc` 服务共享同一 Pipeline 内核。

### 6.1 构建与安装

`doc-gen` 不在顶层 `SERVICES` 列表内，需单独编译：

```bash
cd services/doc-gen && go build -trimpath -o ../../bin/bidgen ./cmd/bidgen
```

### 6.2 全局配置

配置文件默认 `~/.bidgen/bidgen.yaml`（可用 `-c` 指定），示例见 `configs/bidgen.yaml`。环境变量优先级高于配置文件：

| 环境变量 | 配置项 | 说明 |
|---|---|---|
| `BIDGEN_DB_PATH` | `db_path` | SQLite 路径，默认 `~/.bidgen/bidgen.db` |
| `BIDGEN_ROUTER_URL` | `router_url` | router-svc 地址，设置后走 router |
| `LLM_API_KEY` | -（仅 env） | OpenAI 兼容 API key |
| `LLM_API_BASE` | `api_base` | API base URL |
| `LLM_MODEL` | `model` | 默认模型 |
| `LLM_EMBED_MODEL` | `embed_model` | 嵌入模型 |
| `ANTHROPIC_AUTH_TOKEN` | - | Anthropic 兼容 token（含 MiniMax） |
| `ANTHROPIC_BASE_URL` | `api_base` | Anthropic base URL |
| `ANTHROPIC_MODEL` | `model` | Anthropic 模型 |

LLM 后端按以下顺序选择：

1. 设了 `router_url` -> [router-svc](ai-router.md) 客户端
2. 有 key 且 base 含 `anthropic`（或设了 `ANTHROPIC_AUTH_TOKEN`） -> Anthropic 兼容客户端
3. 有 key -> OpenAI 兼容直连客户端
4. 都没有 -> noop（生成将失败）

### 6.3 子命令

#### generate

八步流水线全流程，产出 `.docx` 标书。

```bash
bidgen generate <材料目录> [flags]
```

| flag | 说明 | 默认值 |
|---|---|---|
| `--rfp` | 招标文件路径，为空时自动检测 | - |
| `-o, --out` | 输出路径 | 材料目录下 `标书.docx` |
| `-j, --concurrency` | 章节生成并发数 | 10 |
| `-b, --budget` | 总字数预算 | 60000 |
| `--no-illustrate` | 跳过图表渲染 | false |
| `--no-audit` | 跳过审计 | false |
| `--no-learn` | 跳过学习 | false |

#### index

仅摄取建索引（增量），不生成文档。

```bash
bidgen index <材料目录> --rfp 招标文件.pdf
```

| flag | 说明 |
|---|---|
| `--rfp` | 招标文件路径 |

#### learn

离线学习，把历史标书录入模式库（pattern）。

```bash
bidgen learn <历史标书目录> --label won --industry IT
```

| flag | 说明 | 默认值 |
|---|---|---|
| `--label` | 标签：`won`/`lost`/`draft` | won |
| `--industry` | 行业覆盖 | - |

#### report

查看标书包列表、质量评分与模式库，无参数。

```bash
bidgen report
```

### 6.4 典型工作流

```bash
# 1. 配置 LLM（任选其一）
export LLM_API_KEY=sk-xxx
export ANTHROPIC_AUTH_TOKEN=sk-ant-xxx

# 2. 生成标书
bidgen generate ./materials --rfp 招标文件.pdf --out 标书.docx

# 3. 调试：只跑文本，跳过图表/审计/学习
bidgen generate ./materials --no-illustrate --no-audit --no-learn -j 4

# 4. 先建索引再分步生成
bidgen index ./materials --rfp 招标文件.pdf

# 5. 离线学习历史标书
bidgen learn ./历史标书 --label won --industry IT

# 6. 查看报告与质量评分
bidgen report
```

服务化入口见 [docgen-svc](docgen-svc.md)。

## 7. 演进路径

### Phase 1 — CLI 工具（已实现）

- 单二进制 + SQLite，零外部服务依赖
- LLM 通过环境变量配置（支持 OpenAI/Anthropic/MiniMax）
- 渲染依赖 shell-out（mmdc/matplotlib），缺失时降级为占位符

### Phase 2 — 服务化集成（预留）

- `docgen-svc` 暴露 HTTP API，接入 `workflow-svc` 状态机
- Store 从 SQLite 换 PostgreSQL + pgvector
- 队列从同步换 Asynq + Redis
- LLM 走 router-svc，知识检索走 knowledge-svc

接口契约：
```
POST /api/v1/docgen/generate      { bid_job_id, options } → { task_id }
GET  /api/v1/docgen/tasks/:id     → { status, progress, report }
POST /api/v1/docgen/assemble      { bid_job_id, format } → { download_url }
```

## 8. 相关文档

- [doc-gen 架构设计](../../../docs/doc-gen/architecture.md) — 详细架构
- [doc-gen 算法设计](../../../docs/doc-gen/algorithms.md) — 算法伪代码
- [ADR-0011 CLI 优先策略](../decisions/0011-doc-gen-cli-first.md)
- [架构总览](overview.md) — 系统架构
- [模块设计](modules.md) — 各服务设计
