# 代码规范

> **统一代码风格让所有人能读写。**

## Go 规范

### 1. 基本规则

- Go 1.23+
- `gofmt` + `goimports` 格式化（提交前必跑）
- `golangci-lint run` 通过（无 error）
- 错误必须显式处理（不允许 `_ = err`）

### 2. 命名

| 类型 | 规范 | 示例 |
|---|---|---|
| 包 | 小写单词，不带下划线 | `router`, `audit` |
| 接口 | 动名词或 -er | `Provider`, `Router` |
| 函数 | 驼峰，动词开头 | `ParseRFP`, `GenerateOutline` |
| 导出 | 大写开头 | `Project`, `NewRouter` |
| 私有 | 小写开头 | `parseRFP`, `generateOutline` |
| 常量 | SCREAMING_SNAKE | `MaxRetries` |
| 变量 | 驼峰 | `tenantID`, `maxRetries` |
| 结构体字段 | 驼峰 | `TenantID`, `MaxRetries` |
| 缩写 | 全大写或全小写（保持一致）| `ID` `HTTP` `URL` |

### 3. 错误处理

```go
// ✅ 好：包装错误，保留上下文
result, err := s.repo.Get(ctx, id)
if err != nil {
    return fmt.Errorf("get project %s: %w", id, err)
}

// ✅ 好：sentinel 错误
var ErrProjectNotFound = errors.New("project not found")
if errors.Is(err, ErrProjectNotFound) {
    return nil, ErrProjectNotFound
}

// ❌ 差：忽略错误
_ = s.repo.Save(...)

// ❌ 差：吞掉错误
result, _ := s.repo.Get(ctx, id)
```

### 4. 日志

```go
// ✅ 用 slog
slog.Info("parsing RFP", "project_id", id, "size", size)
slog.Error("parse failed", "err", err, "project_id", id)

// ❌ 不用 fmt.Println / log.Println
fmt.Println("done")
log.Println("done")
```

### 5. Context

```go
// ✅ 第一个参数是 ctx
func (s *Service) Do(ctx context.Context, req Request) (*Result, error)

// ✅ 用 ctx 传递取消信号 / 超时
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()

// ❌ 不要把 ctx 存到结构体
type Service struct {
    ctx context.Context  // 错
}
```

### 6. 并发

```go
// ✅ errgroup 处理并发
g, ctx := errgroup.WithContext(ctx)
for _, item := range items {
    item := item
    g.Go(func() error {
        return s.process(ctx, item)
    })
}
if err := g.Wait(); err != nil {
    return err
}

// ✅ channel 通信
ch := make(chan Result, len(items))
var wg sync.WaitGroup
```

### 7. 测试

- 表格驱动测试
- 命名：`Test<Function>_<Scenario>`
- 至少一个 happy path + 一个 error path

```go
func TestRouter_Route(t *testing.T) {
    tests := []struct {
        name    string
        input   RouteRequest
        want    *RouteResult
        wantErr bool
    }{
        {
            name: "happy path",
            input: RouteRequest{Task: "rfp_parse"},
            want: &RouteResult{Provider: "anthropic"},
        },
        {
            name:  "unknown task",
            input: RouteRequest{Task: "unknown"},
            wantErr: true,
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := router.Route(context.Background(), tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Route() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Route() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### 8. 项目结构

```
services/<service-name>/
├── cmd/
│   └── <service-name>/
│       └── main.go
├── internal/
│   ├── api/             # HTTP handler
│   ├── service/         # 业务逻辑
│   ├── repository/      # 数据访问
│   ├── model/           # 领域模型
│   ├── config/          # 配置
│   └── middleware/
├── pkg/                 # 公共库（可被其他服务 import）
├── configs/
│   ├── routes.yaml
│   └── config.example.yaml
├── migrations/
├── test/
│   ├── integration/
│   └── e2e/
├── Dockerfile
├── Makefile
├── go.mod
├── go.sum
└── .golangci.yml
```

### 9. 必须的工具

`Makefile`：

```makefile
.PHONY: build test lint run dev

