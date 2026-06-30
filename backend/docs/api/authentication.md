# API 认证

## 认证方式

### 1. JWT（默认）

#### 登录获取 token

```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "..."
}
```

**响应 200**：

```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "rt_abc123...",
  "expires_in": 3600,
  "token_type": "Bearer",
  "user": {
    "id": "u-123",
    "email": "user@example.com",
    "name": "User",
    "role": "member",
    "tenant_id": "t-1"
  }
}
```

#### 使用 access_token

```http
GET /api/v1/projects
Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
```

#### 刷新 token

```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "rt_abc123..."
}
```

#### 登出

```http
POST /api/v1/auth/logout
Authorization: Bearer eyJ...
```

### 2. OAuth 2.0 / OIDC

#### 支持的 Provider

- Google
- Microsoft / Azure AD
- GitHub
- 飞书
- 钉钉

#### 流程

```http
GET /api/v1/auth/oauth/{provider}/authorize
  ?redirect_uri=https://app.bidwriter.com/oauth/callback
  &state=random-state-string
```

→ 重定向到 Provider 登录页

→ 回调：

```http
GET /oauth/callback?code=...&state=...
```

后端用 code 换 token，返回 JWT。

### 3. 企业 SSO（SAML 2.0）

适用：企业客户自有 IdP

#### 配置

```bash
# 企业管理员上传 SAML metadata
POST /api/v1/auth/saml/config
Content-Type: application/json

{
  "metadata_xml": "...",
  "domain": "example.com"
}
```

#### 用户登录

```http
GET /api/v1/auth/saml/login?domain=example.com
```

→ 重定向到 IdP

→ 回调返回 JWT

---

## 多因素认证（MFA）

### 启用 TOTP

```http
POST /api/v1/auth/mfa/totp/enable
Authorization: Bearer eyJ...
```

**响应**：

```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "qr_uri": "otpauth://totp/BidWriter:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=BidWriter",
  "backup_codes": ["12345-67890", ...]
}
```

用户用 Google Authenticator / Authy 扫描。

### 登录时验证

```http
POST /api/v1/auth/login
{
  "email": "user@example.com",
  "password": "..."
}
```

如果启用了 MFA：

**响应 200**：

```json
{
  "mfa_required": true,
  "mfa_token": "mfa_temp_..."
}
```

然后：

```http
POST /api/v1/auth/mfa/verify
{
  "mfa_token": "mfa_temp_...",
  "totp_code": "123456"
}
```

**响应 200**：标准 JWT 响应

---

## 权限（RBAC）

### 角色

| 角色 | 说明 |
|---|---|
| `owner` | 团队所有者，全部权限 |
| `admin` | 管理员，管理项目和成员 |
| `member` | 普通成员，创建和编辑 |
| `viewer` | 只读 |

### 资源权限

- 团队（tenant）级：跨项目
- 项目级：单个项目

### 检查

服务端强制，前端 UI 配合：

```http
GET /api/v1/projects
→ 200 OK（用户有权限）

GET /api/v1/projects/p-999
→ 403 Forbidden（用户无该项目权限）
```

---

## 安全建议

### ✅ DO

- 用 HTTPS 传输
- access_token 存内存（不存 localStorage）
- refresh_token 存 httpOnly cookie
- token 过期前主动刷新
- 启用 MFA
- 使用 API Key 而非密码调用 API

### ❌ DON'T

- 不用把 token 写到 URL
- 不在客户端存密码
- 不共享 token
- 不在公网传输明文密码

---

## API Key

适用：服务器间调用

### 创建

```http
POST /api/v1/api-keys
Authorization: Bearer eyJ...

{
  "name": "Backend Service",
  "scopes": ["projects:read", "documents:read", "workflows:write"],
  "expires_at": "2027-01-01T00:00:00Z"
}
```

**响应**：

```json
{
  "id": "ak_123",
  "name": "Backend Service",
  "key": "bw_live_abc123...",  // 只显示一次
  "scopes": [...],
  "expires_at": "..."
}
```

### 使用

```http
GET /api/v1/projects
Authorization: Bearer bw_live_abc123...
```

### 撤销

```http
DELETE /api/v1/api-keys/ak_123
```

---

## 错误

详见 [errors.md](errors.md)

常见：

| HTTP | Code | 含义 |
|---|---|---|
| 401 | UNAUTHORIZED | 缺 token / token 无效 |
| 401 | TOKEN_EXPIRED | access_token 过期 |
| 401 | MFA_REQUIRED | 需要 MFA 验证 |
| 403 | FORBIDDEN | 无权限 |
| 403 | TENANT_MISMATCH | 跨租户访问 |

---

## 相关文档

- [API 概览](overview.md)
- [错误码](errors.md)
- [架构 / 模块设计](../architecture/modules.md)
- [运维 / 安全](../operations/security.md)