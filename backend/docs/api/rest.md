# REST API 约定

本文档描述 BidWriter 各 HTTP API 的**通用约定**。
具体接口定义请查阅 `openapi.yaml`（OpenAPI 3.0.3 规范）。

## 1. 路径与版本

- 所有业务接口以 `/api/v1/` 为前缀
- 路径使用 **名词复数**（`/projects` 而非 `/getProject`）
- 用 HTTP 方法表达动作：
  - `GET`    — 查询
  - `POST`   — 创建
  - `PATCH`  — 部分更新
  - `DELETE` — 删除

## 2. 请求

### 2.1 Headers（必需）

| Header | 必需 | 说明 |
|---|---|---|
| `Authorization` | 除公开接口外必需 | `Bearer <token>` |
| `Content-Type` | `POST` / `PATCH` 必需 | `application/json; charset=utf-8` |
| `X-Request-ID` | 可选；客户端可生成以便追踪 | UUIDv4；服务端会保留或生成 |

### 2.2 Body

- `POST` / `PATCH` 请求体必须是合法 JSON
- 字段命名采用 `snake_case`
- 时间字段使用 ISO 8601：`2026-06-28T12:34:56Z`

### 2.3 查询参数

- 分页：`?limit=N&cursor=<uuid>`（cursor 模式，禁止 OFFSET）
- 过滤：`?status=active`
- 字段筛选：`?fields=id,name,status`

## 3. 响应

### 3.1 成功响应

```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
X-Request-ID: <uuid>

{
  "data": { ... },
  "meta": { "count": 20, "next_cursor": "..." }
}
```

- 单一资源：`{ "data": {...} }`
- 列表：`{ "data": [...], "meta": {...} }`
- 创建成功：`201 Created` + 完整资源对象

### 3.2 错误响应

所有 4xx / 5xx 响应使用统一错误信封（详见 [errors.md](errors.md)）：

```http
HTTP/1.1 404 Not Found
Content-Type: application/json; charset=utf-8

{
  "error": {
    "code": "NOT_FOUND",
    "message": "项目不存在",
    "request_id": "abc-123"
  }
}
```

### 3.3 状态码速查

| 状态码 | 含义 |
|---|---|
| 200 | OK |
| 201 | Created |
| 204 | No Content（删除成功） |
| 400 | 参数无效 |
| 401 | 未认证 |
| 403 | 无权访问 |
| 404 | 资源不存在 |
| 409 | 版本冲突（乐观锁失败） |
| 422 | 业务规则校验失败 |
| 429 | 触发限流 |
| 500 | 服务器内部错误 |
| 502 | 上游不可达 |
| 503 | 服务暂不可用 |

## 4. 鉴权

- 默认使用 JWT（HS256），详见 [authentication.md](authentication.md)
- Access token 有效期 1 小时，refresh token 30 天
- 网关会在 header 中注入 `X-Tenant-ID` / `X-User-ID` / `X-User-Role`，下游服务**必须信任**这些 header 并据此隔离数据

## 5. 租户隔离（ADR-0001）

- 所有数据库查询**必须**带 `tenant_id` 过滤
- 跨租户访问一律返回 404（**不**返回 403，避免泄露存在性）
- 不得在响应中包含其他租户的数据

## 6. 乐观锁（ADR-0002）

- 任何带 `version` 字段的资源，更新时必须提交当前版本号
- `PATCH` 时 `version` 缺失或不匹配 → `409 VERSION_CONFLICT`
- 客户端应刷新数据后重试

## 7. 幂等性

- `POST /api/v1/auth/login` 是幂等的（相同凭据返回相同结果）
- `POST` 创建资源不是幂等的（每次生成新 ID）
- 删除操作可重试：第二次返回 404 视作成功

## 8. 分页（ADR-0006）

- 列表接口必须支持 `limit` + `cursor`
- `limit` 默认 20，最大 100
- 响应中 `meta.next_cursor` 为 `null` 时表示无更多数据
- 不允许用 `OFFSET`（性能差，对热点数据不友好）

## 9. 限流

- 默认每租户 60 req/min（可在 api-gateway 配置）
- 触发限流返回 `429 RATE_LIMITED`
- 客户端应实现指数退避重试

## 10. 参考

- OpenAPI 规范：`openapi.yaml`
- 错误码字典：[errors.md](errors.md)
- 认证方式：[authentication.md](authentication.md)
- 接口总览：[overview.md](overview.md)