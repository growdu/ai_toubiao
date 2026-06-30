# 安全合规

> **安全是产品的地基，不是装饰。**

## 安全模型

```
┌──────────────────────────────────────────────────────────┐
│  传输层安全：TLS 1.3 全链路（外部 + 内部）                  │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  认证：OIDC / OAuth 2.0 / SAML（企业 SSO）                 │
│  授权：RBAC 4 角色（owner / admin / member / viewer）      │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  数据层安全：                                              │
│  - tenant_id 行级隔离                                     │
│  - 加密字段（敏感配置、API key）                          │
│  - 审计日志（不可变）                                     │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  应用层安全：                                              │
│  - 输入校验（防 XSS / SQL 注入 / SSRF）                   │
│  - 速率限制（防爆破 / 防滥用）                            │
│  - CSRF 防护                                              │
│  - CSP 头                                                 │
└──────────────────────────────────────────────────────────┘
                          │
                          ▼
┌──────────────────────────────────────────────────────────┐
│  基础设施安全：                                            │
│  - 镜像签名 + 漏洞扫描                                    │
│  - 密钥管理（K8s Secret / Vault）                         │
│  - 网络策略（NetworkPolicy）                              │
│  - Pod Security Standards                                 │
└──────────────────────────────────────────────────────────┘
```

---

## 认证

### OIDC / OAuth 2.0

支持：

- 邮箱密码
- Google / Microsoft / GitHub OAuth
- 企业 SSO（SAML 2.0）

### JWT

```go
type Claims struct {
    Sub       string    `json:"sub"`        // user_id
    TenantID  string    `json:"tenant_id"`  // 租户
    Role      string    `json:"role"`       // owner / admin / member / viewer
    Exp       int64     `json:"exp"`
    Iat       int64     `json:"iat"`
    Jti       string    `json:"jti"`        // 防重放
}
```

配置：

```yaml
auth:
  jwt:
    secret: ${JWT_SECRET}        # 32 bytes
    issuer: bidwriter
    access_token_ttl: 1h
    refresh_token_ttl: 30d
    algorithm: HS256              # 或 RS256（非对称）
```

### 多因素认证（MFA）

- TOTP（Google Authenticator）
- SMS（企业版）
- WebAuthn / FIDO2（企业版）

---

## 授权（RBAC）

### 角色

| 角色 | 项目 | 团队 | 计费 | 审计 |
|---|---|---|---|---|
| **owner** | CRUD | CRUD | CRUD | R |
| **admin** | CRUD | R | R | R |
| **member** | CRU（自己） | R | R | R |
| **viewer** | R | R | R | R |

### 实现

```go
// middleware/auth.go
func RequireRole(roles ...string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userRole := auth.GetRole(r.Context())
            for _, role := range roles {
                if userRole == role {
                    next.ServeHTTP(w, r)
                    return
                }
            }
            http.Error(w, "forbidden", http.StatusForbidden)
        })
    }
}

// 用法
router.Get("/projects", RequireRole("owner", "admin", "member", "viewer"), listProjects)
router.Delete("/projects/{id}", RequireRole("owner"), deleteProject)
```

### 资源级权限

- 项目级：`project_member(project_id, user_id, role)`
- 文档级：继承项目权限

```sql
CREATE TABLE project_members (
    project_id UUID NOT NULL REFERENCES projects(id),
    user_id UUID NOT NULL REFERENCES users(id),
    role VARCHAR(16) NOT NULL,  -- owner | admin | member | viewer
    PRIMARY KEY (project_id, user_id)
);
```

---

## 数据保护

### 1. 静态加密

```yaml
# PostgreSQL
# 透明数据加密（TDE）+ 磁盘加密（云厂商 KMS）
# 列级加密：使用 pgcrypto 加密敏感字段
```

```sql
-- 加密 API key 等敏感字段
CREATE TABLE router_providers (
    id UUID PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    api_key_encrypted BYTEA NOT NULL,  -- pgp_sym_encrypt(api_key, $encryption_key)
    config JSONB
);
```

应用层解密：

```go
func DecryptAPIKey(encrypted []byte) (string, error) {
    var result string
    err := db.QueryRow(`
        SELECT pgp_sym_decrypt($1, $2)::text
    `, encrypted, encryptionKey).Scan(&result)
    return result, err
}
```

### 2. 传输加密

- TLS 1.3 全链路
- 内部服务通信：mTLS（Istio / Linkerd）
- S3 / MinIO：HTTPS

### 3. 数据脱敏

```go
// 导出用户数据时脱敏
func MaskEmail(email string) string {
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return "***"
    }
    name := parts[0]
    if len(name) <= 2 {
        name = name[:1] + "***"
    } else {
        name = name[:1] + strings.Repeat("*", len(name)-2) + name[len(name)-1:]
    }
    return name + "@" + parts[1]
}
```

### 4. 数据删除

用户要求删除数据（GDPR / 隐私政策）：

