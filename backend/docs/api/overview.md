# API 概览

> **REST + SSE**，所有 API 在 API Gateway（`https://api.bidwriter.com`）下。

## 基本信息

- **Base URL**：`https://api.bidwriter.com/api/v1`
- **认证**：`Authorization: Bearer <jwt>`
- **格式**：JSON（请求 + 响应）
- **字符集**：UTF-8
- **限流**：100 req/min（按 IP），1000 req/min（按 user）
- **响应时间**：p95 < 1s（非 AI 调用）

## 版本控制

URL 路径版本：`/api/v1/...`

向后兼容的变更不升级版本。破坏性变更走新版本。

## 认证

详见 [authentication.md](authentication.md)

```bash
# 登录
curl -X POST https://api.bidwriter.com/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"..."}'

# 响应
{
  "access_token": "eyJ...",
  "refresh_token": "...",
  "expires_in": 3600,
  "user": { "id": "...", "email": "...", "role": "..." }
}

# 后续请求带 token
curl https://api.bidwriter.com/api/v1/projects \
  -H "Authorization: Bearer eyJ..."
```

## 错误响应

详见 [errors.md](errors.md)

```json
{
  "error": {
    "code": "INVALID_INPUT",
    "message": "项目名称不能为空",
    "details": {
      "field": "name",
      "reason": "required"
    },
    "request_id": "req-abc-123"
  }
}
```

HTTP 状态码：

| 状态码 | 含义 |
|---|---|
| 200 | 成功 |
| 201 | 创建成功 |
| 204 | 删除成功（无内容）|
| 400 | 输入错误 |
| 401 | 未认证 |
| 403 | 无权限 |
| 404 | 资源不存在 |
| 409 | 冲突（如重复）|
| 422 | 业务规则错误 |
| 429 | 速率限制 |
| 500 | 服务器错误 |
| 503 | 服务不可用 |

---

## 核心资源

### Project（项目）

| Method | Path | 说明 |
|---|---|---|
| GET | `/projects` | 列出项目 |
| POST | `/projects` | 创建项目 |
| GET | `/projects/{id}` | 获取项目详情 |
| PATCH | `/projects/{id}` | 更新项目 |
| DELETE | `/projects/{id}` | 删除项目 |
| GET | `/projects/{id}/documents` | 列出项目的文档 |

### Document（文档）

| Method | Path | 说明 |
|---|---|---|
| POST | `/documents/upload` | 上传文档 |
| GET | `/documents/{id}` | 获取文档元数据 |
| GET | `/documents/{id}/content` | 获取 Markdown 内容 |
| DELETE | `/documents/{id}` | 删除文档 |

### Workflow（工作流）

| Method | Path | 说明 |
|---|---|---|
| POST | `/workflows` | 创建工作流 |
| GET | `/workflows/{id}` | 获取工作流状态 |
| POST | `/workflows/{id}/step/{step}` | 触发步骤 |
| POST | `/workflows/{id}/cancel` | 取消工作流 |
| GET | `/workflows/{id}/stream` | SSE 实时进度 |

### Knowledge（知识库）

| Method | Path | 说明 |
|---|---|---|
| POST | `/knowledge/documents` | 添加文档 |
| GET | `/knowledge/search` | 搜索 |
| DELETE | `/knowledge/documents/{id}` | 删除 |

### Template（模板）

| Method | Path | 说明 |
|---|---|---|
| GET | `/templates` | 列出模板 |
| POST | `/templates` | 创建模板（M3+） |
| GET | `/templates/{id}` | 获取模板详情 |
| POST | `/templates/{id}/apply` | 应用到项目 |

### Billing（计费）

| Method | Path | 说明 |
|---|---|---|
| GET | `/billing/subscription` | 当前订阅 |
| GET | `/billing/usage` | 用量统计 |
| GET | `/billing/invoices` | 发票列表 |

### Audit（审计）

| Method | Path | 说明 |
|---|---|---|
| POST | `/audits` | 创建审计任务 |
| GET | `/audits/{id}` | 获取审计结果 |
| GET | `/audits/{id}/issues` | 获取问题列表 |

---

## 实时通信（SSE）

工作流进度用 Server-Sent Events：

```javascript
const es = new EventSource(
  `/api/v1/workflows/${workflowId}/stream`,
  { withCredentials: true }
);

es.addEventListener('progress', (e) => {
  const data = JSON.parse(e.data);
  console.log(`Step ${data.step}: ${data.progress}%`);
});

es.addEventListener('step_completed', (e) => {
  const data = JSON.parse(e.data);
  console.log(`Step ${data.step} done in ${data.duration}s`);
});

es.addEventListener('error', (e) => {
  console.error('SSE error', e);
});
```

事件类型：

- `progress`：进度更新（每秒）
- `step_started`：步骤开始
- `step_completed`：步骤完成
- `step_failed`：步骤失败
- `workflow_completed`：工作流完成
- `error`：错误

---

## 分页

列表 API 用 cursor 分页：

```http
GET /api/v1/projects?limit=20&cursor=eyJpZCI6IjEyMyJ9
```

```json
{
  "data": [...],
  "next_cursor": "eyJpZCI6IjQ1NiJ9",
  "has_more": true
}
```

---

## 过滤 & 排序

```http
GET /api/v1/projects?status=draft&sort=-created_at&limit=20
```

支持的过滤和排序字段见各资源文档。

---

## 字段选择

```http
GET /api/v1/projects?fields=id,name,status
```

减少 payload。

---

## 批量操作

部分接口支持批量：

```http
POST /api/v1/projects/batch-delete
{
  "ids": ["...", "...", "..."]
}
```

---

## Webhook

事件驱动集成（v1.1+）：

```http
POST /api/v1/webhooks
{
  "url": "https://your-server.com/webhook",
  "events": ["workflow.completed", "audit.completed"],
  "secret": "..."
}
```

事件：

- `project.created`
- `workflow.completed`
- `workflow.failed`
- `audit.completed`
- `billing.invoice.created`

详见：[webhook-subscriptions skill](https://github.com/yourorg/bidwriter/blob/main/docs/api/webhooks.md)（待补充）

---

## 速率限制

响应头：

```http
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 73
X-RateLimit-Reset: 1624836000
```

超额响应：

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60
```

---

## OpenAPI 规范

完整规范：[openapi.yaml](openapi.yaml)（待生成）

或在 `/api/v1/openapi.json` 获取。

---

## SDK

官方 SDK：

- **TypeScript**：`npm install @bidwriter/sdk`
- **Go**：`go get github.com/yourorg/bidwriter-go`
- **Python**：`pip install bidwriter`（v1.1+）

---

## 相关文档

- [认证](authentication.md)
- [错误码](errors.md)
- [架构 / 模块设计](../architecture/modules.md)