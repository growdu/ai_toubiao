# project-svc

BidWriter project management service. Owns the **Project** aggregate.

## Endpoints

| Method | Path | Auth | Purpose |
|---|---|---|---|
| GET    | /healthz              | no  | Liveness probe |
| GET    | /readyz               | no  | Readiness probe |
| GET    | /api/v1/projects      | yes | List projects (cursor pagination) |
| POST   | /api/v1/projects      | yes | Create project |
| GET    | /api/v1/projects/{id} | yes | Get one project |
| PATCH  | /api/v1/projects/{id} | yes | Partial update (optimistic lock via `version`) |
| DELETE | /api/v1/projects/{id} | yes | Soft delete |

Full contract: [docs/api/rest.md](../../docs/api/rest.md)
Errors: [docs/api/errors.md](../../docs/api/errors.md)

## Architecture

```
cmd/project-svc/main.go     # entrypoint, wiring, graceful shutdown
internal/
├── api/handlers.go         # HTTP layer (chi router)
├── auth/auth.go            # JWT verification
├── config/config.go        # env-driven config
├── httpx/json.go           # tiny JSON helpers
├── middleware/             # auth, request-id, logging
├── model/project.go        # domain types
└── store/project_store.go  # pgx-based data access
```

All queries filter by `tenant_id` from context (ADR-0001).
No cross-tenant access is possible — verified at compile time by `tenant.MustFromContext`.

## Running locally

```bash
# 1. Start deps
make db-up

# 2. Run migrations
make migrate-up

# 3. Run the service
cd services/project-svc
JWT_SECRET=dev-secret DB_DSN='postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable' \
  go run ./cmd/project-svc

# 4. Test
curl http://localhost:8081/healthz
```

## Configuration

| Env | Default | Required | Description |
|---|---|---|---|
| `HTTP_ADDR` | `:8081` | no | Listen address |
| `DB_DSN` | `postgres://...localhost:5432/bidwriter?sslmode=disable` | no | Postgres DSN |
| `JWT_SECRET` | — | **yes** | HS256 signing secret |
| `LOG_LEVEL` | `info` | no | debug / info / warn / error |
| `SERVICE_NAME` | `project-svc` | no | Emitted in log records |

## Testing

```bash
go test ./...
```

Unit tests for shared packages live in `shared/pkg/*/*_test.go`.
Integration tests for project-svc (using testcontainers) are TODO.