```bash
# 1. 软删除（标记 deleted_at）
UPDATE users SET deleted_at = NOW() WHERE id = $user_id;

# 2. 30 天后硬删除（清理 Job）
DELETE FROM users WHERE id = $user_id;
DELETE FROM projects WHERE owner_id = $user_id;
-- ... 关联清理
```

API：

```http
DELETE /api/v1/users/me
{
  "confirm": true
}

200 OK
```

### 5. 数据备份 {#backup}

```bash
# 每日全量备份
pg_dump -U postgres bidwriter | gzip > /backup/$(date +%Y%m%d).sql.gz

# 上传到异地
aws s3 cp /backup/20260627.sql.gz s3://bidwriter-backups/db/

# 保留策略
# - 日备份：保留 7 天
# - 周备份：保留 4 周
# - 月备份：保留 12 月
# - 年备份：保留永久

# 定期恢复演练（季度）
```

---

## 应用安全

### 1. 输入校验

```go
// 用 validator 标签
type CreateProjectRequest struct {
    Name        string `validate:"required,min=1,max=256"`
    TemplateID  string `validate:"omitempty,uuid"`
    Description string `validate:"max=2000"`
}

// 防 SQL 注入：始终用参数化查询
db.Query("SELECT * FROM projects WHERE id = $1", id)  // ✅
// db.Query(fmt.Sprintf("SELECT * FROM projects WHERE id = '%s'", id))  // ❌
```

### 2. 防 XSS

```typescript
// React 默认转义
<div>{userInput}</div>  // ✅

// 危险：dangerouslySetInnerHTML
<div dangerouslySetInnerHTML={{ __html: userInput }} />  // ❌

// 如果必须：用 DOMPurify 清理
import DOMPurify from 'dompurify'
<div dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(html) }} />
```

### 3. 防 CSRF

```go
// SameSite Cookie
http.SetCookie(w, &http.Cookie{
    Name:     "session",
    Value:    token,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteLaxMode,
})

// CSRF Token（API）
// - 双提交 Cookie 模式
// - 或用 SameSite=Strict + Authorization header
```

### 4. CSP 头

```go
// 中间件
func CSPMiddleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            w.Header().Set("Content-Security-Policy",
                "default-src 'self'; "+
                "script-src 'self' 'nonce-{NONCE}'; "+
                "style-src 'self' 'nonce-{NONCE}'; "+
                "img-src 'self' data: https:; "+
                "connect-src 'self'; "+
                "frame-ancestors 'none';")
            next.ServeHTTP(w, r)
        })
    }
}
```

### 5. 速率限制

```go
import "github.com/go-chi/httprate"

// 全局限制：每 IP 100 req/min
r.Use(httprate.LimitByIP(100, time.Minute))

// API 限制：每 user 1000 req/min
r.Use(httprate.LimitByKey(1000, time.Minute, func(r *http.Request) string {
    return auth.GetUserID(r.Context())
}))

// 登录限制：每 IP 5 次/分钟（防爆破）
r.With(httprate.LimitByIP(5, time.Minute)).Post("/login", loginHandler)
```

---

## 密钥管理

### 1. 开发环境

```bash
# .env（不提交）
JWT_SECRET=<32 bytes random>
OPENAI_API_KEY=sk-...
DB_PASSWORD=postgres

# .env.example（提交，占位）
JWT_SECRET=change-me-to-32-bytes-random
OPENAI_API_KEY=your-openai-key
DB_PASSWORD=change-me
```

生成密钥：

```bash
openssl rand -base64 32
```

### 2. 生产环境

**Kubernetes Secret**：

```bash
# 创建
kubectl create secret generic bidwriter-secrets \
  --from-literal=JWT_SECRET=$(openssl rand -base64 32) \
  --from-literal=OPENAI_API_KEY=sk-... \
  --from-literal=DB_PASSWORD=$(openssl rand -base64 32) \
  -n bidwriter

# 查看
kubectl get secret bidwriter-secrets -n bidwriter -o yaml
```

**Helm values**：

```yaml
secrets:
  JWT_SECRET: ""  # 留空，使用 --set 或外部 secret
  OPENAI_API_KEY: ""
```

```bash
helm install bidwriter bidwriter/bidwriter \
  --set secrets.JWT_SECRET=$JWT_SECRET \
  --set secrets.OPENAI_API_KEY=$OPENAI_API_KEY \
  -n bidwriter
```

**外部 Secret 管理（Vault）**：

```yaml
# 使用 external-secrets-operator
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: bidwriter-secrets
spec:
  secretStoreRef:
    name: vault-backend
    kind: ClusterSecretStore
  target:
    name: bidwriter-secrets
  data:
    - secretKey: JWT_SECRET
      remoteRef:
        key: bidwriter/jwt
        property: secret
```

### 3. 密钥轮换