build:
	go build -o bin/api-gateway ./cmd/api-gateway

test:
	go test -race -timeout=10m ./...

test-integration:
	go test -tags=integration -timeout=15m ./test/integration/...

lint:
	golangci-lint run --timeout=5m

run:
	go run ./cmd/api-gateway

dev:
	air

coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
```

`.golangci.yml`：

```yaml
linters:
  enable:
    - gofmt
    - goimports
    - govet
    - errcheck
    - staticcheck
    - unused
    - ineffassign
    - gosimple
    - revive
    - gocritic

run:
  timeout: 5m
```

---

## TypeScript / Next.js 规范

### 1. 基本规则

- TypeScript strict 模式（`"strict": true`）
- 禁用 `any`（用 `unknown` 替代）
- `eslint .` 通过
- `prettier --check .` 通过

### 2. 命名

| 类型 | 规范 | 示例 |
|---|---|---|
| 组件 | 帕斯卡 | `ProjectCard` |
| Hook | use 前缀 | `useProject` |
| 函数 | 驼峰，动词开头 | `fetchProject` |
| 类型 / 接口 | 帕斯卡 | `Project`, `ProjectProps` |
| 常量 | SCREAMING_SNAKE | `MAX_RETRIES` |
| 变量 | 驼峰 | `tenantId`, `maxRetries` |
| 文件（组件） | 帕斯卡 | `ProjectCard.tsx` |
| 文件（其他） | kebab-case | `use-project.ts` |

### 3. 类型

```typescript
// ✅ 用 interface 定义对象
interface Project {
  id: string
  name: string
  tenantId: string
  createdAt: Date
}

// ✅ 用 type 定义联合 / 工具类型
type ProjectStatus = 'draft' | 'review' | 'done'

// ❌ 不用 any
function process(data: any) {} // 错
function process(data: unknown) {} // 对，需要 narrow
```

### 4. 组件

```typescript
// ✅ 函数组件 + 类型 props
interface ProjectCardProps {
  project: Project
  onEdit?: (id: string) => void
}

export function ProjectCard({ project, onEdit }: ProjectCardProps) {
  return (
    <div>
      <h2>{project.name}</h2>
      {onEdit && <button onClick={() => onEdit(project.id)}>编辑</button>}
    </div>
  )
}

// ❌ 不用 React.FC（隐式 children 类型问题）
export const ProjectCard: React.FC<ProjectCardProps> = ({ project }) => {}
```

### 5. Hooks

```typescript
// ✅ 命名 use 开头
export function useProject(id: string) {
  return useQuery({
    queryKey: ['project', id],
    queryFn: () => api.projects.get(id),
  })
}

