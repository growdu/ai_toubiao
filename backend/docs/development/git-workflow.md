# Git 工作流

## 分支策略

我们用 **Trunk-based Development** + **短期 Feature Branch**：

```
main (稳定，随时可发布)
 │
 ├── feat/xxx（短期功能分支，< 3 天）
 │     ↓
 ├─── merge ──→ main ──→ tag: v0.x.0
 │
 ├── fix/xxx
 │     ↓
 └─── merge ──→ main
```

### 分支类型

| 类型 | 命名 | 生命周期 | 来源 / 目标 |
|---|---|---|---|
| `main` | `main` | 永久 | main ← feature |
| 功能 | `feat/<name>` | < 3 天 | main → main |
| 修复 | `fix/<name>` | < 1 天 | main → main |
| 重构 | `refactor/<name>` | < 3 天 | main → main |
| 文档 | `docs/<name>` | < 1 天 | main → main |
| 紧急 | `hotfix/<name>` | < 数小时 | main → main |
| 发布 | `release/vX.Y.Z` | < 1 天 | main → tag |

### 分支命名

✅ **好**：
- `feat/router-svc-deepseek-provider`
- `fix/audit-tenant-leak`
- `docs/api-authentication`
- `hotfix/p0-data-loss`

❌ **差**：
- `my-branch`
- `test`
- `fix-bug`
- `WIP`

---

## Commit 规范

用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type

| Type | 用途 |
|---|---|
| `feat` | 新功能 |
| `fix` | Bug 修复 |
| `docs` | 仅文档 |
| `refactor` | 重构（无功能变化）|
| `perf` | 性能优化 |
| `test` | 测试 |
| `chore` | 构建 / CI / 工具 |

### Scope

服务名 / 模块名：

- `api-gateway`, `project-svc`, `document-svc`, `workflow-svc`, `knowledge-svc`, `router-svc`, `template-svc`, `billing-svc`, `notify-svc`, `audit-svc`
- `web`
- `desktop`
- `docs`
- `ci`

### Subject

- 中文 / 英文都可，建议中文
- 简短（< 72 字符）
- 不带句末标点
- 动词开头：`新增` / `修复` / `重构` / `优化`

### Body

- 解释"为什么"，不解释"是什么"
- 列出关键改动点

### Footer

- 关联 Issue：`Closes #123`
- 关联 PR：`Refs #456`
- 关联文档：`Docs: docs/architecture/ai-router.md updated`

### 示例

```bash
git commit -m "feat(router-svc): 新增 DeepSeek provider

- 实现 DeepSeekChat 适配器
- 添加到 routes.yaml 用于 outline_generate
- 失败时降级到 GPT-4o-mini

Closes #123
Docs: docs/architecture/ai-router.md updated"
```

---

## 提交流程

### 标准流程

```bash
# 1. 拉最新 main
git checkout main
git pull origin main

# 2. 创建分支
git checkout -b feat/router-deepseek

# 3. 写代码 + 测试 + 文档
# ... 编辑文件

# 4. 提交（多次）
git add services/router-svc/
git commit -m "feat(router-svc): 新增 DeepSeek 适配器"

git add docs/architecture/ai-router.md
git commit -m "docs(ai-router): 更新 DeepSeek 路由配置"

# 5. 推送
git push origin feat/router-deepseek

# 6. 创建 PR（在 GitHub 上）

# 7. 等 CI 通过 + Review

# 8. Squash merge 到 main（按钮操作）
```

### Rebase vs Merge

- ✅ **Rebase** 保持线性历史（推荐）
- ❌ **Merge commit** 污染历史

```bash
# 推送前 rebase
git fetch origin
git rebase origin/main

# 解决冲突（如有）
# 然后推送
git push origin feat/router-deepseek --force-with-lease
```

### 合并策略

| 场景 | 策略 |
|---|---|
| 普通功能 PR | Squash merge（合并成 1 个 commit） |
| 长期分支 | Merge commit（保留历史） |
| 紧急修复 | Squash merge |
| 文档 PR | Squash merge |

---

## Pull Request 流程

### 1. 创建 PR

- 标题用 Conventional Commits 格式
- 描述填 `.github/PULL_REQUEST_TEMPLATE.md`
- 关联 Issue
- 至少 1 个 label

### 2. 自检

- [ ] CI 全绿
- [ ] 文档已更新
- [ ] 测试已加
- [ ] 描述清晰
- [ ] 关联 Issue

### 3. Review

- 至少 1 个 maintainer 批准
- CODEOWNERS 自动 @ 相关 reviewer
- 解决所有对话
- 解决所有 review 反馈

