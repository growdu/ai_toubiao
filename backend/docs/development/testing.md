# 测试规范

> **没有测试的代码是不可信的。**

## 测试金字塔

```
        ╱╲
       ╱  ╲           E2E 测试（少量）
      ╱────╲          关键路径全流程
     ╱      ╲
    ╱────────╲        集成测试（适量）
   ╱          ╲       服务间 + DB
  ╱────────────╲
 ╱              ╲     单元测试（大量）
╱────────────────╲    函数 / 方法
```

| 层 | 占比 | 速度 | 范围 |
|---|---|---|---|
| 单元测试 | 70% | < 1s/个 | 函数 / 方法 |
| 集成测试 | 20% | < 10s/个 | 服务 + DB + 外部 |
| E2E 测试 | 10% | < 60s/个 | 完整用户路径 |

## 覆盖率目标

| 代码类型 | 目标 |
|---|---|
| 业务代码（service） | ≥ 80% |
| API handler | ≥ 75% |
| 工具代码（util） | ≥ 60% |
| UI 组件 | ≥ 50% |
| AI 调用代码 | ≥ 70%（mock 外部）|

覆盖率 < 80% 的 PR 需要 reviewer 特别说明。

---

## 单元测试

### Go

- 用 `testing` + `testify/assert`
- 表格驱动
- 命名：`Test<Function>_<Scenario>`

```go
func TestParseRFP_ValidInput(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *RFP
        wantErr bool
    }{
        {name: "simple", input: "招标...", want: &RFP{...}},
        {name: "empty", input: "", wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := ParseRFP(tt.input)
            if (err != nil) != tt.wantErr {
                t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
            }
            if !reflect.DeepEqual(got, tt.want) {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### TypeScript

- 用 Vitest + React Testing Library
- 命名：`describe('Component', () => { it('does X', ...) })`

```typescript
describe('ProjectCard', () => {
  it('renders project name', () => {
    render(<ProjectCard project={mockProject} />)
    expect(screen.getByText('测试项目')).toBeInTheDocument()
  })
})
```

---

## 集成测试

### Go 集成测试

文件加 build tag：

```go
//go:build integration
// +build integration

package service_test

import (
    "testing"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestProjectService_Integration(t *testing.T) {
    // 启动测试 DB（testcontainers）
    pg, err := postgres.RunContainer(ctx, ...)
    require.NoError(t, err)
    defer pg.Terminate(ctx)

    // 跑测试
    // ...
}
```

运行：

```bash
go test -tags=integration -timeout=15m ./test/integration/...
```

### Web 集成测试

用 Playwright：

```typescript
import { test, expect } from '@playwright/test'

test('user can create project', async ({ page }) => {
  await page.goto('/login')
  await page.fill('input[name="email"]', 'admin@bidwriter.local')
  await page.fill('input[name="password"]', 'admin123')
  await page.click('button[type="submit"]')

  await page.goto('/projects/new')
  await page.fill('input[name="name"]', '测试项目')
  await page.click('button:has-text("创建")')

  await expect(page.locator('h1')).toContainText('测试项目')
})
```

---

## Mock / Stub

### Go（mockery）

```go
//go:generate mockery --name=Router --output=mocks
type Router interface {
    Route(ctx context.Context, req RouteRequest) (*RouteResult, error)
}

// 测试中
mockRouter := &mocks.Router{}
mockRouter.On("Route", mock.Anything, mock.Anything).Return(&RouteResult{...}, nil)
svc := NewService(mockRouter)
```

### TypeScript（MSW / vi.mock）

```typescript
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'

const server = setupServer(
  http.post('/api/v1/projects', () => {
    return HttpResponse.json({ id: 'p1', name: '测试' })
  })
)

beforeAll(() => server.listen())
afterEach(() => server.resetHandlers())
afterAll(() => server.close())
```

---

## AI 调用测试

AI 是外部依赖，必须 mock。绝不真实调用 AI 跑 CI。

```go
func TestRouter_WithMockProvider(t *testing.T) {
    // 用 fake provider
    fake := &FakeProvider{
        Response: `{"sections": [...]}`,
    }
    router := NewRouter(fake)
    // ...
}
```

E2E 测试可真实调用 AI（可选，标记 `slow`）。

---

## 数据库测试

- 每个测试用独立 schema（`test_<uuid>`）
- 测试结束清理

```go
func setupTestDB(t *testing.T) (*sql.DB, func()) {
    db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
    require.NoError(t, err)

    schema := fmt.Sprintf("test_%s", uuid.New().String()[:8])
    _, err = db.Exec(fmt.Sprintf("CREATE SCHEMA %s", schema))
    require.NoError(t, err)

    _, err = db.Exec(fmt.Sprintf("SET search_path TO %s", schema))
    require.NoError(t, err)

    // 应用迁移
    applyMigrations(t, db)

    cleanup := func() {
        db.Exec(fmt.Sprintf("DROP SCHEMA %s CASCADE", schema))
        db.Close()
    }

    return db, cleanup
}
```

---

## 性能测试

### 基准测试（Go）

```go
func BenchmarkRouter_Route(b *testing.B) {
    router := NewRouter(&FakeProvider{})
    req := RouteRequest{Task: "rfp_parse"}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = router.Route(context.Background(), req)
    }
}
```

运行：

```bash
go test -bench=BenchmarkRouter_Route -benchmem ./...
```

### 负载测试（k6）

```javascript
// scripts/load-test.js
import http from 'k6/http'
import { check } from 'k6'

