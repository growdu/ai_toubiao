# 0005. 审计 agent 模式默认状态

## 状态

Accepted

## 日期

2026-06-27

## 参与者

- 架构组
- 产品组
- AI 工程组

## 背景

一致性审计有两种模式：
- **normal**：单次 LLM 调用，结构化输出（快速、便宜）
- **agent**：多轮 LLM 调用 + 工具调用 + 自主决策（强、慢、贵 5-10 倍）

借鉴 yibiao 的设计，agent 模式在 OpenCode 子进程中跑。

**约束条件**：
- agent 模式成本高（5-10 倍 normal）
- 用户对"为什么这次贵 10 倍"需要明确知情
- 普通项目 normal 已足够（90% issue 能抓到）
- 复杂项目 / 关键标段 / 客户主动要求 → 开 agent

**需要决策**：audit agent 模式的默认状态。

## 决策

**默认关闭，UI 显式按钮"深度审计"开启；M3 评估后考虑默认开启 normal、自动启用 agent。**

**自动启用规则（M3 评估后启用）**：
- 项目金额 > 100 万
- 关键标段（用户标记）
- 失败率 > X（normal 模式漏检）

## 理由

- ✅ agent 模式成本 5-10 倍，需明确知情
- ✅ 用户主动选择 = 用户负责
- ✅ normal 已能抓 90% issue
- ✅ 复杂场景留升级路径

## 考虑的替代方案

### 方案 A：默认开启 agent

- ❌ 用户意外耗尽配额
- ❌ 成本不可控
- ❌ 性能慢（影响体验）

### 方案 B：默认关闭，需手动开（**选择**）

- ✅ 用户知情同意
- ✅ 成本可控
- ✅ 性能可控
- ⚠️ 用户可能不会主动开（需要产品引导）

### 方案 C：默认 normal，自动 agent（高级场景）

- ✅ 大部分场景体验好
- ✅ 关键场景自动升级
- ⚠️ 实施复杂度中等（M3）

## 后果

### 正面

- 用户预算可控
- 性能可控
- 高级场景可升级

### 负面

- 用户可能不知道有 agent 模式
- M3 前"关键标段"没有自动保护
- 需要产品引导 / 教育

### 中性（需要承担的工作）

- UI 默认勾选 normal
- 高级选项：复选框"启用 agent 深度审计" + 预估成本
- 路由层：agent 强制走最强模型
- 配额保护：agent 单独计费
- M3 评估自动启用规则

## 实施细节

### UI 设计

```
┌─────────────────────────────────────────────┐
│  一致性审计                                  │
├─────────────────────────────────────────────┤
│  模式：                                       │
│   ○ 普通模式（normal）                       │
│   ● 深度审计（agent）                        │
│                                              │
│  ⚠️ 深度审计预估消耗：~50k tokens（¥2.00）   │
│                                              │
│  ☑ 检查废标项                                │
│  ☑ 检查错别字                                │
│  ☑ 检查逻辑冲突                              │
│  ☑ 与历史标书查重                            │
│                                              │
│            [开始审计]                         │
└─────────────────────────────────────────────┘
```

### 路由层配置

```yaml
# router-svc/configs/routes.yaml
routes:
  - task: consistency_audit_normal
    primary: { provider: anthropic, model: claude-sonnet-4 }
    fallback: [{ provider: openai, model: gpt-4o }]

  - task: consistency_audit_agent
    primary: { provider: anthropic, model: claude-sonnet-4 }  # agent 强制 Sonnet 起步
    fallback: []  # agent 不降级（直接失败让用户重试）
    budget:
      max_tokens: 50000
      max_cost_usd: 2.00
      timeout_seconds: 600
```

### 配额保护

```go
type AuditRequest struct {
    Mode    string  // normal | agent
    TenantID string
}

func (s *AuditService) Run(ctx context.Context, req AuditRequest) (*AuditResult, error) {
    if req.Mode == "agent" {
        // 单独计费
        budget, err := s.billing.GetAgentBudget(ctx, req.TenantID)
        if err != nil {
            return nil, ErrNoBudget
        }
        if budget.Used >= budget.Limit {
            return nil, ErrAgentBudgetExhausted
        }
    }
    // ... 执行审计
}
```

### M3 自动启用规则

```go
func ShouldEnableAgent(project *Project, history *AuditHistory) bool {
    // 规则 1：金额 > 100 万
    if project.EstimatedValue > 100_0000 {
        return true
    }
    // 规则 2：关键标段
    if project.IsCritical {
        return true
    }
    // 规则 3：normal 漏检率高
    if history.RecentMissRate > 0.15 {  // 15% 漏检
        return true
    }
    return false
}
```

### 借鉴 yibiao 的设计

yibiao 的 OpenCode 子进程设计：
- 独立子进程隔离（不影响主服务）
- 端口代理通信
- 10 分钟超时
- 心跳保活

我们的实现：
- audit-svc 启动 agent worker 池
- 任务调度用 Asynq
- 端口代理 + 健康检查
- 单次任务最大 600 秒超时

## 退出条件

需要重新评估的触发条件：

- 🔴 用户反复主动开启 agent（> 50%）
- 🔴 normal 模式 issue 漏检率被证明过高
- 🔴 agent 成本下降（用更强模型更便宜）
- 🔴 M3 自动规则效果不佳

## 参考

- [架构 / 模块设计 - audit-svc](../architecture/modules.md#audit-svc)
- [架构 / AI 路由](../architecture/ai-router.md)
- [Plan / v1 设计 第 7 节](../plan/v1-design.md)
- [ADR-0002 模型路由](0002-ai-router-quality.md)
- yibiao OpenCode 子进程设计（参考）