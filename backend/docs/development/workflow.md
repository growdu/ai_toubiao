# 开发流程

> **⚠️ 第一原则：先文档后代码（Docs First）**

本项目的所有开发活动都必须遵守这个工作流。它是**强规则**，不是建议。

---

## 为什么先写文档

### 三个原因

**1. 文档是设计的最终形态**

> 写得清楚 = 想得清楚。写得不清楚 = 还没想清楚。

把想法写进文档的过程就是梳理逻辑、暴露矛盾、找到边界的过程。跳过文档直接写代码，往往写到一半发现架构有问题。

**2. 文档是协作的契约**

新成员加入、跨团队对接、外部审计，都看文档。文档缺失 = 协作失败。

**3. 文档是项目的资产**

代码会重构、库会替换、API 会变。但**好的设计文档 5 年后还有用**。

---

## 强规则

> **任何代码改动必须有对应文档改动（或同步 PR）。**

### 规则细则

| 改动类型 | 必须包含 |
|---|---|
| 新增功能 | 设计文档 + API 文档 + 使用示例 |
| 重大重构 | 架构文档更新 + ADR（如决策改变）|
| Bug 修复 | 必要时更新 FAQ / 故障排查 |
| 依赖升级 | 必要时更新 README + 安全文档 |
| 配置变更 | 更新运维文档 |

### 例外

- 纯 typo 修复（注释、字符串）
- 测试代码（不需要文档，但要有注释）
- 紧急 hotfix（先合并，**24 小时内补文档**）

---

## 标准开发流程

### 1. 需求阶段

```
用户故事 / 反馈
   ↓
创建 Issue（使用模板）
   ↓
产品评审 → 标记优先级
```

**输出**：
- Issue 描述清楚"要解决什么问题"
- 验收标准（可测试）

### 2. 设计阶段（关键）

```
设计文档（docs/ 下）
  ├─ 新功能 → docs/architecture/modules.md 子章节
  ├─ 架构变更 → 新增 ADR（docs/decisions/NNNN-xxx.md）
  ├─ API 变更 → 更新 docs/api/ 下相关文件
  └─ 流程变更 → 更新 docs/development/workflow.md
   ↓
PR: docs: <功能名> 设计文档
   ↓
设计评审（至少 1 人 review）
   ↓
合并后才能开始写代码
```

**设计文档模板**：

```markdown
# <功能名>

## 目标
解决什么问题 / 带来什么价值

## 背景
为什么需要 / 现状是什么

## 设计
### 整体方案
### 数据模型
### API 设计
### 关键流程
### 风险与边界

## 验收标准
- [ ] 可测试的条件 1
- [ ] 可测试的条件 2

## 替代方案（可选）
考虑了哪些其他方案，为什么选这个

## 相关文档
- 链接 1
- 链接 2
```

### 3. 实现阶段

```
创建功能分支（git workflow 见 git-workflow.md）
   ↓
按设计实现
   ↓
本地测试
   ↓
PR: feat: <功能名>
   ↓
CI 通过 + 至少 1 人 review
   ↓
合并到 main
```

**PR 模板要求**：
- 关联 Issue
- 改动文件清单
- 测试覆盖说明
- 截图 / 录屏（UI 改动）
- 文档改动清单

### 4. 发布阶段

```
合并到 main
   ↓
自动部署到 staging
   ↓
手动验证
   ↓
合并到 release 分支
   ↓
自动部署到 production
   ↓
发布说明（CHANGELOG.md）
```

---

## PR 类型

我们用 Conventional Commits 风格：

| 类型 | 用途 | 是否需要文档 |
|---|---|---|
| `docs:` | 仅文档改动 | — |
| `feat:` | 新功能 | ✅ 强制 |
| `fix:` | Bug 修复 | 看情况 |
| `refactor:` | 重构 | ✅ 强制（如架构变）|
| `perf:` | 性能优化 | 建议 |
| `test:` | 测试改动 | — |
| `chore:` | 构建/CI/工具 | 必要时 |

**示例**：

```bash
git commit -m "feat(router-svc): add DeepSeek provider with fallback

- Implement DeepSeekChat adapter
- Add to routes.yaml for outline_generate task
- Fallback to GPT-4o-mini on failure

Closes #123
Docs: docs/architecture/ai-router.md updated"
```

---

## 文档写作规范

### 风格

- ✅ **简洁明确** — 一句话说清楚的别用三句
- ✅ **有例子** — 每个抽象概念配具体例子
- ✅ **可链接** — 章节之间互相引用
- ✅ **中文为主** — 术语第一次出现给英文
- ❌ **避免"未来再做"** — 写当前的状态
- ❌ **避免"显而易见"** — 写出来不显而易见的

### 格式

- 使用 Markdown
- 标题用 ATX 风格（`#`）
- 代码块标语言
- 表格用 GFM 表格
- 用 Mermaid 画图（不要贴图片）

详细：[documentation-style.md](documentation-style.md)

### 位置

| 内容 | 位置 |
|---|---|
| 架构设计 | `docs/architecture/` |
| 版本计划 | `docs/plan/` |
| 决策记录 | `docs/decisions/NNNN-xxx.md` |
| 开发指南 | `docs/development/` |
| 运维手册 | `docs/operations/` |
| API 文档 | `docs/api/` |

---

## CI 检查

GitHub Actions 自动检查：

1. **docs-only check** — 纯文档 PR 不需要跑测试
2. **docs-required check** — 代码 PR 必须包含 docs/ 改动（或 PR 描述说明）
3. **broken link check** — 文档内链接不能 404
4. **markdown lint** — 符合 markdownlint 规则
5. **mkdocs build** — 文档站能成功构建

详细配置：[.github/workflows/ci.yml](../.github/workflows/ci.yml)

---

## 例外流程：紧急 Hotfix

```
紧急问题（生产事故）
   ↓
直接修复（不写文档）
   ↓
合并到 main
   ↓
立即部署
   ↓
24 小时内补：
  - 修复说明（docs/operations/troubleshooting.md）
  - ADR（如有架构决策）
  - PR 描述（事后总结）
```

---

## 检查清单（PR 自检）

提交 PR 前确认：

- [ ] 设计文档已合并（在 main 上可见）
- [ ] 代码实现符合设计文档
- [ ] 单元测试覆盖
- [ ] 集成测试通过
- [ ] API 文档已更新
- [ ] README/CHANGELOG 已更新
- [ ] 没有遗留 TODO（除非是合理的）
- [ ] CI 全绿

---

## 相关文档

- [快速开始](getting-started.md)
- [文档规范](documentation-style.md)
- [代码规范](coding-standards.md)
- [Git 工作流](git-workflow.md)
- [测试规范](testing.md)
- [AGENTS.md](../AGENTS.md) — 给 AI Agent 的硬规则
- [CONTRIBUTING.md](../CONTRIBUTING.md) — 贡献指南
