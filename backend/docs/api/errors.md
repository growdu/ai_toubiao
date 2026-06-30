# 错误码

> **统一的错误响应格式**，方便客户端处理。

## 错误响应格式

```json
{
  "error": {
    "code": "INVALID_INPUT",
    "message": "项目名称不能为空",
    "details": {
      "field": "name",
      "reason": "required"
    },
    "request_id": "req-abc-123",
    "documentation_url": "https://docs.bidwriter.com/api/errors#INVALID_INPUT"
  }
}
```

字段：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `code` | string | 是 | 机器可读错误码 |
| `message` | string | 是 | 人类可读消息 |
| `details` | object | 否 | 错误详情 |
| `request_id` | string | 是 | 请求 ID（用于排查）|
| `documentation_url` | string | 否 | 文档链接 |

---

## HTTP 状态码

| 状态码 | 含义 | 客户端处理 |
|---|---|---|
| 400 | 输入错误 | 修复参数后重试 |
| 401 | 未认证 | 重新登录 |
| 403 | 无权限 | 联系管理员 |
| 404 | 资源不存在 | 检查 ID |
| 409 | 冲突 | 刷新数据后重试 |
| 422 | 业务规则错误 | 提示用户 |
| 429 | 速率限制 | 等待后重试 |
| 500 | 服务器错误 | 重试或联系支持 |
| 503 | 服务不可用 | 重试 |

---

## 错误码列表

### 认证 / 授权（4xx）

| Code | HTTP | 说明 |
|---|---|---|
| `UNAUTHORIZED` | 401 | 缺少或无效的 token |
| `TOKEN_EXPIRED` | 401 | access_token 已过期 |
| `TOKEN_INVALID` | 401 | token 格式错误 |
| `MFA_REQUIRED` | 401 | 需要多因素认证 |
| `MFA_INVALID` | 401 | MFA 验证码错误 |
| `FORBIDDEN` | 403 | 无权限 |
| `TENANT_MISMATCH` | 403 | 跨租户访问 |
| `ROLE_INSUFFICIENT` | 403 | 角色权限不足 |
| `RESOURCE_LOCKED` | 423 | 资源被锁 |

### 输入（4xx）

| Code | HTTP | 说明 |
|---|---|---|
| `INVALID_INPUT` | 400 | 输入校验失败 |
| `MISSING_FIELD` | 400 | 缺少必填字段 |
| `INVALID_FORMAT` | 400 | 字段格式错误 |
| `VALUE_OUT_OF_RANGE` | 400 | 值超出范围 |
| `INVALID_UUID` | 400 | UUID 格式错误 |
| `INVALID_EMAIL` | 400 | 邮箱格式错误 |
| `PASSWORD_TOO_WEAK` | 400 | 密码强度不足 |

### 资源（4xx）

| Code | HTTP | 说明 |
|---|---|---|
| `NOT_FOUND` | 404 | 资源不存在 |
| `ALREADY_EXISTS` | 409 | 资源已存在 |
| `DUPLICATE_NAME` | 409 | 名称重复 |
| `VERSION_CONFLICT` | 409 | 版本冲突（乐观锁）|
| `SYNC_CONFLICT` | 409 | 同步冲突 |
| `REFERENCED` | 422 | 资源被引用，无法删除 |

### 业务（4xx）

| Code | HTTP | 说明 |
|---|---|---|
| `QUOTA_EXCEEDED` | 429 | 配额超限 |
| `AI_BUDGET_EXHAUSTED` | 429 | AI 预算耗尽 |
| `RATE_LIMITED` | 429 | 速率限制 |
| `WORKFLOW_NOT_READY` | 409 | 工作流未就绪 |
| `INVALID_STATE_TRANSITION` | 409 | 状态机非法转换 |
| `TEMPLATE_REQUIRED` | 422 | 必须选择模板 |

### 服务器（5xx）

| Code | HTTP | 说明 |
|---|---|---|
| `INTERNAL_ERROR` | 500 | 服务器内部错误 |
| `DATABASE_ERROR` | 500 | 数据库错误 |
| `EXTERNAL_API_ERROR` | 502 | 外部 API 调用失败 |
| `AI_PROVIDER_ERROR` | 502 | AI Provider 错误 |
| `AI_PROVIDER_TIMEOUT` | 504 | AI Provider 超时 |
| `STORAGE_ERROR` | 500 | 对象存储错误 |
| `SERVICE_UNAVAILABLE` | 503 | 服务不可用 |
| `DEPLOYMENT_IN_PROGRESS` | 503 | 部署进行中 |

