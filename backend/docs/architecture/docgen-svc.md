# docgen-svc 服务化架构（Phase 2）

> doc-gen 模块的 HTTP 服务入口，与 bidgen CLI 共享同一 Pipeline 内核。
> 供 workflow-svc 和 api-gateway 调用，实现标书生成的服务化集成。

*最后更新：2026-07-07*

## 1. 定位

docgen-svc 是 doc-gen 内核的 **HTTP 入口壳**，不包含业务逻辑。
所有生成逻辑在 `internal/core.Pipeline` 中，CLI 和服务共享。

```
bidgen CLI ──→ core.Pipeline ←── docgen-svc HTTP
                   │
    ┌──────────────┼──────────────┐
    ▼              ▼              ▼
  Store         LLMClient      Renderer
(SQLite/PG)  (直连/router)   (mmdc/python)
```

## 2. API 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/healthz` | 健康检查 |
| POST | `/api/v1/docgen/generate` | 发起生成任务（异步） |
| GET | `/api/v1/docgen/tasks/{id}` | 查询任务状态 |

### POST /api/v1/docgen/generate

请求体：
```json
{
  "material_dir": "./投标材料",
  "rfp_path": "./招标文件.pdf",
  "out_path": "./标书.docx",
  "no_illustrate": false,
  "no_audit": false,
  "concurrency": 10,
  "budget": 60000
}
```

响应（202 Accepted）：
```json
{
  "task_id": "6a5eb400-ba20-4a0f-9667-2ff0e606e866",
  "status": "pending"
}
```

### GET /api/v1/docgen/tasks/{id}

响应：
```json
{
  "id": "6a5eb400-...",
  "status": "running",
  "started_at": "2026-07-07T20:55:53+08:00",
  "output_path": "",
  "issues": 0
}
```

状态流转：`pending → running → done / failed`

## 3. 异步任务管理

当前用 goroutine + 内存 TaskManager 实现。Phase 2 完整版替换为 Asynq + Redis：

| 维度 | 当前（骨架） | 完整版 |
|---|---|---|
| 任务队列 | goroutine | Asynq + Redis |
| 状态存储 | 内存 map | Redis / PG |
| 任务持久化 | 无（重启丢失） | 有 |
| 任务超时 | context 30min | Asynq timeout |
| 任务重试 | 无 | Asynq retry |

## 4. 配置

环境变量：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `HTTP_ADDR` | `:8090` | 监听地址 |
| `DOCGEN_DB_PATH` | `docgen.db` | SQLite 路径 |
| `ANTHROPIC_AUTH_TOKEN` | — | Anthropic API key |
| `ANTHROPIC_BASE_URL` | — | Anthropic API base |
| `ANTHROPIC_MODEL` | — | 模型名 |
| `LLM_API_KEY` | — | OpenAI API key |
| `LLM_API_BASE` | `https://api.openai.com/v1` | OpenAI base |
| `LLM_MODEL` | `gpt-4o` | 模型名 |
| `MMDC_PATH` | `mmdc` | mermaid-cli 路径 |
| `PYTHON_PATH` | `python3` | Python 路径 |
| `PUPPETEER_CONFIG` | — | Puppeteer 配置路径 |

## 5. 与 workflow-svc 的集成方案

```
workflow-svc 状态机:
  parsing → outlining → facts → generating → auditing → exporting → done
                                        ↑                    ↑
                                    委托 docgen-svc      委托 docgen-svc
```

- `generating` 阶段：`POST /api/v1/docgen/generate { bid_job_id }`
- `exporting` 阶段：`POST /api/v1/docgen/assemble { bid_job_id, format }`
- 轮询 `GET /api/v1/docgen/tasks/{id}` 直到 `done`，回调 workflow 推进状态

## 6. 相关文档

- [doc-gen 模块架构](doc-gen.md)
- [ADR-0011 CLI 优先策略](../decisions/0011-doc-gen-cli-first.md)
- [架构总览](overview.md)
