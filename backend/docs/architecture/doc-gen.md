# doc-gen 文档生成模块架构

> AI 标书系统的文档生产内核：输入招标文件 + 企业材料目录，输出可交付的 Word/PDF 标书。
> 遵循"CLI 优先、内核复用、服务化演进"三原则。

*最后更新：2026-07-07*

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

```bash
bidgen generate <dir> --rfp xxx.pdf --out 标书.docx  # 生成标书
bidgen index <dir>                                     # 仅建索引
bidgen learn <dir> --label won --industry IT           # 离线学习
bidgen report                                          # 查看报告
```

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
