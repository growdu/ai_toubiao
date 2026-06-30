# Database Migrations

This directory uses [goose](https://github.com/pressly/goose) format:
`-- +goose Up` / `-- +goose Down` markers with `-- +goose StatementBegin` /
`-- +goose StatementEnd` to delimit statement batches.

## Files

| File | What it does |
|---|---|
| `00001_init.sql`    | Schema: `tenants`, `users`, `projects`, `audit_events` + indexes + updated_at trigger |
| `00002_seed_dev.sql`| Dev-only seed: 2 tenants, 3 users |

## Running

```bash
# Install goose CLI
go install github.com/pressly/goose/v3/cmd/goose@latest

# Apply all
goose -dir migrations postgres "postgres://postgres:postgres@localhost:5432/bidwriter?sslmode=disable" up

# Rollback one
goose -dir migrations postgres "$DSN" down

# Status
goose -dir migrations postgres "$DSN" status
```

Or via the Makefile:

```bash
make migrate
make migrate-status
make migrate-down
```

## Schema overview

```
tenants ───┐
           ├─< users ───< projects
           └─< audit_events
```

- **Every query against `projects` MUST filter by `tenant_id`** (ADR-0001).
- `projects.version` provides optimistic locking.
- `projects.deleted_at` provides soft delete.
- All tables have an `updated_at` trigger.

## Indexes

- `idx_projects_tenant_status` — list by tenant + status
- `idx_projects_tenant_owner`  — list user's projects
- `idx_projects_tenant_id_desc` — cursor pagination
- `idx_users_tenant`           — users by tenant (active only)
- `idx_audit_tenant_time`      — audit log time-series

## Adding a new migration

```bash
goose -dir migrations create add_foo_table sql
# Edit the generated file, then:
make migrate
```

Always provide both Up and Down. Never edit a migration that's already
been applied to production — create a new one instead.