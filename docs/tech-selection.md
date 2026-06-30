# 技术选型文档

> 本文档基于 `docs/framework.md` 的设计纲要，对每个技术领域给出推荐选型与候选对比。
> 选型原则：性能与工程化并重——后端优先采用 Go（或 Rust）以保证并发与吞吐，前端统一基于 Node.js 生态；MVP 阶段优先成熟度与生态丰富度；后续按性能、成本、可维护性逐项替换。

---

# 〇、选型原则

1. **后端性能优先**：核心服务用 Go 1.23+（goroutine + 静态二进制，I/O 并发性能优），Rust 作为 CPU 密集模块的备选
2. **前端 Node.js 统一**：Web 与桌面客户端均运行在 Node.js 生态上（Vite/构建、Electron 主进程），统一包管理与构建链
3. **可替换**：每个组件都要有清晰的抽象边界，便于后续替换
4. **可降级**：AI 调用、图表生成、文档转换都要支持 fallback
5. **可观测**：从第一天起就接日志/指标/追踪
6. **本地优先**：MVP 阶段单机可跑，避免过早分布式

> 决策记录：本项目最初考虑 Python FastAPI（AI 生态最强），但为支撑万级章节并发与低资源占用，最终选择 **Go（主）+ Node.js（前端）** 的组合；Rust 在 CPU 密集模块（如大规模文档排版）作为性能备选。

---

# 一、整体架构选型

## 1.1 三种候选

| 方案 | 优点 | 缺点 | 适合场景 |
|---|---|---|---|
| **单体（Go + Gin/Fiber）** | 单二进制部署简单、Goroutine 并发优、启动快、占用低 | AI 生态较 Python 弱（需 HTTP 自实现少数 SDK） | **MVP 推荐** |
| 单体（Python FastAPI） | AI 生态最丰富 | 单机并发上限、GIL/内存占用较高 | AI 强依赖且并发低的内部工具 |
| Go + Rust 双栈 | Go 主服务 + Rust 处理 CPU 密集模块 | 双语言运维、构建链路翻倍 | 后期规模化、有专门性能优化需求 |
| 微服务（多语言） | 各组件独立扩展 | 复杂度高、AI 生态分裂 | 后期规模化 |
| Serverless | 弹性好、按量付费 | 冷启动、AI 长任务不友好 | 突发场景 |

**推荐：单体 Go（Gin/Fiber）**，内置 Asynq/River 异步任务队列；前端统一 Node.js 生态（Web Vite SPA + 桌面 Electron）。

## 1.2 技术栈全景

```
┌──────────────────────────────────────────────────────────┐
│  客户端（Node.js 生态）                                  │
│  - Web: React 19 + TypeScript + Vite 7（Node.js 构建）  │
│  - Desktop: Electron 41（Node.js 主进程 + Chromium）     │
└──────────────────────────────────────────────────────────┘
                          ↓ HTTPS / WSS
┌──────────────────────────────────────────────────────────┐
│  服务端（Go 1.23+，单体）                                │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ API 层     │  │ 任务层     │  │ AI 编排层  │         │
│  │ Gin/Fiber  │  │ Asynq/River│  │ 自研路由   │         │
│  └────────────┘  └────────────┘  └────────────┘         │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐         │
│  │ 存储层     │  │ 图表层     │  │ 文档层     │         │
│  │ GORM/sqlx  │  │ Mermaid    │  │ unioffice  │         │
│  └────────────┘  └────────────┘  └────────────┘         │
└──────────────────────────────────────────────────────────┘
                          ↓
┌──────────────────────────────────────────────────────────┐
│  基础设施                                                │
│  - PostgreSQL / SQLite  - Redis（broker/cache/lock）     │
│  - S3 兼容对象存储        - Prometheus + Grafana         │
└──────────────────────────────────────────────────────────┘
```

> 注：Mermaid 渲染走 Node.js 侧的 `mermaid-cli`（mmdc，调用 puppeteer 渲染 SVG/PNG），与后端解耦；后端只负责把 Mermaid 源码或渲染产物入库/归档。

---

# 二、后端语言与框架

## 2.1 后端语言选型：Go vs Rust

| 维度 | Go 1.23+ | Rust 1.83+ | Python 3.12（备选） |
|---|---|---|---|
| **AI/LLM SDK** | ⭐⭐⭐⭐（Anthropic、OpenAI、Google、DeepSeek、硅基流动 等官方/社区 SDK 齐全） | ⭐⭐⭐（官方 SDK 偏少，部分需自实现 HTTP） | ⭐⭐⭐⭐⭐（生态最全） |
| **并发模型** | ⭐⭐⭐⭐⭐（goroutine，几行代码数万并发） | ⭐⭐⭐⭐⭐（tokio async，类型安全更强） | ⭐⭐⭐（asyncio + GIL 限制） |
| **性能（吞吐）** | ⭐⭐⭐⭐（I/O 并发优，单核 CPU 略弱） | ⭐⭐⭐⭐⭐（CPU 与内存均优） | ⭐⭐ |
| **性能（CPU 密集）** | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐ |
| **类型系统** | 静态 + 简洁 | 静态 + 严格（生命周期/所有权） | 动态 + type hints |
| **编译/构建速度** | ⭐⭐⭐⭐⭐（秒级） | ⭐⭐（分钟级） | 解释运行 |
| **部署** | ⭐⭐⭐⭐⭐（单二进制，可 go:embed） | ⭐⭐⭐⭐⭐（单二进制） | 需要运行时 + 依赖 |
| **招人难度** | 中 | 难 | 易 |
| **MVP 迭代速度** | 快 | 慢（编译反馈慢 + 心智负担） | 最快 |
| **生态成熟度** | web/queue/db/JSON 全面 | web 强，AI/文档/队列较薄 | AI/数据/文档最强 |

**推荐：Go 1.23+**

理由（按权重排序）：
1. **本项目瓶颈是 I/O**：大模型 API 调用延迟 1–30s/次，goroutine 几万并发足以撑满——不需要 Rust 那种 CPU 极致性能
2. **AI SDK 已够用**：Anthropic、OpenAI、Google 均有官方 Go SDK；DeepSeek（OpenAI 兼容）、硅基流动、智谱、月之暗面等国内供应商均提供 HTTP/SDK，缺失部分用 `net/http` + SSE 自实现成本可控
3. **任务队列生态成熟**：Asynq（基于 Redis）、River（基于 Postgres）均为生产级，比 Rust 的 Apalis/background-jobs 稳定得多
4. **构建/CI 反馈快**：Go 编译秒级，CI 跑完整测试 ≤ 2 分钟；Rust 编译分钟级，严重拖慢迭代节奏
5. **招人 + 工程化**：Go 工程师数量远超 Rust，单语言可减少运维/构建/CI 复杂度
6. **资源占用**：单二进制 ~30MB，内存 ~50–200MB/实例，比 Python 节省 3–5 倍

Rust 作为备选保留：若后期出现大规模文档/排版/PDF 计算的 CPU 密集瓶颈，可将该模块用 Rust 重写并通过 gRPC 暴露给 Go 主服务（Go + Rust 双栈）。

## 2.2 Web 框架

| 框架 | 优点 | 缺点 |
|---|---|---|
| **Gin** | 生态最大、中间件丰富、性能强、文档全 | API 风格略老 |
| **Fiber** | 受 Express 启发、API 现代、性能最强之一 | 生态略小于 Gin |
| Echo | 简洁、性能强、中间件机制清晰 | 生态小于 Gin |
| chi | 轻量、net/http 兼容 | 功能单薄 |
| net/http（1.22+ mux） | 零依赖、标准库 | 路由/中间件要自实现 |

**推荐：Gin（默认） / Fiber（追求极致吞吐时）**

理由：
- Gin 社区最大（绝大多数中间件/ORM 集成均默认支持 Gin），招人/招协作者成本最低
- 内置路由、中间件链、JSON 绑定、错误恢复——零样板启动
- 与 GORM / sqlx / Asynq 的官方示例齐全
- 若 QPS 突破 5w，可平滑迁移到 Fiber（API 接近 Express）

