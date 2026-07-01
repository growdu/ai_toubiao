# 0009. knowledge-svc 向量编解码

## 状态

Accepted

## 日期

2026-06-30

## 参与者

- 架构组
- 后端组

## 背景

`migrations/00007_init_kb.up.sql` 已经定义 `kb_chunks.content_vec VECTOR(1536)` 与 `ivfflat` 索引,但 `internal/store/kb_store.go` 的 `SearchChunks` / `CreateChunkWithVec` 都是把 `[]float32` 直接塞给 `pgx.Query` / `QueryRow`。`pgx/v5` 默认 codec 不识别 pgvector 的二进制序列化格式,目前的代码 `go vet` 不会报错但真实运行会被 PostgreSQL 报 `column "content_vec" is of type vector but expression is of type float[]`。

**约束条件:**
- 维度 1536 (OpenAI text-embedding-3-small 兼容),不能写死不能扩展
- 单一项目内只对 `content_vec` 用 vector 类型,不要全局侵入
- 现有 `Search` 业务逻辑 (`kb_service.go`) 不要改,只动 store 层 codec

## 决策

**采用 [`pgvector-go`](https://github.com/pgvector/pgvector-go) 的 `pgx` 子包 (`pgvector/pgx`)**,在 `Store` 构造时 `registerVectorCodec(pool)`,把 vector 列绑定到 pgvector 类型。所有 SQL 不动,只是把传过去的 `[]float32` 在调用点显式转成 `pgvector.Vector`。

```go
import "github.com/pgvector/pgvector-go"

func registerVectorCodec(pool *pgxpool.Pool) {
    pgvector.RegisterTypes(context.Background(), pool) // 注册 vector / halfvec / sparsevec / bit
}

// 调用点
vec := pgvector.NewVector(embedding)
err := pool.QueryRow(ctx, "... content_vec ...", ..., vec).Scan(...)
```

读出时同样:`pgvector.Vector` 实现了 `pgx.Scanner` / `pgx.Valuer`,scan 进 `*pgvector.Vector` 再 `.Slice()`。

## 理由

- ✅ 官方维护,版本与 pgvector 扩展同步
- ✅ 注册动作是局部的,不影响 `pgxpool` 默认行为
- ✅ `SearchChunks` 的查询 SQL 几乎一行不动,只换参数类型
- ✅ Cover both directions (VALUES / SELECT)

## 考虑的替代方案

### 方案 A：手写 Codec

- ❌ 0 行依赖,但 vector 二进制 wire format 文档稀薄,bug 多
- ❌ 维护负担大

### 方案 B：`pgvector-go`(选择)

- ✅ 维护成熟,MIT
- ✅ 单一 `RegisterTypes` 调用覆盖所有 vector 家族

### 方案 C：把向量存到 JSONB (`vec float[]`)

- ❌ 失去 ivfflat / hnsw 索引加速
- ❌ 等于放弃 pgvector 的全部价值

## 后果

### 正面

- 真实能跑向量搜索 (现在这条路径实际是 dead code)
- 单元测试可以 round-trip 验证(无须连 PG,直接用 `pgvector.NewVector(...).Slice()` 比对)

### 负面

- 多一个直接依赖 (`github.com/pgvector/pgvector-go`)
- `go.work.sum` 增加 ~5 行

### 中性

- `kb_store.go` 增加 `registerVectorCodec` 调用 (从 `New(pool)` 内部发起)
- 调用点改成 `pgvector.NewVector(emb)`
- 加一个 `vector_codec_test.go`,验证 []float32 <-> pgvector.Vector round-trip

## 实施细节

```go
// store/kb_store.go
func New(pool *pgxpool.Pool) *Store {
    pgvector.RegisterTypes(context.Background(), pool)
    return &Store{pool: pool}
}
```

## 退出条件

- 🔴 pgvector 引入 v0.8 后三维以上类型,需要 adapter

## 参考

- [architecture / modules](../architecture/modules.md#knowledge-svc)
- [pgvector-go 文档](https://github.com/pgvector/pgvector-go)
