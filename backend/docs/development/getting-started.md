# 快速开始

> **5 分钟跑起来 BidWriter** —— 一个最小可用的本地开发环境。

## 前置条件

- **Docker Desktop** 或 **Docker Engine**（用于 PostgreSQL / Redis / MinIO）
- **Go 1.23+**（[安装](https://go.dev/dl/)）
- **Node.js 20+** 和 **pnpm 9+**（[安装 pnpm](https://pnpm.io/installation)）
- **Git**
- 8GB+ 可用内存
- 50GB+ 磁盘空间

## 1. 克隆仓库

```bash
git clone https://github.com/yourorg/bidwriter.git
cd bidwriter
```

## 2. 启动基础服务

```bash
docker compose up -d
```

这会启动：

| 服务 | 端口 | 用途 |
|---|---|---|
| PostgreSQL 16 | 5432 | 主数据库（含 pgvector） |
| Redis 7 | 6379 | 缓存 + 队列 |
| MinIO | 9000 / 9001 | 对象存储（API / Console） |
| Ollama（可选） | 11434 | 本地 AI 模型 |

等待所有容器 healthy（约 30 秒）：

```bash
docker compose ps
```

## 3. 初始化数据库

```bash
# 应用迁移
make migrate-up

# 或手动：
for f in migrations/*.sql; do
  PGPASSWORD=postgres psql -h localhost -U postgres -d bidwriter -f "$f"
done

# 插入默认数据（行业模板、admin 用户等）
make seed
```

## 4. 启动后端

打开 9 个终端（或用 tmux），分别启动各服务：

```bash
# 终端 1
cd services/api-gateway && cp .env.example .env && go run ./cmd/api-gateway

# 终端 2
cd services/project-svc && go run ./cmd/project-svc

# 终端 3
cd services/document-svc && go run ./cmd/document-svc

# 终端 4
cd services/workflow-svc && go run ./cmd/workflow-svc

# 终端 5
cd services/knowledge-svc && go run ./cmd/knowledge-svc

# 终端 6
cd services/router-svc && go run ./cmd/router-svc

# 终端 7
cd services/template-svc && go run ./cmd/template-svc

# 终端 8
cd services/billing-svc && go run ./cmd/billing-svc

# 终端 9
cd services/audit-svc && go run ./cmd/audit-svc
```

或用 `air`（热重载）：

```bash
make dev
```

**健康检查**：

```bash
curl http://localhost:8080/healthz
# {"status":"ok","services":{"postgres":"up","redis":"up","minio":"up"}}
```

## 5. 启动前端

```bash
cd web
pnpm install
cp .env.example .env.local
pnpm dev
```

打开浏览器：<http://localhost:3000>

默认账号：

| 邮箱 | 密码 | 角色 |
|---|---|---|
| admin@bidwriter.local | admin123 | Owner |
| user@bidwriter.local | user123 | Member |

## 6. 配置 AI Provider

至少配置一个 AI Provider：

```bash
# 编辑 services/router-svc/.env
echo "OPENAI_API_KEY=sk-..." >> services/router-svc/.env
echo "ANTHROPIC_API_KEY=sk-ant-..." >> services/router-svc/.env
echo "DEEPSEEK_API_KEY=sk-..." >> services/router-svc/.env

# 重启 router-svc
cd services/router-svc && go run ./cmd/router-svc
```

或用本地 Ollama（无需 API Key）：

```bash
# 拉取模型
docker exec -it bidwriter-ollama ollama pull qwen2.5:7b-instruct

# 配置 router-svc 使用 Ollama
# 编辑 services/router-svc/configs/routes.yaml
# 把 OpenAI 的 base_url 改成 http://ollama:11434/v1
```

## 7. 验证

按这个流程走一遍：

1. 登录 → 创建项目 → 选"信息化系统集成"模板
2. 上传一个示例招标文档（`examples/sample-rfp.docx`）
3. 触发 Step02 解析
4. 等待 Step02 完成（通常 30-60 秒）
5. 进入 Step03 大纲审核
6. 触发 Step04 + Step05
7. 触发废标审计
8. 导出 Word

完整流程跑通 = 开发环境就绪 ✅

---

## 常见问题

### Q: `docker compose` 启动失败

```bash
# 检查端口冲突
lsof -i :5432
lsof -i :6379

# 看日志
docker compose logs postgres
```

### Q: 数据库迁移失败

```bash
# 强制重置（⚠️ 删数据）
make migrate-reset
```

### Q: 后端起不来

```bash
# 检查 .env 文件
cat services/api-gateway/.env

# 详细日志
cd services/api-gateway
LOG_LEVEL=debug go run ./cmd/api-gateway
```

### Q: 前端编译报错

```bash
cd web
rm -rf node_modules .next
pnpm install
pnpm dev
```

### Q: AI 调用超时

- 检查网络（外网是否通）
- 检查 API Key 是否有效
- 看 router-svc 日志

---

## 下一步

- [开发流程](workflow.md) — 必读！理解"先文档后代码"
- [架构总览](../architecture/overview.md)
- [代码规范](coding-standards.md)
- [测试规范](testing.md)
- [Git 工作流](git-workflow.md)

---

## IDE 配置建议

### VS Code

推荐扩展：

- Go (golang.go)
- TypeScript
- ESLint
- Prettier
- Markdown All in One
- Mermaid Preview
- GitLens

`.vscode/settings.json`：

```json
{
  "go.lintTool": "golangci-lint",
  "go.testFlags": ["-race"],
  "editor.formatOnSave": true,
  "typescript.tsdk": "node_modules/typescript/lib"
}
```

### GoLand / IntelliJ

- 安装 Go 插件
- 配置 `gofmt` + `goimports`
- 启用 `go vet`

---

## 性能建议

- **数据库**：至少分配 2GB 给 PostgreSQL
- **Redis**：至少 512MB
- **AI 模型**：本地 Ollama 用 7B 模型起步，避免卡顿
- **IDE**：TypeScript 项目用 `pnpm`（比 npm 快 3-5 倍）

---

## 相关文档

- [架构 / 模块设计](../architecture/modules.md)
- [运维 / 部署](../operations/deployment.md)
- [开发流程](workflow.md)