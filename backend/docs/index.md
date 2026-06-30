# BidWriter 文档

> AI 标书自动撰写系统 — 让中小型投标团队 4 小时内交付一份 80% 完成度的高合规标书。

---

## 📖 文档导航

### 🚀 新人必读

- [产品概览](introduction/overview.md) — 这是什么、解决什么问题
- [目标用户](introduction/users.md) — 谁会用、怎么用
- [与同类产品差异](introduction/comparison.md) — 为什么不一样

### 🏗️ 架构设计

- [架构总览](architecture/overview.md) — 系统全景图
- [设计框架](architecture/framework.md) — 5-Agent 抽象
- [模块设计](architecture/modules.md) — 各服务职责
- [数据模型](architecture/data-model.md) — PostgreSQL schema
- [AI 路由](architecture/ai-router.md) — 多模型路由核心创新
- [状态机](architecture/state-machine.md) — Step01-05 工作流

### 📅 计划

- [v1 设计](plan/v1-design.md) — 完整 1011 行设计文档
- [7 个决策](plan/open-questions-decisions.md) — Open Questions 决策记录
- [迭代路线](plan/roadmap.md) — M1-M4 里程碑

### 📐 决策记录（ADR）

- [ADR 索引](decisions/README.md) — 架构决策记录总览
- 7 个核心决策（0001-0007）

### 💻 开发

- [快速开始](development/getting-started.md) — 5 分钟跑起来
- [开发流程](development/workflow.md) — **⚠️ 先文档后代码（强规则）**
- [文档规范](development/documentation-style.md) — Markdown / 图表
- [代码规范](development/coding-standards.md) — Go / TypeScript
- [测试规范](development/testing.md) — 单元 / 集成 / E2E
- [Git 工作流](development/git-workflow.md) — 分支 / Commit / PR

### 🔧 运维

- [部署指南](operations/deployment.md) — Helm / Docker
- [监控告警](operations/monitoring.md) — Prometheus / Grafana
- [故障排查](operations/troubleshooting.md)
- [安全合规](operations/security.md)

### 📡 API

- [API 概览](api/overview.md)
- [认证](api/authentication.md)
- [错误码](api/errors.md)

---

## 📜 文档约定

### 版本控制

- 文档与代码同仓（`docs/` 目录）
- 文档改动走正常 PR 流程
- GitHub Pages 自动部署

### 强规则

> **任何代码改动必须有对应文档改动（或同步 PR）。**
> 详见 [development/workflow.md](development/workflow.md)

### 文档质量

- ✅ 至少一个具体例子
- ✅ 链接到相关模块/ADR
- ✅ 注明写作时间 + 最后更新
- ❌ 避免"未来再做"模糊表达
- ❌ 避免脱离代码的纯设计稿

---

## 🤝 贡献

发现文档错误、补充内容、改进示例？[提个 PR](https://github.com/yourorg/bidwriter/pulls)！

提交前请阅读：
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [AGENTS.md](AGENTS.md)（给 AI Agent 的硬规则）

---

## 📮 联系

- [GitHub Issues](https://github.com/yourorg/bidwriter/issues)
- [GitHub Discussions](https://github.com/yourorg/bidwriter/discussions)
- 邮件：team@bidwriter.app

---

<p align="center">
  <strong>Made with ❤️ by the BidWriter Team</strong>
  <br/>
  Copyright © 2026 · AGPL-3.0
</p>
