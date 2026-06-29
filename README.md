# ai_toubiao · AI 标书自动生成系统

> 本仓库承载 **AI 标书自动生成系统** 的产品调研、需求分析与设计文档。
> 不包含代码实现 —— 代码位于 `OpenBidKit_Yibiao`（Electron 客户端）与 `bidwriter`（Go 后端）。

## 文档索引

| 文档 | 内容 |
|---|---|
| [docs/diaoyan.md](docs/diaoyan.md) | 调研：行业现状、痛点、机会、目标用户 |
| [docs/framework.md](docs/framework.md) | 设计纲要：系统目标、核心三要素（AI/章节任务/图表）、状态机、人在回路点 |
| [docs/tech-selection.md](docs/tech-selection.md) | 技术选型：FastAPI + Celery + PostgreSQL + Redis + 多 LLM provider + Mermaid |
| [docs/high-level-design.md](docs/high-level-design.md) | 概要设计（HLD）：组件架构、核心流程、数据模型、接口、可观测、安全、部署 |

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