export const options = {
  stages: [
    { duration: '1m', target: 50 },
    { duration: '3m', target: 50 },
    { duration: '1m', target: 0 },
  ],
}

export default function () {
  const res = http.post('http://localhost:8080/api/v1/projects', JSON.stringify({
    name: 'load-test',
  }))
  check(res, { 'status 201': (r) => r.status === 201 })
}
```

---

## 测试命令

### 后端

```bash
# 单元测试
go test -race -timeout=10m ./...

# 集成测试（需要 DB）
go test -tags=integration -timeout=15m ./test/integration/...

# 覆盖率
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# 基准测试
go test -bench=. -benchmem ./...

# 全部
make test-all
```

### 前端

```bash
# 单元 + 组件
pnpm test

# E2E（需要 Playwright）
pnpm test:e2e

# 类型检查
pnpm typecheck

# 全部
make test-all
```

---

## CI 集成

GitHub Actions 自动跑：

1. 单元测试（每次 PR）
2. 集成测试（main 分支 + 关键 PR）
3. E2E 测试（main 分支 + 每日）
4. 覆盖率（codecov）

详细：[.github/workflows/ci.yml](../.github/workflows/ci.yml)

---

## 写测试的好习惯

### ✅ DO

- 测行为，不测实现
- 一个测试一个断言（或一组相关断言）
- 失败信息清晰
- 测试要快（< 1s）
- 测试要独立（不依赖顺序）
- 用真实的数据（mock 时也要真实结构）

### ❌ DON'T

- 测试实现细节（私有方法）
- 测试间共享状态
- 用 sleep 等异步
- 写脆弱的测试（依赖 UI 结构）
- 真实调用外部 API
- 提交不稳定的测试（flaky）

---

## 测试反模式

### ❌ 测试用例只覆盖 happy path

```go
// ❌ 只测成功
func TestParse(t *testing.T) {
    got, err := Parse(validInput)
    if err != nil {
        t.Fatal(err)
    }
    if got == nil {
        t.Fatal("expected result")
    }
}
```

### ✅ 测 happy + error + edge

```go
// ✅ 测多种情况
func TestParse(t *testing.T) {
    t.Run("valid", ...)
    t.Run("empty input", ...)
    t.Run("invalid format", ...)
    t.Run("timeout", ...)
}
```

### ❌ 测试互相依赖

```go
// ❌ 全局状态
var globalProject *Project

func TestA(t *testing.T) {
    globalProject = createProject()
}
func TestB(t *testing.T) {
    // 依赖 globalProject
}
```

### ✅ 每个测试独立

```go
// ✅ 独立 setup
func TestA(t *testing.T) {
    p := createProject()
    // ...
}
func TestB(t *testing.T) {
    p := createProject()
    // ...
}
```

---

## 相关文档

- [代码规范](coding-standards.md)
- [开发流程](workflow.md)
- [架构 / 模块设计](../architecture/modules.md)