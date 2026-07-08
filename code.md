# doc-gen 开发日志

> 记录 doc-gen 模块从设计到实现的全过程：设计决策、偏离、权衡、待确认问题。
> 最后更新：2026-07-08

---

## 一、项目背景与目标

### 起点
- BidWriter 后端已有 10 个 Go 微服务（api-gateway/workflow-svc/router-svc 等）
- 缺少完整的标书生成内核：图表渲染、美化、学习迭代三大短板未补齐
- 需要一个独立的文档生成模块，先 CLI 验证，再服务化集成

### 目标
- **CLI 优先**：`bidgen` 单二进制 + SQLite，零外部依赖，本地验证全链路
- **内核复用**：`core` 包纯逻辑，CLI 和服务共享同一 Pipeline
- **服务化演进**：`docgen-svc` HTTP 服务，接入现有微服务体系

---

## 二、设计决策记录

### D1: 内核与入口分离

**决策**：`internal/core/` 是纯逻辑包，不含 IO 绑定；`cmd/bidgen` 和 `cmd/docgen-svc` 只是两个入口壳。

**理由**：
- 同一套 Pipeline 逻辑既能 CLI 运行也能服务运行
- 测试时不需要启动 HTTP 服务
- Phase 1→2 切换零核心代码改动

**实现**：Pipeline 通过 8 个组件接口（Ingestor/Analyzer/Planner/Generator/Illustrator/Auditor/Assembler/Learner）依赖注入，组件通过 Store/LLMClient/Renderer 三个可插拔接口访问 IO。

---

### D2: Store 接口抽象 — SQLite 优先

**决策**：Store 接口定义 20+ 方法，Phase 1 用 SQLite + 内存 cosine 检索，Phase 2 用 PostgreSQL + pgvector。

**理由**：
- CLI 模式零外部依赖（不需要 PG/Redis）
- SQLite 嵌入式，单文件 `bidgen.db`
- 内存 cosine 对 CLI 数据量（< 1000 chunks）性能足够

**权衡**：
- ✅ 优点：CLI 可离线运行，开发迭代快
- ⚠️ 缺点：需维护两套 Store 实现；SQLite 无原生向量检索
- ⚠️ 缺点：并发写入受限（SQLite 单写）

**实现**：
- `store/sqlite.go`：11 张表 + `cosineSim()` 内存计算
- `store/postgres.go`：相同 schema + pgvector `<->` 运算符
- 编译时接口检查：`var _ Store = (*PostgresStore)(nil)`

---

### D3: LLM 客户端三实现 + 重试包装

**决策**：LLMClient 接口有 3 个实现 + 1 个重试包装器。

| 实现 | 用途 | 场景 |
|---|---|---|
| `DirectClient` | OpenAI 兼容 API | 直连 OpenAI/DeepSeek |
| `AnthropicClient` | Anthropic 兼容 API | MiniMax/Claude |
| `RouterClient` | router-svc HTTP | 服务化模式 |
| `RetryClient` | 重试包装器 | 包装以上任一，3 次指数退避 |

**偏离**：原始设计只提到直连和 router-svc 两种。实际开发中发现环境使用 MiniMax（Anthropic 兼容 API），API 格式与 OpenAI 不同（`/v1/messages` vs `/chat/completions`），因此新增 AnthropicClient。

**权衡**：
- Anthropic API 不提供 embedding 端点 → 向量检索降级为关键词匹配
- 重试机制增加延迟（最坏 2s+4s+8s=14s），但避免单次超时导致整章失败

---

### D4: 图表渲染 — shell-out + 降级

**决策**：Renderer 接口 4 个实现，CLI 模式 shell-out 外部工具。

| 渲染器 | 实现 | 依赖 |
|---|---|---|
| MermaidRenderer | shell-out `mmdc` | Node.js + Chromium |
| DataChartRenderer | shell-out `python3` | matplotlib |
| AIImageRenderer | LLM 生成 | 无额外依赖 |
| TableRenderer | 原生 OOXML | 无额外依赖 |

**偏离**：mmdc 默认主题 `base` 在当前版本不支持，改为 `default`。Chromium 在 sandbox 环境需要 `--no-sandbox`，增加 PuppeteerConfig 自动创建逻辑。

**权衡**：
- ✅ 优点：利用成熟工具生态（mmdc/matplotlib），不重复造轮子
- ⚠️ 缺点：shell-out 有进程启动开销（mmdc ~2.5s/次）
- ✅ 降级策略：依赖缺失时返回占位图，不阻断流水线

---

### D5: 审计去重 — 模糊匹配

**决策**：★号废标条款用模糊匹配去重（`clauseExists` + `isClauseHeader`）。

**问题**：LLM 和正则同时检测★条款，返回略有差异的文本（如 `★1. 投标保证金...` vs `投标保证金...`），导致重复。

**实现**：
- `normalizeClause()`：去除★号/序号/空白，截取前 30 字
- `clauseExists()`：包含关系检查（任一方包含另一方）
- `isClauseHeader()`：过滤标题/说明（含"带★号"/"条款说明"等）
- `dedupStarClauses()`：最终去重，保留首次出现