需要 OpenAPI 文档时搭配 [`swaggo/swag`](https://github.com/swaggo/swag) 或 [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen)（从 spec 生成代码）。

---

# 三、任务调度与并发

## 3.1 任务队列

| 选项 | 优点 | 缺点 |
|---|---|---|
| **Asynq（Redis）** | 生产级、调度/重试/优先级/UI 全、Go 生态最流行 | 依赖 Redis |
| **River（Postgres）** | 事务一致性最强、纯 Postgres 不需 Redis | 较新、生态小于 Asynq |
| Temporal（Go SDK） | 工作流引擎、状态机友好 | 学习曲线陡、运维重 |
| machinery（Redis/RabbitMQ） | 历史最久 | 社区维护放缓 |
| NATS JetStream | 轻量、内置消息 + KV | 任务模型较薄 |

**推荐：Asynq（默认，MVP）/ River（要求强事务时）**

理由：
- 章节任务有重试/超时/优先级/延迟需求，Asynq 原生支持
- 配套提供 `asynqmon`（Web UI）监控任务状态、失败重试、worker 负载
- broker 用 Redis 起步（同时充当缓存 + 分布式锁），与基础设施栈一致
- 若未来切 Postgres 为主，可平滑迁移到 River（API 类似）
- 替代 Celery 的角色：Asynq 的 Group/Chain API 对应 Celery 的 canvas（group/chord/chain）

## 3.2 任务模式实现

章节级并行 + 章节内串行：

```go
// 章节规划完成后
chapters := []*Chapter{ch1, ch2, ch3, ...} // 50 个章节

// 章节级并行：每章节入同一队列（默认 10 并发）
for _, ch := range chapters {
    payload, _ := json.Marshal(ChapterPayload{ChapterID: ch.ID})
    if err := asynqClient.Enqueue(
        asynq.NewTask(TypeChapterPipeline, payload),
        asynq.Queue("chapter-q"),
        asynq.MaxRetry(3),
        asynq.Timeout(30*time.Minute),
    ); err != nil {
        return err
    }
}

// 章节内串行：worker 内同步调用（也可拆成 4 个子任务用 chain）
func HandleChapterPipeline(ctx context.Context, t *asynq.Task) error {
    var p ChapterPayload
    if err := json.Unmarshal(t.Payload(), &p); err != nil {
        return err
    }
    // 章节内串行：检索 → 撰写 → 图表 → 内审
    materials, err := retrieveMaterials(ctx, p.ChapterID)          // 串行 1
    if err != nil { return err }
    content, err := writeChapter(ctx, p.ChapterID, materials)      // 串行 2
    if err != nil { return err }
    illustrations, err := generateIllustrations(ctx, p.ChapterID)  // 串行 3
    if err != nil { return err }
    return chapterAudit(ctx, p.ChapterID, content, illustrations)  // 串行 4
}

// 启动 worker（独立进程，可水平扩容）
func main() {
    srv := asynq.NewServer(
        asynq.RedisClientOpt{Addr: redisAddr},
        asynq.Config{
            Concurrency: 10,
            Queues: map[string]int{"export-q": 1, "planner-q": 5, "chapter-q": 10, "auditor-q": 3},
            StrictPriority: true,
        },
    )
    mux := asynq.NewServeMux()
    mux.HandleFunc(TypeChapterPipeline, HandleChapterPipeline)
    mux.HandleFunc(TypePlanner, HandlePlanner)
    mux.HandleFunc(TypeAuditor, HandleAuditor)
    mux.HandleFunc(TypeExport, HandleExport)
    if err := srv.Run(mux); err != nil { log.Fatal(err) }
}
```

## 3.3 任务组锁

Asynq 不直接提供任务组锁，可用以下方式实现：
- **Redis SETNX + EX**：`SET lock:chapter:<id> <worker-id> NX EX 300`，释放用 Lua 脚本 CAS
- **Postgres advisory lock**：`pg_try_advisory_lock(hashtext('chapter:' || $1))`
- **redislock 库**（[`bsm/redislock`](https://github.com/bsm/redislock)）：开箱即用的 Redis 分布式锁

**推荐：redislock（Redis）**（MVP）→ **Postgres advisory lock**（规模化或要求强一致时）

```go
import "github.com/bsm/redislock"

func withChapterLock(ctx context.Context, chapterID string, fn func() error) error {
    locker := redislock.New(client)
    lock, err := locker.Obtain(ctx, "lock:chapter:"+chapterID, 5*time.Minute, nil)
    if err != nil {
        return err // ErrNotObtained 表示被其他 worker 持有
    }
    defer lock.Release(ctx)
    return fn()
}
```

---

# 四、数据库与存储

## 4.1 元数据库

| 选项 | 优点 | 缺点 |
|---|---|---|
| **PostgreSQL** | JSONB、advisory lock、全文检索 | 部署稍重 |
| SQLite | 零运维、单文件 | 并发写弱 |
| MySQL | 成熟 | JSON 支持弱 |

**推荐：PostgreSQL 16+**

理由：
- JSONB 完美契合 chapter_specs / chapter_contents 的结构化字段
- advisory lock 实现任务组锁
- 全文检索（tsvector）可用于知识库章节检索
- LISTEN/NOTIFY 可用于章节状态实时推送

## 4.2 ORM / 数据访问

| 选项 | 优点 | 缺点 |
|---|---|---|
| **GORM v2** | 全功能 ORM、关联/迁移/Hooks 完善、文档好 | 性能中等、生成的 SQL 有时不直观 |
| **sqlx** | 轻量、原生 SQL、显式可控 | 无自动迁移 |
| **sqlc** | 从 SQL 生成类型安全代码、性能最佳 | 写 SQL，不适合动态查询 |
| **ent** | Facebook 出品、图模式、类型安全 | 学习曲线、复杂查询表达力一般 |
| **pgx**（纯 Postgres） | Postgres 特性最全、性能最佳 | 绑死 Postgres |

**推荐：GORM v2（默认，CRUD + 关联便利）/ sqlx（复杂查询性能场景）/ pgx（要求 Postgres 原生特性时）**

理由：
- GORM 模型定义清晰、自动迁移减少样板，适合 MVP 快速迭代
- 复杂章节规格查询用 `db.Raw` + sqlx 风格的 named 参数，避免 ORM 的 N+1
- 全文检索/JSONB 复杂查询直接用 pgx 写原生 SQL，性能更好

> 注意：Python 时代推荐的 SQLAlchemy 在 Go 中无对应物；GORM 是 Go 生态最接近的全功能 ORM，但哲学不同——Go 社区更推崇"显式优于魔法"，关键路径倾向于 sqlx/pgx 写裸 SQL。

## 4.3 文件存储

| 选项 | 场景 |
|---|---|
| **本地文件系统** | MVP 单机部署 |
| **S3 兼容**（MinIO / 阿里云 OSS / AWS S3） | 规模化、多副本 |
| 分布式文件系统（cephfs / juicefs） | 大规模、跨节点 |

**目录结构**：
```
storage/
 ├── chapters/{chapter_id}/content_v{n}.md
 ├── illustrations/{illustration_id}/source.{ext}
 ├── illustrations/{illustration_id}/rendered.{png|svg}
 ├── tenders/{tender_id}/rfp.pdf
 └── evidence/{evidence_id}/source.{ext}
```

**推荐：MVP 用本地文件系统（用 `afero` 抽象）** → S3 用 [`aws-sdk-go-v2`](https://github.com/aws/aws-sdk-go-v2) service/s3 或 [`minio-go`](https://github.com/minio/minio-go)（直接对接 MinIO/兼容 S3）

```go
// 抽象接口，便于本地 ↔ S3 切换
type Storage interface {
    Put(ctx context.Context, key string, r io.Reader) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Presign(ctx context.Context, key string, ttl time.Duration) (string, error)
}
```

---

# 五、AI / LLM 选型

## 5.1 主力模型

| 模型 | 上下文 | Prompt 缓存 | 中文 | 成本（输入/输出 $/M token） |
|---|---|---|---|---|
| **Claude Sonnet 4.6** | 200K | ✅ cache_control | 优 | 3 / 15 |
| GPT-4o | 128K | ✅ 自动 | 良 | 2.5 / 10 |
| GPT-4o mini | 128K | ✅ 自动 | 良 | 0.15 / 0.6 |
| DeepSeek V3 | 64K | ✅ | 优 | 0.27 / 1.1 |
| Qwen Max | 128K | ✅ | 优 | 0.34 / 1.4 |

**推荐主力：Claude Sonnet 4.6**

理由：
- 200K 上下文可容纳章节规格 + 全部章节素材 + global_facts
- `cache_control` 主动控制缓存前缀，命中率高，**可降本 5-10 倍**
- 写作质量稳定，结构化输出（JSON）容错好
- Tool use 能力强

**推荐备选**：
- 成本敏感场景：DeepSeek V3（中文场景强，价格 1/10）
- 多模态/简单任务：GPT-4o mini（速度最快）

## 5.2 双层模型策略

```
粗稿/规划阶段：便宜模型（DeepSeek V3 / GPT-4o mini）
  → 章节规格、章节大纲、章节素材筛选

精稿/写作阶段：主力模型（Claude Sonnet 4.6）
  → 章节正文、图表描述、响应矩阵

校对阶段：主力模型 + agent 模式
  → 跨章节一致性审计
```

## 5.3 Prompt 缓存实现

Anthropic 的 `cache_control` 关键点（Go SDK `github.com/anthropics/anthropic-sdk-go`）：

```go
import (
    "context"
    "github.com/anthropics/anthropic-sdk-go"
)

func WriteChapter(ctx context.Context, client *anthropic.Client, sys, spec string, userContent string) (*anthropic.Message, error) {
    resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
        Model: anthropic.ModelClaudeSonnet4_6,
        MaxTokens: 8192,
        System: []anthropic.TextBlockParam{
            {Type: "text", Text: sys},            // global_facts + 术语表（强缓存）
            // 注意：cache_control 标记只放在需要缓存的最末段
        },
        Messages: []anthropic.MessageParam{
            {
                Role: anthropic.MessageRoleUser,
                Content: []anthropic.ContentBlockParamUnion{
                    anthropic.NewTextBlock(spec + "\n\n" + userContent),
                },
            },
        },
    })
    return resp, err
}
```

缓存位置策略：
1. `system` 段 1：global_facts + 术语表（所有章节共享）→ **强缓存**
2. `system` 段 2：章节规格（本章节唯一）→ 章节内复用，配合 `CacheControl: {Type: "ephemeral"}`
3. `messages`：上一轮对话（同章节多轮扩写时复用前缀）

## 5.4 AI 调用抽象

```go
type ChatMessage struct {
    Role    string // "user" | "assistant" | "system"
    Content string
}

type CacheControl struct {
    Type string // "ephemeral"
}

type ChatRequest struct {
    Model        string
    Messages     []ChatMessage
    System       []SystemBlock // 可携带 cache_control
    MaxTokens    int
    Temperature  float64
    ExtraHeaders map[string]string
}

type ChatResponse struct {
    Content      string
    Usage        Usage // prompt/cache_read/cache_write tokens
    StopReason   string
    RawProvider  string // "anthropic" | "openai" | "deepseek" | ...
}

type LLMProvider interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    Name() string
}

type AnthropicProvider struct{ /* ... */ }
type OpenAIProvider    struct{ /* ... */ }
type DeepSeekProvider  struct{ /* ... */ } // OpenAI 兼容 API

// Router：按 task 名选择 provider，支持 fallback
type LLMRouter struct {
    providers map[string][]LLMProvider // task_name -> ordered list
}

func (r *LLMRouter) Route(ctx context.Context, taskName string, req ChatRequest) (*ChatResponse, error) {
    for _, p := range r.providers[taskName] {
        resp, err := p.Chat(ctx, req)
        if err == nil { return resp, nil }
        if !isRetryable(err) { return nil, err }
        // 继续下一个 provider
    }
    return nil, ErrAllProvidersFailed
}
```

## 5.5 JSON 修复链路

```go
func ParseJSONResponse[T any](ctx context.Context, content string, retry func(ctx context.Context, err error) (string, error)) (T, error) {
    var zero T

    // 1. 直接解析
    var v T
    if err := json.Unmarshal([]byte(content), &v); err == nil {
        return v, nil
    }

    // 2. 抽取第一个 {...} 块
    extracted := extractFirstJSONBlock(content)
    if err := json.Unmarshal([]byte(extracted), &v); err == nil {
        return v, nil
    }

    // 3. 局部修补（不重发）
    if repaired, ok := repairJSON(extracted); ok {
        if err := json.Unmarshal([]byte(repaired), &v); err == nil {
            return v, nil
        }
    }

    // 4. 重发一次（仅发送错误信息 + 原内容，不发整个 prompt）
    if retry != nil {
        if repaired, err := retry(ctx, errors.New("json parse failed: "+content)); err == nil {
            if err := json.Unmarshal([]byte(repaired), &v); err == nil {
                return v, nil
            }
        }
    }
    return zero, errors.New("json repair exhausted")
}
```

---

# 六、图表生成选型

## 6.1 Mermaid（流程/架构/时序图）

| 渲染方式 | 优点 | 缺点 |
|---|---|---|
| **mermaid.ink（在线 API）** | 零部署 | 依赖外网、有频率限制 |
| **mermaid-cli（本地）** | 无外网依赖 | 需要 Node.js + puppeteer |
| **kroki（统一 API）** | 多图表统一 | 部署重 |

**推荐：mermaid.ink 主用 + mermaid-cli 本地 fallback**

实现：

```go
// RenderMermaid 渲染主流程：mermaid.ink → 本地 mmdc → 占位图
func RenderMermaid(ctx context.Context, source string) ([]byte, error) {
    // 1. 校验语法
    if err := validateMermaidSyntax(source); err != nil {
        return nil, err
    }

    // 2. 主用：mermaid.ink
    if img, err := fetchMermaidInk(ctx, source, 15*time.Second); err == nil {
        return img, nil
    }

    // 3. 降级：本地 mmdc（需要 Node.js 子进程）
    if img, err := renderMermaidLocal(ctx, source, 30*time.Second); err == nil {
        return img, nil
    }

    // 4. 占位图（永不失败）
    return generatePlaceholderImage("Mermaid 渲染失败"), nil
}

func renderMermaidLocal(ctx context.Context, source string, timeout time.Duration) ([]byte, error) {
    cmd := exec.CommandContext(ctx, "mmdc",
        "-i", "-", "-o", "-", "-e", "png", "-q",
    )
    cmd.Stdin = strings.NewReader(source)
    return cmd.Output() // 走 PATH；mmdc 由前端 Node.js 工具链安装
}
```

## 6.2 AI 图（示意图/封面图/组织架构图）

| 提供商 | 强项 | 成本 |
|---|---|---|
| **DALL-E 3**（OpenAI） | 通用、风格可控 | $0.04/张 |
| **Stable Diffusion 3** | 可本地部署 | GPU 成本 |
| **Midjourney** | 质量最高 | $0.05+/张 |
| **Replicate** | 多模型聚合 | 按模型定价 |
| 国产（通义万相 / 文心一格） | 中文友好 | ¥0.1-0.3/张 |

**推荐：DALL-E 3 主用 + Replicate（备选）**

理由：
- 同一份标书需要风格统一，DALL-E 3 风格可控性最强
- 失败时降级到 Replicate 上的其他模型
- 国产模型作为中文场景补充

### 6.2.1 DALL-E 3 vs FLUX / Stable Diffusion 深度对比（需求 §3.7.6 + D3）

> 需求 §3.7.6 要求"文生图补充方案"，依赖 D3 列出 FLUX / SD。本节对三类模型做系统对比，给出选型矩阵。

| 维度 | DALL-E 3 | Stable Diffusion 3 / 3.5 | FLUX.1 [pro/dev/schnell] |
|---|---|---|---|
| **提供方** | OpenAI（API） | Stability AI / 社区 | Black Forest Labs |
| **接入方式** | 云端 API | 本地 / Replicate | 云端 API 或本地 |
| **质量** | 高（写实风） | 中-高（写实偏弱） | 极高（写实+艺术） |
| **风格可控性** | ★★★★ | ★★★ | ★★★★★ |
| **中文支持** | 良好（prompt 英文效果更好） | 取决于底模 | 良好 |
| **分辨率** | 1024×1024 / 1792×1024 | 可自定义（512-2048） | 可自定义（1024+） |
| **生成速度** | 5-10s | 10-30s（GPU） | 10-20s（GPU） |
| **成本** | $0.04/张 | GPU 电费（≈$0.005/张） | API $0.05/张 或本地 |
| **私有化** | ❌ 仅 API | ✅ 本地部署 | ✅ 本地部署 |
| **数据合规** | 数据回流 OpenAI | 本地无回流 | 本地无回流 |
| **风格模板** | 通过 prompt 实现 | LoRA / ControlNet | LoRA / ControlNet |
| **版权风险** | 商业可用（OpenAI 条款） | 模型许可证（SD3.5 商业可用） | 商业可用 |

**选型矩阵（按场景）**：

| 场景 | 推荐 | 理由 |
|---|---|---|
| **SaaS 模式 + 高质量** | DALL-E 3 主、FLUX API 备 | 质量稳定、API 简单、降级有 |
| **SaaS 模式 + 成本敏感** | SD 3.5 + Replicate | $0.005/张，成本降 8x |
| **私有化模式** | SD 3.5 本地 + FLUX 本地 | 无外网，必须本地推理 |
| **中文场景** | 通义万相 + Qwen 文生图 | 中文 prompt 友好 |
| **风格统一** | SD 3.5 + 训练 LoRA | LoRA 锁定企业风格 |
| **封面图** | FLUX.1 pro | 写实 + 艺术质量最高 |
| **示意图（架构 / 组织）** | Mermaid 优先 | 矢量、可编辑、零成本 |

**推荐最终方案**：

| 部署模式 | 主力 | 备选 | 兜底 |
|---|---|---|---|
| **SaaS 模式** | DALL-E 3（云端 API） | FLUX.1 [schnell]（Replicate） | 简化 prompt 重试 → 占位图 |
| **私有化模式** | SD 3.5（本地 + LoRA） | FLUX.1 [dev]（本地） | 简化 prompt → 占位图 |

**LoRA 风格锁定**（私有化推荐做法，SD 训练脚本，与后端语言无关——私有化模式下独立 Python 训练任务，产出 `safetensors` 供 Go 主服务调用推理）：

```python
# 训练企业专属 LoRA（diffusers + PEFT，仅私有化模式运行）
lora_config = {
    "base_model": "stabilityai/stable-diffusion-3.5-large",
    "lora_rank": 32,
    "training_data": "企业历史标书封面 200 张 + 行业模板 100 张",
    "training_steps": 5000,
    "output": "models/lora/enterprise_style_v1.safetensors",
}

# 推理时挂载（私有化模式由 Python 推理服务挂载，Go 主服务仅发起 HTTP 请求）
pipe.load_lora_weights("models/lora/enterprise_style_v1.safetensors")
image = pipe(prompt=prompt, lora_scale=0.8).images[0]
```

**风格一致性保证**（跨标书）：
- SaaS：DALL-E 3 用统一 style prefix（"professional Chinese government bid cover, blue corporate style, vector illustration style"）
- 私有化：LoRA 锁定 + 固定 seed + 负向 prompt（"low quality, watermark, text"）

**性能与成本**（单标书约 30 张 AI 图）：

| 方案 | 单图成本 | 单标书总成本 | 单图延迟 |
|---|---|---|---|
| DALL-E 3 | $0.04 | $1.2 | 5-10s |
| SD 3.5（本地） | GPU 电费 $0.005 | $0.15 | 10-20s |
| FLUX schnell（API） | $0.05 | $1.5 | 8-12s |

---

## 6.3 表格 / 响应矩阵

直接生成 HTML / JSON，自实现 HTML → docx 表格，无需第三方。

## 6.4 数据图表

| 选项 | 优点 | 缺点 |
|---|---|---|
| **go-echarts** | 类型安全、绑定 Apache ECharts、样式丰富 | 输出 HTML，需前端渲染或 wkhtmltopdf |
| **gonum/plot** | 纯 Go、矢量输出 (PDF/SVG/PNG) | 样式较朴素 |
| **go-chart** | 简单易用 | 功能较少 |
| **matplotlib**（Python 微服务） | 样式成熟、生态最广 | 需独立 Python 进程（破坏纯 Go 栈） |
| plotly | 交互式、样式现代 | 输出体积大 |

**推荐：go-echarts（默认，绑定 ECharts 输出 HTML/PNG） / gonum/plot（需要纯 Go 矢量图时）**

> 若团队已有 matplotlib 资产且强需求，可启动一个 Python 微服务专门做图表（gRPC 接口），主服务仍是 Go——这属于 §2.1 提到的"Go + Rust/Python 双栈"特例。

数据来源走 evidence chain，无数据不生成。

## 6.5 图表占位符协议（章节正文 → 图表解耦）

章节正文采用**结构化占位符**嵌入图表，使正文撰写与图表渲染解耦：

```
正文 Markdown 中嵌入：

   本系统采用分层架构，如 [!figure:arch-overview type=mermaid caption=系统分层架构图] 所示。

渲染器：
   1. 扫描 `\[!figure:(<id>)\s+([^\]]*)\]` 模式
   2. 抽取 id / attributes（type, caption, data, ...）
   3. 按 id 在 illustrations 表查渲染产物
   4. 替换为 docx 图片 + 编号 + 图注
```

占位符的好处：
- 撰写阶段只关心"哪里需要图、要什么图"，不必关心怎么渲染
- 图表生成可异步、可重试、可换实现
- 替换阶段才绑定具体渲染产物（PNG/SVG）
- 同一个 placeholder 在不同输出格式下可走不同渲染路径

## 6.6 章节内图表流水线

```
章节内容（Markdown）
   │
   ↓ 正则扫描
[FigureSpec(id, type, caption, attrs), ...]
   │
   ↓ 串行生成（每张独立任务）
   ├──→ 查 illustrations 表（命中即跳过）
   ├──→ 数据准备（基于 evidence chain）
   ├──→ 调用对应 renderer（mermaid / go-echarts / DALL-E / 自实现表格）
   ├──→ 校验（视觉 + 语义）
   ├──→ 失败 → fallback 链
   └──→ 成功 → 写库 + 落 S3
   │
   ↓
返回 Illustration 列表（含 source_path / rendered_path / status）
```

**章节内图表必须串行**（与第六章 Prompt 缓存策略一致）：
- 章节正文 → 章节正文+图表占位 → 串行渲染
- 因为占位符按出现顺序绑定章节号，串行确保编号稳定
- 图表独立可并发（MVP 用串行简化调试与日志）

## 6.7 失败回退链（按图表类型）

| 图表类型 | 主路径 | 降级 1 | 降级 2 | 最终兜底 |
|---|---|---|---|---|
| Mermaid | mermaid.ink | mermaid-cli 本地 | 语法修正重试 | 占位图 + 文字描述 |
| AI 图 | DALL-E 3 | Replicate SDXL | 国产模型 | 简化 prompt 重试 → 占位图 |
| 数据图 | go-echarts | plotly（PNG 导出） | 表格替代 | 占位图 + 数据表 |
| 表格 | 自实现 HTML→docx | pandoc | 纯文本对齐 | 强制输出 |

所有失败记录到 `illustrations.status='failed'` 和 `audit.illustration_issues`，由人在回路点 2 统一处理。

---

# 七、文档导出选型

## 7.1 Word（.docx）—— 主输出格式

| 库 | 优点 | 缺点 |
|---|---|---|
| **unidoc/unioffice** | Go 原生、API 覆盖度好、支持模板/表格/图表 | 部分高级样式 API 较底层 |
| docx-template（`gotemplate`） | 类 docxtpl 思路、模板填充 | 复杂逻辑不便 |
| pandoc（子进程） | Markdown → docx 强 | 样式定制弱 |
| LibreOffice headless | 完美兼容 Word | 重、启动慢 |
| go-docx（[`JohnReedLOL/go-docx`](https://github.com/JohnReedLOL/go-docx)） | 简单文档够用 | 功能较薄 |

**推荐：unidoc/unioffice（主） + pandoc 子进程（降级）**

理由：
- 标书样式复杂（页眉页脚、目录、序号、图表编号）——`unidoc/unioffice` 的 low-level XML 控制力是 Go 生态中唯一可用的
- 走 Markdown → AST → docx，绕过 headless browser
- pandoc 作为降级：处理简单 Markdown 转换场景，作为兜底
- 若需"模板套用"能力，可自行实现模板占位符解析（`{{var}}` / `{{loop}}`），参考 `unioffice` 的文档/段落/表格 API

> 注意：Go 生态的 docx 库远不如 Python `python-docx` 成熟，部分高级排版需求（复杂表格合并、页眉多列、嵌套样式）需要直接操作底层 XML；这是选择 Go 的代价，但 unioffice 已覆盖 80% 场景。

**为什么 Word 是主输出（详见 high-level-design.md §6）：**
- 标书交付物 99% 是 docx（甲方要求、可批注、可修订、可签章）
- Word 模板是企业知识资产，复用率高
- docx 是审计、修改、合并的事实标准
- PDF 是 docx 的衍生品（投标准备/打印场景）

## 7.2 PDF —— 衍生输出

| 选项 | 优点 | 缺点 |
|---|---|---|
| **LibreOffice headless** | docx → pdf 一致性最好 | 重 |
| weasyprint | HTML → pdf 灵活 | 与 docx 样式不同 |
| pdfkit（wkhtmltopdf） | 成熟 | 维护停滞 |

**推荐：LibreOffice headless**

理由：docx → pdf 样式一致性最关键（标书格式不能变），其他方案都会有偏差。

PDF 生成在 Word 输出之后异步触发（避免阻塞主流程），并写 PDF 到 S3。

## 7.3 Mermaid → docx 内嵌

```go
import (
    "github.com/unidoc/unioffice/document"
    "github.com/unidoc/unioffice/measurement"
)

func embedMermaidInDocx(doc *document.Document, ill *Illustration) error {
    renderedPath := ill.RenderedPath // PNG/SVG（绝对路径或 io.Reader）

    // 1. 插入图片（宽 14cm）
    img, err := doc.AddImage(renderedPath)
    if err != nil { return err }
    para := doc.AddParagraph()
    run := para.AddRun()
    run.AddImage(img, measurement.Centimeter*14)

    // 2. 居中 + 图注（图 3-2 标题）
    para.Properties().SetAlignment(wml.ST_JcCenter)
    caption := doc.AddParagraph()
    caption.Properties().SetAlignment(wml.ST_JcCenter)
    caption.AddRun().AddText(fmt.Sprintf("图 %s-%d %s", ill.ChapterNumber, ill.Order, ill.Title))

    return nil
}
```

---

# 八、知识库与检索

## 8.1 检索方案

framework.md 提出"你不需要 RAG"哲学，技术实现：

| 方案 | 适用 |
|---|---|
| **目录树 + 关键词**（PostgreSQL 全文检索） | MVP 推荐 |
| BM25 + 关键词 | 简单场景 |
| 向量检索（pgvector / Qdrant） | 复杂语义检索 |
| 混合检索（关键词 + 向量） | 规模化 |

**推荐：MVP 用 PostgreSQL tsvector 全文检索；后续加 pgvector 做混合检索**

理由：标书场景召回率 > 相似度，目录树 + 关键词足够。

### 8.1.1 双向语义索引（需求 §4.9-② + HLD §5.10）

> 需求 §4.9-② 要求"文-图双向语义索引"——通过文本检索图表、通过图表反查相关正文段落。MVP 用 PostgreSQL tsvector 是不够的，必须升级到混合检索。

**向量库选型**：

| 方案 | 性能 | 运维 | 适用 |
|---|---|---|---|
| **pgvector** | 百万级 OK | 零运维（复用 PG） | **MVP 推荐** |
| Qdrant | 亿级 | 独立服务 | 规模化 |
| Milvus | 亿级 | 独立集群 | 超大规模 |
| Weaviate | 千万级 | 中等 | 通用场景 |
| Chroma | 百万级 | 轻量 | 本地原型 |

**推荐：pgvector（MVP）+ Qdrant（规模化）**

理由：
- 标书知识库单企业规模 10K-100K 文档，pgvector 完全够用
- 复用 PostgreSQL 主备/备份/快照，省一套独立服务
- 迁移路径平滑：pgvector → Qdrant 仅需换客户端，schema 几乎不变

**Embedding 模型**：

| 模型 | 维度 | 性能 | 适用 |
|---|---|---|---|
| **BGE-large-zh-v1.5** | 1024 | 中文 SOTA | **推荐（中文场景）** |
| BGE-M3 | 1024 | 多语言 + 长文本（8K） | 长文档 |
| M3E-large | 1024 | 中文 | 备选 |
| text-embedding-3-large（OpenAI） | 3072 | 通用 | 私有化不可用 |
| Qwen3-Embedding | 1024+ | 中文 + 多语言 | 备选（国产） |

**私有化部署**：BGE-large-zh-v1.5 + vLLM 推理，1× GPU 16G 即可。

**双向索引的数据模型**：

```sql
-- 1. 文本侧指纹（每段正文 + 章节标题）
CREATE TABLE text_chunks (
    id BIGSERIAL PRIMARY KEY,
    doc_id BIGINT NOT NULL,
    doc_type VARCHAR(32),          -- 'chapter' | 'requirement' | 'evidence' | 'scoring_item'
    chapter_id VARCHAR(64),
    content TEXT NOT NULL,
    content_tsv TSVECTOR,          -- tsvector（全文索引）
    content_vec VECTOR(1024),      -- pgvector（语义索引）
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_text_tsv ON text_chunks USING GIN(content_tsv);
CREATE INDEX idx_text_vec ON text_chunks USING ivfflat(content_vec vector_cosine_ops) WITH (lists = 100);

-- 2. 图表侧指纹（图题 + 描述 + caption）
CREATE TABLE figure_chunks (
    id BIGSERIAL PRIMARY KEY,
    figure_id BIGINT NOT NULL,
    chapter_id VARCHAR(64),
    caption TEXT,                  -- 图表标题
    description TEXT,              -- 图表描述/正文片段
    caption_tsv TSVECTOR,
    caption_vec VECTOR(1024),
    image_hash VARCHAR(64),        -- 视觉指纹（pHash）
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_figure_tsv ON figure_chunks USING GIN(caption_tsv);
CREATE INDEX idx_figure_vec ON figure_chunks USING ivfflat(caption_vec vector_cosine_ops) WITH (lists = 100);

-- 3. 正向关系（章节 → 图表）+ 反向关系（图表 → 章节）
CREATE TABLE chunk_figure_links (
    text_chunk_id BIGINT REFERENCES text_chunks(id),
    figure_chunk_id BIGINT REFERENCES figure_chunks(id),
    link_type VARCHAR(32),         -- 'contains' | 'describes' | 'evidence_for' | 'derived_from'
    similarity_score FLOAT,        -- 双向索引的相似度
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (text_chunk_id, figure_chunk_id)
);
```

**混合检索实现**：

```go
type SearchResult struct {
    ID      int64
    Content string
    Score   float64
}

func HybridSearch(ctx context.Context, db *pgxpool.Pool, query string, topK int) ([]SearchResult, error) {
    // 1. 关键词检索（tsvector）
    keywordRows, err := db.Query(ctx, `
        SELECT id, content, ts_rank_cd(content_tsv, query) AS score
        FROM text_chunks, plainto_tsquery('simple', $1) query
        WHERE content_tsv @@ query
        ORDER BY score DESC LIMIT $2 * 3
    `, query, topK)
    if err != nil { return nil, err }
    defer keywordRows.Close()

    // 2. 语义检索（pgvector）
    queryVec, err := embed(ctx, query) // BGE-large-zh
    if err != nil { return nil, err }
    semanticRows, err := db.Query(ctx, `
        SELECT id, content, 1 - (content_vec <=> $1) AS score
        FROM text_chunks
        ORDER BY content_vec <=> $1
        LIMIT $2 * 3
    `, queryVec, topK)
    if err != nil { return nil, err }
    defer semanticRows.Close()

    // 3. 倒数排名融合（RRF）
    return reciprocalRankFusion(keywordRows, semanticRows, topK), nil
}
```

**双向查询示例**：

```go
// 查询 1: 文本 → 相关图表（图题检索）
func FindRelatedFigures(ctx context.Context, db *pgxpool.Pool, textChunkID int64, topK int) ([]FigureChunk, error) {
    return db.Query(ctx, `
        SELECT f.id, f.caption, l.similarity_score
        FROM chunk_figure_links l
        JOIN figure_chunks f ON l.figure_chunk_id = f.id
        WHERE l.text_chunk_id = $1
        ORDER BY l.similarity_score DESC
        LIMIT $2
    `, textChunkID, topK)
}

// 查询 2: 图表 → 相关正文（图反查文）
func FindRelatedText(ctx context.Context, db *pgxpool.Pool, figureID int64, topK int) ([]TextChunk, error) {
    return db.Query(ctx, `
        SELECT t.id, t.content, l.similarity_score
        FROM chunk_figure_links l
        JOIN text_chunks t ON l.text_chunk_id = t.id
        WHERE l.figure_chunk_id = $1
        ORDER BY l.similarity_score DESC
        LIMIT $2
    `, figureID, topK)
}

// 查询 3: 跨模态（用图查图、用文查文）
func CrossModalSearch(ctx context.Context, db *pgxpool.Pool, figureID int64, topK int) ([]FigureChunk, error) {
    return db.Query(ctx, `
        WITH anchor AS (SELECT caption_vec FROM figure_chunks WHERE id = $1)
        SELECT id, caption, 1 - (caption_vec <=> (SELECT caption_vec FROM anchor)) AS score
        FROM figure_chunks
        WHERE id != $1
        ORDER BY caption_vec <=> (SELECT caption_vec FROM anchor)
        LIMIT $2
    `, figureID, topK)
}
```

**索引构建流程**：

```go
func IndexChapter(ctx context.Context, db *pgxpool.Pool, chapterID, content string) error {
    chunks := splitIntoChunks(content, 512, 64) // size=512, overlap=64
    for _, chunk := range chunks {
        vec, err := embed(ctx, chunk.Text)
        if err != nil { return err }

        // 1. 写入正文指纹
        var textChunkID int64
        err = db.QueryRow(ctx, `
            INSERT INTO text_chunks (doc_id, doc_type, chapter_id, content, content_tsv, content_vec)
            VALUES ($1, 'chapter', $2, $3, to_tsvector('simple', $3), $4)
            RETURNING id
        `, chapterID, chapterID, chunk.Text, vec).Scan(&textChunkID)
        if err != nil { return err }

        // 2. 解析图表占位符
        figures := extractFigurePlaceholders(chunk.Text)
        for _, fig := range figures {
            figVec, _ := embed(ctx, fig.Caption)
            figID, err := upsertFigure(ctx, db, fig)
            if err != nil { return err }

            // 写入图表指纹
            _, err = db.Exec(ctx, `
                INSERT INTO figure_chunks (figure_id, chapter_id, caption, description, caption_tsv, caption_vec)
                VALUES ($1, $2, $3, $4, to_tsvector('simple', $3), $5)
                ON CONFLICT (figure_id) DO UPDATE SET caption_vec = EXCLUDED.caption_vec
            `, figID, chapterID, fig.Caption, chunk.Text, figVec)
            if err != nil { return err }

            // 3. 关联
            _, err = db.Exec(ctx, `
                INSERT INTO chunk_figure_links (text_chunk_id, figure_chunk_id, link_type, similarity_score)
                VALUES ($1, $2, 'describes', $3)
                ON CONFLICT DO NOTHING
            `, textChunkID, figID, cosineSimilarity(vec, figVec))
            if err != nil { return err }
        }
    }
    return nil
}
```

**性能指标**（10 万条文本 + 5 万张图）：

| 操作 | 延迟 | 备注 |
|---|---|---|
| 单次混合检索 | < 100ms | 关键词 + 向量并行 |
| 文本→图表查询 | < 50ms | 索引命中 |
| 图表→文本查询 | < 50ms | 索引命中 |
| 跨模态查询 | < 200ms | 纯向量 |
| 入库（每 chunk） | < 80ms | 含 embedding |

**成本**（私有化部署）：
- Embedding 推理：1× A100 16G，500 chunks/秒
- pgvector 内存：100K × 1024 × 4B = 400MB
- 索引构建：首次全量 ~30 分钟，增量 < 5 分钟

## 8.2 文档预处理

```go
type Document struct {
    Path     string
    Content  string
    Chunks   []Chunk
    Metadata map[string]string
}

// 文档统一走 doc2markdown 中间态
func IngestDocument(ctx context.Context, path string) (*Document, error) {
    var content string
    var err error
    switch {
    case strings.HasSuffix(path, ".pdf"):
        content, err = pdfToMarkdown(ctx, path)
    case strings.HasSuffix(path, ".docx"):
        content, err = docxToMarkdown(ctx, path)
    default:
        content, err = readFile(path)
    }
    if err != nil { return nil, err }

    return &Document{
        Path:     path,
        Content:  content,
        Chunks:   splitIntoChunks(content, 512, 64),
        Metadata: extractMetadata(path),
    }, nil
}
```

## 8.3 行业适配策略（需求 §4.9-④ + HLD §5.12）

> 需求 §4.9-④ 要求"行业适配深度（聚焦 2-3 个行业）"。MVP 阶段我们聚焦以下 3 个行业，提供差异化模板与领域词典。

### 8.3.1 聚焦行业

**行业优先级**（按招标频次 + 单项目金额）：

| 行业 | 招标频次 | 平均金额 | 模板成熟度 | MVP 优先级 |
|---|---|---|---|---|
| **信息技术（软件开发 / 集成）** | 极高 | 中-高 | 高 | **P0** |
| **建筑工程（房建 / 市政）** | 极高 | 高 | 中 | **P0** |
| **政府采购（货物 / 服务）** | 高 | 中 | 高 | **P1** |
| **能源（电力 / 化工）** | 中 | 高 | 低 | P2 |
| **医疗（设备 / 信息化）** | 中 | 中 | 中 | P2 |
| **金融（系统 / 数据）** | 中 | 高 | 低 | P2 |

**MVP 重点**：信息技术 + 建筑工程 + 政府采购

### 8.3.2 行业模板

每个行业提供独立的：
- 章节模板（章节类型 + 推荐顺序）
- 评分项映射规则
- 资质门槛清单
- 常见图表类型
- 行业术语词典

```yaml
# config/industry/it.yaml
industry: it
display_name: "信息技术"
chapter_templates:
  - type: "技术方案"
    sections: ["系统架构", "技术选型", "接口设计", "性能指标", "安全方案", "部署方案"]
    weight: 0.40          # 占总分 40%
  - type: "实施方案"
    sections: ["项目组织", "实施计划", "培训方案", "运维方案"]
    weight: 0.20
  - type: "商务方案"
    sections: ["公司简介", "资质证书", "类似业绩", "报价说明"]
    weight: 0.30
  - type: "售后方案"
    sections: ["质保期", "响应时间", "服务团队"]
    weight: 0.10
qualifications:
  - "ISO 9001 质量管理体系认证"
  - "CMMI 3 级（或以上）认证"
  - "软件企业认定证书"
  - "近 3 年同类项目业绩（≥3 个）"
common_figures:
  - "系统架构图"
  - "网络拓扑图"
  - "数据流程图"
  - "ER 图"
  - "接口时序图"
```

### 8.3.3 领域词典

**作用**：让 Embedding 和关键词检索更精准，理解行业黑话。

```yaml
# config/industry/it_terms.yaml
synonyms:
  - "中间件" -> ["消息队列", "MQ", "Kafka", "RabbitMQ", "RocketMQ"]
  - "数据库" -> ["MySQL", "PostgreSQL", "Oracle", "达梦", "人大金仓"]
  - "云服务" -> ["阿里云", "华为云", "腾讯云", "AWS", "Azure"]
  - "等保" -> ["等级保护", "等保2.0", "等保三级", "等保2.0三级"]
  - "信创" -> ["国产化", "自主可控", "国产芯片", "麒麟", "统信"]
  - "CMMI" -> ["CMMI 3", "CMMI 4", "CMMI 5", "能力成熟度"]
  - "DevOps" -> ["CI/CD", "持续集成", "持续部署", "Jenkins", "GitLab CI"]

stopwords:
  - "本项目"
  - "本公司"
  - "投标人"

entity_types:
  PRODUCT: ["系统", "平台", "软件", "中间件", "数据库"]
  STANDARD: ["等保三级", "CMMI 3", "ISO 9001", "ISO 27001"]
  CERT: ["软件著作权", "专利", "软件企业认定", "高新技术企业"]
```

### 8.3.4 行业适配的实施路径

**MVP 阶段**（M0-M3）：
1. 落地信息技术 + 建筑工程 + 政府采购 3 个行业的 YAML 模板
2. 准备 3 套行业词典（BGE embedding 微调或拼接到 prompt）
3. 准备 3 套行业评分项映射规则
4. 准备 100+ 个行业模板标书样本（用于评测）

**v1.0 阶段**（M3-M6）：
1. 加入能源 / 医疗 2 个行业
2. 行业词典自动学习（从历史标书抽取）
3. 行业级微调 LLM（针对 Top 2 行业）

**v2.0 阶段**（M6+）：
1. 行业知识图谱（招标方-行业-业绩-人员多维关系）
2. 行业级 LoRA（每个行业一个 LoRA）
3. 客户自定义行业（上传历史标书自动学习）

### 8.3.5 行业识别

**自动识别流程**：

```go
type IndustryTag string

func DetectIndustry(ctx context.Context, parsed *ParsedRFP, llm LLMRouter) (IndustryTag, error) {
    // 1. 关键词匹配
    scores := map[IndustryTag]int{}
    for ind, cfg := range IndustryConfigs {
        s := 0
        for _, kw := range cfg.Keywords {
            if strings.Contains(parsed.RawText, kw) {
                s++
            }
        }
        scores[ind] = s
    }

    // 取 top
    top, topScore := topIndustry(scores)

    // 2. 关键词不足 → LLM 二次确认
    if topScore < 3 {
        industry, err := llmClassifyIndustry(ctx, llm, parsed)
        if err != nil { return "", err }
        return industry, nil
    }
    return top, nil
}

func llmClassifyIndustry(ctx context.Context, llm LLMRouter, parsed *ParsedRFP) (IndustryTag, error) {
    resp, err := llm.Route(ctx, "industry-classify", ChatRequest{
        Model: "deepseek-v3",
        System: []SystemBlock{{Text: "你是行业分类专家，只输出行业标签之一：it/construction/government/energy/medical/other"}},
        Messages: []ChatMessage{{Role: "user", Content: parsed.RawText[:min(len(parsed.RawText), 4000)]}},
        MaxTokens: 16,
        Temperature: 0,
    })
    if err != nil { return "", err }
    return IndustryTag(strings.TrimSpace(resp.Content)), nil
}
```

**用户可手动覆盖**：解析完成后 UI 展示识别结果，允许切换。

---

# 九、客户端（前端：Node.js 生态）

> **前端统一基于 Node.js 运行时**：构建工具（Vite 7、esbuild）、桌面壳（Electron 41）、包管理（pnpm/npm）全部运行在 Node.js 上；浏览器端代码（React/Vue）编译产物仍是 JS/HTML/CSS，但开发与构建链路完全 Node.js 化。

## 9.1 桌面客户端

| 技术 | 用途 |
|---|---|
| **Electron 41+** | 跨平台桌面壳（Node.js 主进程 + Chromium 渲染进程） |
| React 19 + TypeScript 5.9 | UI |
| Vite 7 | 构建（Node.js + esbuild） |
| Radix UI / shadcn/ui | 组件库 |
| Zustand / Redux Toolkit | 状态管理 |

> Electron 的 IPC 桥接允许 Node.js 主进程访问本地文件系统、调用原生能力；UI 仍由 React 在 Chromium 渲染。

## 9.2 Web 客户端

| 技术 | 用途 |
|---|---|
| **React 19 + Vite 7** | SPA（Vite dev server 与生产构建均运行在 Node.js 上） |
| TanStack Query | 服务端状态 |
| React Router | 路由 |
| Radix UI | 组件 |

**Node.js 版本要求**：≥ 20 LTS（Vite 7 要求 Node 20.19+ / 22.12+）。

**推荐：MVP 用 Web 优先**（开发快），后续加桌面壳。

---

# 十、基础设施

## 10.1 部署

| 阶段 | 推荐 |
|---|---|
| MVP | Docker Compose 单机（Go 后端单二进制 + 前端 Nginx 静态） |
| 中期 | K8s（minikube → 生产） |
| 大规模 | 多区域 K8s + CDN |

## 10.2 反向代理

**推荐：Nginx**（成熟）或 **Caddy**（自动 HTTPS）

## 10.3 监控与可观测性

| 维度 | 工具 |
|---|---|
| **指标** | Prometheus + Grafana（Go 客户端 [`prometheus/client_golang`](https://github.com/prometheus/client_golang)） |
| **日志** | Loki + Promtail |
| **追踪** | OpenTelemetry + Jaeger（Go SDK [`otel-go`](https://github.com/open-telemetry/opentelemetry-go)） |
| **错误** | Sentry |

## 10.4 CI/CD

**GitHub Actions**（推荐）：
- PR 检查：lint + `go vet` + test + type check（前端用 `tsc`）
- main 合并：自动构建 Docker 镜像 + 部署

## 10.5 配置管理

| 选项 | 场景 |
|---|---|
| 环境变量 | 简单（用 [`caarlos0/env`](https://github.com/caarlos0/env) 解析到结构体） |
| **spf13/viper** | 推荐（YAML/ENV/flags/etcd 全支持，热加载） |
| **kelseyhightower/envconfig** | 纯环境变量 + struct tag（轻量） |
| Consul / etcd | 分布式动态配置 |

```go
type Config struct {
    HTTPAddr  string        `env:"HTTP_ADDR" envDefault:":8080"`
    DBURL     string        `env:"DB_URL" envDefault:"postgres://localhost/aitb"`
    RedisURL  string        `env:"REDIS_URL" envDefault:"redis://localhost:6379"`
    Anthropic string        `env:"ANTHROPIC_API_KEY,required"`
    LogLevel  string        `env:"LOG_LEVEL" envDefault:"info"`
    Timeout   time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`
}

func LoadConfig() (*Config, error) {
    var c Config
    if err := env.Parse(&c); err != nil { return nil, err }
    return &c, nil
}
```

## 10.6 合规认证路径（需求 FR-3.8-C / E + HLD §11.5）

> 投标系统必须满足 **等保三级 + ISO/IEC 42001** 两项硬性合规。本节说明技术组件选型与认证实施路径。

### 10.6.1 身份鉴别与访问控制

| 组件 | 选型 | 适用 |
|---|---|---|
| **Keycloak** | 开源 OIDC + SAML + RBAC | **推荐（自托管）** |
| Authentik | 轻量自托管 | 备选 |
| Ory Hydra / Kratos | 云原生 | 复杂场景 |
| 阿里云 RAM / 华为云 IAM | 云 SaaS | 仅 SaaS 模式 |

**Keycloak 部署**：
- 单实例 2 vCPU / 4GB，PostgreSQL 后端
- 内置 OAuth2 / OIDC / SAML 2.0
- 支持 MFA（TOTP、SMS、Email）
- 角色继承 + 细粒度权限

**多租户隔离**：
- Keycloak Realm 隔离（每客户一个 Realm）
- 或同 Realm + Group 隔离（适合多子部门）
- ABAC 通过 Protocol Mapper 注入用户属性到 JWT

### 10.6.2 密钥管理

| 方案 | 适用 |
|---|---|
| **HashiCorp Vault** | 私有化推荐 |
| 云 KMS（AWS KMS / 阿里云 KMS） | SaaS 模式 |
| SOPS + age | 轻量加密 |

**Vault 用法**：
```bash
# 启用 Transit 引擎（应用层加密）
vault secrets enable transit
vault write -f transit/keys/bid-data

# 应用层加密
vault write transit/encrypt/bid-data plaintext=$(echo "敏感数据" | base64)
vault write transit/decrypt/bid-data ciphertext="vault:v1:..."
```

### 10.6.3 数据加密

| 层 | 方案 |
|---|---|
| 传输 | TLS 1.3（ACME 自动证书） |
| 磁盘 | LUKS（Linux）/ cloud KMS 加密 EBS / 阿里云盘 |
| 应用层 | AES-256-GCM（敏感字段：身份证、银行卡、报价） |
| 备份 | GPG 加密 + 异地存储 |

**应用层加密示例**：

```go
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/hex"
    "errors"
    "io"
)

func EncryptField(plaintext string, key []byte) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil { return "", err }
    aesgcm, err := cipher.NewGCM(block)
    if err != nil { return "", err }
    nonce := make([]byte, aesgcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return "", err }
    ct := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)
    return hex.EncodeToString(nonce) + ":" + hex.EncodeToString(ct), nil
}

func DecryptField(ciphertext string, key []byte) (string, error) {
    parts := strings.SplitN(ciphertext, ":", 2)
    if len(parts) != 2 { return "", errors.New("invalid ciphertext") }
    nonce, err := hex.DecodeString(parts[0])
    if err != nil { return "", err }
    ct, err := hex.DecodeString(parts[1])
    if err != nil { return "", err }
    block, err := aes.NewCipher(key)
    if err != nil { return "", err }
    aesgcm, err := cipher.NewGCM(block)
    if err != nil { return "", err }
    pt, err := aesgcm.Open(nil, nonce, ct, nil)
    if err != nil { return "", err }
    return string(pt), nil
}
```

### 10.6.4 审计日志

**Go `log/slog` + 结构化 JSON**（Go 1.21+ 标准库）：

```go
import (
    "log/slog"
    "os"
)

func NewLogger() *slog.Logger {
    h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    return slog.New(h)
}

// 使用
log := NewLogger()
log.Info("bid_created",
    "bid_id", "b-001",
    "user_id", "u-001",
    "industry", "it",
    "chapters", 50,
)
```

输出（JSON，每行一条，Promtail 抓取）：

```json
{"time":"2026-01-15T10:23:45Z","level":"INFO","msg":"bid_created","bid_id":"b-001","user_id":"u-001","industry":"it","chapters":50}
```

> 若需要更高性能或更丰富的 sink（Kafka、Elasticsearch），可换 [`rs/zerolog`](https://github.com/rs/zerolog)（零分配）或 [`uber-go/zap`](https://github.com/uber-go/zap)。

**审计日志要求**（等保三级）：
- 保留 ≥ 180 天在线 + 3 年归档
- 防篡改：append-only S3 + Object Lock
- 全覆盖：登录、CRUD、LLM 调用、文件下载、权限变更

**Loki 部署**：
- 单实例 2 vCPU / 4GB（百万级日志/天）
- 启用 S3 后端
- Promtail 收集容器日志

### 10.6.5 WAF / DDoS

| 部署模式 | 方案 |
|---|---|
| SaaS | **Cloudflare**（含 WAF + DDoS） |
| 私有化（公网） | ModSecurity + Nginx + 长亭雷池 |
| 私有化（内网） | 仅 Nginx ACL |

### 10.6.6 ISO 42001 专项

**AI 决策可追溯**（ISO 42001 核心要求）：
- LLM 调用必须留痕：prompt + response + model_version + temperature
- 章节级 traceability：每段正文可追溯到 evidence_id
- 决策可回放：审计员可重放任意时刻的 AI 决策

**数据治理**：
- 训练数据（如有）标注来源（招股书 / 公开标书 / 客户授权数据）
- 推理数据：客户上传的招标文件 → 严格客户隔离
- 模型版本控制：MLflow 或 DVC

**第三方管理**（LLM Provider）：
- 多个 provider 备选（Claude / DeepSeek / 通义千问）
- 熔断降级：provider 不可用时切换
- SLA 监控：可用性 ≥ 99.5%、P99 延迟 < 30s

### 10.6.7 认证时间表

| 阶段 | 时间 | 工作 |
|---|---|---|
| **M0-M3** 准备 | 3 个月 | 差距分析、补齐控制、文档 |
| **M3-M6** 试运行 | 3 个月 | 内部审计 + 模拟测评 |
| **M6-M9** 认证 | 3 个月 | 等保三级测评 + ISO 42001 第三方认证 |
| **M9+** 监督 | 年度 | 复审 + 持续改进 |

## 10.7 私有化部署与本地 LLM 推理（需求 D4 + HLD §12.3）

> 私有化模式必须 100% 离线运行，LLM / Embedding / 文生图全部本地推理。

### 10.7.1 本地 LLM 推理引擎

| 引擎 | 优势 | 适用 |
|---|---|---|
| **vLLM** | 吞吐量最高（连续批处理） | **推荐（主）** |
| SGLang | 复杂 prompt 结构优化 | 备选 |
| Ollama | 易用、本地原型 | 轻量场景 |
| llama.cpp | 纯 CPU 推理 | 无 GPU 环境 |
| TensorRT-LLM | NVIDIA 优化 | 高性能 GPU |

**vLLM 部署示例**（独立 Python 推理服务，OpenAI 兼容 HTTP，Go 主服务通过 §5 LLM Router 调用）：

```bash
# Qwen2.5-72B-Instruct-AWQ
python -m vllm.entrypoints.openai.api_server \
    --model Qwen/Qwen2.5-72B-Instruct-AWQ \
    --quantization awq \
    --tensor-parallel-size 2 \
    --gpu-memory-utilization 0.85 \
    --max-model-len 8192 \
    --port 8001

# Qwen2.5-7B-Instruct（快速任务）
python -m vllm.entrypoints.openai.api_server \
    --model Qwen/Qwen2.5-7B-Instruct \
    --port 8002
```

### 10.7.2 本地图表渲染

| 图表类型 | 工具 | 替代（公网） |
|---|---|---|
| 流程图 / 架构图 | **Mermaid CLI**（本地） | mermaid.ink |
| 数据图 | **go-echarts / gonum/plot**（本地导出 PNG/SVG） | - |
| 文生图（可选） | **SD 3.5 + diffusers**（本地 LoRA） | DALL-E 3 |
| OCR | **PaddleOCR**（本地） | 云 OCR |

**Mermaid CLI 离线安装**：
```bash
# 安装 mmdc（Node.js）
npm install -g @mermaid-js/mermaid-cli

# 离线渲染
mmdc -i input.mmd -o output.svg -p puppeteerConfig.json
```

### 10.7.3 模型量化与降级

| 客户硬件 | 推荐配置 | 性能 |
|---|---|---|
| **充足**（2× A100 80G） | Qwen2.5-72B AWQ | 100% 质量 |
| **中等**（1× A100 40G） | Qwen2.5-32B AWQ | 95% 质量 |
| **紧张**（1× RTX 4090 24G） | Qwen2.5-7B INT4 | 80% 质量 |
| **极简**（CPU 32 核） | Qwen2.5-1.5B + llama.cpp | 60% 质量 |

**自适应调度**：

```go
func SelectModel(taskComplexity string, availableGPUs int) string {
    switch {
    case taskComplexity == "high" && availableGPUs >= 2:
        return "qwen2.5-72b-awq"
    case taskComplexity == "medium":
        return "qwen2.5-32b-awq"
    default:
        return "qwen2.5-7b-int4"
    }
}
```

### 10.7.4 离线升级包

```bash
# 升级包结构
bid-system-upgrade-v1.2.3/
├── manifest.yaml              # 版本、checksum
├── images/
│   ├── api-v1.2.3.tar
│   ├── worker-v1.2.3.tar
│   ├── frontend-v1.2.3.tar
│   └── llm-qwen2.5-72b-v1.tar  # 模型权重
├── configs/
│   └── default.yaml
├── scripts/
│   ├── pre-check.sh
│   ├── backup.sh
│   ├── upgrade.sh
│   └── rollback.sh
└── README.md
```

**升级流程**：
```bash
# 1. 预检
./scripts/pre-check.sh
  ✓ 磁盘空间 ≥ 100GB
  ✓ 数据库连接正常
  ✓ 当前版本 v1.2.2

# 2. 备份
./scripts/backup.sh
  → 数据库快照（pg_dump）
  → MinIO 增量备份
  → 配置文件备份

# 3. 加载镜像
docker load -i images/api-v1.2.3.tar
docker load -i images/worker-v1.2.3.tar

# 4. 启动新版本
./scripts/upgrade.sh
  → 蓝绿部署：先启 v1.2.3，验证健康，切流，停 v1.2.2

# 5. 验证
./scripts/post-check.sh
  ✓ 健康检查通过
  ✓ 冒烟测试通过

# 6. 失败回滚
./scripts/rollback.sh
```

### 10.7.4.1 离线升级流程（mermaid 版）

```mermaid
flowchart TB
    Start([运维人员触发升级]) --> PreCheck{预检<br/>磁盘/DB/版本}
    PreCheck -->|失败| Fix[修复环境]
    Fix --> PreCheck
    PreCheck -->|通过| Backup[备份<br/>PG 快照 + MinIO 增量 + 配置]

    Backup --> LoadImg[docker load<br/>加载新镜像]
    LoadImg --> BlueGreen[蓝绿部署]

    BlueGreen --> StartV123[启动 v1.2.3]
    StartV123 --> HealthCheck{健康检查<br/>5s × 3}
    HealthCheck -->|失败| Rollback[自动回滚<br/>v1.2.2 恢复]
    HealthCheck -->|通过| Smoke[冒烟测试]
    Smoke -->|失败| Rollback
    Smoke -->|通过| Cutover[切流<br/>Nginx upstream 切换]
    Cutover --> StopOld[停止 v1.2.2]
    StopOld --> PostCheck{后检<br/>监控/告警}
    PostCheck -->|异常| Rollback
    PostCheck -->|正常| Done([升级完成 v1.2.3])

    Rollback --> VerifyRollback{回滚验证}
    VerifyRollback -->|成功| DoneRollback([回到 v1.2.2])
    VerifyRollback -->|失败| ManualEscalate[人工介入<br/>现场支持]

    classDef success fill:#d4edda,stroke:#28a745,stroke-width:2px
    classDef failure fill:#f8d7da,stroke:#dc3545,stroke-width:2px
    classDef process fill:#e1f5ff,stroke:#0066cc
    class Done success
    class Rollback,ManualEscalate,VerifyRollback failure
    class Backup,LoadImg,Smoke,Cutover,PostCheck process
```

### 10.7.5 数据隔离

**单租户数据库实例**：
- 每客户独立 PostgreSQL 实例（不同容器 / 不同主机）
- 数据库连接串含 customer_id 标识
- 应用层强制 customer_id 过滤

**文件存储隔离**：
- MinIO 桶命名：`bid-customer-{customer_id}-*`
- 桶 ACL 严格按客户隔离
- 跨客户访问 0 容忍

**审计**：
- 所有跨客户访问尝试 → 立即告警 + 阻断
- 季度安全审计 + 渗透测试

---

# 十一、成本预估

按 50 章节、每章节 2000 字、每章节 3 张图、Claude Sonnet 4.6 计算：

| 项 | 单价 | 用量 | 小计 |
|---|---|---|---|
| 章节规划 | $3 / 1M tok in | 50K input + 5K output | $0.15 |
| 章节撰写 | $3 / 1M tok in + $15 / 1M tok out | 50 × (30K in + 4K out) = 1.5M in + 200K out | $4.5 + $3 = $7.5 |
| 图表描述 | 同上 | 50 × (10K in + 1K out) | $1.5 + $0.75 |
| AI 图生成 | $0.04/张 | 150 张 | $6 |
| Mermaid 渲染 | 免费（在线 API） | 100 张 | $0 |
| 跨章节审计 | 同章节撰写 | 1 × (200K in + 20K out) | $0.6 + $0.3 |
| **合计** | | | **~$16 / 份标书** |

使用 Prompt 缓存 + DeepSeek 粗稿：可降至 **~$5-8 / 份标书**。

---

# 十二、风险与备选

| 风险 | 备选方案 |
|---|---|
| Claude API 不可用 | DeepSeek → GPT-4o → 本地模型 |
| mermaid.ink 不可用 | 本地 mermaid-cli |
| AI 图风格不一致 | 缓存相似图、固定 prompt 模板 |
| PostgreSQL 单点 | 主从 + PgBouncer |
| Asynq broker 单点 | Redis Sentinel / Cluster / 切 River（PG） |

---

# 十三、关键决策记录

| 决策 | 选择 | 理由 |
|---|---|---|
| 后端语言 | **Go 1.23+**（Rust 备选） | I/O 并发优、单二进制、CI 反馈快 |
| Web 框架 | **Gin**（Fiber 备选） | 生态最大、与 Asynq/GORM 集成最成熟 |
| 任务队列 | **Asynq**（River 备选） | Go 生态生产级、Redis broker、`asynqmon` UI |
| ORM | **GORM**（sqlx/pgx 备选） | MVP 速度快；复杂查询用 sqlx/pgx |
| 数据库 | PostgreSQL 16 | JSONB + advisory lock + tsvector + pgvector |
| 前端运行时 | **Node.js 20+** | Vite 7 / Electron 41 / 构建工具全栈 Node |
| 主力模型 | Claude Sonnet 4.6 | 200K 上下文 + cache_control |
| 成本模型 | DeepSeek 粗稿 + Claude 精稿 | 双层降本 |
| 文档导出 | unidoc/unioffice + LibreOffice | Go 生态唯一可用 + PDF 兜底 |
| 部署 | Docker Compose MVP → K8s 后期 | 渐进式 |
| CI/CD | GitHub Actions | 与仓库集成最好 |

---

文档完成。下游 `docs/high-level-design.md` 将基于本选型给出组件、数据流、接口的具体设计。