### 4. 合并

- 必须 rebase 到最新 main
- 必须 squash merge
- 删除远程分支

---

## 紧急修复（Hotfix）

```bash
# 1. 从 main 创建 hotfix 分支
git checkout main
git pull origin main
git checkout -b hotfix/p0-data-loss

# 2. 修复 + 测试（最简化）
git commit -m "fix(p0): 修复数据丢失 bug

严重：项目数据在并发时丢失

修复：在事务中加行锁

Refs #999"

# 3. PR + 紧急合并（可联系 maintainer 加速 review）

# 4. 立即部署到生产

# 5. 24 小时内补：
#    - 复盘文档（docs/operations/troubleshooting.md）
#    - 测试用例（防止回归）
#    - ADR（如有架构决策）
```

---

## Release 流程

### 版本号

用 [Semantic Versioning](https://semver.org/)：

```
MAJOR.MINOR.PATCH
  │      │      │
  │      │      └─ Bug 修复
  │      └──────── 新功能（向后兼容）
  └────────────── 破坏性变更
```

### Tag & Release

```bash
# 1. 从 main 创建 release 分支
git checkout main
git checkout -b release/v0.1.0

# 2. 更新版本号
# - 各服务的 VERSION 常量
# - web/package.json
# - helm/Chart.yaml

# 3. 更新 CHANGELOG.md

# 4. PR 到 main → 合并

# 5. 打 tag
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0

# 6. GitHub Actions 自动构建 + 发布
```

### CHANGELOG.md 格式

```markdown
# Changelog

## [Unreleased]

## [0.1.0] - 2026-07-15

### Added
- 工作流 Step01-05 编排
- 多模型路由（OpenAI / Anthropic / DeepSeek）
- 6 个行业模板
- 实时协作（Web）
- 一致性审计（normal + agent）

### Changed
- 重构知识库从单一索引到三层架构

### Fixed
- 修复审计在并发场景下的漏检

### Security
- 升级依赖至最新
```

---

## 撤销 / 回滚

### 撤销最后一次 commit（未推送）

```bash
git reset --soft HEAD^       # 保留改动
git reset --hard HEAD^       # 丢弃改动
```

### 撤销已推送的 commit

```bash
git revert <commit-hash>     # 生成新 commit 撤销
git push
```

### 回滚已合并的 PR

```bash
# 1. 找到合并 commit
git log --oneline --merges | head -5

# 2. revert
git revert -m 1 <merge-commit-hash>
git push
```

---

## 工具配置

### .gitignore（关键）

```gitignore
# 依赖
node_modules/
vendor/

# 构建
dist/
build/
bin/
*.exe

# 环境
.env
.env.local
*.local

# IDE
.vscode/
.idea/
*.swp

# 测试覆盖率
coverage/
*.out

# 临时
tmp/
*.log
.DS_Store

# OS
Thumbs.db
```

### git config

```bash
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
git config --global pull.rebase true
git config --global core.autocrlf input  # Linux/Mac
# git config --global core.autocrlf true   # Windows
git config --global init.defaultBranch main
```

### Git LFS（大文件）

```bash
# 大文件用 LFS
git lfs install
git lfs track "*.pdf"
git lfs track "*.docx"
git lfs track "*.psd"
```

---

## 常见错误

### ❌ 提交大文件

```bash
# 错：提交 node_modules
git add node_modules/

# 对：用 .gitignore
echo "node_modules/" >> .gitignore
```

### ❌ 提交密钥

```bash
# 错
git add .env
git commit -m "add config"

# 对：用 secret 管理
# - 开发：.env.example（占位）
# - 部署：Kubernetes Secret / Vault
# - CI：GitHub Secrets
```

如果已经提交：

```bash
# 删除历史（⚠️ 危险）
git filter-branch --force --index-filter \
  "git rm --cached --ignore-unmatch .env" \
  --prune-empty --tag-name-filter cat -- --all
git push --force
# 然后立刻 rotate 所有密钥
```

### ❌ 直接 push 到 main

```bash
# 错
git push origin main

# 对：先 PR
git checkout -b feat/xxx
git push origin feat/xxx
# 然后在 GitHub 上创建 PR
```

### ❌ 合并 main 到 feature 分支（污染历史）

```bash
# 错
git checkout feat/xxx
git merge main

# 对：rebase
git rebase main
```

---

## 相关文档

- [开发流程](workflow.md) — **必读** "先文档后代码"
- [代码规范](coding-standards.md)
- [测试规范](testing.md)
- [CONTRIBUTING.md](../CONTRIBUTING.md)