**效果**：★条款从 11 条降至 6 条（去除 LLM+正则重复 + 标题误报）

---

### D6: 异步任务 — goroutine 先行

**决策**：docgen-svc 用 goroutine + 内存 TaskManager 实现异步任务，而非 Asynq。

**理由**：
- Phase 2 骨架阶段，不需要 Redis 依赖
- goroutine 足够验证 HTTP API 链路
- 后续替换为 Asynq 只改任务调度层

**权衡**：
- ⚠️ 缺点：任务不持久化（重启丢失）
- ⚠️ 缺点：无任务重试/超时控制（靠 context 30min 超时）
- ✅ 优点：零依赖，快速验证

---

### D7: api-gateway 路由集成

**决策**：在 api-gateway 添加 `/api/v1/docgen/*` → docgen-svc (:8090) 的反向代理路由。

**实现**：
- `config.go`：新增 `DocgenSvcURL` 字段 + `DOCGEN_SVC_URL` 环境变量
- `main.go`：`buildRoutes()` 添加 docgenURL 解析 + 路由条目
- `run()`：添加 chi handler 挂载

**验证**：api-gateway 全部测试通过，不破坏现有功能。

---

## 三、偏离记录

### O1: go.mod 版本升级

**情况**：pgx/v5 v5.10.0 要求 go >= 1.25.0，`go mod tidy` 自动将 go.mod 从 1.23 升级到 1.25.0，并下载 go1.25.11 工具链。

**影响**：本地需用 Go 1.25+ 编译。go.work 已是 1.25.0，一致。

**待确认**：生产环境是否已部署 Go 1.25+？若否则需锁定 pgx 版本到 v5.7.x（兼容 go 1.23）。

---

### O2: Anthropic embedding 缺失

**情况**：Anthropic API（含 MiniMax）不提供 embedding 端点。

**影响**：CLI 模式下向量检索不可用，RAG 退化为关键词匹配。

**临时方案**：`AnthropicClient.Embed()` 返回 nil，ingest 跳过向量化，generator 用关键词检索。

**待确认**：是否需要单独配置 embedding provider（如 OpenAI text-embedding-3-small）？

---

### O3: mmdc sandbox 问题

**情况**：Chromium 在 Ubuntu 24.04 的 AppArmor 下无法使用 sandbox。

**解决**：自动创建 `~/.bidgen/puppeteer.json`，内容 `{"args":["--no-sandbox"]}`，传给 mmdc 的 `-p` 参数。

**风险**：`--no-sandbox` 在生产环境有安全隐患，仅适合开发/CI。

---

## 四、权衡记录

### T1: SQLite 单写并发

- SQLite `SetMaxOpenConns(1)` 保证写入安全，但牺牲并发
- CLI 单用户场景够用；PG Store 无此限制

### T2: 内存 cosine vs pgvector

- CLI 模式全表加载做 cosine（O(n)），n<1000 时 <1ms
- PG 模式用 pgvector ANN 索引，支持大规模
- 切换仅需改 Store 实现，内核不变

### T3: 规则降级 vs 硬失败

- 每个组件 LLM 失败时降级为规则模式而非报错退出
- analyze → 正则扫描★条款；plan → 评分项映射章节；generate → 占位文本
- 优点：总能产出 .docx；缺点：质量下降

### T4: 生成并发数

- 默认 concurrency=10，实测 API 限流时 3-5 更稳
- 过高触发 429；过低耗时太长
- 可通过 `--concurrency` / 请求体 `concurrency` 调整

---

## 五、待确认问题

### Q1: Go 版本要求
pgx/v5 v5.10.0 要求 go 1.25.0。是否锁定到 v5.7.x 兼容 go 1.23？还是升级全项目到 1.25？

### Q2: Embedding provider
Anthropic/MiniMax 无 embedding 端点。是否：
- A) 单独配置 OpenAI embedding（需要第二个 API key）
- B) 用本地模型（如 Ollama nomic-embed-text）
- C) 保持关键词匹配降级（当前方案）

### Q3: docgen-svc 任务持久化
当前 goroutine + 内存 map，重启丢失。是否：
- A) Phase 2 直接用 Asynq + Redis
- B) 先用 SQLite 持久化任务状态
- C) 保持现状（够用就行）

### Q4: workflow-svc 集成方式
docgen-svc 如何接入 workflow-svc 状态机？
- A) workflow-svc 主动调用 docgen-svc（HTTP client）
- B) docgen-svc 订阅 workflow 事件（需要消息队列）
- C) 在 workflow-svc 的 worker 中直接嵌入 doc-gen core（不需要 HTTP）

### Q5: 图表渲染安全
mmdc `--no-sandbox` 在生产环境的风险如何处理？
- A) 生产环境用常驻 gRPC Python Worker（Phase 2 方案）
- B) 配置 AppArmor 规则允许 Chromium sandbox
- C) 用 mermaid.ink API 替代本地 mmdc

