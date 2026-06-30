# api-gateway

The single public entry point for BidWriter. All web/desktop traffic flows through here.

## Responsibilities

| | |
|---|---|
| **Auth**          | Login → JWT issuance (HS256, 1h access + 30d refresh) |
| **Rate limiting** | Per-tenant token bucket, 60 req/min by default |
| **Routing**       | Reverse-proxy to upstream services |
| **CORS**          | Permissive headers for the web frontend |
| **Request ID**    | Generate/propagate X-Request-ID |
| **Header injection** | X-Tenant-ID / X-User-ID / X-User-Role → upstream |

## Route table

| Method | Path                    | Handler |
|---|---|---|
| GET    | `/healthz`              | local (200) |
| POST   | `/api/v1/auth/login`    | local (DB lookup + bcrypt + JWT) |
| POST   | `/api/v1/auth/refresh`  | local (TODO: revocation list) |
| GET    | `/api/v1/projects`      | proxy → project-svc |
| POST   | `/api/v1/projects`      | proxy → project-svc |
| GET    | `/api/v1/projects/{id}` | proxy → project-svc |
| PATCH  | `/api/v1/projects/{id}` | proxy → project-svc |
| DELETE | `/api/v1/projects/{id}` | proxy → project-svc |
| *      | `/api/v1/documents/*`   | proxy → document-svc (TODO) |

## Configuration

| Env | Default | Required | Description |
|---|---|---|---|
| `HTTP_ADDR`        | `:8080`                      | no  | Listen address |
| `DB_DSN`           | `postgres://...localhost:5432/bidwriter` | no | Postgres for user lookup |
| `JWT_SECRET`       | —                            | **yes** | HS256 signing key (shared with other services that verify) |
| `JWT_TTL`          | `1h`                         | no  | Access token lifetime |
| `REFRESH_TTL`      | `720h` (30d)                 | no  | Refresh token lifetime |
| `RATE_LIMIT_PER_MIN` | `60`                       | no  | Per-tenant requests/minute |
| `PROJECT_SVC_URL`  | `http://localhost:8081`      | no  | project-svc address |
| `DOCUMENT_SVC_URL` | `http://localhost:8082`      | no  | document-svc address |

## Running locally

```bash
# 1. Start deps
make db-up

# 2. Migrate
goose -dir migrations postgres "$DB_DSN" up

# 3. Build & run
make build
JWT_SECRET=dev-secret \
DB_DSN="postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable" \
PROJECT_SVC_URL=http://localhost:8081 \
./bin/api-gateway

# 4. Login
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"tenant_slug":"demo-a","email":"admin@demo-a.test","password":"password123"}'
```

## Architecture

```
cmd/api-gateway/main.go          # entrypoint, routing, middleware
internal/
├── auth/         # login, JWT issue/verify, bcrypt
├── config/       # env-driven config
├── proxy/        # httputil.ReverseProxy wrapper
└── ratelimit/    # in-memory per-key token bucket
```

The gateway is **stateless** beyond the in-memory rate limiter. To run
multiple replicas, replace the limiter with Redis (see TODO).

## TODO

- [ ] Refresh-token revocation list (currently NOT_IMPLEMENTED)
- [ ] Redis-backed rate limiter for multi-replica deployments
- [ ] User registration / password reset endpoints
- [ ] MFA / WebAuthn
- [ ] OIDC / OAuth 2.0 third-party login
- [ ] Move user/auth tables to a dedicated auth-svc