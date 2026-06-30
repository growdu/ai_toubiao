# 0002. 模型路由的"质量历史"如何积累

## 状态

Accepted

## 日期

2026-06-27

## 参与者

- 架构组
- AI 工程组

## 背景

AI 路由要回答"什么任务用什么模型"。人工配置（专家经验）不准确、不可持续。需要建立**数据驱动的质量反馈机制**。

**约束条件**：
- 冷启动无数据
- 显式反馈（点赞/点踩）成本高、样本稀疏
- 隐式反馈（采纳率、修改率）丰富但需要埋点
- LLM 评估成本高，不能 100% 用

**需要决策**：分阶段积累"质量历史"。

## 决策

**三阶段演进**：
1. **M1-M2 规则路由**（任务画像 → 路由表）
2. **M3 隐式反馈**（用户行为信号）
3. **M4+ LLM-as-a-Judge**（LLM 评估）

## 理由

- ✅ 冷启动无数据，规则路由先用起来，让真实流量积累
- ✅ 隐式反馈成本低、信号丰富、自然累积
- ✅ LLM 评估最后接入，避免成本浪费
- ✅ 每阶段都是渐进增强，不破坏前阶段

## 考虑的替代方案

### 方案 A：一开始就用 LLM-as-a-Judge

- ❌ 冷启动无标注数据
- ❌ 成本太高（每次都评估）
- ❌ 准确率不稳定

### 方案 B：纯人工规则路由（永远不进化）

- ❌ 路由质量上限低
- ❌ 新模型/新任务需要专家持续配置
- ❌ 不可持续

### 方案 C：三阶段演进（**选择**）

- ✅ M1 立即可用
- ✅ 流量自然积累数据
- ✅ 每阶段都有真实数据支撑

## 后果

### 正面

- M1 立即可上线，路由策略可手动调整
- 真实流量驱动反馈，不依赖人工经验
- 渐进式增强，每阶段都有价值

### 负面

- M1 路由质量依赖专家经验
- 反馈聚合有延迟（M3 每日聚合，非实时）
- LLM-as-a-Judge 评估成本叠加

### 中性（需要承担的工作）

- 路由表存 PG（带 version）
- 每次调用结果落 `router_call_logs` 表
- M3 引入反馈聚合离线任务
- M4 引入 LLM 评估器

## 实施细节

### M1-M2 规则路由

```yaml
# router-svc/configs/routes.yaml
routes:
  - task: rfp_parse
    primary: { provider: anthropic, model: claude-sonnet-4 }
    fallback:
      - { provider: openai, model: gpt-4o }
      - { provider: deepseek, model: deepseek-chat }

  - task: outline_generate
    primary: { provider: deepseek, model: deepseek-chat }
    fallback:
      - { provider: openai, model: gpt-4o-mini }

  - task: content_generate
    primary: { provider: anthropic, model: claude-sonnet-4 }
    fallback:
      - { provider: openai, model: gpt-4o }

  - task: consistency_audit
    primary: { provider: anthropic, model: claude-sonnet-4 }
    fallback:
      - { provider: openai, model: gpt-4o }
```

**路由决策公式**：

```
model = f(任务画像, 路由表, 成本预算, 降级链)
```

**记录字段**：

```sql
CREATE TABLE router_call_logs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    workflow_id UUID,
    task VARCHAR(64) NOT NULL,         -- rfp_parse / outline_generate
    provider VARCHAR(32) NOT NULL,     -- anthropic / openai / deepseek
    model VARCHAR(64) NOT NULL,
    prompt_tokens INT NOT NULL,
    completion_tokens INT NOT NULL,
    latency_ms INT NOT NULL,
    error TEXT,
    cost_usd DECIMAL(10, 6) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### M3 隐式反馈

**收集信号**：

| 信号 | 含义 | 埋点 |
|---|---|---|
| 采纳率 | AI 生成后用户未修改的比例 | 编辑器对比 diff |
| 修改率 | 用户修改的字符占比 | 编辑器对比 diff |
| 重生成次数 | 用户点"重写"次数 | UI 事件 |
| 完成度 | 用户对章节标"完成" | UI 事件 |

**聚合任务**（每日 03:00）：

```sql
INSERT INTO router_quality_scores (
    provider, model, task, date,
    adopt_rate, modify_rate, regenerate_rate,
    sample_count, quality_score
)
SELECT
    provider, model, task,
    DATE(created_at),
    AVG(CASE WHEN adopted THEN 1 ELSE 0 END),
    AVG(modify_ratio),
    AVG(regenerate_count),
    COUNT(*),
    -- 加权得分
    0.5 * adopt_rate + 0.3 * (1 - modify_rate) + 0.2 * (1 - regenerate_rate)
FROM router_call_logs l
JOIN user_feedback f ON l.id = f.call_log_id
WHERE DATE(created_at) = CURRENT_DATE - INTERVAL '1 day'
GROUP BY provider, model, task, DATE(created_at);
```

**路由表动态调整**：

- 低质量分（< 0.6）降权 / 降级
- 高质量分（> 0.85）升权
- 用 A/B 灰度验证

### M4+ LLM-as-a-Judge

**抽样 1% 请求**，用 Claude Opus 评估：

```yaml
# ai_router/judge_config.yaml
judge:
  enabled: true
  sample_rate: 0.01
  provider: anthropic
  model: claude-opus-4
  prompt: |
    评估以下 AI 生成内容的质量（0-100）：
    任务：{task}
    输入：{input_summary}
    输出：{output_summary}

    评分维度：
    - 准确性（40%）
    - 完整性（30%）
    - 可读性（20%）
    - 合规性（10%）
```

**输出**：

```json
{
  "score": 87,
  "issues": ["格式不规范", "缺少数据来源"],
  "recommendation": "good"
}
```

## 退出条件

需要重新评估的触发条件：

- 🔴 规则路由明显劣于人工选择（< 70% 满意度）
- 🔴 隐式反馈信号噪声过大（误判多）
- 🔴 LLM-as-a-Judge 评估本身质量不够
- 🔴 出现新模型（如 GPT-5）需要快速接入

## 参考

- [架构 / AI 路由](../architecture/ai-router.md)
- [Plan / v1 设计 第 6 节](../plan/v1-design.md)
- [ADR-0003 自托管模型](0003-self-hosted-model.md)