# 0006. 行业模板市集冷启动

## 状态

Accepted

## 日期

2026-06-27

## 参与者

- 产品组
- 架构组
- 运营组

## 背景

投标行业差异大（信息化 / 政采 / 工程 / 咨询 / 设备 / 教育）。每个行业都有独特的章节结构、评分点、废标项。

**约束条件**：
- v1 没有用户内容（UGC）
- 模板质量是客户体验的关键
- 6 个主要行业需要覆盖
- UGC 需要 KYC 和审核

**需要决策**：v1 模板策略。

## 决策

**三阶段策略**：
1. **v1**：内部预置 6 个行业模板
2. **M3**：开放 UGC（KYC + 审核）
3. **M4**：引入付费模板（专家定制）

## 理由

- ✅ 冷启动需要预置内容
- ✅ 6 个行业覆盖 80%+ 国内投标市场
- ✅ UGC + 付费模板分阶段做，避免早期混乱
- ✅ KYC + 审核保证质量

## 考虑的替代方案

### 方案 A：完全开放 UGC（v1）

- ❌ 冷启动无内容
- ❌ 质量参差
- ❌ 审核成本高

### 方案 B：仅预置，永不开放（v1 + 永远）

- ❌ 模板数量受限
- ❌ 长尾行业无法覆盖

### 方案 C：分阶段（**选择**）

- ✅ v1 立即可用
- ✅ 渐进引入 UGC
- ✅ 最终商业生态完整

## 后果

### 正面

- v1 用户开箱即用
- 6 个主流行业覆盖
- 后续 UGC + 付费形成生态

### 负面

- 长尾行业覆盖不足
- 预置模板可能与实际客户需求有偏差
- UGC 需要持续运营投入

### 中性（需要承担的工作）

- 6 个行业模板制作
- 模板数据模型设计
- M3 UGC 审核机制
- M4 付费模板分成系统

## 实施细节

### 6 个预置行业

按国内投标市场份额选：

1. **信息化系统集成**（最大类，IT 服务 / 系统集成 / 软件采购）
2. **政府采购**（政企客户，规则严格）
3. **工程 EPC**（基建 / 房建 / 市政）
4. **咨询服务**（可研 / 监理 / 审计）
5. **设备采购**（硬件为主，技术参数密集）
6. **教育培训**（课程 / 培训服务）

### 模板内容结构

每个模板包含：

```
template/
├── meta.yaml              # 名称、行业、版本、作者
├── structure/
│   ├── outline.yaml       # 3 级大纲 + 子节点
│   └── scoring_points.yaml # 评分点对应
├── modules/               # 必备模块
│   ├── qualifications.md  # 资质清单
│   ├── cases.md           # 案例模板
│   ├── team.md            # 团队配置
│   ├── technical.md       # 技术方案框架
│   └── commercial.md      # 商务报价模板
├── rules/
│   ├── must_answer.yaml   # 必答条款清单
│   ├── blacklist.yaml     # 废标项黑名单
│   └── style_guide.md     # AI 写作风格指南
└── prompts/               # AI Prompt 模板
    ├── parse.md
    ├── outline.md
    ├── content.md
    └── audit.md
```

### 数据模型

```sql
CREATE TABLE templates (
    id UUID PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    industry VARCHAR(64) NOT NULL,
    version VARCHAR(32) NOT NULL,
    author_id UUID,                          -- NULL = 官方模板
    visibility VARCHAR(16) NOT NULL,         -- private | team | marketplace
    price_cents INT DEFAULT 0,
    downloads INT DEFAULT 0,
    rating_avg DECIMAL(3, 2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE template_sections (
    id UUID PRIMARY KEY,
    template_id UUID NOT NULL REFERENCES templates(id),
    parent_id UUID REFERENCES template_sections(id),
    title VARCHAR(256) NOT NULL,
    order_idx INT NOT NULL,
    type VARCHAR(32) NOT NULL,                -- heading | paragraph | table | image | ...
    config JSONB NOT NULL DEFAULT '{}',
    FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE
);

CREATE TABLE template_rules (
    id UUID PRIMARY KEY,
    template_id UUID NOT NULL,
    rule_type VARCHAR(32) NOT NULL,           -- must_answer | blacklist | scoring
    content TEXT NOT NULL,
    severity VARCHAR(16) NOT NULL,           -- critical | warning | info
    FOREIGN KEY (template_id) REFERENCES templates(id) ON DELETE CASCADE
);
```

### 模板应用流程

```
用户创建项目
   ↓
选择模板（或空白）
   ↓
若选模板：
  - 复制 template_sections 到 project_outline_nodes
  - 复制 template_rules 到 project_audit_rules
  - 加载 prompts
   ↓
项目可以编辑大纲（不影响原模板）
```

### M3 UGC 规则

**作者资格**：
- 完成 KYC（实名认证）
- 历史成功标书数 ≥ 5
- 信用分 ≥ 4.0

**审核机制**：
- 内部审核（24 小时内）
- 用户举报（限 5 次/天/用户）
- 自动检测敏感词、版权词

**收益**：
- 模板被使用时分成 10-30%（阶梯式）
- 评价好的模板获得"精选"标签

### M4 付费模板

- 行业专家定制模板（5000 - 50000 元/个）
- BidWriter 官方担保
- 30 天无理由退款
- 模板作者获得 70% 收入

## 退出条件

需要重新评估的触发条件：

- 🔴 预置模板与实际需求严重不符（评分 < 3.5）
- 🔴 6 个行业不足以覆盖目标客户（客户主动要求新行业 > 30%）
- 🔴 UGC 模板质量无法保证
- 🔴 付费模板市场冷淡

## 参考

- [架构 / 模块设计 - template-svc](../architecture/modules.md#template-svc)
- [Plan / v1 设计 第 9 节](../plan/v1-design.md)
- [产品概览 - 目标用户](../introduction/users.md)