### Q6: 标书质量评分校准
当前 QualityScorer 权重是人工设定的（评分项覆盖 30% / 字数 15% / 图表 15%...）。是否需要：
- A) 从历史中标/落标数据学习权重
- B) 允许用户自定义权重
- C) 保持固定权重（当前方案）

---

## 六、实现进度

### 已完成

| 模块 | 文件数 | 测试数 | 状态 |
|---|---|---|---|
| core (types/config/pipeline) | 3 | 0 | ✅ |
| store (SQLite + PostgreSQL) | 3 | 5 | ✅ |
| llm (direct/anthropic/router/retry) | 5 | 0 | ✅ |
| ingest (摄取+分块) | 3 | 8 | ✅ |
| analyzer (LLM抽取+规则+去重) | 3 | 7 | ✅ |
| planner (规划+字数分配) | 2 | 7 | ✅ |
| generator (RAG+并发+自审) | 1 | 0 | ✅ |
| illustrator (4渲染器+美化) | 4 | 6 | ✅ |
| auditor (废标+一致性+暗标) | 2 | 7 | ✅ |
| assembler (OOXML+.docx) | 1 | 0 | ✅ |
| learner (模式+Bandit+评分) | 2 | 7 | ✅ |
| cmd/bidgen (CLI) | 1 | 0 | ✅ |
| cmd/docgen-svc (HTTP) | 1 | 0 | ✅ |
| api-gateway 集成 | 2 | (现有) | ✅ |
| **合计** | **33** | **47** | |

### 端到端验证

| 场景 | 结果 | 说明 |
|---|---|---|
| `bidgen index` | ✅ | 4 文件正确分类分块 |
| `bidgen generate` (LLM 正常) | ✅ | 38 章 → .docx 65KB → 质量 75.34 |
| `bidgen generate` (API 限流) | ✅ | 降级完成 → .docx 生成成功 |
| `docgen-svc` HTTP API | ✅ | healthz/generate/tasks 三端点通过 |
| 图表渲染独立测试 | ✅ | mmdc 2.5s / matplotlib 1.5s / table / beautifier |

### 待完成

- [ ] assembler 单元测试
- [ ] generator 单元测试（需 mock LLM）
- [ ] docker-compose 添加 docgen-svc
- [ ] workflow-svc 集成
- [ ] 前端集成

---

## 七、文件清单

```
backend/services/doc-gen/
├── cmd/
│   ├── bidgen/main.go              # CLI 入口 (Phase 1)
│   └── docgen-svc/main.go          # HTTP 服务入口 (Phase 2)
├── internal/
│   ├── core/
│   │   ├── types.go                # 领域类型 (230行)
│   │   ├── config.go               # Theme/Options/RunConfig
│   │   └── pipeline.go             # 8步编排 + 组件接口
│   ├── store/
│   │   ├── store.go                # Store 接口
│   │   ├── sqlite.go               # SQLite 实现 (440行)
│   │   ├── sqlite_test.go          # SQLite 测试 (5个)
│   │   └── postgres.go             # PostgreSQL 实现 (350行)
│   ├── llm/
│   │   ├── client.go               # Client 接口 + Noop
│   │   ├── direct.go               # OpenAI 兼容直连
│   │   ├── anthropic.go            # Anthropic 兼容
│   │   ├── router_client.go        # router-svc 客户端
│   │   └── retry.go                # 重试包装器
│   ├── ingest/
│   │   ├── ingest.go               # 目录遍历+解析+向量化
│   │   ├── chunk.go                # zip XML提取+语义分块
│   │   └── chunk_test.go           # 分块测试 (8个)
│   ├── analyzer/
│   │   ├── analyzer.go             # LLM抽取+规则补全
│   │   ├── dedup.go                # 模糊去重
│   │   └── analyzer_test.go        # 分析器测试 (7个)
│   ├── planner/
│   │   ├── planner.go              # 权重→字数+图表推断
│   │   └── planner_test.go         # 规划器测试 (7个)
│   ├── generator/
│   │   └── generator.go            # RAG接地+并发+自审
│   ├── illustrator/
│   │   ├── renderer.go             # Renderer接口+编排
│   │   ├── renderers.go            # 4渲染器+美化层
│   │   ├── renderers_test.go       # 渲染器测试 (6个)
│   │   └── testutil_test.go        # 测试辅助
│   ├── auditor/
│   │   ├── auditor.go              # 废标+一致性+暗标
│   │   └── auditor_test.go         # 审计器测试 (7个)
│   ├── assembler/
│   │   └── assembler.go            # Markdown→OOXML+.docx
│   └── learner/
│       ├── learner.go              # 模式+Bandit+评分
│       └── learner_test.go         # 学习器测试 (7个)
├── themes/default.yaml             # 默认主题
├── configs/bidgen.yaml             # CLI 配置
├── testdata/投标材料/              # 测试材料
└── go.mod
```