---

## 详细错误

### `INVALID_INPUT`

```json
{
  "error": {
    "code": "INVALID_INPUT",
    "message": "输入校验失败",
    "details": {
      "fields": [
        {
          "field": "name",
          "reason": "required",
          "message": "项目名称不能为空"
        },
        {
          "field": "template_id",
          "reason": "format",
          "message": "template_id 必须是 UUID"
        }
      ]
    },
    "request_id": "req-abc-123"
  }
}
```

### `VERSION_CONFLICT`

```json
{
  "error": {
    "code": "VERSION_CONFLICT",
    "message": "资源已被其他用户修改",
    "details": {
      "your_version": 5,
      "current_version": 6,
      "modified_by": "user-456",
      "modified_at": "2026-06-27T14:30:00Z"
    },
    "request_id": "req-abc-123"
  }
}
```

客户端处理：

```javascript
if (err.code === 'VERSION_CONFLICT') {
  // 提示用户重新加载
  toast.warning('内容已被其他人修改，请刷新查看最新版本');
  refetch();
}
```

### `AI_PROVIDER_ERROR`

```json
{
  "error": {
    "code": "AI_PROVIDER_ERROR",
    "message": "AI 调用失败",
    "details": {
      "provider": "anthropic",
      "model": "claude-sonnet-4",
      "task": "rfp_parse",
      "upstream_error": "rate limit exceeded",
      "retried": 3,
      "fallback_used": "openai/gpt-4o"
    },
    "request_id": "req-abc-123"
  }
}
```

---

## 客户端处理建议

### JavaScript / TypeScript

```typescript
async function apiCall<T>(url: string, options?: RequestInit): Promise<T> {
  const response = await fetch(url, options)

  if (response.ok) {
    return response.json()
  }

  const error = await response.json().then(r => r.error)

  switch (error.code) {
    case 'UNAUTHORIZED':
    case 'TOKEN_EXPIRED':
      // 重定向登录
      window.location.href = '/login'
      break

    case 'FORBIDDEN':
      toast.error('无权限执行此操作')
      break

    case 'RATE_LIMITED':
    case 'AI_BUDGET_EXHAUSTED':
      toast.warning('请求过于频繁，请稍后再试')
      break

    case 'VERSION_CONFLICT':
      toast.warning('内容已被其他人修改')
      // refetch
      break

    case 'INTERNAL_ERROR':
    case 'SERVICE_UNAVAILABLE':
      toast.error('服务暂时不可用，请稍后再试')
      // 重试（带退避）
      break

    default:
      toast.error(error.message || '未知错误')
  }

  throw new ApiError(error.code, error.message, error.details, response.status)
}
```

### Go

```go
type APIError struct {
    Code       string                 `json:"code"`
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details"`
    RequestID  string                 `json:"request_id"`
    StatusCode int                    `json:"-"`
}

func (e *APIError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func IsRetryable(err error) bool {
    var apiErr *APIError
    if !errors.As(err, &apiErr) {
        return false
    }
    switch apiErr.Code {
    case "RATE_LIMITED", "SERVICE_UNAVAILABLE", "AI_PROVIDER_TIMEOUT":
        return true
    }
    return false
}
```

---

## 错误监控

### Sentry 集成（推荐）

```typescript
import * as Sentry from '@sentry/nextjs'

Sentry.init({
  dsn: process.env.NEXT_PUBLIC_SENTRY_DSN,
  beforeSend(event, hint) {
    const error = hint.originalException
    if (error instanceof ApiError) {
      event.tags = {
        ...event.tags,
        'api.error_code': error.code,
        'api.status': error.StatusCode,
      }
      event.user = { id: getUserID() }
    }
    return event
  },
})
```

### 服务端

```go
slog.Error("API error",
    "request_id", requestID,
    "code", apiErr.Code,
    "status", apiErr.StatusCode,
    "user_id", userID,
    "tenant_id", tenantID,
    "err", err,
)
```

告警规则：[monitoring.md 告警规则](../operations/monitoring.md#alert-rules)

---

## 相关文档

- [API 概览](overview.md)
- [认证](authentication.md)
- [运维 / 监控](../operations/monitoring.md)
- [运维 / 故障排查](../operations/troubleshooting.md)