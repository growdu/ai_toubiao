# 0001. 多租户隔离粒度

## 状态

Accepted

## 日期

2026-06-27

## 参与者

- 架构组
- 后端组

## 背景

BidWriter 是多租户 SaaS，每个用户属于一个团队（tenant），租户之间数据必须严格隔离。

**约束条件**：
- 中小企业规模（5-50 人/租户）
- 客户量预期：M4 末 ~1000 租户
- 单租户数据：~1-10 GB
- 需要支持跨租户分析（产品迭代、行业洞察）
- pgvector 索引、Redis 缓存需要共享

**需要决策**：选择多租户隔离方案。

## 决策

**采用行级隔离（tenant_id 字段），M1-M3 长期使用。**

所有业务表必须带 `tenant_id` 列（NOT NULL），应用层强制注入到每个查询。

## 理由

- ✅ 中小企业规模行级完全够用
- ✅ 单库管理运维成本低：备份、迁移、监控一套搞定
- ✅ 跨租户分析（行业洞察、产品迭代）容易做
- ✅ pgvector 索引、Redis 缓存可共享
- ✅ 实施成本低：sqlc 模板 + 中间件即可

## 考虑的替代方案

### 方案 A：Schema-per-tenant（PostgreSQL 多 schema）

- ✅ 物理隔离更强
- ✅ 单租户可以独立备份/恢复
- ❌ 跨租户分析困难
- ❌ 连接数受限（PG 默认 100 连接）
- ❌ 运维复杂：迁移需要遍历所有 schema

### 方案 B：Database-per-tenant

- ✅ 完全物理隔离
- ✅ 合规友好
- ❌ 成本极高（每个租户独立 PG）
- ❌ 运维地狱
- ❌ 跨租户分析不可能

### 方案 C：行级隔离（**选择**）

- ✅ 实施简单
- ✅ 跨租户分析容易
- ✅ 共享基础设施成本低
- ⚠️ 必须应用层严格约束（防泄漏）
- ⚠️ 单租户大客户会成为瓶颈

## 后果

### 正面

- 单一 PG 实例管理（备份、监控、迁移一套）
- 跨租户 SQL 容易（用于产品分析）
- pgvector / Redis 索引共享
- 实施成本最低

### 负面

- 应用层必须严格注入 `tenant_id`，一处遗漏就是数据泄漏
- 单租户超大（> 10GB / 高 QPS）会成为瓶颈
- 不适合金融/政府强合规场景

### 中性（需要承担的工作）

- 所有业务表加 `tenant_id` 列
- sqlc 模板自动注入
- 中间件层强制校验
- 测试用例必须覆盖"跨租户访问应拒绝"

## 实施细节

### 数据模型

```sql
CREATE TABLE projects (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,  -- 强制 NOT NULL
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (tenant_id) REFERENCES tenants(id)
);

CREATE INDEX idx_projects_tenant_id ON projects(tenant_id);
```

### 应用层强制

```go
// middleware/tenant.go
func TenantMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            tenantID := auth.GetTenantID(r.Context())
            if tenantID == uuid.Nil {
                http.Error(w, "tenant required", http.StatusUnauthorized)
                return
            }
            ctx := context.WithValue(r.Context(), tenantKey, tenantID)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### sqlc 模板

所有查询自动加 `WHERE tenant_id = $tenantID`：

```sql
-- name: GetProject :one
SELECT * FROM projects
WHERE id = $1 AND tenant_id = $2;  -- 自动注入
```

### 测试

每个 API 端点必须有：

```go
func TestGetProject_OtherTenant_Forbidden(t *testing.T) {
    // 用 tenantA 的 token 访问 tenantB 的项目
    // 期望 404 或 403
}
```

## 退出条件

需要重新评估的触发条件：

- 🔴 单 PG 实例 > 500 GB
- 🔴 某租户合规要求物理隔离（金融/政府强合规）
- 🔴 跨租户查询成为性能瓶颈
- 🔴 单租户 QPS > 1000

切换方案：迁移到 schema-per-tenant。

## 参考

- [架构 / 数据模型](../architecture/data-model.md)
- [运维 / 安全](../operations/security.md)
- [Plan / v1 设计 第 4 节](../plan/v1-design.md)