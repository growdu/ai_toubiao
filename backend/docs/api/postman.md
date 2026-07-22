# Postman / HTTP Client 集合

> 把 [`openapi.generated.yaml`](openapi.generated.yaml) 翻译成 [Postman v2.1](https://schema.getpostman.com/json/collection/v2.1.0/collection.json) 集合，
> 方便在 Postman / VS Code REST Client / curl 里直接调试后端 API。

## 文件

- [`postman_collection.json`](postman_collection.json) — 完整集合（10 个文件夹，73 个请求）。

## 变量（导入后可在 Postman 右上角修改）

| 变量 | 默认值 | 用途 |
|---|---|---|
| `baseUrl` | `http://localhost:8080/api/v1` | 通过 api-gateway 转发 |
| `jwt` | `<paste access token>` | Bearer Token；先调 `POST /api/v1/auth/login` 取得 |
| `tenantId` | `11111111-1111-1111-1111-111111111111` | 多租户头 `X-Tenant-Id` |

## 使用方式

### 1. Postman

- 打开 Postman → **Import** → 选 `postman_collection.json`。
- 编辑 `Variables`：
  - `baseUrl` 改成你的 api-gateway 地址；
  - 调 `POST /api/v1/auth/login`，把响应里的 `access_token` 复制到 `jwt`。
- 其它请求会自动带 `Authorization: Bearer {{jwt}}`。

### 2. VS Code REST Client（`.http`）

REST Client 不直接读 Postman 集合，可借助 [`openapi-generator`](https://openapi-generator.tech) 转一份 `.http` 文件：

```bash
npx --yes openapi-typescript-codegen \
  --input docs/api/openapi.generated.yaml \
  --output web/src/api/generated
```

或者手写关键请求示例（保存在 `docs/api/examples/*.http`，作为新人 oncall 参考）。

### 3. curl

```bash
# 1. 登录拿 JWT
TOKEN=$(curl -s -X POST "$BASE_URL/api/v1/auth/login" \
  -H 'Content-Type: application/json' \
  -d '{"email":"dev@bidwriter.app","password":"dev"}' \
  | jq -r .access_token)

# 2. 列项目
curl -s "$BASE_URL/api/v1/projects" \
  -H "Authorization: Bearer $TOKEN" \
  -H "X-Tenant-Id: $TENANT_ID" | jq .
```

## 目录组织

| 文件夹 | 来源服务 | ops 数 |
|---|---|---|
| `api-gateway` | api-gateway | 3 |
| `audit` | audit-svc | 3 |
| `billing` | billing-svc | 6 |
| `documents` | document-svc | 10 |
| `knowledge` | knowledge-svc | 5 |
| `notifications` | notify-svc | 5 |
| `project` | project-svc | 5 |
| `router` | router-svc | 10 |
| `template` | template-svc | 6 |
| `workflow` | workflow-svc | 19 |

> 当前 schema 是占位（`GenericRequest` / `OKEnvelope`），等每个服务接上 swag 注释后，重新跑 `python3 scripts/gen-api.py` 即可同步。
