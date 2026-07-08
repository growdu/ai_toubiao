# 0011. doc-gen 模块 CLI 优先策略

## 状态
Accepted

## 日期
2026-07-07

## 背景

BidWriter 后端已有 10 个 Go 微服务（api-gateway / workflow-svc / router-svc 等），
通过 PostgreSQL + Redis + Asynq 协作完成标书生成流程。

但标书生成的核心逻辑（材料摄取 → RFP 分析 → 大纲规划 → 章节生成 → 图表渲染 →
审计 → 组装 → 学习）尚未有完整实现。现有 `workflow-svc` 的 workers 仅做基础编排，
`document-svc` 仅做文件解析，缺乏图表渲染、美化、学习迭代能力。

需要决定：如何实现这套完整的文档生成内核？

## 决策

采用 **"CLI 优先、内核复用、服务化演进"** 三原则：

1. **Phase 1**：先实现 `bidgen` CLI 工具，单二进制 + SQLite，零外部服务依赖，
   通过环境变量配置 LLM。在本地验证全链路后再进入 Phase 2。

2. **内核与入口分离**：`core` 包是纯逻辑，不含 IO 绑定。CLI 和服务共用同一 Pipeline，
   通过 `Store` / `LLMClient` / `Renderer` 三个可插拔接口切换实现。

3. **Phase 2**：内核稳定后加 `docgen-svc` 服务入口，替换 Store 为 PostgreSQL，
   队列换 Asynq，接入 `workflow-svc` 状态机。

## 理由

- **快速验证**：CLI 模式不需要启动 PG/Redis/MinIO 全套基础设施，单命令即可验证全链路。
- **开发效率**：本地迭代速度快，改代码后 `go build` 即可测试，无需重新部署微服务。
- **可移植**：CLI 单二进制可离线运行，适合私有化场景和开发者本地使用。
- **零核心改动演进**：Phase 1→2 仅替换依赖注入的实现，Pipeline 代码不变。
- **降低风险**：先在 CLI 模式下验证算法正确性（RAG 接地、权重分配、Prompt Bandit 等），
  再接入微服务体系，避免在分布式环境中调试复杂算法。

## 替代方案

### A. 直接实现 docgen-svc 微服务

- 优点：与现有架构一致，一步到位。
- 缺点：需要同时搭建 PG/Redis/MinIO 基础设施，开发迭代慢；算法在分布式环境中调试困难；
  无法离线使用。
- 未选原因：开发效率和验证速度优先。

### B. 在 workflow-svc 中直接扩展

- 优点：不新增服务，复用现有状态机和队列。
- 缺点：workflow-svc 已有 4 个 worker，再加图表/学习逻辑会过于庞大；
  无法独立测试和复用；违反单一职责。
- 未选原因：模块边界不清晰。

### C. 用 Python 实现

- 优点：LLM 生态丰富（LangChain/LlamaIndex），数据处理库多。
- 缺点：与现有 Go 后端技术栈不一致，增加运维复杂度；CLI 分发不如 Go 单二进制方便。
- 未选原因：技术栈一致性优先。

## 后果

- **正面影响**：
  - 开发速度快，本地即可验证全链路（已验证：38 章生成 + .docx 输出 + 质量评分 75.34）。
  - 内核可复用，Phase 2 零核心代码改动。
  - CLI 可作为私有化部署的轻量方案独立交付。
  - 三个可插拔接口（Store/LLMClient/Renderer）使测试和扩展容易。

- **负面影响**：
  - 需要维护两套 Store 实现（SQLite + PostgreSQL）。
  - CLI 模式的向量检索用内存 cosine，大数据量时性能不足（Phase 2 换 pgvector 解决）。
  - 渲染依赖 shell-out（mmdc/python），缺失时降级为占位符。

- **需要承担的成本**：
  - SQLite schema 需与 PostgreSQL schema 保持兼容。
  - CLI 和服务的配置格式需统一（YAML + 环境变量）。

## 参考

- [doc-gen 架构设计](../../docs/doc-gen/architecture.md)
- [doc-gen 算法设计](../../docs/doc-gen/algorithms.md)
- [架构总览](../architecture/overview.md)
- [ADR-0002 AI 路由质量优先](0002-ai-router-quality.md)
- [ADR-0005 审计 Agent 模式](0005-audit-agent-mode.md)