// ✅ 依赖数组正确
useEffect(() => {
  fetchData(id)
}, [id])
```

### 6. 错误处理

```typescript
// ✅ 显式处理
try {
  const result = await api.projects.create(data)
  return result
} catch (err) {
  if (err instanceof ApiError) {
    toast.error(err.message)
  } else {
    toast.error('未知错误')
    console.error(err)
  }
  throw err
}
```

### 7. 测试

- 用 Vitest + React Testing Library
- 单元测试组件行为，不测实现细节

```typescript
describe('ProjectCard', () => {
  it('renders project name', () => {
    render(<ProjectCard project={mockProject} />)
    expect(screen.getByText('测试项目')).toBeInTheDocument()
  })

  it('calls onEdit when button clicked', () => {
    const onEdit = vi.fn()
    render(<ProjectCard project={mockProject} onEdit={onEdit} />)
    fireEvent.click(screen.getByText('编辑'))
    expect(onEdit).toHaveBeenCalledWith('p1')
  })
})
```

### 8. 项目结构

```
web/
├── src/
│   ├── app/                    # Next.js App Router
│   │   ├── (auth)/
│   │   ├── (dashboard)/
│   │   ├── api/
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── components/             # 共享组件
│   │   ├── ui/                 # Radix UI 包装
│   │   └── ...
│   ├── hooks/                  # 自定义 hooks
│   ├── lib/                    # 工具库
│   │   ├── api/
│   │   ├── auth/
│   │   └── utils/
│   ├── stores/                 # Zustand stores
│   ├── styles/
│   └── types/
├── public/
├── tests/
├── package.json
├── pnpm-lock.yaml
├── tsconfig.json
├── next.config.js
├── tailwind.config.ts
└── .eslintrc.json
```

### 9. ESLint 配置

```json
{
  "extends": [
    "next/core-web-vitals",
    "plugin:@typescript-eslint/recommended"
  ],
  "rules": {
    "@typescript-eslint/no-explicit-any": "error",
    "@typescript-eslint/no-unused-vars": ["error", { "argsIgnorePattern": "^_" }],
    "no-console": ["warn", { "allow": ["warn", "error"] }]
  }
}
```

---

## 数据库规范

### 命名

| 类型 | 规范 | 示例 |
|---|---|---|
| 表名 | snake_case 复数 | `projects`, `outline_nodes` |
| 列名 | snake_case | `tenant_id`, `created_at` |
| 主键 | `id` | `id UUID PRIMARY KEY` |
| 外键 | `<table>_id` | `project_id`, `tenant_id` |
| 索引 | `idx_<table>_<columns>` | `idx_projects_tenant_id` |
| 唯一索引 | `uq_<table>_<columns>` | `uq_users_email` |
| 时间 | `_at` 后缀 | `created_at`, `updated_at` |

### 字段类型

- **主键**：`UUID`（v7，可排序），不用 `SERIAL`
- **时间**：`TIMESTAMPTZ`，不用 `TIMESTAMP`
- **JSON**：`JSONB`，不用 `JSON`
- **布尔**：`BOOLEAN`，不用 `INT 0/1`
- **金额**：`DECIMAL(precision, scale)` 或 `BIGINT`（cents）
- **枚举**：用 `VARCHAR` + CHECK 约束（不用 `ENUM` 类型，难改）

### 必备列

每张业务表必备：

```sql
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- ... 业务字段
);
```

### 迁移

- 用 sqlc + golang-migrate
- 文件名：`NNNN_description.sql`（按顺序）
- 永远不要改已应用的迁移

```sql
-- migrations/0001_create_projects.sql
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL REFERENCES tenants(id),
    name VARCHAR(256) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projects_tenant_id ON projects(tenant_id);
CREATE INDEX idx_projects_created_at ON projects(created_at DESC);
```

---

## Commit 规范

用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <subject>

<body>

<footer>
```

**Type**：`feat` | `fix` | `docs` | `refactor` | `perf` | `test` | `chore`

**Scope**：服务名或 `web` / `docs`

**示例**：

```bash
git commit -m "feat(router-svc): add DeepSeek provider

- Implement DeepSeekChat adapter
- Add to routes.yaml for outline_generate task
- Fallback to GPT-4o-mini on failure

Closes #123
Docs: docs/architecture/ai-router.md updated"
```

---

## PR 规范

- 单 PR < 500 行代码
- 标题用 Conventional Commits 风格
- 描述填模板（.github/PULL_REQUEST_TEMPLATE.md）
- 至少 1 人 review
- CI 全绿
- 关联 Issue

---

## 检查清单

提交前：

- [ ] `gofmt` + `goimports`
- [ ] `golangci-lint run`
- [ ] `go test ./...`
- [ ] `pnpm lint`
- [ ] `pnpm typecheck`
- [ ] `pnpm test`
- [ ] `mkdocs build --strict`
- [ ] Conventional Commits 格式
- [ ] 文档已更新

---

## 相关文档

- [开发流程](workflow.md)
- [测试规范](testing.md)
- [Git 工作流](git-workflow.md)
- [AGENTS.md](../AGENTS.md)