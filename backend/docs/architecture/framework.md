好，这一层我们直接进入**标书 AI 的"核心发动机"设计：RFP → Agent拆解系统（真正决定产品上限）**。

这一部分做清楚，后面不管是 Dify、LangChain 还是自研，都只是"实现方式"。

---

# 📄 标书 AI Agent 系统设计（核心模块）

> 本文档是设计纲要；实证对照项目：OpenBidKit_Yibiao（易标投标工具箱，
> AGPL-3.0，GitHub FB208/OpenBidKit_Yibiao，当前 v2.15.1）。
> 项目分析详见 `/work/ai/OpenBidKit_Yibiao/yibiao.md`（511 行），
> 本文件每一处抽象概念都对应到该项目的实际代码 / 表 / 服务。

---

# 〇、设计 → 实现的对照表（导读）

抽象概念              实证位置（yibiao）                              关键文件
─────────────────────────────────────────────────────────────────────────────
RFP Parser Agent       Step02 招标文件解析（三档）                    electron/services/opencode/* + doc2markdown/
Requirement Decomposer Step03 大纲生成                                features/technical-plan/ + technical_plan_outline_nodes
Knowledge Retrieval    知识库（"你不需要 RAG"哲学）                  features/knowledge-base/ + knowledge_* 表
Multi-Writer Agents    Step05 正文生成（6290 行状态机）               electron/services/contentGenerationTask.cjs
Consistency Checker    全文一致性审计 + agent 修复                    electron/utils/textEdit.cjs + agentService.cjs
Workflow 编排          Step01-05 + 任务组锁                          electron/services/taskService.cjs
数据结构持久化        SQLite v15（35+ 表）+ 大文本文件分离           electron/services/sqliteDatabase.cjs
AI 调用               aiService（队列） + agentService（OpenCode）   electron/services/{ai,agent}Service.cjs
Word 导出             2315 行 exportService                          electron/services/exportService.cjs

下面章节每个抽象都会给出"yibiao 实证锚点"小节，方便设计评审对照。

---

# 一、问题重新定义（非常关键）

传统系统的问题是：

> ❌ "把标书当作文档生成问题"

而真正正确的抽象是：

> ✅ "把标书当作结构化任务分解与证据生成问题"

yibiao 实证锚点：
- 项目内部把"标书写作"分解为 Step01-05，每一步只做一件事，可独立回滚 / 暂停 / 恢复
- 文档只是中间载体，正文是"按大纲节点生成的 Section 集合"，每个 Section 有独立状态
- 工程现实：把"AI 标书助手"做成"AI 标书任务调度系统 + 证据聚合 + 校验流水线"

---

## 1. 输入不是"文档"，而是"约束集合"

RFP 本质包含：

* 技术要求
* 商务要求
* 合规要求
* 打分规则
* 禁止项

yibiao 实证锚点：
- 技术方案 meta 表 50+ 列，从招标文件 Markdown 抽取的字段包括：项目名、采购方、标段、
  投标截止时间、评分规则摘要、合规条款清单
- 配置文件 user_config.json 持久化 API Key / 模型选择 / 全局参数
- 全局事实（global_fact_groups）作为跨章节共享的"约束集"

---

## 2. 输出不是"文本"，而是：

* 响应矩阵（Response Matrix）
* 证据链（Evidence Chain）
* 标书章节结构（Bid Structure）

yibiao 实证锚点：
- 响应矩阵 → 废标项检查（rejection_check_* 三类并发：废标/错别字/逻辑谬误）
- 证据链 → knowledge_* 文档库 + global_fact_groups 跨章节事实聚合
- 章节结构 → technical_plan_outline_nodes（树形，level+parentId+order）+ 
  technical_plan_content_sections（按状态：pending/done/failed）

---

# 二、核心系统架构（Agent版）

```text id="bid_agent_arch"
                ┌────────────────────┐
                │   RFP Document      │
                └─────────┬──────────┘
                          ↓
            ┌──────────────────────────┐
            │ 1. RFP Parser Agent       │  ← Step02 + doc2markdown
            │（结构化拆解）             │
            └─────────┬──────────────┘
                      ↓
        ┌──────────────────────────────┐
        │ 2. Requirement Decomposer     │  ← Step03 + outline_nodes
        │（需求拆分成任务树）          │
        └─────────┬────────────────────┘
                      ↓
      ┌──────────────────────────────────┐
      │ 3. Knowledge Retrieval Agent      │  ← knowledge-base（"你不需要 RAG"）
      │（历史标书 / 公司知识库）        │
      └─────────┬──────────────────────┘
                      ↓
   ┌──────────────────────────────────────┐
   │ 4. Multi-Writer Agents               │  ← Step05 内容生成 + 配图
   │ - 技术标写作Agent                    │
   │ - 商务标写作Agent                    │
   │ - 合规标写作Agent                    │
   └─────────┬──────────────────────────┘
                      ↓
   ┌──────────────────────────────────────┐
   │ 5. Consistency & Fact Checker Agent   │  ← textEdit.cjs + agentService
   │（防幻觉 / 资质校验）                 │
   └─────────┬──────────────────────────┘
                      ↓
              Final Bid Package
```

yibiao 实证补充：所有 Agent 任务都在主进程后台跑（taskService.cjs），Renderer
只订阅事件 + 读 Store；任务状态全程落盘可恢复（paused/restoring/auditing等）。

---

# 三、核心 Agent 设计（重点）

---

# 1. RFP Parser Agent（结构化解析）

## 目标

把 PDF/Word 变成：

```json
{
  "project": "...",
  "sections": [
    {
      "title": "技术要求",
      "items": [...]
    }
  ]
}
```

## 技术关键点

### 输入处理

* PDF parsing（layout-aware）
* OCR（扫描件）
* docx结构抽取

yibiao 实证锚点 — **三档解析策略**（Step02 招标文件解析）：
```
本地模式      LibreOffice 转换 + doc2markdown
MinerU-agent  内部 Agent 边转换边抽取（适合复杂 PDF）
MinerU 精准   付费 API（高精度）
```
特殊处理：
- .doc / .wps 自动识别；WPS > LibreOffice > Word(Win COM)
- 文件编码强制 UTF-8；Windows 中文路径是默认场景
- 大文本不入库 — 招标正文落文件（technical-plan/tender.md），SQLite 只存 hash

---

### LLM任务

```text
你是RFP解析专家，请将以下文档拆解为结构化需求树：
- 章节
- 子需求
- 评分点
- 强制条款
```

---

## 输出结构（标准）

```json
{
  "requirements": [
    {
      "id": "T1",
      "type": "technical",
      "text": "...",
      "mandatory": true,
      "keywords": []
    }
  ]
}
```

yibiao 实证锚点 — 该结构对应 technical_plan_meta 50+ 列单行表：
- 解析结果按字段写入对应列（不是 JSON blob），便于查询 / 索引 / 检索
- 强制条款另存到 compliance 子表，供 Step05 一致性审计时按条款核验

---

# 2. Requirement Decomposer Agent（核心中的核心）

## 目标

把 RFP 变成"可执行任务树"

## 输入

```json
requirements[]
```

## 输出（关键）

```json
{
  "tasks": [
    {
      "task_id": "T1",
      "category": "technical",
      "subtasks": [
        "系统架构设计",
        "技术路线说明",
        "性能指标响应"
      ]
    }
  ]
}
```

## 本质

这是一个：

> 🧠 "规划器（Planner Agent）"

## 关键能力

* 任务拆解
* 优先级排序
* 依赖分析

yibiao 实证锚点 — Step03 大纲生成 + technical_plan_outline_nodes 表：
```sql
CREATE TABLE technical_plan_outline_nodes (
  id          INTEGER PRIMARY KEY,
  plan_id     INTEGER NOT NULL,
  parent_id   INTEGER,                       -- 树形
  level       INTEGER NOT NULL,              -- 1/2/3 级标题
  order_index INTEGER NOT NULL,              -- 同级排序
  title       TEXT NOT NULL,
  status      TEXT,                          -- pending/generating/done
  -- ... 节点级元数据
);
CREATE INDEX idx_outline_parent_order ON outline_nodes(parent_id, order_index);
CREATE INDEX idx_outline_level ON outline_nodes(level);
```
- 拆解粒度：标题 3 级 + 子节点 ≤3 轮扩写（MAX_OUTLINE_EXPANSION_ROUNDS=3，
  每轮 6 步 / 目标覆盖率 0.8）
- 优先级：合规条款 → 技术要求 → 商务要求 → 加分项
- 依赖：父子节点强依赖；兄弟节点弱依赖（可并行生成）

---

# 3. Knowledge Retrieval Agent（RAG层）

## 输入

* task
* requirement context

## 检索内容

* 历史标书
* 公司资质
* 技术文档
* 成功案例

## 技术实现

### 向量检索

* embedding
* top-k retrieval

### 关键增强

#### hybrid retrieval：

* keyword search（BM25）
* semantic search（embedding）

## 输出

```json
{
  "evidence": [
    {
      "source": "historical_bid_2023.docx",
      "content": "..."
    }
  ]
}
```

yibiao 实证锚点 — **设计哲学："你不需要 RAG"**：
项目作者（Mark）在「标书智能体（七）」中明确反对盲目上 RAG，实际采用：
- 文档统一走 doc2markdown → Markdown 中间态
- knowledge_documents（v11+ 新增 sort_order）+ knowledge_folders + knowledge_chunks
- 检索走"目录树 + 关键词 + 用户主动分类"，而非向量相似度
- 全局事实（global_fact_groups）作为跨章节"准 RAG"层：
  - 从招标 + 参考文档抽取事实标题清单
  - 注入正文编排 + 生成 Prompt 作为"硬约束"
  - Step05 前置条件：globalFactsTask.status === 'success'

理由：
- 标书场景召回率比相似度重要，目录树 + 标签已足够
- 弱模型在 RAG 上容易"串文档"，反而引入幻觉
- 用户愿意手动维护目录，因为这是企业核心资产

---

# 4. Multi-Writer Agents（写作层）

## 结构设计

不是一个 LLM，而是多个"角色 Agent"：

### ① 技术标写作 Agent

输出：

* 技术方案
* 架构设计
* 实施方案

### ② 商务标写作 Agent

输出：

* 公司介绍
* 项目经验
* 资源能力

### ③ 合规标 Agent

输出：

* 响应表
* 条款对照
* 合规说明

## 写作方式（关键）

### 输入：

```json
{
  "task": "...",
  "evidence": [...]
}
```

### Prompt约束：

必须满足：

* 只能基于 evidence
* 不允许编造
* 必须结构化输出

## 输出示例：

```json
{
  "section": "技术方案",
  "content": "...",
  "citations": ["doc1", "doc2"]
}
```

yibiao 实证锚点 — **Step05 完整状态机**（contentGenerationTask.cjs，6290 行）：

```
planning → generating → outline-expanding (≤3 轮, 6 步/轮, 目标比 0.8)
   ↓
restoring        (仅 existing-plan-expansion)
   ↓
expanding        (单小节补足 ≥800 字)
   ↓
original-auditing (仅 existing-plan-expansion，repaired 状态)
   ↓
auditing         (一致性 normal / agent 模式)
   ↓
table-cleaning   (上下文 600 字 / 批 3 万字)
   ↓
illustrating     (AI 图 + Mermaid，并发 2/5)
   ↓
done
```

关键常量：
```
CONSISTENCY_AUDIT_GROUP_WORD_LIMIT = 300000   # 按字数分桶防 context 爆
CONSISTENCY_REPAIR_MAX_ATTEMPTS   = 2
ORIGINAL_PLAN_SEGMENT_MAX_CHARS    = 6000
MAX_OUTLINE_EXPANSION_ROUNDS       = 3
MERMAID_RENDER_TIMEOUT_MS          = 15000
text 并发                          = 10
image 并发                         = 2
```

并发模型：
- 章节级并行（Outline expansion）
- 段级串行（保证 Prompt 缓存命中）
- 配图阶段 AI 图 + Mermaid 并发 2/5
- AI 队列全局调度（aiRequestQueue.cjs），作用域由 taskService.withQueueScope() 注入

---

# 5. Consistency & Fact Checker Agent（防废标关键）

## 这是整个系统最重要的一层

## 检查内容：

### 1. 事实一致性

* 是否虚构资质？
* 是否引用不存在案例？

### 2. 跨章节一致性

* 技术参数是否冲突
* 商务承诺是否矛盾

### 3. 合规检查

* 是否满足强制条款
* 是否遗漏必填项

## 输出：

```json
{
  "issues": [
    {
      "type": "hallucination",
      "section": "商务标",
      "problem": "引用了不存在的ISO认证"
    }
  ]
}
```

yibiao 实证锚点 — 三层防护体系：

**第一层：废标项检查**（rejection-check 模块，2134 行）
- documents 表（v12 改多份投标文件）
- 三类并发检查：
  - risk_findings      废标项（合规）
  - typo_findings      错别字
  - logic_findings     逻辑谬误
- 任务组锁 group-exclusive：解析 + 检查共用缓存

**第二层：全文一致性审计**（Step05 auditing）
- 触发位置：ensureMinimumWords() 之后、配图之前
- 字数分桶：每桶 ≤30 万字防 context 爆
- 普通模式：基于文本相似度的局部比对
- agent 模式：调用 OpenCode 多步推理 + 精确替换

**第三层：精确替换引擎**（textEdit.cjs，446 行）
```
findTextMatches(content, oldText, options)
  ├─ findExactMatches              精确字符串
  ├─ findTrimmedBoundaryMatches    边界 trim
  ├─ findLineTrimmedMatches        行级 trim
  ├─ findWhitespaceNormalizedMatches 空白归一
  └─ findBlockAnchorMatches        块级 anchor + Levenshtein
planTextEdits(content, edits)
applyRangeEdits(content, edits)    真正写入
```
- OpenCode 风格 old_text / new_text 精确唯一替换
- 多策略 fallback：精确不命中 → trim → 行 trim → 空白归一 → anchor 模糊
- validateNonOverlapping() 防编辑区重叠
- isDisproportionateMatch() 防误把大段当单次 edit

**第四层：标书查重**（duplicate-check，1028 行 + 14 张表）
- outline_pairwise（score 索引）
- content_duplicates
- duplicate_images（hash 索引）
- 与历史标书比对，避免内容雷同被废

---

# 四、核心数据结构设计（非常关键）

## 1. RFP结构

```json
RFP
 ├── sections
 ├── requirements
 ├── scoring_rules
 ├── compliance_rules
```

## 2. Task Graph

```json
Task
 ├── subtasks
 ├── dependencies
 ├── required_evidence
```

## 3. Evidence Graph

```json
Evidence
 ├── source
 ├── reliability_score
 ├── timestamp
```

yibiao 实证锚点 — **SQLite v15 schema（35+ 表）**：

```
表族                表数    关键表                              用途
─────────────────────────────────────────────────────────────────────
technical_plan_*    8       meta(50列单行)                     主菜
                            tasks
                            outline_nodes(树形, level+parent)
                            content_sections(按状态)
                            content_plans
                            global_fact_groups                 跨章节事实
                            reference_docs
duplicate_check_*   14      files
                            outline_pairwise(score 索引)
                            content_duplicates
                            duplicate_images(hash 索引)
                            metadata_items
                            outline_items(归一化索引)
rejection_check_*   8       documents(v12 改多份)
                            results
                            risk_findings / typo_findings
                            logic_findings
knowledge_*         5+      folders
                            documents(v11+ sort_order)
                            chunks
export_templates_*  (v15 新)                                     模板库
config_*            -                                            KV 配置
```

启动设置：
```
journal_mode  = WAL
foreign_keys  = ON
busy_timeout  = 5000
schemaVersion = 15
migrations[]   = v1..v15
currentVersion > schemaVersion → 拒绝运行（提示升级客户端）
升级前自动备份 *.db → userData/workspace/backups/
```

迁移链关键节点：
- v6  标段字段（幂等 ALTER，try/catch）
- v11 知识库 sort_order
- v12 废标项多投标文件（重命名 → 重建 → 数据迁移 → 删旧表，单事务）
- v13 已有方案目录 outline_expansion_mode
- v14 多标段优化状态
- v15 导出模板库

---

# 五、核心工作流（完整链路）

```text id="bid_flow"
RFP上传
   ↓
解析Agent                                              ← Step02
   ↓
需求拆解Agent                                          ← Step03
   ↓
任务图生成
   ↓
知识检索（"你不需要 RAG"）                            ← knowledge-base
   ↓
全局事实抽取                                           ← Step04（新增）
   ↓
多写作Agent并行生成                                   ← Step05 planning→generating
   ↓
大纲扩写（≤3 轮，目标 0.8）                          ← Step05 outline-expanding
   ↓
单小节补足（≥800 字）                                ← Step05 expanding
   ↓
一致性审计（normal / agent）                         ← Step05 auditing
   ↓
表格清洗                                              ← Step05 table-cleaning
   ↓
配图（AI 图 + Mermaid）                              ← Step05 illustrating
   ↓
废标项检查（三类并发）                                ← rejection-check
   ↓
标书查重                                              ← duplicate-check
   ↓
人工审核（可选）
   ↓
Word / PDF 输出                                       ← exportService（2315 行）
```

yibiao 实证锚点 — **任务组锁矩阵**（taskService.cjs）：

```
任务组                       锁策略           备注
─────────────────────────────────────────────────────────────────
technical-plan               group-exclusive  Step02/03/04 互斥；Step05 独立 group
content-generation           group-exclusive  Step05 主任务，独立 group
consistency-audit            group-exclusive  全文一致性审计
consistency-repair           group-exclusive  一致性修复（agent mode）
original-plan-expansion      group-exclusive  已有方案扩写
outline-coverage-repair      group-exclusive  原方案覆盖率修复
table-cleaning               group-exclusive  表格清洗
duplicate-check              group-exclusive  外层单跑，子流程并发
rejection-check              group-exclusive  解析 + 检查共用缓存
knowledge-base               scope-exclusive  按 documentId 加锁（旧式未迁入）
```

任务生命周期（来自 taskService.cjs）：
- 页面卸载不取消任务；重挂载必须读 Store + 订阅事件 + getActiveTasks() 状态回放
- 异常关闭后状态落盘可恢复（paused / restoring / auditing 等）
- 队列作用域 withQueueScope({ scope, fn }) 注入 scoped aiService
- AI_QUEUE_SCOPE_PAUSED 让业务层走自己的暂停流程（如 Step05 pauseIfRequested()）

---

# 六、关键技术难点（真实难点）

## 1. 长文档结构理解

RFP通常：

* 200~1000页
* 非结构化

👉 难点是"语义切块"

yibiao 实证：
- doc2markdown 中间态（HTML 保留表格/图片/公式）
- userTextSplitter.cjs 按章节切分
- Step02 三档解析按文档类型自适应
- 全局事实（global_fact_groups）作为跨章节"索引"

## 2. 多章节一致性

例如：

* 技术说 A 架构
* 商务说 B 成本
* 合规说 C 承诺

必须统一

yibiao 实证 — 三层防护（见 Consistency Agent 章节）：
- 废标项扫描（rejection-check）
- 全文一致性审计（Step05 auditing）
- 跨章节事实聚合（global_fact_groups 作为"硬约束"注入 Prompt）

## 3. 证据约束生成（最关键）

必须做到：

> "没有证据就不能写"

yibiao 实证 — Prompt 缓存策略（与 aiService.cjs 对应）：
- 大段共享上下文放最前面
- 相同 system prompt 复用前缀（成本直降 10 倍，见「标书智能体（四）」）
- 先预热再 fan-out
- 二级以下目录可并发

JSON 修复链路（aiRetry.cjs + aiService.cjs）：
```
streamJSON 主请求结束
   ↓
parseJsonResponseContent() 提取
   ↓
validateJsonShape() 校验
   ↓ 失败
repairJsonResponse() 局部修补（不重发完整大文本）
```

## 4. 行业差异极大

* IT标书
* 政府采购
* EPC工程

结构完全不同

yibiao 实证 — 模板系统（export-format，2523 行）：
- 页框 / 主题 / 表格样式 / 段落样式 / 章节页 / 序号编号
- 用户可自定义模板，导出时实时套用
- 同一份大纲可生成多种格式的 Word 文档

---

# 七、产品化建议（非常关键）

## MVP路线（强烈建议）

### 第一阶段（1-2周）

* RFP解析 + 章节拆解

### 第二阶段（2-4周）

* RAG知识库 + 自动写章节

### 第三阶段（4-6周）

* 多Agent + 校验系统

## 不建议一开始做：

❌ 全自动标书生成
❌ 复杂UI系统
❌ 多行业支持

---

# 八、核心结论（非常重要）

## 标书AI系统本质是：

> 🧠 "结构化任务系统 + 证据驱动生成系统"

## 真正的竞争壁垒：

1. RFP结构理解能力
2. 企业知识库沉淀
3. Agent拆解质量
4. 合规控制能力

---

# 九、工程实现参考（来自 yibiao 实证）

> 这一章是设计文档之外、根据实际项目代码补充的"工程落地要素"。
> 如果你要自研一个标书 AI 系统，下面这些是必须同时设计、不能等代码写完再补的部分。

## 9.1 三层进程架构（Electron 桌面客户端）

```
┌─────────────────────────────────────────────────────────────────────────┐
│ 渲染进程 (client/src, ESM TypeScript)                                  │
│  - React 19 + Vite 7 + Radix UI                                       │
│  - 路由、Provider、UI 编排、状态订阅                                   │
│  - 不直接访问 Node API                                                │
└─────────────────────────────────────────────────────────────────────────┘
                            │ contextBridge
                            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ Preload (electron/preload.cjs)                                        │
│  - 唯一暴露 window.yibiao.* 桥（类型在 src/shared/types/ipc.ts）      │
│  - 共 ~99 个 typed wrapper                                             │
└─────────────────────────────────────────────────────────────────────────┘
                            │ ipcRenderer.invoke / on
                            ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 主进程 (electron/, CommonJS .cjs)                                     │
│  - ipc/  只注册 / 转发，零业务（76 个 handler / 14 个分类文件）       │
│  - services/ 全部业务逻辑（Store / AI / Task / Export / Agent...）    │
│  - utils/ 工具（aiRetry / aiRequestQueue / textEdit / mermaidCache...） │
└─────────────────────────────────────────────────────────────────────────┘
```

边界纪律（AGENTS.md 强规则）：
- Renderer 严禁直接 Node / fs / ipcRenderer，只走 window.yibiao
- IPC 文件零业务，逻辑放 services/*.cjs
- shared/ 不引用任何 feature（feature 之间不互引）
- 不在 Renderer 存 API Key

## 9.2 SQLite 单库 + 大文本文件分离

为什么不把所有东西塞 SQLite：
- 标书正文 10-50 万字，BLOB 会让库膨胀 / 备份 / diff 都难做
- 大文本走文件系统，SQLite 只存 path / hash / count / status

布局（userData/）：
```
user_config.json                  配置（API Key / 模型 / analytics_created_at）
workspace/yibiao.sqlite           主库（v15, 35+ 表）
workspace/technical-plan/tender.md 招标 Markdown 原文
workspace/<feature>/...           各功能子目录
workspace/backups/*.db            升级前自动备份
userData/logs/<module>/*.jsonl    开发者模式才写
```

## 9.3 AI 双轨架构（核心）

```
                ┌─────────────────────────────────────────────┐
                │  Renderer (共享 aiClient 接口)              │
                │  window.yibiao.ai.chat / requestJson        │
                └────────────────┬────────────────────────────┘
                                 │
              ┌──────────────────┴──────────────────┐
              ▼                                     ▼
  ┌────────────────────────┐         ┌────────────────────────────┐
  │ 轨道 A：aiService.cjs   │         │ 轨道 B：agentService.cjs   │
  │ - 全局 text/image 队列 │         │ - OpenCode 子进程          │
  │ - textModel 并发 10    │         │ - 独立 runtime + workspace  │
  │ - imageModel 并发 2    │         │ - AI proxy 二次封装        │
  │ - 重试 ≤3 次           │         │ - 适用：多步/工具/长链推理 │
  │ - 适用：单次任务       │         │                            │
  └────────────────────────┘         └────────────────────────────┘
```

OpenCode 集成（"邪修"模式，详见「文章/新系列二」）：
- 每次 runTask 启动一组双进程：
  - AI 反向代理（createAiServiceOpenAiProxy）
    - server.listen(0, '127.0.0.1', ...)  // 内核随机端口
    - token: YIBIAO_OPENCODE_PROXY_TOKEN  // 双向 header auth
    - 内部再封装 AI_REQUEST_QUEUE + 重试
  - OpenCode serve 子进程（spawn vendor/opencode）
    - OPENCODE_SERVER_USERNAME/PASSWORD  // Basic Auth
    - YIBIAO_OPENCODE_PROXY_TOKEN         // 拿上游 AI
    - 关闭 autoupdate/plugins/models_fetch/claude_code

环境隔离（每个任务独立）：
```
getAgentRuntimeDir(app)/<taskId>/
  ├── home/                        HOME 重定向
  ├── workspace/                   payload.files + output_content
  └── XDG_CONFIG_HOME/DATA/CACHE   子目录
```

默认超时 10 分钟；健康检查 30 秒。

关键约束：业务侧（如 consistencyRepair agent 模式）调用 agentService.runTask
后拿到输出 Markdown，由调用方决定如何合并到 outline.content ——
**agent 不直接动业务数据**，必须经过适配层。

## 9.4 队列与重试

aiRequestQueue.cjs（全局）：
- textModel 默认并发 10
- imageModel 默认并发 2
- 任务失败延后重试，不阻塞队列

aiRetry.cjs：
- AI_REQUEST_MAX_ATTEMPTS = 3
- isRetryableAiRequestError()：HTTP 408/429/5xx / 网络超时 / ECONNRESET
- getAiRetryDelayMs()：指数退避

taskService.cjs（作用域）：
- withQueueScope({ scope, fn }) 注入 scoped aiService
- AI_QUEUE_SCOPE_PAUSED 让业务层走自己的暂停流程

## 9.5 Word 导出（exportService.cjs，2315 行）

```
exportWord({ outline, project_name, ... })
  ├─ countOutlineStats(outline)              字数/Mermaid/表格
  ├─ dialog.showSaveDialog()                 默认下载目录
  ├─ buildDocxResult(payload, ...)
  │     ├─ markdown → AST (unified/remark/rehype)
  │     ├─ htmlNodeToDocxBlocks / htmlTableToDocx / htmlListToDocx
  │     ├─ resolveMermaidImageForExport()    mermaid.ink 或本地
  │     ├─ imageRunFromNode()                图片节点
  │     ├─ mmToTwips()                       长度单位转换
  │     └─ buildDocxBuffer()                 docx 库
  ├─ fs.writeFileSync(filePath, buffer)
  └─ developerLogger.write('export.word.completed', ...)
```

进度上报：onProgress(percent, message, { phase, stats, warnings })
Mermaid 处理：先 mermaid.ink 在线渲染，失败降级本地，仍失败则作为占位图片
HTML → docx：自实现，不走 headless browser

## 9.6 远程埋点统计（独立 Cloudflare 全栈）

为什么独立部署而不是嵌进桌面客户端：
- 桌面客户端在用户本地，统计无法跨用户聚合
- Cloudflare 边缘零运维，按请求计费
- Worker + D1 + R2 + AE + KV 全家桶覆盖实时/历史/资源/公告

```
client --HTTP /track--> Worker (analytics/)
                          ├ Analytics Engine (实时聚合, 12 blob + 4 double)
                          └ D1 stats_* (历史聚合, Cron 02:15 跑昨日)
Worker /api/overview|traffic|clients|model-usage|config-usage|ip-stats
Dashboard 读 D1 历史 + AE 近期
```

Cron 5 段拆分（北京时间 01-03 点）：
```
01:00 discover + daily
01:30 clients
02:00 pages + versions
02:30 configs + models
03:00 retention + resources
```

---

# 十、风险与边界（来自 yibiao 实证）

## 10.1 单点复杂度

- technical-plan 模块 6067 行 TS + 一票 cjs（Step05 状态机 6290 行）
- 任何 Step 改造都要 double-check 两个入口（生成 + 已有方案扩写）
- 缓解：严守任务组锁边界，每个 group 一个 .cjs 文件

## 10.2 Native ABI 陷阱

- better-sqlite3 / sqlite3 必须在 Electron ABI 下重建
- 旧版 GCC（< 9）无法编译 native binding（`-std=c++20` 不识别）
- 必须 postinstall: electron-builder install-app-deps
- npm ci 后忘记 postinstall → config:load 未注册 → 全局崩溃

## 10.3 代码签名

- Windows / macOS 暂未签名
- 用户首次启动会看到"未知发布者"警告
- 已知约束，未阻塞发布

## 10.4 Agent 与业务 Store 的边界

- OpenCode agent 第一版只写自己 workspace，不直接改业务 Store
- 业务侧必须做适配层合并 → 增加一次工程负担
- 优点：agent 故障不会污染业务数据
- 缺点：UI 上 agent 结果不会立即可见，需要手动"应用"

## 10.5 弱模型 JSON 不稳定

- 标书场景 prompt 极长，弱模型（7B/13B）经常输出残缺 JSON
- 必须配 repairJsonResponse() 链路
- 流式 JSON 主请求结束再走修复，不重发完整大文本
- 详见「标书智能体（五）」

## 10.6 任务取消语义

- 页面卸载不取消任务（用户体验优先）
- 重挂载必须读 Store + 订阅事件 + getActiveTasks() 回放
- 异常关闭后状态可恢复（paused/restoring/auditing）
- 任务组锁保证不会两个 Step 并发写同一份 outline

---

# 十一、下一步建议（结合 yibiao 演进）

原建议（设计阶段）：
1. 标书 Agent Prompt 体系（工业级）
2. Dify / LangGraph MVP 实现方案
3. 商业化产品 PRD（对标 RFPIO + 国内差异）

补充（基于 yibiao 已落地经验）：
4. **任务状态机可视化** —— 用户能清楚看到"当前卡在哪一步、可恢复/可暂停"
5. **大文档分段策略** —— Step05 全局事实当前没做大文件分段（活跃 Issue #114）
6. **废标项多投标文件** —— v12 已支持，仍在持续打磨（活跃 Issue #49/超长标书）
7. **OpenCode agent 适配层** —— 把 agent 输出合并到 outline.content 的标准协议
8. **代码签名 + 跨平台分发** —— 移除"未知发布者"警告
9. **行业模板市集** —— v15 导出模板库已就位，需要模板生态
10. **企业知识库云同步** —— 当前 knowledge-base 只在本地，企业级需云端 + 权限

---

# 附录 A：文件 / 模块速查表（yibiao）

```
client/electron/
  main.cjs                                 入口
  preload.cjs                              桥接（99 API）
  ipc/   14 文件                           76 个 handler
  services/  27 文件                       业务逻辑
    ├ aiService.cjs                        单次 AI
    ├ agentService.cjs                     OpenCode agent 入口
    ├ opencode/  4 文件                    双进程编排
    ├ taskService.cjs                      任务队列 + 组锁
    ├ sqliteDatabase.cjs                   v15 schema
    ├ exportService.cjs                    Word 导出（2315 行）
    ├ contentGenerationTask.cjs            Step05（6290 行）
    ├ duplicateCheckService.cjs            标书查重
    ├ rejectionCheckTask.cjs               废标项检查
    ├ knowledgeBaseService.cjs             知识库
    ├ configStore.cjs                      全局配置
    ├ technicalPlanStore.cjs               技术方案数据
    ├ textTokenStatsStore.cjs              Token 用量
    └ updateService.cjs                    GitHub release 自动更新
  utils/  16 文件
    ├ aiRetry.cjs                          重试退避
    ├ aiRequestQueue.cjs                   AI 队列
    ├ textEdit.cjs                         精确替换（446 行）
    ├ mermaidCache.cjs                     Mermaid 缓存
    ├ userTextSplitter.cjs                 切片
    └ ...

client/src/
  app/                                     路由 / Provider / 菜单
  features/  12 模块
    ├ technical-plan/    6067 行          主菜
    ├ export-format/     2523 行          模板
    ├ rejection-check/   2134 行          废标
    ├ settings/          1921 行          设置
    ├ knowledge-base/    1803 行          知识库
    ├ duplicate-check/   1028 行          查重
    ├ developer/         669 行           开发者模式
    ├ business-bid/      105 行           商务标（轻量）
    ├ bid-opportunity/   95 行            标讯（轻量）
    ├ resources/         208 行           资源下载
    └ export-format/, my-templates/
  shared/                                  跨 feature 公共件
    ├ ai/                                aiClient
    ├ analytics/                         埋点
    ├ prompts/                           Prompt 体系
    ├ types/                             类型权威
    ├ ui/                                UI 组件
    └ utils/

analytics/                                 独立 Cloudflare 全栈
  worker/  (Routes + Cron + D1 + R2 + AE + KV)
  dashboard/  静态 SPA
  scripts/

文章/  9 篇系列博客（产品文档级）
  标书智能体（一）-（七）  +  新系列一/二
```

---

# 附录 B：版本与状态（截至 v2.15.1）

```
最近 commit  95d5906  Merge PR #128 导出格式增加序号编号  2026-06-26
最近 tag     v2.15.1
schema       v15
npm 依赖     better-sqlite3@12.4.1, sqlite3@5.1.7, electron 41+, vite 7, react 19, ts 5.9
发布         GitHub release（FB208/OpenBidKit_Yibiao）→ yibiaoai/yibiao-simple 镜像
官网         https://yibiao.pro
许可         AGPL-3.0
```

活跃 Issue（部分）：
- #113 招标文件解析任务锁定异常
- #114 全局事实没做大文件分段
- #47 招标文件分析报告（临时替代商务标）
- #49 超长标书废标项检查

---

文档完成。所有抽象概念均已在 yibiao 项目中实证；新增的"工程实现"
"风险与边界" "下一步建议"三章来自实际代码与运维经验，可直接作为
自研立项的工程参考清单。

继续往下拆到可以开工级别。