# Contributing to BidWriter

> **欢迎贡献！** 任何形式的参与都让 BidWriter 更好。

---

## 🚨 第一原则：先文档后代码

> **任何代码改动必须先有对应的文档改动（或同步 PR）。**

详见 [docs/development/workflow.md](docs/development/workflow.md)。这是本项目的**硬规则**，不是建议。

---

## 🤝 贡献方式

### 1. 报告 Bug

[提交 Issue](https://github.com/yourorg/bidwriter/issues/new?template=bug_report.md)

请包含：
- 复现步骤
- 期望行为
- 实际行为
- 截图 / 日志
- 环境信息（OS / 浏览器 / 版本）

### 2. 提出功能建议

[提交 Feature Request](https://github.com/yourorg/bidwriter/issues/new?template=feature_request.md)

请说明：
- 解决什么问题
- 谁会受益
- 备选方案
- 实现复杂度

### 3. 改进文档

- 修 typo、补充例子、改进措辞 → 直接 PR
- 添加新章节 → 先用 [discussion](https://github.com/yourorg/bidwriter/discussions) 讨论

### 4. 贡献代码

[看 Development Workflow](docs/development/workflow.md)

### 5. 审查 PR

任何人都可以 review PR，特别是相关模块的 maintainer。

### 6. 分享经验

写 blog、录视频、做内部分享 —— 让更多人用上 BidWriter。

---

## 📋 完整开发流程

### 步骤 1：准备

```bash
# Fork 仓库
# Clone 你的 fork
git clone https://github.com/<your-username>/bidwriter.git
cd bidwriter

# 添加 upstream
git remote add upstream https://github.com/yourorg/bidwriter.git

# 安装依赖
docker compose up -d
cd web && pnpm install && pnpm dev
```

### 步骤 2：创建分支

```bash
git checkout -b feat/<feature-name>
# 或
git checkout -b fix/<bug-name>
# 或
git checkout -b docs/<doc-topic>
```

### 步骤 3：写文档（如涉及代码改动）

```bash
# 编辑 docs/ 下文件
# 提交
git add docs/
git commit -m "docs(<scope>): <description>

Closes #<issue>"
git push origin docs/<topic>
```

**等设计文档合并后**，再写代码。

### 步骤 4：写代码

```bash
# 拉取最新 main
git fetch upstream
git rebase upstream/main

# 编辑代码
# 写测试
# 跑测试
go test ./...
cd web && pnpm test

# 提交
git add .
git commit -m "feat(<scope>): <description>

- 改动点 1
- 改动点 2

Closes #<issue>
Docs: docs/<file>.md updated"
```

### 步骤 5：提 PR

```bash
git push origin feat/<feature-name>
```

然后去 GitHub 创建 PR，**填好 PR 模板**。

---

## ✅ PR 审查清单

提交 PR 前自检：

- [ ] 代码符合 [coding-standards.md](docs/development/coding-standards.md)
- [ ] 单元测试覆盖 ≥ 80%
- [ ] `go test ./...` 通过
- [ ] `pnpm test` 通过
- [ ] `golangci-lint run` 无错误
- [ ] `pnpm lint` 无错误
- [ ] `tsc --noEmit` 无错误
- [ ] 文档已更新（或本次纯文档 PR）
- [ ] PR 描述填好模板
- [ ] 关联了 Issue

---

## 🧪 测试要求

### 必须有测试

- 新功能 → 单元测试 + 集成测试
- Bug 修复 → 复现测试 + 回归测试
- 重构 → 确保现有测试通过

### 测试覆盖率

- 业务代码：≥ 80%
- 工具代码：≥ 60%
- UI 代码：≥ 50%

详见 [testing.md](docs/development/testing.md)

---

## 📚 文档要求

### 何时必须写文档

- 新增 API → 更新 `docs/api/`
- 新增服务 → 更新 `docs/architecture/modules.md`
- 架构变更 → 新增 ADR `docs/decisions/NNNN-*.md`
- 配置变更 → 更新 `docs/operations/deployment.md`
- 新增依赖 → 更新 `README.md`

### 文档风格

- 中文为主
- 有具体例子
- 用 Mermaid 画图
- 互相链接

详见 [documentation-style.md](docs/development/documentation-style.md)

---

## 🏷️ Commit 规范

用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type**：

| 类型 | 用途 |
|---|---|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 仅文档 |
| `refactor` | 重构（无功能变化）|
| `perf` | 性能优化 |
| `test` | 测试 |
| `chore` | 构建/CI/工具 |

**Scope**：

- `api-gateway`, `project-svc`, ..., `web`, `docs`

**示例**：

```bash
git commit -m "feat(router-svc): add DeepSeek provider with fallback

- Implement DeepSeekChat adapter
- Add to routes.yaml for outline_generate
- Fallback to GPT-4o-mini on failure

Closes #123
Docs: docs/architecture/ai-router.md updated"
```

---

## 🔒 安全问题

**请勿在公开 Issue 报告安全问题！**

发送邮件到 security@bidwriter.app，包含：
- 问题描述
- 复现步骤
- 影响范围
- 建议修复（可选）

我们会在 48 小时内响应。

---

## 📜 许可证

贡献即同意按 [AGPL-3.0](LICENSE) 协议授权。

---

## 🙏 行为准则

- 友善、专业、包容
- 接受建设性批评
- 关注社区利益
- 不容忍任何形式的骚扰

违反 → 警告 → 封禁。

---

## 💬 联系我们

- [GitHub Discussions](https://github.com/yourorg/bidwriter/discussions) — 一般讨论
- [GitHub Issues](https://github.com/yourorg/bidwriter/issues) — Bug / Feature
- 邮件：team@bidwriter.app — 其他

---

<p align="center">
  感谢你的贡献！❤️
</p>