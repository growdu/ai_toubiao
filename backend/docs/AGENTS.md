# AGENTS.md

> **给 AI Agent（Claude / GPT / Cursor / Copilot / OpenCode）的硬规则**
>
> 本文件是机器可读的工程契约。所有 AI Agent 在为本项目生成代码、文档、PR 时，必须遵守。

---

## 🚨 第一原则：先文档后代码

> **任何代码改动必须先有对应的文档改动（或同步 PR）。**
>
> 详细规范：[docs/development/workflow.md](development/workflow.md)

违反此规则的 PR 会被 CI 拒绝，必须拆分或补文档。

---

## 📋 项目上下文

- **项目名**：BidWriter
- **目标**：AI 标书自动撰写系统（中小企业 SaaS）
- **形态**：Web + 桌面 + 微服务
- **技术栈**：Go 1.23+ / Next.js 14 / PostgreSQL 16 / Redis 7
- **许可证**：AGPL-3.0
- **架构文档**：[docs/architecture/overview.md](architecture/overview.md)

开始任何任务前，**必须先读**：
1. [README.md](https://github.com/yourorg/bidwriter/blob/main/README.md)
2. [docs/index.md](index.md)
3. [docs/architecture/overview.md](architecture/overview.md)
4. [docs/development/workflow.md](development/workflow.md)
5. [AGENTS.md](AGENTS.md)（本文件）
6. [CONTRIBUTING.md](CONTRIBUTING.md)

---

## 🏗️ 项目结构

```
bidwriter/
├── docs/                    # 📚 所有文档（GitHub Pages 源）
│   ├── architecture/        # 架构设计
│   ├── plan/                # 版本计划
│   ├── decisions/           # ADR 决策记录
│   ├── development/         # 开发指南
│   ├── operations/          # 运维手册
│   └── api/                 # API 文档
├── services/                # 🔧 Go 微服务（10 个）
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
├── web/                     # 🌐 Next.js 14 前端
├── desktop/                 # 🖥️ Tauri 2 桌面（M4 起）
├── shared/                  # 🤝 proto / 类型 / prompts
├── migrations/              # 📦 SQL 迁移（sqlc + golang-migrate）
├── helm/                    # ⎈ Kubernetes Helm Chart
├── .github/                 # ⚙️ CI/CD + 配置
└── mkdocs.yml               # 文档站配置
```

---

## 🛠️ 技术约定

### Go

- **版本**：1.23+
- **包管理**：Go modules
- **错误处理**：必须显式处理，不允许 `_ = err`
- **日志**：用 `slog`（标准库），不用 `logrus`
- **配置**：用 `viper` 或环境变量，不用 flag
- **数据库**：用 `sqlc` 生成代码，不用 ORM
- **HTTP**：用 `chi` 或 `gin`，不用 `net/http` 原生
- **测试**：`testify/assert` + 表格驱动
- **格式化**：`gofmt` + `goimports`
- **Lint**：`golangci-lint run`
- **提交前**：`go vet ./... && golangci-lint run && go test ./...`

### TypeScript / Next.js

- **版本**：Node 20+，Next.js 14+
- **包管理**：pnpm
- **类型**：严格模式（`strict: true`），禁用 `any`
- **状态**：服务端状态用 React Query，客户端状态用 Zustand
- **UI**：Radix UI 原语 + Tailwind CSS
- **表单**：react-hook-form + zod
- **Lint**：`eslint .` + `prettier --check .`
- **类型检查**：`tsc --noEmit`

### 数据库

- **PostgreSQL 16** + pgvector
- **命名**：snake_case（表/列/索引）
- **主键**：UUID v7（不是 v4，可排序）
- **时间**：timestamptz，不允许 timestamp
- **JSON**：jsonb，不用 json
- **迁移**：sqlc + golang-migrate

### 命名规范

| 类型 | 规范 | 示例 |
|---|---|---|
| Go 包 | 小写单词 | `router`, `audit` |
| Go 接口 | 动名词或 -er | `Provider`, `Router` |
| Go 函数 | 驼峰，动词开头 | `ParseRFP`, `GenerateOutline` |
| TypeScript 组件 | 帕斯卡 | `ProjectCard`, `OutlineTree` |
| 数据库表 | snake_case 复数 | `projects`, `outline_nodes` |
| 数据库列 | snake_case | `tenant_id`, `created_at` |
| 环境变量 | SCREAMING_SNAKE | `DATABASE_URL`, `OPENAI_API_KEY` |
| 配置文件 | kebab-case | `docker-compose.yml`, `routes.yaml` |
| 文档 | kebab-case | `getting-started.md`, `data-model.md` |

---

## 📁 文档约定（强规则）

### 文档位置

| 内容类型 | 位置 | 文件命名 |
|---|---|---|
| 架构设计 | `docs/architecture/` | `<topic>.md` |
| 版本计划 | `docs/plan/` | `v<n>-<name>.md` |
| 决策记录 | `docs/decisions/` | `NNNN-<slug>.md` |
| 开发指南 | `docs/development/` | `<topic>.md` |
| 运维手册 | `docs/operations/` | `<topic>.md` |
| API 文档 | `docs/api/` | `<topic>.md` |

### 文档规范

- ✅ 中文为主，术语第一次给英文
- ✅ 至少一个具体例子
- ✅ 链接到相关模块/ADR
- ✅ 注明"最后更新"日期
- ✅ 用 Mermaid 画图（不用图片）
- ❌ 避免"未来再做"模糊表达
- ❌ 避免脱离代码的纯设计稿

详细：[docs/development/documentation-style.md](development/documentation-style.md)

### ADR 模板

任何架构决策必须新增 ADR，文件 `docs/decisions/NNNN-<slug>.md`：

```markdown
# NNNN. 决策标题

## 状态
Proposed / Accepted / Deprecated / Superseded by NNNN

## 日期
YYYY-MM-DD

## 背景
什么问题 / 现状是什么

## 决策
我们决定做什么

## 理由
为什么这么选

## 替代方案
考虑过哪些其他方案，为什么没选

## 后果
- 正面影响
- 负面影响
- 需要承担的成本

## 参考
- 相关文档
- 相关 Issue / PR
```

---

## 🤖 AI Agent 专属规则

### 生成代码前

1. **先确认上下文** —— 读相关模块的设计文档和已有代码
2. **先更新文档** —— 如有架构/接口变更，先 PR 文档改动
3. **再生成代码** —— 文档合并后才动代码

### 生成代码时

1. **遵守上面的技术约定** —— 不要发明新技术栈
2. **不要重复造轮子** —— 优先用现有工具库
3. **写测试** —— 单元测试覆盖 ≥ 80%
4. **写注释** —— 关键逻辑必须有中文注释
5. **避免大块改动** —— 单 PR < 500 行代码
6. **可运行可验证** —— 提交前确保本地能跑

### 生成文档时

1. **位置正确** —— 按上面的"文档位置"放
2. **格式规范** —— 按 documentation-style.md
3. **Mermaid 画图** —— 不要贴图片
4. **链接完整** —— 章节之间互相引用
5. **中文为主** —— 不要机翻风

### 提交 PR 时

PR 描述必须包含：
- 关联 Issue
- 改动文件清单
- 文档改动清单
- 测试覆盖说明
- 截图（UI 改动）
- Checklist（自检）

---

## 🚫 禁止事项

- ❌ 不要修改 `go.mod` / `package.json` 版本不说明
- ❌ 不要删除测试用例
- ❌ 不要提交大文件（> 1MB）到仓库
- ❌ 不要把 `.env` / API key 提交到代码
- ❌ 不要用 emoji 装饰 commit message（除非用户明确要求）
- ❌ 不要生成"看起来对"但实际跑不起来的代码
- ❌ 不要绕过 CI 检查
- ❌ 不要直接 merge 到 main（走 PR）

---

## ✅ 快速任务模板

### 用户："新增 XX 功能"

```
1. 读 docs/architecture/modules.md 看现有模块
2. 读 docs/decisions/ 看相关历史决策
3. 写设计文档 → PR: docs(xxx): <功能> 设计
4. 等设计 review 通过
5. 写代码 + 测试 → PR: feat(xxx): <功能>
6. 更新 API 文档 + README
7. 提交，CI 通过后 merge
```

### 用户："修复 XX bug"

```
1. 定位 bug（用 systematic-debugging skill）
2. 写最小复现测试
3. 修复
4. 验证测试通过
5. 必要时更新 troubleshooting.md
6. PR: fix(xxx): <bug 描述>
```

### 用户："优化 XX 性能"

```
1. 先 benchmark（记录基线）
2. 优化
3. 再 benchmark（记录改进）
4. 更新相关文档
5. PR: perf(xxx): <优化>
```

---

## 📞 求助

- 看 [docs/development/](development/) 找指南
- 看 [docs/decisions/](decisions/) 找历史决策
- 看 [docs/plan/v1-design.md](plan/v1-design.md) 找完整设计
- 问 GitHub Issues
- 紧急联系：team@bidwriter.app

---

## 🔄 本文件的更新

本文件随项目演进更新。改动规则：
- 重大约定变更 → PR 评审
- 小修小补 → 直接 commit
- 写明更新日期 + 更新原因

**最后更新**：2026-06-27（项目初始化）