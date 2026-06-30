# 架构决策记录（ADR）

> **ADR** = Architecture Decision Record，记录重要的架构决策。

## 什么是 ADR

每个 ADR 记录一个**重要的架构决策**：
- 当时面临什么问题
- 做了什么样的决策
- 为什么这么选
- 有哪些后果

## 为什么写 ADR

- ✅ 让团队成员理解"为什么"
- ✅ 避免重复讨论已决定的事
- ✅ 给未来的自己留线索
- ✅ 外部审计 / 新成员 onboarding

## ADR 索引

| 编号 | 标题 | 状态 | 日期 |
|---|---|---|---|
| [0001](0001-multi-tenant.md) | 多租户隔离粒度 | Accepted | 2026-06-27 |
| [0002](0002-ai-router-quality.md) | 模型路由的"质量历史"如何积累 | Accepted | 2026-06-27 |
| [0003](0003-self-hosted-model.md) | 私有化是否提供模型 | Accepted | 2026-06-27 |
| [0004](0004-word-format.md) | Word 导出格式 | Accepted | 2026-06-27 |
| [0005](0005-audit-agent-mode.md) | 审计 agent 模式默认状态 | Accepted | 2026-06-27 |
| [0006](0006-template-marketplace.md) | 行业模板市集冷启动 | Accepted | 2026-06-27 |
| [0007](0007-data-sync.md) | Web 与桌面数据同步冲突 | Accepted | 2026-06-27 |

## ADR 模板

```markdown
# NNNN. 决策标题

## 状态
Proposed | Accepted | Deprecated | Superseded by NNNN

## 日期
YYYY-MM-DD

## 参与者
- @user1
- @user2

## 背景
什么问题 / 现状是什么 / 约束条件

## 决策
我们决定做什么

## 理由
为什么这么选

## 考虑的替代方案
- 方案 A：...，为什么没选
- 方案 B：...，为什么没选

## 后果
### 正面
- 优势 1
- 优势 2

### 负面
- 劣势 1
- 劣势 2

### 中性
- 需要做的事

## 参考
- 相关文档
- 相关 Issue / PR
```

## 命名规范

- 文件名：`NNNN-<kebab-case-slug>.md`
- 编号：4 位数字，递增
- slug：英文短描述

例如：
- `0001-multi-tenant.md`
- `0002-ai-router-quality.md`

## 状态流转

```
Proposed → Accepted → (可选) Superseded by NNNN
       → Rejected
       → Deprecated
```

- **Proposed** — 讨论中
- **Accepted** — 已接受，正在实施
- **Deprecated** — 已废弃，不再使用
- **Superseded by NNNN** — 被新决策替代
- **Rejected** — 讨论后拒绝

## 如何新增 ADR

1. 复制上面的模板
2. 编号用下一个可用数字
3. 写完后提交 PR
4. CI 会检查格式

## 相关文档

- [v1 设计](../plan/v1-design.md) — 完整设计背景
- [决策汇总](../plan/open-questions-decisions.md) — 决策对 Plan 的影响
- [开发流程](../development/workflow.md) — 何时需要写 ADR