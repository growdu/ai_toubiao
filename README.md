# ai_toubiao · AI 标书自动生成系统

> 本仓库承载 **AI 标书自动生成系统** 的产品调研、需求分析与设计文档。
> 不包含代码实现 —— 代码位于 `OpenBidKit_Yibiao`（Electron 客户端）与 `bidwriter`（Go 后端）。

## 文档索引

### 调研与设计纲要

| 文档 | 内容 |
|---|---|
| [docs/diaoyan.md](docs/diaoyan.md) | 调研：行业现状、痛点、机会、目标用户 |
| [docs/framework.md](docs/framework.md) | 设计纲要：系统目标、核心三要素（AI/章节任务/图表）、状态机、人在回路点 |

### 技术与架构

| 文档 | 内容 |
|---|---|
| [docs/tech-selection.md](docs/tech-selection.md) | 技术选型（13 节）：后端 / 队列 / LLM 路由 / 图表 / Word / KB / 编排 / 存储 / 可观测 / 部署 / 成本 / 风险 / 决策 |
| [docs/high-level-design.md](docs/high-level-design.md) | 概要设计 HLD（15 节）：组件架构、核心流程、**章节划分与调度 ★**、**图表设计与实现 ★**、**Word 输出流水线 ★**、数据模型、接口、算法、可观测、安全、部署 |

### HLD 重点章节速查

| 章节 | 解决的问题 |
|---|---|
| §4 章节划分与调度 | 章节怎么分？优先级？依赖？并发？防饿死？ |
| §5 图表设计与实现 | 图表分几类？怎么定义？怎么渲染？怎么校验？失败怎么办？ |
| §6 Word 输出流水线 | 为什么 Word 为主？模板怎么用？Markdown 怎么变 docx？图表怎么嵌？ |

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
| 链接检查 | lychee | 宽松（仅 Job Summary） |

工作流：`.github/workflows/ci.yml`，对 `push` 到 `main` 与所有 `pull_request` 触发。

## License

Private · 仅供内部使用