```bash
# JWT_SECRET 轮换（每 90 天）
# 1. 生成新密钥
NEW_SECRET=$(openssl rand -base64 32)

# 2. 更新 Secret
kubectl patch secret bidwriter-secrets -n bidwriter \
  -p "{\"data\":{\"JWT_SECRET\":\"$(echo -n $NEW_SECRET | base64)\"}}"

# 3. 重启服务（让所有实例用新密钥）
kubectl rollout restart deployment/api-gateway -n bidwriter

# 4. 验证（5 分钟内）
curl http://api.bidwriter.com/healthz

# 5. 旧 token 在 1 小时内自然过期（access_token_ttl）
```

---

## 漏洞管理

### 1. 依赖扫描

- Dependabot（自动 PR）
- Trivy / Snyk（CI 扫描）

```yaml
# .github/workflows/security-scan.yml
name: Security Scan
on: [push, pull_request]
jobs:
  trivy:
    runs-on: ubuntu-latest
    steps:
      - uses: aquasecurity/trivy-action@master
        with:
          scan-type: 'fs'
          scan-ref: '.'
          format: 'sarif'
          output: 'trivy-fs.sarif'
      - uses: github/codeql-action/upload sarif@v3
        with:
          sarif_file: 'trivy-fs.sarif'
```

### 2. 容器镜像扫描

```bash
# Trivy 扫描镜像
trivy image ghcr.io/yourorg/api-gateway:v0.1.0

# 看 Critical / High 漏洞
trivy image --severity CRITICAL,HIGH ghcr.io/yourorg/api-gateway:v0.1.0
```

### 3. 渗透测试

- **季度**：内部安全团队
- **年度**：第三方专业机构

### 4. 漏洞响应

| 严重程度 | 响应时间 | 修复时间 |
|---|---|---|
| Critical | 1 小时 | 24 小时 |
| High | 4 小时 | 7 天 |
| Medium | 1 天 | 30 天 |
| Low | 1 周 | 90 天 |

---

## 合规

### GDPR（中国版：个人信息保护法 PIPL）

✅ **我们做到**：

- 用户数据不用于训练 AI 模型
- 用户可导出个人数据（`GET /api/v1/users/me/export`）
- 用户可删除账户（`DELETE /api/v1/users/me`）
- 数据处理有合法基础（合同 / 同意）
- 数据传输有合规机制
- 任命数据保护负责人（DPO）

### 等保 2.0（中国信息安全等级保护）

- **SaaS**：等保三级（公有云）
- **私有化**：等保三级（客户机房）

要求：

- 物理安全
- 网络安全
- 主机安全
- 应用安全
- 数据安全
- 安全管理

### SOC 2（私有化出口客户）

- 安全（Security）
- 可用性（Availability）
- 保密性（Confidentiality）

---

## 审计日志

### 不可变日志

```sql
CREATE TABLE audit_logs (
    id BIGSERIAL PRIMARY KEY,
    tenant_id UUID NOT NULL,
    user_id UUID,
    action VARCHAR(64) NOT NULL,     -- create_project / delete_user / ...
    resource_type VARCHAR(32),       -- project / user / billing
    resource_id UUID,
    ip_address INET,
    user_agent TEXT,
    request_id UUID,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 只追加，禁止 UPDATE/DELETE
CREATE RULE no_update_audit AS ON UPDATE TO audit_logs DO INSTEAD NOTHING;
CREATE RULE no_delete_audit AS ON DELETE TO audit_logs DO INSTEAD NOTHING;
```

### 记录内容

| 事件类型 | 必记字段 |
|---|---|
| 登录 | user_id, ip, user_agent, success |
| 创建资源 | user_id, resource_type, resource_id |
| 修改资源 | user_id, resource_id, before, after |
| 删除资源 | user_id, resource_id |
| 权限变更 | user_id, target_user_id, old_role, new_role |
| 计费变更 | user_id, amount, type |
| 安全事件 | user_id, event_type, severity |

### 保留与查询

- 保留期：1 年（在线）+ 5 年（归档）
- 查询 API：`GET /api/v1/audit-logs?from=...&to=...&user_id=...`

---

## 事件响应

### 报告安全问题

**请勿在公开 Issue 报告！**

发送邮件到：**security@bidwriter.app**

包含：

- 问题描述
- 复现步骤
- 影响范围
- 建议修复（可选）

### 响应承诺

- **48 小时内** 确认
- **Critical 漏洞**：24 小时内修复 + 通知用户
- **公开致谢**（如愿意）

### Bug Bounty

- **严重**：$500 - $5,000
- **高**：$100 - $500
- **中**：$50 - $100
- **低**：致谢 + 礼物

详见 <https://bidwriter.app/security/bounty>

---

## 相关文档

- [架构总览](../architecture/overview.md)
- [运维 / 部署](deployment.md)
- [运维 / 监控](monitoring.md)
- [运维 / 故障排查](troubleshooting.md)
- [ADR-0001 多租户隔离](../decisions/0001-multi-tenant.md)