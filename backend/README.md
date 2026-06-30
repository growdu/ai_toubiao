# BidWriter

> **AI 标书自动撰写系统** —— 让中小型投标团队 4 小时内交付一份 80% 完成度的高合规标书。

[![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.23%2B-blue.svg)](https://go.dev/)
[![Node](https://img.shields.io/badge/node-20%2B-green.svg)](https://nodejs.org/)
[![CI](https://img.shields.io/github/actions/workflow/status/yourorg/bidwriter/ci.yml?branch=main&label=CI)](https://github.com/yourorg/bidwriter/actions)
[![Docs](https://img.shields.io/github/actions/workflow/status/yourorg/bidwriter/docs.yml?branch=main&label=Docs)](https://yourorg.github.io/bidwriter/)
[![GitHub Pages](https://img.shields.io/badge/docs-GitHub%20Pages-blue)](https://yourorg.github.io/bidwriter/)

---

## ✨ 这是什么

BidWriter 是一款面向**中小企业投标团队**的 AI 辅助标书撰写系统。它把"标书"从一份 100-1000 页的非结构化文档，拆解为"结构化任务 + 证据驱动 + 多模型路由"的可执行工作流。

**核心能力：**

- 🧩 **五步法工作流** —— 解析 → 拆解 → 大纲 → 全局事实 → 正文生成
- 🤖 **多模型路由** —— OpenAI / DeepSeek / Claude / 本地 Ollama，按成本/质量自动选
- 🔍 **三层知识库** —— 精确匹配 + 弱 RAG + 全局事实，杜绝幻觉
- 🛡️ **四层一致性审计** —— 废标项 / 错别字 / 逻辑 / 查重
- 👥 **团队协作** —— 多角色、实时评论、评审流、操作审计
- 💰 **按需计费** —— 项目订阅 + Token 用量，硬限保护

---

## 🚀 快速开始

### 在线使用（推荐）

访问 [https://bidwriter.app](https://bidwriter.app) （即将上线），注册即用。

### 本地开发

```bash
# 1. 克隆
git clone https://github.com/yourorg/bidwriter.git
cd bidwriter

# 2. 启动基础服务（PostgreSQL + Redis + MinIO）
docker compose up -d

# 3. 启动后端
cd services/api-gateway
go run ./cmd/api-gateway

# 4. 启动 Web 前端
cd web
pnpm install
pnpm dev

# 5. 打开浏览器
open http://localhost:3000
```

详细开发流程：[**docs/development/getting-started.md**](docs/development/getting-started.md)

---

## 📚 文档

> 📖 **完整文档站**：[**https://yourorg.github.io/bidwriter/**](https://yourorg.github.io/bidwriter/)

| 章节 | 用途 |
|---|---|
| [**docs/architecture/**](docs/architecture/) | 系统架构、模块设计、技术选型 |
| [**docs/plan/v1-design.md**](docs/plan/v1-design.md) | v1 版本完整设计（1011 行） |
| [**docs/plan/open-questions-decisions.md**](docs/plan/open-questions-decisions.md) | 7 个关键决策 |
| [**docs/decisions/**](docs/decisions/) | 架构决策记录（ADR） |
| [**docs/development/**](docs/development/) | 开发流程、代码规范、贡献指南 |
| [**docs/operations/**](docs/operations/) | 部署、监控、故障排查 |
| [**docs/api/**](docs/api/) | API 文档（OpenAPI 规范） |

---

## 🏗️ 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│  客户端层                                                        │
│  ┌──────────────────────┐  ┌──────────────────────┐            │
│  │ Web (Next.js)        │  │ Desktop (Tauri)      │            │
│  │ 主入口、协作 UI      │  │ 离线、缓存、同步      │            │
│  └──────────┬───────────┘  └──────────┬───────────┘            │
└─────────────┼──────────────────────────┼────────────────────────┘
              │                          │
              └──────────┬───────────────┘
                         ▼
┌─────────────────────────────────────────────────────────────────┐
│  API Gateway (Go)  ──  认证 / 路由 / 限流 / 计量                  │
└─────────────┬───────────────────────────────────────────────────┘
              ▼
┌─────────────────────────────────────────────────────────────────┐
│  业务服务 (Go 微服务)                                              │
│  project-svc · document-svc · workflow-svc · knowledge-svc       │
│  router-svc  · template-svc  · billing-svc  · audit-svc        │
└─────────────┬───────────────────────────────────────────────────┘
              ▼
┌─────────────────────────────────────────────────────────────────┐
│  AI 路由层 ── 多 Provider 适配、降级、Prompt 缓存、用量计量        │
└─────────────┬───────────────────────────────────────────────────┘
              ▼
┌─────────────────────────────────────────────────────────────────┐
│  数据层                                                          │
│  PostgreSQL 16 (pgvector) · Redis 7 · S3 / R2 / MinIO           │
└─────────────────────────────────────────────────────────────────┘
```

完整架构见 [**docs/architecture/overview.md**](docs/architecture/overview.md)

---

## 🛠️ 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 1.23+ · sqlc · Asynq (Redis) · gRPC + REST |
| 前端 | Next.js 14 · TypeScript · Radix UI · Tailwind |
| 桌面 | Tauri 2 (备选) |
| 数据库 | PostgreSQL 16 + pgvector |
| 缓存/队列 | Redis 7 |
| 对象存储 | S3 兼容（R2 / MinIO / 阿里云 OSS） |
| AI | OpenAI · Anthropic · DeepSeek · Ollama |
| 部署 | Docker · Helm · Kubernetes |
| 监控 | OpenTelemetry · Prometheus · Grafana |
| 文档 | MkDocs Material · Mermaid |
| CI/CD | GitHub Actions · GitHub Pages |

---

## 📦 项目结构

```
bidwriter/
├── docs/                          # 📚 所有文档（GitHub Pages 源）
│   ├── architecture/              # 架构设计
│   ├── plan/                      # 版本计划
│   ├── decisions/                 # ADR 决策记录
│   ├── development/               # 开发指南
│   ├── operations/                # 运维手册
│   ├── api/                       # API 文档
│   └── assets/                    # 图片/图表
├── services/                      # 🔧 Go 微服务
│   ├── api-gateway/
│   ├── project-svc/
│   ├── document-svc/
│   ├── workflow-svc/
│   ├── knowledge-svc/
│   ├── router-svc/
│   ├── template-svc/
│   ├── billing-svc/
│   ├── notify-svc/
│   └── audit-svc/
├── web/                           # 🌐 Next.js 前端
├── desktop/                       # 🖥️ Tauri 桌面备
├── shared/                        # 🤝 共享类型/proto/prompts
├── migrations/                    # 📦 SQL 迁移
├── helm/                          # ⎈ Kubernetes Helm Chart
├── .github/                       # ⚙️ CI/CD + GitHub 配置
│   ├── workflows/                 # GitHub Actions
│   ├── ISSUE_TEMPLATE/
│   └── PULL_REQUEST_TEMPLATE/
├── docker-compose.yml             # 本地开发环境
├── mkdocs.yml                     # 文档站配置
├── AGENTS.md                      # 🤖 给 AI Agent 的硬规则
├── CONTRIBUTING.md                # 👥 贡献指南
├── LICENSE                        # AGPL-3.0
└── README.md                      # 本文件
```

---

## 🤝 贡献

我们欢迎任何形式的贡献 —— 文档、代码、Bug 报告、功能建议。

**⚠️ 强规则：先文档后代码**

> **任何代码改动必须先有对应的文档改动（或同步 PR）。**
> 这是本项目的第一原则。详见 [**docs/development/workflow.md**](docs/development/workflow.md)

- 提交前阅读 [**AGENTS.md**](AGENTS.md) 和 [**CONTRIBUTING.md**](CONTRIBUTING.md)
- 文档约定：[**docs/development/documentation-style.md**](docs/development/documentation-style.md)
- 代码规范：[**docs/development/coding-standards.md**](docs/development/coding-standards.md)
- 决策记录模板：[**docs/decisions/README.md**](docs/decisions/README.md)

---

## 📜 许可证

[AGPL-3.0](LICENSE)

---

## 🙏 致谢

本项目在架构设计上借鉴了 [OpenBidKit_Yibiao](https://github.com/FB208/OpenBidKit_Yibiao)
（易标投标工具箱）的工程经验，特别是：
- Step01-05 工作流抽象
- 任务状态机设计（paused / restoring / auditing）
- 全文一致性审计（普通 + agent 双模式）
- Prompt 缓存与 JSON 修复链路
- 文本精确替换（多策略 fallback）

感谢 Mark / FB208 在标书 AI 领域的开源贡献。

---

## 📮 联系我们

- Issue：[GitHub Issues](https://github.com/yourorg/bidwriter/issues)
- 讨论：[GitHub Discussions](https://github.com/yourorg/bidwriter/discussions)
- 邮件：team@bidwriter.app

---

<p align="center">
  Made with ❤️ by the BidWriter Team
</p>
