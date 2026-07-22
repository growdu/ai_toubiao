# Backend HTTP API: openapi.yaml vs actual routes
> 由 `scripts/gen-api.py` 产出，伴随 `openapi.generated.yaml` 一并提交。
## 对比基准

- `docs/api/openapi.yaml`（旧规范，仓库初始版本）：9 ops / 6 paths
- 实际注册路由（来自 `internal/api/*.go` + `api-gateway/cmd/.../main.go`）：73 ops

## 1. 旧 `openapi.yaml` 缺失的路由（按服务分组）

### api-gateway

- `POST /api/v1/auth/login` — `loginHandler`
- `POST /api/v1/auth/refresh` — `refreshHandler`
- `POST /api/v1/auth/register` — `registerHandler`

### audit-svc

- `GET /api/v1/audit/bidjobs/{id}/report` — `h.getReport`
- `POST /api/v1/audit/bidjobs/{id}/report` — `h.triggerReport`
- `POST /api/v1/audit/bidjobs/{id}/resolve` — `h.resolveIssue`

### billing-svc

- `GET /api/v1/billing/budget/current` — `h.getCurrentBudget`
- `GET /api/v1/billing/plan` — `h.getCurrentPlan`
- `GET /api/v1/billing/transactions` — `h.getTransactions`
- `POST /api/v1/billing/budget` — `h.setBudget`
- `POST /api/v1/billing/checkout` — `h.checkout`
- `POST /api/v1/billing/transactions` — `h.addTransaction`

### document-svc

- `DELETE /api/v1/documents/{id}` — `h.delete`
- `GET /api/v1/documents` — `h.list`
- `GET /api/v1/documents/{id}` — `h.get`
- `GET /api/v1/documents/{id}/content` — `h.download`
- `GET /api/v1/documents/{id}/parse-result` — `h.getParseResult`
- `PATCH /api/v1/documents/{id}` — `h.update`
- `POST /api/v1/documents` — `h.upload`
- `POST /api/v1/documents/document` — `h.exportDocument`
- `POST /api/v1/documents/json` — `h.createJSON`
- `POST /api/v1/documents/{id}/parse` — `h.parse`

### knowledge-svc

- `DELETE /api/v1/kb/materials/{id}` — `h.deleteMaterial`
- `GET /api/v1/kb/materials` — `h.listMaterials`
- `GET /api/v1/kb/materials/{id}` — `h.getMaterial`
- `POST /api/v1/kb/ingest` — `h.ingest`
- `POST /api/v1/kb/materials` — `h.createMaterial`
- `POST /api/v1/kb/search` — `h.search`

### notify-svc

- `DELETE /api/v1/notifications/preferences/{id}` — `h.deletePreference`
- `GET /api/v1/notifications/preferences` — `h.listPreferences`
- `PATCH /api/v1/notifications/preferences/{id}` — `h.updatePreference`
- `POST /api/v1/notifications/preferences` — `h.createPreference`
- `POST /api/v1/notifications/send` — `h.send`

### project-svc

- `DELETE /api/v1/projects/{id}` — `h.delete`
- `GET /api/v1/projects` — `h.list`
- `GET /api/v1/projects/{id}` — `h.get`
- `PATCH /api/v1/projects/{id}` — `h.update`
- `POST /api/v1/projects` — `h.create`

### router-svc

- `GET /api/v1/router/budget` — `h.GetBudget`
- `GET /api/v1/router/cache/stats` — `h.CacheStats`
- `GET /api/v1/router/calls` — `h.ListCalls`
- `GET /api/v1/router/health` — `h.ProviderHealth`
- `GET /api/v1/router/routes` — `h.ListRoutes`
- `GET /api/v1/router/stats` — `h.Stats`
- `POST /api/v1/router/cache/clear` — `h.CacheClear`
- `POST /api/v1/router/chat` — `h.Chat`
- `POST /api/v1/router/embed` — `h.Embed`
- `PUT /api/v1/router/budget` — `h.SetBudget`

### template-svc

- `DELETE /api/v1/templates/{id}` — `h.delete`
- `GET /api/v1/templates` — `h.list`
- `GET /api/v1/templates/{id}` — `h.get`
- `GET /api/v1/templates/{id}/download` — `h.download`
- `PATCH /api/v1/templates/{id}` — `h.update`
- `POST /api/v1/templates` — `h.upload`

### workflow-svc

- `DELETE /api/v1/bids/chapters/{chapterId}` — `h.deleteChapter`
- `GET /api/v1/bids` — `h.list`
- `GET /api/v1/bids/chapters/{chapterId}/content` — `h.getChapterContent`
- `GET /api/v1/bids/outline` — `h.listOutline`
- `GET /api/v1/bids/{id}` — `h.get`
- `GET /api/v1/bids/{id}/events` — `h.listEvents`
- `GET /api/v1/bids/{id}/export/pdf` — `h.exportPDFHandler`
- `GET /api/v1/bids/{id}/export/word` — `h.exportWordHandler`
- `GET /api/v1/bids/{id}/steps` — `h.listSteps`
- `POST /api/v1/bids` — `h.create`
- `POST /api/v1/bids/chapters/{chapterId}/generate` — `h.generateChapter`
- `POST /api/v1/bids/outline` — `h.addChapter`
- `POST /api/v1/bids/{id}/export` — `h.exportDocumentHandler`
- `POST /api/v1/bids/{id}/pause` — `h.pause`
- `POST /api/v1/bids/{id}/resume` — `h.resume`
- `POST /api/v1/bids/{id}/transition` — `h.transition`
- `PUT /api/v1/bids/chapters/{chapterId}` — `h.updateChapter`
- `PUT /api/v1/bids/chapters/{chapterId}/content` — `h.saveChapterContent`
- `PUT /api/v1/bids/material` — `h.saveMaterial`

## 2. 旧 `openapi.yaml` 声明但代码未注册的路由（应删除）

- `DELETE /projects/{id}`
- `GET /healthz`
- `GET /projects`
- `GET /projects/{id}`
- `GET /readyz`
- `PATCH /projects/{id}`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /projects`

## 3. 路径前缀不一致

旧 `openapi.yaml` 使用裸路径（`/projects`、`/auth/login`），实际网关以 `/api/v1/<svc>` 前缀对外暴露。本规范已统一为 `/api/v1/...`。

## 4. 总结

- 旧 `openapi.yaml`：9 ops，覆盖率 0/73 = 0%
- 新 `openapi.generated.yaml`：73 ops，覆盖率 100%
